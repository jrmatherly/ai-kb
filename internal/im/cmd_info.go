package im

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// InfoCommand implements /info.
// It shows the bound agent's profile and capabilities so IM users can
// understand what the bot can do without leaving the chat.
type InfoCommand struct {
	kbService interfaces.KnowledgeBaseService
}

func newInfoCommand(kbService interfaces.KnowledgeBaseService) *InfoCommand {
	return &InfoCommand{kbService: kbService}
}

// Name returns the command keyword.
func (c *InfoCommand) Name() string { return "info" }

// Description returns the help text shown for /info.
func (c *InfoCommand) Description() string { return "Show the bound agent's profile and capabilities" }

func (c *InfoCommand) Execute(ctx context.Context, cmdCtx *CommandContext, _ []string) (*CommandResult, error) {
	var sb strings.Builder

	// Note: Feishu card markdown only renders **bold** when it occupies the
	// entire inline segment. "**label:**value" on the same line will show
	// raw asterisks. Always keep bold text self-contained on its own line.

	// ── Header ──
	name := cmdCtx.AgentName
	if name == "" {
		name = "Untitled agent"
	}
	sb.WriteString(fmt.Sprintf("🤖 **%s**\n", name))
	if cmdCtx.CustomAgent != nil && cmdCtx.CustomAgent.Description != "" {
		sb.WriteString(fmt.Sprintf("> %s\n", cmdCtx.CustomAgent.Description))
	}

	if cmdCtx.CustomAgent == nil {
		sb.WriteString("\nNo agent is bound. Send `/help` to see available commands.")
		return &CommandResult{Content: sb.String()}, nil
	}

	cfg := cmdCtx.CustomAgent.Config

	// ── Mode ──
	if cmdCtx.CustomAgent.IsAgentMode() {
		sb.WriteString("\n🧠 **Agent mode**\n")
		sb.WriteString("Multi-step reasoning and tool calls (ReAct)\n")
	} else {
		sb.WriteString("\n🧠 **Agent mode**\n")
		sb.WriteString("Answers directly from knowledge base retrieval (RAG)\n")
	}

	// ── Knowledge bases ──
	// KBSelectionMode: "all" uses every KB under the tenant (IDs list is empty),
	// "selected" uses the explicit KnowledgeBases list, "none"/empty means disabled.
	sb.WriteString("\n📚 **Knowledge bases**\n")
	if cfg.KBSelectionMode == "all" {
		kbs, err := c.kbService.ListKnowledgeBasesByTenantID(ctx, cmdCtx.TenantID)
		if err == nil && len(kbs) > 0 {
			for _, kb := range kbs {
				sb.WriteString(fmt.Sprintf("  · %s\n", kb.Name))
			}
			sb.WriteString(fmt.Sprintf("  %d total (all enabled)\n", len(kbs)))
		} else {
			sb.WriteString("  All enabled\n")
		}
	} else if len(cfg.KnowledgeBases) > 0 {
		kbs, err := c.kbService.ListKnowledgeBasesByTenantID(ctx, cmdCtx.TenantID)
		if err == nil {
			nameMap := make(map[string]string, len(kbs))
			for _, kb := range kbs {
				nameMap[kb.ID] = kb.Name
			}
			for _, id := range cfg.KnowledgeBases {
				label := id
				if n, ok := nameMap[id]; ok {
					label = n
				}
				sb.WriteString(fmt.Sprintf("  · %s\n", label))
			}
		} else {
			sb.WriteString(fmt.Sprintf("  %d selected\n", len(cfg.KnowledgeBases)))
		}
	} else {
		sb.WriteString("  Not configured\n")
	}

	// ── Skills ──
	sb.WriteString("\n⚡ **Skills**\n")
	if cfg.SkillsSelectionMode == "all" {
		sb.WriteString("  All enabled\n")
	} else if cfg.SkillsSelectionMode == "selected" && len(cfg.SelectedSkills) > 0 {
		for _, s := range cfg.SelectedSkills {
			sb.WriteString(fmt.Sprintf("  · %s\n", s))
		}
	} else {
		sb.WriteString("  Not configured\n")
	}

	// ── MCP ──
	sb.WriteString("\n🔌 **MCP services**\n")
	if cfg.MCPSelectionMode == "all" {
		sb.WriteString("  All connected\n")
	} else if cfg.MCPSelectionMode == "selected" && len(cfg.MCPServices) > 0 {
		sb.WriteString(fmt.Sprintf("  %d service(s) connected\n", len(cfg.MCPServices)))
	} else {
		sb.WriteString("  Not configured\n")
	}

	// ── Web search ──
	sb.WriteString("\n🌐 **Web search**\n")
	if cfg.WebSearchEnabled {
		sb.WriteString("  Enabled\n")
	} else {
		sb.WriteString("  Disabled\n")
	}

	// ── Footer ──
	outputLabel := "Streaming"
	if cmdCtx.ChannelOutputMode == "full" {
		outputLabel = "Full output"
	}
	sb.WriteString(fmt.Sprintf("\n⚙️ **Output mode**\n  %s\n", outputLabel))
	sb.WriteString("\n---\nSend `/help` to see all available commands")

	return &CommandResult{Content: sb.String()}, nil
}
