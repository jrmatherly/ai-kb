<template>
  <div class="ollama-settings">
    <div class="section-header">
      <h2>{{ $t('ollamaSettings.title') }}</h2>
      <p class="section-description">{{ $t('ollamaSettings.description') }}</p>
    </div>

    <div class="settings-group">
      <!-- Ollama service status -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('ollamaSettings.status.label') }}</label>
          <p class="desc">{{ $t('ollamaSettings.status.desc') }}</p>
        </div>
        <div class="setting-control">
          <div class="status-display">
            <t-tag 
              v-if="testing"
              theme="default"
              variant="light"
            >
              <t-icon name="loading" class="status-icon spinning" />
              {{ $t('ollamaSettings.status.testing') }}
            </t-tag>
            <t-tag 
              v-else-if="connectionStatus === true"
              theme="success"
              variant="light"
            >
              <t-icon name="check-circle-filled" />
              {{ $t('ollamaSettings.status.available') }}
            </t-tag>
            <t-tag 
              v-else-if="connectionStatus === false"
              theme="danger"
              variant="light"
            >
              <t-icon name="close-circle-filled" />
              {{ $t('ollamaSettings.status.unavailable') }}
            </t-tag>
            <t-tag 
              v-else
              theme="default"
              variant="light"
            >
              <t-icon name="help-circle" />
              {{ $t('ollamaSettings.status.untested') }}
            </t-tag>
            <t-button 
              size="small" 
              variant="text"
              :loading="testing"
              @click="testConnection"
            >
              <template #icon>
                <t-icon name="refresh" />
              </template>
              {{ $t('ollamaSettings.status.retest') }}
            </t-button>
          </div>
        </div>
      </div>

      <!-- Ollama service address -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('ollamaSettings.address.label') }}</label>
          <p class="desc">{{ $t('ollamaSettings.address.desc') }}</p>
        </div>
        <div class="setting-control">
          <div class="url-control-group">
            <t-input 
              v-model="localBaseUrl" 
              :placeholder="$t('ollamaSettings.address.placeholder')"
              disabled
              style="flex: 1;"
            />
          </div>
          <t-alert 
            v-if="connectionStatus === false"
            theme="warning"
            :message="$t('ollamaSettings.address.failed')"
            style="margin-top: 8px;"
          />
        </div>
      </div>

    </div>

    <!-- Download a new model -->
    <div v-if="connectionStatus === true" class="model-category-section">
      <div class="category-header">
        <div class="header-info">
          <h3>{{ $t('ollamaSettings.download.title') }}</h3>
          <p>
            {{ $t('ollamaSettings.download.descPrefix') }}
            <a href="https://ollama.com/search" target="_blank" rel="noopener noreferrer" class="doc-link">
              {{ $t('ollamaSettings.download.browse') }}
              <t-icon name="link" class="link-icon" />
            </a>
          </p>
        </div>
      </div>
      
      <div class="download-content">
        <div class="input-group">
          <t-input 
            v-model="downloadModelName" 
            :placeholder="$t('ollamaSettings.download.placeholder')"
            style="flex: 1;"
          />
          <t-button
            variant="base"
            theme="default"
            size="small"
            class="download-btn"
            :loading="downloading"
            :disabled="!downloadModelName.trim()"
            @click="downloadModel"
          >
            <template #icon><t-icon name="download" /></template>
            {{ $t('ollamaSettings.download.download') }}
          </t-button>
        </div>
        
        <div v-if="downloadProgress > 0" class="download-progress">
          <div class="progress-info">
            <span>{{ $t('ollamaSettings.download.downloading', { name: downloadModelName }) }}</span>
            <span>{{ downloadProgress.toFixed(2) }}%</span>
          </div>
          <t-progress :percentage="downloadProgress" size="small" />
        </div>
      </div>
    </div>

    <!-- Downloaded models -->
    <div v-if="connectionStatus === true" class="model-category-section">
      <div class="category-header">
        <div class="header-info">
          <h3>{{ $t('ollamaSettings.installed.title') }}</h3>
          <p>{{ $t('ollamaSettings.installed.desc') }}</p>
        </div>
        <t-button 
          size="small" 
          variant="text"
          :loading="loadingModels"
          @click="refreshModels"
        >
          <template #icon>
            <t-icon name="refresh" />
          </template>
          {{ $t('common.refresh') }}
        </t-button>
      </div>
      
      <div v-if="loadingModels" class="loading-state">
        <t-loading size="small" />
        <span>{{ $t('common.loading') }}</span>
      </div>
      <div v-else-if="downloadedModels.length > 0" class="model-list-container">
        <div v-for="model in downloadedModels" :key="model.name" class="model-card">
          <div class="model-info">
            <div class="model-name">{{ model.name }}</div>
            <div class="model-meta">
              <span class="model-size">{{ formatSize(model.size) }}</span>
              <span class="model-modified">{{ formatDate(model.modified_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="empty-state">
        <p class="empty-text">{{ $t('ollamaSettings.installed.empty') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { checkOllamaStatus, listOllamaModels, downloadOllamaModel, getDownloadProgress, type OllamaModelInfo } from '@/api/initialization'

const settingsStore = useSettingsStore()
const { t } = useI18n()

const localBaseUrl = ref(settingsStore.settings.ollamaConfig?.baseUrl ?? '')

const testing = ref(false)
const connectionStatus = ref<boolean | null>(null)
const loadingModels = ref(false)
const downloadedModels = ref<OllamaModelInfo[]>([])
const downloading = ref(false)
const downloadModelName = ref('')
const downloadProgress = ref(0)

// Test the connection
const testConnection = async () => {
  testing.value = true
  connectionStatus.value = null
  
  try {
    // Save the config
    settingsStore.updateOllamaConfig({ baseUrl: localBaseUrl.value })
    
    // Test the connection against the real Ollama API
    const result = await checkOllamaStatus()
    
    // If the API returned a baseUrl that differs from the input value, update to the returned value
    if (result.baseUrl && result.baseUrl !== localBaseUrl.value) {
      localBaseUrl.value = result.baseUrl
      settingsStore.updateOllamaConfig({ baseUrl: result.baseUrl })
    }
    
    connectionStatus.value = result.available
    
    if (connectionStatus.value) {
      MessagePlugin.success(t('ollamaSettings.toasts.connected'))
      refreshModels()
    } else {
      MessagePlugin.error(result.error || t('ollamaSettings.toasts.connectFailed'))
    }
  } catch (error: any) {
    connectionStatus.value = false
    MessagePlugin.error(error.message || t('ollamaSettings.toasts.connectFailed'))
  } finally {
    testing.value = false
  }
}

// Refresh the model list
const refreshModels = async () => {
  loadingModels.value = true
  
  try {
    // Get the model list from the real Ollama API (it now returns full model info)
    const models = await listOllamaModels()
    downloadedModels.value = models
  } catch (error: any) {
    console.error('Failed to get the model list:', error)
    MessagePlugin.error(error.message || t('ollamaSettings.toasts.listFailed'))
  } finally {
    loadingModels.value = false
  }
}

// Format the file size
const formatSize = (bytes: number): string => {
  if (!bytes || bytes === 0 || isNaN(bytes)) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

// Format the date
const formatDate = (dateStr: string): string => {
  if (!dateStr) return t('ollama.unknown')

  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return t('ollama.unknown')

  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))

  if (days === 0) return t('ollama.today')
  if (days === 1) return t('ollama.yesterday')
  if (days < 7) return t('ollama.daysAgo', { days })
  return date.toLocaleDateString()
}

// Download a model
const downloadModel = async () => {
  if (!downloadModelName.value.trim()) return
  
  downloading.value = true
  downloadProgress.value = 0
  
  try {
    // Download the model through the real Ollama API
    const result = await downloadOllamaModel(downloadModelName.value)
    
    if (result.status === 'failed') {
      MessagePlugin.error(t('ollamaSettings.toasts.downloadFailed'))
      downloading.value = false
      downloadProgress.value = 0
      return
    }
    
    MessagePlugin.success(t('ollamaSettings.toasts.downloadStarted', { name: downloadModelName.value }))
    
    // Poll the download progress
    const taskId = result.taskId
    const progressInterval = setInterval(async () => {
      try {
        const task = await getDownloadProgress(taskId)
        downloadProgress.value = task.progress
        
        if (task.status === 'completed') {
          clearInterval(progressInterval)
          MessagePlugin.success(t('ollamaSettings.toasts.downloadCompleted', { name: downloadModelName.value }))
          downloadModelName.value = ''
          downloadProgress.value = 0
          downloading.value = false
          refreshModels()
        } else if (task.status === 'failed') {
          clearInterval(progressInterval)
          MessagePlugin.error(task.message || t('ollamaSettings.toasts.downloadFailed'))
          downloading.value = false
          downloadProgress.value = 0
        }
      } catch (error) {
        clearInterval(progressInterval)
        MessagePlugin.error(t('ollamaSettings.toasts.progressFailed'))
        downloading.value = false
        downloadProgress.value = 0
      }
    }, 1000)
  } catch (error: any) {
    console.error('Download failed:', error)
    MessagePlugin.error(error.message || t('ollamaSettings.toasts.downloadFailed'))
    downloading.value = false
    downloadProgress.value = 0
  }
}

// Initialize the Ollama service address
const initOllamaBaseUrl = async () => {
  try {
    const result = await checkOllamaStatus()
    // If the API returned a baseUrl, prefer it
    if (result.baseUrl) {
      localBaseUrl.value = result.baseUrl
      // If the store has none saved yet, save it to the store too
      if (!settingsStore.settings.ollamaConfig?.baseUrl) {
        settingsStore.updateOllamaConfig({ baseUrl: result.baseUrl })
      }
    } else if (!localBaseUrl.value) {
      // If the API returned none and the store has none, use the default
      localBaseUrl.value = 'http://localhost:11434'
    }
    
    // Use the status fetched at init directly, to avoid a repeat call
      connectionStatus.value = result.available
      if (result.available) {
        refreshModels()
    }
    
    return result
  } catch (error) {
    console.error('Failed to initialize the Ollama address:', error)
    // On failure, use the default or the store value
    if (!localBaseUrl.value) {
      localBaseUrl.value = 'http://localhost:11434'
    }
    return null
  }
}

// Check the connection automatically on mount
onMounted(async () => {
  // Initialize the service address; if enabled, use the returned status directly to avoid a repeat call
  await initOllamaBaseUrl()
})
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/settings-section.less';

.ollama-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  .setting-row();
}

.setting-info {
  .setting-info();
}

.setting-control {
  .setting-control();
  min-width: 360px;
  max-width: 360px;
  flex-direction: column;
  align-items: flex-end;
}

.status-display {
  display: flex;
  align-items: center;
  gap: 12px;

  .status-icon.spinning {
    animation: wk-spin 1s linear infinite;
  }
}

.url-control-group {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-category-section {
  margin-top: 32px;
  margin-bottom: 32px;
  padding-top: 32px;
  border-top: 1px solid var(--td-component-stroke);

  &:first-of-type {
    margin-top: 24px;
    padding-top: 24px;
  }

  &:last-child {
    margin-bottom: 0;
  }
}

.category-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 24px;

  .header-info {
    flex: 1;

    h3 {
      font-size: 17px;
      font-weight: 600;
      color: var(--td-text-color-primary);
      margin: 0 0 6px 0;
    }

    p {
      font-size: var(--app-text-md);
      color: var(--td-text-color-placeholder);
      margin: 0;
      line-height: 1.5;
    }

  }
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 48px 0;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-base);
}

.model-list-container {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;

  @media (max-width: 768px) {
    grid-template-columns: 1fr;
  }
}

.model-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);
  background: var(--td-bg-color-secondarycontainer);
  transition: all var(--app-motion-base);

  &:hover {
    border-color: var(--td-brand-color);
    background: var(--td-bg-color-container);
  }
}

.model-info {
  flex: 1;
  min-width: 0;

  .model-name {
    font-size: var(--app-text-base);
    font-weight: 500;
    color: var(--td-text-color-primary);
    margin-bottom: 4px;
    font-family: var(--app-font-family-mono);
  }

  .model-meta {
    display: flex;
    gap: 12px;
    font-size: var(--app-text-sm);
    color: var(--td-text-color-secondary);
  }
}

.download-content {
  display: flex;
  flex-direction: column;
  gap: 16px;

  .input-group {
    display: flex;
    gap: 8px;
    align-items: center;

    .download-btn {
      flex-shrink: 0;
      height: 32px;
    }
  }

  .download-progress {
    padding: 16px;
    background: var(--td-bg-color-secondarycontainer);
    border-radius: var(--app-radius-md);
    border: 1px solid var(--td-component-stroke);

    .progress-info {
      display: flex;
      justify-content: space-between;
      margin-bottom: 10px;
      font-size: var(--app-text-md);
      color: var(--td-text-color-primary);
      font-weight: 500;
    }
  }
}

.empty-state {
  padding: 48px 0;
  text-align: center;

  .empty-text {
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
    margin: 0;
  }
}

</style>
