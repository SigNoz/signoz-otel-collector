// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousemetricsexporter

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Test_Shutdown checks after Shutdown is called, incoming calls to PushMetrics return error.
func Test_Shutdown(t *testing.T) {
	che := &ClickHouseExporter{
		wg:        new(sync.WaitGroup),
		closeChan: make(chan struct{}),
	}
	wg := new(sync.WaitGroup)
	err := che.Shutdown(context.Background())
	require.NoError(t, err)
	errChan := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- che.PushMetrics(context.Background(), pmetric.NewMetrics())
		}()
	}
	wg.Wait()
	close(errChan)
	for ok := range errChan {
		assert.Error(t, ok)
	}
}

func Test_validateAndSanitizeExternalLabels(t *testing.T) {
	tests := []struct {
		name                string
		inputLabels         map[string]string
		expectedLabels      map[string]string
		returnErrorOnCreate bool
	}{
		{"success_case_no_labels",
			map[string]string{},
			map[string]string{},
			false,
		},
		{"success_case_with_labels",
			map[string]string{"key1": "val1"},
			map[string]string{"key1": "val1"},
			false,
		},
		{"success_case_2_with_labels",
			map[string]string{"__key1__": "val1"},
			map[string]string{"__key1__": "val1"},
			false,
		},
		{"success_case_with_sanitized_labels",
			map[string]string{"__key1.key__": "val1"},
			map[string]string{"__key1_key__": "val1"},
			false,
		},
		{"fail_case_empty_label",
			map[string]string{"": "val1"},
			map[string]string{},
			true,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLabels, err := validateAndSanitizeExternalLabels(tt.inputLabels)
			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.EqualValues(t, tt.expectedLabels, newLabels)
			assert.NoError(t, err)
		})
	}
}
