package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/infrastructure/docparser"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/models/providers"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

func listedProviderValues(t *testing.T) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewModelHandler(nil, nil)
	r := gin.New()
	r.GET("/models/providers", h.ListModelProviders)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/models/providers", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data []struct {
			Value string `json:"value"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	values := make([]string, 0, len(body.Data))
	for _, p := range body.Data {
		values = append(values, p.Value)
	}
	return values
}

func TestListModelProvidersHidesWeKnoraCloudWhenDisabled(t *testing.T) {
	t.Setenv(docparser.WeKnoraCloudEnabledEnv, "")
	require.NotContains(t, listedProviderValues(t), providers.WeKnoraCloudID)

	t.Setenv(docparser.WeKnoraCloudEnabledEnv, "true")
	require.Contains(t, listedProviderValues(t), providers.WeKnoraCloudID)
}

func TestCreateModelRefusesWeKnoraCloudWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(docparser.WeKnoraCloudEnabledEnv, "")
	h := NewModelHandler(nil, nil)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/models", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		h.CreateModel(c)
	})
	payload := bytes.NewBufferString(
		`{"name":"chat","type":"KnowledgeQA","source":"remote","parameters":{"provider":"weknoracloud"}}`,
	)
	req := httptest.NewRequest(http.MethodPost, "/models", payload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), docparser.WeKnoraCloudEnabledEnv)
}

type existingModelService struct {
	interfaces.ModelService
	model *types.Model
}

func (s existingModelService) GetModelByID(context.Context, string) (*types.Model, error) {
	return s.model, nil
}

func TestUpdateModelRefusesExistingWeKnoraCloudModelWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(docparser.WeKnoraCloudEnabledEnv, "")
	existing := &types.Model{
		ID: "m1", Name: "chat", Parameters: types.ModelParameters{Provider: providers.WeKnoraCloudID},
	}
	h := NewModelHandler(existingModelService{model: existing}, nil)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.PUT("/models/:id", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		h.UpdateModel(c)
	})
	req := httptest.NewRequest(http.MethodPut, "/models/m1", bytes.NewBufferString(`{"name":"chat","parameters":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), docparser.WeKnoraCloudEnabledEnv)
}
