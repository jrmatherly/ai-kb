package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

func encodedAttachment(data string, declaredSize int64) AttachmentUpload {
	return AttachmentUpload{
		Data:     base64.StdEncoding.EncodeToString([]byte(data)),
		FileName: "file.txt",
		FileSize: declaredSize,
	}
}

func TestDecodeAndValidateAttachmentUploadsUsesActualBytes(t *testing.T) {
	uploads := []AttachmentUpload{encodedAttachment("123456", 1)}

	_, err := decodeAndValidateAttachmentUploads(uploads, 5, 5, 100)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds size limit")
}

func TestDecodeAndValidateAttachmentUploadsBoundsCountAndTotal(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		uploads := make([]AttachmentUpload, 6)
		for i := range uploads {
			uploads[i] = encodedAttachment("x", 1)
		}
		_, err := decodeAndValidateAttachmentUploads(uploads, 5, 10, 100)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at most 5")
	})

	t.Run("total", func(t *testing.T) {
		uploads := []AttachmentUpload{
			encodedAttachment(strings.Repeat("a", 6), 6),
			encodedAttachment(strings.Repeat("b", 6), 6),
		}
		_, err := decodeAndValidateAttachmentUploads(uploads, 5, 10, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "total request limit")
	})
}

func TestDecodeAndValidateAttachmentUploadsReturnsDecodedBytes(t *testing.T) {
	decoded, err := decodeAndValidateAttachmentUploads(
		[]AttachmentUpload{encodedAttachment("hello", 999)},
		5,
		10,
		10,
	)

	require.NoError(t, err)
	require.Len(t, decoded, 1)
	assert.Equal(t, []byte("hello"), decoded[0])
}

type attachmentTestSessions struct{ interfaces.SessionService }

func (attachmentTestSessions) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return &types.Session{ID: "session-1", TenantID: 7}, nil
}

type failingAttachmentStore struct{ interfaces.FileService }

func (failingAttachmentStore) SaveBytes(context.Context, []byte, uint64, string, bool) (string, error) {
	return "", stderrors.New("dial tcp 10.0.0.9:9000: connection refused")
}

func TestQAAttachmentProcessingErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, fileName string
		status         int
		body           string
	}{
		{
			"unsupported type is the client's", "payload.exe", http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"attachment 1: unsupported file type: .exe"}}`,
		},
		{
			"storage failure is scrubbed", "notes.txt", http.StatusInternalServerError,
			`{"success":false,"error":{"code":1007,"message":"Internal server error"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				sessionService:      attachmentTestSessions{},
				attachmentProcessor: NewAttachmentProcessor(failingAttachmentStore{}, nil, nil, nil),
			}
			router := gin.New()
			router.Use(middleware.ErrorHandler())
			router.POST("/knowledge-chat/:session_id", h.KnowledgeQA)
			body, err := json.Marshal(CreateKnowledgeQARequest{
				Query:             "summarize",
				AttachmentUploads: []AttachmentUpload{{Data: "aGk=", FileName: tc.fileName}},
			})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "/knowledge-chat/session-1", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
			assert.JSONEq(t, tc.body, response.Body.String())
		})
	}
}
