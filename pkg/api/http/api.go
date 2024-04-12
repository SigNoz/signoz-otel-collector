package http

import (
	"net/http"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"github.com/SigNoz/signoz-otel-collector/pkg/log"
	"github.com/gin-gonic/gin"
)

type status struct {
	name string
}

var (
	statusSuccess = status{name: "success"}
	statusError   = status{name: "error"}
)

type response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Type   string      `json:"type,omitempty"`
	Error  string      `json:"error,omitempty"`
}

var (
	errorTypeToHttpCode = map[errors.Type]int{
		errors.TypeInternal:     http.StatusInternalServerError,
		errors.TypeInvalidInput: http.StatusBadRequest,
		errors.TypeUnsupported:  http.StatusBadRequest,
	}
)

type API struct {
	logger log.Logger
	engine *gin.Engine
}

func NewAPI(logger log.Logger) *API {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	api := &API{
		logger: logger,
		engine: engine,
	}

	// Create a default health endpoint
	api.engine.GET("/healthz", func(gctx *gin.Context) {
		api.SendSuccessResponse(gctx, http.StatusOK, map[string]string{"status": "ok"})
	})

	return api
}

func (api *API) Handler() *gin.Engine {
	return api.engine
}

func (api *API) SendSuccessResponse(gctx *gin.Context, code int, data interface{}) {
	gctx.JSON(code, response{Status: statusSuccess.name, Data: data})
}

func (api *API) SendErrorResponse(gctx *gin.Context, cause error) {
	// See if this is an instance of the base error or not
	t, info, _ := errors.Unwrapb(cause)

	// Derive the http code from the error type
	httpCode := http.StatusInternalServerError
	code, ok := errorTypeToHttpCode[t]
	if ok {
		httpCode = code
	}

	gctx.AbortWithStatusJSON(httpCode, response{Status: statusError.name, Type: t.String(), Error: info})
}
