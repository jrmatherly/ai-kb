// Wire shapes mirror the JSON tags in internal/handler/auth.go, internal/handler/dto/auth.go,
// internal/types/user.go and internal/types/search.go.

export type Auth =
  | { kind: 'api_key'; key: string }
  | { kind: 'bearer'; token: string; refreshToken: string }
  | { kind: 'signed_out' };

export type Connection = {
  serverUrl: string;
  auth: Auth;
  tenantId: number | null;
  lastKbId: string | null;
  quickClip: boolean;
};

export type Reference = {
  id: string;
  knowledge_id: string;
  knowledge_base_id?: string;
  knowledge_title: string;
  content: string;
  score?: number;
};

export type StreamEvent = {
  response_type: string;
  content?: string;
  done?: boolean;
  id?: string;
  data?: Record<string, unknown>;
  knowledge_references?: Reference[];
  session_id?: string;
  assistant_message_id?: string;
};

export type KnowledgeBase = { id: string; name: string; type?: string };

export type SharedKnowledgeBase = {
  knowledge_base: KnowledgeBase | null;
  permission: 'viewer' | 'editor' | 'admin';
};

export type Agent = {
  id: string;
  name: string;
  config: { agent_mode: string; kb_selection_mode: 'all' | 'none' | 'selected'; knowledge_bases?: string[] };
};

export type Session = { id: string; title: string; updated_at: string };

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  created_at: string;
  knowledge_references?: Reference[];
};

export type Knowledge = {
  id: string;
  title: string;
  metadata?: string | { content?: string; status?: string };
  parse_status: string;
  error_message?: string;
};

export type CaptureDraft = {
  kind: 'article' | 'select' | 'note';
  title: string;
  url: string;
  markdown: string;
  meta: { author?: string; published?: string; site?: string };
};

export type Capture = {
  knowledgeId: string;
  kbId: string;
  title: string;
  url: string;
  savedAt: string;
  status: 'pending' | 'completed' | 'failed' | 'cancelled' | 'timeout';
  // The server and workspace it was saved under; a capture is listed only there.
  scope?: string;
};

export type Intent = 'clip:article' | 'clip:select' | 'quick:article' | 'quick:select' | 'note' | 'ask';

export type Membership = { tenant_id: number; tenant_name: string; role?: string };

export type Me = {
  user: { id: string; email: string };
  tenant: { id: number; name: string } | null;
  memberships: Membership[];
};

export type LoginResponse = {
  success: boolean;
  token: string;
  refresh_token: string;
  active_tenant?: { id: number };
  memberships: Membership[];
};

export type RefreshResponse = { success: boolean; access_token: string; refresh_token: string };

export type OidcConfig = { success: boolean; enabled: boolean; provider_display_name?: string };

export type OidcResult = {
  success: boolean;
  message?: string;
  tenant?: { id: number };
  memberships: Membership[];
  token: string;
  refresh_token: string;
  is_new_user?: boolean;
};
