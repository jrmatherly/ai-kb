package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmitGroupSynced(t *testing.T) {
	audit := &recordingAudit{}
	h := &AuthHandler{auditSvc: audit}
	user := &types.User{ID: "u1"}
	tenant := &types.Tenant{ID: 20}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback", nil)

	h.emitGroupSynced(context.Background(), c, &types.OIDCCallbackResponse{User: user, Tenant: tenant})
	assert.Empty(t, audit.entries, "no row without a sync")

	sync := &types.OIDCGroupSync{Added: []uint64{20}, Admin: "granted", Skipped: []string{}}
	h.emitGroupSynced(context.Background(), c, &types.OIDCCallbackResponse{User: user, Tenant: tenant, GroupSync: sync})
	row := audit.only(t)
	assert.Equal(t, types.AuditActionOIDCGroupSync, row.Action)
	assert.Equal(t, "u1", row.ActorUserID)
	assert.Equal(t, uint64(20), row.TenantID)
	assert.Equal(t, types.AuditOutcomeSuccess, row.Outcome)
	assert.Equal(t, http.MethodGet, row.RequestMethod)
	var details map[string]any
	require.NoError(t, json.Unmarshal(row.Details, &details))
	assert.Equal(t, "granted", details["admin"])

	audit.entries = nil
	sync.Skipped = []string{"last_owner:11"}
	h.emitGroupSynced(context.Background(), c, &types.OIDCCallbackResponse{User: user, Tenant: tenant, GroupSync: sync})
	assert.Equal(t, types.AuditOutcomePartial, audit.only(t).Outcome)
}
