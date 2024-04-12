package http

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

type LimitAPI struct {
	base *API
}

func NewLimitAPI(base *API) *KeyAPI {
	return &KeyAPI{

		base: base,
	}
}

func (api *LimitAPI) Register() {
	group := api.base.Handler().Group("/limits")

	group.GET("", func(gctx *gin.Context) {
		_, cancel := context.WithTimeout(gctx, 5*time.Second)
		defer cancel()
	})

	group.POST("", func(gctx *gin.Context) {
		_, cancel := context.WithTimeout(gctx, 5*time.Second)
		defer cancel()
	})

	group.DELETE("", func(gctx *gin.Context) {
		_, cancel := context.WithTimeout(gctx, 5*time.Second)
		defer cancel()
	})

}
