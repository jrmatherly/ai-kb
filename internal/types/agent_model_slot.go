package types

import (
	"context"
	"fmt"
	"strings"
)

// ModelLister is the part of the model service the slot rule reads.
type ModelLister interface {
	ListModels(ctx context.Context) ([]*Model, error)
}

// AgentModelSlot returns the model an agent runs with for one model slot
// (chat model_id, rerank_model_id). A configured ID is returned as is, stale or
// not, so callers keep failing on it. An empty slot of the workspace's own
// built-in agent resolves to DeclaredDefaultModelID; every other empty slot
// stays "". The wiki fixer is excluded: its callers handle the empty result
// with their KB/system fallback.
func AgentModelSlot(
	ctx context.Context,
	models ModelLister,
	agent *CustomAgent,
	sharedReadOnly bool,
	configured string,
	modelType ModelType,
) (string, error) {
	if id := strings.TrimSpace(configured); id != "" {
		return id, nil
	}
	if agent == nil || sharedReadOnly || agent.ID == BuiltinWikiFixerID ||
		(!agent.IsBuiltin && !IsBuiltinAgentID(agent.ID)) {
		return "", nil
	}
	list, err := models.ListModels(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve default %s model for agent %s: %w", modelType, agent.ID, err)
	}
	return DeclaredDefaultModelID(list, modelType), nil
}

// DeclaredDefaultModelID returns the workspace's own active is_default model of
// modelType, else the active built-in is_default one, else "". It never picks
// an undeclared model.
func DeclaredDefaultModelID(models []*Model, modelType ModelType) string {
	builtin := ""
	for _, m := range models {
		if m == nil || m.Type != modelType || !m.IsDefault ||
			(m.Status != "" && m.Status != ModelStatusActive) {
			continue
		}
		if !m.IsBuiltin {
			return m.ID
		}
		if builtin == "" {
			builtin = m.ID
		}
	}
	return builtin
}
