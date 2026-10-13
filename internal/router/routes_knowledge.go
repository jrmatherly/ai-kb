package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/handler"
)

// RegisterChunkerDebugRoutes wires the read-only chunker preview endpoint
// used by the KB editor's debug panel. Stateless — uses no service deps.
//
// Viewer+ floor: the endpoint surfaces inside the tenant UI, so any
// authenticated tenant member can call it; revoked accounts whose JWT
// has not yet expired are kept out by the role check, matching the
// rest of the RBAC matrix in this file.
func RegisterChunkerDebugRoutes(r *gin.RouterGroup, g *rbacGuards) {
	g.apiKeyRoute(r, http.MethodPost, "/chunker/preview", apiKeyRetrieve(apiKeyIngest(apiKeyFullAccess())), g.Viewer(), handler.PreviewChunking)
}

// RegisterChunkRoutes registers the chunk routes
//
// Mutating routes addressed via :knowledge_id inherit per-KB ownership
// from the owning knowledge entry's KB (PR 5, #1303); the chain hop is
// shared with RegisterKnowledgeRoutes via OwnedChunkKBOrAdmin so the
// same "creator-of-the-KB OR Admin+" rule applies to chunk edits.
func RegisterChunkRoutes(r *gin.RouterGroup, handler *handler.ChunkHandler, g *rbacGuards) {
	// Chunk route group. Scoped API keys need the ingest capability to write content and retrieve to read it;
	// both are still bound by the KB allow-list.
	chunks := g.apiKeyGroup(r.Group("/chunks"), apiKeyIngest(apiKeyFullAccess()))
	chunkRead := chunks.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		// List chunks: Viewer+ with read access to the parent KB (own / shared / via shared agent)
		chunkRead.GET("/:knowledge_id", g.Viewer(), g.KBAccessReadFromKnowledgeIDParam("knowledge_id"), handler.ListKnowledgeChunks)
		// Get one chunk by chunk_id (no knowledge_id needed): Viewer+ with read access to the parent KB
		chunkRead.GET("/by-id/:id", g.Viewer(), g.KBAccessReadFromChunkIDParam("id"), handler.GetChunkByIDOnly)
		chunkRead.GET("/:knowledge_id/:id/revisions", g.Viewer(), g.KBAccessReadFromKnowledgeIDParam("knowledge_id"), handler.ListChunkRevisions)
		// Delete a chunk: KB owner OR Admin+, with write access to the parent KB
		chunks.DELETE("/:knowledge_id/:id", g.OwnedChunkKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("knowledge_id"), handler.DeleteChunk)
		// Delete every chunk of a knowledge item: KB owner OR Admin+, with write access to the parent KB
		chunks.DELETE("/:knowledge_id", g.OwnedChunkKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("knowledge_id"), handler.DeleteChunksByKnowledgeID)
		// Update a chunk: KB owner OR Admin+, with write access to the parent KB
		chunks.PUT("/:knowledge_id/:id", g.OwnedChunkKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("knowledge_id"), handler.UpdateChunk)
		chunks.POST("/:knowledge_id/:id/revert", g.OwnedChunkKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("knowledge_id"), handler.RevertChunk)
		// Delete one generated question (by chunk id): consistent with the other chunk mutations,
		// KB owner OR Admin+. Earlier the lookup chain (chunk_id -> knowledge_id ->
		// kb -> creator_id) was not wired yet, so this was temporarily lowered to Contributor, which made
		// this route looser than the rule that governs editing every other chunk.
		// KBCreatorLookupFromChunkIDParam now fills in that hop, so the matrix is uniform.
		chunks.DELETE("/by-id/:id/questions", g.OwnedChunkKBOrAdminFromChunkID(), g.KBAccessWriteFromChunkIDParam("id"), handler.DeleteGeneratedQuestion)
		chunks.PUT("/by-id/:id/questions", g.OwnedChunkKBOrAdminFromChunkID(), g.KBAccessWriteFromChunkIDParam("id"), handler.UpsertGeneratedQuestion)
		chunks.POST("/by-id/:id/questions/regenerate", g.OwnedChunkKBOrAdminFromChunkID(), g.KBAccessWriteFromChunkIDParam("id"), handler.RegenerateGeneratedQuestions)
	}
}

// RegisterKnowledgeRoutes registers the knowledge routes
//
// Per-KB ownership applies on the per-:id mutating routes (PR 5,
// #1303): the URL :id is a knowledge id, OwnedKnowledgeKBOrAdmin
// walks it back to KB.CreatorID so a Contributor who owns the KB can
// edit/delete any of its documents while a non-owner Contributor gets
// 403. KB-scoped upload routes (`/knowledge-bases/:id/knowledge/...`)
// reuse OwnedKBOrAdmin because the URL :id is the KB id directly.
// Body-scoped batch operations have a Contributor route gate and resolve
// their KB ownership plus Editor operation grant inside the handler.
func RegisterKnowledgeRoutes(r *gin.RouterGroup, handler *handler.KnowledgeHandler, g *rbacGuards) {
	// Knowledge routes under a knowledge base (URL :id is the KB id). Scoped API keys need
	// the ingest capability to write content and are still limited to their KB scope; clearing a KB is full-access
	// only.
	kb := g.apiKeyGroup(r.Group("/knowledge-bases/:id/knowledge"), apiKeyIngest(apiKeyFullAccess()))
	kbRead := kb.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		kb.POST("/file", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.CreateKnowledgeFromFile)
		kb.POST("/url", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.CreateKnowledgeFromURL)
		kb.POST("/manual", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.CreateManualKnowledge)
		kbRead.GET("", g.Viewer(), g.KBAccessRead("id"), handler.ListKnowledge)
		// Raw file download keeps the Contributor + Editor permission boundary of single-file download.
		kbRead.POST("/batch-download", g.Contributor(), g.KBAccessWrite("id"), handler.BatchDownloadKnowledge)
		kbRead.GET("/folders", g.Viewer(), g.KBAccessRead("id"), handler.ListKnowledgeFolders)
		kb.PUT("/folders", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.RenameKnowledgeFolder)
		// Clearing all contents under a KB is a destructive op; gate
		// behind Admin instead of Contributor.
		kb.With(apiKeyFullAccess()).DELETE("", g.Admin(), g.KBAccessWrite("id"), handler.ClearKnowledgeBaseContents)
	}

	// Image gallery: list every image asset of a KB (read-only, Viewer+).
	// Lives directly under /knowledge-bases/:id so it parallels /knowledge,
	// /faq and /tags rather than nesting under /knowledge (documents).
	kbImages := g.apiKeyGroup(r.Group("/knowledge-bases/:id"), apiKeyRetrieve(apiKeyFullAccess()))
	kbImagesRead := kbImages.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		kbImagesRead.GET("/images", g.Viewer(), g.KBAccessRead("id"), handler.ListImages)
		// Same listing with the filter as a JSON body: knowledge_ids escapes
		// the URL-length cap, so the source picker's select-all can name
		// every document of a large KB.
		kbImagesRead.POST("/images", g.Viewer(), g.KBAccessRead("id"), handler.ListImagesPost)
		// Self-describing gallery contract: live attribute sources, the
		// resolved attribute list (definitions + merged usage) and the
		// caller's search activation state. Per-KB because KB-defined
		// attribute sources resolve against :id.
		kbImagesRead.GET("/gallery-config", g.Viewer(), g.KBAccessRead("id"), handler.GetGalleryConfig)
	}

	// Knowledge route group (URL :id is a knowledge id; the guard walks it to the parent KB)
	kgrp := r.Group("/knowledge")
	k := g.apiKeyGroup(kgrp, apiKeyIngest(apiKeyFullAccess()))
	kRead := k.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		// Cross-knowledge endpoints (no :id) can't be gated on a single
		// KB via the URL — they accept a kb_id (or source/target KB) in the
		// body and the handler fans out the access check itself. /batch and
		// /search are read routes (retrieve). /move, /batch-delete,
		// /batch-reparse and /tags are content writes that each bound
		// themselves to a single (or source+target) KB and enforce the API
		// key's KB allow-list downstream — MoveKnowledge via
		// requireTenantAPIKeyKnowledgeBases(source,target); the batch ops via
		// validateKnowledgeBaseAccessWithKBID (requireTenantAPIKeyKnowledgeBase)
		// plus a per-item "belongs to the authorized KB" check in the handler
		// and service. They are therefore declared for API keys under the
		// ingest capability, matching their single-document siblings
		// (k.DELETE("/:id"), k.POST("/:id/reparse"), k.PUT("/:id")).
		kRead.GET("/batch", g.Viewer(), handler.GetKnowledgeBatch)
		kRead.GET("/:id", g.Viewer(), g.KBAccessReadFromKnowledgeIDParam("id"), handler.GetKnowledge)
		kRead.GET("/:id/stages", g.Viewer(), g.KBAccessReadFromKnowledgeIDParam("id"), handler.GetKnowledgeSpans)
		kRead.GET("/:id/spans", g.Viewer(), g.KBAccessReadFromKnowledgeIDParam("id"), handler.GetKnowledgeSpans)
		k.DELETE("/:id", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.DeleteKnowledge)
		k.PUT("/:id", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.UpdateKnowledge)
		k.POST("/:id/regenerate-summary", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.RegenerateKnowledgeSummary)
		k.PUT("/manual/:id", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.UpdateManualKnowledge)
		k.POST("/:id/reparse", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.ReparseKnowledge)
		k.POST("/:id/cancel-parse", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.CancelKnowledgeParse)
		// Downloading exposes the original source file, so it has a stricter
		// boundary than viewing parsed content or previewing it: tenant Viewers
		// cannot download from their own workspace, and org-shared Viewer access
		// cannot download from the source workspace. API keys still follow the
		// retrieve capability declared by kRead; role guards intentionally defer
		// machine-principal authorization to the API-key gate.
		kRead.GET("/:id/download", g.Contributor(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.DownloadKnowledgeFile)
		kRead.GET("/:id/preview", g.Viewer(), g.KBAccessReadFromKnowledgeIDParam("id"), handler.PreviewKnowledgeFile)
		k.PUT("/image/:id/:chunk_id", g.OwnedKnowledgeKBOrAdmin(), g.KBAccessWriteFromKnowledgeIDParam("id"), handler.UpdateImageInfo)
		kRead.GET("/search", g.Viewer(), handler.SearchKnowledge)
		kRead.GET("/move/progress/:task_id", g.Viewer(), handler.GetKnowledgeMoveProgress)
		// Batch / cross-KB content writes: JWT Contributor+, or an API key
		// with the ingest capability (or full access). Each handler binds the
		// operation to a single (move: source+target) KB and rejects any KB
		// or knowledge id outside the key's allow-list, so a scoped ingest key
		// can only touch KBs it is already permitted to write.
		k.PUT("/tags", g.Contributor(), handler.UpdateKnowledgeTagBatch)
		k.POST("/batch-reparse", g.Contributor(), handler.BatchReparseKnowledge)
		k.POST("/batch-delete", g.Contributor(), handler.BatchDeleteKnowledge)
		k.POST("/folder", g.Contributor(), handler.MoveKnowledgeToFolder)
		k.POST("/move", g.Contributor(), handler.MoveKnowledge)
	}
}

// RegisterFAQRoutes registers the FAQ routes
//
// FAQ entries are KB content: reads are Viewer+, all mutations
// (create / update / upsert / delete / batch field+tag updates,
// import display flag) are Contributor+. Search is read-only.
func RegisterFAQRoutes(r *gin.RouterGroup, handler *handler.FAQHandler, g *rbacGuards) {
	if handler == nil {
		return
	}
	// FAQ entries are a KB sub-resource (the content of an FAQ-type KB). Editing an FAQ
	// is editing KB content, so it must follow the KB "creator OR Admin+" matrix,
	// consistent with chunks / wiki pages. Viewer+ can read; a Contributor cannot
	// edit the FAQs of a KB they do not own.
	faq := g.apiKeyGroup(r.Group("/knowledge-bases/:id/faq"), apiKeyIngest(apiKeyFullAccess()))
	faqRead := faq.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		// KBAccessRead/Write resolve own/shared/agent-visible access and
		// rewrite the request's tenant context — handler no longer
		// carries an effectiveCtxForKB helper.
		faqRead.GET("/entries", g.Viewer(), g.KBAccessRead("id"), handler.ListEntries)
		faqRead.GET("/entries/export", g.Viewer(), g.KBAccessRead("id"), handler.ExportEntries)
		faqRead.GET("/entries/:entry_id", g.Viewer(), g.KBAccessRead("id"), handler.GetEntry)
		faq.POST("/entries", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.UpsertEntries)
		faq.POST("/entry", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.CreateEntry)
		faq.PUT("/entries/:entry_id", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.UpdateEntry)
		faq.POST("/entries/:entry_id/similar-questions", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.AddSimilarQuestions)
		// Unified batch update API - supports is_enabled, is_recommended, tag_id
		faq.PUT("/entries/fields", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.UpdateEntryFieldsBatch)
		faq.PUT("/entries/tags", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.UpdateEntryTagBatch)
		faq.DELETE("/entries", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.DeleteEntries)
		// Search is a read route: scoped API keys may call it with retrieve
		// even though POST is otherwise an unsafe method.
		faqRead.POST("/search", g.Viewer(), g.KBAccessRead("id"), handler.SearchFAQ)
		// FAQ import result display status
		faq.PUT("/import/last-result/display", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.UpdateLastImportResultDisplayStatus)
	}
	// FAQ import progress route (outside of knowledge-base scope) — Viewer+.
	// Scoped API keys that can ingest (they start the import) or retrieve may
	// poll their own import/dry-run progress. The task is tenant-scoped by
	// requireTaskProgressTenant, so a key only ever sees its own tenant's
	// tasks. Declared through apiKeyRoute so the APIKeyGate doesn't fail-closed
	// and 403 the poller with "scope does not allow this operation".
	g.apiKeyRoute(r, http.MethodGet, "/faq/import/progress/:task_id",
		apiKeyRetrieve(apiKeyIngest(apiKeyFullAccess())), g.Viewer(), handler.GetImportProgress)
}

// RegisterKnowledgeBaseRoutes registers the knowledge base routes
func RegisterKnowledgeBaseRoutes(r *gin.RouterGroup, handler *handler.KnowledgeBaseHandler, g *rbacGuards) {
	// Knowledge base route group. API-key reachability has two capability tiers, all declared through apiKeyGroup.
	// Do not register with a bare kbgrp.Handle (that bypasses the gate and silently default-denies every key):
	//
	//   1. Read (list/detail/search/progress/move-targets): retrieve OR full-access (kb)
	//   2. KB lifecycle management (create/copy/duplicate/update/delete)
	//      : manage_kbs OR full-access (kbManagement)
	//
	// Tier 2 shares one policy across the KB lifecycle: manage_kbs is the "manage knowledge bases" capability,
	// and create/copy/update/delete are all within it. The KB allow-list still applies downstream: the target KB of
	// copy/duplicate/update/delete is caught by the allow-list. Create has no source to constrain, so a key with
	// an allow-list creates a KB outside that allow-list (same tenant, no privilege escalation, it just cannot manage
	// it afterwards),
	// while a key with an empty allow-list manages every KB in the tenant, so new KBs are in scope. KB content writes
	// (documents/
	// chunks/FAQ/Tag/Wiki) are governed by the ingest capability on the matching sub-routes, not by this group.
	kbgrp := r.Group("/knowledge-bases")
	kb := g.apiKeyGroup(kbgrp, apiKeyRetrieve(apiKeyFullAccess()))
	kbManagement := kb.With(apiKeyManageKnowledgeBases(apiKeyFullAccess()))
	{
		// Create a knowledge base: JWT Contributor+; API keys need manage_kbs or full-access.
		kbManagement.POST("", g.Contributor(), handler.CreateKnowledgeBase)
		// List knowledge bases: Viewer+ for JWT callers; retrieve-capable API keys pass via the gate.
		kb.GET("", g.Viewer(), handler.ListKnowledgeBases)
		// Get knowledge base details: Viewer+ with read access to the KB
		kb.GET("/:id", g.Viewer(), g.KBAccessRead("id"), handler.GetKnowledgeBase)
		// Update/delete a knowledge base: two orthogonal authorization layers, both required:
		//   OwnedKBOrAdmin  governs ownership within the tenant: a Contributor who is not the creator cannot edit
		//                   a colleague's KB (a cross-tenant KB gets lookup=NotFound here and is left to
		//                   downstream handling, not blocked here).
		//   KBAccessWrite   governs the cross-tenant access level: own KB or shared through an organization (editor).
		// The handler then makes the final decision by permission/owner tenant. In particular DeleteKnowledgeBase
		// checks kb.TenantID against the caller's own tenant (c.Keys, not rewritten by KBAccess),
		// which restricts deletion to "owner tenant + Admin": a shared editor cannot delete the source KB.
		kbManagement.PUT("/:id", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.UpdateKnowledgeBase)
		// Regenerate the knowledge base AI description now: same authorization tier as update; runs one small-model
		// call synchronously.
		kbManagement.POST("/:id/profile/generate", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"),
			handler.GenerateKnowledgeBaseProfile)
		kbManagement.DELETE("/:id", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), handler.DeleteKnowledgeBase)
		// Pin/unpin a knowledge base: the creator OR Admin+, with write access to the KB
		// Pin state is now per-(user, kb) (migration 000050). Anyone with
		// at least Viewer-level read access to the KB — including users
		// who reached it via a shared agent — may pin it for themselves;
		// no edit permission is required. The OwnedKBOrAdmin guard was
		// removed accordingly. The route still requires KB read access
		// so callers can't poke at KBs they can't see.
		kb.PUT("/:id/pin", g.Viewer(), g.KBAccessRead("id"), handler.TogglePinKnowledgeBase)
		// Hybrid search: Viewer+ with read access to the KB (read-only)
		// POST is preferred; GET with JSON body is kept for backward compatibility (#1727).
		kb.POST("/:id/hybrid-search", g.Viewer(), g.KBAccessRead("id"), handler.HybridSearch)
		kb.GET("/:id/hybrid-search", g.Viewer(), g.KBAccessRead("id"), handler.HybridSearch)
		// Copy a knowledge base: produces a new KB, same tier as create: JWT Contributor+, API keys need manage_kbs or
		// full-access.
		// The source KB comes from source_id in the body (not the :id path parameter), so the path-parameter-based
		// KBAccessRead cannot apply; the handler checks the tenant ownership and allow-list of the source and target
		// KBs
		// (requireTenantAPIKeyKnowledgeBases puts source_id/target_id through the allow-list).
		// The copy belongs to the caller; ownership of the original KB is not required.
		kbManagement.POST("/copy", g.Contributor(), handler.CopyKnowledgeBase)
		// Duplicate a knowledge base: produces a new KB, same tier as create: JWT Contributor+, API keys need
		// manage_kbs or full-access,
		// plus read access to the source KB (KBAccessRead checks the source KB for restricted keys). Creates only the
		// new KB settings record; content, indexes and shares are not copied.
		kbManagement.POST("/:id/duplicate", g.Contributor(), g.KBAccessRead("id"), handler.DuplicateKnowledgeBase)
		// Get knowledge base copy progress: Viewer+, read-only. Either manage_kbs (the key that started the copy) or
		// retrieve may poll; tasks are isolated per tenant (requireTaskProgressTenant), so a key can only
		// see its own tenant's tasks.
		kb.With(apiKeyRetrieve(apiKeyManageKnowledgeBases(apiKeyFullAccess()))).
			GET("/copy/progress/:task_id", g.Viewer(), handler.GetKBCloneProgress)
		// List the knowledge bases a move can target: Viewer+ with read access to the KB
		kb.GET("/:id/move-targets", g.Viewer(), g.KBAccessRead("id"), handler.ListMoveTargets)
	}
}

// RegisterImageAttrRoutes wires the global, read-only image-attribute registry
// that drives the KB editor's attribute panel.
//
// The registry is a single source of truth, so adding an attribute is a
// backend-only change (one registry row) and the UI follows automatically. It
// carries no KB id and needs only the Viewer role, so — like the other
// read-only KB-editor helpers GET /chunker/preview and GET
// /system/parser-engines — it is mounted at the top level rather than under
// the /knowledge-bases collection.
func RegisterImageAttrRoutes(r *gin.RouterGroup, handler *handler.KnowledgeBaseHandler, g *rbacGuards) {
	g.apiKeyRoute(r, http.MethodGet, "/image-attrs/schema",
		apiKeyRetrieve(apiKeyFullAccess()), g.Viewer(), handler.GetImageAttrsSchema)
}

// RegisterKnowledgeBaseActivityRoutes exposes the read-only per-KB activity
// feed. It intentionally stays JWT-only: audit history is a sensitive owner
// surface and no existing workspace API-key capability grants audit access.
func RegisterKnowledgeBaseActivityRoutes(r *gin.RouterGroup, auditHandler *handler.AuditLogHandler, g *rbacGuards) {
	if auditHandler == nil {
		return
	}
	r.GET("/knowledge-bases/:id/activity",
		g.OwnedKBOrAdmin(), g.KBAccessRead("id"), auditHandler.ListKnowledgeBaseActivity)
}

// RegisterKnowledgeTagRoutes registers the knowledge base tag routes.
//
// Tags are KB metadata: Viewer reads, Contributor writes. Per-KB
// ownership granularity for tags is out of scope for PR 2; this is
// purely role-based.
func RegisterKnowledgeTagRoutes(r *gin.RouterGroup, tagHandler *handler.TagHandler, g *rbacGuards) {
	if tagHandler == nil {
		return
	}
	// Tags are a KB sub-resource: creating, editing or deleting a tag changes how KB content is classified for
	// retrieval,
	// so they follow the KB's "creator OR Admin+" matrix. That keeps an unrelated
	// Contributor from creating or deleting tags in someone else's KB and disrupting how the KB owner organizes
	// content.
	kbTags := g.apiKeyGroup(r.Group("/knowledge-bases/:id/tags"), apiKeyIngest(apiKeyFullAccess()))
	kbTagsRead := kbTags.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		// KBAccessRead/Write resolve own/shared/agent-visible access and
		// rewrite the request's tenant context to the effective tenant
		// for the duration of the handler — so the handler no longer
		// needs its own effectiveCtxForKB helper.
		kbTagsRead.GET("", g.Viewer(), g.KBAccessRead("id"), tagHandler.ListTags)
		kbTags.POST("", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), tagHandler.CreateTag)
		kbTags.PUT("/:tag_id", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), tagHandler.UpdateTag)
		kbTags.DELETE("/:tag_id", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), tagHandler.DeleteTag)
	}
}

// RegisterWikiPageRoutes registers wiki page related routes.
//
// Wiki pages are KB content (wiki mode): reads are Viewer+ and gated by
// KBAccessRead (own / org-shared / via shared agent), matching FAQ /
// chunk / tag read routes. Content mutations (create/update/delete) and
// maintenance actions (rebuild-links, auto-fix, change issue status)
// honour per-KB ownership via OwnedWikiKBOrAdmin (PR 5, #1303): the URL
// :kb_id resolves directly to the owning KB so a Contributor who owns
// the KB can manage its wiki, while a non-owner Contributor gets 403.
func RegisterWikiPageRoutes(r *gin.RouterGroup, wikiHandler *handler.WikiPageHandler, g *rbacGuards) {
	wiki := g.apiKeyGroup(r.Group("/knowledgebase/:kb_id/wiki"), apiKeyIngest(apiKeyFullAccess()))
	wikiRead := wiki.With(apiKeyRetrieve(apiKeyFullAccess()))
	{
		// Page CRUD
		wikiRead.GET("/pages", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.ListPages)
		wiki.POST("/pages", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.CreatePage)
		wiki.PUT("/move-page", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.MovePage)
		wikiRead.GET("/pages/*slug", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.GetPage)
		wiki.PUT("/pages/*slug", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.UpdatePage)
		wiki.DELETE("/pages/*slug", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.DeletePage)

		// Revision history (slug is a catch-all like /pages; revert carries
		// the slug in the body for the same reason move-page does)
		wikiRead.GET("/revisions/*slug", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.ListRevisions)
		wiki.POST("/revert", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.RevertPage)

		// Folder tree (directory nodes)
		wikiRead.GET("/folders", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.ListFolders)
		wiki.POST("/folders", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.CreateFolder)
		wiki.PUT("/folders/:folder_id", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.UpdateFolder)
		wiki.DELETE("/folders/:folder_id", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.DeleteFolder)

		// Special pages
		wikiRead.GET("/index", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.GetIndex)

		// Graph and stats
		wikiRead.GET("/graph", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.GetGraph)
		wikiRead.GET("/stats", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.GetStats)

		// Search and maintenance
		wikiRead.GET("/search", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.SearchPages)
		wiki.POST("/rebuild-links", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.RebuildLinks)
		wikiRead.GET("/lint", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.Lint)
		wiki.POST("/auto-fix", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.AutoFix)

		// Issues
		wikiRead.GET("/issues", g.Viewer(), g.KBAccessRead("kb_id"), wikiHandler.ListIssues)
		wiki.PUT("/issues/:issue_id/status", g.OwnedWikiKBOrAdmin(), g.KBAccessWrite("kb_id"), wikiHandler.UpdateIssueStatus)
	}

	wikiSearch := g.apiKeyGroup(r.Group("/wiki-search", g.Viewer()), apiKeyRetrieve(apiKeyFullAccess()))
	{
		wikiSearch.POST("", wikiHandler.SearchPagesAcross)
	}
}
