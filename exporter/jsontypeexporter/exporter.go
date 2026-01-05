package jsontypeexporter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/constants"
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
	distributedPathTypesTableName = constants.SignozMetadataDB + "." + constants.DistributedPathTypesTable
	defaultKeyCacheSize           = 10_000
	ArraySeparator                = "[]."
	ArraySuffix                   = "[]"
)

type jsonTypeExporter struct {
	config  *Config
	logger  *zap.Logger
	limiter chan struct{}
	conn    clickhouse.Conn
	// this cache doesn't contains full paths, only keys from different levels
	// it is used to avoid checking if a key is high cardinality or not for every log record
	keyCache  *lru.Cache[string, struct{}]
	closeChan chan struct{}
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

	keyCache, err := lru.New[string, struct{}](defaultKeyCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create key cache: %w", err)
	}

	return &jsonTypeExporter{
		config:    &cfg,
		logger:    set.Logger,
		limiter:   make(chan struct{}, utils.Concurrency()),
		conn:      conn,
		keyCache:  keyCache,
		closeChan: make(chan struct{}),
	}, nil
}

func (e *jsonTypeExporter) Start(ctx context.Context, host component.Host) error {
	e.logger.Info("JSON Type exporter started")
	return nil
}

func (e *jsonTypeExporter) Shutdown(ctx context.Context) error {
	close(e.closeChan)
	e.logger.Info("JSON Type exporter shutdown")
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}

func (e *jsonTypeExporter) processLogs(ctx context.Context, ld plog.Logs) error {
	select {
	case <-e.closeChan:
		return errors.New("shutdown has been called")
	default:
	}

	group, groupCtx := errgroup.WithContext(ctx)

	// per-batch type registry with bitmask aggregation
	types := TypeSet{
		types: sync.Map{},
	}

logIteration:
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
				select {
				case <-groupCtx.Done():
					break logIteration // jump immidiately to group.Wait()
				case e.limiter <- struct{}{}:
				}
				group.Go(func() error {
					defer func() {
						<-e.limiter
					}()

					// analyze body using pcommon.Value directly
					return e.analyzePValue(groupCtx, body, &types)
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

func (e *jsonTypeExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	if err := e.processLogs(ctx, ld); err != nil {
		if e.config.FailOnError {
			return err
		}
		e.logger.Error("Failed to process logs", zap.Error(err))
	}

	return nil
}

// api.parameters.list.search -> maps are flattened
//
// api.routes[].kubernetes.container_name -> []. is used as nestedness indicator in Arrays
//
// analyzePValue walks OTel pcommon.Value without converting to Go maps/slices, minimizing allocations.
func (e *jsonTypeExporter) analyzePValue(ctx context.Context, val pcommon.Value, typeSet *TypeSet) error {
	generatePath := func(prefix string, key string) string {
		if prefix == "" {
			return key
		}

		if keycheck.IsBacktickRequired(key) {
			key = "`" + key + "`"
		}

		return prefix + "." + key
	}

	var closure func(ctx context.Context, prefix string, val pcommon.Value, typeSet *TypeSet, level int) error
	closure = func(ctx context.Context, prefix string, val pcommon.Value, typeSet *TypeSet, level int) error {
		// skip if level is greater than the allowed limit
		shouldSkipNesting := level >= *e.config.MaxDepthTraverse
		// If current value is a container (map/array) and we've exceeded depth, do not descend further
		// else just analyze the value type
		if shouldSkipNesting && (val.Type() == pcommon.ValueTypeMap || val.Type() == pcommon.ValueTypeSlice) {
			return nil
		}

		switch val.Type() {
		case pcommon.ValueTypeMap:
			m := val.Map()
			// skip if map contains too many keys
			if m.Len() > defaultMaxKeysAtLevel {
				return nil
			}

			var iterErr error
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

				if err := closure(ctx, generatePath(prefix, key), value, typeSet, level+1); err != nil {
					iterErr = err
					return false
				}

				return true
			})
			if iterErr != nil {
				return iterErr
			}

			return nil
		case pcommon.ValueTypeSlice:
			s := val.Slice()
			// skip this slice since it contains too many elements
			if s.Len() > *e.config.MaxArrayElementsAllowed {
				return nil
			}

			var prev uint16
			mixed := false
			for i := 0; i < s.Len(); i++ {
				el := s.At(i)
				var cur uint16
				switch el.Type() {
				case pcommon.ValueTypeMap:
					// When traversing into array element objects, do not increase depth level.
					// This ensures fields like `array[].a` are discoverable at the same depth budget
					// as their parent array, aligning depth semantics with expected behavior.
					if err := closure(ctx, prefix+ArraySuffix, el, typeSet, level); err != nil {
						return err
					}
					cur = maskArrayJSON
				case pcommon.ValueTypeSlice:
					// pass via the array element
					e.logger.Error("arrays inside arrays are not supported!", zap.String("path", prefix))
				case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
					cur = maskArrayString
				case pcommon.ValueTypeBool:
					cur = maskArrayBool
				case pcommon.ValueTypeDouble:
					cur = maskArrayFloat
				case pcommon.ValueTypeInt:
					cur = maskArrayInt
				default:
					// move to next element
					e.logger.Error("unknown element type in array at path", zap.String("path", prefix), zap.Any("type", el.Type()))
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

	return closure(ctx, "", val, typeSet, 0)
}

// persistTypes writes the collected types to the ClickHouse database
func (e *jsonTypeExporter) persistTypes(ctx context.Context, typeSet *TypeSet) error {
	// Prepare the SQL statement
	sql := fmt.Sprintf("INSERT INTO %s (path, type, last_seen) VALUES (?, ?, ?)", distributedPathTypesTableName)

	statement, err := e.conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}
	defer statement.Close()

	now := time.Now().UnixNano()
	insertedCount := 0

	var iterErr error
	// Iterate through all collected types and insert them
	typeSet.types.Range(func(key, value interface{}) bool {
		path := key.(string)
		typeSet := value.(*utils.ConcurrentSet[string])

		// Get all types for this path
		types := typeSet.Keys()
		for _, typeStr := range types {
			iterErr = statement.Append(path, typeStr, now)
			if iterErr != nil {
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
	if iterErr != nil {
		return fmt.Errorf("failed to append types to batch: %w", iterErr)
	}

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
		zap.String("table", distributedPathTypesTableName))

	return nil
}
