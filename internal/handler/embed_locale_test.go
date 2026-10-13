package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type localeEmbedSvc struct{ interfaces.EmbedChannelService }

func (localeEmbedSvc) Create(
	_ context.Context, _ uint64, _ string, ch *types.EmbedChannel,
) (*types.EmbedChannel, string, error) {
	return ch, "token", nil
}

func (localeEmbedSvc) Update(
	_ context.Context, _ uint64, _ string, ch *types.EmbedChannel,
	_, _, _, _ *bool, locale, _, _ *string,
) (*types.EmbedChannel, error) {
	if locale != nil {
		ch.DefaultLocale = *locale
	}
	return ch, nil
}

// An unsupported default_locale is rejected like the IM channel locale, not
// stored as empty.
func TestEmbedChannelDefaultLocaleIsValidated(t *testing.T) {
	h := &EmbedChannelHandler{embedSvc: localeEmbedSvc{}}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.POST("/agents/:id/embed-channels", h.CreateEmbedChannel)
	engine.PUT("/embed-channels/:channel_id", h.UpdateEmbedChannel)

	const wantError = `{"success":false,"error":{"code":1000,` +
		`"message":"locale must be one of: 'en-US', 'ja-JP', 'ko-KR', 'ru-RU', 'zh-CN', 'zh-TW'"}}`
	cases := []struct {
		name, method, path, body string
		wantCode                 int
		wantBody                 string
	}{
		{
			"create with unsupported locale", http.MethodPost, "/agents/a-1/embed-channels",
			`{"allowed_origins":["https://example.com"],"default_locale":"fr-FR"}`, http.StatusBadRequest, wantError,
		},
		{
			"update with unsupported locale", http.MethodPut, "/embed-channels/c-1",
			`{"default_locale":"fr-FR"}`, http.StatusBadRequest, wantError,
		},
		{
			"create with supported locale", http.MethodPost, "/agents/a-1/embed-channels",
			`{"allowed_origins":["https://example.com"],"default_locale":" en-US "}`, http.StatusCreated,
			`"default_locale":"en-US"`,
		},
		{
			"update clearing the locale", http.MethodPut, "/embed-channels/c-1",
			`{"default_locale":""}`, http.StatusOK, `"success":true`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			require.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), tc.wantBody)
		})
	}
}
