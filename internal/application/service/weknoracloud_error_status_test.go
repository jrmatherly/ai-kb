package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
)

func TestWeKnoraCloudSaveCredentialsRejectsMissingFieldsAs400(t *testing.T) {
	svc := NewWeKnoraCloudService(nil, nil)
	for _, tc := range []struct{ appID, appSecret, want string }{
		{"", "secret", "app_id is required"},
		{"app", "", "app_secret is required"},
	} {
		err := svc.SaveCredentials(context.Background(), tc.appID, tc.appSecret)
		appErr, ok := apperrors.IsAppError(err)
		require.True(t, ok, "got %v", err)
		require.Equal(t, http.StatusBadRequest, appErr.HTTPCode)
		require.Equal(t, tc.want, appErr.Message)
	}
}
