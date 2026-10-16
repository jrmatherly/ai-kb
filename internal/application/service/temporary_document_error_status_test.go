package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestTemporaryDocumentCreateRejectsUnsupportedExtensionAs400(t *testing.T) {
	svc := NewTemporaryDocumentService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := svc.Create(context.Background(), 7, "session-1", "tool.exe", "application/octet-stream", 4,
		strings.NewReader("MZ.."), types.TemporaryDocumentCreateOptions{})
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok, "got %v", err)
	require.Equal(t, http.StatusBadRequest, appErr.HTTPCode)
	require.Equal(t, "unsupported file type: .exe", appErr.Message)
}
