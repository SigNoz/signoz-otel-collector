// Copyright 2017, 2018 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package clickhousemetricsexporter provides writer for ClickHouse storage.
package clickhousemetricsexporter

import (
	"context"
	"fmt"
	"math"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	chproto "github.com/ClickHouse/ch-go/proto"
	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/base"
	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/utils/timeseries"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/prometheus/prometheus/prompb"
)

const (
	namespace                        = "promhouse"
	subsystem                        = "clickhouse"
	nameLabel                        = "__name__"
	DISTRIBUTED_TIME_SERIES_TABLE    = "distributed_time_series_v2"
	DISTRIBUTED_TIME_SERIES_TABLE_V3 = "distributed_time_series_v3"
	DISTRIBUTED_TIME_SERIES_TABLE_V4 = "distributed_time_series_v4"
	DISTRIBUTED_SAMPLES_TABLE        = "distributed_samples_v2"
	DISTRIBUTED_SAMPLES_TABLE_V4     = "distributed_samples_v4"
	DISTRIBUTED_EXP_HIST_TABLE       = "distributed_exp_hist"
	TIME_SERIES_TABLE                = "time_series_v2"
	temporalityLabel                 = "__temporality__"
	envLabel                         = "env"
)

// clickHouse implements storage interface for the ClickHouse.
type clickHouse struct {
	conn                 clickhouse.Conn
	l                    *logrus.Entry
	database             string
	maxTimeSeriesInQuery int

	cache *ttlcache.Cache[uint64, bool]

	timeSeriesRW sync.RWMutex
	// Maintains the lookup map for fingerprints that are
	// written to time series table. This map is used to eliminate the
	// unnecessary writes to table for the records that already exist.
	timeSeries      map[uint64]struct{}
	prevShardCount  uint64
	watcherInterval time.Duration
	writeTSToV4     bool

	mWrittenTimeSeries prometheus.Counter

	exporterID uuid.UUID
}

type ClickHouseParams struct {
	DSN                  string
	DropDatabase         bool
	MaxIdleConns         int
	MaxOpenConns         int
	MaxTimeSeriesInQuery int
	WatcherInterval      time.Duration
	WriteTSToV4          bool
	ExporterId           uuid.UUID
}

func NewClickHouse(params *ClickHouseParams) (base.Storage, error) {
	l := logrus.WithField("component", "clickhouse")

	options, err := clickhouse.ParseDSN(params.DSN)

	if err != nil {
		return nil, err
	}

	if options.MaxIdleConns < params.MaxIdleConns {
		options.MaxIdleConns = params.MaxIdleConns
	}
	if options.MaxOpenConns < params.MaxOpenConns {
		options.MaxOpenConns = params.MaxOpenConns
	}
	if options.DialTimeout < 1*time.Minute {
		options.DialTimeout = 1 * time.Minute
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("could not connect to clickhouse: %s", err)
	}

	cache := ttlcache.New[uint64, bool](
		ttlcache.WithTTL[uint64, bool](45*time.Minute),
		ttlcache.WithDisableTouchOnHit[uint64, bool](),
	)

	go cache.Start()

	ch := &clickHouse{
		conn:                 conn,
		l:                    l,
		database:             options.Auth.Database,
		maxTimeSeriesInQuery: params.MaxTimeSeriesInQuery,
		cache:                cache,

		timeSeries: make(map[uint64]struct{}, 8192),

		mWrittenTimeSeries: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "written_time_series",
			Help:      "Number of written time series.",
		}),
		watcherInterval: params.WatcherInterval,
		writeTSToV4:     params.WriteTSToV4,
		exporterID:      params.ExporterId,
	}

	go func() {
		ctx := pprof.WithLabels(context.TODO(), pprof.Labels("component", "clickhouse_reloader"))
		pprof.SetGoroutineLabels(ctx)
		ch.shardCountWatcher(ctx)
	}()

	return ch, nil
}

func (ch *clickHouse) shardCountWatcher(ctx context.Context) {
	ticker := time.NewTicker(ch.watcherInterval)
	defer ticker.Stop()

	q := `SELECT count() FROM system.clusters WHERE cluster='cluster'`
	for {

		err := func() error {
			ch.l.Debug(q)
			row := ch.conn.QueryRow(ctx, q)
			if row.Err() != nil {
				return row.Err()
			}

			var shardCount uint64
			err := row.Scan(&shardCount)
			if err != nil {
				return err
			}

			ch.timeSeriesRW.Lock()
			if ch.prevShardCount != shardCount {
				ch.l.Infof("Shard count changed from %d to %d. Resetting time series map.", ch.prevShardCount, shardCount)
				ch.timeSeries = make(map[uint64]struct{})
			}
			ch.prevShardCount = shardCount
			ch.timeSeriesRW.Unlock()
			return nil
		}()
		if err != nil {
			ch.l.Error(err)
		}

		select {
		case <-ctx.Done():
			ch.l.Warn(ctx.Err())
			return
		case <-ticker.C:
		}
	}
}

func (ch *clickHouse) Describe(c chan<- *prometheus.Desc) {
	ch.mWrittenTimeSeries.Describe(c)
}

func (ch *clickHouse) Collect(c chan<- prometheus.Metric) {
	ch.mWrittenTimeSeries.Collect(c)
}

func (ch *clickHouse) GetDBConn() interface{} {
	return ch.conn
}

func (ch *clickHouse) Write(ctx context.Context, data *prompb.WriteRequest, metricNameToMeta map[string]base.MetricMeta) error {
	// calculate fingerprints, map them to time series
	fingerprints := make([]uint64, len(data.Timeseries))
	timeSeries := make(map[uint64][]*prompb.Label, len(data.Timeseries))
	fingerprintToName := make(map[uint64]map[string]string)

	for i, ts := range data.Timeseries {
		var metricName string
		var env string = "default"
		labelsOverridden := make(map[string]*prompb.Label)
		for _, label := range ts.Labels {
			labelsOverridden[label.Name] = &prompb.Label{
				Name:  label.Name,
				Value: label.Value,
			}
			if label.Name == nameLabel {
				metricName = label.Value
			}
			if label.Name == semconv.AttributeDeploymentEnvironment || label.Name == sanitize(semconv.AttributeDeploymentEnvironment) {
				env = label.Value
			}
		}
		var labels []*prompb.Label
		for _, l := range labelsOverridden {
			labels = append(labels, l)
		}
		// add temporality label
		if metricName != "" {
			if t, ok := metricNameToMeta[metricName]; ok {
				labels = append(labels, &prompb.Label{
					Name:  temporalityLabel,
					Value: t.Temporality.String(),
				})
			}
		}
		timeseries.SortLabels(labels)
		f := timeseries.Fingerprint(labels)
		fingerprints[i] = f
		timeSeries[f] = labels
		if _, ok := fingerprintToName[f]; !ok {
			fingerprintToName[f] = make(map[string]string)
		}
		fingerprintToName[f][nameLabel] = metricName
		fingerprintToName[f][envLabel] = env
	}
	if len(fingerprints) != len(timeSeries) {
		ch.l.Debugf("got %d fingerprints, but only %d of them were unique time series", len(fingerprints), len(timeSeries))
	}

	// find new time series
	newTimeSeries := make(map[uint64][]*prompb.Label)
	ch.timeSeriesRW.Lock()
	for f, m := range timeSeries {
		_, ok := ch.timeSeries[f]
		if !ok {
			ch.timeSeries[f] = struct{}{}
			newTimeSeries[f] = m
		}
	}
	ch.timeSeriesRW.Unlock()

	err := func() error {
		statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s (metric_name, temporality, timestamp_ms, fingerprint, labels, description, unit, type, is_monotonic) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", ch.database, DISTRIBUTED_TIME_SERIES_TABLE), driver.WithReleaseConnection())
		if err != nil {
			return err
		}
		timestamp := model.Now().Time().UnixMilli()
		for fingerprint, labels := range newTimeSeries {
			encodedLabels := string(marshalLabels(labels, make([]byte, 0, 128)))
			meta := metricNameToMeta[fingerprintToName[fingerprint][nameLabel]]
			err = statement.Append(
				fingerprintToName[fingerprint][nameLabel],
				meta.Temporality.String(),
				timestamp,
				fingerprint,
				encodedLabels,
				meta.Description,
				meta.Unit,
				meta.Typ.String(),
				meta.IsMonotonic,
			)
			if err != nil {
				return err
			}
		}

		start := time.Now()
		err = statement.Send()
		ctx, _ = tag.New(ctx,
			tag.Upsert(exporterKey, string(component.DataTypeMetrics.String())),
			tag.Upsert(tableKey, DISTRIBUTED_TIME_SERIES_TABLE),
		)
		stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
		return err
	}()

	if err != nil {
		return err
	}

	metrics := map[string]usage.Metric{}
	err = func() error {
		ctx := context.Background()

		statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", ch.database, DISTRIBUTED_SAMPLES_TABLE), driver.WithReleaseConnection())
		if err != nil {
			return err
		}

		for i, ts := range data.Timeseries {
			fingerprint := fingerprints[i]
			for _, s := range ts.Samples {

				// usage collection checks
				tenant := "default"
				collectUsage := true
				for _, val := range timeSeries[fingerprint] {
					if val.Name == nameLabel && (strings.HasPrefix(val.Value, "signoz_") || strings.HasPrefix(val.Value, "chi_") || strings.HasPrefix(val.Value, "otelcol_")) {
						collectUsage = false
						break
					}
					if val.Name == "tenant" {
						tenant = val.Value
					}
				}

				if collectUsage {
					usage.AddMetric(metrics, tenant, 1, int64(len(s.String())))
				}

				err = statement.Append(
					fingerprintToName[fingerprint][nameLabel],
					fingerprint,
					s.Timestamp,
					s.Value,
				)
				if err != nil {
					return err
				}
			}
		}
		start := time.Now()
		err = statement.Send()
		ctx, _ = tag.New(ctx,
			tag.Upsert(exporterKey, string(component.DataTypeMetrics.String())),
			tag.Upsert(tableKey, DISTRIBUTED_SAMPLES_TABLE),
		)
		stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
		return err
	}()
	if err != nil {
		return err
	}

	for k, v := range metrics {
		stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, ch.exporterID.String())}, ExporterSigNozSentMetricPoints.M(int64(v.Count)), ExporterSigNozSentMetricPointsBytes.M(int64(v.Size)))
	}

	// write to distributed_samples_v4 table
	if ch.writeTSToV4 {
		err = func() error {
			statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, value) VALUES (?, ?, ?, ?, ?, ?)", ch.database, DISTRIBUTED_SAMPLES_TABLE_V4), driver.WithReleaseConnection())
			if err != nil {
				return err
			}

			for i, ts := range data.Timeseries {
				fingerprint := fingerprints[i]
				for _, s := range ts.Samples {
					metricName := fingerprintToName[fingerprint][nameLabel]
					err = statement.Append(
						fingerprintToName[fingerprint][envLabel],
						metricNameToMeta[metricName].Temporality.String(),
						metricName,
						fingerprint,
						s.Timestamp,
						s.Value,
					)
					if err != nil {
						return err
					}
				}
			}

			start := time.Now()
			err = statement.Send()
			ctx, _ = tag.New(ctx,
				tag.Upsert(exporterKey, string(component.DataTypeMetrics.String())),
				tag.Upsert(tableKey, DISTRIBUTED_SAMPLES_TABLE_V4),
			)
			stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
			return err
		}()

		if err != nil {
			return err
		}
	}

	// write to distributed_time_series_v4 table
	if ch.writeTSToV4 {
		err = func() error {
			statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s (env, temporality, metric_name, description, unit, type, is_monotonic, fingerprint, unix_milli, labels) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", ch.database, DISTRIBUTED_TIME_SERIES_TABLE_V4), driver.WithReleaseConnection())
			if err != nil {
				return err
			}
			// timestamp in milliseconds with nearest hour precision
			unixMilli := model.Now().Time().UnixMilli() / 3600000 * 3600000

			for fingerprint, labels := range timeSeries {
				if ch.cache.Get(fingerprint) != nil && ch.cache.Get(fingerprint).Value() {
					continue
				}
				encodedLabels := string(marshalLabels(labels, make([]byte, 0, 128)))
				meta := metricNameToMeta[fingerprintToName[fingerprint][nameLabel]]
				err = statement.Append(
					fingerprintToName[fingerprint][envLabel],
					meta.Temporality.String(),
					fingerprintToName[fingerprint][nameLabel],
					meta.Description,
					meta.Unit,
					meta.Typ.String(),
					meta.IsMonotonic,
					fingerprint,
					unixMilli,
					encodedLabels,
				)
				if err != nil {
					return err
				}
				ch.cache.Set(fingerprint, true, ttlcache.DefaultTTL)
			}

			start := time.Now()
			err = statement.Send()
			ctx, _ = tag.New(ctx,
				tag.Upsert(exporterKey, string(component.DataTypeMetrics.String())),
				tag.Upsert(tableKey, DISTRIBUTED_TIME_SERIES_TABLE_V4),
			)
			stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
			return err
		}()

		if err != nil {
			return err
		}
	}

	n := len(newTimeSeries)
	if n != 0 {
		ch.mWrittenTimeSeries.Add(float64(n))
		ch.l.Debugf("Wrote %d new time series.", n)
	}

	err = func() error {
		statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s (env, temporality, metric_name, fingerprint, unix_milli, count, sum, min, max, sketch) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", ch.database, DISTRIBUTED_EXP_HIST_TABLE), driver.WithReleaseConnection())
		if err != nil {
			return err
		}

		for i, ts := range data.Timeseries {
			fingerprint := fingerprints[i]
			for _, s := range ts.Histograms {

				sum := s.Sum
				var count uint64
				if x, ok := s.Count.(*prompb.Histogram_CountInt); ok {
					count = uint64(x.CountInt)
				} else if x, ok := s.Count.(*prompb.Histogram_CountFloat); ok {
					count = uint64(x.CountFloat)
				}
				min, max := s.PositiveCounts[1], s.PositiveCounts[2]
				gamma := math.Pow(2, math.Pow(2, float64(-s.Schema)))
				positiveOffset := s.PositiveCounts[0]
				negativeOffset := s.NegativeCounts[0]
				var positivebinCounts []float64
				for _, x := range s.PositiveDeltas {
					positivebinCounts = append(positivebinCounts, float64(x))
				}
				var negativebinCounts []float64
				for _, x := range s.NegativeDeltas {
					negativebinCounts = append(negativebinCounts, float64(x))
				}
				var zeroCount int
				if x, ok := s.ZeroCount.(*prompb.Histogram_ZeroCountInt); ok {
					zeroCount = int(x.ZeroCountInt)
				} else if x, ok := s.ZeroCount.(*prompb.Histogram_ZeroCountFloat); ok {
					zeroCount = int(x.ZeroCountFloat)
				}

				sketch := chproto.DD{
					Mapping: &chproto.IndexMapping{Gamma: gamma},
					PositiveValues: &chproto.Store{
						ContiguousBinIndexOffset: int32(positiveOffset),
						ContiguousBinCounts:      positivebinCounts,
					},
					NegativeValues: &chproto.Store{
						ContiguousBinIndexOffset: int32(negativeOffset),
						ContiguousBinCounts:      negativebinCounts,
					},
					ZeroCount: float64(zeroCount),
				}

				meta := metricNameToMeta[fingerprintToName[fingerprint][nameLabel]]
				err = statement.Append(
					fingerprintToName[fingerprint][envLabel],
					meta.Temporality.String(),
					fingerprintToName[fingerprint][nameLabel],
					fingerprint,
					s.Timestamp,
					count,
					sum,
					min,
					max,
					sketch,
				)
				if err != nil {
					return err
				}
			}
		}

		start := time.Now()
		err = statement.Send()
		ctx, _ = tag.New(ctx,
			tag.Upsert(exporterKey, string(component.DataTypeMetrics.String())),
			tag.Upsert(tableKey, DISTRIBUTED_EXP_HIST_TABLE),
		)
		stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
		return err
	}()
	if err != nil {
		return err
	}

	return nil
}

// check interfaces
var (
	_ base.Storage = (*clickHouse)(nil)
)
