package http

import (
	"context"
	"net/http"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type TenantAPI struct {
	base    *API
	storage *storage.Storage
}

func NewTenantAPI(base *API, storage *storage.Storage) *TenantAPI {
	return &TenantAPI{
		base:    base,
		storage: storage,
	}
}

func (api *TenantAPI) Register() {
	group := api.base.Handler().Group("/tenants")

	group.POST("", func(gctx *gin.Context) {
		ctx, cancel := context.WithTimeout(gctx, 5*time.Second)
		defer cancel()

		type Req struct {
			Name string `json:"name" binding:"required"`
		}

		req := new(Req)
		if err := gctx.MustBindWith(req, binding.JSON); err != nil {
			api.base.SendErrorResponse(gctx, err)
			return
		}

		tenant := entity.NewTenant(req.Name)

		if err := api.storage.DAO.Tenants().Insert(ctx, tenant); err != nil {
			api.base.SendErrorResponse(gctx, err)
			return
		}

		type Res struct {
			Id string `json:"id"`
		}

		api.base.SendSuccessResponse(gctx, http.StatusCreated, Res{Id: tenant.Id.String()})
	})

}
