package http

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

type KeyAPI struct {
	base *API
}

func NewKeyAPI(base *API) *KeyAPI {
	return &KeyAPI{
		base: base,
	}
}

func (api *KeyAPI) Register() {
	group := api.base.Handler().Group("/keys")

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
