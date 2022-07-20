// Copyright 2020, OpenTelemetry Authors
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

package clickhouselogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouselogsexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type clickhouseLogsExporter struct {
	client        *sql.DB
	insertLogsSQL string

	logger *zap.Logger
	cfg    *Config
	ksuid  ksuid.KSUID
}

func newExporter(logger *zap.Logger, cfg *Config) (*clickhouseLogsExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newClickhouseClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	insertLogsSQL := renderInsertLogsSQL(cfg)

	return &clickhouseLogsExporter{
		client:        client,
		insertLogsSQL: insertLogsSQL,
		logger:        logger,
		cfg:           cfg,
	}, nil
}

// Shutdown will shutdown the exporter.
func (e *clickhouseLogsExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func (e *clickhouseLogsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()
	err := doWithTx(ctx, e.client, func(tx *sql.Tx) error {
		statement, err := tx.PrepareContext(ctx, e.insertLogsSQL)
		if err != nil {
			return fmt.Errorf("PrepareContext:%w", err)
		}
		defer func() {
			_ = statement.Close()
		}()
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			logs := ld.ResourceLogs().At(i)
			res := logs.Resource()
			resources := attributesToSlice(res.Attributes())
			for j := 0; j < logs.ScopeLogs().Len(); j++ {
				rs := logs.ScopeLogs().At(j).LogRecords()
				for k := 0; k < rs.Len(); k++ {
					r := rs.At(k)

					id, err := ksuid.NewRandomWithTime(r.Timestamp().AsTime())
					if err != nil {
						return fmt.Errorf("Error creating id: %w", err)
					}

					attributes := attributesToSlice(r.Attributes())
					_, err = statement.ExecContext(ctx,
						uint64(r.Timestamp()),
						uint64(r.ObservedTimestamp()),
						id.String(),
						r.TraceID().HexString(),
						r.SpanID().HexString(),
						r.Flags(),
						r.SeverityText(),
						int32(r.SeverityNumber()),
						r.Body().AsString(),
						resources.StringKeys,
						resources.StringValues,
						attributes.StringKeys,
						attributes.StringValues,
						attributes.IntKeys,
						attributes.IntValues,
						attributes.FloatKeys,
						attributes.FloatValues,
					)
					if err != nil {
						return fmt.Errorf("ExecContext:%w", err)
					}
				}
				time.Sleep(2 * time.Second)
			}
		}
		return nil
	})
	duration := time.Since(start)
	e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
		zap.String("cost", duration.String()))
	return err
}

type attributesToSliceResponse struct {
	StringKeys   []string
	StringValues []string
	IntKeys      []string
	IntValues    []int64
	FloatKeys    []string
	FloatValues  []float64
}

func attributesToSlice(attributes pcommon.Map) (response attributesToSliceResponse) {
	attributes.Range(func(k string, v pcommon.Value) bool {
		switch v.Type().String() {
		case "INT":
			response.IntKeys = append(response.IntKeys, formatKey(k))
			response.IntValues = append(response.IntValues, v.IntVal())
		case "DOUBLE":
			response.FloatKeys = append(response.FloatKeys, formatKey(k))
			response.FloatValues = append(response.FloatValues, v.DoubleVal())
		default: // store it as string
			response.StringKeys = append(response.StringKeys, formatKey(k))
			response.StringValues = append(response.StringValues, v.AsString())
		}
		return true
	})
	return response
}

func formatKey(k string) string {
	return strings.ReplaceAll(k, ".", "_")
}

const (
	// language=ClickHouse SQL
	insertLogsSQLTemplate = `INSERT INTO %s.%s (
                        timestamp,
						observed_timestamp,
						id,
                        trace_id,
                        span_id,
                        trace_flags,
                        severity_text,
                        severity_number,
                        body,
                        resources_string_key,
						resources_string_value,
						attributes_string_key, 
						attributes_string_value,
						attributes_int64_key,
						attributes_int64_value,
						attributes_float64_key,
						attributes_float64_value
                        ) VALUES (
                                  ?,
								  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
                                  ?,
								  ?,
								  ?,
								  ?,
								  ?,
								  ?,
								  ?,
								  ?
                                  )`
)

var driverName = "clickhouse" // for testing

// newClickhouseClient create a clickhouse client.
func newClickhouseClient(logger *zap.Logger, cfg *Config) (*sql.DB, error) {
	// use empty database to create database
	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql.Open:%w", err)
	}

	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", cfg.DatabaseName)
	_, err = db.Exec(q)
	if err != nil {
		return nil, fmt.Errorf("failed to create database, err: %s", err)
	}

	// do the migration here
	logger.Info("Running migrations from path: ", zap.Any("test", cfg.Migrations))
	clickhouseUrl, err := buildClickhouseMigrateURL(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to build Clickhouse migrate URL, error: %s", err)
	}
	m, err := migrate.New("file://"+cfg.Migrations, clickhouseUrl)
	if err != nil {
		return nil, fmt.Errorf("Clickhouse Migrate failed to run, error: %s", err)
	}
	err = m.Up()
	if err != nil && !strings.HasSuffix(err.Error(), "no change") {
		return nil, fmt.Errorf("Clickhouse Migrate failed to run, error: %s", err)
	}

	logger.Info("Clickhouse Migrate finished")
	return db, nil
}

func buildClickhouseMigrateURL(cfg *Config) (string, error) {
	// return fmt.Sprintf("clickhouse://localhost:9000?database=default&x-multi-statement=true"), nil
	var clickhouseUrl string
	database := cfg.DatabaseName
	parsedURL, err := url.Parse(cfg.DSN)
	if err != nil {
		return "", err
	}
	host := parsedURL.Host
	if host == "" {
		return "", fmt.Errorf("Unable to parse host")

	}
	paramMap, err := url.ParseQuery(parsedURL.RawQuery)
	if err != nil {
		return "", err
	}
	username := paramMap["username"]
	password := paramMap["password"]

	if len(username) > 0 && len(password) > 0 {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s:%s@%s/%s?x-multi-statement=true", username[0], password[0], host, database)
	} else {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s/%s?x-multi-statement=true", host, database)
	}
	return clickhouseUrl, nil
}

func renderInsertLogsSQL(cfg *Config) string {
	return fmt.Sprintf(insertLogsSQLTemplate, cfg.DatabaseName, cfg.LogsTableName)
}

func doWithTx(_ context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("db.Begin: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
