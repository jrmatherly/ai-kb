package session

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestAgentQAImageSaveErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &types.CustomAgent{ID: "a", TenantID: 7}
	agent.Config.ImageUploadEnabled = true
	h := &Handler{
		sessionService:     engineTestSessions{},
		customAgentService: &resolveOwnAgentStub{agent: agent},
		fileService:        failingSaveFileService{},
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	r.POST("/agent-chat/:session_id", h.AgentQA)

	images := func(n int) []ImageAttachment {
		out := make([]ImageAttachment, n)
		for i := range out {
			out[i] = ImageAttachment{Data: "data:image/png;base64,aGk="}
		}
		return out
	}
	for _, tc := range []struct {
		name   string
		images []ImageAttachment
		status int
		want   string
	}{
		{
			"too many images", images(6), http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"Image save failed: too many images, max 5"}}`,
		},
		{"storage failure", images(1), http.StatusInternalServerError, scrubbed500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(CreateKnowledgeQARequest{Query: "what is this?", AgentID: "a", Images: tc.images})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/agent-chat/session-1", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}
