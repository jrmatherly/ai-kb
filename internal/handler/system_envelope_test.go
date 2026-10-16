package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type failingWeKnoraCloudService struct{ interfaces.WeKnoraCloudService }

func (failingWeKnoraCloudService) CheckStatus(context.Context) (*types.WeKnoraCloudStatusResult, error) {
	return nil, errors.New("credential store unreachable")
}

type missingMCPEndpointService struct{ interfaces.MCPEndpointService }

func (missingMCPEndpointService) Get(context.Context, uint64, string) (*types.MCPEndpoint, error) {
	return nil, service.ErrMCPEndpointNotFound
}

func TestSystemAreaHandlersReturnErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.GET("/models/weknoracloud/status", NewWeKnoraCloudHandler(failingWeKnoraCloudService{}).Status)
	r.POST("/wechat/qrcode/status", (&IMHandler{}).WeChatPollQRCodeStatus)
	r.GET("/mcp-endpoints/:endpoint_id", NewMCPEndpointHandler(missingMCPEndpointService{}).GetMCPEndpoint)

	cases := []struct {
		method, path, body string
		wantStatus         int
		wantBody           string
	}{
		{
			http.MethodGet, "/models/weknoracloud/status", "", http.StatusInternalServerError,
			`{"success":false,"error":{"code":1007,"message":"Internal server error"}}`,
		},
		{
			http.MethodPost, "/wechat/qrcode/status", `{}`, http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"qrcode is required"}}`,
		},
		{
			http.MethodGet, "/mcp-endpoints/ep-1", "", http.StatusNotFound,
			`{"success":false,"error":{"code":1003,"message":"mcp endpoint not found"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			require.JSONEq(t, tc.wantBody, w.Body.String())
		})
	}
}
