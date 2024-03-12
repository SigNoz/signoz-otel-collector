package ottlfunctions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func makePCommonSlice(b []byte) pcommon.ByteSlice {
	r := pcommon.NewByteSlice()
	r.FromRaw(b)
	return r
}

func TestBytesToString(t *testing.T) {
	tests := []struct {
		name        string
		target      any
		expected    string
		shouldError bool
	}{
		{
			name:        "byte array",
			target:      [5]byte{'h', 'e', 'l', 'l', 'o'},
			expected:    "hello",
			shouldError: false,
		}, {
			name:        "byte slice",
			target:      []byte("hello world"),
			expected:    "hello world",
			shouldError: false,
		}, {
			name:        "pcommon byte slice",
			target:      makePCommonSlice([]byte("hello world")),
			expected:    "hello world",
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := BytesGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (interface{}, error) {
					return tt.target, nil
				},
			}

			fn, err := bytesToString(target)
			assert.NoError(t, err)

			result, err := fn(context.Background(), nil)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, result, tt.expected)
			}
		})
	}
}
