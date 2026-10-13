import { get, post, postUpload, put, del } from '../../utils/request';
import i18n from '@/i18n'
import { ModelInUseError, modelInUseErrorFromRequest } from './modelUsage'

export * from './modelUsage'

const t = (key: string) => i18n.global.t(key)

// Protocol-neutral thinking level. Mirrors internal/models/api.ReasoningEffort.
export type ReasoningEffortLevel =
  | 'off'
  | 'auto'
  | 'minimal'
  | 'low'
  | 'medium'
  | 'high'
  | 'xhigh'
  | 'max'

// Catalog view of a saved chat/VLM model. Mirrors internal/models/catalog.Capabilities.
// thinking_levels: empty array = the model cannot be asked to think.
export interface ModelCapabilities {
  provider: string;
  api: string;
  cataloged: boolean;
  reasoning: boolean;
  thinking_levels: ReasoningEffortLevel[];
  thinking_format: string;
  input?: string[];
  context_window?: number;
  max_output_tokens?: number;
  max_tokens_field?: string;
}

// Per-row override of a catalog entry. Mirrors internal/types.ModelSpecOverride.
// compat is the flat, protocol-specific object defined in
// internal/models/api/*_settings.go (free-form JSON).
export interface ModelSpecOverride {
  api?: string;
  reasoning?: boolean;
  input?: string[];
  context_window?: number;
  max_output_tokens?: number;
  thinking_levels?: Record<string, string | null>;
  compat?: Record<string, unknown>;
}

// Model type definitions
export interface ModelConfig {
  id?: string;
  tenant_id?: number;
  name: string;
  display_name?: string;
  type: 'KnowledgeQA' | 'Embedding' | 'Rerank' | 'VLLM' | 'ASR';
  source: 'local' | 'remote';
  description?: string;
  parameters: {
    base_url?: string;
    api_key?: string;
    provider?: string; // Provider identifier: openai, aliyun, zhipu, generic
    embedding_parameters?: {
      dimension?: number;
      truncate_prompt_tokens?: number;
      supports_dimension_override?: boolean;
    };
    interface_type?: 'ollama' | 'openai'; // VLLM only
    parameter_size?: string; // Ollama model parameter size (e.g., "7B", "13B", "70B")
    extra_config?: Record<string, string>; // Provider-specific configuration
    // Custom HTTP request headers (like extra_headers in the Python OpenAI SDK),
    // added to every request when calling the remote model API. Reserved headers such as Authorization and Content-Type are ignored.
    custom_headers?: Record<string, string>;
    supports_vision?: boolean; // Whether the model accepts image/multimodal input
    // Context window (tokens) for chat and VLM. 0 or unset means the backend default of 200000.
    context_window?: number;
    max_output_tokens?: number;
    // Concurrency cap for background tasks (ingestion, enrichment) on this model, shared by model ID across all replicas.
    // 0 or unset means the global default (model.max_concurrency); applies only to chat, embedding and vllm.
    max_concurrency?: number;
    app_id?: string;
    // Secret fields (api_key, app_secret) are never returned by the server in
    // this shape - they live behind the /credentials subresource. They are
    // kept on the type so create-mode payloads can still carry them in the
    // initial POST body.
    app_secret?: string;
    // Per-model catalog override (protocol, limits, protocol compat knobs).
    spec?: ModelSpecOverride;
  };
  // Catalog view (chat / VLM remote models only): protocol, thinking levels,
  // context window. Computed by the backend from provider + name + overrides.
  capabilities?: ModelCapabilities;
  is_default?: boolean;
  is_builtin?: boolean;
  status?: string;
  // Per-field configured? metadata from the main response. For builtin
  // models it is returned only to system administrators.
  credentials?: Record<ModelCredentialField, { configured: boolean }>;
  created_at?: string;
  updated_at?: string;
  deleted_at?: string | null;
}

// Copy a model. Only the display name is sent; the server copies name and credentials from the source model.
export function copyModelConfig(id: string, displayName: string): Promise<ModelConfig> {
  return new Promise((resolve, reject) => {
    post(`/api/v1/models/${id}/copy`, { display_name: displayName })
      .then((response: any) => {
        if (response.success && response.data) {
          resolve(response.data);
        } else {
          reject(new Error(response.message || ''));
        }
      })
      .catch((error: any) => {
        console.error('Failed to copy model:', error);
        reject(error);
      });
  });
}

// Create a model
export function createModel(data: ModelConfig): Promise<ModelConfig> {
  return new Promise((resolve, reject) => {
    post('/api/v1/models', data)
      .then((response: any) => {
        if (response.success && response.data) {
          resolve(response.data);
        } else {
          reject(new Error(response.message || t('error.model.createFailed')));
        }
      })
      .catch((error: any) => {
        console.error('Failed to create model:', error);
        reject(error);
      });
  });
}

// List models
export function listModels(type?: string): Promise<ModelConfig[]> {
  return new Promise((resolve, reject) => {
    const url = `/api/v1/models`;
    get(url)
      .then((response: any) => {
        if (response.success && response.data) {
          if (type) {
            response.data = response.data.filter((item: ModelConfig) => item.type === type);
          }
          resolve(response.data);
        } else {
          resolve([]);
        }
      })
      .catch((error: any) => {
        console.error('Failed to list models:', error);
        // Throw instead of swallowing, so callers (the cache layer included) can tell a real failure from "succeeded with no models"
        // and do not cache the empty result of a transient failure. Every UI call site already catches it.
        reject(error);
      });
  });
}

// Get one model
export function getModel(id: string): Promise<ModelConfig> {
  return new Promise((resolve, reject) => {
    get(`/api/v1/models/${id}`)
      .then((response: any) => {
        if (response.success && response.data) {
          resolve(response.data);
        } else {
          reject(new Error(response.message || t('error.model.getFailed')));
        }
      })
      .catch((error: any) => {
        console.error('Failed to get model:', error);
        reject(error);
      });
  });
}

// Update a model
export function updateModel(id: string, data: Partial<ModelConfig>): Promise<ModelConfig> {
  return new Promise((resolve, reject) => {
    put(`/api/v1/models/${id}`, data)
      .then((response: any) => {
        if (response.success && response.data) {
          resolve(response.data);
        } else {
          reject(new Error(response.message || t('error.model.updateFailed')));
        }
      })
      .catch((error: any) => {
        console.error('Failed to update model:', error);
        reject(error);
      });
  });
}

// Delete a model
export function deleteModel(id: string): Promise<void> {
  return new Promise((resolve, reject) => {
    del(`/api/v1/models/${id}`)
      .then((response: any) => {
        if (response.success) {
          resolve();
        } else {
          const conflict = modelInUseErrorFromRequest(response)
          if (conflict) {
            reject(conflict)
            return
          }
          reject(new Error(response.message || t('error.model.deleteFailed')));
        }
      })
      .catch((error: any) => {
        console.error('Failed to delete model:', error);
        if (error instanceof ModelInUseError) {
          reject(error)
          return
        }
        const conflict = modelInUseErrorFromRequest(error)
        if (conflict) {
          reject(conflict)
          return
        }
        reject(error);
      });
  });
}

export interface ModelDebugOptions {
  system_prompt?: string
  temperature?: number
  top_p?: number
  max_tokens?: number
  thinking?: boolean
  // Graded thinking level; takes precedence over the boolean when set.
  reasoning_effort?: ReasoningEffortLevel | string
  // Embedding only: score `documents` (and the file) against `input` as a query.
  similarity?: boolean
}

export interface ModelDebugResult {
  ok: boolean
  elapsed_ms: number
  request: Record<string, unknown>
  raw_response: unknown
  observations: Record<string, unknown>
  error?: string
}

export async function debugModel(
  id: string,
  data: {
    input?: string
    documents?: string[]
    options?: ModelDebugOptions
    file?: File | null
  },
): Promise<ModelDebugResult> {
  const form = new FormData()
  form.append('input', data.input || '')
  form.append('documents', JSON.stringify(data.documents || []))
  form.append('options', JSON.stringify(data.options || {}))
  if (data.file) form.append('file', data.file)
  const response: any = await postUpload(
    `/api/v1/models/${id}/debug`,
    form,
    undefined,
    { timeout: 300000 },
  )
  if (response?.success && response?.data) return response.data
  throw new Error(response?.message || t('error.model.getFailed'))
}

// ----------------------------------------------------------------------------
// Model credential subresource. See mcp-service.ts for the matching MCP API
// shape and the design notes in internal/handler/dto/mcp.go.
// ----------------------------------------------------------------------------

export type ModelCredentialField = 'api_key' | 'app_secret'

export interface ModelCredentialsResponse {
  fields: Record<ModelCredentialField, { configured: boolean }>
}

export async function putModelCredentials(
  id: string,
  body: Partial<Record<ModelCredentialField, string>>,
): Promise<ModelCredentialsResponse> {
  const response: any = await put(`/api/v1/models/${id}/credentials`, body)
  return (response.data ?? response) as ModelCredentialsResponse
}

export async function deleteModelCredentialField(
  id: string,
  field: ModelCredentialField,
): Promise<void> {
  await del(`/api/v1/models/${id}/credentials/${field}`)
}

export interface InitializeWeKnoraCloudRequest {
  app_id: string
  app_secret: string
}

// Save only the WeKnoraCloud credentials, without creating models
export function saveWeKnoraCloudCredentials(data: InitializeWeKnoraCloudRequest): Promise<{ success: boolean; message: string }> {
  return new Promise((resolve, reject) => {
    post('/api/v1/weknoracloud/credentials', data)
      .then((response: any) => {
        if (response.success) {
          resolve(response)
        } else {
          reject(new Error(response.message || response.error || 'Failed to save credentials'))
        }
      })
      .catch((error: any) => {
        console.error('Failed to save WeKnoraCloud credentials:', error)
        reject(error)
      })
  })
}

export interface WeKnoraCloudStatusResult {
  has_models: boolean
  needs_reinit: boolean
  reason?: string
}

export function getWeKnoraCloudStatus(): Promise<WeKnoraCloudStatusResult> {
  return new Promise((resolve, reject) => {
    get('/api/v1/models/weknoracloud/status')
      .then((response: any) => {
        // The status endpoint returns the object directly, not wrapped in success/data
        if (response && typeof response.has_models === 'boolean') {
          resolve(response)
        } else if (response?.success && response?.data) {
          resolve(response.data)
        } else {
          resolve({ has_models: false, needs_reinit: false })
        }
      })
      .catch(() => {
        resolve({ has_models: false, needs_reinit: false })
      })
  })
}
