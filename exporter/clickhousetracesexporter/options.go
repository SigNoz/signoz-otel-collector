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

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/spf13/viper"
)

const (
	defaultDatasource               string   = "tcp://127.0.0.1:9000/?database=signoz_traces"
	defaultTraceDatabase            string   = "signoz_traces"
	defaultMigrations               string   = "/migrations"
	defaultOperationsTable          string   = "distributed_signoz_operations"
	defaultIndexTable               string   = "distributed_signoz_index_v2"
	localIndexTable                 string   = "signoz_index_v2"
	defaultErrorTable               string   = "distributed_signoz_error_index_v2"
	defaultSpansTable               string   = "distributed_signoz_spans"
	defaultAttributeTable           string   = "distributed_span_attributes"
	defaultAttributeKeyTable        string   = "distributed_span_attributes_keys"
	defaultDurationSortTable        string   = "durationSort"
	defaultDurationSortMVTable      string   = "durationSortMV"
	defaultArchiveSpansTable        string   = "signoz_archive_spans"
	defaultClusterName              string   = "cluster"
	defaultDependencyGraphTable     string   = "dependency_graph_minutes"
	defaultDependencyGraphServiceMV string   = "dependency_graph_minutes_service_calls_mv"
	defaultDependencyGraphDbMV      string   = "dependency_graph_minutes_db_calls_mv"
	DependencyGraphMessagingMV      string   = "dependency_graph_minutes_messaging_calls_mv"
	defaultEncoding                 Encoding = EncodingJSON
)

const (
	suffixEnabled         = ".enabled"
	suffixDatasources     = ".datasources"
	suffixTraceDatabase   = ".trace-database"
	suffixMigrations      = ".migrations"
	suffixOperationsTable = ".operations-table"
	suffixIndexTable      = ".index-table"
	suffixSpansTable      = ".spans-table"
	suffixEncoding        = ".encoding"
)

// NamespaceConfig is Clickhouse's internal configuration data
type namespaceConfig struct {
	namespace                  string
	Enabled                    bool
	Datasources                []string
	Migrations                 string
	TraceDatabase              string
	OperationsTable            string
	IndexTable                 string
	LocalIndexTable            string
	SpansTable                 string
	ErrorTable                 string
	AttributeTable             string
	AttributeKeyTable          string
	Cluster                    string
	DurationSortTable          string
	DurationSortMVTable        string
	DependencyGraphServiceMV   string
	DependencyGraphDbMV        string
	DependencyGraphMessagingMV string
	DependencyGraphTable       string
	DockerMultiNodeCluster     bool
	Encoding                   Encoding
	Connectors                 Connectors
}

// Connecto defines how to connect to the database
type Connectors func(cfg *namespaceConfig) ([]clickhouse.Conn, error)

func connectorFor(datasource string, cluster string) (clickhouse.Conn, error) {
	ctx := context.Background()
	dsnURL, err := url.Parse(datasource)
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

	query := fmt.Sprintf(`CREATE DATABASE IF NOT EXISTS %s ON CLUSTER %s`, dsnURL.Query().Get("database"), cluster)
	if err := db.Exec(ctx, query); err != nil {
		return nil, err
	}
	return db, nil
}

func defaultConnectors(cfg *namespaceConfig) ([]clickhouse.Conn, error) {
	connectors := make([]clickhouse.Conn, 0, len(cfg.Datasources))
	for _, ds := range cfg.Datasources {
		conn, err := connectorFor(ds, cfg.Cluster)
		if err != nil {
			return nil, err
		}
		connectors = append(connectors, conn)
	}
	return connectors, nil
}

// Options store storage plugin related configs
type Options struct {
	primary *namespaceConfig

	others map[string]*namespaceConfig
}

// NewOptions creates a new Options struct.
func NewOptions(migrations string, datasources []string, dockerMultiNodeCluster bool, primaryNamespace string, otherNamespaces ...string) *Options {

	if len(datasources) == 0 {
		datasources = []string{defaultDatasource}
	}
	if migrations == "" {
		migrations = defaultMigrations
	}

	options := &Options{
		primary: &namespaceConfig{
			namespace:                  primaryNamespace,
			Enabled:                    true,
			Datasources:                datasources,
			Migrations:                 migrations,
			TraceDatabase:              defaultTraceDatabase,
			OperationsTable:            defaultOperationsTable,
			IndexTable:                 defaultIndexTable,
			LocalIndexTable:            localIndexTable,
			ErrorTable:                 defaultErrorTable,
			SpansTable:                 defaultSpansTable,
			AttributeTable:             defaultAttributeTable,
			AttributeKeyTable:          defaultAttributeKeyTable,
			DurationSortTable:          defaultDurationSortTable,
			DurationSortMVTable:        defaultDurationSortMVTable,
			Cluster:                    defaultClusterName,
			DependencyGraphTable:       defaultDependencyGraphTable,
			DependencyGraphServiceMV:   defaultDependencyGraphServiceMV,
			DependencyGraphDbMV:        defaultDependencyGraphDbMV,
			DependencyGraphMessagingMV: DependencyGraphMessagingMV,
			DockerMultiNodeCluster:     dockerMultiNodeCluster,
			Encoding:                   defaultEncoding,
			Connectors:                 defaultConnectors,
		},
		others: make(map[string]*namespaceConfig, len(otherNamespaces)),
	}

	for _, namespace := range otherNamespaces {
		if namespace == archiveNamespace {
			options.others[namespace] = &namespaceConfig{
				namespace:       namespace,
				Datasources:     datasources,
				Migrations:      migrations,
				OperationsTable: "",
				IndexTable:      "",
				SpansTable:      defaultArchiveSpansTable,
				Encoding:        defaultEncoding,
				Connectors:      defaultConnectors,
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

type stringSliceValue struct {
	slice *[]string
}

func (ssv *stringSliceValue) String() string {
	return fmt.Sprintf("%v", *ssv.slice)
}

func (ssv *stringSliceValue) Set(value string) error {
	*ssv.slice = append(*ssv.slice, value)
	return nil
}

func addFlags(flagSet *flag.FlagSet, nsConfig *namespaceConfig) {
	if nsConfig.namespace == archiveNamespace {
		flagSet.Bool(
			nsConfig.namespace+suffixEnabled,
			nsConfig.Enabled,
			"Enable archive storage")
	}

	var datasources []string
	flagSet.Var(
		&stringSliceValue{&datasources},
		nsConfig.namespace+suffixDatasources,
		"Clickhouse datasource string. Can be specified multiple times.",
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

	flagSet.String(
		nsConfig.namespace+suffixEncoding,
		string(nsConfig.Encoding),
		"Encoding to store spans (json allows out of band queries, protobuf is more compact)",
	)
}

// InitFromViper initializes Options with properties from viper
func (opt *Options) InitFromViper(v *viper.Viper) error {
	if err := initFromViper(opt.primary, v); err != nil {
		return err
	}
	for _, cfg := range opt.others {
		if err := initFromViper(cfg, v); err != nil {
			return err
		}
	}
	return nil
}

func initFromViper(cfg *namespaceConfig, v *viper.Viper) error {
	cfg.Enabled = v.GetBool(cfg.namespace + suffixEnabled)
	datasources := v.Get(cfg.namespace + suffixDatasources)
	if datasources != nil {
		switch ds := datasources.(type) {
		case string:
			cfg.Datasources = []string{ds}
		case []interface{}:
			var dsSlice []string
			for _, d := range ds {
				if s, ok := d.(string); ok {
					dsSlice = append(dsSlice, s)
				} else {
					return fmt.Errorf("invalid type for %s: %T", cfg.namespace+suffixDatasources, d)
				}
			}
			cfg.Datasources = dsSlice
		default:
			return fmt.Errorf("invalid type for %s: %T", cfg.namespace+suffixDatasources, datasources)
		}
	}
	cfg.TraceDatabase = v.GetString(cfg.namespace + suffixTraceDatabase)
	cfg.IndexTable = v.GetString(cfg.namespace + suffixIndexTable)
	cfg.SpansTable = v.GetString(cfg.namespace + suffixSpansTable)
	cfg.OperationsTable = v.GetString(cfg.namespace + suffixOperationsTable)
	cfg.Encoding = Encoding(v.GetString(cfg.namespace + suffixEncoding))
	return nil
}

// GetPrimary returns the primary namespace configuration
func (opt *Options) getPrimary() *namespaceConfig {
	return opt.primary
}
