package handler

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// emitAudit writes one audit row for a handler-level event. A nil service
// (legacy test fixtures) is a no-op; a failed write is logged, never
// surfaced, since the user's action already succeeded. Actor, role, tenant
// and outcome default from the request context when the caller leaves them
// empty.
func emitAudit(ctx context.Context, svc interfaces.AuditLogService, entry *types.AuditLog) {
	if svc == nil || entry == nil {
		return
	}
	if entry.ActorUserID == "" {
		entry.ActorUserID, _ = types.UserIDFromContext(ctx)
	}
	if entry.ActorRole == "" {
		entry.ActorRole = string(types.TenantRoleFromContext(ctx))
	}
	if entry.TenantID == 0 {
		entry.TenantID, _ = types.TenantIDFromContext(ctx)
	}
	if entry.Outcome == "" {
		entry.Outcome = types.AuditOutcomeSuccess
	}
	if err := svc.Log(ctx, entry); err != nil {
		logger.Warnf(ctx, "audit: failed to record %s: %v", entry.Action, err)
	}
}

// changeDetails is the details payload of a configuration change: the
// operation and the names of the fields it touched, never their values.
func changeDetails(op string, fields ...string) types.JSON {
	payload := map[string]any{"op": op}
	if len(fields) > 0 {
		payload["fields"] = fields
	}
	raw, _ := json.Marshal(payload)
	return types.JSON(raw)
}

// changeRow builds a configuration-change audit row for one target.
func changeRow(action types.AuditAction, targetType, targetID, op string, fields ...string) *types.AuditLog {
	return &types.AuditLog{
		Action: action, TargetType: targetType, TargetID: targetID, Details: changeDetails(op, fields...),
	}
}

// auditFieldNames lists the keys of an update-fields map in stable order.
func auditFieldNames(fields map[string]bool) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
