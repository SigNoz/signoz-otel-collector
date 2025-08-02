package signozclickhousemeter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	samplesSQLTmpl = "INSERT INTO %s.%s (temporality, metric_name, description, unit, type, is_monotonic, labels, fingerprint, unix_milli, value) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
)

const NanDetectedErrMsg = "NaN detected in data point, skipping entire data point"

type clickhouseMeterExporter struct {
	cfg        *Config
	logger     *zap.Logger
	conn       clickhouse.Conn
	wg         sync.WaitGroup
	samplesSQL string
	closeChan  chan struct{}
}

// sample represents a single metric sample directly mapped to the table `samples` schema
type sample struct {
	temporality pmetric.AggregationTemporality
	metricName  string
	description string
	unit        string
	typ         pmetric.MetricType
	isMonotonic bool
	labels      string
	fingerprint uint64
	unixMilli   int64
	value       float64
}

func NewClickHouseExporter(logger *zap.Logger, config component.Config) (*clickhouseMeterExporter, error) {
	cfg := config.(*Config)

	connOptions, err := clickhouse.ParseDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(connOptions)
	if err != nil {
		return nil, err
	}

	return &clickhouseMeterExporter{
		cfg:        cfg,
		logger:     logger,
		conn:       conn,
		samplesSQL: fmt.Sprintf(samplesSQLTmpl, cfg.Database, cfg.SamplesTable),
		closeChan:  make(chan struct{}),
	}, nil
}

func (c *clickhouseMeterExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *clickhouseMeterExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (c *clickhouseMeterExporter) Shutdown(ctx context.Context) error {
	close(c.closeChan)
	c.wg.Wait()
	return c.conn.Close()
}

// processSum processes sum metrics
func (c *clickhouseMeterExporter) processSum(batch *batch, metric pmetric.Metric, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) {
	name := metric.Name()
	desc := metric.Description()
	unit := metric.Unit()
	typ := metric.Type()
	// sum metrics have a temporality
	temporality := metric.Sum().AggregationTemporality()
	isMonotonic := metric.Sum().IsMonotonic()

	resourceFingerprintMap := resourceFingerprint.AttributesAsMap()
	scopeFingerprintMap := scopeFingerprint.AttributesAsMap()

	for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
		dp := metric.Sum().DataPoints().At(i)
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		if math.IsNaN(value) {
			c.logger.Warn(NanDetectedErrMsg, zap.String("metric_name", name))
			continue
		}
		unixMilli := dp.Timestamp().AsTime().UnixMilli()
		fingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, scopeFingerprint.Hash(), dp.Attributes(), map[string]string{
			"__temporality__": temporality.String(),
		})
		fingerprintMap := fingerprint.AttributesAsMap()
		batch.addSample(&sample{
			temporality: temporality,
			metricName:  name,
			fingerprint: fingerprint.HashWithName(name),
			unixMilli:   unixMilli,
			value:       value,
			description: desc,
			unit:        unit,
			typ:         typ,
			isMonotonic: isMonotonic,
			labels:      pkgfingerprint.NewLabelsAsJSONString(name, fingerprintMap, scopeFingerprintMap, resourceFingerprintMap),
		})
	}
}

func (c *clickhouseMeterExporter) prepareBatch(_ context.Context, md pmetric.Metrics) *batch {
	batch := newBatch()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceFingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.ResourceFingerprintType, pkgfingerprint.InitialOffset, rm.Resource().Attributes(), map[string]string{})
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeFingerprint := pkgfingerprint.NewFingerprint(pkgfingerprint.ScopeFingerprintType, resourceFingerprint.Hash(), sm.Scope().Attributes(), map[string]string{
				"__scope.name__":       sm.Scope().Name(),
				"__scope.version__":    sm.Scope().Version(),
				"__scope.schema_url__": sm.SchemaUrl(),
			})

			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					c.processSum(batch, metric, resourceFingerprint, scopeFingerprint)
				default:
					c.logger.Warn("unknown metric type", zap.String("metric_name", metric.Name()), zap.String("metric_type", metric.Type().String()))
				}
			}
		}
	}
	return batch
}

func (c *clickhouseMeterExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	c.wg.Add(1)
	defer c.wg.Done()
	select {
	case <-c.closeChan:
		return errors.New("shutdown has been called")
	default:
		return c.writeBatch(ctx, c.prepareBatch(ctx, md))
	}
}

func (c *clickhouseMeterExporter) writeBatch(ctx context.Context, batch *batch) error {
	if len(batch.samples) == 0 {
		return nil
	}
	statement, err := c.conn.PrepareBatch(ctx, c.samplesSQL, driver.WithReleaseConnection())
	if err != nil {
		return err
	}
	defer statement.Close()

	for _, sample := range batch.samples {
		roundedUnixMilli := sample.unixMilli / 3600000 * 3600000
		err = statement.Append(
			sample.temporality.String(),
			sample.metricName,
			sample.description,
			sample.unit,
			sample.typ.String(),
			sample.isMonotonic,
			sample.labels,
			sample.fingerprint,
			roundedUnixMilli,
			sample.value,
		)
		if err != nil {
			return err
		}
	}

	return statement.Send()
}
