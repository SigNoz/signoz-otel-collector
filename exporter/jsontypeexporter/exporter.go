package jsontypeexporter

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/utils/set"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type jsonTypeExporter struct {
	config  *Config
	logger  *zap.Logger
	limiter chan struct{}
	conn    clickhouse.Conn
}

func newExporter(cfg Config, set exporter.Settings) (*jsonTypeExporter, error) {
	concurrency := int(math.Max(1, float64(runtime.GOMAXPROCS(0))))

	// Initialize ClickHouse connection
	connOptions, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ClickHouse DSN: %w", err)
	}

	conn, err := clickhouse.Open(connOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	return &jsonTypeExporter{
		config:  &cfg,
		logger:  set.Logger,
		limiter: make(chan struct{}, concurrency),
		conn:    conn,
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

	// per-batch type registry
	types := sync.Map{} // map[string]*set.ConcurrentSet[string]
	// setType records an observed type for a given JSON path in a concurrent set.

	setType := func(path string, typ string) {
		actual, _ := types.LoadOrStore(path, set.New[string]())
		cs := actual.(*set.ConcurrentSet[string])
		cs.Insert(typ)
	}

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				// analyze body as well
				if body := lr.Body().AsRaw(); body != nil {
					if body, ok := body.(map[string]any); ok {
						// process async
						e.limiter <- struct{}{}
						group.Go(func() error {
							defer func() {
								<-e.limiter
							}()
							if err := e.analyzeTypes(groupCtx, "", body, setType); err != nil {
								return err
							}

							return nil // not returning error to avoid cancelling groupCtx
						})

					}
				}
			}
		}
	}

	// wait for the group execution
	_ = group.Wait()

	// Persist collected types to database
	if err := e.persistTypes(ctx, &types); err != nil {
		e.logger.Error("Failed to persist types to database", zap.Error(err))
		return err
	}

	return nil
}

// api.parameters.list.search -> maps are flattened
//
// api.routes:kubernetes.container_name -> : is used as nestedness indicator in Arrays
func (e *jsonTypeExporter) analyzeTypes(ctx context.Context, prefix string, json map[string]any, setType func(path string, typ string)) error {
	for key, value := range json {
		select {
		case <-ctx.Done():
			return nil
		default:
			path := prefix + "." + key
			if prefix == "" {
				path = key
			} else if strings.HasSuffix(prefix, ":") { // -> at objects inside arrays
				path = prefix + key
			}

			switch kind := reflect.TypeOf(value).Kind(); kind {
			case reflect.Map:
				if err := e.analyzeTypes(ctx, path, value.(map[string]any), setType); err != nil {
					return err
				}
			case reflect.Slice:
				var kinds []reflect.Kind
				for _, v := range value.([]any) {
					switch reflect.TypeOf(v).Kind() {
					case reflect.Map:
						path = path + ":" // -> diving into objects inside arrays
						if err := e.analyzeTypes(ctx, path, v.(map[string]any), setType); err != nil {
							return err
						}
						kinds = append(kinds, reflect.Map)
					case reflect.Slice:
						return fmt.Errorf("arrays inside arrays are not supported! found at path: %s", path)
					case reflect.String:
						kinds = append(kinds, reflect.String)
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
						kinds = append(kinds, reflect.Int64)
					case reflect.Float32, reflect.Float64:
						kinds = append(kinds, reflect.Float64)
					case reflect.Bool:
						kinds = append(kinds, reflect.Bool)
					default:
						return fmt.Errorf("unknown kind: %v", reflect.TypeOf(v).Kind())
					}
				}
				if len(kinds) > 1 {
					setType(path, ArrayDynamic)
					for _, k := range kinds {
						setType(path, kindToArrayTypes[k])
					}
				} else {
					setType(path, kindToArrayTypes[kinds[0]])
				}
			default:
				typ, found := kindToTypes[kind]
				if !found {
					return fmt.Errorf("unknown kind: %v", kind)
				}
				setType(path, typ)
			}
		}
	}

	return nil
}

// persistTypes writes the collected types to the ClickHouse database
func (e *jsonTypeExporter) persistTypes(ctx context.Context, types *sync.Map) error {
	// Prepare the SQL statement
	tableName := "signoz_logs.distributed_path_types"
	sql := fmt.Sprintf("INSERT INTO %s (path, type, last_seen) VALUES (?, ?, ?)", tableName)

	statement, err := e.conn.PrepareBatch(ctx, sql, driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}
	defer statement.Close()

	now := time.Now().UnixNano()
	insertedCount := 0

	// Iterate through all collected types and insert them
	types.Range(func(key, value interface{}) bool {
		path := key.(string)
		typeSet := value.(*set.ConcurrentSet[string])

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
		zap.String("table", tableName))

	return nil
}
