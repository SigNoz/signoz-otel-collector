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
	"context"
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/viper"
)

const (
	defaultDatasource        string        = "tcp://127.0.0.1:9000/?database=signoz_traces"
	defaultTraceDatabase     string        = "signoz_traces"
	defaultMigrations        string        = "/migrations"
	defaultOperationsTable   string        = "signoz_operations"
	defaultIndexTable        string        = "signoz_index_v2"
	defaultErrorTable        string        = "signoz_error_index"
	defaultSpansTable        string        = "signoz_spans"
	defaultArchiveSpansTable string        = "signoz_archive_spans"
	defaultWriteBatchDelay   time.Duration = 5 * time.Second
	defaultWriteBatchSize    int           = 10000
	defaultEncoding          Encoding      = EncodingJSON
)

const (
	suffixEnabled         = ".enabled"
	suffixDatasource      = ".datasource"
	suffixTraceDatabase   = ".trace-database"
	suffixMigrations      = ".migrations"
	suffixOperationsTable = ".operations-table"
	suffixIndexTable      = ".index-table"
	suffixSpansTable      = ".spans-table"
	suffixWriteBatchDelay = ".write-batch-delay"
	suffixWriteBatchSize  = ".write-batch-size"
	suffixEncoding        = ".encoding"
)

// NamespaceConfig is Clickhouse's internal configuration data
type namespaceConfig struct {
	namespace       string
	Enabled         bool
	Datasource      string
	Migrations      string
	TraceDatabase   string
	OperationsTable string
	IndexTable      string
	SpansTable      string
	ErrorTable      string
	WriteBatchDelay time.Duration
	WriteBatchSize  int
	Encoding        Encoding
	Connector       Connector
}

// Connecto defines how to connect to the database
type Connector func(cfg *namespaceConfig) (clickhouse.Conn, error)

func defaultConnector(cfg *namespaceConfig) (clickhouse.Conn, error) {
	ctx := context.Background()
	dsnURL, err := url.Parse(cfg.Datasource)
	options := &clickhouse.Options{
		Addr: []string{dsnURL.Host},
	}
	if dsnURL.Query().Get("username") != "" {
		auth := clickhouse.Auth{
			Username: dsnURL.Query().Get("username"),
			Password: dsnURL.Query().Get("password"),
		}
		options.Auth = auth
	}
	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s`, dsnURL.Query().Get("database"))
	if err := db.Exec(ctx, query); err != nil {
		return nil, err
	}
	return db, nil
}

// Options store storage plugin related configs
type Options struct {
	primary *namespaceConfig

	others map[string]*namespaceConfig
}

// NewOptions creates a new Options struct.
func NewOptions(migrations string, datasource string, primaryNamespace string, otherNamespaces ...string) *Options {

	if datasource == "" {
		datasource = defaultDatasource
	}
	if migrations == "" {
		migrations = defaultMigrations
	}

	options := &Options{
		primary: &namespaceConfig{
			namespace:       primaryNamespace,
			Enabled:         true,
			Datasource:      datasource,
			Migrations:      migrations,
			TraceDatabase:   defaultTraceDatabase,
			OperationsTable: defaultOperationsTable,
			IndexTable:      defaultIndexTable,
			ErrorTable:      defaultErrorTable,
			SpansTable:      defaultSpansTable,
			WriteBatchDelay: defaultWriteBatchDelay,
			WriteBatchSize:  defaultWriteBatchSize,
			Encoding:        defaultEncoding,
			Connector:       defaultConnector,
		},
		others: make(map[string]*namespaceConfig, len(otherNamespaces)),
	}

	for _, namespace := range otherNamespaces {
		if namespace == archiveNamespace {
			options.others[namespace] = &namespaceConfig{
				namespace:       namespace,
				Datasource:      datasource,
				Migrations:      migrations,
				OperationsTable: "",
				IndexTable:      "",
				SpansTable:      defaultArchiveSpansTable,
				WriteBatchDelay: defaultWriteBatchDelay,
				WriteBatchSize:  defaultWriteBatchSize,
				Encoding:        defaultEncoding,
				Connector:       defaultConnector,
			}
		} else {
			options.others[namespace] = &namespaceConfig{namespace: namespace}
		}
	}

	return options
}

// AddFlags adds flags for Options
func (opt *Options) AddFlags(flagSet *flag.FlagSet) {
	addFlags(flagSet, opt.primary)
	for _, cfg := range opt.others {
		addFlags(flagSet, cfg)
	}
}

func addFlags(flagSet *flag.FlagSet, nsConfig *namespaceConfig) {
	if nsConfig.namespace == archiveNamespace {
		flagSet.Bool(
			nsConfig.namespace+suffixEnabled,
			nsConfig.Enabled,
			"Enable archive storage")
	}

	flagSet.String(
		nsConfig.namespace+suffixDatasource,
		nsConfig.Datasource,
		"Clickhouse datasource string.",
	)

	if nsConfig.namespace != archiveNamespace {
		flagSet.String(
			nsConfig.namespace+suffixOperationsTable,
			nsConfig.OperationsTable,
			"Clickhouse operations table name.",
		)

		flagSet.String(
			nsConfig.namespace+suffixIndexTable,
			nsConfig.IndexTable,
			"Clickhouse index table name.",
		)
	}

	flagSet.String(
		nsConfig.namespace+suffixSpansTable,
		nsConfig.SpansTable,
		"Clickhouse spans table name.",
	)

	flagSet.Duration(
		nsConfig.namespace+suffixWriteBatchDelay,
		nsConfig.WriteBatchDelay,
		"A duration after which spans are flushed to Clickhouse",
	)

	flagSet.Int(
		nsConfig.namespace+suffixWriteBatchSize,
		nsConfig.WriteBatchSize,
		"A number of spans buffered before they are flushed to Clickhouse",
	)

	flagSet.String(
		nsConfig.namespace+suffixEncoding,
		string(nsConfig.Encoding),
		"Encoding to store spans (json allows out of band queries, protobuf is more compact)",
	)
}

// InitFromViper initializes Options with properties from viper
func (opt *Options) InitFromViper(v *viper.Viper) {
	initFromViper(opt.primary, v)
	for _, cfg := range opt.others {
		initFromViper(cfg, v)
	}
}

func initFromViper(cfg *namespaceConfig, v *viper.Viper) {
	cfg.Enabled = v.GetBool(cfg.namespace + suffixEnabled)
	cfg.Datasource = v.GetString(cfg.namespace + suffixDatasource)
	cfg.TraceDatabase = v.GetString(cfg.namespace + suffixTraceDatabase)
	cfg.IndexTable = v.GetString(cfg.namespace + suffixIndexTable)
	cfg.SpansTable = v.GetString(cfg.namespace + suffixSpansTable)
	cfg.OperationsTable = v.GetString(cfg.namespace + suffixOperationsTable)
	cfg.WriteBatchDelay = v.GetDuration(cfg.namespace + suffixWriteBatchDelay)
	cfg.WriteBatchSize = v.GetInt(cfg.namespace + suffixWriteBatchSize)
	cfg.Encoding = Encoding(v.GetString(cfg.namespace + suffixEncoding))
}

// GetPrimary returns the primary namespace configuration
func (opt *Options) getPrimary() *namespaceConfig {
	return opt.primary
}
