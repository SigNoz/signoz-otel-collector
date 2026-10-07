package spanfields

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name  string
		kind  int8
		attrs map[string]any
		want  Calculated
	}{
		{
			name:  "status code from an int",
			kind:  2,
			attrs: map[string]any{"http.status_code": 404},
			want:  Calculated{ResponseStatusCode: "404"},
		},
		{
			name:  "status code from a string",
			kind:  2,
			attrs: map[string]any{"http.response.status_code": "503"},
			want:  Calculated{ResponseStatusCode: "503"},
		},
		{
			name:  "status code from a string that is not a number",
			kind:  2,
			attrs: map[string]any{"http.status_code": "OK"},
			want:  Calculated{ResponseStatusCode: "0"},
		},
		{
			name:  "status code from a double",
			kind:  2,
			attrs: map[string]any{"http.status_code": 200.0},
			want:  Calculated{ResponseStatusCode: "0"},
		},
		{
			name:  "grpc status code from a string",
			kind:  2,
			attrs: map[string]any{"rpc.grpc.status_code": "2"},
			want:  Calculated{ResponseStatusCode: "2"},
		},
		{
			name:  "client span: external url, host from the url, external method",
			kind:  3,
			attrs: map[string]any{"url.full": "https://api.example.com:8080/path", "http.request.method": "POST"},
			want: Calculated{
				HTTPURL: "https://api.example.com:8080/path", ExternalHTTPURL: "api.example.com", HTTPHost: "api.example.com",
				HTTPMethod: "POST", ExternalHTTPMethod: "POST",
			},
		},
		{
			name:  "server span: url and method only",
			kind:  2,
			attrs: map[string]any{"http.url": "https://api.example.com/path", "http.method": "GET"},
			want:  Calculated{HTTPURL: "https://api.example.com/path", HTTPMethod: "GET"},
		},
		{
			name:  "an explicit host attribute wins over the url host",
			kind:  3,
			attrs: map[string]any{"server.address": "explicit-host.com", "url.full": "https://url-host.com/path"},
			want:  Calculated{HTTPHost: "explicit-host.com", HTTPURL: "https://url-host.com/path", ExternalHTTPURL: "url-host.com"},
		},
		{
			name:  "database fields",
			kind:  3,
			attrs: map[string]any{"db.namespace": "orders", "db.operation.name": "SELECT"},
			want:  Calculated{DBName: "orders", DBOperation: "SELECT"},
		},
		{
			name:  "grpc and json-rpc status codes",
			kind:  2,
			attrs: map[string]any{"rpc.grpc.status_code": 14},
			want:  Calculated{ResponseStatusCode: "14"},
		},
		{
			name:  "json-rpc error code",
			kind:  2,
			attrs: map[string]any{"rpc.jsonrpc.error_code": "-32600"},
			want:  Calculated{ResponseStatusCode: "-32600"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			require.NoError(t, attrs.FromRaw(tt.attrs))
			assert.Equal(t, tt.want, Calculate(attrs, tt.kind))
		})
	}
}

func TestIsRemote(t *testing.T) {
	assert.Equal(t, "unknown", IsRemote(0))
	assert.Equal(t, "no", IsRemote(hasIsRemoteMask))
	assert.Equal(t, "yes", IsRemote(hasIsRemoteMask|isRemoteMask))
	assert.Equal(t, "unknown", IsRemote(isRemoteMask), "the remote bit counts only with the has-remote bit")
}
