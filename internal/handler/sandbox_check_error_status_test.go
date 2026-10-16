package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
)

type sandboxConfigLookupRepo struct {
	repository.TenantSandboxConfigRepository
}

func (sandboxConfigLookupRepo) GetByID(
	_ context.Context, _ uint64, id string,
) (*types.TenantSandboxConfigEntity, error) {
	if id == "broken" {
		return nil, errors.New("dial tcp 10.0.0.5:5432: connection refused")
	}
	return nil, nil
}

func TestCheckSandboxConfigLookupErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SystemHandler{
		sandboxConfigSvc: service.NewTenantSandboxConfigService(sandboxConfigLookupRepo{}, nil, nil, nil, nil),
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantInfoContextKey, &types.Tenant{ID: 7})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/system/sandbox-check", h.CheckSandboxConfig)

	runStatusCases(t, r, []statusCase{
		{
			"missing config", http.MethodPost, "/system/sandbox-check", `{"config_id":"missing"}`, 400,
			`{"success":false,"error":{"code":1000,"message":"Sandbox config not found"}}`,
		},
		{"repository failure", http.MethodPost, "/system/sandbox-check", `{"config_id":"broken"}`, 500, scrubbed500},
	})
}
