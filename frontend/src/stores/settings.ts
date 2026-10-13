import { defineStore } from "pinia";
import { nextTick } from "vue";
import { BUILTIN_QUICK_ANSWER_ID, BUILTIN_SMART_REASONING_ID } from "@/api/agent";
import { getApiBaseUrl } from "@/utils/api-base";
import { isAgentStreamAgentId } from "@/utils/agent-mode";
import { loadAndReconcileSettings } from "@/stores/settingsStorage";
import { isReasoningLevel, type ReasoningLevel } from "@/utils/reasoningEffort";

// Settings interface
interface Settings {
  endpoint: string;
  apiKey: string;
  knowledgeBaseId: string;
  isAgentEnabled: boolean;
  agentConfig: AgentConfig;
  selectedKnowledgeBases: string[];  // IDs of the currently selected knowledge bases
  selectedFiles: string[]; // IDs of the currently selected files
  selectedFileKbMap: Record<string, string>; // file ID -> knowledge base ID, so shared knowledge base files can be fetched with kb_id after a refresh
  selectedTags: Array<{ id: string; name: string; kbId: string; kbName?: string }>;
  selectedMCPServices: string[];
  selectedSkills: string[];
  selectedTools?: string[];
  modelConfig: ModelConfig;  // model config
  ollamaConfig: OllamaConfig;  // Ollama config
  localBrowserEnabled: boolean; // Explicit source preference; composer activates it only while the extension is online
  webSearchEnabled: boolean;  // whether web search is enabled
  conversationModels: ConversationModels;
  selectedAgentId: string;  // ID of the currently selected agent
  selectedAgentSourceTenantId: string | null;  // with a shared agent, the source space ID (for backend model/KB/MCP resolution)
  autoCheckUpdate?: boolean; // whether to check for and download updates automatically
}

// Agent config interface
interface AgentConfig {
  maxIterations: number;
  temperature: number;
  allowedTools: string[];
  system_prompt?: string;  // Unified system prompt (uses {{web_search_status}} placeholder)
}

interface ConversationModels {
  summaryModelId: string;
  rerankModelId: string;
  selectedChatModelId: string;  // ID of the chat model the user currently selected
}

// Single model item interface
interface ModelItem {
  id: string;  // unique ID
  name: string;  // display name
  source: 'local' | 'remote';  // model source
  modelName: string;  // model identifier
  baseUrl?: string;  // remote API URL
  apiKey?: string;  // remote API key
  dimension?: number;  // embedding only: vector dimension
  interfaceType?: 'ollama' | 'openai';  // VLLM only: interface type
  isDefault?: boolean;  // whether this is the default model
}

// Model config interface - supports multiple models
interface ModelConfig {
  chatModels: ModelItem[];
  embeddingModels: ModelItem[];
  rerankModels: ModelItem[];
  vllmModels: ModelItem[];  // VLLM vision models
}

// Ollama config interface
interface OllamaConfig {
  baseUrl: string;  // Ollama service address
  enabled: boolean;  // whether it is enabled
}

// Default settings
const defaultSettings: Settings = {
  endpoint: getApiBaseUrl(),
  apiKey: "",
  knowledgeBaseId: "",
  isAgentEnabled: false,
  agentConfig: {
    maxIterations: 5,
    temperature: 0.7,
    allowedTools: [],  // empty by default; loaded from the backend through the API
    system_prompt: "",
  },
  selectedKnowledgeBases: [],  // empty array by default
  selectedFiles: [], // empty array by default
  selectedFileKbMap: {},  // file ID -> knowledge base ID
  selectedTags: [],
  selectedMCPServices: [],
  selectedSkills: [],
  modelConfig: {
    chatModels: [],
    embeddingModels: [],
    rerankModels: [],
    vllmModels: []
  },
  ollamaConfig: {
    baseUrl: "http://localhost:11434",
    enabled: true
  },
  localBrowserEnabled: false,
  webSearchEnabled: false,  // web search off by default
  conversationModels: {
    summaryModelId: "",
    rerankModelId: "",
    selectedChatModelId: "",  // ID of the chat model the user currently selected
  },
  selectedAgentId: BUILTIN_QUICK_ANSWER_ID,  // quick answer mode selected by default
  selectedAgentSourceTenantId: null as string | null,  // source space ID of a shared agent
  autoCheckUpdate: true,
};

export const useSettingsStore = defineStore("settings", {
  state: () => ({
    // Load settings from local storage, or use the defaults
    settings: loadAndReconcileSettings(defaultSettings),
    // Take a snapshot of the "global defaults" on entering a session and restore it on leaving. Not a persisted field:
    // a page refresh is the same as entering the session again, so the snapshot is retaken naturally.
    _defaultsSnapshot: null as Settings | null,
    /** Restoring the input bar from session.last_request_state, so the agent-switch watch does not overwrite the KB selection */
    _isApplyingSessionState: false,
    // Session-only preference: never written into global settings/localStorage.
    reasoningEffortOverride: '' as ReasoningLevel | '',
  }),

  getters: {
    // Whether Agent is enabled
    isAgentEnabled: (state) => state.settings.isAgentEnabled || false,

    // Whether this is the built-in quick answer (checks selectedAgentId first, so it does not drift from isAgentEnabled)
    isQuickAnswerMode: (state) =>
      (state.settings.selectedAgentId || BUILTIN_QUICK_ANSWER_ID) === BUILTIN_QUICK_ANSWER_ID,

    // Whether to use the Agent streaming pipeline (smart reasoning / custom agents); quick answer uses the RAG pipeline
    isAgentStreamMode: (state) =>
      isAgentStreamAgentId(
        state.settings.selectedAgentId,
        state.settings.isAgentEnabled || false,
      ),
    
    // Whether Agent is ready (config complete)
    // Requires: 1) allowed tools configured 2) a chat model set 3) a rerank model set
    isAgentReady: (state) => {
      const config = state.settings.agentConfig || defaultSettings.agentConfig
      const models = state.settings.conversationModels || defaultSettings.conversationModels
      return Boolean(
        config.allowedTools && config.allowedTools.length > 0 &&
        models.summaryModelId && models.summaryModelId.trim() !== '' &&
        models.rerankModelId && models.rerankModelId.trim() !== ''
      )
    },
    
    // Whether normal mode (quick answer) is ready
    // Requires: 1) a chat model set 2) a rerank model set
    isNormalModeReady: (state) => {
      const models = state.settings.conversationModels || defaultSettings.conversationModels
      return Boolean(
        models.summaryModelId && models.summaryModelId.trim() !== '' &&
        models.rerankModelId && models.rerankModelId.trim() !== ''
      )
    },
    
    // Get the Agent config
    agentConfig: (state) => state.settings.agentConfig || defaultSettings.agentConfig,

    conversationModels: (state) => state.settings.conversationModels || defaultSettings.conversationModels,
    
    // Get the model config
    modelConfig: (state) => state.settings.modelConfig || defaultSettings.modelConfig,
    
    // Query source for this turn
    isLocalBrowserEnabled: (state) => state.settings.localBrowserEnabled === true,
    isWebSearchEnabled: (state) => state.settings.webSearchEnabled || false,
    
    // Whether to check for and download updates automatically
    isAutoCheckUpdateEnabled: (state) => state.settings.autoCheckUpdate ?? true,

    // ID of the currently selected agent
    selectedAgentId: (state) => state.settings.selectedAgentId || BUILTIN_QUICK_ANSWER_ID,
    // Source space ID of a shared agent (optional)
    selectedAgentSourceTenantId: (state) => state.settings.selectedAgentSourceTenantId ?? null,
  },

  actions: {
    // Save settings
    saveSettings(settings: Settings) {
      this.settings = { ...settings };
      // Save to localStorage
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    // Get settings
    getSettings(): Settings {
      return this.settings;
    },

    // Get the API endpoint
    getEndpoint(): string {
      return this.settings.endpoint || defaultSettings.endpoint;
    },

    // Get the API key
    getApiKey(): string {
      return this.settings.apiKey;
    },

    // Get the knowledge base ID
    getKnowledgeBaseId(): string {
      return this.settings.knowledgeBaseId;
    },
    
    // Enable/disable Agent
    toggleAgent(enabled: boolean) {
      this.settings.isAgentEnabled = enabled;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Update the Agent config
    updateAgentConfig(config: Partial<AgentConfig>) {
      this.settings.agentConfig = { ...this.settings.agentConfig, ...config };
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    updateConversationModels(models: Partial<ConversationModels>) {
      const current = this.settings.conversationModels || defaultSettings.conversationModels;
      this.settings.conversationModels = { ...current, ...models };
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Update the model config
    updateModelConfig(config: Partial<ModelConfig>) {
      this.settings.modelConfig = { ...this.settings.modelConfig, ...config };
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Add a model
    addModel(type: 'chat' | 'embedding' | 'rerank' | 'vllm', model: ModelItem) {
      const key = `${type}Models` as keyof ModelConfig;
      const models = [...this.settings.modelConfig[key]] as ModelItem[];
      // If set as default, clear the default flag on the other models
      if (model.isDefault) {
        models.forEach(m => m.isDefault = false);
      }
      // The first model becomes the default automatically
      if (models.length === 0) {
        model.isDefault = true;
      }
      models.push(model);
      this.settings.modelConfig[key] = models as any;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Update a model
    updateModel(type: 'chat' | 'embedding' | 'rerank' | 'vllm', modelId: string, updates: Partial<ModelItem>) {
      const key = `${type}Models` as keyof ModelConfig;
      const models = [...this.settings.modelConfig[key]] as ModelItem[];
      const index = models.findIndex(m => m.id === modelId);
      if (index !== -1) {
        // If it is being set as default, clear the default flag on the other models
        if (updates.isDefault) {
          models.forEach(m => m.isDefault = false);
        }
        models[index] = { ...models[index], ...updates };
        this.settings.modelConfig[key] = models as any;
        localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
      }
    },
    
    // Delete a model
    deleteModel(type: 'chat' | 'embedding' | 'rerank' | 'vllm', modelId: string) {
      const key = `${type}Models` as keyof ModelConfig;
      let models = [...this.settings.modelConfig[key]] as ModelItem[];
      const deletedModel = models.find(m => m.id === modelId);
      models = models.filter(m => m.id !== modelId);
      // If the default model was deleted, make the first one the default
      if (deletedModel?.isDefault && models.length > 0) {
        models[0].isDefault = true;
      }
      this.settings.modelConfig[key] = models as any;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Set the default model
    setDefaultModel(type: 'chat' | 'embedding' | 'rerank' | 'vllm', modelId: string) {
      const key = `${type}Models` as keyof ModelConfig;
      const models = [...this.settings.modelConfig[key]] as ModelItem[];
      models.forEach(m => m.isDefault = (m.id === modelId));
      this.settings.modelConfig[key] = models as any;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Update the Ollama config
    updateOllamaConfig(config: Partial<OllamaConfig>) {
      this.settings.ollamaConfig = { ...this.settings.ollamaConfig, ...config };
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Select knowledge bases (replaces the whole list)
    selectKnowledgeBases(kbIds: string[]) {
      this.settings.selectedKnowledgeBases = kbIds;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Add one knowledge base
    addKnowledgeBase(kbId: string) {
      if (!this.settings.selectedKnowledgeBases.includes(kbId)) {
        this.settings.selectedKnowledgeBases.push(kbId);
        localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
      }
    },
    
    // Remove one knowledge base
    removeKnowledgeBase(kbId: string) {
      this.settings.selectedKnowledgeBases = 
        this.settings.selectedKnowledgeBases.filter((id: string) => id !== kbId);
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Clear the knowledge base selection
    clearKnowledgeBases() {
      this.settings.selectedKnowledgeBases = [];
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Get the selected knowledge base list
    getSelectedKnowledgeBases(): string[] {
      return this.settings.selectedKnowledgeBases || [];
    },
    
    // The local browser and web search can be chosen independently.
    toggleLocalBrowser(enabled: boolean) {
      this.settings.localBrowserEnabled = enabled;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    toggleWebSearch(enabled: boolean) {
      this.settings.webSearchEnabled = enabled;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    // Enable/disable automatic update checks
    toggleAutoCheckUpdate(enabled: boolean) {
      this.settings.autoCheckUpdate = enabled;
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    // File selection actions
    addFile(fileId: string) {
      if (!this.settings.selectedFiles) this.settings.selectedFiles = [];
      if (!this.settings.selectedFiles.includes(fileId)) {
        this.settings.selectedFiles.push(fileId);
        localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
      }
    },

    removeFile(fileId: string) {
      if (!this.settings.selectedFiles) return;
      this.settings.selectedFiles = this.settings.selectedFiles.filter((id: string) => id !== fileId);
      if (this.settings.selectedFileKbMap) delete this.settings.selectedFileKbMap[fileId];
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    clearFiles() {
      this.settings.selectedFiles = [];
      this.settings.selectedFileKbMap = {};
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    addTag(tag: { id: string; name: string; kbId: string; kbName?: string }) {
      if (!this.settings.selectedTags) this.settings.selectedTags = [];
      if (!this.settings.selectedTags.some(t => t.id === tag.id && t.kbId === tag.kbId)) {
        this.settings.selectedTags.push(tag);
        localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
      }
    },

    removeTag(tagId: string, kbId?: string) {
      if (!this.settings.selectedTags) return;
      this.settings.selectedTags = this.settings.selectedTags.filter(t => !(t.id === tagId && (!kbId || t.kbId === kbId)));
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    clearTags() {
      this.settings.selectedTags = [];
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    addMCPService(serviceId: string) {
      if (!this.settings.selectedMCPServices) this.settings.selectedMCPServices = [];
      if (!this.settings.selectedMCPServices.includes(serviceId)) {
        this.settings.selectedMCPServices.push(serviceId);
        localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
      }
    },

    removeMCPService(serviceId: string) {
      if (!this.settings.selectedMCPServices) return;
      this.settings.selectedMCPServices = this.settings.selectedMCPServices.filter(id => id !== serviceId);
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    addSkill(skillName: string) {
      if (!this.settings.selectedSkills) this.settings.selectedSkills = [];
      if (!this.settings.selectedSkills.includes(skillName)) {
        this.settings.selectedSkills.push(skillName);
        localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
      }
    },

    removeSkill(skillName: string) {
      if (!this.settings.selectedSkills) return;
      this.settings.selectedSkills = this.settings.selectedSkills.filter(name => name !== skillName);
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    setFileKbMap(updates: Record<string, string>) {
      if (!this.settings.selectedFileKbMap) this.settings.selectedFileKbMap = {};
      Object.assign(this.settings.selectedFileKbMap, updates);
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },

    removeFileKbId(fileId: string) {
      if (this.settings.selectedFileKbMap) delete this.settings.selectedFileKbMap[fileId];
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    getSelectedFiles(): string[] {
      return this.settings.selectedFiles || [];
    },

    /**
     * Scope for suggested-questions API (KB / file / tag @mentions).
     * Leave `limit` undefined to let the backend apply the agent's configured
     * starter count; pass a number only to request a specific count.
     */
    getSuggestedQuestionsParams(limit?: number) {
      const selectedKBs = this.getSelectedKnowledgeBases();
      const selectedFiles = this.getSelectedFiles();
      const tags = this.settings.selectedTags || [];
      const tagScopes = Object.entries(tags.reduce<Record<string, string[]>>((scopes, tag) => {
        if (!tag.id || !tag.kbId) return scopes;
        (scopes[tag.kbId] ||= []).push(tag.id);
        return scopes;
      }, {})).map(([knowledge_base_id, ids]) => ({
        knowledge_base_id,
        tag_ids: [...new Set(ids)],
      }));
      return {
        // A tag's parent KB is only an ownership hint, not an explicit whole-KB
        // selection. Keep it in tag_scopes so the backend cannot widen a tag to
        // every document in that KB.
        knowledge_base_ids: selectedKBs.length > 0 ? selectedKBs : undefined,
        knowledge_ids: selectedFiles.length > 0 ? selectedFiles : undefined,
        tag_scopes: tagScopes.length > 0 ? tagScopes : undefined,
        limit,
      };
    },
    
    // Select an agent (sourceTenantId is passed only for a shared agent)
    selectAgent(agentId: string, sourceTenantId?: string | null) {
      this.reasoningEffortOverride = '';
      this.settings.selectedAgentId = agentId;
      this.settings.selectedAgentSourceTenantId = (sourceTenantId != null && sourceTenantId !== "") ? sourceTenantId : null;
      // The agent config only decides whether web search is available, not whether to use it this turn for the user.
      // It defaults to off on every agent selection, and only the user can turn it on from the input box afterwards.
      this.settings.webSearchEnabled = false;
      this.settings.localBrowserEnabled = false;
      // Switch Agent mode automatically by agent type
      if (agentId === BUILTIN_QUICK_ANSWER_ID) {
        this.settings.isAgentEnabled = false;
      } else if (agentId === BUILTIN_SMART_REASONING_ID) {
        this.settings.isAgentEnabled = true;
      }
      // A custom agent decides from its config
      
      // Reset the knowledge base and file selection when switching agents,
      // because different agents link different knowledge bases, so the user's earlier selection must be cleared
      this.settings.selectedKnowledgeBases = [];
      this.settings.selectedFiles = [];
      this.settings.selectedFileKbMap = {};
      this.settings.selectedTags = [];
      this.settings.selectedMCPServices = [];
      this.settings.selectedSkills = [];
      localStorage.setItem("AIKB_settings", JSON.stringify(this.settings));
    },
    
    // Get the selected agent ID
    getSelectedAgentId(): string {
      return this.settings.selectedAgentId || BUILTIN_QUICK_ANSWER_ID;
    },

    // --- Per-session input state restore --- //
    //
    // The input bar's agent / model / KB / web search / MCP selections live in this store and are shared across sessions.
    // But users expect to see the state a request was sent with when they open an old session.
    // Strategy: on entering a session, stash the "current global defaults" in a non-persisted `_defaultsSnapshot`
    // field, then overwrite the store with session.last_request_state; on leaving the session, restore from the snapshot.
    // The snapshot is not written to localStorage, because it only matters while the route is "inside an old session";
    // a page refresh is the same as "entering the session again" -> retake the snapshot + overwrite, so the user's global defaults are not lost.

    // Snapshot the current settings as "the defaults to restore after leaving the session".
    // An existing snapshot is not overwritten, so switching between sessions (B -> B') does not mistake the restored store for the defaults.
    snapshotAsDefaultsIfNeeded() {
      if (this._defaultsSnapshot) return;
      this._defaultsSnapshot = JSON.parse(JSON.stringify(this.settings));
    },

    // Restore the defaults (if there is a snapshot), when leaving a session or switching across sessions.
    restoreDefaultsIfSnapshotted() {
      this.reasoningEffortOverride = '';
      if (!this._defaultsSnapshot) return;
      this.settings = this._defaultsSnapshot;
      this._defaultsSnapshot = null;
      // Do not write localStorage: the defaults were already written to localStorage before the snapshot, so what is restored here
      // is the existing value in localStorage, and writing again would only add pointless IO.
    },

    // The first send of a new session keeps createChat's input state, which an empty or stale record returned asynchronously must not overwrite.
    // preserveDraft must be captured before requesting the session details, not read when the response returns.
    hydrateSessionInputState(state: SessionLastRequestStatePayload | null | undefined, preserveDraft = false) {
      if (!state || preserveDraft) return;
      this.snapshotAsDefaultsIfNeeded();
      this.applyLastRequestState(state);
    },

    // Overwrite the input bar fields from session.last_request_state.
    // Touch only the fields this record has, and do **not** clear other unrelated fields in the store (such as the model list).
    // Restore by field meaning, without writing the session's selections into the user's default settings.
    applyLastRequestState(state: SessionLastRequestStatePayload | null | undefined) {
      if (!state) return;
      this._isApplyingSessionState = true;
      try {
        this.reasoningEffortOverride = isReasoningLevel(state.reasoning_effort) ? state.reasoning_effort : '';
        if (typeof state.agent_enabled === "boolean") {
          this.settings.isAgentEnabled = state.agent_enabled;
        }
        if (typeof state.agent_id === "string" && state.agent_id) {
          this.settings.selectedAgentId = state.agent_id;
          // The server does not yet say whether the last record was an own or a shared agent, so no sourceTenantId comes back.
          // Unlike selectAgent(), this does **not** reset the KB/file selection, because the KB/files from state
          // overwrite it right after, so there is no need to clear first.
        }
        if (state.model_id !== undefined) {
          const current = this.settings.conversationModels || defaultSettings.conversationModels;
          this.settings.conversationModels = { ...current, selectedChatModelId: state.model_id || "" };
        }
        // The backend omits empty lists with omitempty, so a missing value means "this session has no KB scope",
        // not "keep the previous session's selection". The latter would keep sending a stale @KB to the server.
        this.settings.selectedKnowledgeBases = Array.isArray(state.knowledge_base_ids)
          ? [...state.knowledge_base_ids]
          : [];
        // The server omits empty selection lists; keeping the current value would carry files, tags and tool selections
        // from other sessions or the global defaults into this session.
        this.settings.selectedFiles = Array.isArray(state.knowledge_ids) ? [...state.knowledge_ids] : [];
        // Keep the known KB ownership of files that are still selected (shared files need it); the cache for other files does not carry across sessions.
        this.settings.selectedFileKbMap = Object.fromEntries(
          Object.entries(this.settings.selectedFileKbMap || {})
            .filter(([id]) => this.settings.selectedFiles.includes(id))
        );
        if (Array.isArray(state.mentioned_items)) {
          const fromMentions = state.mentioned_items
            .filter(item => item.type === "tag" && item.id && item.kb_id)
            .filter(item => !Array.isArray(state.tag_ids) || state.tag_ids.includes(item.id))
            .map(item => ({ id: item.id, name: item.name || item.id, kbId: item.kb_id!, kbName: item.kb_name }));
          const covered = new Set(fromMentions.map(t => t.id));
          const orphanTagIds = (state.tag_ids || []).filter(id => id && !covered.has(id));
          if (orphanTagIds.length > 0 && Array.isArray(state.knowledge_base_ids) && state.knowledge_base_ids.length === 1) {
            const kbId = state.knowledge_base_ids[0];
            orphanTagIds.forEach(id => {
              fromMentions.push({ id, name: id, kbId, kbName: undefined });
            });
          }
          this.settings.selectedTags = fromMentions;
        } else if (Array.isArray(state.tag_ids)) {
          const existing = this.settings.selectedTags || [];
          this.settings.selectedTags = existing.filter(tag => state.tag_ids?.includes(tag.id));
        } else {
          this.settings.selectedTags = [];
        }
        if (Array.isArray(state.mcp_service_ids)) {
          this.settings.selectedMCPServices = [...state.mcp_service_ids];
        } else if (Array.isArray(state.mentioned_items)) {
          this.settings.selectedMCPServices = state.mentioned_items
            .filter(item => item.type === "mcp" && item.id)
            .map(item => item.id);
        } else {
          this.settings.selectedMCPServices = [];
        }
        if (Array.isArray(state.skill_names)) {
          this.settings.selectedSkills = [...state.skill_names];
        } else if (Array.isArray(state.mentioned_items)) {
          this.settings.selectedSkills = state.mentioned_items
            .filter(item => item.type === "skill" && item.id)
            .map(item => item.skill_name || item.id);
        } else {
          this.settings.selectedSkills = [];
        }
        this.settings.localBrowserEnabled = state.local_browser_enabled === true;
        if (typeof state.web_search_enabled === "boolean") {
          this.settings.webSearchEnabled = state.web_search_enabled;
        }
      } finally {
        // The reset must wait until after the next flush: the watcher on selectedAgentId defaults to
        // flush:'pre' and runs asynchronously; resetting synchronously here would leave the flag already
        // false when the watcher actually runs, the guard would do nothing, and the restored KB would still be overwritten by the agent config. nextTick
        // guarantees the watcher this state change triggers runs while the flag is still true.
        nextTick(() => {
          this._isApplyingSessionState = false;
        });
      }
      // Note: deliberately not writing localStorage - an old session's state must not pollute the "user defaults".
      // On leaving the session, restoreDefaultsIfSnapshotted syncs the complete
      // defaults in localStorage back into this.settings.
    },
  },
});

// JSON shape of the backend sessions.last_request_state (aligned with SessionLastRequestState).
// Every field is optional: history sessions, and requests before a new session's first send, have no such record.
export interface SessionLastRequestStatePayload {
  reasoning_effort?: string;
  agent_id?: string;
  agent_enabled?: boolean;
  model_id?: string;
  knowledge_base_ids?: string[];
  knowledge_ids?: string[];
  tag_ids?: string[];
  mcp_service_ids?: string[];
  skill_names?: string[];
  mentioned_items?: Array<{
    id: string;
    name?: string;
    type: string;
    kb_id?: string;
    kb_name?: string;
    skill_name?: string;
  }>;
  local_browser_enabled?: boolean;
  web_search_enabled?: boolean;
}
