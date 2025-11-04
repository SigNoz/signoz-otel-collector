package jsontypeexporter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
	"github.com/SigNoz/signoz-otel-collector/utils"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	DistributedPathTypesTableName = "signoz_logs.distributed_path_types"
)

type jsonTypeExporter struct {
	config  *Config
	logger  *zap.Logger
	limiter chan struct{}
	conn    clickhouse.Conn
	// this cache doesn't contains full paths, only keys from different levels
	// it is used to avoid checking if a key is high cardinality or not for every log record
	keyCache *lru.Cache[string, struct{}]
}

type TypeSet struct {
	types sync.Map // map[string]*utils.ConcurrentSet[string]
}

func (t *TypeSet) Insert(path string, mask uint16) {
	actual, _ := t.types.LoadOrStore(path, utils.WithCapacityConcurrentSet[string](3))
	cs := actual.(*utils.ConcurrentSet[string])
	// expand mask to strings
	if mask&maskString != 0 {
		cs.Insert(StringType)
	}
	if mask&maskInt != 0 {
		cs.Insert(IntType)
	}
	if mask&maskFloat != 0 {
		cs.Insert(Float64Type)
	}
	if mask&maskBool != 0 {
		cs.Insert(BooleanType)
	}
	if mask&maskArrayString != 0 {
		cs.Insert(ArrayString)
	}
	if mask&maskArrayInt != 0 {
		cs.Insert(ArrayInt)
	}
	if mask&maskArrayFloat != 0 {
		cs.Insert(ArrayFloat64)
	}
	if mask&maskArrayBool != 0 {
		cs.Insert(ArrayBoolean)
	}
	if mask&maskArrayJSON != 0 {
		cs.Insert(ArrayJSON)
	}
	if mask&maskArrayDynamic != 0 {
		cs.Insert(ArrayDynamic)
	}
}

func newExporter(cfg Config, set exporter.Settings) (*jsonTypeExporter, error) {
	// Initialize ClickHouse connection
	connOptions, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ClickHouse DSN: %w", err)
	}

	conn, err := clickhouse.Open(connOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	keyCache, err := lru.New[string, struct{}](100000)
	if err != nil {
		return nil, fmt.Errorf("failed to create key cache: %w", err)
	}

	return &jsonTypeExporter{
		config:   &cfg,
		logger:   set.Logger,
		limiter:  make(chan struct{}, utils.Concurrency()),
		conn:     conn,
		keyCache: keyCache,
	}, nil
}

func (e *jsonTypeExporter) start(ctx context.Context, host component.Host) error {
	e.logger.Info("JSON Type exporter started")
	return nil
}

func (e *jsonTypeExporter) shutdown(ctx context.Context) error {
	e.logger.Info("JSON Type exporter shutdown")
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

func (e *jsonTypeExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	group, groupCtx := errgroup.WithContext(ctx)

	// per-batch type registry with bitmask aggregation
	types := TypeSet{
		types: sync.Map{},
	}

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				// skip if body is not a map
				body := lr.Body()
				if body.Type() != pcommon.ValueTypeMap {
					continue
				}
				// analyze body using pcommon.Value directly
				e.limiter <- struct{}{}
				group.Go(func() error {
					defer func() {
						<-e.limiter
					}()

					return e.analyzePValue(groupCtx, "", false, body, &types, 0)
				})
			}
		}
	}

	// wait for the group execution
	err := group.Wait()
	if err != nil {
		e.logger.Error("Failed to analyze log records", zap.Error(err))
		return err
	}

	// Persist collected types to database
	if err := e.persistTypes(ctx, &types); err != nil {
		e.logger.Error("Failed to persist types to database", zap.Error(err))
		return err
	}

	return nil
}

// bitmasks for compact aggregation
const (
	maskString       uint16 = 1 << 0
	maskInt          uint16 = 1 << 1
	maskFloat        uint16 = 1 << 2
	maskBool         uint16 = 1 << 3
	maskArrayDynamic uint16 = 1 << 4
	maskArrayBool    uint16 = 1 << 5
	maskArrayFloat   uint16 = 1 << 6
	maskArrayInt     uint16 = 1 << 7
	maskArrayString  uint16 = 1 << 8
	maskArrayJSON    uint16 = 1 << 9
)

// api.parameters.list.search -> maps are flattened
//
// api.routes:kubernetes.container_name -> : is used as nestedness indicator in Arrays
//
// analyzePValue walks OTel pcommon.Value without converting to Go maps/slices, minimizing allocations.
func (e *jsonTypeExporter) analyzePValue(ctx context.Context, prefix string, inArray bool, val pcommon.Value, typeSet *TypeSet, level int) error {
	// skip if level is greater than the allowed limit + 1 (for the primary type analysis at last level)
	if level > e.config.MaxDepthTraverse+1 {
		return nil
	}

	switch val.Type() {
	case pcommon.ValueTypeMap:
		m := val.Map()
		m.Range(func(key string, value pcommon.Value) bool {
			select {
			case <-ctx.Done():
				return false
			default:
			}

			contains := e.keyCache.Contains(key)
			// if high cardinality, skip the key
			if !contains && keycheck.IsCardinal(key) {
				return true
			}
			if !contains {
				e.keyCache.Add(key, struct{}{}) // add key to cache to avoid checking it again for next log records
			}

			path := prefix + "." + key
			if prefix == "" {
				path = key
			} else if inArray {
				path = prefix + key
			}
			if err := e.analyzePValue(ctx, path, false, value, typeSet, level+1); err != nil {
				return false
			}

			return true
		})
		return nil
	case pcommon.ValueTypeSlice:
		s := val.Slice()
		var prev uint16
		mixed := false
		for i := 0; i < s.Len(); i++ {
			el := s.At(i)
			var cur uint16
			switch el.Type() {
			case pcommon.ValueTypeMap:
				// analyze first object deeply for path discovery
				if err := e.analyzePValue(ctx, prefix+":", true, el, typeSet, level+1); err != nil {
					return err
				}
				cur = maskArrayJSON
			case pcommon.ValueTypeSlice:
				return fmt.Errorf("arrays inside arrays are not supported! found at path: %s", prefix)
			case pcommon.ValueTypeStr:
				cur = maskArrayString
			case pcommon.ValueTypeBool:
				cur = maskArrayBool
			case pcommon.ValueTypeDouble:
				cur = maskArrayFloat
			case pcommon.ValueTypeInt:
				cur = maskArrayInt
			default:
				return fmt.Errorf("unknown element type in array at path: %s", prefix)
			}
			if i > 0 && cur != prev {
				mixed = true
				break
			}
			prev = cur
		}
		if mixed {
			typeSet.Insert(prefix, maskArrayDynamic)
		} else if prev != 0 {
			typeSet.Insert(prefix, prev)
		}
		return nil
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
		typeSet.Insert(prefix, maskString)
		return nil
	case pcommon.ValueTypeBool:
		typeSet.Insert(prefix, maskBool)
		return nil
	case pcommon.ValueTypeDouble:
		typeSet.Insert(prefix, maskFloat)
		return nil
	case pcommon.ValueTypeInt:
		typeSet.Insert(prefix, maskInt)
		return nil
	default:
		return fmt.Errorf("unknown type at path: %s", prefix)
	}
}

// persistTypes writes the collected types to the ClickHouse database
func (e *jsonTypeExporter) persistTypes(ctx context.Context, typeSet *TypeSet) error {
	// Prepare the SQL statement
	sql := fmt.Sprintf("INSERT INTO %s (path, type, last_seen) VALUES (?, ?, ?)", DistributedPathTypesTableName)

	statement, err := e.conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}
	defer statement.Close()

	now := time.Now().UnixNano()
	insertedCount := 0

	// Iterate through all collected types and insert them
	typeSet.types.Range(func(key, value interface{}) bool {
		path := key.(string)
		typeSet := value.(*utils.ConcurrentSet[string])

		// Get all types for this path
		types := typeSet.Keys()
		for _, typeStr := range types {
			err := statement.Append(path, typeStr, now)
			if err != nil {
				e.logger.Error("Failed to append type to batch",
					zap.String("path", path),
					zap.String("type", typeStr),
					zap.Error(err))
				return false
			}
			insertedCount++
		}
		return true
	})

	if insertedCount == 0 {
		e.logger.Debug("No types to persist")
		return nil
	}

	// Send the batch to the database
	if err := statement.Send(); err != nil {
		return fmt.Errorf("failed to send batch to database: %w", err)
	}

	e.logger.Debug("Successfully persisted types to database",
		zap.Int("count", insertedCount),
		zap.String("table", DistributedPathTypesTableName))

	return nil
}
