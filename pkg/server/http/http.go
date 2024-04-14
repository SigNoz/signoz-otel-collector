package http

import (
	"context"
	"net/http"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/gin-gonic/gin"
)

// This is a wrapper over http.Server
type Server struct {
	srv     *http.Server
	logger  log.Logger
	handler *gin.Engine
	opts    options
}

// Creates a new http server
func NewServer(logger log.Logger, handler *gin.Engine, opts ...Option) *Server {
	//Set default values
	serverOpts := options{
		listen: "0.0.0.0:8001",
	}

	// Merge default and input values
	for _, opt := range opts {
		opt(&serverOpts)
	}

	srv := &http.Server{
		Addr:           serverOpts.listen,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &Server{
		srv:     srv,
		logger:  logger,
		handler: handler,
		opts:    serverOpts,
	}
}

func (server *Server) Start(ctx context.Context) error {
	server.logger.Infoctx(ctx, "starting http server", "listen", server.opts.listen)
	if err := server.srv.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			server.logger.Errorctx(ctx, "failed to start server", err)
		}
		return err
	}
	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.srv.Shutdown(ctx); err != nil {
		server.logger.Errorctx(ctx, "failed to stop server", err)
		return err
	}

	server.logger.Infoctx(ctx, "server stopped gracefully")
	return nil
}
