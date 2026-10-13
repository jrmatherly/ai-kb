package im

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func TestRelocalizeBuiltinChannelAgentNames(t *testing.T) {
	restore := types.OverrideBuiltinAgentEntriesForTest(map[string]*types.BuiltinAgentEntry{
		types.BuiltinQuickAnswerID: {
			ID:     types.BuiltinQuickAnswerID,
			Avatar: "quick.png",
			I18n: map[string]types.BuiltinAgentI18n{
				"default": {Name: "\u5feb\u901f\u95ee\u7b54"},
				"en-US":   {Name: "Quick Answer"},
				"zh-CN":   {Name: "\u5feb\u901f\u95ee\u7b54"},
			},
		},
	})
	t.Cleanup(restore)

	rows := []ChannelWithAgent{
		{AgentID: "custom-1", TenantID: 1, AgentName: "Mine"},
		{AgentID: types.BuiltinQuickAnswerID, TenantID: 1, AgentName: ""},
		{AgentID: types.BuiltinQuickAnswerID, TenantID: 1, AgentName: "frozen-zh"},
	}
	ctx := context.WithValue(context.Background(), types.LanguageContextKey, "en-US")
	relocalizeBuiltinChannelAgentNames(ctx, rows)

	require.Equal(t, "Mine", rows[0].AgentName)
	require.Equal(t, "Quick Answer", rows[1].AgentName)
	require.Equal(t, "Quick Answer", rows[2].AgentName)
}
