package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type dataSourceCredentialsStub struct {
	interfaces.DataSourceService
}

func (dataSourceCredentialsStub) GetDataSource(_ context.Context, id string) (*types.DataSource, error) {
	return &types.DataSource{ID: id, KnowledgeBaseID: "kb-1"}, nil
}

func (dataSourceCredentialsStub) UpdateDataSourceCredentials(
	_ context.Context, id string, _ map[string]interface{},
) (*types.DataSource, error) {
	if id == "invalid" {
		return nil, apperrors.NewBadRequestError(datasource.ErrDataSourceInvalid.Error())
	}
	return nil, fmt.Errorf("update data source: %w", errors.New("dial tcp 10.0.0.5:5432: connection refused"))
}

func TestDataSourceCredentialsPutErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewDataSourceCredentialsHandler(dataSourceCredentialsStub{}, &stubFolderKBService{})
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	r.PUT("/datasource/:id/credentials", h.Put)

	body := `{"credentials":{"token":"t"}}`
	runStatusCases(t, r, []statusCase{
		{
			"datasource sentinel", http.MethodPut, "/datasource/invalid/credentials", body, 400,
			`{"success":false,"error":{"code":1000,"message":"data source configuration is invalid"}}`,
		},
		{"repository failure", http.MethodPut, "/datasource/broken/credentials", body, 500, scrubbed500},
	})
}
