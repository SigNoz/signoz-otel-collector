package configrouter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

func TestMuxConfig(t *testing.T) {
	ctx := context.Background()

	routerConfig := NewDefaultMuxConfig().ToMuxRouter(zap.NewNop())
	routerConfig.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	}).Methods(http.MethodGet)
	routerConfig.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
	}).Methods(http.MethodGet)

	serverConfig := confighttp.ServerConfig{
		Endpoint: "localhost:0",
	}

	httpListener, err := serverConfig.ToListener(ctx)
	require.NoError(t, err)

	httpServer, err := serverConfig.ToServer(context.Background(), componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), routerConfig)
	require.NoError(t, err)

	go func() {
		_ = httpServer.Serve(httpListener)
	}()

	t.Cleanup(func() {
		require.NoError(t, httpServer.Shutdown(ctx))
	})

	testCases := []struct {
		name               string
		body               io.Reader
		path               string
		method             string
		expectedCode       int
		expectedCodeInBody int64
	}{
		{
			name:               "NotFound",
			body:               strings.NewReader("[{}]"),
			path:               "does-not-exist",
			method:             http.MethodPost,
			expectedCode:       http.StatusNotFound,
			expectedCodeInBody: int64(codes.NotFound),
		},
		{
			name:               "MethodNotAllowed",
			body:               strings.NewReader("[{}]"),
			path:               "test",
			method:             http.MethodPost,
			expectedCode:       http.StatusMethodNotAllowed,
			expectedCodeInBody: int64(codes.Unimplemented),
		},
		{
			name:               "Panic",
			body:               strings.NewReader("[{}]"),
			path:               "panic",
			method:             http.MethodGet,
			expectedCode:       http.StatusInternalServerError,
			expectedCodeInBody: int64(codes.Internal),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(
				tc.method,
				fmt.Sprintf("http://%s/%s", httpListener.Addr().String(), tc.path),
				tc.body,
			)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			err = resp.Body.Close()
			require.NoError(t, err)

			assert.Equal(t, tc.expectedCode, resp.StatusCode)
			assert.Equal(t, tc.expectedCodeInBody, gjson.Get(string(body), "code").Int())
		})
	}
}
