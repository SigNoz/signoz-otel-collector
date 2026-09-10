package spanfields

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestCalculatedFrom(t *testing.T) {
	tests := []struct {
		name  string
		kind  ptrace.SpanKind
		attrs map[string]any
		want  Calculated
	}{
		{
			name:  "server span keeps the full url and takes the host from the host attribute",
			kind:  ptrace.SpanKindServer,
			attrs: map[string]any{"http.method": "GET", "http.url": "https://api.example.com/users/42", "http.host": "api.example.com", "http.status_code": int64(200)},
			want:  Calculated{HttpMethod: "GET", HttpUrl: "https://api.example.com/users/42", HttpHost: "api.example.com", ResponseStatusCode: "200"},
		},
		{
			name:  "client span sets the external fields and derives the host from the url",
			kind:  ptrace.SpanKindClient,
			attrs: map[string]any{"http.request.method": "POST", "url.full": "https://payments.example.com/charge"},
			want:  Calculated{HttpMethod: "POST", ExternalHttpMethod: "POST", HttpUrl: "https://payments.example.com/charge", ExternalHttpUrl: "payments.example.com", HttpHost: "payments.example.com"},
		},
		{
			name:  "string status codes and database fields",
			kind:  ptrace.SpanKindClient,
			attrs: map[string]any{"rpc.grpc.status_code": "14", "db.namespace": "orders", "db.operation.name": "SELECT"},
			want:  Calculated{ResponseStatusCode: "14", DBName: "orders", DBOperation: "SELECT"},
		},
		{
			name:  "no matching attributes",
			kind:  ptrace.SpanKindInternal,
			attrs: map[string]any{"custom": "x"},
			want:  Calculated{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			assert.NoError(t, attrs.FromRaw(tt.attrs))
			assert.Equal(t, tt.want, CalculatedFrom(attrs, tt.kind))
		})
	}
}

func TestIsRemote(t *testing.T) {
	assert.Equal(t, "unknown", IsRemote(0))
	assert.Equal(t, "no", IsRemote(hasIsRemoteMask))
	assert.Equal(t, "yes", IsRemote(hasIsRemoteMask|isRemoteMask))
}
