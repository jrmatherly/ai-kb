import { get, post, put } from '../../utils/request';
import i18n from '@/i18n'
import type { ModelCapabilities, ModelSpecOverride, ReasoningEffortLevel } from '../model'

const t = (key: string) => i18n.global.t(key)

// GET /initialization/config/:kbId exposes credential presence, not values.
export interface ModelCredentialStatus {
    apiKey?: boolean;
}

export interface COSCredentialStatus {
    secretId?: boolean;
    secretKey?: boolean;
}

// Initialization config data types
export interface InitializationConfig {
    llm: {
        source: string;
        modelName: string;
        baseUrl?: string;
        /** @deprecated Use credentials.apiKey from GET responses */
        apiKey?: string;
        credentials?: ModelCredentialStatus;
    };
    embedding: {
        source: string;
        modelName: string;
        baseUrl?: string;
        /** @deprecated Use credentials.apiKey from GET responses */
        apiKey?: string;
        dimension?: number; // embedding dimension field
        credentials?: ModelCredentialStatus;
    };
    rerank: {
        modelName: string;
        baseUrl: string;
        /** @deprecated Use credentials.apiKey from GET responses */
        apiKey?: string;
        enabled: boolean;
        credentials?: ModelCredentialStatus;
    };
    multimodal: {
        enabled: boolean;
        storageType: 'cos' | 'minio';
        vlm?: {
            modelName: string;
            baseUrl: string;
            /** @deprecated Use credentials.apiKey from GET responses */
            apiKey?: string;
            interfaceType?: string; // "ollama" or "openai"
            credentials?: ModelCredentialStatus;
        };
        cos?: {
            region: string;
            bucketName: string;
            appId: string;
            pathPrefix?: string;
            /** @deprecated Use credentials from GET responses */
            secretId?: string;
            /** @deprecated Use credentials from GET responses */
            secretKey?: string;
            credentials?: COSCredentialStatus;
        };
        minio?: {
            bucketName: string;
            pathPrefix?: string;
        };
    };
    documentSplitting: {
        chunkSize: number;
        chunkOverlap: number;
        separators: string[];
        // Adaptive chunking strategy. Empty / "legacy" = classic recursive splitter.
        // "auto" lets the backend profiler pick a tier; "heading" / "heuristic"
        // pin the tier explicitly. See backend chunker package for details.
        strategy?: string;
        // Cap chunk size in approx tokens. 0 = char-based budget only.
        tokenLimit?: number;
        // Language hints for heuristic patterns ("de", "en", "zh"). Empty = auto-detect.
        languages?: string[];
    };
    // Frontend-only hint for storage selection UI
    storageType?: 'cos' | 'minio';
    nodeExtract: {
        enabled: boolean,
        text: string,
        tags: string[],
        nodes: Node[],
        relations: Relation[]
    }
}

// Download task status type
export interface DownloadTask {
    id: string;
    modelName: string;
    status: 'pending' | 'downloading' | 'completed' | 'failed';
    progress: number;
    message: string;
    startTime: string;
    endTime?: string;
}

// Simplified knowledge base config update API (model IDs only)
export interface KBModelConfigRequest {
    llmModelId: string
    embeddingModelId: string
    vlm_config?: {
        enabled: boolean
        model_id?: string
        description_language?: string
        custom_instructions?: string
    }
    asr_config?: {
        enabled: boolean
        model_id?: string
        language?: string
    }
    documentSplitting: {
        chunkSize: number
        chunkOverlap: number
        separators: string[]
        parserEngineRules?: {
            file_types: string[]
            engine: string
            xlsx_first_row_as_header?: boolean
        }[]
        enableParentChild?: boolean
        parentChunkSize?: number
        childChunkSize?: number
        // Adaptive chunking strategy ("auto" | "heading" | "heuristic" | "legacy").
        // The backend uses pointer-based DTOs for these three fields:
        // - undefined / not set in payload → no change on server
        // - "" / 0 / [] explicitly sent     → clears the value
        // Send the field whenever the user has opened the editor - even
        // empty values - so the user can always reset back to defaults.
        strategy?: string
        // Approximate token budget per chunk; 0 = char-based.
        tokenLimit?: number
        // Language hints for heuristic patterns. Empty array = auto-detect.
        languages?: string[]
        tableMetadataInstructions?: string
    }
    multimodal: {
        enabled: boolean
    }
    /** Storage engine choice: "local" | "minio" | "cos" | "obs" and so on; affects document uploads and storage of images inside documents */
    storageBackendId?: string
    storageProvider?: string
    nodeExtract: {
        enabled: boolean
        text: string
        tags: string[]
        nodes: Node[]
        relations: Relation[]
        customInstructions?: string
    }
    questionGeneration?: {
        enabled: boolean
        questionCount: number
        customInstructions?: string
    }
}

export function updateKBConfig(kbId: string, config: KBModelConfigRequest): Promise<any> {
    return new Promise((resolve, reject) => {
        console.log('Starting KB config update (simplified)...', kbId, config);
        put(`/api/v1/initialization/config/${kbId}`, config)
            .then((response: any) => {
                console.log('KB config update completed', response);
                resolve(response);
            })
            .catch((error: any) => {
                console.error('Failed to update KB config:', error);
                reject(error.error || error);
            });
    });
}

// Run a config update by knowledge base ID (legacy, kept for compatibility)
export function initializeSystemByKB(kbId: string, config: InitializationConfig): Promise<any> {
    return new Promise((resolve, reject) => {
        console.log('Starting KB config update...', kbId, config);
        post(`/api/v1/initialization/initialize/${kbId}`, config)
            .then((response: any) => {
                console.log('KB config update completed', response);
                resolve(response);
            })
            .catch((error: any) => {
                console.error('Failed to update KB config:', error);
                reject(error.error || error);
            });
    });
}

// Check the Ollama service status
export function checkOllamaStatus(): Promise<{ available: boolean; version?: string; error?: string; baseUrl?: string }> {
    return new Promise((resolve, reject) => {
        get('/api/v1/initialization/ollama/status')
            .then((response: any) => {
                resolve(response.data || { available: false });
            })
            .catch((error: any) => {
                console.error('Failed to check Ollama status:', error);
                resolve({ available: false, error: error.message || t('error.initialization.checkFailed') });
            });
    });
}

// Ollama model details API
export interface OllamaModelInfo {
    name: string;
    size: number;
    digest: string;
    modified_at: string;
}

// List installed Ollama models (with details)
export function listOllamaModels(): Promise<OllamaModelInfo[]> {
    return new Promise((resolve, reject) => {
        get('/api/v1/initialization/ollama/models')
            .then((response: any) => {
                resolve((response.data && response.data.models) || []);
            })
            .catch((error: any) => {
                console.error('Failed to list Ollama models:', error);
                resolve([]);
            });
    });
}

// Check the Ollama model status
export function checkOllamaModels(models: string[]): Promise<{ models: Record<string, boolean> }> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/ollama/models/check', { models })
            .then((response: any) => {
                resolve(response.data || { models: {} });
            })
            .catch((error: any) => {
                console.error('Failed to check Ollama models:', error);
                reject(error);
            });
    });
}

// Start an Ollama model download (async)
export function downloadOllamaModel(modelName: string): Promise<{ taskId: string; modelName: string; status: string; progress: number }> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/ollama/models/download', { modelName })
            .then((response: any) => {
                resolve(response.data || { taskId: '', modelName, status: 'failed', progress: 0 });
            })
            .catch((error: any) => {
                console.error('Failed to start Ollama model download:', error);
                reject(error);
            });
    });
}

// Query download progress
export function getDownloadProgress(taskId: string): Promise<DownloadTask> {
    return new Promise((resolve, reject) => {
        get(`/api/v1/initialization/ollama/download/progress/${taskId}`)
            .then((response: any) => {
                resolve(response.data);
            })
            .catch((error: any) => {
                console.error('Failed to get download progress:', error);
                reject(error);
            });
    });
}

// Get all download tasks
export function listDownloadTasks(): Promise<DownloadTask[]> {
    return new Promise((resolve, reject) => {
        get('/api/v1/initialization/ollama/download/tasks')
            .then((response: any) => {
                resolve(response.data || []);
            })
            .catch((error: any) => {
                console.error('Failed to list download tasks:', error);
                reject(error);
            });
    });
}


export function getCurrentConfigByKB(kbId: string): Promise<InitializationConfig & { hasFiles: boolean }> {
    return new Promise((resolve, reject) => {
        get(`/api/v1/initialization/config/${kbId}`)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to get KB config:', error);
                reject(error);
            });
    });
}

// Shared optional params for every "test connection" API.
// customHeaders / extraConfig / interfaceType match the fields of the same name in the backend ModelTestRequest,
// and are passed through to the real model assembly flow, so the connection test takes exactly the same path as production calls.
interface BaseModelTestPayload {
    spec?: ModelSpecOverride;
    customHeaders?: Record<string, string>;
    extraConfig?: Record<string, string>;
    interfaceType?: string;
    /** Second key (such as the Tencent Cloud SecretKey for LKEAP Rerank) */
    appSecret?: string;
}

// Check a remote API model
export function checkRemoteModel(modelConfig: {
    modelName: string;
    baseUrl: string;
    apiKey?: string;
    provider?: string;
    // When editing an existing model, send modelId and the backend fills in the apiKey from storage
    // (the frontend no longer shows the plaintext key, so the connection test must use this backfill path)
    modelId?: string;
} & BaseModelTestPayload): Promise<{
    available: boolean;
    message?: string;
}> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/remote/check', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to check remote model:', error);
                reject(error);
            });
    });
}

// Test whether an embedding model (local or remote) works
export function testEmbeddingModel(modelConfig: {
    source: 'local' | 'remote';
    modelName: string;
    baseUrl?: string;
    apiKey?: string;
    dimension?: number;
    supportsDimensionOverride?: boolean;
    provider?: string;
    modelId?: string;
} & BaseModelTestPayload): Promise<{ available: boolean; message?: string; dimension?: number }> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/embedding/test', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to test Embedding model:', error);
                reject(error);
            });
    });
}


export function checkRerankModel(modelConfig: {
    modelName: string;
    baseUrl: string;
    apiKey?: string;
    provider?: string;
    modelId?: string;
} & BaseModelTestPayload): Promise<{
    available: boolean;
    message?: string;
}> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/rerank/check', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to check Rerank model:', error);
                reject(error);
            });
    });
}

// Check the ASR model connection (tested through the /v1/audio/transcriptions endpoint)
export function checkASRModel(modelConfig: {
    modelName: string;
    baseUrl: string;
    apiKey?: string;
    provider?: string;
    modelId?: string;
} & BaseModelTestPayload): Promise<{
    available: boolean;
    message?: string;
}> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/asr/check', modelConfig)
            .then((response: any) => {
                resolve(response.data || {});
            })
            .catch((error: any) => {
                console.error('Failed to check ASR model:', error);
                reject(error);
            });
    });
}

export function testMultimodalFunction(testData: {
    image: File;
    vlm_model: string;
    vlm_base_url: string;
    vlm_api_key?: string;
    vlm_interface_type?: string;
    storage_type?: 'cos' | 'minio';
    // COS optional fields (required only when storage_type === 'cos')
    cos_secret_id?: string;
    cos_secret_key?: string;
    cos_region?: string;
    cos_bucket_name?: string;
    cos_app_id?: string;
    cos_path_prefix?: string;
    // MinIO optional fields
    minio_bucket_name?: string;
    minio_path_prefix?: string;
    chunk_size: number;
    chunk_overlap: number;
    separators: string[];
}): Promise<{
    success: boolean;
    caption?: string;
    ocr?: string;
    processing_time?: number;
    message?: string;
}> {
    return new Promise((resolve, reject) => {
        const formData = new FormData();
        formData.append('image', testData.image);
        formData.append('vlm_model', testData.vlm_model);
        formData.append('vlm_base_url', testData.vlm_base_url);
        if (testData.vlm_api_key) {
            formData.append('vlm_api_key', testData.vlm_api_key);
        }
        if (testData.vlm_interface_type) {
            formData.append('vlm_interface_type', testData.vlm_interface_type);
        }
        if (testData.storage_type) {
            formData.append('storage_type', testData.storage_type);
        }
        // Append COS fields only when storage_type is COS
        if (testData.storage_type === 'cos') {
            if (testData.cos_secret_id) formData.append('cos_secret_id', testData.cos_secret_id);
            if (testData.cos_secret_key) formData.append('cos_secret_key', testData.cos_secret_key);
            if (testData.cos_region) formData.append('cos_region', testData.cos_region);
            if (testData.cos_bucket_name) formData.append('cos_bucket_name', testData.cos_bucket_name);
            if (testData.cos_app_id) formData.append('cos_app_id', testData.cos_app_id);
            if (testData.cos_path_prefix) formData.append('cos_path_prefix', testData.cos_path_prefix);
        }
        // MinIO fields
        if (testData.minio_bucket_name) formData.append('minio_bucket_name', testData.minio_bucket_name);
        if (testData.minio_path_prefix) formData.append('minio_path_prefix', testData.minio_path_prefix);
        formData.append('chunk_size', testData.chunk_size.toString());
        formData.append('chunk_overlap', testData.chunk_overlap.toString());
        formData.append('separators', JSON.stringify(testData.separators));

        // Get the auth token
        const token = localStorage.getItem('aikb_token');
        const headers: Record<string, string> = {};
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        // Cross-space access header: always send it, to avoid the short-circuit "do not send when selectedTenantId
        // === defaultTenantId", which in some edge cases silently drops the header.
        // Same behavior as utils/request.ts and api/chat/streame.ts.
        const selectedTenantId = localStorage.getItem('aikb_selected_tenant_id');
        if (selectedTenantId) {
            headers['X-Tenant-ID'] = selectedTenantId;
        }

        // Use native fetch because FormData must be sent
        fetch('/api/v1/initialization/multimodal/test', {
            method: 'POST',
            headers,
            body: formData
        })
            .then(response => response.json())
            .then((data: any) => {
                if (data.success) {
                    resolve(data.data || {});
                } else {
                    resolve({ success: false, message: data.error?.message || data.message || t('error.initialization.testFailed') });
                }
            })
            .catch((error: any) => {
                console.error('Failed multimodal test:', error);
                reject(error);
            });
    });
}

// Text relation extraction API
export interface TextRelationExtractionRequest {
    text: string;
    tags: string[];
    model_id: string;
}

export interface Node {
    name: string;
    attributes: string[];
}

export interface Relation {
    node1: string;
    node2: string;
    type: string;
}

export interface TextRelationExtractionResponse {
    nodes: Node[];
    relations: Relation[];
}

// Text relation extraction
export function extractTextRelations(request: TextRelationExtractionRequest): Promise<TextRelationExtractionResponse> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/extract/text-relation', request, { timeout: 60000 })
            .then((response: any) => {
                resolve(response.data || { nodes: [], relations: [] });
            })
            .catch((error: any) => {
                console.error('Failed to extract text relations:', error);
                reject(error);
            });
    });
}

export interface FabriTextRequest {
    tags: string[];
    model_id: string;
}

export interface FabriTextResponse {
    text: string;
}

// Text generation
export function fabriText(request: FabriTextRequest): Promise<FabriTextResponse> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/extract/fabri-text', request)
            .then((response: any) => {
                resolve(response.data || { text: '' });
            })
            .catch((error: any) => {
                console.error('Failed to generate text:', error);
                reject(error);
            });
    });
}

export interface FabriTagRequest {
}

export interface FabriTagResponse {
    tags: string[];
}

// Tag generation
export function fabriTag(request: FabriTagRequest): Promise<FabriTagResponse> {
    return new Promise((resolve, reject) => {
        post('/api/v1/initialization/extract/fabri-tag', request)
            .then((response: any) => {
                resolve(response.data || { tags: [] as string[] });
            })
            .catch((error: any) => {
                console.error('Failed to generate tags:', error);
                reject(error);
            });
    });
}

// Vendor extra config fields (Azure api_version, LKEAP secret_key and so on).
// Mirrors internal/models/catalog.ExtraField.
export interface ModelProviderExtraFieldOption {
    label: string;
    labels?: Record<string, string>;
    value: string;
}

/**
 * Renames the primary credential input for vendors whose API does not take a
 * plain API key. LKEAP and Volcengine sign their rerank requests with a
 * CAM / IAM key pair, so the first field is a SecretId / Access Key ID.
 * The vendor declares the wording so the editor needs no vendor table.
 */
export interface ModelProviderCredentialLabel {
    label: string;
    labels?: Record<string, string>;
    placeholder?: string;
    placeholders?: Record<string, string>;
    hint?: string;
    hints?: Record<string, string>;
    // Backend model types ("KnowledgeQA", "Rerank", ...); empty = all.
    model_types?: string[];
    required?: boolean;
}

export interface ModelProviderExtraField {
    key: string;
    label: string;
    labels?: Record<string, string>;
    type: 'string' | 'number' | 'boolean' | 'select' | 'password' | string;
    required?: boolean;
    default?: string;
    placeholder?: string;
    placeholders?: Record<string, string>;
    options?: ModelProviderExtraFieldOption[];
    // Backend model types ("KnowledgeQA", "Embedding", "Rerank", "VLLM", "ASR"); empty = all.
    model_types?: string[];
    // Secret values are never echoed back; they are stored as the app_secret credential.
    secret?: boolean;
}

// Vendor built-in model catalog entry. Mirrors handler.ModelCatalogEntryDTO.
export interface ModelCatalogEntry {
    id: string;
    name: string;
    type: string;
    api?: string;
    reasoning?: boolean;
    input?: string[];
    context_window?: number;
    max_output_tokens?: number;
    dimension?: number;
    thinking_levels?: ReasoningEffortLevel[];
    cost?: Record<string, unknown>;
    // Vendor page these facts came from, for the "read the docs" link.
    source?: string;
}

export interface ModelProviderThinking {
    format: string;
    levels: ReasoningEffortLevel[];
}

// Model vendor info type. Mirrors handler.ModelProviderDTO - the frontend keeps no
// vendor table of its own; everything rendered for a vendor comes from here.
export interface ModelProviderOption {
    value: string;        // provider identifier
    label: string;        // display name (English / default)
    labels?: Record<string, string>;  // display names per locale
    description: string;  // description
    descriptions?: Record<string, string>;
    website?: string;
    icon?: string;        // data:image/svg+xml;base64,... usable directly in <img src>
    api?: string;
    auth?: string;
    requiresAuth?: boolean;
    defaultUrls: Record<string, string>;  // default URL per model type
    modelTypes: string[]; // supported model types
    extraFields?: ModelProviderExtraField[];
    credentialLabels?: ModelProviderCredentialLabel[];
    models?: ModelCatalogEntry[];
    thinking?: ModelProviderThinking;
    order?: number;
}

// List model vendors.
//
// Rejects on failure (no longer swallowed into an empty array): the only caller, stores/modelProviders, must
// tell "the backend really has no vendors" from "this request failed"; caching a failure as an empty list
// would leave the vendor dropdown empty for the whole session until the user refreshes the page.
export function listModelProviders(modelType?: string): Promise<ModelProviderOption[]> {
    const url = modelType
        ? `/api/v1/models/providers?model_type=${encodeURIComponent(modelType)}`
        : '/api/v1/models/providers';
    return get(url).then((response: any) => {
        const data = response?.data;
        // Reject rather than coerce: the store distinguishes "this vendor list
        // is genuinely empty" (cacheable) from "the request did not produce a
        // list" (retry on the next mount). Returning [] here would make a
        // malformed 200 look like the former and blank the vendor dropdown and
        // every card icon for the rest of the session.
        if (!Array.isArray(data)) {
            return Promise.reject(new Error('model providers response is not a list'));
        }
        return data as ModelProviderOption[];
    });
}

export interface ResolveModelCatalogParams {
    spec?: ModelSpecOverride;
    provider: string;
    model?: string;
    base_url?: string;
    model_type?: string;
    api?: string;
    thinking_control?: string;
    remote_model_name?: string;
    // Vendor-declared non-secret extra fields (Azure api_version, ...) are
    // forwarded by key so the preview resolves the same request the runtime
    // will make. The backend only accepts keys the vendor declares.
    [extraField: string]: string | ModelSpecOverride | undefined;
}

// Catalog resolution result. Mirrors handler.ResolveModelCatalog response data.
export interface ResolvedModelCatalog {
    provider: string;
    api: string;
    // base_url and url are only returned to callers who may configure
    // integrations; a viewer gets the capability answer without the endpoint.
    base_url?: string;
    // url is the endpoint the row will actually call, and only vendors that
    // compute their own URL report one (Azure, whose api_version picks
    // between the v1 data plane and the dated deployments path).
    url?: string;
    remote_model: string;
    cataloged: boolean;
    model: Record<string, unknown>;
    capabilities: ModelCapabilities;
}

// Resolve a model's effective access config (protocol, thinking levels, context window and so on) for live display in the editor.
export function resolveModelCatalog(params: ResolveModelCatalogParams): Promise<ResolvedModelCatalog> {
    return new Promise((resolve, reject) => {
        post(`/api/v1/models/catalog/resolve`, params)
            .then((response: any) => {
                if (response?.success && response?.data) {
                    resolve(response.data as ResolvedModelCatalog);
                } else {
                    reject(new Error(response?.message || 'resolve failed'));
                }
            })
            .catch((error: any) => {
                reject(error?.error || error);
            });
    });
}
