// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousemetricsexporter

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
	"go.opencensus.io/stats/view"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/prometheus/prometheus/prompb"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/base"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const maxBatchByteSize = 314572800

// ClickHouseExporter converts OTLP metrics to Prometheus remote write TimeSeries and
// writes to ClickHouse
type ClickHouseExporter struct {
	wg               *sync.WaitGroup
	closeChan        chan struct{}
	concurrency      int
	settings         component.TelemetrySettings
	ch               base.Storage
	usageCollector   *usage.UsageCollector
	metricNameToMeta map[string]base.MetricMeta
	mux              *sync.Mutex
	logger           *zap.Logger
}

// NewClickHouseExporter initializes a new ClickHouseExporter instance
func NewClickHouseExporter(cfg *Config, set exporter.CreateSettings) (*ClickHouseExporter, error) {
	params := &ClickHouseParams{
		DSN:             cfg.Endpoint,
		MaxOpenConns:    75,
		WatcherInterval: cfg.WatcherInterval,
		WriteTSToV4:     cfg.WriteTSToV4,
		Cluster:         cfg.Cluster,
	}
	ch, err := NewClickHouse(params)
	if err != nil {
		log.Fatalf("Error creating clickhouse client: %v", err)
	}

	collector := usage.NewUsageCollector(ch.GetDBConn().(clickhouse.Conn),
		usage.Options{
			ReportingInterval: usage.DefaultCollectionInterval,
		},
		"signoz_metrics",
		UsageExporter,
	)
	if err != nil {
		log.Fatalf("Error creating usage collector for metrics: %v", err)
	}

	collector.Start()

	if err := view.Register(MetricPointsCountView, MetricPointsBytesView); err != nil {
		return nil, err
	}

	return &ClickHouseExporter{
		wg:               new(sync.WaitGroup),
		closeChan:        make(chan struct{}),
		concurrency:      cfg.QueueSettings.NumConsumers,
		settings:         set.TelemetrySettings,
		ch:               ch,
		usageCollector:   collector,
		metricNameToMeta: make(map[string]base.MetricMeta),
		mux:              new(sync.Mutex),
		logger:           set.Logger,
	}, nil
}

func (che *ClickHouseExporter) Start(_ context.Context, host component.Host) (err error) {
	return nil
}

// Shutdown stops the exporter from accepting incoming calls(and return error), and wait for current export operations
// to finish before returning
func (che *ClickHouseExporter) Shutdown(context.Context) error {
	// shutdown usage reporting.
	if che.usageCollector != nil {
		che.usageCollector.Stop()
	}

	close(che.closeChan)
	che.wg.Wait()
	return nil
}

// PushMetrics converts metrics to Prometheus remote write TimeSeries and write to ClickHouse. It maintain a map of
// TimeSeries, validates and handles each individual metric, adding the converted TimeSeries to the map, and finally
// exports the map.
func (che *ClickHouseExporter) PushMetrics(ctx context.Context, md pmetric.Metrics) error {
	che.wg.Add(1)
	defer che.wg.Done()

	select {
	case <-che.closeChan:
		return errors.New("shutdown has been called")
	default:
		tsMap := map[string]*prompb.TimeSeries{}
		dropped := 0
		var errs error
		resourceMetricsSlice := md.ResourceMetrics()
		for i := 0; i < resourceMetricsSlice.Len(); i++ {
			resourceMetrics := resourceMetricsSlice.At(i)
			resource := resourceMetrics.Resource()
			scopeMetricsSlice := resourceMetrics.ScopeMetrics()
			for j := 0; j < scopeMetricsSlice.Len(); j++ {
				scopeMetrics := scopeMetricsSlice.At(j)
				metricSlice := scopeMetrics.Metrics()

				// TODO: decide if scope information should be exported as labels
				for k := 0; k < metricSlice.Len(); k++ {
					metric := metricSlice.At(k)
					var temporality pmetric.AggregationTemporality

					metricType := metric.Type()

					switch metricType {
					case pmetric.MetricTypeGauge:
						temporality = pmetric.AggregationTemporalityUnspecified
					case pmetric.MetricTypeSum:
						temporality = metric.Sum().AggregationTemporality()
					case pmetric.MetricTypeHistogram:
						temporality = metric.Histogram().AggregationTemporality()
					case pmetric.MetricTypeSummary:
						temporality = pmetric.AggregationTemporalityUnspecified
					default:
					}
					metricName := getPromMetricName(metric)
					meta := base.MetricMeta{
						Name:        metricName,
						Temporality: temporality,
						Description: metric.Description(),
						Unit:        metric.Unit(),
						Typ:         metricType,
					}
					if metricType == pmetric.MetricTypeSum {
						meta.IsMonotonic = metric.Sum().IsMonotonic()
					}
					che.metricNameToMeta[metricName] = meta

					if metricType == pmetric.MetricTypeHistogram || metricType == pmetric.MetricTypeSummary {
						che.metricNameToMeta[metricName+bucketStr] = meta
						che.metricNameToMeta[metricName+countStr] = base.MetricMeta{
							Name:        metricName,
							Temporality: temporality,
							Description: metric.Description(),
							Unit:        metric.Unit(),
							Typ:         pmetric.MetricTypeSum,
							IsMonotonic: temporality == pmetric.AggregationTemporalityCumulative,
						}
						che.metricNameToMeta[metricName+sumStr] = base.MetricMeta{
							Name:        metricName,
							Temporality: temporality,
							Description: metric.Description(),
							Unit:        metric.Unit(),
							Typ:         pmetric.MetricTypeSum,
							IsMonotonic: temporality == pmetric.AggregationTemporalityCumulative,
						}
					}

					// handle individual metric based on type
					switch metricType {
					case pmetric.MetricTypeGauge:
						dataPoints := metric.Gauge().DataPoints()
						if err := che.addNumberDataPointSlice(dataPoints, tsMap, resource, metric); err != nil {
							dropped++
							errs = multierr.Append(errs, err)
						}
					case pmetric.MetricTypeSum:
						dataPoints := metric.Sum().DataPoints()
						if err := che.addNumberDataPointSlice(dataPoints, tsMap, resource, metric); err != nil {
							dropped++
							errs = multierr.Append(errs, err)
						}
					case pmetric.MetricTypeHistogram:
						dataPoints := metric.Histogram().DataPoints()
						if dataPoints.Len() == 0 {
							dropped++
							che.logger.Warn("Dropped histogram metric with no data points", zap.String("name", metric.Name()))
						}
						for x := 0; x < dataPoints.Len(); x++ {
							addSingleHistogramDataPoint(dataPoints.At(x), resource, metric, tsMap)
						}
					case pmetric.MetricTypeSummary:
						dataPoints := metric.Summary().DataPoints()
						if dataPoints.Len() == 0 {
							dropped++
							che.logger.Warn("Dropped summary metric with no data points", zap.String("name", metric.Name()))
						}
						for x := 0; x < dataPoints.Len(); x++ {
							addSingleSummaryDataPoint(dataPoints.At(x), resource, metric, tsMap)
						}
					case pmetric.MetricTypeExponentialHistogram:
						// TODO(srikanthccv): implement
					default:
						dropped++
						name := metric.Name()
						typ := metric.Type().String()
						che.logger.Warn("Unsupported metric type", zap.String("name", name), zap.String("type", typ))
					}
				}
			}
		}

		if exportErrors := che.export(ctx, tsMap); len(exportErrors) != 0 {
			dropped = md.MetricCount()
			errs = multierr.Append(errs, multierr.Combine(exportErrors...))
		}

		if dropped != 0 {
			return errs
		}

		return nil
	}
}

func validateAndSanitizeExternalLabels(externalLabels map[string]string) (map[string]string, error) {
	sanitizedLabels := make(map[string]string)
	for key, value := range externalLabels {
		if key == "" || value == "" {
			return nil, fmt.Errorf("prometheus remote write: external labels configuration contains an empty key or value")
		}

		// Sanitize label keys to meet Prometheus Requirements
		if len(key) > 2 && key[:2] == "__" {
			key = "__" + sanitize(key[2:])
		} else {
			key = sanitize(key)
		}
		sanitizedLabels[key] = value
	}

	return sanitizedLabels, nil
}

func (che *ClickHouseExporter) addNumberDataPointSlice(dataPoints pmetric.NumberDataPointSlice, tsMap map[string]*prompb.TimeSeries, resource pcommon.Resource, metric pmetric.Metric) error {
	for x := 0; x < dataPoints.Len(); x++ {
		addSingleNumberDataPoint(dataPoints.At(x), resource, metric, tsMap)
	}
	return nil
}

// export sends a Snappy-compressed WriteRequest containing TimeSeries to a remote write endpoint in order
func (che *ClickHouseExporter) export(ctx context.Context, tsMap map[string]*prompb.TimeSeries) []error {
	che.mux.Lock()
	// make a copy of metricNameToMeta
	metricNameToMeta := make(map[string]base.MetricMeta)
	for k, v := range che.metricNameToMeta {
		metricNameToMeta[k] = v
	}
	che.mux.Unlock()
	var errs []error
	// Calls the helper function to convert and batch the TsMap to the desired format
	requests, err := batchTimeSeries(tsMap, maxBatchByteSize)
	if err != nil {
		errs = append(errs, consumererror.NewPermanent(err))
		return errs
	}

	input := make(chan *prompb.WriteRequest, len(requests))
	for _, request := range requests {
		input <- request
	}
	close(input)

	var mu sync.Mutex
	var wg sync.WaitGroup

	concurrencyLimit := int(math.Min(float64(che.concurrency), float64(len(requests))))
	wg.Add(concurrencyLimit) // used to wait for workers to be finished

	// Run concurrencyLimit of workers until there
	// is no more requests to execute in the input channel.
	for i := 0; i < concurrencyLimit; i++ {
		go func() {
			defer wg.Done()

			for request := range input {
				err := che.ch.Write(ctx, request, metricNameToMeta)
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	return errs
}
