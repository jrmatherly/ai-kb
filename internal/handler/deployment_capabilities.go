package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/sandbox"
)

// DeploymentCapabilityKeys is the canonical capability key list shared with
// frontend/src/config/deploymentCapabilities.ts - keep both in sync.
var DeploymentCapabilityKeys = []string{
	"organizations",
	"agents",
	"integrations.im",
	"integrations.embed",
	"integrations.api",
	"integrations.mcpserver",
	"settings.mcp",
	"settings.websearch",
	"settings.vectorstore",
	"settings.storage",
	"settings.sandbox",
	"settings.sandbox.docker",
	"settings.sandbox.host",
	"settings.sandbox.remote",
	"settings.weknoracloud",
	"settings.ollama",
}

// OllamaEnabledEnv shows the Ollama settings section. Off by default.
const OllamaEnabledEnv = "AIKB_OLLAMA_ENABLED"

// OllamaEnabled reads OllamaEnabledEnv at call time; unset or unparsable is false.
func OllamaEnabled() bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(OllamaEnabledEnv)))
	return err == nil && parsed
}

// DeploymentCapability describes whether a deployment exposes a feature route.
type DeploymentCapability struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
}

// DeploymentCapabilitiesData is returned by GET /system/capabilities.
type DeploymentCapabilitiesData struct {
	Edition      string                          `json:"edition"`
	Capabilities map[string]DeploymentCapability `json:"capabilities"`
	// IMPlatforms is the deployment's AIKB_IM_PLATFORMS allow-list. A client
	// that does not find the field (an older backend) offers every platform.
	IMPlatforms []string `json:"im_platforms"`
}

// DeploymentFeatureAvailability mirrors injected backend handlers/services.
type DeploymentFeatureAvailability struct {
	Organizations bool
	Agents        bool
	IM            bool
	IMPlatforms   []string
	Embed         bool
	API           bool
	MCPServer     bool
	MCP           bool
	WebSearch     bool
	VectorStore   bool
	Storage       bool
	Sandbox       bool
	SandboxDocker bool
	SandboxHost   bool
	SandboxRemote bool
	WeKnoraCloud  bool
	Ollama        bool
}

func supportedDeploymentCapability(supported bool) DeploymentCapability {
	if supported {
		return DeploymentCapability{Supported: true}
	}
	return DeploymentCapability{Supported: false, Reason: "route_not_registered"}
}

// BuildDeploymentCapabilities derives the deployment capability snapshot.
func BuildDeploymentCapabilities(
	edition string,
	available DeploymentFeatureAvailability,
) DeploymentCapabilitiesData {
	isLite := strings.EqualFold(strings.TrimSpace(edition), "lite")
	organizations := supportedDeploymentCapability(available.Organizations && !isLite)
	if isLite {
		organizations.Reason = "not_supported_in_lite"
	}

	sandboxDocker := DeploymentCapability{
		Supported: available.Sandbox && available.SandboxDocker,
	}
	if available.Sandbox && !available.SandboxDocker {
		sandboxDocker.Reason = "docker_backend_disabled"
	} else if !available.Sandbox {
		sandboxDocker.Reason = "route_not_registered"
	}

	sandboxHost := DeploymentCapability{
		Supported: available.Sandbox && available.SandboxHost,
	}
	if available.Sandbox && !available.SandboxHost {
		// The platform has no OS-enforced backend (Windows/Linux today).
		sandboxHost.Reason = "platform_unsupported"
	} else if !available.Sandbox {
		sandboxHost.Reason = "route_not_registered"
	}

	sandboxRemote := DeploymentCapability{Supported: available.Sandbox && available.SandboxRemote}
	if available.Sandbox && !available.SandboxRemote {
		sandboxRemote.Reason = "not_supported_in_lite"
	} else if !available.Sandbox {
		sandboxRemote.Reason = "route_not_registered"
	}

	imPlatforms := available.IMPlatforms
	if imPlatforms == nil {
		imPlatforms = []string{}
	}
	imCap := supportedDeploymentCapability(available.IM)
	if available.IM && len(imPlatforms) == 0 {
		imCap = DeploymentCapability{Supported: false, Reason: "no_platform_enabled"}
	}

	weKnoraCloud := DeploymentCapability{Supported: available.WeKnoraCloud}
	if !available.WeKnoraCloud {
		weKnoraCloud.Reason = "disabled_by_deployment"
	}

	ollama := DeploymentCapability{Supported: available.Ollama}
	if !available.Ollama {
		ollama.Reason = "disabled_by_deployment"
	}

	return DeploymentCapabilitiesData{
		Edition:     edition,
		IMPlatforms: imPlatforms,
		Capabilities: map[string]DeploymentCapability{
			"organizations":           organizations,
			"agents":                  supportedDeploymentCapability(available.Agents),
			"integrations.im":         imCap,
			"integrations.embed":      supportedDeploymentCapability(available.Embed),
			"integrations.api":        supportedDeploymentCapability(available.API),
			"integrations.mcpserver":  supportedDeploymentCapability(available.MCPServer),
			"settings.mcp":            supportedDeploymentCapability(available.MCP),
			"settings.websearch":      supportedDeploymentCapability(available.WebSearch),
			"settings.vectorstore":    supportedDeploymentCapability(available.VectorStore),
			"settings.storage":        supportedDeploymentCapability(available.Storage),
			"settings.sandbox":        supportedDeploymentCapability(available.Sandbox),
			"settings.sandbox.docker": sandboxDocker,
			"settings.sandbox.host":   sandboxHost,
			"settings.sandbox.remote": sandboxRemote,
			"settings.weknoracloud":   weKnoraCloud,
			"settings.ollama":         ollama,
		},
	}
}

// BindDeploymentCapabilities stores the startup snapshot used by GetDeploymentCapabilities.
func (h *SystemHandler) BindDeploymentCapabilities(data DeploymentCapabilitiesData) {
	h.deploymentCapabilities = data
}

// GetDeploymentCapabilities godoc
// @Summary      Get deployment capabilities
// @Description  Returns the deployed version and the capabilities backed by the registered backend routes; only
// @Description  supported=false means the entry point should be hidden
// @Tags         System
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "code/msg/data envelope; data is DeploymentCapabilitiesData"
// @Router       /system/capabilities [get]
func (h *SystemHandler) GetDeploymentCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": overlayLiveDockerSandboxCapability(h.deploymentCapabilities),
	})
}

// overlayLiveDockerSandboxCapability replaces the startup snapshot's Docker
// flag with the live 3-tier value so a System Settings toggle is visible
// without restarting the process.
func overlayLiveDockerSandboxCapability(data DeploymentCapabilitiesData) DeploymentCapabilitiesData {
	caps := make(map[string]DeploymentCapability, len(data.Capabilities))
	for key, capability := range data.Capabilities {
		caps[key] = capability
	}
	sandboxCap := caps["settings.sandbox"]
	docker := DeploymentCapability{
		Supported: sandboxCap.Supported && sandbox.DockerBackendEnabled(),
	}
	if sandboxCap.Supported && !docker.Supported {
		docker.Reason = "docker_backend_disabled"
	} else if !sandboxCap.Supported {
		docker.Reason = "route_not_registered"
	}
	caps["settings.sandbox.docker"] = docker
	data.Capabilities = caps
	return data
}
