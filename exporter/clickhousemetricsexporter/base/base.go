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

// Package base provides common storage code.
package base

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/prometheus/prometheus/prompb"
)

type MetricMeta struct {
	Name        string
	Temporality pmetric.AggregationTemporality
	Description string
	Unit        string
	Typ         pmetric.MetricType
	IsMonotonic bool
}

// Storage represents generic storage.
type Storage interface {
	// Write puts data into storage.
	Write(context.Context, *prompb.WriteRequest, map[string]MetricMeta) error

	// Returns the DB conn.
	GetDBConn() interface{}
}
