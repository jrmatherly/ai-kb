package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slugTenantService records the slug writes the handler makes.
type slugTenantService struct {
	interfaces.TenantService
	tenant    *types.Tenant
	taken     string
	updateErr error
	setCalls  []*string
	updated   *types.Tenant
}

func (s *slugTenantService) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	copied := *s.tenant
	return &copied, nil
}

func (s *slugTenantService) SetTenantSlug(_ context.Context, _ uint64, slug *string) error {
	if slug != nil && *slug == s.taken {
		return service.ErrTenantSlugTaken
	}
	s.setCalls = append(s.setCalls, slug)
	return nil
}

func (s *slugTenantService) UpdateTenant(_ context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.updated = tenant
	return tenant, nil
}

func slugTestEngine(svc *slugTenantService, systemAdmin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &TenantHandler{service: svc}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, svc.tenant.ID)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleOwner)
		ctx = context.WithValue(ctx, types.SystemAdminContextKey, systemAdmin)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/tenants/:id", h.GetTenant)
	r.PUT("/tenants/:id", h.UpdateTenant)
	return r
}

func putTenant(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/tenants/7", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newSlugService() *slugTenantService {
	return &slugTenantService{tenant: &types.Tenant{ID: 7, Name: "Seven"}}
}

func TestUpdateTenant_SlugRequiresSystemAdmin(t *testing.T) {
	svc := newSlugService()
	r := slugTestEngine(svc, false)
	w := putTenant(t, r, `{"slug":"home"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, svc.setCalls)

	w = putTenant(t, r, `{"name":"Renamed"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.updated)
	assert.Equal(t, "Renamed", svc.updated.Name)
}

func TestUpdateTenant_SlugRejectsInvalid(t *testing.T) {
	svc := newSlugService()
	w := putTenant(t, slugTestEngine(svc, true), `{"slug":"Home KB"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.setCalls)
}

func TestUpdateTenant_SlugDuplicateIs409(t *testing.T) {
	svc := newSlugService()
	svc.taken = "home"
	w := putTenant(t, slugTestEngine(svc, true), `{"slug":"home"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Empty(t, svc.setCalls)
}

func TestUpdateTenant_SlugIsSavedOnTheTenant(t *testing.T) {
	svc := newSlugService()
	w := putTenant(t, slugTestEngine(svc, true), `{"slug":"home"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.setCalls, 1)
	require.NotNil(t, svc.setCalls[0])
	assert.Equal(t, "home", *svc.setCalls[0])
	var body struct {
		Data struct {
			Slug *string `json:"slug"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data.Slug)
	assert.Equal(t, "home", *body.Data.Slug)
}

func TestUpdateTenant_SlugClearStoresNull(t *testing.T) {
	svc := newSlugService()
	w := putTenant(t, slugTestEngine(svc, true), `{"slug":""}`)
	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.setCalls, 1)
	assert.Nil(t, svc.setCalls[0])
}

func TestGetTenant_ResponseCarriesSlug(t *testing.T) {
	svc := newSlugService()
	slug := "home"
	svc.tenant.Slug = &slug
	req := httptest.NewRequest(http.MethodGet, "/tenants/7", nil)
	w := httptest.NewRecorder()
	slugTestEngine(svc, false).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			Slug *string `json:"slug"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data.Slug)
	assert.Equal(t, "home", *body.Data.Slug)
}

func TestUpdateTenant_SlugIsNotWrittenWhenTheUpdateFails(t *testing.T) {
	svc := newSlugService()
	svc.updateErr = errors.New("storage bucket in use")
	w := putTenant(t, slugTestEngine(svc, true), `{"slug":"home","name":"Renamed"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, svc.setCalls)
}
