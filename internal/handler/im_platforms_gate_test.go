package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/im"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func TestCreateIMChannelRefusesDisabledPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(im.PlatformsEnv, "slack")
	h := NewIMHandler(nil)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/agents/:id/im-channels", h.CreateIMChannel)
	req := httptest.NewRequest(http.MethodPost, "/agents/agent-1/im-channels",
		bytes.NewBufferString(`{"platform":"wecom","name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "'slack'")
	require.NotContains(t, rec.Body.String(), "'wecom'")
}

func TestDeploymentCapabilitiesCarryIMPlatforms(t *testing.T) {
	on := BuildDeploymentCapabilities("standard", DeploymentFeatureAvailability{
		IM: true, IMPlatforms: []string{"slack", "telegram"},
	})
	require.Equal(t, []string{"slack", "telegram"}, on.IMPlatforms)
	require.True(t, on.Capabilities["integrations.im"].Supported)

	none := BuildDeploymentCapabilities("standard", DeploymentFeatureAvailability{IM: true})
	require.Equal(t, []string{}, none.IMPlatforms)
	require.False(t, none.Capabilities["integrations.im"].Supported)
	require.Equal(t, "no_platform_enabled", none.Capabilities["integrations.im"].Reason)
}
