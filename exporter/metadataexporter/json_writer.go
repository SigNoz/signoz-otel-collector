package metadataexporter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/internal/common"
	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
	"github.com/SigNoz/signoz-otel-collector/utils"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	distributedPathTypesTable  = constants.SignozMetadataDB + "." + constants.DistributedPathTypesTable
	distributedTagAttrsV2Table = "signoz_logs.distributed_tag_attributes_v2"

	jsonArraySuffix  = "[]"
	jsonMessageField = "message"
)

// valueAccumulator writes tag attribute rows directly to a ClickHouse batch,
// applying cardinality guards before each append.
type valueAccumulator struct {
	stmt driver.Batch

	shouldSkipFromDB func(ctx context.Context, key, datasource string) bool
	shouldSkipUVT    func(ctx context.Context, key, datasource string) bool
	addToUVT         func(ctx context.Context, uvtKey, value, datasource string)
}

// record appends a single path/value pair to the batch. Only String, Slices
// is written; all other types are silently skipped.
func (va *valueAccumulator) record(ctx context.Context, path string, val pcommon.Value, tagType utils.TagType, unixMilli int64) error {
	switch val.Type() {
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes, pcommon.ValueTypeSlice:
		// record string + slices value
		s := val.AsString()
		if len(s) == 0 || len(s) > common.MaxAttributeValueLength {
			return nil
		}

		ds := pipeline.SignalLogs.String()
		key := utils.MakeKeyForAttributeKeys(path, tagType, utils.TagDataTypeString)
		if va.shouldSkipFromDB(ctx, key, ds) {
			return nil
		}
		if va.shouldSkipUVT(ctx, key, ds) {
			return nil
		}
		va.addToUVT(ctx, makeUVTKey(key, ds), s, ds)
		return va.stmt.Append(unixMilli, path, tagType, utils.TagDataTypeString, s, nil)
	default:
		return nil
	}
}

func (va *valueAccumulator) flush() error {
	return va.stmt.Send()
}

// typeSetFlusher wraps typeSet with a flush method that writes to distributed_json_path_types.
type typeSetFlusher struct {
	ts      *typeSet
	signal  string
	context string
}

// Append appends this flusher's type records to the provided shared batch statement.
// The caller is responsible for calling stmt.Send() after all flushers have appended.
func (f *typeSetFlusher) Append(stmt driver.Batch, now int64) error {
	var iterErr error
	f.ts.types.Range(func(key, value any) bool {
		path := key.(string)
		cs := value.(*utils.ConcurrentSet[string])
		for _, typeStr := range cs.Keys() {
			if err := stmt.Append(f.signal, f.context, path, typeStr, now); err != nil {
				iterErr = err
				return false
			}
		}
		return true
	})
	return iterErr
}

// jsonMetadataWriter is a logsMetadataWriter that performs a single walk over
// configured JSON sources (body today, attributes in future) per log record,
// feeding both type collection and value suggestions from one traversal.
//
// It writes to two tables per flush:
//   - signoz_metadata.distributed_json_path_types  (path → ClickHouse type)
//   - signoz_logs.distributed_tag_attributes_v2    (path → string value, tag_type=source)
type jsonMetadataWriter struct {
	sources          []utils.TagType
	cfg              JSONConfig
	logger           *zap.Logger
	cardinalKeyCache *lru.Cache[string, struct{}]
	conn             driver.Conn

	shouldSkipFromDB func(ctx context.Context, key, datasource string) bool
	shouldSkipUVT    func(ctx context.Context, key, datasource string) bool
	addToUVT         func(ctx context.Context, uvtKey, value, datasource string)
}

func newJSONMetadataWriter(
	sources []utils.TagType,
	cfg JSONConfig,
	logger *zap.Logger,
	e *metadataExporter,
) (*jsonMetadataWriter, error) {
	cache, err := lru.New[string, struct{}](defaultJSONKeyCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cardinal key cache: %w", err)
	}
	return &jsonMetadataWriter{
		sources:          sources,
		cfg:              cfg,
		logger:           logger,
		cardinalKeyCache: cache,
		conn:             e.conn,
		shouldSkipFromDB: e.shouldSkipAttributeFromDB,
		shouldSkipUVT:    e.shouldSkipAttributeUVT,
		addToUVT:         e.addToUVT,
	}, nil
}

func (w *jsonMetadataWriter) ProcessMetadata(ctx context.Context, ld plog.Logs) error {
	// One typeSet (and flusher) per source so each gets the correct signal/context.
	tsFlushers := make([]*typeSetFlusher, len(w.sources))
	for i, src := range w.sources {
		tsFlushers[i] = &typeSetFlusher{
			ts:      &typeSet{types: sync.Map{}},
			signal:  pipeline.SignalLogs.String(),
			context: string(src),
		}
	}

	vaStmt, err := w.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s", distributedTagAttrsV2Table), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare tag attrs batch: %w", err)
	}
	defer vaStmt.Close()

	va := &valueAccumulator{
		stmt:             vaStmt,
		shouldSkipFromDB: w.shouldSkipFromDB,
		shouldSkipUVT:    w.shouldSkipUVT,
		addToUVT:         w.addToUVT,
	}

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)
				unixMilli := lr.Timestamp().AsTime().UnixMilli()

				for si, src := range w.sources {
					val := w.extractSource(lr, src)
					if val.Type() != pcommon.ValueTypeMap {
						continue
					}
					if err := w.walk(ctx, val, src, unixMilli, tsFlushers[si].ts, va); err != nil {
						w.logger.Error("json walk failed", zap.String("source", string(src)), zap.Error(err))
					}
				}
			}
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.AppendAndFlush(gCtx, tsFlushers) })
	g.Go(va.flush)
	return g.Wait()
}

// AppendAndFlush creates a single shared batch statement for all typeSetFlushers,
// appends each flusher's records to it, and sends the batch in one shot.
func (w *jsonMetadataWriter) AppendAndFlush(ctx context.Context, flushers []*typeSetFlusher) error {
	sql := fmt.Sprintf("INSERT INTO %s (signal, context, path, type, last_seen) VALUES (?, ?, ?, ?, ?)", distributedPathTypesTable)
	stmt, err := w.conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare path types batch: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UnixNano()
	for _, f := range flushers {
		if err := f.Append(stmt, now); err != nil {
			return fmt.Errorf("failed to append to path types batch: %w", err)
		}
	}
	return stmt.Send()
}

func (w *jsonMetadataWriter) extractSource(lr plog.LogRecord, tagType utils.TagType) pcommon.Value {
	switch tagType {
	case utils.TagTypeBody:
		return lr.Body()
	default:
		return pcommon.NewValueEmpty()
	}
}

// walk traverses a pcommon.Value without converting to Go maps, feeding both
// the typeSet (for distributed_json_path_types) and the valueAccumulator (for
// distributed_tag_attributes_v2) in a single pass.
func (w *jsonMetadataWriter) walk(
	ctx context.Context,
	val pcommon.Value,
	tagType utils.TagType,
	unixMilli int64,
	ts *typeSet,
	va *valueAccumulator,
) error {
	generatePath := func(prefix, key string) string {
		if prefix == "" {
			return key
		}
		return prefix + "." + key
	}

	var closure func(prefix string, val pcommon.Value, level int) error
	closure = func(prefix string, val pcommon.Value, level int) error {
		// Skip diving into type-hint fields (e.g. "message" is a fixed column).
		if strings.HasPrefix(prefix, jsonMessageField+".") {
			return nil
		}
		if prefix == jsonMessageField {
			ts.record(prefix, maskString)
			// record value suggestions
			return va.record(ctx, prefix, val, tagType, unixMilli)
		}

		// Depth guard: do not descend into containers beyond the configured limit.
		if level >= w.cfg.MaxDepthTraverse &&
			(val.Type() == pcommon.ValueTypeMap || val.Type() == pcommon.ValueTypeSlice) {
			return nil
		}

		switch val.Type() {
		case pcommon.ValueTypeMap:
			m := val.Map()
			if m.Len() > w.cfg.MaxKeysAtLevel {
				return nil
			}
			var iterErr error
			m.Range(func(key string, value pcommon.Value) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}
				// Skip high-cardinality keys (e.g. UUIDs used as map keys).
				if !w.cardinalKeyCache.Contains(key) {
					if keycheck.IsCardinal(key) {
						return true
					}
					w.cardinalKeyCache.Add(key, struct{}{})
				}
				if err := closure(generatePath(prefix, key), value, level+1); err != nil {
					iterErr = err
					return false
				}
				return true
			})
			return iterErr
		case pcommon.ValueTypeSlice:
			s := val.Slice()
			if s.Len() == 0 || s.Len() > w.cfg.MaxArrayElementsAllowed {
				return nil
			}
			types := make([]pcommon.ValueType, 0, s.Len())
			for i := 0; i < s.Len(); i++ {
				el := s.At(i)
				switch el.Type() {
				case pcommon.ValueTypeMap:
					// Do not increment depth for array element objects — array indexing
					// should not consume depth budget.
					if err := closure(prefix+jsonArraySuffix, el, level); err != nil {
						return err
					}
					types = append(types, el.Type())
				case pcommon.ValueTypeSlice:
					w.logger.Error("nested arrays not supported", zap.String("path", prefix))
					return nil
				default:
					if el.Type() == pcommon.ValueTypeEmpty {
						continue
					}
					types = append(types, el.Type())
				}
			}
			if len(types) == 0 {
				return nil
			}
			if mask := inferArrayMask(types); mask != 0 {
				ts.record(prefix, mask)
				switch mask {
				case maskArrayDynamic, maskArrayJSON:
					// skip recording complex array types
				default:
					return va.record(ctx, prefix, val, tagType, unixMilli)
				}
			}
			return nil
		// handle primitive types
		case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
			ts.record(prefix, maskString)
			return va.record(ctx, prefix, val, tagType, unixMilli)
		case pcommon.ValueTypeBool:
			ts.record(prefix, maskBool)
		case pcommon.ValueTypeDouble:
			ts.record(prefix, maskFloat)
		case pcommon.ValueTypeInt:
			ts.record(prefix, maskInt)
		default:
			return fmt.Errorf("unknown value type at path %q: %v", prefix, val.Type())
		}

		return nil
	}

	return closure("", val, 0)
}

// inferArrayMask determines the correct array type bitmask from a slice of element types.
func inferArrayMask(types []pcommon.ValueType) uint16 {
	unique := make(map[pcommon.ValueType]bool, len(types))
	for _, t := range types {
		if t == pcommon.ValueTypeBytes {
			t = pcommon.ValueTypeStr
		}
		unique[t] = true
	}

	hasJSON := unique[pcommon.ValueTypeMap]
	hasPrimitive := (hasJSON && len(unique) > 1) || (!hasJSON && len(unique) > 0)

	if hasJSON {
		if !hasPrimitive {
			return maskArrayJSON
		}
		return maskArrayDynamic
	}

	if unique[pcommon.ValueTypeStr] {
		if len(unique) > 1 {
			return maskArrayDynamic
		}
		return maskArrayString
	}
	if unique[pcommon.ValueTypeDouble] {
		return maskArrayFloat
	}
	if unique[pcommon.ValueTypeInt] {
		return maskArrayInt
	}
	if unique[pcommon.ValueTypeBool] {
		return maskArrayBool
	}
	return maskArrayDynamic
}
