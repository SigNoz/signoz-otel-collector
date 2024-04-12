package http

import (
	"net/http"

	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/gin-gonic/gin"
)

type API struct {
	logger log.Logger
	engine *gin.Engine
}

func NewAPI(logger log.Logger) *API {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	// Create a default health endpoint
	engine.GET("/healthz", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	return &API{
		logger: logger,
		engine: engine,
	}
}

func (api *API) Handler() *gin.Engine {
	return api.engine
}
