package metadataexporter

import (
	"context"
	"fmt"
	"strings"
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

// record appends a single path/value pair to the batch. Only String, Bytes,
// and Slice values are written; any other type is a caller error.
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
		return fmt.Errorf("unsupported value type %v at path %q", val.Type(), path)
	}
}

func (va *valueAccumulator) flush() error {
	return va.stmt.Send()
}

// appendTypeSet appends all type records from ts to stmt using the given signal and context.
// The caller is responsible for calling stmt.Send() after all type sets have been appended.
func appendTypeSet(ts *typeSet, signal, context string, stmt driver.Batch, now int64) error {
	var iterErr error
	ts.types.Range(func(key, value any) bool {
		path := key.(string)
		cs := value.(*utils.ConcurrentSet[string])
		for _, typeStr := range cs.Keys() {
			if err := stmt.Append(signal, context, path, typeStr, now); err != nil {
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
	// One typeSet per source so each gets its own signal/context on flush.
	typeSets := make([]*typeSet, len(w.sources))
	for i := range w.sources {
		typeSets[i] = &typeSet{}
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
					val, err := extractSource(lr, src)
					if err != nil {
						w.logger.Error("unsupported json source", zap.String("source", string(src)), zap.Error(err))
						continue
					}
					if val.Type() != pcommon.ValueTypeMap {
						continue
					}
					if err := w.walk(ctx, val, src, unixMilli, typeSets[si], va); err != nil {
						w.logger.Error("json walk failed", zap.String("source", string(src)), zap.Error(err))
					}
				}
			}
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.flushTypeSets(gCtx, typeSets) })
	g.Go(va.flush)
	return g.Wait()
}

// flushTypeSets creates a single shared batch for all type sets, appends each
// set's records using the corresponding source as context, and sends in one shot.
func (w *jsonMetadataWriter) flushTypeSets(ctx context.Context, typeSets []*typeSet) error {
	sql := fmt.Sprintf("INSERT INTO %s (signal, context, path, type, last_seen) VALUES (?, ?, ?, ?, ?)", distributedPathTypesTable)
	stmt, err := w.conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare path types batch: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UnixNano()
	signal := pipeline.SignalLogs.String()
	for i, ts := range typeSets {
		if err := appendTypeSet(ts, signal, string(w.sources[i]), stmt, now); err != nil {
			return fmt.Errorf("failed to append to path types batch: %w", err)
		}
	}
	return stmt.Send()
}

// extractSource returns the pcommon.Value for the given tag type from a log record.
// It returns an error for unsupported tag types so callers cannot silently skip input.
func extractSource(lr plog.LogRecord, tagType utils.TagType) (pcommon.Value, error) {
	switch tagType {
	case utils.TagTypeBody:
		return lr.Body(), nil
	default:
		return pcommon.NewValueEmpty(), fmt.Errorf("unsupported tag type %q", tagType)
	}
}

// walk traverses a pcommon.Value, feeding both the typeSet (for
// distributed_json_path_types) and the valueAccumulator (for
// distributed_tag_attributes_v2) in a single pass.
func (w *jsonMetadataWriter) walk(
	ctx context.Context,
	val pcommon.Value,
	tagType utils.TagType,
	unixMilli int64,
	ts *typeSet,
	va *valueAccumulator,
) error {
	return w.walkNode(ctx, "", val, 0, tagType, unixMilli, ts, va)
}

// walkNode is the recursive core of walk. It handles type-hint guards, depth
// limiting, and dispatches to walkMap / walkSlice for container types.
func (w *jsonMetadataWriter) walkNode(
	ctx context.Context,
	prefix string,
	val pcommon.Value,
	level int,
	tagType utils.TagType,
	unixMilli int64,
	ts *typeSet,
	va *valueAccumulator,
) error {
	// Skip children of the "message" type-hint field (fixed column).
	if strings.HasPrefix(prefix, jsonMessageField+".") {
		return nil
	} else if prefix == jsonMessageField {
		ts.record(prefix, maskString)
		// Record value suggestions only for string-typed message fields.
		if val.Type() == pcommon.ValueTypeStr || val.Type() == pcommon.ValueTypeBytes {
			return va.record(ctx, prefix, val, tagType, unixMilli)
		}
		return nil
	}

	// Depth guard: do not descend into containers beyond the configured limit.
	if level >= w.cfg.MaxDepthTraverse &&
		(val.Type() == pcommon.ValueTypeMap || val.Type() == pcommon.ValueTypeSlice) {
		return nil
	}

	switch val.Type() {
	case pcommon.ValueTypeMap:
		return w.walkMap(ctx, prefix, val, level, tagType, unixMilli, ts, va)
	case pcommon.ValueTypeSlice:
		return w.walkSlice(ctx, prefix, val, level, tagType, unixMilli, ts, va)
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

// walkMap iterates over a map, skips high-cardinality keys, and recurses into
// each child value.
func (w *jsonMetadataWriter) walkMap(
	ctx context.Context,
	prefix string,
	val pcommon.Value,
	level int,
	tagType utils.TagType,
	unixMilli int64,
	ts *typeSet,
	va *valueAccumulator,
) error {
	m := val.Map()
	if m.Len() > w.cfg.MaxKeysAtLevel {
		return nil
	}
	var iterErr error
	m.Range(func(key string, child pcommon.Value) bool {
		// Skip high-cardinality keys (e.g. UUIDs used as map keys).
		if !w.cardinalKeyCache.Contains(key) {
			if keycheck.IsCardinal(key) {
				return true
			}
			w.cardinalKeyCache.Add(key, struct{}{})
		}
		childPath := joinPath(prefix, key)
		if err := w.walkNode(ctx, childPath, child, level+1, tagType, unixMilli, ts, va); err != nil {
			iterErr = err
			return false
		}
		return true
	})
	return iterErr
}

// walkSlice collects element types, recurses into map elements, and records
// the inferred array type mask.
func (w *jsonMetadataWriter) walkSlice(
	ctx context.Context,
	prefix string,
	val pcommon.Value,
	level int,
	tagType utils.TagType,
	unixMilli int64,
	ts *typeSet,
	va *valueAccumulator,
) error {
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
			if err := w.walkNode(ctx, prefix+jsonArraySuffix, el, level, tagType, unixMilli, ts, va); err != nil {
				return err
			}
			types = append(types, el.Type())
		case pcommon.ValueTypeSlice:
			w.logger.Error("nested arrays not supported", zap.String("path", prefix))
			return nil
		case pcommon.ValueTypeEmpty:
			// skip empty elements
		default:
			types = append(types, el.Type())
		}
	}
	if len(types) == 0 {
		return nil
	}
	if mask := inferArrayMask(types); mask != 0 {
		ts.record(prefix, mask)
		if mask != maskArrayDynamic && mask != maskArrayJSON {
			return va.record(ctx, prefix, val, tagType, unixMilli)
		}
	}
	return nil
}

// joinPath builds a dotted JSON path from a parent prefix and a child key.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
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
