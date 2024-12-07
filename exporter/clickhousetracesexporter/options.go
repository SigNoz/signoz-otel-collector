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
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

const (
	defaultDatasource               string   = "tcp://127.0.0.1:9000/?database=signoz_traces"
	DefaultTraceDatabase            string   = "signoz_traces"
	defaultOperationsTable          string   = "distributed_signoz_operations"
	DefaultIndexTable               string   = "distributed_signoz_index_v2"
	LocalIndexTable                 string   = "signoz_index_v2"
	defaultErrorTable               string   = "distributed_signoz_error_index_v2"
	defaultSpansTable               string   = "distributed_signoz_spans"
	defaultAttributeTable           string   = "distributed_span_attributes"
	defaultAttributeTableV2         string   = "distributed_tag_attributes_v2"
	defaultAttributeKeyTable        string   = "distributed_span_attributes_keys"
	DefaultDurationSortTable        string   = "durationSort"
	DefaultDurationSortMVTable      string   = "durationSortMV"
	defaultArchiveSpansTable        string   = "signoz_archive_spans"
	defaultDependencyGraphTable     string   = "dependency_graph_minutes"
	defaultDependencyGraphServiceMV string   = "dependency_graph_minutes_service_calls_mv"
	defaultDependencyGraphDbMV      string   = "dependency_graph_minutes_db_calls_mv"
	DependencyGraphMessagingMV      string   = "dependency_graph_minutes_messaging_calls_mv"
	defaultEncoding                 Encoding = EncodingJSON
	defaultIndexTableV3             string   = "distributed_signoz_index_v3"
	defaultResourceTableV3          string   = "distributed_traces_v3_resource"
)

// NamespaceConfig is Clickhouse's internal configuration data
type namespaceConfig struct {
	namespace                  string
	Enabled                    bool
	Datasource                 string
	TraceDatabase              string
	OperationsTable            string
	IndexTable                 string
	LocalIndexTable            string
	SpansTable                 string
	ErrorTable                 string
	AttributeTable             string
	AttributeTableV2           string
	AttributeKeyTable          string
	DurationSortTable          string
	DurationSortMVTable        string
	DependencyGraphServiceMV   string
	DependencyGraphDbMV        string
	DependencyGraphMessagingMV string
	DependencyGraphTable       string
	NumConsumers               int
	Encoding                   Encoding
	Connector                  Connector
	ExporterId                 uuid.UUID
	UseNewSchema               bool
	IndexTableV3               string
	ResourceTableV3            string
	MaxDistinctValues          int
	FetchKeysInterval          time.Duration
}

// Connecto defines how to connect to the database
type Connector func(cfg *namespaceConfig) (clickhouse.Conn, error)

func defaultConnector(cfg *namespaceConfig) (clickhouse.Conn, error) {
	ctx := context.Background()
	options, err := clickhouse.ParseDSN(cfg.Datasource)

	if err != nil {
		return nil, err
	}

	// setting maxOpenIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout`
	// error when using multiple consumers along with usage exporter
	maxIdleConnections := cfg.NumConsumers + 1

	if options.MaxIdleConns < maxIdleConnections {
		options.MaxIdleConns = maxIdleConnections
		options.MaxOpenConns = maxIdleConnections + 5
	}
	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(ctx); err != nil {
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
func NewOptions(exporterId uuid.UUID, config Config, primaryNamespace string, useNewSchema bool, otherNamespaces ...string) *Options {

	datasource := config.Datasource
	if datasource == "" {
		datasource = defaultDatasource
	}

	options := &Options{
		primary: &namespaceConfig{
			namespace:                  primaryNamespace,
			Enabled:                    true,
			Datasource:                 datasource,
			TraceDatabase:              DefaultTraceDatabase,
			OperationsTable:            defaultOperationsTable,
			IndexTable:                 DefaultIndexTable,
			LocalIndexTable:            LocalIndexTable,
			ErrorTable:                 defaultErrorTable,
			SpansTable:                 defaultSpansTable,
			AttributeTable:             defaultAttributeTable,
			AttributeTableV2:           defaultAttributeTableV2,
			AttributeKeyTable:          defaultAttributeKeyTable,
			DurationSortTable:          DefaultDurationSortTable,
			DurationSortMVTable:        DefaultDurationSortMVTable,
			DependencyGraphTable:       defaultDependencyGraphTable,
			DependencyGraphServiceMV:   defaultDependencyGraphServiceMV,
			DependencyGraphDbMV:        defaultDependencyGraphDbMV,
			DependencyGraphMessagingMV: DependencyGraphMessagingMV,
			NumConsumers:               config.QueueConfig.NumConsumers,
			Encoding:                   defaultEncoding,
			Connector:                  defaultConnector,
			ExporterId:                 exporterId,
			UseNewSchema:               useNewSchema,
			IndexTableV3:               defaultIndexTableV3,
			ResourceTableV3:            defaultResourceTableV3,
			MaxDistinctValues:          config.AttributesLimits.MaxDistinctValues,
			FetchKeysInterval:          config.AttributesLimits.FetchKeysInterval,
		},
		others: make(map[string]*namespaceConfig, len(otherNamespaces)),
	}

	for _, namespace := range otherNamespaces {
		if namespace == archiveNamespace {
			options.others[namespace] = &namespaceConfig{
				namespace:         namespace,
				Datasource:        datasource,
				OperationsTable:   "",
				IndexTable:        "",
				SpansTable:        defaultArchiveSpansTable,
				Encoding:          defaultEncoding,
				Connector:         defaultConnector,
				ExporterId:        exporterId,
				UseNewSchema:      useNewSchema,
				IndexTableV3:      defaultIndexTableV3,
				ResourceTableV3:   defaultResourceTableV3,
				AttributeTableV2:  defaultAttributeTableV2,
				MaxDistinctValues: config.AttributesLimits.MaxDistinctValues,
				FetchKeysInterval: config.AttributesLimits.FetchKeysInterval,
			}
		} else {
			options.others[namespace] = &namespaceConfig{namespace: namespace}
		}
	}

	return options
}

// getPrimary returns the primary namespace configuration
func (opt *Options) getPrimary() *namespaceConfig {
	return opt.primary
}
