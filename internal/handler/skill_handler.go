package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/access"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	// Referenced only by the @Failure annotations (errors.ErrorResponse); swag
	// resolves annotation types through the file's imports.
	_ "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// usableSkillLister returns the installed skills a chat turn can actually
// invoke on one sandbox config. The @ picker and the agent editor both read
// this set so they cannot offer a skill the running image does not carry.
type usableSkillLister interface {
	ListUsableSkills(ctx context.Context, tenantID uint64, configID string) []*types.TenantSkillEntity
}

// SkillHandler handles skill-related HTTP requests
type SkillHandler struct {
	usableSkills usableSkillLister
	catalog      skillCatalogService
	// agents resolves a shared agent so the @ picker can list the skills that
	// agent can actually invoke, which live in ITS OWNER's workspace. Nil
	// disables the shared-agent path (and with it the @Skill picker for shared
	// agents) rather than falling back to the caller's own workspace.
	agents access.SharedAgentLookup
}

type skillCatalogService interface {
	ListCatalog(ctx context.Context, tenantID uint64) ([]service.SkillCatalogView, error)
	RegisterCatalogFromArchive(ctx context.Context, tenantID uint64, archive []byte) (*types.TenantSkillCatalogEntity, error)
	RegisterCatalogFromSource(ctx context.Context, tenantID uint64, source string) (*types.TenantSkillCatalogEntity, error)
	InstallCatalogToConfigs(ctx context.Context, tenantID uint64, catalogID string, configIDs []string) (*service.CatalogInstallResult, error)
	DeleteCatalog(ctx context.Context, tenantID uint64, catalogID string) error
	ListCatalogFiles(ctx context.Context, tenantID uint64, catalogID string) ([]service.SkillFileEntry, error)
	ReadCatalogFile(ctx context.Context, tenantID uint64, catalogID, relativePath string) (*service.SkillFileContent, error)
}

// NewSkillHandler creates a new skill handler. catalog may be nil in tests
// that only exercise the chat picker.
func NewSkillHandler(
	usableSkills usableSkillLister,
	catalog skillCatalogService,
	agents access.SharedAgentLookup,
) *SkillHandler {
	return &SkillHandler{
		usableSkills: usableSkills,
		catalog:      catalog,
		agents:       agents,
	}
}

// SkillInfoResponse represents the skill info returned to frontend
type SkillInfoResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListSkills godoc
// @Summary      List the skills runnable on a sandbox config
// @Description  Return the installed skills (ready and enabled) in the given sandbox config's image that an agent can
// @Description  actually call. Without sandbox_config_id the list is empty.
// @Tags         Skills
// @Accept       json
// @Produce      json
// @Param        sandbox_config_id      query  string  false  "Sandbox config ID; ignored for a shared agent"
// @Param        agent_id               query  string  false  "Agent ID; needs agent_source_tenant_id"
// @Param        agent_source_tenant_id query  int     false  "Shared agent source workspace"
// @Success      200  {object}  map[string]interface{}  "Skill list"
// @Failure      403  {object}  errors.ErrorResponse         "Not allowed to use this shared agent"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /skills [get]
func (h *SkillHandler) ListSkills(c *gin.Context) {
	// A shared agent runs its skills in its OWNER's workspace, on the sandbox
	// config that agent selected. Reading either from the caller's workspace
	// (or from the query string) would list nothing, which is why the @Skill
	// picker used to come up empty for every shared agent.
	agent, err := sharedAgentPickerScope(c, h.agents)
	if err != nil {
		_ = c.Error(err)
		return
	}

	configID := c.Query("sandbox_config_id")
	tenantID := sandboxConfigTenantID(c)
	// allowed nil means "no name filter"; anyAllowed false means the agent can
	// invoke no skill at all, so nothing is looked up.
	var allowed map[string]bool
	anyAllowed := true
	if agent != nil {
		// Take the config from the agent, never from the query: the caller
		// could otherwise name any config id in the owner's workspace and
		// enumerate skills the shared agent does not use.
		configID = agent.Config.SandboxConfigID
		tenantID = agent.TenantID
		allowed, anyAllowed = sharedAgentSkillScope(agent)
	}

	if configID == "" || tenantID == 0 || !anyAllowed || h.usableSkills == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"data":             []SkillInfoResponse{},
			"skills_available": false,
		})
		return
	}

	rows := h.usableSkills.ListUsableSkills(c.Request.Context(), tenantID, configID)
	response := make([]SkillInfoResponse, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		if allowed != nil && !allowed[row.Name] {
			continue
		}
		response = append(response, SkillInfoResponse{
			Name:        row.Name,
			Description: row.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"data":             response,
		"skills_available": true,
	})
}
