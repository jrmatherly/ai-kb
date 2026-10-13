package router

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/dig"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/handler"
	"github.com/jrmatherly/ai-kb/internal/handler/session"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/mcpserver"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/tracing/langfuse"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"

	_ "github.com/jrmatherly/ai-kb/docs" // swagger docs
)

// RouterParams holds the router dependencies
type RouterParams struct {
	dig.In

	Config                       *config.Config
	FileService                  interfaces.FileService
	UserService                  interfaces.UserService
	KBService                    interfaces.KnowledgeBaseService
	KnowledgeService             interfaces.KnowledgeService
	ChunkService                 interfaces.ChunkService
	SessionService               interfaces.SessionService
	MessageService               interfaces.MessageService
	ModelService                 interfaces.ModelService
	EvaluationService            interfaces.EvaluationService
	KBShareService               interfaces.KBShareService
	AgentShareService            interfaces.AgentShareService
	KBHandler                    *handler.KnowledgeBaseHandler
	KnowledgeHandler             *handler.KnowledgeHandler
	TenantHandler                *handler.TenantHandler
	TenantService                interfaces.TenantService
	TenantAPIKeyService          interfaces.TenantAPIKeyService
	TenantMemberService          interfaces.TenantMemberService
	TenantMemberHandler          *handler.TenantMemberHandler
	TenantInvitationHandler      *handler.TenantInvitationHandler
	AuditLogHandler              *handler.AuditLogHandler
	AuditLogService              interfaces.AuditLogService
	ChunkHandler                 *handler.ChunkHandler
	SessionHandler               *session.Handler
	MessageHandler               *handler.MessageHandler
	MessageSuggestionHandler     *handler.MessageSuggestionHandler
	ModelHandler                 *handler.ModelHandler
	ModelCredentialsHandler      *handler.ModelCredentialsHandler
	SandboxConfigHandler         *handler.SandboxConfigHandler
	SandboxSkillHandler          *handler.SandboxSkillHandler
	MeEnvVarHandler              *handler.MeEnvVarHandler
	EvaluationHandler            *handler.EvaluationHandler
	AuthHandler                  *handler.AuthHandler
	InitializationHandler        *handler.InitializationHandler
	SystemHandler                *handler.SystemHandler
	MCPServiceHandler            *handler.MCPServiceHandler
	MCPCredentialsHandler        *handler.MCPCredentialsHandler
	MCPOAuthHandler              *handler.MCPOAuthHandler
	WebSearchHandler             *handler.WebSearchHandler
	WebSearchProviderHandler     *handler.WebSearchProviderHandler
	WebSearchCredentialsHandler  *handler.WebSearchProviderCredentialsHandler
	VectorStoreHandler           *handler.VectorStoreHandler
	StorageBackendHandler        *handler.StorageBackendHandler
	StorageBackendResolver       interfaces.StorageBackendResolver
	ResourceCatalog              interfaces.ResourceCatalog
	FAQHandler                   *handler.FAQHandler
	TagHandler                   *handler.TagHandler
	CustomAgentHandler           *handler.CustomAgentHandler
	UserFavoriteHandler          *handler.UserResourceFavoriteHandler
	SkillHandler                 *handler.SkillHandler
	OrganizationHandler          *handler.OrganizationHandler
	IMHandler                    *handler.IMHandler
	EmbedChannelHandler          *handler.EmbedChannelHandler
	EmbedChannelService          interfaces.EmbedChannelService
	MCPEndpointHandler           *handler.MCPEndpointHandler
	MCPEndpointService           interfaces.MCPEndpointService
	MCPServer                    *mcpserver.Server
	RedisClient                  *redis.Client
	DataSourceHandler            *handler.DataSourceHandler
	DataSourceCredentialsHandler *handler.DataSourceCredentialsHandler
	WeKnoraCloudHandler          *handler.WeKnoraCloudHandler
	WikiPageHandler              *handler.WikiPageHandler
	MemoryHandler                *handler.MemoryHandler
	HostSandbox                  service.HostSandboxManager
}

// NewRouter creates the router
func NewRouter(params RouterParams) *gin.Engine {
	r := gin.New()
	r.ContextWithFallback = true
	// Remove the multipart temp files left by FormFile/MultipartForm parsing, so the container's /tmp does not keep
	// growing.
	r.Use(middleware.MultipartFormCleanup())

	// Trusted proxies: gin defaults to trusting ALL proxies, which makes
	// c.ClientIP() honor a client-supplied X-Forwarded-For. Public, unauthed
	// embed endpoints rate-limit per (channel, ClientIP), so a spoofed XFF would
	// trivially bypass the limiter. Restrict to the fronting proxy network so
	// only the real client IP (appended by nginx) is returned. Configurable via
	// AIKB_TRUSTED_PROXIES (comma-separated CIDRs/IPs).
	if err := r.SetTrustedProxies(trustedProxies()); err != nil {
		logger.Errorf(context.Background(), "[Router] failed to set trusted proxies: %v", err)
	}

	// The CORS middleware goes first.
	// Note: with a wildcard AllowOrigins the browser rejects every credentialed (cookie) cross-origin
	// request (the CORS spec forbids combining "*" with credentials), so AllowCredentials
	// only takes effect if this is later changed to echo a specific Origin. All authentication currently uses explicit
	// Authorization / X-API-Key headers and relies on no ambient credentials. Before introducing cookie
	// authentication, AllowOrigins must become a controlled list.
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key", "X-Request-ID", "X-Tenant-ID",
			"X-Embed-Session", "X-External-User-ID", "X-External-User-Token", "X-AIKB-Desktop-Token",
			// Streamable HTTP MCP clients running in a browser send these on
			// the /mcp/:endpoint_id surface.
			"MCP-Protocol-Version", "Mcp-Method", "Mcp-Name", "Mcp-Session-Id", "Last-Event-ID",
		},
		ExposeHeaders: []string{
			"Content-Length", "Content-Disposition", "Access-Control-Allow-Origin", "Mcp-Session-Id",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Base middleware (no authentication)
	r.Use(middleware.RequestID())
	r.Use(middleware.Language())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.ErrorHandler())

	// Health check (no authentication)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger API docs (only outside production)
	// Decided by the GIN_MODE environment variable: Swagger is disabled in release mode
	if gin.Mode() != gin.ReleaseMode {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler,
			ginSwagger.DefaultModelsExpandDepth(-1), // collapse Models by default
			ginSwagger.DocExpansion("list"),         // "list" expands tags, "full" everything, "none" nothing
			ginSwagger.DeepLinking(true),            // enable deep linking
			ginSwagger.PersistAuthorization(true),   // persist authorization
		))
	}

	// Embed page framing policy: emit a per-channel `frame-ancestors` CSP so the
	// embed SPA page (/embed/:channelId) can only be iframed by the channel's
	// allowed origins. This is the page-level counterpart to the API Origin
	// allowlist enforced in EmbedAuth. Registered before the static handler so
	// it runs for the embed HTML response.
	if params.EmbedChannelService != nil {
		r.Use(embedFrameAncestorsMiddleware(params.EmbedChannelService))
	}

	// Frontend static files (only the Lite build embeds the frontend)
	if handler.Edition == "lite" {
		serveFrontendStatic(r)
	}

	// IM callback routes (registered before the auth middleware; each platform's own signature verification applies)
	RegisterIMRoutes(r, params.IMHandler)

	// Public web embed routes (authenticated by publish token, not the global Auth)
	RegisterEmbedPublicRoutes(
		r,
		params.EmbedChannelHandler,
		params.EmbedChannelService,
		params.TenantService,
		params.RedisClient,
		params.FileService,
		params.StorageBackendResolver,
		params.ResourceCatalog,
	)

	// Workspace MCP server surface (/mcp/:endpoint_id): bearer-token auth per
	// endpoint, so it must precede the global Auth middleware.
	RegisterMCPServerRoutes(r, params.MCPServer, params.MCPEndpointService, params.TenantService)

	// Short-lived capability URLs for IM and other clients that cannot attach
	// AI KB authentication headers.
	serveResourceGrants(r, params.ResourceCatalog, params.TenantService, params.FileService, params.StorageBackendResolver)

	// Sandbox terminal WebSocket (self-authenticated via a short-lived
	// query ticket - see RegisterSandboxTerminalRoutes; browsers cannot set
	// auth headers on the WS handshake, so this must precede the global Auth
	// middleware). The ticket is minted by an authenticated POST.
	RegisterSandboxTerminalRoutes(r, params.SessionHandler)
	RegisterSandboxDesktopRoutes(r, params.SessionHandler)
	r.GET("/api/v1/local-browser/extension", params.SessionHandler.BrowserSkillExtension)
	r.POST("/api/v1/local-browser/extension/authorize", params.SessionHandler.BrowserSkillAuthorize)
	r.POST("/api/v1/local-browser/internal", params.SessionHandler.BrowserSkillInternal)

	// Authentication middleware
	r.Use(middleware.Auth(params.TenantService, params.UserService, params.TenantMemberService, params.TenantAPIKeyService, params.Config))

	// File service: a single proxy for the local/MinIO/COS/TOS storage backends (authentication required)
	serveFilesWithResources(r, params.FileService, params.StorageBackendResolver, params.ResourceCatalog)

	// Presigned file access: no auth required, signature-verified.
	servePresignedFiles(r, params.TenantService, params.StorageBackendResolver)

	// Diagnostic preview of presigned URLs (Admin only, behind auth middleware).
	servePresignedPreview(r, params.Config, params.StorageBackendResolver, params.ResourceCatalog)

	// Langfuse observability - only active when LANGFUSE_* env vars are set.
	// The middleware is registered unconditionally; when disabled it's a no-op.
	r.Use(langfuse.GinMiddleware())

	// Audit log injection - middleware/rbac.go's reject paths and the
	// admin-only /tenants/:id/audit-log endpoint pull the service out
	// of the gin context. Provider is a no-op when AuditLogService is
	// nil (e.g. lite mode without DB), so the rbac path degrades to
	// "log to stderr only" instead of crashing.
	r.Use(middleware.AuditServiceProvider(params.AuditLogService))

	// API routes that require authentication
	v1 := r.Group("/api/v1")
	{
		// rbacGuards bundles the role-gating middleware factories so each
		// Register* function below can attach the right guard without
		// taking a *config.Config dependency directly. The guards honour
		// cfg.Tenant.EnableRBAC: when false, they log but pass through,
		// preserving today's behaviour during the rollout window.
		rbacGuards := newRBACGuards(
			params.Config,
			params.KBHandler,
			params.CustomAgentHandler,
			params.KnowledgeHandler,
			params.ChunkHandler,
			params.WikiPageHandler,
			params.KBService,
			params.KnowledgeService,
			params.ChunkService,
			params.KBShareService,
			params.AgentShareService,
		)

		// API-key gate: single authority for X-API-Key principals. Runs
		// first on every /api/v1 route (JWT sessions pass straight
		// through) and denies any route not explicitly declared via the
		// apiKeyGroup helpers. Must be attached BEFORE the Register* calls
		// so that sub-groups inherit it.
		v1.Use(rbacGuards.apiKeyAuthorizer.Middleware())

		RegisterAuthRoutes(v1, params.AuthHandler, rbacGuards)
		RegisterTenantRoutes(v1, params.TenantHandler, params.TenantMemberHandler, params.TenantInvitationHandler, params.AuditLogHandler, rbacGuards)
		RegisterMyInvitationRoutes(v1, params.TenantInvitationHandler)
		RegisterKnowledgeBaseRoutes(v1, params.KBHandler, rbacGuards)
		RegisterImageAttrRoutes(v1, params.KBHandler, rbacGuards)
		RegisterKnowledgeBaseActivityRoutes(v1, params.AuditLogHandler, rbacGuards)
		// KB-scoped image proxy: lets tenants render images embedded in
		// org-shared / agent-visible KB content, which the tenant-scoped
		// /files route cannot serve because it enforces same-tenant paths.
		serveKBScopedFiles(
			v1,
			rbacGuards,
			params.TenantService,
			params.FileService,
			params.StorageBackendResolver,
			params.ResourceCatalog,
		)
		// Message-scoped image proxy: shared-agent replies belong to the
		// caller's session but may reference resources stored in the agent's
		// source workspace. Authorization is derived from the persisted message,
		// never from a client-provided workspace ID. Replies produced by the
		// caller's own agent over an org-shared KB fall back to the KB share
		// relation instead (#3022).
		serveMessageScopedFiles(
			v1,
			rbacGuards,
			params.MessageService,
			params.AgentShareService,
			params.TenantService,
			params.FileService,
			params.StorageBackendResolver,
			params.ResourceCatalog,
			params.KBShareService,
			params.KBService,
			params.KnowledgeService,
		)
		RegisterKnowledgeTagRoutes(v1, params.TagHandler, rbacGuards)
		RegisterKnowledgeRoutes(v1, params.KnowledgeHandler, rbacGuards)
		RegisterFAQRoutes(v1, params.FAQHandler, rbacGuards)
		RegisterChunkRoutes(v1, params.ChunkHandler, rbacGuards)
		RegisterSessionRoutes(v1, params.SessionHandler, params.MessageSuggestionHandler, rbacGuards)
		RegisterChatRoutes(v1, params.SessionHandler, rbacGuards)
		RegisterMessageRoutes(v1, params.MessageHandler, rbacGuards)
		RegisterModelRoutes(v1, params.ModelHandler, params.ModelCredentialsHandler, rbacGuards)
		RegisterSandboxConfigRoutes(v1, params.SandboxConfigHandler, params.SandboxSkillHandler, rbacGuards)
		RegisterMyEnvVarRoutes(v1, params.MeEnvVarHandler)
		v1.GET("/me/browser", params.SessionHandler.BrowserSkillAccount)
		v1.GET("/me/browser/extension", params.SessionHandler.BrowserSkillDownload)
		v1.POST("/me/browser", params.SessionHandler.BrowserSkillAccount)
		RegisterEvaluationRoutes(v1, params.EvaluationHandler, rbacGuards)
		RegisterInitializationRoutes(v1, params.InitializationHandler, rbacGuards)
		params.SystemHandler.BindDeploymentCapabilities(deploymentCapabilitiesFromRouter(params))
		RegisterSystemRoutes(v1, params.SystemHandler, rbacGuards)
		RegisterSystemAdminRoutes(v1, params.SystemHandler, params.AuditLogHandler, rbacGuards)
		RegisterMCPServiceRoutes(v1, params.MCPServiceHandler, params.MCPCredentialsHandler, params.MCPOAuthHandler, rbacGuards)
		RegisterWebSearchRoutes(v1, params.WebSearchHandler, rbacGuards)
		RegisterWebSearchProviderRoutes(v1, params.WebSearchProviderHandler, params.WebSearchCredentialsHandler, rbacGuards)
		RegisterVectorStoreRoutes(v1, params.VectorStoreHandler, rbacGuards)
		RegisterStorageBackendRoutes(v1, params.StorageBackendHandler, rbacGuards)
		RegisterCustomAgentRoutes(v1, params.CustomAgentHandler, rbacGuards)
		RegisterUserFavoriteRoutes(v1, params.UserFavoriteHandler, rbacGuards)
		RegisterSkillRoutes(v1, params.SkillHandler, rbacGuards)
		RegisterOrganizationRoutes(v1, params.OrganizationHandler, rbacGuards)
		RegisterIMChannelRoutes(v1, params.IMHandler, rbacGuards)
		RegisterEmbedChannelRoutes(v1, params.EmbedChannelHandler, rbacGuards)
		RegisterMCPEndpointRoutes(v1, params.MCPEndpointHandler, rbacGuards)
		RegisterDataSourceRoutes(v1, params.DataSourceHandler, params.DataSourceCredentialsHandler, rbacGuards)
		RegisterWeKnoraCloudRoutes(v1, params.WeKnoraCloudHandler, rbacGuards)
		RegisterWikiPageRoutes(v1, params.WikiPageHandler, rbacGuards)
		RegisterMemoryRoutes(v1, params.MemoryHandler, rbacGuards)
		RegisterChunkerDebugRoutes(v1, rbacGuards)

		// Fail fast if any declared API-key policy points at a route
		// template that does not actually exist (typo / path drift). A
		// stale template would silently 403 every API key on that route,
		// so we panic at startup instead of shipping a dead policy.
		rbacGuards.assertAPIKeyPoliciesMatchRoutes(r)
	}

	return r
}

// trustedProxies returns the proxy CIDRs/IPs whose X-Forwarded-For headers
// gin should trust when resolving the client IP. Defaults to loopback and
// private ranges (covers the bundled nginx in a container network); override
// with AIKB_TRUSTED_PROXIES (comma-separated). An explicit empty value
// disables proxy trust entirely so ClientIP() returns the direct peer.
func trustedProxies() []string {
	raw, ok := os.LookupEnv("AIKB_TRUSTED_PROXIES")
	if !ok {
		return []string{
			"127.0.0.0/8",
			"::1/128",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"fc00::/7",
		}
	}
	proxies := make([]string, 0)
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return proxies
}
