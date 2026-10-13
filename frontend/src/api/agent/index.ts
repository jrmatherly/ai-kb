import { get, post, put, del } from "../../utils/request";

// Agent type preset IDs under Smart Reasoning:
// 'rag-qa'       : classic document/FAQ chunk RAG
// 'wiki-qa'      : Wiki graph navigation Q&A
// 'hybrid-rag-wiki': Wiki + chunk hybrid retrieval
// 'custom'       : fully custom (no preset applied)
export type AgentType = 'rag-qa' | 'wiki-qa' | 'hybrid-rag-wiki' | 'data-analysis' | 'custom';

export interface QuestionSuggestionConfig {
  starters: {
    enabled: boolean;
    mode: 'curated' | 'knowledge' | 'hybrid';
    items: string[];
    count: number;
  };
  follow_ups: {
    enabled: boolean;
    mode: 'generated' | 'knowledge' | 'hybrid';
    count: number;
    model_id?: string;
    additional_instruction?: string;
    categories: Array<'clarify' | 'deepen' | 'action'>;
    max_context_turns: number;
    suppress_on_fallback: boolean;
    suppress_when_answer_asks_question: boolean;
    knowledge_fallback: boolean;
    allow_regenerate: boolean;
  };
}

export interface CustomAgentConfig {
  // ===== Basic settings =====
  agent_mode?: 'quick-answer' | 'smart-reasoning';  // quick-answer = RAG mode, smart-reasoning = ReAct agent mode
  // Type preset under Smart Reasoning: applies a "system prompt + tools + KB compatibility" bundle in one step.
  // Only applies when agent_mode === 'smart-reasoning'; quick-answer ignores it.
  agent_type?: AgentType;
  system_prompt?: string;           // single system prompt; the {{web_search_status}} placeholder switches behavior
  system_prompt_id?: string;        // referenced prompt template ID (set by presets)
  context_template_id?: string;     // Inherit the referenced context template when text is empty
  context_template?: string;        // context template (Quick Answer)

  // ===== Model settings =====
  model_id?: string;
  rerank_model_id?: string;
  temperature?: number;
  max_completion_tokens?: number;   // 0 = system default (Quick Answer 2048; Smart Reasoning 4096, or 24576 with a sandbox that can write files). Above 0 is a custom limit.
  thinking?: boolean;                      // thinking mode, for models that support extended thinking
  // Thinking effort: off | auto | minimal | low | medium | high | xhigh | max.
  // Empty falls back to the thinking boolean (true == auto, false == off).
  reasoning_effort?: string;
  citation_enabled?: boolean;        // cite knowledge base and web sources in the final answer (default on)

  // ===== Agent mode settings =====
  max_iterations?: number;          // -1 = unlimited
  llm_call_timeout?: number;        // seconds
  allowed_tools?: string[];
  reflection_enabled?: boolean;
  // MCP service selection: all = every enabled MCP service, selected = listed services, none = no MCP
  mcp_selection_mode?: 'all' | 'selected' | 'none';
  mcp_services?: string[];          // selected MCP service IDs
  // Seconds to wait for an OAuth authorization triggered mid-conversation; the prompt is skipped when it expires.
  // <=0 uses the server default. Applies only to MCP services that use OAuth.
  mcp_auth_wait_timeout?: number;

  // ===== Skills settings (agent mode only) =====
  // Skills selection: all = every preinstalled skill, selected = listed, none = no skills
  skills_selection_mode?: 'all' | 'selected' | 'none';
  selected_skills?: string[];       // selected skill names

  // ===== Sandbox settings =====
  // Sandbox config this agent's skill scripts run on; empty disables sandbox execution.
  // Points at the logical config, not a version, so rotating credentials needs no reassignment per agent.
  sandbox_config_id?: string;

  // ===== Knowledge base settings =====
  // Knowledge base selection: all = every knowledge base, selected = listed ones, none = no knowledge base
  kb_selection_mode?: 'all' | 'selected' | 'none';
  knowledge_bases?: string[];
  // Retrieve from knowledge bases only on an explicit @ mention (default: false)
  // true: retrieve only when the user @ mentions a knowledge base or document
  // false: retrieve automatically per kb_selection_mode
  retrieve_kb_only_when_mentioned?: boolean;

  // ===== Image upload / multimodal settings =====
  image_upload_enabled?: boolean;    // default: false
  vlm_model_id?: string;            // VLM model ID for image analysis
  image_storage_provider?: string;
  audio_upload_enabled?: boolean;    // audio upload / ASR transcription (default: false)
  asr_model_id?: string;            // ASR model ID for audio transcription
  // Attachment image understanding / scanned-document OCR (default: false; enabling it slows parsing)
  attachment_image_understanding?: boolean;
  // Max pages for scanned-document OCR (0 = global default AIKB_CHAT_ATTACHMENT_OCR_MAX_PAGES)
  attachment_ocr_max_pages?: number;
  // Max seconds one turn waits for attachment parsing (0 = global default AIKB_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC)
  attachment_parse_wait_timeout_sec?: number;

  // ===== Chat attachment parser engine rules =====
  // Parser engine per file type; precedence: request parser_engine > agent rules > workspace rules > auto
  chat_parser_engine_rules?: { file_types: string[]; engine: string }[];

  // ===== File type restriction =====
  // Supported file types (e.g. ["csv", "xlsx", "xls"])
  // Empty means all file types
  supported_file_types?: string[];

  // ===== Web search settings =====
  web_search_enabled?: boolean;
  web_search_provider_id?: string;
  web_search_max_results?: number;

  // ===== Multi-turn settings =====
  multi_turn_enabled?: boolean;
  history_turns?: number;           // history turns kept

  // ===== Long-term memory =====
  // Whether this agent may read the user's long-term memory.
  // Unset (old data) means true: this switch can only turn it off; when the workspace setting is off,
  // turning it on here has no effect.
  memory_enabled?: boolean;

  // ===== Retrieval settings =====
  embedding_top_k?: number;         // embedding recall top K
  keyword_threshold?: number;       // keyword recall threshold
  vector_threshold?: number;        // vector recall threshold
  rerank_top_k?: number;            // rerank top K
  rerank_threshold?: number;        // rerank threshold

  // ===== Advanced settings (mostly Quick Answer) =====
  enable_query_expansion?: boolean; // query expansion
  enable_rewrite?: boolean;         // question rewrite
  rewrite_prompt_system?: string;   // rewrite system prompt
  rewrite_prompt_user?: string;     // rewrite user prompt template
  fallback_strategy?: 'fixed' | 'model'; // fallback strategy
  fallback_response?: string;       // fixed fallback response
  fallback_prompt?: string;         // fallback prompt (when the model generates it)
  // Intent prompts: override the main system prompt for non-retrieval intents (greetings, chitchat, etc.)
  intent_prompts?: Record<string, string>;

  // ===== Deprecated fields (kept for compatibility) =====
  welcome_message?: string;
  question_suggestions?: QuestionSuggestionConfig;
}

export interface CustomAgent {
  id: string;
  name: string;
  description?: string;
  avatar?: string;
  is_builtin: boolean;
  tenant_id?: number;
  created_by?: string;
  // creator_name is backfilled by the backend list endpoint; used only for the source badge on list cards.
  creator_name?: string;
  config: CustomAgentConfig;
  created_at?: string;
  updated_at?: string;
}

export interface CreateAgentRequest {
  name: string;
  description?: string;
  avatar?: string;
  config?: CustomAgentConfig;
}

export interface UpdateAgentRequest {
  name: string;
  description?: string;
  avatar?: string;
  config?: CustomAgentConfig;
}

// Built-in agent IDs
export const BUILTIN_QUICK_ANSWER_ID = 'builtin-quick-answer';
export const BUILTIN_SMART_REASONING_ID = 'builtin-smart-reasoning';

export const AGENT_MODE_QUICK_ANSWER = 'quick-answer';
export const AGENT_MODE_SMART_REASONING = 'smart-reasoning';

// Deprecated: Use BUILTIN_QUICK_ANSWER_ID instead
export const BUILTIN_AGENT_NORMAL_ID = BUILTIN_QUICK_ANSWER_ID;
// Deprecated: Use BUILTIN_SMART_REASONING_ID instead
export const BUILTIN_AGENT_AGENT_ID = BUILTIN_SMART_REASONING_ID;

// Lists agents, built-in ones included.
// disabled_own_agent_ids: "my" agents the current workspace hides from the conversation dropdown; affects only this workspace
export function listAgents(params?: {
  /**
   * Optional creator filter; mirrors listKnowledgeBases. Built-in agents
   * (is_builtin=true) are always returned regardless of this filter so
   * the conversation dropdown never silently loses quick-answer /
   * smart-reasoning when a user picks "Created by me".
   */
  creator?: 'all' | 'mine' | 'others';
}) {
  const qs = params?.creator && params.creator !== 'all' ? `?creator=${params.creator}` : '';
  return get<{ data: CustomAgent[]; disabled_own_agent_ids?: string[] }>(`/api/v1/agents${qs}`);
}

export function getAgentById(id: string) {
  return get<{ data: CustomAgent }>(`/api/v1/agents/${id}`);
}

export function createAgent(data: CreateAgentRequest) {
  return post<{ data: CustomAgent }>('/api/v1/agents', data);
}

export function updateAgent(id: string, data: UpdateAgentRequest) {
  return put<{ data: CustomAgent }>(`/api/v1/agents/${id}`, data);
}

export function deleteAgent(id: string) {
  return del<{ success: boolean }>(`/api/v1/agents/${id}`);
}

export function copyAgent(id: string) {
  return post<{ data: CustomAgent }>(`/api/v1/agents/${id}/copy`);
}

// Built-in agent IDs start with 'builtin-'.
export function isBuiltinAgent(agentId: string): boolean {
  return agentId.startsWith('builtin-');
}

export interface PlaceholderDefinition {
  name: string;
  label: string;
  description: string;
}

export interface PlaceholdersResponse {
  all: PlaceholderDefinition[];
  system_prompt: PlaceholderDefinition[];
  agent_system_prompt: PlaceholderDefinition[];
  context_template: PlaceholderDefinition[];
  rewrite_system_prompt: PlaceholderDefinition[];
  rewrite_prompt: PlaceholderDefinition[];
  fallback_prompt: PlaceholderDefinition[];
}

export function getPlaceholders() {
  return get<{ data: PlaceholdersResponse }>('/api/v1/agents/placeholders');
}

// ===== Agent type presets =====

// Backend kb_filter shape (see internal/types/agent_type_preset.go)
export interface AgentTypeKBFilter {
  any_of?: string[];   // KB has at least one
  all_of?: string[];   // KB has all
  none_of?: string[];  // KB has none
}

// KB capability flags (JSON of backend types.KBCapabilities)
export interface KBCapabilities {
  vector: boolean;
  keyword: boolean;
  wiki: boolean;
  graph: boolean;
  faq: boolean;
}

// Preset auto-fill payload: only the fields the preset overrides; other fields are untouched
export interface AgentTypePresetConfig {
  system_prompt_id?: string;
  temperature?: number;
  max_iterations?: number;
  allowed_tools?: string[];
  retain_retrieval_history?: boolean;
  faq_priority_enabled?: boolean;
  web_search_enabled?: boolean;
  supported_file_types?: string[];
  kb_selection_mode?: 'all' | 'selected' | 'none';
}

export interface AgentTypePresetI18n {
  label: string;
  description: string;
}

export interface AgentTypePreset {
  id: AgentType;
  i18n: Record<string, AgentTypePresetI18n>;
  config?: AgentTypePresetConfig;     // empty means the "custom" type (no preset)
  kb_filter?: AgentTypeKBFilter;      // empty means every KB is selectable
}

// Type preset list for the editor
export function getAgentTypePresets() {
  return get<{ data: AgentTypePreset[] }>('/api/v1/agents/type-presets');
}

// ===== IM channels =====

export interface IMChannel {
  id: string;
  tenant_id?: number;
  agent_id: string;
  // 'lark' is Feishu's international edition; it shares Feishu's credentials and modes.
  platform: 'wecom' | 'feishu' | 'lark' | 'slack' | 'telegram' | 'dingtalk' | 'mattermost' | 'wechat' | 'qqbot' | 'yunzhijia';
  name: string;
  enabled: boolean;
  mode: 'webhook' | 'websocket' | 'longpoll';
  output_mode: 'stream' | 'full';
  locale?: '' | 'en-US';
  session_mode?: 'user' | 'thread';
  knowledge_base_id?: string;
  credentials: Record<string, any>;
  created_at?: string;
  updated_at?: string;
}

export function listIMChannels(agentId: string) {
  return get<{ data: IMChannel[] }>(`/api/v1/agents/${agentId}/im-channels`);
}

// Tenant-wide overview row. Credentials are intentionally omitted — use
// listIMChannels(agentId) when you need to edit a specific channel.
export interface IMChannelOverview {
  id: string;
  tenant_id: number;
  agent_id: string;
  agent_name: string; // localized built-in name when the agent is built-in
  platform: IMChannel['platform'];
  name: string;
  enabled: boolean;
  mode: IMChannel['mode'];
  output_mode: IMChannel['output_mode'];
  locale?: IMChannel['locale'];
  session_mode?: IMChannel['session_mode'];
  bot_identity: string;
  created_at: string;
  updated_at: string;
}

export function listAllIMChannels() {
  return get<{ data: IMChannelOverview[] }>('/api/v1/im-channels');
}

export function createIMChannel(agentId: string, data: Partial<IMChannel>) {
  return post<{ data: IMChannel }>(`/api/v1/agents/${agentId}/im-channels`, data);
}

export function updateIMChannel(id: string, data: Partial<IMChannel>) {
  return put<{ data: IMChannel }>(`/api/v1/im-channels/${id}`, data);
}

export function deleteIMChannel(id: string) {
  return del<{ success: boolean }>(`/api/v1/im-channels/${id}`);
}

export function toggleIMChannel(id: string) {
  return post<{ data: IMChannel }>(`/api/v1/im-channels/${id}/toggle`);
}

// ===== Suggested questions =====

export interface SuggestedQuestion {
  question: string;
  source: 'faq' | 'document' | 'agent_config' | 'wiki';
  knowledge_base_id?: string;
  knowledge_id?: string;
}

// Suggested questions for an agent, scoped to its knowledge bases; quick prompts in the chat panel
export function getSuggestedQuestions(
  agentId: string,
  params?: {
    knowledge_base_ids?: string[];
    knowledge_ids?: string[];
    tag_scopes?: Array<{ knowledge_base_id: string; tag_ids: string[] }>;
    limit?: number;
  }
) {
  const query = new URLSearchParams();
  if (params?.knowledge_base_ids?.length) query.set('knowledge_base_ids', params.knowledge_base_ids.join(','));
  if (params?.knowledge_ids?.length) query.set('knowledge_ids', params.knowledge_ids.join(','));
  if (params?.tag_scopes?.length) query.set('tag_scopes', JSON.stringify(params.tag_scopes));
  if (params?.limit) query.set('limit', String(params.limit));
  const qs = query.toString();
  return get<{ data: { questions: SuggestedQuestion[] } }>(`/api/v1/agents/${agentId}/suggested-questions${qs ? '?' + qs : ''}`);
}
// ===== WeChat QR Code Login =====

export interface WeChatQRCodeResult {
  qrcode_url: string;
  qrcode: string;
}

export interface WeChatQRCodeStatus {
  status: 'wait' | 'scaned' | 'confirmed' | 'expired';
  credentials?: {
    bot_token: string;
    ilink_bot_id: string;
    ilink_user_id: string;
  };
  baseurl?: string;
}

export function getWeChatQRCode() {
  return post<{ data: WeChatQRCodeResult }>('/api/v1/wechat/qrcode');
}

export function pollWeChatQRCodeStatus(qrcode: string) {
  return post<{ data: WeChatQRCodeStatus }>('/api/v1/wechat/qrcode/status', { qrcode });
}
