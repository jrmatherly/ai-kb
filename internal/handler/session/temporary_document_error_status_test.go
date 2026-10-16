package session

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

const scrubbed500 = `{"success":false,"error":{"code":1007,"message":"Internal server error"}}`

type failingSaveFileService struct {
	interfaces.FileService
}

func (failingSaveFileService) SaveBytes(context.Context, []byte, uint64, string, bool) (string, error) {
	return "", errors.New("minio: PutObject http://10.0.0.7:9000/ai-kb: connection refused")
}

func TestUploadTemporaryDocumentErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{
		sessionService: engineTestSessions{},
		temporaryDocuments: service.NewTemporaryDocumentService(
			nil, failingSaveFileService{}, nil, nil, nil, nil, nil, nil, nil),
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	r.POST("/sessions/:session_id/attachments", h.UploadTemporaryDocument)

	for _, tc := range []struct {
		name, file string
		status     int
		want       string
	}{
		{
			"unsupported type", "tool.exe", http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"unsupported file type: .exe"}}`,
		},
		{"storage failure", "notes.txt", http.StatusInternalServerError, scrubbed500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			form := multipart.NewWriter(body)
			part, err := form.CreateFormFile("file", tc.file)
			require.NoError(t, err)
			_, _ = part.Write([]byte("content"))
			require.NoError(t, form.Close())
			req := httptest.NewRequest(http.MethodPost, "/sessions/session-1/attachments", body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.JSONEq(t, tc.want, w.Body.String())
		})
	}
}
