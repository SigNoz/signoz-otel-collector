package configrouter

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MuxConfig is a configuration for the mux router.
type MuxConfig struct {
}

// NewDefaultMuxConfig returns a new default mux config.
func NewDefaultMuxConfig() *MuxConfig {
	return &MuxConfig{}
}

// ToMuxRouter returns a new mux router.
func (c *MuxConfig) ToMuxRouter(logger *zap.Logger) *mux.Router {
	router := mux.NewRouter()
	router.MethodNotAllowedHandler = methodNotAllowed()
	router.NotFoundHandler = notFound()
	router.StrictSlash(false)
	router.Use(recovery(logger))

	return router
}

// Recovers from panics and writes an error to the response writer.
func recovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				err := recover()
				if err != nil {
					logger.Error("panic recovered", zap.Any("error", err))

					newErr := status.Newf(codes.Internal, "something went wrong")
					WriteError(w, FromStatus(newErr))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Returns a handler that returns a 405 status code and a message that the method is not allowed.
func methodNotAllowed() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := status.Newf(codes.Unimplemented, "%v method not allowed, supported: [POST]", req.Method)
		WriteError(w, FromStatus(err).WithCode(http.StatusMethodNotAllowed))
	})
}

// Returns a handler that returns a 404 status code and a message that the path is not found.
func notFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := status.Newf(codes.NotFound, "%v path not found", req.URL.Path)
		WriteError(w, FromStatus(err))
	})
}
