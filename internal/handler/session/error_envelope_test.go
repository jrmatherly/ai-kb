package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/browserskill"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type missingOwnedSessionService struct {
	interfaces.SessionService
}

func (missingOwnedSessionService) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return nil, errors.ErrSessionNotFound
}

func serveEnvelope(t *testing.T, method, route, target, body string, h gin.HandlerFunc) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Handle(method, route, h)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestSessionHandlersReturnErrorEnvelope(t *testing.T) {
	cases := []struct {
		name, method, route, target, body string
		handler                           gin.HandlerFunc
		wantCode                          int
		wantBody                          string
	}{
		{
			name: "desktop ticket for a missing session", method: http.MethodPost,
			route: "/sessions/:session_id/sandbox/desktop-ticket", target: "/sessions/s1/sandbox/desktop-ticket",
			handler:  (&Handler{sessionService: missingOwnedSessionService{}}).IssueSandboxDesktopTicket,
			wantCode: http.StatusNotFound,
			wantBody: `{"success":false,"error":{"code":1003,"message":"session not found"}}`,
		},
		{
			// Rejected before the upgrade, so the handshake response is still plain HTTP.
			name: "terminal handshake without a ticket", method: http.MethodGet,
			route: "/sessions/:id/sandbox/terminal", target: "/sessions/s1/sandbox/terminal",
			handler:  (&Handler{}).SandboxTerminalWS,
			wantCode: http.StatusUnauthorized,
			wantBody: `{"success":false,"error":{"code":1001,"message":"Unauthorized: invalid or expired ticket"}}`,
		},
		{
			name: "browser account without a user", method: http.MethodGet,
			route: "/me/browser", target: "/me/browser",
			handler:  (&Handler{browserSkill: browserskill.NewManager()}).BrowserSkillAccount,
			wantCode: http.StatusUnauthorized,
			wantBody: `{"success":false,"error":{"code":1001,"message":"login required"}}`,
		},
		{
			// No events means no SSE header has been written yet.
			name: "continue-stream with no events", method: http.MethodGet,
			route: "/sessions/continue-stream/:session_id", target: "/sessions/continue-stream/s1?message_id=m1",
			handler: (&Handler{
				sessionService: &stubSessionService{},
				messageService: &stubMessageServiceForStream{},
				streamManager:  &stubStreamManager{},
			}).ContinueStream,
			wantCode: http.StatusNotFound,
			wantBody: `{"success":false,"error":{"code":1003,"message":"No stream events found"}}`,
		},
		{
			name: "stop without message_id", method: http.MethodPost,
			route: "/sessions/:session_id/stop", target: "/sessions/s1/stop", body: `{}`,
			handler:  (&Handler{}).StopSession,
			wantCode: http.StatusBadRequest,
			wantBody: `{"success":false,"error":{"code":1000,"message":"message_id is required"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := serveEnvelope(t, tc.method, tc.route, tc.target, tc.body, tc.handler)
			require.Equal(t, tc.wantCode, code, body)
			require.JSONEq(t, tc.wantBody, body)
		})
	}
}
