package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type weKnoraCloudSaveStub struct {
	interfaces.WeKnoraCloudService
}

func (weKnoraCloudSaveStub) SaveCredentials(_ context.Context, appID, _ string) error {
	switch appID {
	case "rejected":
		return apperrors.NewBadRequestError("app_id is required")
	case "unreachable":
		return apperrors.NewBadGatewayError("credential verification failed: service unreachable: i/o timeout")
	}
	return errors.New("update tenant: dial tcp 10.0.0.5:5432: connection refused")
}

func TestWeKnoraCloudSaveCredentialsErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/weknoracloud/credentials", NewWeKnoraCloudHandler(weKnoraCloudSaveStub{}).SaveCredentials)

	body := func(appID string) string { return `{"app_id":"` + appID + `","app_secret":"s"}` }
	runStatusCases(t, r, []statusCase{
		{
			"authored 400", http.MethodPost, "/weknoracloud/credentials", body("rejected"), 400,
			`{"success":false,"error":{"code":1000,"message":"app_id is required"}}`,
		},
		{
			"upstream 502", http.MethodPost, "/weknoracloud/credentials", body("unreachable"), 502,
			`{"success":false,"error":{"code":1011,` +
				`"message":"credential verification failed: service unreachable: i/o timeout"}}`,
		},
		{"internal failure", http.MethodPost, "/weknoracloud/credentials", body("db"), 500, scrubbed500},
	})
}
