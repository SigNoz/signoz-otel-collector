package metadataexporter

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
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

	shouldSkipFromDB func(key string) bool
	shouldSkipUVT    func(key string) bool
	addToUVT         func(key string, value any)
}

// guard runs the DB and UVT skip checks. When value is non-nil it also
// records the value in UVT. Returns true when the record should be skipped.
func (va *valueAccumulator) guard(path string, value any, tagType utils.TagType) bool {
	key := utils.MakeKeyForAttributeKeys(path, tagType, utils.TagDataTypeString)
	if va.shouldSkipFromDB(key) {
		return true
	}
	if value == nil {
		return false
	}
	if va.shouldSkipUVT(key) {
		return true
	}
	va.addToUVT(key, value)
	return false
}

// record appends a single path/value pair to the batch. Supports String,
// Bytes, Int, Double, and Bool values; any other type is a caller error.
func (va *valueAccumulator) record(path string, val pcommon.Value, tagType utils.TagType, unixMilli int64) error {
	switch val.Type() {
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
		str := val.AsString()
		if len(str) == 0 || len(str) > common.MaxAttributeValueLength {
			return nil
		}
		if va.guard(path, str, tagType) {
			return nil
		}
		return va.stmt.Append(unixMilli, path, tagType, utils.TagDataTypeString, str, nil)
	case pcommon.ValueTypeInt, pcommon.ValueTypeDouble:
		floatVal := val.Double()
		if val.Type() == pcommon.ValueTypeInt {
			floatVal = float64(val.Int())
		}
		if va.guard(path, floatVal, tagType) {
			return nil
		}
		return va.stmt.Append(unixMilli, path, tagType, utils.TagDataTypeNumber, nil, floatVal)
	case pcommon.ValueTypeBool:
		// Cardinality is always 2; no UVT tracking needed.
		return va.stmt.Append(unixMilli, path, tagType, utils.TagDataTypeBool, nil, nil)
	}
	return fmt.Errorf("unsupported value type %v at path %q", val.Type(), path)
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

// jsonMetadataWriter is a LogsMetadataWriter that performs a single walk over
// configured JSON sources (body today, attributes in future) per log record,
// feeding both type collection and value suggestions from one traversal.
//
// It writes to two tables per flush:
//   - signoz_metadata.distributed_json_path_types  (path → ClickHouse type)
//   - signoz_logs.distributed_tag_attributes_v2    (path → value, tag_type=source)
type jsonMetadataWriter struct {
	cfg              JSONConfig
	logger           *zap.Logger
	cardinalKeyCache *lru.Cache[string, struct{}]
	conn             driver.Conn

	// skipKeys is swapped atomically by the background fetcher.
	// Presence in the map means the key's distinct-value count exceeded the limit.
	skipKeys atomic.Pointer[map[string]struct{}]

	// inherited fields from metadataexporter
	valueTracker *ValueTracker
	limits       LimitsConfig
}

func newJSONMetadataWriter(
	ctx context.Context,
	cfg JSONConfig,
	logger *zap.Logger,
	e *metadataExporter,
) (*jsonMetadataWriter, error) {
	cache, err := lru.New[string, struct{}](defaultJSONKeyCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cardinal key cache: %w", err)
	}
	w := &jsonMetadataWriter{
		cfg:              cfg,
		logger:           logger,
		cardinalKeyCache: cache,
		conn:             e.conn,
		valueTracker:     e.logsTracker,
		limits:           e.cfg.MaxDistinctValues.Logs,
	}
	empty := make(map[string]struct{})
	w.skipKeys.Store(&empty)

	go w.skipKeysTicker(ctx)

	return w, nil
}

func (w *jsonMetadataWriter) skipKeysTicker(ctx context.Context) {
	w.fetchSkipKeys(ctx)
	if w.limits.FetchInterval <= 0 {
		return
	}
	ticker := time.NewTicker(w.limits.FetchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.fetchSkipKeys(ctx)
		}
	}
}

// fetchSkipKeys queries distributed_tag_attributes_v2 for keys whose
// distinct-value count exceeds the limit and atomically replaces skipKeys.
//
// TODO(Piyush): We've used MaxStringDistinctValues for all the types, look into it in future
func (w *jsonMetadataWriter) fetchSkipKeys(ctx context.Context) {
	query := fmt.Sprintf(`
		SELECT tag_key, tag_type, tag_data_type
		FROM %s
		WHERE unix_milli >= toUnixTimestamp(now() - INTERVAL 6 HOUR) * 1000
		  AND tag_type = '%s'
		GROUP BY tag_key, tag_type, tag_data_type
		HAVING uniq(string_value) > %d OR uniq(number_value) > %d
		SETTINGS max_threads = 2`,
		distributedTagAttrsV2Table,
		utils.TagTypeBody,
		w.limits.MaxStringDistinctValues,
		w.limits.MaxStringDistinctValues,
	)
	type row struct {
		TagKey      string `ch:"tag_key"`
		TagType     string `ch:"tag_type"`
		TagDataType string `ch:"tag_data_type"`
	}
	var rows []row
	if err := w.conn.Select(ctx, &rows, query); err != nil {
		w.logger.Error("failed to fetch json skip keys", zap.Error(err))
		return
	}
	newMap := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		key := utils.MakeKeyForAttributeKeys(r.TagKey, utils.TagType(r.TagType), utils.TagDataType(r.TagDataType))
		newMap[key] = struct{}{}
	}
	w.skipKeys.Store(&newMap)
}

// skipStoringInDB returns true when the key is present in the skip map,
// meaning its distinct-value count exceeded the limit at the last DB fetch.
func (w *jsonMetadataWriter) skipStoringInDB(key string) bool {
	m := w.skipKeys.Load()
	_, ok := (*m)[key]
	return ok
}

// skipUVT returns true when the in-memory distinct-value count for
// this key already exceeds the configured limit. Unlike the parent exporter's
// implementation it does NOT unconditionally skip number or bool keys.
func (w *jsonMetadataWriter) skipUVT(key string) bool {
	return w.valueTracker.GetUniqueValueCount(key) > int(w.limits.MaxStringDistinctValues)
}

func (w *jsonMetadataWriter) ProcessMetadata(ctx context.Context, ld plog.Logs) error {
	bodyTypes := &typeSet{}

	vaStmt, err := w.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s", distributedTagAttrsV2Table), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare tag attrs batch: %w", err)
	}
	defer vaStmt.Close()

	va := &valueAccumulator{
		stmt:             vaStmt,
		shouldSkipFromDB: w.skipStoringInDB,
		shouldSkipUVT:    w.skipUVT,
		addToUVT:         w.valueTracker.AddValue,
	}

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			logs := sls.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)
				ts := lr.Timestamp()
				if ts == 0 {
					ts = lr.ObservedTimestamp()
				}

				val := lr.Body()
				if val.Type() != pcommon.ValueTypeMap {
					continue
				}
				if err := w.walk(ctx, val, utils.TagTypeBody, ts.AsTime().UnixMilli(), bodyTypes, va); err != nil {
					w.logger.Error("json walk failed", zap.String("source", string(utils.TagTypeBody)), zap.Error(err))
				}
			}
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.flushTypeSet(gCtx, bodyTypes) })
	g.Go(va.flush)
	return g.Wait()
}

// flushTypeSet writes all path-type records for the body source to distributed_json_path_types.
func (w *jsonMetadataWriter) flushTypeSet(ctx context.Context, ts *typeSet) error {
	sql := fmt.Sprintf("INSERT INTO %s (signal, context, path, type, last_seen) VALUES (?, ?, ?, ?, ?)", distributedPathTypesTable)
	stmt, err := w.conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare path types batch: %w", err)
	}
	defer stmt.Close()

	if err := appendTypeSet(ts, pipeline.SignalLogs.String(), string(utils.TagTypeBody), stmt, time.Now().UnixNano()); err != nil {
		return fmt.Errorf("failed to append to path types batch: %w", err)
	}
	return stmt.Send()
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
		// We intentionally skip recording value suggestions for message field, due to high cardinality.
		return nil
	}

	// Depth guard: do not descend into containers beyond the configured limit.
	if level > *w.cfg.MaxDepthTraverse &&
		(val.Type() == pcommon.ValueTypeMap || val.Type() == pcommon.ValueTypeSlice) {
		return nil
	}

	switch val.Type() {
	case pcommon.ValueTypeMap:
		return w.walkMap(ctx, prefix, val, level+1, tagType, unixMilli, ts, va)
	case pcommon.ValueTypeSlice:
		return w.walkSlice(ctx, prefix, val, level, tagType, unixMilli, ts, va)
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
		ts.record(prefix, maskString)
	case pcommon.ValueTypeBool:
		ts.record(prefix, maskBool)
	case pcommon.ValueTypeDouble:
		ts.record(prefix, maskFloat)
	case pcommon.ValueTypeInt:
		ts.record(prefix, maskInt)
	default:
		return fmt.Errorf("unknown value type at path %q: %v", prefix, val.Type())
	}

	return va.record(prefix, val, tagType, unixMilli)
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
	if m.Len() > *w.cfg.MaxKeysAtLevel {
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
	if s.Len() == 0 || s.Len() > *w.cfg.MaxArrayElementsAllowed {
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
			w.logger.Debug("nested arrays not supported", zap.String("path", prefix))
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
