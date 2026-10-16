package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type failingSettingRepo struct {
	interfaces.SystemSettingRepository
}

func (failingSettingRepo) List(context.Context) ([]*types.SystemSetting, error) { return nil, nil }

func (failingSettingRepo) Get(context.Context, string) (*types.SystemSetting, error) {
	return nil, errors.New("dial tcp 10.0.0.5:5432: connection refused")
}

func (failingSettingRepo) Upsert(context.Context, *types.SystemSetting) error {
	return errors.New("dial tcp 10.0.0.5:5432: connection refused")
}

func (failingSettingRepo) Delete(context.Context, string) (bool, error) {
	return false, errors.New("dial tcp 10.0.0.5:5432: connection refused")
}

func TestSystemSettingErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SystemHandler{systemSettingSvc: service.NewSystemSettingService(failingSettingRepo{}, nil, nil, nil)}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.GET("/settings/:key", h.GetSystemSetting)
	r.PUT("/settings/:key", h.UpdateSystemSetting)
	r.DELETE("/settings/:key", h.ResetSystemSetting)

	unknown := `{"success":false,"error":{"code":1000,"message":"unknown setting key \"nope\""}}`
	runStatusCases(t, r, []statusCase{
		{"get unknown key", http.MethodGet, "/settings/nope", "", 400, unknown},
		{"get repository failure", http.MethodGet, "/settings/ssrf.whitelist", "", 500, scrubbed500},
		{"update unknown key", http.MethodPut, "/settings/nope", `{"value":true}`, 400, unknown},
		{
			"update repository failure", http.MethodPut, "/settings/ssrf.whitelist",
			`{"value":["example.com"]}`, 500, scrubbed500,
		},
		{"reset unknown key", http.MethodDelete, "/settings/nope", "", 400, unknown},
		{"reset repository failure", http.MethodDelete, "/settings/ssrf.whitelist", "", 500, scrubbed500},
	})
}
