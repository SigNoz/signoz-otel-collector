// Copyright  The OpenTelemetry Authors
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

package clickhousetracesexporter

import (
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Factory implements storage.Factory for Clickhouse backend.
type Factory struct {
	logger     *zap.Logger
	Options    *Options
	db         clickhouse.Conn
	archive    clickhouse.Conn
	datasource string
	makeWriter writerMaker
}

// Writer writes spans to storage.
type Writer interface {
	WriteSpan(span *Span) error
}

type writerMaker func(logger *zap.Logger, db clickhouse.Conn, traceDatabase string, spansTable string, indexTable string, errorTable string, encoding Encoding, delay time.Duration, size int) (Writer, error)

// NewFactory creates a new Factory.
func ClickHouseNewFactory(migrations string, datasource string) *Factory {
	return &Factory{
		Options: NewOptions(migrations, datasource, primaryNamespace, archiveNamespace),
		// makeReader: func(db *clickhouse.Conn, operationsTable, indexTable, spansTable string) (spanstore.Reader, error) {
		// 	return store.NewTraceReader(db, operationsTable, indexTable, spansTable), nil
		// },
		makeWriter: func(logger *zap.Logger, db clickhouse.Conn, traceDatabase string, spansTable string, indexTable string, errorTable string, encoding Encoding, delay time.Duration, size int) (Writer, error) {
			return NewSpanWriter(logger, db, traceDatabase, spansTable, indexTable, errorTable, encoding, delay, size), nil
		},
	}
}

// Initialize implements storage.Factory
func (f *Factory) Initialize(logger *zap.Logger) error {
	f.logger = logger

	db, err := f.connect(f.Options.getPrimary())
	if err != nil {
		return fmt.Errorf("error connecting to primary db: %v", err)
	}

	f.db = db

	archiveConfig := f.Options.others[archiveNamespace]
	if archiveConfig.Enabled {
		archive, err := f.connect(archiveConfig)
		if err != nil {
			return fmt.Errorf("error connecting to archive db: %v", err)
		}

		f.archive = archive
	}

	f.logger.Info("Running migrations from path: ", zap.Any("test", f.Options.primary.Migrations))
	clickhouseUrl, err := buildClickhouseMigrateURL(f.Options.primary.Datasource)
	if err != nil {
		return fmt.Errorf("Failed to build Clickhouse migrate URL, error: %s", err)
	}
	m, err := migrate.New(
		"file://"+f.Options.primary.Migrations,
		clickhouseUrl)
	if err != nil {
		return fmt.Errorf("Clickhouse Migrate failed to run, error: %s", err)
	}
	err = m.Up()
	f.logger.Info("Clickhouse Migrate finished", zap.Error(err))
	return nil
}

func buildClickhouseMigrateURL(datasource string) (string, error) {
	// return fmt.Sprintf("clickhouse://localhost:9000?database=default&x-multi-statement=true"), nil
	var clickhouseUrl string
	database := "default"
	parsedURL, err := url.Parse(datasource)
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
		clickhouseUrl = fmt.Sprintf("clickhouse://%s?database=%s&x-multi-statement=true", host, database)
	}
	return clickhouseUrl, nil
}

func (f *Factory) connect(cfg *namespaceConfig) (clickhouse.Conn, error) {
	if cfg.Encoding != EncodingJSON && cfg.Encoding != EncodingProto {
		return nil, fmt.Errorf("unknown encoding %q, supported: %q, %q", cfg.Encoding, EncodingJSON, EncodingProto)
	}

	return cfg.Connector(cfg)
}

// AddFlags implements plugin.Configurable
func (f *Factory) AddFlags(flagSet *flag.FlagSet) {
	f.Options.AddFlags(flagSet)
}

// InitFromViper implements plugin.Configurable
func (f *Factory) InitFromViper(v *viper.Viper) {
	f.Options.InitFromViper(v)
}

// CreateSpanWriter implements storage.Factory
func (f *Factory) CreateSpanWriter() (Writer, error) {
	cfg := f.Options.getPrimary()
	return f.makeWriter(f.logger, f.db, cfg.TraceDatabase, cfg.SpansTable, cfg.IndexTable, cfg.ErrorTable, cfg.Encoding, cfg.WriteBatchDelay, cfg.WriteBatchSize)
}

// CreateArchiveSpanWriter implements storage.ArchiveFactory
func (f *Factory) CreateArchiveSpanWriter() (Writer, error) {
	if f.archive == nil {
		return nil, nil
	}
	cfg := f.Options.others[archiveNamespace]
	return f.makeWriter(f.logger, f.archive, "", cfg.TraceDatabase, cfg.SpansTable, cfg.ErrorTable, cfg.Encoding, cfg.WriteBatchDelay, cfg.WriteBatchSize)
}

// Close Implements io.Closer and closes the underlying storage
func (f *Factory) Close() error {
	if f.db != nil {
		err := f.db.Close()
		if err != nil {
			return err
		}

		f.db = nil
	}

	if f.archive != nil {
		err := f.archive.Close()
		if err != nil {
			return err
		}

		f.archive = nil
	}

	return nil
}
