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
	"net/url"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/base"
	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousemetricsexporter/utils/timeseries"
	"github.com/prometheus/prometheus/prompb"
)

const (
	namespace                     = "promhouse"
	subsystem                     = "clickhouse"
	CLUSTER                       = "cluster"
	DISTRIBUTED_TIME_SERIES_TABLE = "distributed_time_series_v2"
	DISTRIBUTED_SAMPLES_TABLE     = "distributed_samples_v2"
	TIME_SERIES_TABLE             = "time_series_v2"
	SAMPLES_TABLE                 = "samples_v2"
)

// clickHouse implements storage interface for the ClickHouse.
type clickHouse struct {
	conn                 clickhouse.Conn
	l                    *logrus.Entry
	database             string
	maxTimeSeriesInQuery int

	timeSeriesRW sync.RWMutex
	// Maintains the lookup map for fingerprints that are
	// written to time series table. This map is used to eliminate the
	// unnecessary writes to table for the records that already exist.
	timeSeries map[uint64]struct{}

	mWrittenTimeSeries prometheus.Counter
}

type ClickHouseParams struct {
	DSN                  string
	DropDatabase         bool
	MaxOpenConns         int
	MaxTimeSeriesInQuery int
}

func NewClickHouse(params *ClickHouseParams) (base.Storage, error) {
	l := logrus.WithField("component", "clickhouse")

	dsnURL, err := url.Parse(params.DSN)

	if err != nil {
		return nil, err
	}
	database := dsnURL.Query().Get("database")
	if database == "" {
		return nil, fmt.Errorf("database should be set in ClickHouse DSN")
	}

	var queries []string
	if params.DropDatabase {
		queries = append(queries, fmt.Sprintf(`DROP DATABASE IF EXISTS %s ON CLUSTER %s;`, database, CLUSTER))
	}
	queries = append(queries, fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s ON CLUSTER %s`, database, CLUSTER))

	queries = append(queries, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s ON CLUSTER %s (
			metric_name LowCardinality(String),
			fingerprint UInt64 Codec(DoubleDelta, LZ4),
			timestamp_ms Int64 Codec(DoubleDelta, LZ4),
			value Float64 Codec(Gorilla, LZ4)
		)
		ENGINE = MergeTree
			PARTITION BY toDate(timestamp_ms / 1000)
			ORDER BY (metric_name, fingerprint, timestamp_ms);`, database, SAMPLES_TABLE, CLUSTER))

	queries = append(queries, fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.%s ON CLUSTER %s AS %s.%s ENGINE = Distributed("%s", "%s", %s, cityHash64(metric_name, fingerprint));`, database, DISTRIBUTED_SAMPLES_TABLE, CLUSTER, database, SAMPLES_TABLE, CLUSTER, database, SAMPLES_TABLE))

	queries = append(queries, `SET allow_experimental_object_type = 1`)

	queries = append(queries, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s ON CLUSTER %s(
			metric_name LowCardinality(String),
			fingerprint UInt64 Codec(DoubleDelta, LZ4),
			timestamp_ms Int64 Codec(DoubleDelta, LZ4),
			labels String Codec(ZSTD(5))
		)
		ENGINE = ReplacingMergeTree
			PARTITION BY toDate(timestamp_ms / 1000)
			ORDER BY (metric_name, fingerprint)`, database, TIME_SERIES_TABLE, CLUSTER))

	queries = append(queries, fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s.%s ON CLUSTER %s AS %s.%s ENGINE = Distributed("%s", %s, %s, cityHash64(metric_name, fingerprint));`, database, DISTRIBUTED_TIME_SERIES_TABLE, CLUSTER, database, TIME_SERIES_TABLE, CLUSTER, database, TIME_SERIES_TABLE))

	queries = append(queries, fmt.Sprintf(`
		ALTER TABLE %s.%s ON CLUSTER %s DROP COLUMN IF EXISTS labels_object`, database, TIME_SERIES_TABLE, CLUSTER))

	queries = append(queries, fmt.Sprintf(`
		ALTER TABLE %s.%s ON CLUSTER %s DROP COLUMN IF EXISTS labels_object`, database, DISTRIBUTED_TIME_SERIES_TABLE, CLUSTER))

	options := &clickhouse.Options{
		Addr: []string{dsnURL.Host},
	}
	if dsnURL.Query().Get("username") != "" {
		auth := clickhouse.Auth{
			// Database: "",
			Username: dsnURL.Query().Get("username"),
			Password: dsnURL.Query().Get("password"),
		}

		options.Auth = auth
	}
	conn, err := clickhouse.Open(options)

	if err != nil {
		return nil, fmt.Errorf("could not connect to clickhouse: %s", err)
	}

	for _, q := range queries {
		q = strings.TrimSpace(q)
		l.Infof("Executing:\n%s\n", q)
		if err = conn.Exec(context.Background(), q); err != nil {
			return nil, err
		}
	}

	ch := &clickHouse{
		conn:                 conn,
		l:                    l,
		database:             database,
		maxTimeSeriesInQuery: params.MaxTimeSeriesInQuery,

		timeSeries: make(map[uint64]struct{}, 8192),

		mWrittenTimeSeries: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "written_time_series",
			Help:      "Number of written time series.",
		}),
	}

	go func() {
		ctx := pprof.WithLabels(context.TODO(), pprof.Labels("component", "clickhouse_reloader"))
		pprof.SetGoroutineLabels(ctx)
		ch.runTimeSeriesReloader(ctx)
	}()

	return ch, nil
}

// runTimeSeriesReloader periodically queries the time series table
// and updates the timeSeries lookup map with new fingerprints.
// One might wonder why is there a need to reload the data from clickhouse
// when it just suffices to keep track of the fingerprint for the incoming
// write requests. This is because there could be multiple instance of
// metric exporters and they would only contain partial info with latter
// approach.
func (ch *clickHouse) runTimeSeriesReloader(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	q := fmt.Sprintf(`SELECT DISTINCT fingerprint FROM %s.%s`, ch.database, DISTRIBUTED_TIME_SERIES_TABLE)
	for {
		ch.timeSeriesRW.RLock()
		timeSeries := make(map[uint64]struct{}, len(ch.timeSeries))
		ch.timeSeriesRW.RUnlock()

		err := func() error {
			ch.l.Debug(q)
			rows, err := ch.conn.Query(ctx, q)
			if err != nil {
				return err
			}
			defer rows.Close()

			var f uint64
			for rows.Next() {
				if err = rows.Scan(&f); err != nil {
					return err
				}
				timeSeries[f] = struct{}{}
			}
			return rows.Err()
		}()
		if err == nil {
			ch.timeSeriesRW.Lock()
			n := len(timeSeries) - len(ch.timeSeries)
			for f, m := range timeSeries {
				ch.timeSeries[f] = m
			}
			ch.timeSeriesRW.Unlock()
			ch.l.Debugf("Loaded %d existing time series, %d were unknown to this instance.", len(timeSeries), n)
		} else {
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

func (ch *clickHouse) Write(ctx context.Context, data *prompb.WriteRequest) error {
	// calculate fingerprints, map them to time series
	fingerprints := make([]uint64, len(data.Timeseries))
	timeSeries := make(map[uint64][]*prompb.Label, len(data.Timeseries))
	fingerprintToName := make(map[uint64]string)

	for i, ts := range data.Timeseries {
		var metricName string
		labelsOverridden := make(map[string]*prompb.Label)
		for _, label := range ts.Labels {
			labelsOverridden[label.Name] = &prompb.Label{
				Name:  label.Name,
				Value: label.Value,
			}
			if label.Name == "__name__" {
				metricName = label.Value
			}
		}
		var labels []*prompb.Label
		for _, l := range labelsOverridden {
			labels = append(labels, l)
		}
		timeseries.SortLabels(labels)
		f := timeseries.Fingerprint(labels)
		fingerprints[i] = f
		timeSeries[f] = labels
		fingerprintToName[f] = metricName
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
		ctx := context.Background()
		err := ch.conn.Exec(ctx, `SET allow_experimental_object_type = 1`)
		if err != nil {
			return err
		}

		statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s (metric_name, timestamp_ms, fingerprint, labels) VALUES (?, ?, ?, ?)", ch.database, DISTRIBUTED_TIME_SERIES_TABLE))
		if err != nil {
			return err
		}
		timestamp := model.Now().Time().UnixMilli()
		for fingerprint, labels := range newTimeSeries {
			encodedLabels := string(marshalLabels(labels, make([]byte, 0, 128)))
			err = statement.Append(
				fingerprintToName[fingerprint],
				timestamp,
				fingerprint,
				encodedLabels,
			)
			if err != nil {
				return err
			}
		}

		return statement.Send()

	}()

	if err != nil {
		return err
	}

	err = func() error {
		ctx := context.Background()

		statement, err := ch.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", ch.database, DISTRIBUTED_SAMPLES_TABLE))
		if err != nil {
			return err
		}
		for i, ts := range data.Timeseries {
			fingerprint := fingerprints[i]
			for _, s := range ts.Samples {
				err = statement.Append(
					fingerprintToName[fingerprint],
					fingerprint,
					s.Timestamp,
					s.Value,
				)
				if err != nil {
					return err
				}
			}
		}

		return statement.Send()

	}()
	if err != nil {
		return err
	}

	n := len(newTimeSeries)
	if n != 0 {
		ch.mWrittenTimeSeries.Add(float64(n))
		ch.l.Debugf("Wrote %d new time series.", n)
	}
	return nil
}

// check interfaces
var (
	_ base.Storage = (*clickHouse)(nil)
)
