<template>
  <SettingDrawer :visible="dialogVisible" :title="isEdit ? $t('model.editor.editTitle') : $t('model.editor.addTitle')"
    :description="getModalDescription()" :icon="modelTypeIcon" :confirm-loading="saving" :cancel-disabled="saving"
    :confirm-text="$t('model.editor.saveAndClose')"
    :close-on-overlay-click="!saving" :close-on-esc-keydown="!saving"
    :confirm-disabled="formData.provider === 'weknoracloud' && wkcCredentialState !== 'configured'"
    @update:visible="(v: boolean) => dialogVisible = v" @confirm="handleConfirm" @cancel="handleCancel">

    <template v-if="formData.source === 'remote'" #footer-left>
      <t-button variant="outline" @click="checkRemoteAPI" :loading="checking"
        :disabled="saving || !formData.modelName || (!formData.baseUrl && formData.provider !== 'weknoracloud') || (formData.provider === 'weknoracloud' && wkcCredentialState !== 'configured')">
        <template #icon>
          <t-icon v-if="!checking && remoteChecked && remoteAvailable" name="check-circle-filled"
            class="status-icon available" />
          <t-icon v-else-if="!checking && remoteChecked && !remoteAvailable" name="close-circle-filled"
            class="status-icon unavailable" />
        </template>
        {{ checking ? $t('model.editor.testing') : $t('model.editor.testConnection') }}
      </t-button>
      <span v-if="remoteChecked" :class="['connection-status', remoteAvailable ? 'success' : 'error']">
        {{ remoteAvailable ? $t('model.editor.connectionSuccess') : $t('model.editor.connectionFailed') }}
      </span>
    </template>

    <template #footer-extra>
      <div v-if="saveError" class="connection-result error" role="alert">
        <strong>{{ $t('modelSettings.toasts.saveFailed') }}</strong>
        <div class="connection-result__details" tabindex="0">{{ saveError }}</div>
      </div>
      <div v-if="formData.source === 'remote'" class="connection-feedback" aria-live="polite">
        <p class="connection-hint">{{ $t(isEdit ? 'model.editor.testDraftEditHint' : 'model.editor.testDraftHint') }}</p>
        <p v-if="remoteStale" class="connection-hint">{{ $t('model.editor.testStale') }}</p>
        <div v-if="remoteChecked && !remoteAvailable" class="connection-result error">
          <div class="connection-result__header">
            <strong>{{ $t('model.editor.connectionFailed') }}</strong>
            <t-button size="small" variant="text" @click="copyWithToast(remoteMessage, 'common.copied')">
              {{ $t('common.copy') }}
            </t-button>
          </div>
          <div class="connection-result__details" tabindex="0">{{ remoteMessage }}</div>
        </div>
      </div>
    </template>

    <t-form :inert="saving || undefined" ref="formRef" :data="formData" :rules="rules" layout="vertical">

      <section v-if="!isEdit" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('model.editor.sectionType') }}</h4>
        <div class="model-type-options" role="radiogroup" :aria-label="$t('model.editor.typeLabel')">
          <button
            v-for="opt in modelTypeChoices"
            :key="opt.value"
            type="button"
            class="model-type-option"
            :class="{ 'is-active': activeModelType === opt.value }"
            role="radio"
            :aria-checked="activeModelType === opt.value"
            @click="selectModelType(opt.value)"
          >
            <t-icon :name="opt.icon" class="model-type-option__icon" />
            <span class="model-type-option__label">{{ opt.label }}</span>
          </button>
        </div>
      </section>

      <!--
        Section 1 - model source + model name (the source decides the fields below, so they share a section)
      -->
      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('model.editor.sectionSource') }}</h4>

        <div class="form-item">
          <!--
            The section title already says "Model source", so the label is not repeated here;
            the segmented control is the section's first content, avoiding a "double title" feel.
          -->
          <div class="source-options" role="radiogroup" :aria-label="$t('model.editor.sourceLabel')">
            <button
              type="button"
              class="source-option"
              :class="{ 'is-active': formData.source === 'remote' }"
              role="radio"
              :aria-checked="formData.source === 'remote'"
              @click="formData.source = 'remote'"
            >
              <t-icon name="cloud" class="source-option__icon" />
              <span class="source-option__label">{{ $t('model.editor.sourceRemote') }}</span>
            </button>
            <button
              v-if="activeModelType !== 'rerank'"
              type="button"
              class="source-option"
              :class="{ 'is-active': formData.source === 'local', 'is-disabled': ollamaServiceStatus === false }"
              :disabled="ollamaServiceStatus === false"
              role="radio"
              :aria-checked="formData.source === 'local'"
              @click="formData.source = 'local'"
            >
              <t-icon name="server" class="source-option__icon" />
              <span class="source-option__label">{{ $t('model.editor.sourceLocal') }}</span>
            </button>
          </div>

          <!-- Notice when Ollama is unavailable -->
          <div v-if="shouldShowOllamaUnavailableTip(formData.source, activeModelType, ollamaServiceStatus)"
            class="ollama-unavailable-tip">
            <t-icon name="error-circle-filled" class="tip-icon" />
            <span class="tip-text">{{ $t('model.editor.ollamaUnavailable') }}</span>
            <t-button variant="text" size="small" @click="goToOllamaSettings" class="tip-link">
              <template #icon><t-icon name="jump" /></template>
              {{ $t('model.editor.goToOllamaSettings') }}
            </t-button>
          </div>
        </div>

        <!-- Ollama local model picker -->
        <div v-if="formData.source === 'local'" class="form-item">
          <label class="form-label required">{{ $t('model.modelName') }}</label>
          <div class="model-select-row">
            <t-select v-model="formData.modelName" :loading="loadingOllamaModels" :class="{ 'downloading': downloading }"
              :style="downloading ? `--progress: ${downloadProgress}%` : ''" filterable :filter="handleModelFilter"
              :placeholder="$t('model.searchPlaceholder')" @focus="loadOllamaModels"
              @visible-change="handleDropdownVisibleChange">
              <!-- Downloaded models -->
              <t-option v-for="model in filteredOllamaModels" :key="model.name" :value="model.name" :label="model.name">
                <div class="model-option">
                  <t-icon name="check-circle-filled" class="downloaded-icon" />
                  <span class="model-name">{{ model.name }}</span>
                  <span class="model-size">{{ formatModelSize(model.size) }}</span>
                </div>
              </t-option>

              <!-- Download-new-model option (shown only when the search term is not in the list) -->
              <t-option v-if="showDownloadOption" :value="`__download__${searchKeyword}`"
                :label="$t('model.editor.downloadLabel', { keyword: searchKeyword })" class="download-option">
                <div class="model-option download">
                  <t-icon name="download" class="download-icon" />
                  <span class="model-name">{{ $t('model.editor.downloadLabel', { keyword: searchKeyword }) }}</span>
                </div>
              </t-option>

              <!-- Download progress suffix -->
              <template v-if="downloading" #suffix>
                <div class="download-suffix">
                  <t-icon name="loading" class="spinning" />
                  <span class="progress-text">{{ downloadProgress.toFixed(1) }}%</span>
                </div>
              </template>
            </t-select>

            <!-- Refresh button -->
            <t-button variant="text" size="small" :loading="loadingOllamaModels" @click="refreshOllamaModels"
              class="refresh-btn">
              <t-icon name="refresh" />
              {{ $t('model.editor.refreshList') }}
            </t-button>
          </div>
        </div>
      </section>

      <!-- Remote API config -->
      <template v-if="formData.source === 'remote'">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t('model.editor.sectionProvider') }}</h4>

          <!-- Vendor picker -->
          <div class="form-item">
            <label class="form-label">{{ $t('model.editor.providerLabel') }}</label>
            <t-select v-model="formData.provider" :placeholder="$t('model.editor.providerPlaceholder')"
              :loading="loadingProviders" filterable
              @change="handleProviderChange"
              :popup-props="{ overlayClassName: 'wk-popover provider-select-popup', overlayInnerStyle: matchTriggerWidth }">
              <!--
                Selected value: icon + localized name (the description shows only in the dropdown, so the input stays short).
                When the catalog no longer has this provider (vendor removed / old data), fall back to the raw id,
                otherwise the input goes blank and looks like "no vendor selected", while saving still sends it.
              -->
              <template #valueDisplay>
                <span v-if="formData.provider" class="provider-value">
                  <img v-if="selectedProviderIcon" :src="selectedProviderIcon" class="provider-icon" alt="" />
                  <span class="provider-value__name">{{ selectedProviderDisplayLabel }}</span>
                </span>
              </template>
              <!--
                show-overflow-tooltip=false: TDesign by default floats a small bubble with the
                full label on hover, but the options here are already two lines (main name + description) and never
                truncate, so the tooltip only clashes with the gray hit background. Turn it off.
              -->
              <t-option v-for="opt in providerOptions" :key="opt.value" :value="opt.value"
                :label="providerDisplayLabel(opt)" :show-overflow-tooltip="false">
                <div class="provider-option">
                  <img v-if="providerIcon(opt)" :src="providerIcon(opt)" class="provider-icon" alt="" />
                  <span v-else class="provider-icon provider-icon--placeholder" aria-hidden="true">
                    {{ providerDisplayLabel(opt).slice(0, 1) }}
                  </span>
                  <div class="provider-option__text">
                    <span class="provider-name">{{ providerDisplayLabel(opt) }}</span>
                    <span class="provider-desc">{{ providerDisplayDescription(opt) }}</span>
                  </div>
                </div>
              </t-option>
            </t-select>
            <p v-if="vendorDocLink" class="form-desc provider-doc-link">
              <a :href="vendorDocLink" target="_blank" rel="noopener noreferrer">
                {{ $t('model.editor.providerDocs', { provider: selectedProviderDisplayLabel }) }}
                <t-icon name="jump" size="12px" />
              </a>
            </p>
          </div>

          <!-- WeKnoraCloud notice -->
          <template v-if="formData.provider === 'weknoracloud'">
            <!-- Credentials configured -->
            <div v-if="wkcCredentialState === 'configured'" class="weknoracloud-hint weknoracloud-hint--ok">
              <t-icon name="check-circle-filled" class="hint-icon hint-icon--ok" />
              <div>
                {{ $t('settings.weknoraCloud.modelHintConfigured') }}
                <a href="https://developers.weixin.qq.com/doc/aispeech/knowledge/atomic_capability/atomic_interface.html"
                  target="_blank" rel="noopener noreferrer" class="doc-link">
                  {{ $t('settings.weknoraCloud.modelHintDocsLink') }}
                  <t-icon name="link" class="link-icon" />
                </a>
              </div>
            </div>

            <!-- Not configured / invalid -->
            <div v-else-if="wkcCredentialState !== 'loading'" class="weknoracloud-hint weknoracloud-hint--warn">
              <t-icon name="error-circle-filled" class="hint-icon hint-icon--warn" />
              <div style="flex: 1;">
                <template v-if="wkcCredentialState === 'expired'">
                  {{ $t('settings.weknoraCloud.credentialExpired') }}
                </template>
                <template v-else>
                  {{ $t('settings.weknoraCloud.credentialUnconfigured') }}
                </template>
                <div style="margin-top: 8px;">
                  <t-button variant="text" size="small" @click="goToWeKnoraCloudSettings"
                    style="padding: 0; height: auto;">
                    <template #icon><t-icon name="jump" /></template>
                    {{ $t('settings.weknoraCloud.goToSettings') }}
                  </t-button>
                </div>
              </div>
            </div>

            <!-- Loading -->
            <div v-else class="weknoracloud-hint">
              <t-icon name="loading" class="spinning hint-icon hint-icon--loading" />
              <span>{{ $t('settings.weknoraCloud.checkingStatus') }}</span>
            </div>
          </template>

          <!--
            Model name: the vendor's built-in catalog supplies candidates, and free input is allowed too (filterable + creatable).
            Picking a catalog model fills in the context window / output limit / vision / dimension (empty fields only).
          -->
          <div class="form-item">
            <label class="form-label required">{{ $t('model.modelName') }}</label>
            <!--
              Without a built-in catalog, use a plain input: TDesign's select hides the whole
              popup when it has zero options (hideEmptyPopup), the creatable "Create" row included, so
              the user can type but has no way to submit, and the input is lost on blur. Custom
              (OpenAI-compatible API) is exactly that case, so no model name could be entered there.
            -->
            <t-input
              v-if="!hasCatalog"
              v-model="formData.modelName"
              :placeholder="getModelNamePlaceholder()"
              :disabled="formData.provider === 'weknoracloud' && wkcCredentialState !== 'configured'"
              clearable
              autocomplete="off"
              spellcheck="false"
            />
            <t-select
              v-else
              v-model="formData.modelName"
              filterable
              creatable
              clearable
              :placeholder="getModelNamePlaceholder()"
              :disabled="formData.provider === 'weknoracloud' && wkcCredentialState !== 'configured'"
              :popup-props="{ overlayClassName: 'wk-popover catalog-model-select-popup', overlayInnerStyle: matchTriggerWidth }"
              @create="handleCatalogModelCreate"
              @change="handleCatalogModelChange"
            >
              <t-option v-for="opt in catalogModelOptions" :key="opt.value" :value="opt.value" :label="opt.label"
                :show-overflow-tooltip="false">
                <div class="catalog-model-option">
                  <span class="catalog-model-option__name">{{ opt.label }}</span>
                  <span v-if="opt.value !== opt.label" class="catalog-model-option__id">{{ opt.value }}</span>
                  <span class="catalog-model-option__badges">
                    <span v-if="opt.contextWindow" class="catalog-badge">{{ opt.contextWindow }}</span>
                    <span v-if="opt.dimension" class="catalog-badge">{{ $t('model.editor.dimensionLabel') }} {{ opt.dimension }}</span>
                    <span v-if="opt.reasoning" class="catalog-badge catalog-badge--accent">{{ $t('model.editor.catalog.reasoning') }}</span>
                    <span v-if="opt.vision" class="catalog-badge catalog-badge--accent">{{ $t('model.editor.catalog.vision') }}</span>
                  </span>
                </div>
              </t-option>
            </t-select>
            <p v-if="hasCatalog" class="form-desc">{{ $t('model.editor.catalog.hint') }}</p>
          </div>

          <div class="form-item">
            <label class="form-label">{{ $t('model.editor.displayNameLabel') }}</label>
            <t-input v-model="formData.displayName" :placeholder="$t('model.editor.displayNamePlaceholder')" />
            <p class="form-desc">{{ $t('model.editor.displayNameDesc') }}</p>
          </div>

          <div v-if="formData.provider !== 'weknoracloud'" class="form-item">
            <label class="form-label required">{{ $t('model.editor.baseUrlLabel') }}</label>
            <t-input v-model="formData.baseUrl" :placeholder="getBaseUrlPlaceholder()" />
          </div>

          <div v-if="formData.provider !== 'weknoracloud'" class="form-item">
            <label class="form-label" :class="{ required: apiKeyRequired }">{{ apiKeyLabel }}</label>
            <!--
              Edit mode: credentials live behind the /credentials subresource
              of the model - managed by the shared CredentialResource card,
              which now renders an INPUT-LOOKING row (32px tall, same border
              + radius as t-input) so it sits flush with the Base URL field
              above and the Custom headers controls below - no more
              "card inside a card" feel.
              Create mode: the resource doesn't exist yet, so we render a
              plain password input with a leading lock icon; TDesign's password
              input provides the show/hide toggle.
            -->
            <CredentialResource v-if="isEdit && props.modelData?.id" :api="credentialApi" :fields="credentialFields"
              :meta="credentialMeta" @changed="invalidateConnectionTest()" />
            <t-input v-else v-model="formData.apiKey" type="password"
              :placeholder="apiKeyPlaceholder"
              class="api-key-input" autocomplete="new-password" spellcheck="false">
              <template #prefix-icon><t-icon name="lock-on" /></template>
            </t-input>
            <p v-if="apiKeyHint" class="form-desc">{{ apiKeyHint }}</p>
          </div>

          <!--
            Secret extra fields the vendor declares (the SecretKey of LKEAP / Volcano Engine Rerank and so on):
            create mode writes them to app_secret; edit mode manages them through the CredentialResource card above.
          -->
          <div v-if="secretExtraField && !isEdit && formData.provider !== 'weknoracloud'" class="form-item">
            <label class="form-label" :class="{ required: secretExtraField.required }">{{ extraFieldDisplayLabel(secretExtraField) }}</label>
            <t-input v-model="formData.appSecret" type="password"
              :placeholder="secretExtraField.placeholder || ''" autocomplete="new-password" spellcheck="false">
              <template #prefix-icon><t-icon name="lock-on" /></template>
            </t-input>
            <p v-if="secretExtraField.placeholder" class="form-desc">{{ secretExtraField.placeholder }}</p>
          </div>

          <!-- The vendor's other declared extra fields: rendered by type, values stored in extra_config[key] -->
          <div v-for="field in plainExtraFields" :key="field.key" class="form-item">
            <label class="form-label" :class="{ required: field.required }">{{ extraFieldDisplayLabel(field) }}</label>
            <div v-if="field.type === 'boolean'" class="vision-toggle">
              <t-switch :model-value="extraConfigBool(field.key)"
                @update:model-value="(v: boolean) => setExtraConfig(field.key, v ? 'true' : 'false')" />
              <span v-if="extraFieldDisplayPlaceholder(field)" class="form-desc form-desc--inline">{{ extraFieldDisplayPlaceholder(field) }}</span>
            </div>
            <t-select v-else-if="field.type === 'select'" :model-value="formData.extraConfig[field.key] || ''"
              :placeholder="extraFieldDisplayPlaceholder(field)" clearable
              @update:model-value="(v: string) => setExtraConfig(field.key, v)">
              <t-option v-for="opt in (field.options || [])" :key="opt.value" :value="opt.value"
                :label="extraFieldDisplayOptionLabel(opt)" />
            </t-select>
            <t-input v-else-if="field.type === 'number'" :model-value="formData.extraConfig[field.key] || ''"
              type="number" :placeholder="extraFieldDisplayPlaceholder(field)"
              @update:model-value="(v: string | number) => setExtraConfig(field.key, String(v ?? ''))" />
            <t-input v-else-if="field.type === 'password'" :model-value="formData.extraConfig[field.key] || ''"
              type="password" :placeholder="extraFieldDisplayPlaceholder(field)" autocomplete="new-password" spellcheck="false"
              @update:model-value="(v: string) => setExtraConfig(field.key, v)">
              <template #prefix-icon><t-icon name="lock-on" /></template>
            </t-input>
            <t-input v-else :model-value="formData.extraConfig[field.key] || ''"
              :placeholder="extraFieldDisplayPlaceholder(field)"
              @update:model-value="(v: string) => setExtraConfig(field.key, v)" />
          </div>

          <!-- Custom HTTP headers (like extra_headers in the OpenAI Python SDK) -->
          <div v-if="formData.provider !== 'weknoracloud'" class="form-item">
            <div class="custom-headers-header">
              <label class="form-label" style="margin-bottom: 0;">{{ $t('model.editor.customHeadersLabel') }}</label>
              <t-button variant="text" size="small" theme="primary" @click="addCustomHeader">
                <template #icon><t-icon name="add" /></template>
                {{ $t('model.editor.customHeadersAdd') }}
              </t-button>
            </div>
            <p class="form-desc custom-headers-desc">{{ $t('model.editor.customHeadersDesc') }}</p>
            <div v-if="formData.customHeaders && formData.customHeaders.length > 0" class="custom-headers-list">
              <div v-for="(item, idx) in formData.customHeaders" :key="idx" class="custom-header-row">
                <t-input v-model="item.key" :placeholder="$t('model.editor.customHeadersKeyPlaceholder')"
                  class="custom-header-key" />
                <t-input v-model="item.value" :placeholder="$t('model.editor.customHeadersValuePlaceholder')"
                  class="custom-header-value" />
                <t-button variant="text" shape="square" size="small" class="custom-header-remove"
                  @click="removeCustomHeader(idx)" :aria-label="$t('common.delete')">
                  <t-icon name="close" />
                </t-button>
              </div>
            </div>
          </div>

          <!--
            Access diagnostics: the effective protocol / thinking format / supported levels /
            in-catalog flag / context window resolved live from the backend catalog, read-only. Refreshes with a 400ms debounce after the vendor, model name or Base URL changes.
            Meaningful only for chat / VLM: embedding, rerank and ASR have neither thinking levels nor
            a context window, and showing them would suggest these values apply to them.
          -->
          <div v-if="showResolvedPanel" class="form-item">
            <div class="resolved-panel" aria-live="polite">
              <div class="resolved-panel__header">
                <t-icon name="system-code" class="resolved-panel__icon" />
                <span class="resolved-panel__title">{{ $t('model.editor.resolved.title') }}</span>
                <t-icon v-if="resolving" name="loading" class="spinning resolved-panel__loading" />
              </div>
              <p v-if="!resolved && !resolving && !resolveFailed" class="form-desc">{{ $t('model.editor.resolved.empty') }}</p>
              <!-- The backend error body has no fixed shape; when no readable text is found, show only the title, not [object Object] -->
              <p v-else-if="resolveFailed" class="form-desc form-desc--warn">
                {{ $t('model.editor.resolved.failed') }}<template v-if="resolveError">: {{ resolveError }}</template>
              </p>
              <dl v-else-if="resolved" class="resolved-panel__grid">
                <dt>{{ $t('model.editor.resolved.protocol') }}</dt>
                <dd><code>{{ resolved.api }}</code></dd>
                <dt>{{ $t('model.editor.resolved.catalog') }}</dt>
                <dd>
                  <span class="catalog-badge" :class="{ 'catalog-badge--accent': resolved.cataloged }">
                    {{ resolved.cataloged ? $t('model.editor.resolved.catalogedYes') : $t('model.editor.resolved.catalogedNo') }}
                  </span>
                </dd>
                <template v-if="resolved.url">
                  <dt>{{ $t('model.editor.resolved.endpoint') }}</dt>
                  <dd><code class="resolved-endpoint">{{ resolved.url }}</code></dd>
                </template>
                <template v-if="resolved.remote_model && resolved.remote_model !== formData.modelName">
                  <dt>{{ $t('model.editor.advanced.remoteModelName.label') }}</dt>
                  <dd><code>{{ resolved.remote_model }}</code></dd>
                </template>
                <template v-if="isChatLike">
                  <dt>{{ $t('model.editor.resolved.thinkingFormat') }}</dt>
                  <dd><code>{{ resolved.capabilities?.thinking_format || '-' }}</code></dd>
                  <dt>{{ $t('model.editor.resolved.thinkingLevels') }}</dt>
                  <dd>
                    <template v-if="resolvedThinkingLevels.length > 0">
                      <span v-for="level in resolvedThinkingLevels" :key="level" class="catalog-badge">
                        {{ $t(levelLabelKey(level)) }}
                      </span>
                    </template>
                    <span v-else class="form-desc form-desc--inline">{{ $t('model.editor.resolved.noThinking') }}</span>
                  </dd>
                  <template v-if="resolved.capabilities?.context_window">
                    <dt>{{ $t('model.editor.contextWindowLabel') }}</dt>
                    <dd>{{ formatTokenCount(resolved.capabilities.context_window) }}</dd>
                  </template>
                  <template v-if="resolved.capabilities?.max_output_tokens">
                    <dt>{{ $t('model.editor.maxOutputTokensLabel') }}</dt>
                    <dd>{{ formatTokenCount(resolved.capabilities.max_output_tokens) }}</dd>
                  </template>
                </template>
              </dl>
            </div>
          </div>

          <!--
            Connection test action moved to the drawer footer (footer-left
            slot above) so primary actions live in one row at the bottom.
          -->
        </section>
      </template>

      <!-- Section 3 - advanced options (rendered only when there is content, so an empty section does not show a bottom divider) -->
      <section v-if="['embedding', 'chat', 'vllm'].includes(activeModelType) || formData.source === 'remote'" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t('model.editor.sectionAdvanced') }}</h4>

        <!-- Embedding only: dimension -->
        <div v-if="activeModelType === 'embedding'" class="form-item">
          <label class="form-label">{{ $t('model.editor.dimensionLabel') }}</label>
          <div class="dimension-control">
            <t-input v-model.number="formData.dimension" type="number" :min="128" :max="4096"
              :placeholder="$t('model.editor.dimensionPlaceholder')"
              :disabled="!formData.supportsDimensionOverride || (formData.source === 'local' && checking)" />
            <!-- Ollama local model: auto-detect dimension button -->
            <t-button v-if="formData.source === 'local' && formData.modelName" variant="text" size="small"
              :loading="checking" @click="checkOllamaDimension" class="dimension-check-btn">
              <t-icon name="refresh" />
              {{ $t('model.editor.checkDimension') }}
            </t-button>
          </div>
          <p v-if="dimensionChecked && dimensionMessage" class="dimension-hint" :class="{ success: dimensionSuccess }">
            {{ dimensionMessage }}
          </p>
        </div>

        <div v-if="activeModelType === 'embedding'" class="form-item">
          <label class="form-label">{{ $t('model.editor.dimensionOverrideLabel') }}</label>
          <div class="vision-toggle">
            <t-switch v-model="formData.supportsDimensionOverride" />
            <span class="form-desc form-desc--inline">{{ $t('model.editor.dimensionOverrideDesc') }}</span>
          </div>
        </div>

        <!-- Embedding: image input. Catalogued models answer it themselves; a row declares it in spec.input. -->
        <div v-if="activeModelType === 'embedding'" class="form-item">
          <label class="form-label">{{ $t('model.editor.embeddingImageInputLabel') }}</label>
          <div class="vision-toggle">
            <t-switch v-model="embeddingAcceptsImages" />
            <span class="form-desc form-desc--inline">{{ $t('model.editor.embeddingImageInputDesc') }}</span>
          </div>
        </div>

        <!-- Chat / VLM: context window. Agent compaction sizes itself from this. -->
        <div v-if="activeModelType === 'chat' || activeModelType === 'vllm'" class="form-item">
          <label class="form-label">{{ $t('model.editor.contextWindowLabel') }}</label>
          <t-input v-model.number="formData.contextWindow" type="number" :min="1024" :max="10000000"
            :placeholder="$t('model.editor.contextWindowPlaceholder', { value: DEFAULT_MODEL_CONTEXT_WINDOW })" />
          <p class="form-desc">{{ $t('model.editor.contextWindowDesc') }}</p>
        </div>

        <!-- Chat: supports vision toggle (VLLM models are inherently multimodal) -->
        <div v-if="activeModelType === 'chat'" class="form-item">
          <label class="form-label">{{ $t('model.editor.supportsVisionLabel') }}</label>
          <div class="vision-toggle">
            <t-switch v-model="formData.supportsVision" />
            <span class="form-desc form-desc--inline">{{ $t('model.editor.supportsVisionDesc') }}</span>
          </div>
        </div>

        <!-- Chat / VLM: max output tokens (catalog default when empty). -->
        <div v-if="activeModelType === 'chat' || activeModelType === 'vllm'" class="form-item">
          <label class="form-label">{{ $t('model.editor.maxOutputTokensLabel') }}</label>
          <t-input v-model.number="formData.maxOutputTokens" type="number" :min="1" :max="10000000"
            :placeholder="$t('model.editor.maxOutputTokensPlaceholder')" />
          <p class="form-desc">{{ $t('model.editor.maxOutputTokensDesc') }}</p>
        </div>

        <!--
          Background concurrency cap for this model. Only chat / embedding / vllm
          are gated by the governor (see internal/models/limiter), so we surface
          it just for those three. 0 = fall back to the global default.
        -->
        <div v-if="['embedding', 'chat', 'vllm'].includes(activeModelType)" class="form-item">
          <label class="form-label">{{ $t('model.editor.maxConcurrencyLabel') }}</label>
          <t-input v-model.number="formData.maxConcurrency" type="number" :min="0" :max="4096"
            :placeholder="$t('model.editor.maxConcurrencyPlaceholder')" />
          <p class="form-desc">{{ $t('model.editor.maxConcurrencyDesc') }}</p>
        </div>

        <!--
          Advanced: protocol override / remote model name / catalog compat override, plus the legacy
          thinking_control choice shown only for old data. Collapsed by default; most users never need it.
        -->
        <template v-if="formData.source === 'remote' && formData.provider !== 'weknoracloud'">
          <button type="button" class="advanced-toggle" :aria-expanded="advancedOpen" @click="advancedOpen = !advancedOpen">
            <t-icon name="chevron-right" class="toggle-arrow" :class="{ open: advancedOpen }" />
            <span>{{ $t('model.editor.advanced.toggle') }}</span>
          </button>

          <template v-if="advancedOpen">
            <!--
              extra_config.api only picks a chat protocol. Embedding / rerank rows do not read it:
              the embedding protocol override goes in the compat JSON below ("api"), and its values are vector protocols.
            -->
            <div v-if="isChatLike" class="form-item">
              <label class="form-label">{{ $t('model.editor.advanced.api.label') }}</label>
              <t-select :model-value="formData.extraConfig.api || ''" clearable
                @update:model-value="(v: string) => setExtraConfig('api', v)">
                <t-option value="" :label="$t('model.editor.advanced.api.auto')" />
                <t-option v-for="api in PROTOCOL_OPTIONS" :key="api" :value="api" :label="api" />
              </t-select>
              <p class="form-desc">{{ $t('model.editor.advanced.api.desc') }}</p>
            </div>

            <div class="form-item">
              <label class="form-label">{{ $t('model.editor.advanced.remoteModelName.label') }}</label>
              <t-input :model-value="formData.extraConfig.remote_model_name || ''"
                :placeholder="$t('model.editor.advanced.remoteModelName.placeholder')"
                @update:model-value="(v: string) => setExtraConfig('remote_model_name', v)" />
              <p class="form-desc">{{ $t('model.editor.advanced.remoteModelName.desc') }}</p>
            </div>

            <!-- Old data only: shown when extra_config.thinking_control exists; can be cleared to use the catalog default -->
            <div v-if="showLegacyThinkingControl" class="form-item">
              <label class="form-label">{{ $t('model.editor.advanced.legacyThinking.label') }}</label>
              <t-select :model-value="formData.thinkingControl || ''"
                @update:model-value="(v: string) => formData.thinkingControl = v">
                <t-option value="" :label="$t('model.editor.advanced.legacyThinking.catalog')" />
                <t-option v-for="opt in LEGACY_THINKING_CONTROL_VALUES" :key="opt" :value="opt"
                  :label="opt === 'none' ? $t('model.editor.advanced.legacyThinking.none') : opt" />
              </t-select>
              <p class="form-desc form-desc--warn">{{ $t('model.editor.advanced.legacyThinking.desc') }}</p>
            </div>

            <div class="form-item">
              <label class="form-label">{{ $t('model.editor.advanced.compat.label') }}</label>
              <t-textarea v-model="formData.specCompat" :autosize="{ minRows: 3, maxRows: 10 }"
                :placeholder="$t('model.editor.advanced.compat.placeholder')" class="compat-textarea"
                :status="specCompatError ? 'error' : undefined" />
              <p v-if="specCompatError" class="form-desc form-desc--error">{{ $t('model.editor.advanced.compat.invalid') }}: {{ specCompatError }}</p>
              <p v-else class="form-desc">
                {{ $t('model.editor.advanced.compat.desc') }}
                <a :href="COMPAT_DOC_URL" target="_blank" rel="noopener noreferrer" class="compat-doc-link">
                  {{ $t('model.editor.advanced.compat.docLink') }}
                  <t-icon name="jump" size="12px" />
                </a>
              </p>
            </div>
          </template>
        </template>
      </section>

    </t-form>
  </SettingDrawer>
</template>

<script setup lang="ts">
import { ref, watch, computed, onUnmounted, nextTick } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import {
  checkOllamaModels, checkRemoteModel, testEmbeddingModel, checkRerankModel, checkASRModel, listOllamaModels,
  downloadOllamaModel, getDownloadProgress, checkOllamaStatus, resolveModelCatalog,
  type OllamaModelInfo, type ModelProviderOption, type ModelProviderExtraField,
  type ModelProviderExtraFieldOption, type ModelCatalogEntry,
  type ResolvedModelCatalog,
} from '@/api/initialization'
import {
  getWeKnoraCloudStatus,
  putModelCredentials,
  deleteModelCredentialField,
  type ModelCredentialField,
  type ModelSpecOverride,
} from '@/api/model'
import { useI18n } from 'vue-i18n'
import { useUIStore } from '@/stores/ui'
import { useModelProvidersStore } from '@/stores/modelProviders'
import {
  credentialLabelForModelType,
  extraFieldLabel,
  extraFieldOptionLabel,
  extraFieldPlaceholder,
  extraFieldsForModelType,
  pickLocalized,
  providerDescription,
  providerIcon,
  providerLabel,
} from '@/stores/modelProvidersState'
import { levelLabelKey, supportedLevels } from '@/utils/reasoningEffort'
import { DEFAULT_MODEL_CONTEXT_WINDOW, formatTokenCount } from '@/utils/contextWindow'
import { copyWithToast } from '@/utils/clipboard'
import SettingDrawer from '@/components/settings/SettingDrawer.vue'
import CredentialResource, {
  type CredentialFieldDef,
  type CredentialResourceApi,
} from '@/components/credentials/CredentialResource.vue'
import { shouldShowOllamaUnavailableTip } from '@/components/modelEditorSourceState'
import { AIKB_CLOUD_PROVIDER, WKC_MODEL_KINDS, WKC_MODEL_NAME_BY_KIND } from '@/utils/weknoraCloudModels'
import { docsUrl } from '@/utils/docsUrl'

interface CustomHeaderItem {
  key: string
  value: string
}

interface ModelFormData {
  id: string
  name: string
  source: 'local' | 'remote'
  provider?: string // Provider identifier: openai, aliyun, zhipu, generic, etc.
  modelName: string
  displayName?: string
  baseUrl?: string
  apiKey?: string
  dimension?: number
  supportsDimensionOverride?: boolean
  interfaceType?: 'ollama' | 'openai'
  isDefault: boolean
  supportsVision?: boolean
  /** Chat/VLM context window (tokens). Empty/0 means the default of 200000. */
  contextWindow?: number
  /** Concurrency cap for background tasks on this model; 0/undefined means the global default. Applies only to chat/embedding/vllm. */
  maxConcurrency?: number
  /** Chat/VLM per-call output limit (tokens). Empty/0 means the catalog default. */
  maxOutputTokens?: number
  /**
   * Legacy extra_config.thinking_control (none | enable_thinking | thinking_type
   * | chat_template_kwargs). Only rows saved by older UIs carry it; the catalog
   * decides the encoding otherwise. Empty string = drop the key on save.
   */
  thinkingControl?: string
  /**
   * Provider-specific extra_config entries: vendor-declared extra fields
   * (api_version, region, ...) plus the advanced overrides `api` and
   * `remote_model_name`. thinking_control lives in its own field above.
   */
  extraConfig: Record<string, string>
  /** parameters.spec.compat as JSON text (validated before save). */
  specCompat?: string
  /** Other parameters.spec fields preserved verbatim from the loaded row. */
  spec?: ModelSpecOverride | null
  // Custom HTTP request headers (like extra_headers in the OpenAI Python SDK)
  customHeaders?: CustomHeaderItem[]
  /** Second key (a vendor secret extra field, such as the SecretKey of LKEAP / Volcano Engine Rerank); written to app_secret on create */
  appSecret?: string
}

/** Protocols the backend accepts in extra_config.api (internal/models/api.API). */
const PROTOCOL_OPTIONS = [
  'openai-completions',
  'openai-responses',
  'anthropic-messages',
  'google-generative-ai',
] as const

/** Field reference for parameters.spec.compat, per protocol and model type. */
const COMPAT_DOC_URL = docsUrl('modelsCompat')

/** Legacy thinking_control values still honoured by catalog.Resolve. */
const LEGACY_THINKING_CONTROL_VALUES = ['none', 'enable_thinking', 'thinking_type', 'chat_template_kwargs'] as const

/** Keys of extra_config that are edited by dedicated controls, not the generic renderer. */
const RESERVED_EXTRA_CONFIG_KEYS = new Set(['thinking_control'])

type EditorModelType = 'chat' | 'embedding' | 'rerank' | 'vllm' | 'asr'

interface Props {
  visible: boolean
  modelType: EditorModelType
  modelData?: ModelFormData | null
  saveModel: (data: ModelFormData & { modelType?: EditorModelType }) => Promise<void>
}

const { t, locale } = useI18n()
const uiStore = useUIStore()
const providersStore = useModelProvidersStore()

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  modelData: null
})

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const draftModelType = ref<EditorModelType>(props.modelType)

const isEdit = computed(() => !!props.modelData)

const activeModelType = computed(() => (
  isEdit.value ? props.modelType : draftModelType.value
))

const modelTypeChoices = computed(() => ([
  { value: 'chat' as const, label: t('modelSettings.typeShort.chat'), icon: 'chat' },
  { value: 'embedding' as const, label: t('modelSettings.typeShort.embedding'), icon: 'chart-bubble' },
  { value: 'rerank' as const, label: t('modelSettings.typeShort.rerank'), icon: 'filter-sort' },
  { value: 'vllm' as const, label: t('modelSettings.typeShort.vllm'), icon: 'image' },
  { value: 'asr' as const, label: t('modelSettings.typeShort.asr'), icon: 'sound' },
]))

// The vendor list comes entirely from the backend catalog (the store caches it per model type); the frontend keeps no vendor table.
const loadingProviders = computed(() => providersStore.isLoading(activeModelType.value))

const loadProviders = async (force = false) => {
  try {
    await providersStore.ensureLoaded(activeModelType.value, force)
  } catch (error) {
    console.error('Failed to load providers from API', error)
  }
}

const providerOptions = computed<ModelProviderOption[]>(() => providersStore.providersFor(activeModelType.value))

const selectedProvider = computed<ModelProviderOption | undefined>(() => {
  const id = formData.value.provider
  if (!id) return undefined
  return providerOptions.value.find(p => p.value === id) || providersStore.providerById(id)
})

const currentLocale = computed(() => String(locale.value || ''))
const providerDisplayLabel = (p: ModelProviderOption) => providerLabel(p, currentLocale.value)
const providerDisplayDescription = (p: ModelProviderOption) => providerDescription(p, currentLocale.value)
const selectedProviderIcon = computed(() => providerIcon(selectedProvider.value))
/** Localized vendor name, or the raw stored id when the catalog no longer has it. */
const selectedProviderDisplayLabel = computed(() => (
  selectedProvider.value
    ? providerDisplayLabel(selectedProvider.value)
    : (formData.value.provider || '')
))
/**
 * Pin a select's dropdown to the width of its input.
 *
 * TDesign sizes a select popup as max(popup content, trigger)
 * (select-input/hooks/useOverlayInnerStyle), so one long option - a vendor
 * whose description lists half a dozen model ids - stretches the whole menu
 * past the field it belongs to, leaving a wide band of empty space and
 * pushing the tick mark far from the text. Passing a function here replaces
 * that matching outright (the hook keeps a function as-is), and the option
 * rows already ellipsize, so the long ones simply truncate.
 */
const matchTriggerWidth = (triggerElement: HTMLElement) => ({
  width: `${triggerElement.offsetWidth}px`,
})

const extraFieldDisplayLabel = (field: ModelProviderExtraField) => extraFieldLabel(field, currentLocale.value)
const extraFieldDisplayPlaceholder = (field: ModelProviderExtraField) =>
  extraFieldPlaceholder(field, currentLocale.value)
const extraFieldDisplayOptionLabel = (option: ModelProviderExtraFieldOption) =>
  extraFieldOptionLabel(option, currentLocale.value)

/**
 * Vendors whose API is not a bearer-token API name their first credential
 * themselves (LKEAP rerank takes a SecretId, Volcengine rerank an Access Key
 * ID). Falling back to the generic "API Key" wording is what led operators to
 * paste an `sk-` token into a signature field.
 */
const credentialLabel = computed(() =>
  credentialLabelForModelType(selectedProvider.value?.credentialLabels, activeModelType.value),
)
const apiKeyLabel = computed(() => {
  const label = credentialLabel.value
  return label ? pickLocalized(label.labels, currentLocale.value, label.label) : t('model.editor.apiKeyOptional')
})
const apiKeyRequired = computed(
  () => credentialLabel.value?.required === true || (!!selectedProvider.value?.requiresAuth && !isEdit.value),
)
const apiKeyHint = computed(() => {
  const label = credentialLabel.value
  if (!label?.hint) return ''
  return pickLocalized(label.hints, currentLocale.value, label.hint)
})

// Vendor extra fields: filtered by the current model type; secret fields go to the app_secret credential, the rest into extra_config.
const visibleExtraFields = computed<ModelProviderExtraField[]>(() =>
  extraFieldsForModelType(selectedProvider.value?.extraFields, activeModelType.value)
    .filter(field => !RESERVED_EXTRA_CONFIG_KEYS.has(field.key)),
)
/**
 * A credential-bearing vendor field. `secret` is the backend's own marker;
 * `type: 'password'` is treated the same way so a vendor that forgets the flag
 * still cannot get its value echoed back from GET /models into extra_config.
 */
const isSecretExtraField = (field: ModelProviderExtraField) => field.secret === true || field.type === 'password'
// Only one credential slot (app_secret) exists per model, so the first such
// field wins; vendors today declare at most one (LKEAP / Volcengine rerank).
const secretExtraField = computed<ModelProviderExtraField | undefined>(() =>
  visibleExtraFields.value.find(isSecretExtraField),
)
const plainExtraFields = computed<ModelProviderExtraField[]>(() =>
  visibleExtraFields.value.filter(field => !isSecretExtraField(field)),
)

/** extra_config keys that belong to the connection, not to the vendor. */
const VENDOR_NEUTRAL_EXTRA_CONFIG_KEYS = ['api', 'remote_model_name'] as const

/** What survives a vendor (or model-type) switch: everything else is vendor-specific. */
const keepVendorNeutralExtraConfig = (): Record<string, string> => {
  const keep: Record<string, string> = {}
  for (const key of VENDOR_NEUTRAL_EXTRA_CONFIG_KEYS) {
    const value = formData.value.extraConfig?.[key]
    if (value) keep[key] = value
  }
  return keep
}

const setExtraConfig = (key: string, value: string | null | undefined) => {
  const next = { ...(formData.value.extraConfig || {}) }
  const normalized = value == null ? '' : String(value)
  if (normalized === '') delete next[key]
  else next[key] = normalized
  formData.value.extraConfig = next
}

const extraConfigBool = (key: string) => {
  const raw = (formData.value.extraConfig?.[key] || '').trim().toLowerCase()
  return raw === 'true' || raw === '1' || raw === 'yes'
}

/** Pre-fill vendor defaults for fields the user has not touched. */
const applyExtraFieldDefaults = () => {
  for (const field of plainExtraFields.value) {
    if (!field.default) continue
    if ((formData.value.extraConfig?.[field.key] ?? '') === '') {
      setExtraConfig(field.key, field.default)
    }
  }
}

// Built-in model catalog -> model name dropdown candidates (free input allowed)
interface CatalogModelOption {
  value: string
  label: string
  contextWindow: string
  dimension?: number
  reasoning: boolean
  vision: boolean
  entry?: ModelCatalogEntry
}

const catalogEntries = computed<ModelCatalogEntry[]>(() => {
  const entries = selectedProvider.value?.models || []
  // Managed aliases are already used by the cloud setup page. They have no
  // published limits, so the backend catalog is empty, but they can still be
  // offered by name without inventing context windows or embedding dimensions.
  if (formData.value.provider === AIKB_CLOUD_PROVIDER && entries.length === 0) {
    const kind = WKC_MODEL_KINDS.find(kind => kind === activeModelType.value)
    if (kind) {
      const name = WKC_MODEL_NAME_BY_KIND[kind]
      return [{ id: name, name, type: kind === 'vllm' ? 'chat' : kind, input: kind === 'vllm' ? ['text', 'image'] : ['text'] }]
    }
  }
  // The list is already scoped: providers are fetched per model type, so the
  // backend returned exactly the entries that type can use. Filtering again
  // here on entry.type was wrong for vision - a VLM entry is a chat model that
  // accepts images, so it arrives typed "chat" and every one of them was
  // dropped, leaving the picker empty for every vendor.
  return entries
})

const catalogModelOptions = computed<CatalogModelOption[]>(() => {
  const options: CatalogModelOption[] = catalogEntries.value.map(entry => ({
    value: entry.id,
    label: entry.name || entry.id,
    contextWindow: entry.context_window ? formatTokenCount(entry.context_window) : '',
    dimension: entry.dimension || undefined,
    reasoning: !!entry.reasoning || (entry.thinking_levels?.length ?? 0) > 0,
    vision: Array.isArray(entry.input) && entry.input.includes('image'),
    entry,
  }))
  const current = (formData.value.modelName || '').trim()
  if (current && !options.some(o => o.value === current)) {
    options.unshift({ value: current, label: current, contextWindow: '', reasoning: false, vision: false })
  }
  return options
})

// Decided by the vendor catalog alone: catalogModelOptions also holds the typed name, so keying the
// input-versus-select switch on it swapped the field out after the first keystroke.
const hasCatalog = computed(() => catalogEntries.value.length > 0)

const findCatalogEntry = (name: string) => catalogEntries.value.find(m => m.id === name)

/**
 * Where to read about what is configured right now.
 *
 * Prefer the page the selected model's facts were taken from - that is the
 * page listing its context window, thinking levels and price - and fall back
 * to the vendor's own site when the model is not in the catalog. Both come
 * from the catalog, so a new vendor gets the link without a UI change.
 */
const vendorDocLink = computed(() => {
  const provider = selectedProvider.value
  if (!provider) return ''
  const entry = findCatalogEntry((formData.value.modelName || '').trim())
  const url = entry?.source || provider.website || ''
  return /^https?:\/\//i.test(url) ? url : ''
})

/**
 * What applyCatalogEntry filled in for the model currently selected.
 *
 * Switching vendor has to take those values back - a context window from the
 * previous vendor's model is exactly the "too large means compression never triggers and the upstream rejects it outright"
 * case this field warns about - but it must not touch a number the operator
 * typed. Remembering what was filled, and only clearing a field that still
 * holds it, separates the two.
 */
const catalogFilled = ref<Partial<ModelFormData>>({})

/** Whether the catalog says the selected embedding model takes images. */
const catalogEmbeddingAcceptsImages = computed(() =>
  !!findCatalogEntry((formData.value.modelName || '').trim())?.input?.includes('image'))

/**
 * Image input of an embedding model: spec.input when the row declares it,
 * the catalog otherwise. The row only stores a declaration that differs from
 * the catalog, so a later catalog correction still reaches it.
 */
const embeddingAcceptsImages = computed<boolean>({
  get: () => {
    const input = formData.value.spec?.input
    return Array.isArray(input) && input.length > 0
      ? input.includes('image')
      : catalogEmbeddingAcceptsImages.value
  },
  set: (value) => {
    const spec: ModelSpecOverride = { ...(formData.value.spec || {}) }
    if (value === catalogEmbeddingAcceptsImages.value) delete spec.input
    else spec.input = value ? ['text', 'image'] : ['text']
    formData.value.spec = Object.keys(spec).length > 0 ? spec : null
  },
})

/** A declared input belongs to the model it was declared for. */
const dropEmbeddingInputOverride = () => {
  if (!formData.value.spec?.input) return
  const spec: ModelSpecOverride = { ...formData.value.spec }
  delete spec.input
  formData.value.spec = Object.keys(spec).length > 0 ? spec : null
}

/** Fill blank capability fields from a catalog entry the user just picked. */
const applyCatalogEntry = (entry: ModelCatalogEntry) => {
  if (activeModelType.value === 'chat' || activeModelType.value === 'vllm') {
    if (!formData.value.contextWindow && entry.context_window) {
      formData.value.contextWindow = entry.context_window
      catalogFilled.value.contextWindow = entry.context_window
    }
    if (!formData.value.maxOutputTokens && entry.max_output_tokens) {
      formData.value.maxOutputTokens = entry.max_output_tokens
      catalogFilled.value.maxOutputTokens = entry.max_output_tokens
    }
  }
  if (activeModelType.value === 'chat' && !formData.value.supportsVision && Array.isArray(entry.input) && entry.input.includes('image')) {
    formData.value.supportsVision = true
    catalogFilled.value.supportsVision = true
  }
  if (activeModelType.value === 'embedding') dropEmbeddingInputOverride()
  if (activeModelType.value === 'embedding' && !formData.value.dimension && entry.dimension) {
    formData.value.dimension = entry.dimension
    catalogFilled.value.dimension = entry.dimension
  }
}

const handleCatalogModelCreate = (value: string | number | boolean | bigint) => {
  formData.value.modelName = String(value ?? '').trim()
}

const handleCatalogModelChange = (value: unknown) => {
  const name = typeof value === 'string' ? value.trim() : ''
  if (!name) return
  const entry = findCatalogEntry(name)
  if (entry) applyCatalogEntry(entry)
}

// Access diagnostics (read-only): call catalog resolution with a 400ms debounce after the vendor / model name / Base URL / protocol override changes
const resolved = ref<ResolvedModelCatalog | null>(null)
const resolving = ref(false)
const resolveFailed = ref(false)
/** Human-readable detail of the last failure; '' when the error carried none. */
const resolveError = ref('')
let resolveTimer: ReturnType<typeof setTimeout> | null = null
let resolveRevision = 0

/** The protocol / thinking levels / context window resolved from the catalog only matter for chat and VLM models. */
const isChatLike = computed(() => activeModelType.value === 'chat' || activeModelType.value === 'vllm')

const showResolvedPanel = computed(() =>
  isChatLike.value
  && formData.value.source === 'remote'
  && !!formData.value.provider
  && formData.value.provider !== 'weknoracloud',
)

const resolvedThinkingLevels = computed(() => supportedLevels(resolved.value?.capabilities))

const runResolve = async () => {
  const revision = ++resolveRevision
  const provider = (formData.value.provider || '').trim()
  // Embedding / ReRank / ASR never render the panel, so do not spend a
  // request (and do not leave a stale chat result behind) for them.
  if (!props.visible || !showResolvedPanel.value || !provider) {
    resolved.value = null
    resolving.value = false
    resolveFailed.value = false
    resolveError.value = ''
    return
  }
  resolving.value = true
  try {
    const result = await resolveModelCatalog({
      provider,
      spec: buildSpec(),
      model: formData.value.modelName || '',
      base_url: formData.value.baseUrl || '',
      model_type: activeModelType.value,
      api: formData.value.extraConfig?.api || '',
      thinking_control: formData.value.thinkingControl || '',
      remote_model_name: formData.value.extraConfig?.remote_model_name || '',
      // Vendor fields decide the request too - Azure's api_version picks
      // between the v1 data plane and the dated deployments path - so the
      // preview has to see them or it describes a different request than
      // the one this row will make. Secret fields never travel in a query
      // string; plainExtraFields already excludes them.
      ...Object.fromEntries(
        plainExtraFields.value
          .map(field => [field.key, formData.value.extraConfig?.[field.key] || ''])
          .filter(([, value]) => !!value),
      ),
    })
    if (revision !== resolveRevision) return
    resolved.value = result
    resolveFailed.value = false
    resolveError.value = ''
  } catch (error: any) {
    if (revision !== resolveRevision) return
    resolved.value = null
    resolveFailed.value = true
    const detail = typeof error === 'string'
      ? error
      : (error?.message || error?.error?.message || error?.error)
    resolveError.value = typeof detail === 'string' ? detail.trim() : ''
  } finally {
    if (revision === resolveRevision) resolving.value = false
  }
}

const scheduleResolve = () => {
  if (resolveTimer) clearTimeout(resolveTimer)
  resolveTimer = setTimeout(() => {
    resolveTimer = null
    void runResolve()
  }, 400)
}

// Advanced collapsible section
const advancedOpen = ref(false)
/** Set when the loaded row carried thinking_control, so the select stays visible after clearing it. */
const legacyThinkingControlLoaded = ref(false)
const showLegacyThinkingControl = computed(() =>
  activeModelType.value === 'chat' && !!(formData.value.thinkingControl || legacyThinkingControlLoaded.value),
)

const specCompatError = computed(() => {
  const text = (formData.value.specCompat || '').trim()
  if (!text) return ''
  try {
    const parsed = JSON.parse(text)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return t('model.editor.advanced.compat.mustBeObject')
    }
    return ''
  } catch (error: any) {
    return error?.message || 'invalid JSON'
  }
})

// Match the parent save path: retain row metadata, replace the edited compat.
const buildSpec = (): ModelSpecOverride => {
  if (specCompatError.value) throw new Error(specCompatError.value)
  const spec: ModelSpecOverride = { ...(formData.value.spec || {}) }
  const text = (formData.value.specCompat || '').trim()
  if (text) spec.compat = JSON.parse(text)
  else delete spec.compat
  return spec
}

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => {
    if (!saving.value) emit('update:visible', val)
  }
})

/** Filling the form from modelData; ignore the side effects of programmatic changes to the vendor/source controls */
const hydratingForm = ref(false)

// Header icon for the SettingDrawer - uses the same TDesign icon name table
// as the model card list, so the drawer's leading badge visually matches the
// card the user just clicked on.
const modelTypeIcon = computed(() => {
  const map: Record<string, string> = {
    chat: 'chat',
    embedding: 'chart-bubble',
    rerank: 'filter-sort',
    vllm: 'image',
    asr: 'sound',
  }
  return map[activeModelType.value] || 'setting'
})

// Credential resource binding for the shared <CredentialResource> component.
// A vendor-declared secret extra field (LKEAP / Volcengine rerank SecretKey)
// is stored as the app_secret credential, so it shows up here in edit mode.
const credentialFields = computed<CredentialFieldDef<ModelCredentialField>[]>(() => {
  const fields: CredentialFieldDef<ModelCredentialField>[] = [
    { key: 'api_key', label: t('model.editor.apiKeyOptional') as string },
  ]
  if (formData.value.provider === 'weknoracloud') {
    fields.push({ key: 'app_secret', label: 'App Secret' })
  } else if (secretExtraField.value) {
    fields.push({ key: 'app_secret', label: extraFieldDisplayLabel(secretExtraField.value) })
  }
  return fields
})

const credentialApi = computed<CredentialResourceApi<ModelCredentialField>>(() => {
  const id = props.modelData?.id ?? ''
  return {
    save: async (patch) => {
      const meta = await putModelCredentials(id, patch)
      return meta.fields
    },
    remove: async (field) => {
      await deleteModelCredentialField(id, field)
    },
  }
})

// Initial credential metadata. ModelSettings.convertToLegacyFormat
// preserves `credentials` from the main ListModels response so the card
// renders the correct "Configured" state on dialog open.
const credentialMeta = computed(() => (props.modelData as any)?.credentials ?? {
  api_key: { configured: false },
  app_secret: { configured: false },
})

// Placeholder hint for the create-mode API key input. Edit mode replaces
// this input entirely with a <CredentialResource> card.
const apiKeyPlaceholder = computed(() => {
  const label = credentialLabel.value
  if (label?.placeholder) return pickLocalized(label.placeholders, currentLocale.value, label.placeholder)
  return t('model.editor.apiKeyPlaceholder')
})

const formRef = ref()
const saving = ref(false)
const saveError = ref('')

// Settings itself listens on window for Escape. Capture it while saving so
// the parent cannot unmount this editor before the request finishes.
const handleSaveEscape = (event: KeyboardEvent) => {
  if (props.visible && saving.value && (event.key === 'Escape' || event.code === 'Escape')) {
    event.preventDefault()
    event.stopImmediatePropagation()
  }
}
watch(() => props.visible && saving.value, (locked) => {
  if (locked) window.addEventListener('keydown', handleSaveEscape, true)
  else window.removeEventListener('keydown', handleSaveEscape, true)
}, { flush: 'sync' })
// Toggles the create-mode API key input between masked and plain text. Lets
// the user proofread a freshly pasted secret without losing the password
// affordance for everyday use. Reset every time the drawer closes (see
// reset block in the visible watcher) so we never leak the previous value
// across editor sessions.
const modelChecked = ref(false)
const modelAvailable = ref(false)
const checking = ref(false)
const remoteChecked = ref(false)
const remoteAvailable = ref(false)
const remoteMessage = ref('')
const remoteStale = ref(false)
let connectionRevision = 0
let applyingDetectedDimension = false

// Invalidate pending responses too, even when the user changes a field back.
const invalidateConnectionTest = (showStale = true) => {
  connectionRevision++
  remoteStale.value = showStale && (remoteStale.value || checking.value || remoteChecked.value)
  checking.value = false
  remoteChecked.value = false
  remoteAvailable.value = false
  remoteMessage.value = ''
  dimensionChecked.value = false
  dimensionSuccess.value = false
  dimensionMessage.value = ''
}

const applyDetectedDimension = (dimension: number) => {
  applyingDetectedDimension = true
  try {
    formData.value.dimension = dimension
  } finally {
    applyingDetectedDimension = false
  }
}
const dimensionChecked = ref(false)
const dimensionSuccess = ref(false)
const dimensionMessage = ref('')

// Ollama model status
const ollamaModelList = ref<OllamaModelInfo[]>([])
const loadingOllamaModels = ref(false)
const searchKeyword = ref('')
const downloading = ref(false)
const downloadProgress = ref(0)
const currentDownloadModel = ref('')
let downloadInterval: any = null

// Ollama service status
const ollamaServiceStatus = ref<boolean | null>(null)
const checkingOllamaStatus = ref(false)

// WeKnoraCloud credential status
const wkcCredentialState = ref<'loading' | 'unconfigured' | 'configured' | 'expired'>('loading')

const checkWkcCredentialStatus = async () => {
  wkcCredentialState.value = 'loading'
  try {
    const status = await getWeKnoraCloudStatus()
    if (status.needs_reinit) {
      wkcCredentialState.value = 'expired'
    } else if (status.has_models) {
      wkcCredentialState.value = 'configured'
    } else {
      wkcCredentialState.value = 'unconfigured'
    }
  } catch {
    wkcCredentialState.value = 'unconfigured'
  }
}

const goToWeKnoraCloudSettings = async () => {
  emit('update:visible', false)
  if (uiStore.showSettingsModal) {
    uiStore.closeSettings()
    await nextTick()
  }
  uiStore.openSettings('weknoracloud')
}

const formData = ref<ModelFormData>({
  id: '',
  name: '',
  source: 'remote',
  provider: 'generic',
  modelName: '',
  displayName: '',
  baseUrl: '',
  apiKey: '',
  dimension: undefined,
  supportsDimensionOverride: false,
  interfaceType: 'ollama',
  isDefault: false,
  supportsVision: false,
  contextWindow: undefined,
  maxConcurrency: undefined,
  maxOutputTokens: undefined,
  thinkingControl: '',
  extraConfig: {},
  specCompat: '',
  spec: null,
  customHeaders: [],
  appSecret: '',
})

const rules = computed(() => ({
  modelName: [
    { required: true, message: t('model.editor.validation.modelNameRequired') },
    {
      validator: (val: string) => {
        if (!val || !val.trim()) {
          return { result: false, message: t('model.editor.validation.modelNameEmpty') }
        }
        if (val.trim().length > 100) {
          return { result: false, message: t('model.editor.validation.modelNameMax') }
        }
        return { result: true }
      },
      trigger: 'blur'
    }
  ],
  baseUrl: [
    {
      required: true,
      message: t('model.editor.validation.baseUrlRequired'),
      trigger: 'blur'
    },
    {
      validator: (val: string) => {
        if (!val || !val.trim()) {
          return { result: false, message: t('model.editor.validation.baseUrlEmpty') }
        }
        // Simple URL format check
        try {
          new URL(val.trim())
          return { result: true }
        } catch {
          return { result: false, message: t('model.editor.validation.baseUrlInvalid') }
        }
      },
      trigger: 'blur'
    }
  ]
}))

// Triggers for access diagnostics: visibility / source / vendor / model name / Base URL / advanced overrides / model type
watch(
  () => [
    props.visible, formData.value.source, formData.value.provider, formData.value.modelName,
    formData.value.baseUrl, formData.value.extraConfig?.api, formData.value.extraConfig?.remote_model_name,
    formData.value.thinkingControl, activeModelType.value,
 formData.value.specCompat, JSON.stringify(formData.value.spec),
  ],
  () => {
    if (!props.visible) return
    scheduleResolve()
  },
)

// Get the dialog description text
const getModalDescription = () => {
  const key = `model.editor.description.${activeModelType.value}` as const
  return t(key) || t('model.editor.description.default')
}

// Get the model name placeholder
const getModelNamePlaceholder = () => {
  const catalogModel = catalogEntries.value[0]
  if (formData.value.source === 'remote' && catalogModel) return catalogModel.id
  if (activeModelType.value === 'vllm') {
    return formData.value.source === 'local'
      ? t('model.editor.modelNamePlaceholder.localVllm')
      : t('model.editor.modelNamePlaceholder.remoteVllm')
  }
  if (activeModelType.value === 'asr') {
    return t('model.editor.modelNamePlaceholder.remoteAsr')
  }
  return formData.value.source === 'local'
    ? t('model.editor.modelNamePlaceholder.local')
    : t('model.editor.modelNamePlaceholder.remote')
}

const getBaseUrlPlaceholder = () => {
  if (activeModelType.value === 'vllm') {
    return t('model.editor.baseUrlPlaceholderVllm')
  }
  if (activeModelType.value === 'asr') {
    return t('model.editor.baseUrlPlaceholderAsr')
  }
  return t('model.editor.baseUrlPlaceholder')
}

// Check the Ollama service status
const checkOllamaServiceStatus = async () => {
  console.log('Checking Ollama service status...')
  checkingOllamaStatus.value = true
  try {
    const result = await checkOllamaStatus()
    ollamaServiceStatus.value = result.available
    console.log('Ollama service status check done:', result.available)
  } catch (error) {
    console.error('Failed to check Ollama service status:', error)
    ollamaServiceStatus.value = false
  } finally {
    checkingOllamaStatus.value = false
  }

  // When Ollama is unavailable, default new models to remote
  if (ollamaServiceStatus.value === false && !isEdit.value && formData.value.source === 'local') {
    formData.value.source = 'remote'
  }
}

// Open the Ollama settings window
const goToOllamaSettings = async () => {
  console.log('Clicked the go-to-Ollama-settings button')
  // Close the current dialog
  emit('update:visible', false)

  // Close the settings dialog first (if it is open)
  if (uiStore.showSettingsModal) {
    uiStore.closeSettings()
    // Wait for the DOM to update
    await nextTick()
  }

  // Open the settings window and go straight to the Ollama settings
  console.log('Calling uiStore.openSettings')
  uiStore.openSettings('ollama')
  console.log('uiStore.openSettings call done')
}

// modelData id from the last open: tells switching models / adding vs. consecutive opens of the same add
const lastOpenedModelId = ref<string | null>(null)

const selectModelType = async (type: EditorModelType) => {
  if (saving.value || isEdit.value || draftModelType.value === type) return
  draftModelType.value = type
  // A different model type starts a new connection, including credentials
  // and advanced overrides. Clear the old draft before waiting for its catalog.
  resetForm()
  const draft = formData.value
  await loadProviders()
  if (props.visible && !isEdit.value && formData.value === draft && activeModelType.value === type) {
    applyExtraFieldDefaults()
  }
}

// Watch visible changes and initialize the form
watch(() => props.visible, (val) => {
  if (val) {
    const typeChanged = draftModelType.value !== props.modelType
    // Sync the entry type before loading its catalog; the previous draft may
    // belong to another settings tab (including the initial chat default).
    draftModelType.value = props.modelType

    // Check the Ollama service status
    checkOllamaServiceStatus()

    advancedOpen.value = false

    // Clear the leftover check/test results from the last open every time, so editing another model does not
    // show the previous "Connected" straight away
    modelChecked.value = false
    modelAvailable.value = false
    remoteChecked.value = false
    remoteAvailable.value = false
    remoteMessage.value = ''
    dimensionChecked.value = false
    dimensionSuccess.value = false
    dimensionMessage.value = ''

    const currentId = props.modelData?.id ?? null

    hydratingForm.value = true
    try {
      if (props.modelData) {
        // Edit: always overwrite with the latest modelData. apiKey field is left blank - in
        // edit mode the credential is owned by the <CredentialResource> card,
        // not by this form's apiKey field.
        const loadedExtra: Record<string, string> = {}
        for (const [key, value] of Object.entries(props.modelData.extraConfig || {})) {
          if (RESERVED_EXTRA_CONFIG_KEYS.has(key)) continue
          if (value != null && String(value) !== '') loadedExtra[key] = String(value)
        }
        const loadedSpec = props.modelData.spec || null
        formData.value = {
          ...props.modelData,
          apiKey: '',
          appSecret: '',
          extraConfig: loadedExtra,
          thinkingControl: props.modelData.thinkingControl || '',
          spec: loadedSpec,
          specCompat: props.modelData.specCompat
            ?? (loadedSpec?.compat ? JSON.stringify(loadedSpec.compat, null, 2) : ''),
          customHeaders: Array.isArray(props.modelData.customHeaders)
            ? props.modelData.customHeaders.map(h => ({ key: h.key, value: h.value }))
            : [],
        }
        legacyThinkingControlLoaded.value = !!props.modelData.thinkingControl
      } else if (typeChanged || lastOpenedModelId.value !== null || !formData.value.id) {
        // Switching type, last time was editing a model, or the first add -> reset to blank
        resetForm()
      }
      // Two consecutive "add" opens of the same type (closed by clicking the overlay / ESC in between) -> keep what was entered

      lastOpenedModelId.value = currentId

      // ReRank models force the remote source (Ollama does not support ReRank)
      if (activeModelType.value === 'rerank') {
        formData.value.source = 'remote'
      }

      // If the current provider is WeKnoraCloud, check the credential status
      if (formData.value.provider === 'weknoracloud') {
        checkWkcCredentialStatus()
      }
    } finally {
      nextTick(() => {
        hydratingForm.value = false
      })
    }

    // Catalogs can be published while this page is open. Refresh this type,
    // but do not apply defaults to a different draft if the request finishes late.
    const draft = formData.value
    const type = activeModelType.value
    loadProviders(true).then(() => {
      if (props.visible && !isEdit.value && formData.value === draft && activeModelType.value === type) {
        applyExtraFieldDefaults()
      }
    })
  }
})

watch(
  () => [
    activeModelType.value, props.modelData?.id, formData.value.source,
    formData.value.provider, formData.value.modelName, formData.value.baseUrl,
    formData.value.apiKey, formData.value.appSecret, formData.value.customHeaders,
    formData.value.dimension, formData.value.supportsDimensionOverride,
    formData.value.extraConfig, formData.value.thinkingControl,
 formData.value.specCompat, formData.value.spec,
  ],
  () => {
    if (!applyingDetectedDimension) invalidateConnectionTest(props.visible && !hydratingForm.value)
  },
  { deep: true, flush: 'sync' },
)

watch(() => props.visible, () => {
  invalidateConnectionTest(false)
  saveError.value = ''
}, { flush: 'sync' })

// Reset the form
const resetForm = () => {
  legacyThinkingControlLoaded.value = false
  formData.value = {
    id: generateId(),
    name: '', // field kept but unused; saving uses modelName
    source: 'remote',
    provider: 'generic',
    modelName: '',
    displayName: '',
    baseUrl: '',
    apiKey: '',
    dimension: undefined, // empty by default; the user types it or gets it from the detect button
    supportsDimensionOverride: false,
    interfaceType: undefined,
    isDefault: false,
    supportsVision: false,
    contextWindow: undefined,
    maxConcurrency: undefined,
    maxOutputTokens: undefined,
    thinkingControl: '',
    extraConfig: {},
    specCompat: '',
    spec: null,
    customHeaders: [],
    appSecret: '',
  }
  catalogFilled.value = {}
  advancedOpen.value = false
  saveError.value = ''
  searchKeyword.value = ''
  invalidateConnectionTest(false)
  resolveRevision++
  if (resolveTimer) {
    clearTimeout(resolveTimer)
    resolveTimer = null
  }
  resolving.value = false
  resolved.value = null
  resolveFailed.value = false
  resolveError.value = ''
  modelChecked.value = false
  modelAvailable.value = false
  remoteChecked.value = false
  remoteAvailable.value = false
  remoteMessage.value = ''
  dimensionChecked.value = false
  dimensionSuccess.value = false
  dimensionMessage.value = ''
  nextTick(() => formRef.value?.clearValidate?.())
}

// Handle a vendor selection change (fills in the default URL)
/**
 * Drop a model name the new vendor does not serve, along with whatever the
 * catalog filled in for it.
 *
 * The same id does exist at several vendors - deepseek-v4-pro is sold by
 * DeepSeek, Aliyun, Volcengine and the gateways - so switching between them
 * should keep the selection. Anything else is a name from the previous
 * vendor: left in place it is saved verbatim, resolves as an uncatalogued
 * model and fails at the first call.
 */
const resetModelSelectionForVendor = () => {
  const current = (formData.value.modelName || '').trim()
  if (!current || findCatalogEntry(current)) return

  formData.value.modelName = ''
  // Take back only the values applyCatalogEntry put there; a number the
  // operator typed is theirs and survives the switch.
  const filled = catalogFilled.value
  if (filled.contextWindow && formData.value.contextWindow === filled.contextWindow) {
    formData.value.contextWindow = undefined
  }
  if (filled.maxOutputTokens && formData.value.maxOutputTokens === filled.maxOutputTokens) {
    formData.value.maxOutputTokens = undefined
  }
  if (filled.dimension && formData.value.dimension === filled.dimension) {
    formData.value.dimension = undefined
  }
  if (filled.supportsVision && formData.value.supportsVision) {
    formData.value.supportsVision = false
  }
  if (activeModelType.value === 'embedding') dropEmbeddingInputOverride()
  catalogFilled.value = {}
  modelChecked.value = false
  modelAvailable.value = false
  dimensionChecked.value = false
  dimensionSuccess.value = false
  dimensionMessage.value = ''
}

const handleProviderChange = (value: string) => {
  const provider = providerOptions.value.find(opt => opt.value === value)
  // Older catalogs omit inherited type URLs. Mirror GetDefaultURL's chat
  // fallback, preserving an explicitly empty URL and never the previous vendor's.
  formData.value.baseUrl = provider?.defaultUrls?.[activeModelType.value]
    ?? provider?.defaultUrls?.chat
    ?? ''
  // Reset the check state: it describes the previous vendor's connectivity, unrelated to the new vendor. It sits outside the defaultUrls
  // check, otherwise switching to a vendor without a default URL would leave a stale "Connected" verdict.
  remoteChecked.value = false
  remoteAvailable.value = false
  remoteMessage.value = ''
  // WeKnoraCloud: check the credential status
  if (value === 'weknoracloud') {
    checkWkcCredentialStatus()
  }
  if (hydratingForm.value) return
  resetModelSelectionForVendor()
  // Vendor change: drop the previous vendor's fields (keeping advanced overrides such as protocol / remote model name), then fill in the new vendor's defaults
  formData.value.extraConfig = keepVendorNeutralExtraConfig()
  formData.value.appSecret = ''
  applyExtraFieldDefaults()
}

// Watch source changes and reset the check state (merged into the watch below)

// Generate a unique ID
const generateId = () => {
  return `model_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`
}

// Custom HTTP header editing
const addCustomHeader = () => {
  if (!Array.isArray(formData.value.customHeaders)) {
    formData.value.customHeaders = []
  }
  formData.value.customHeaders.push({ key: '', value: '' })
}

const removeCustomHeader = (idx: number) => {
  if (!Array.isArray(formData.value.customHeaders)) return
  formData.value.customHeaders.splice(idx, 1)
}

// Filtered model list
const filteredOllamaModels = computed(() => {
  if (!searchKeyword.value) return ollamaModelList.value
  return ollamaModelList.value.filter(model =>
    model.name.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

// Whether to show the "Download model" option
const showDownloadOption = computed(() => {
  if (!searchKeyword.value.trim()) return false
  // Check whether the search term already exists in the model list
  const exists = ollamaModelList.value.some(model =>
    model.name.toLowerCase() === searchKeyword.value.toLowerCase()
  )
  return !exists
})

// Custom filter logic (captures the search keyword)
const handleModelFilter = (filterWords: string) => {
  searchKeyword.value = filterWords
  return true // let TDesign use our filteredOllamaModels
}

// Load the Ollama model list
const loadOllamaModels = async () => {
  // Load only when the local source is selected
  if (formData.value.source !== 'local') return

  loadingOllamaModels.value = true
  try {
    const models = await listOllamaModels()
    ollamaModelList.value = models
  } catch (error) {
    console.error(t('model.editor.loadModelListFailed'), error)
    MessagePlugin.error(t('model.editor.loadModelListFailed'))
  } finally {
    loadingOllamaModels.value = false
  }
}

// Refresh the model list
const refreshOllamaModels = async () => {
  ollamaModelList.value = [] // clear to force a reload
  await loadOllamaModels()
  MessagePlugin.success(t('model.editor.listRefreshed'))
}

// Watch dropdown visibility changes
const handleDropdownVisibleChange = (visible: boolean) => {
  if (!visible) {
    searchKeyword.value = ''
  }
}

// Format the model size
const formatModelSize = (bytes: number): string => {
  if (!bytes || bytes === 0) return ''
  const gb = bytes / (1024 * 1024 * 1024)
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / (1024 * 1024)).toFixed(0)} MB`
}

// Check the model status (Ollama local model)
const checkModelStatus = async () => {
  if (!formData.value.modelName || formData.value.source !== 'local') {
    return
  }

  try {
    // Call the real Ollama API to check whether the model exists
    const result = await checkOllamaModels([formData.value.modelName])
    modelChecked.value = true
    modelAvailable.value = result.models[formData.value.modelName] || false
  } catch (error) {
    console.error('Failed to check model status:', error)
    modelChecked.value = false
    modelAvailable.value = false
  }
}

// Check the dimension of an Ollama local embedding model
const checkOllamaDimension = async () => {
  if (checking.value || saving.value) return
  if (!formData.value.modelName || formData.value.source !== 'local' || activeModelType.value !== 'embedding') {
    return
  }

  const revision = ++connectionRevision
  checking.value = true
  dimensionChecked.value = false
  dimensionMessage.value = ''

  try {
    const result = await testEmbeddingModel({
      source: 'local',
      modelName: formData.value.modelName,
      dimension: formData.value.dimension,
      supportsDimensionOverride: formData.value.supportsDimensionOverride ?? false,
    })

    if (revision !== connectionRevision) return
    dimensionChecked.value = true
    dimensionSuccess.value = result.available || false

    if (result.available && result.dimension) {
      applyDetectedDimension(result.dimension)
      dimensionMessage.value = t('model.editor.dimensionDetected', { value: result.dimension })
      MessagePlugin.success(dimensionMessage.value)
    } else {
      if (result.message) {
        console.debug('Backend dimension message:', result.message)
      }
      dimensionMessage.value = t('model.editor.dimensionFailed')
      MessagePlugin.warning(dimensionMessage.value)
    }
  } catch (error: any) {
    if (revision !== connectionRevision) return
    console.error('Ollama dimension check failed:', error)
    dimensionChecked.value = true
    dimensionSuccess.value = false
    dimensionMessage.value = t('model.editor.dimensionFailed')
    MessagePlugin.error(dimensionMessage.value)
  } finally {
    if (revision === connectionRevision) checking.value = false
  }
}

/** extra_config exactly as it will be persisted: trimmed, empty values dropped. */
const buildExtraConfig = (): Record<string, string> => {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(formData.value.extraConfig || {})) {
    const trimmed = (value ?? '').toString().trim()
    if (key && trimmed) out[key] = trimmed
  }
  const legacy = (formData.value.thinkingControl || '').trim()
  if (legacy && activeModelType.value === 'chat' && formData.value.source === 'remote') {
    out.thinking_control = legacy
  }
  return out
}

// Check the remote API connection (calls a different endpoint per model type)
const checkRemoteAPI = async () => {
  if (checking.value || saving.value) return
  if (!formData.value.modelName || (!formData.value.baseUrl && formData.value.provider !== 'weknoracloud')) {
    MessagePlugin.warning(t('model.editor.fillModelAndUrl'))
    return
  }

  const revision = ++connectionRevision
  checking.value = true
  remoteStale.value = false
  remoteChecked.value = false
  remoteMessage.value = ''

  try {
    let result: any

    // Turn the custom headers in the form, a key-value array, into the map the backend expects.
    // Same as ModelSettings.vue on save: empty rows are dropped, so the connection test and the production calls after a real save
    // use exactly the same header set.
    const customHeaders: Record<string, string> = {}
    if (Array.isArray(formData.value.customHeaders)) {
      for (const item of formData.value.customHeaders) {
        const key = (item?.key ?? '').trim()
        const value = (item?.value ?? '').trim()
        if (key && value) customHeaders[key] = value
      }
    }
    // Send the field only when non-empty, so no empty object shows up in URL queries / logs
    const headerPayload = Object.keys(customHeaders).length > 0
      ? { customHeaders }
      : {}

    // Call a different check endpoint per model type
    // In edit mode the apiKey is managed separately by <CredentialResource> and is not in formData.
    // Pass modelId through to the backend so it falls back to the stored decrypted value when apiKey is empty,
    // avoiding a "connection test fails because it sent no apiKey".
    const idPayload = isEdit.value && props.modelData?.id
      ? { modelId: props.modelData.id as string }
      : {}

    // extra_config exactly as on a real save (vendor fields + advanced overrides + legacy thinking_control),
    // so the connection test takes the same catalog resolution path as production calls.
    const extraConfig = buildExtraConfig()
    const extraPayload = {
      spec: buildSpec(),
      ...(Object.keys(extraConfig).length > 0 ? { extraConfig } : {}),
      ...(formData.value.appSecret?.trim() ? { appSecret: formData.value.appSecret.trim() } : {}),
    }

    switch (activeModelType.value) {
      case 'chat':
        // Chat model (KnowledgeQA)
        result = await checkRemoteModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || '',
          apiKey: formData.value.apiKey || '',
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
          ...extraPayload,
        })
        break

      case 'embedding':
        // Embedding model
        result = await testEmbeddingModel({
          source: 'remote',
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || '',
          apiKey: formData.value.apiKey || '',
          dimension: formData.value.dimension,
          supportsDimensionOverride: formData.value.supportsDimensionOverride ?? false,
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
          ...extraPayload,
        })
        // If the test succeeded and returned a dimension, fill it in
        if (revision !== connectionRevision) return
        if (result.available && result.dimension) applyDetectedDimension(result.dimension)
        break

      case 'rerank':
        result = await checkRerankModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || '',
          apiKey: formData.value.apiKey || '',
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
          ...extraPayload,
        })
        break

      case 'vllm':
        // VLLM model (multimodal)
        // VLLM uses checkRemoteModel for a basic connection test
        result = await checkRemoteModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || '',
          apiKey: formData.value.apiKey || '',
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
          ...extraPayload,
        })
        break

      case 'asr':
        // ASR model (speech recognition) - uses the dedicated ASR test endpoint (/v1/audio/transcriptions)
        result = await checkASRModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || '',
          apiKey: formData.value.apiKey || '',
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
          ...extraPayload,
        })
        break

      default:
        MessagePlugin.error(t('model.editor.unsupportedModelType'))
        return
    }

    if (revision !== connectionRevision) return
    remoteChecked.value = true
    remoteAvailable.value = result.available || false
    remoteMessage.value = result.available
      ? t('model.editor.connectionSuccess')
      : result.message || t('model.editor.connectionFailed')
  } catch (error: any) {
    if (revision !== connectionRevision) return
    remoteChecked.value = true
    remoteAvailable.value = false
    remoteMessage.value = error?.message || t('model.editor.connectionConfigError')
  } finally {
    if (revision === connectionRevision) checking.value = false
  }
}

// Confirm save
const handleConfirm = async () => {
  if (saving.value) return
  saving.value = true
  if (checking.value) invalidateConnectionTest()
  saveError.value = ''
  try {
    // Check required fields by hand
    if (!formData.value.modelName || !formData.value.modelName.trim()) {
      MessagePlugin.warning(t('model.editor.validation.modelNameRequired'))
      return
    }

    if (formData.value.modelName.trim().length > 100) {
      MessagePlugin.warning(t('model.editor.validation.modelNameMax'))
      return
    }

    // For a remote type other than WeKnoraCloud, baseUrl is required
    if (formData.value.source === 'remote' && formData.value.provider !== 'weknoracloud') {
      if (!formData.value.baseUrl || !formData.value.baseUrl.trim()) {
        MessagePlugin.warning(t('model.editor.remoteBaseUrlRequired'))
        return
      }

      // Check the Base URL format
      try {
        new URL(formData.value.baseUrl.trim())
      } catch {
        MessagePlugin.warning(t('model.editor.validation.baseUrlInvalid'))
        return
      }
    }

    if (formData.value.source === 'remote' && formData.value.provider !== 'weknoracloud') {
      // Required extra fields the vendor declares
      for (const field of plainExtraFields.value) {
        if (field.required && !(formData.value.extraConfig?.[field.key] || '').trim()) {
          MessagePlugin.warning(t('model.editor.validation.extraFieldRequired', { name: extraFieldDisplayLabel(field) }))
          return
        }
      }
      if (!isEdit.value && secretExtraField.value?.required && !(formData.value.appSecret || '').trim()) {
        MessagePlugin.warning(t('model.editor.validation.extraFieldRequired', { name: extraFieldDisplayLabel(secretExtraField.value) }))
        return
      }
      if (specCompatError.value) {
        advancedOpen.value = true
        MessagePlugin.warning(`${t('model.editor.advanced.compat.invalid')}: ${specCompatError.value}`)
        return
      }
    }

    // Run form validation
    const validation = await formRef.value?.validate()
    if (validation !== undefined && validation !== true) return

    // Credential removal in edit mode is handled inline by the
    // CredentialResource card (it confirms + DELETEs to /credentials), so
    // the main save flow no longer needs to confirm or handle clear flags.

    // When adding without an id, generate one
    if (!formData.value.id) {
      formData.value.id = generateId()
    }

    await props.saveModel({
      ...formData.value,
      extraConfig: buildExtraConfig(),
      ...(isEdit.value ? {} : { modelType: activeModelType.value }),
    })
    emit('update:visible', false)
    invalidateConnectionTest(false)
    // Reset the draft after a successful save, so the next add-model open is blank
    resetForm()
    lastOpenedModelId.value = null
    // No success message here; the parent handles it
  } catch (error: any) {
    saveError.value = error?.message || t('modelSettings.toasts.saveFailed')
  } finally {
    saving.value = false
  }
}

// Watch model selection changes (download logic and the auto dimension detection hint)
watch(() => formData.value.modelName, async (newValue, oldValue) => {
  if (!newValue) return

  // Download logic
  if (newValue.startsWith('__download__')) {
    // Extract the model name
    const modelName = newValue.replace('__download__', '')

    // Reset the selection (so the __download__ prefix is not shown)
    formData.value.modelName = ''

    // Start the download
    await startDownload(modelName)
    return
  }

  // For an embedding model with an Ollama local model selected, when the model name actually changed
  if (activeModelType.value === 'embedding' &&
    formData.value.source === 'local' &&
    newValue !== oldValue &&
    oldValue !== '') {
    // Hint that the user can detect the dimension
    MessagePlugin.info(t('model.editor.dimensionHint'))
  }
})

// Start downloading the model
const startDownload = async (modelName: string) => {
  downloading.value = true
  downloadProgress.value = 0
  currentDownloadModel.value = modelName

  try {
    // Start the download
    const result = await downloadOllamaModel(modelName)
    const taskId = result.taskId

    MessagePlugin.success(t('model.editor.downloadStarted', { name: modelName }))

    // Poll the download progress
    downloadInterval = setInterval(async () => {
      try {
        const progress = await getDownloadProgress(taskId)
        downloadProgress.value = progress.progress

        if (progress.status === 'completed') {
          // Download complete
          clearInterval(downloadInterval)
          downloadInterval = null
          downloading.value = false

          MessagePlugin.success(t('model.editor.downloadCompleted', { name: modelName }))

          // Refresh the model list
          await loadOllamaModels()

          // Select the newly downloaded model automatically
          formData.value.modelName = modelName

          // Reset state
          downloadProgress.value = 0
          currentDownloadModel.value = ''

        } else if (progress.status === 'failed') {
          // Download failed
          clearInterval(downloadInterval)
          downloadInterval = null
          downloading.value = false
          MessagePlugin.error(progress.message || t('model.editor.downloadFailed', { name: modelName }))
          downloadProgress.value = 0
          currentDownloadModel.value = ''
        }
      } catch (error) {
        console.error('Failed to get download progress:', error)
      }
    }, 1000) // query once per second

  } catch (error: any) {
    downloading.value = false
    downloadProgress.value = 0
    currentDownloadModel.value = ''
    console.error('Download start failed:', error)
    MessagePlugin.error(t('model.editor.downloadStartFailed'))
  }
}

// Clear the timer on component unmount
onUnmounted(() => {
  window.removeEventListener('keydown', handleSaveEscape, true)
  invalidateConnectionTest(false)
  if (resolveTimer) {
    clearTimeout(resolveTimer)
    resolveTimer = null
  }
  resolveRevision++
  if (downloadInterval) {
    clearInterval(downloadInterval)
  }
})

// Watch source changes and clear all state
watch(() => formData.value.source, () => {
  // Reset the check state
  modelChecked.value = false
  modelAvailable.value = false
  remoteChecked.value = false
  remoteAvailable.value = false
  remoteMessage.value = ''
  dimensionChecked.value = false
  dimensionSuccess.value = false
  dimensionMessage.value = ''

  // Clear the download state
  searchKeyword.value = ''
  if (downloadInterval) {
    clearInterval(downloadInterval)
    downloadInterval = null
  }
  downloading.value = false
  downloadProgress.value = 0
  currentDownloadModel.value = ''
})

// Watch model name changes and clear the dimension detection state
watch(() => formData.value.modelName, () => {
  dimensionChecked.value = false
  dimensionSuccess.value = false
  dimensionMessage.value = ''
})

// Cancel (fired by the footer "Cancel" button; clicking the overlay / ESC does not fire it, so the draft is kept)
const handleCancel = () => {
  if (saving.value) return
  resetForm()
  lastOpenedModelId.value = null
  dialogVisible.value = false
}
</script>

<style lang="less" scoped>
.provider-doc-link {
  margin-top: 6px;
}

.provider-doc-link a,
.compat-doc-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: var(--td-text-color-link);
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}

// Empty the native t-form-item container (this component uses a custom .form-item + hand-written label)
:deep(.t-form) {
  .t-form-item {
    display: none;
  }
}

// Form item styles
.form-item {
  // No bottom margin - vertical rhythm is owned by the parent
  // .setting-drawer__section's `gap`. That keeps the spacing inside a section
  // tight and the gap between sections visually distinct.
  margin-bottom: 0;
}

.form-label {
  display: block;
  margin-bottom: 6px;
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.4;

  // TDesign-style required marker: leading asterisk before the label text,
  // matching the rest of the app's <t-form-item required ...> appearance.
  &.required::before {
    content: '*';
    color: var(--td-error-color);
    margin-right: 4px;
    font-weight: 500;
    line-height: 1;
  }
}

.model-type-options,
.source-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

// "Model type" and "Model source" are both single choice, so they use the same button. The model source used to be a gray-track
// segmented control: the #e7e7e7 track is a solid dark gray block in a light form, the white pill does not float above it,
// and the model type right next to it looked different. Without the track, the two groups naturally form one family.
.model-type-option,
.source-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: inherit;
  padding: 6px 12px;
  min-height: 32px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-md);
  line-height: 1.4;
  cursor: pointer;
  transition: border-color var(--app-motion-fast) ease, color var(--app-motion-fast) ease, background var(--app-motion-fast) ease;

  &__icon {
    font-size: var(--app-text-lg);
    flex-shrink: 0;
  }

  &__label {
    white-space: nowrap;
  }

  &:hover:not(.is-active) {
    border-color: var(--td-brand-color-3);
    color: var(--td-text-color-primary);
  }

  // The selected state matches the "Model source" segments below: white background + theme-color border + theme-color text.
  // It used to also lay a 10% theme-color background; that block was the heaviest color on the screen among the five buttons, and
  // the same problem as the full-row green background just removed from the dropdown.
  &.is-active {
    border-color: var(--td-brand-color);
    background: var(--td-bg-color-container);
    color: var(--td-brand-color);
    font-weight: 500;
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 2px;
  }
}



.source-option {
  // The look comes from the shared rule above; this only adds the disabled state (Ollama not ready / rerank not supported).
  &.is-disabled {
    cursor: not-allowed;
    opacity: 0.45;
  }
}

// Input styles: set the font size only on the outer .t-input, to avoid adding borders
// and border-radius again on the inner wrap/inner, which looks like "nested rounded containers"
:deep(.t-input),
:deep(.t-select),
:deep(.t-textarea),
:deep(.t-input-number) {
  width: 100%;
  font-size: var(--app-text-md);
}

// Vendor picker styles - moved to a non-scoped block, because the t-select popup renders under body
// see the end of the file for .provider-option styles

// Checkbox
:deep(.t-checkbox) {
  font-size: var(--app-text-md);

  .t-checkbox__label {
    font-size: var(--app-text-md);
    color: var(--td-text-color-primary);
  }
}

// API key input: a leading lock icon + a trailing clickable "show/hide" eye.
// TDesign shows prefix-icon in gray by default, left unchanged; the eye in the suffix
// uses the placeholder color and switches to the main text color on hover, so it does not steal attention.
.api-key-input {
  :deep(.t-input__prefix) {
    color: var(--td-text-color-placeholder);
  }

  :deep(.t-input__suffix) {
    color: var(--td-text-color-placeholder);
  }

}

// API test area - a light card: a light background + dashed border frame "action + feedback" as one block,
// so users see it as a separate "action unit" rather than another ordinary field.
// (Legacy styles kept: used only when some branch still renders the test block inline; the current RemoteAPI
// test moved up to the SettingDrawer footer-left slot, so the main flow no longer uses this block.)
.api-test-section {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  background: var(--td-bg-color-container-hover);
  border: 1px dashed var(--td-component-stroke);
  border-radius: var(--app-radius-md);

  .test-message {
    font-size: var(--app-text-md);
    line-height: 1.5;
    flex: 1;

    &.success {
      color: var(--td-brand-color-active);
    }

    &.error {
      color: var(--td-error-color);
    }
  }

  :deep(.t-button) {
    min-width: 88px;
    height: 32px;
    font-size: var(--app-text-md);
    border-radius: var(--app-radius-sm);
    flex-shrink: 0;
  }

  .status-icon {
    font-size: var(--app-text-xl);
    flex-shrink: 0;

    &.available {
      color: var(--td-brand-color);
    }

    &.unavailable {
      color: var(--td-error-color);
    }
  }
}

.connection-status {
  font-size: var(--app-text-sm);
  &.success { color: var(--td-brand-color-active); }
  &.error { color: var(--td-error-color); }
}

.connection-hint {
  margin: 0 0 8px;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
  line-height: 1.5;
}

.connection-result {
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid var(--td-error-color-3);
  border-radius: var(--td-radius-default);
  background: var(--td-error-color-1);
  color: var(--td-error-color);
  font-size: var(--app-text-sm);
  text-align: left;

  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  &__details {
    max-height: min(160px, 20vh);
    overflow: auto;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    line-height: 1.5;
    user-select: text;
  }
}

// Status icon variant used inside the footer button.
.status-icon {
  font-size: var(--app-text-xl);
  flex-shrink: 0;

  &.available {
    color: var(--td-brand-color);
  }

  &.unavailable {
    color: var(--td-error-color);
  }
}

// WeKnoraCloud notice
.weknoracloud-hint {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border-radius: var(--app-radius-md);
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  line-height: 1.5;

  // Theming via tokens so the warn/ok states track light/dark switches
  // instead of fighting hardcoded `#fff7ed` etc.
  &--ok {
    background: var(--td-success-color-light);
    border: 1px solid var(--td-success-color-focus);
  }

  &--warn {
    background: var(--td-warning-color-light);
    border: 1px solid var(--td-warning-color-focus);
    border-left: 3px solid var(--td-warning-color);
  }

  .hint-icon {
    font-size: var(--app-text-xl);
    flex-shrink: 0;
    margin-top: 2px;

    &--ok {
      color: var(--td-success-color);
    }

    &--warn {
      color: var(--td-warning-color);
    }

    &--loading {
      color: var(--td-text-color-placeholder);
    }
  }
}

// Ollama model picker styles
.model-option {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 4px 0;

  .downloaded-icon {
    font-size: var(--app-text-base);
    color: var(--td-brand-color);
    flex-shrink: 0;
  }

  .download-icon {
    font-size: var(--app-text-base);
    color: var(--td-brand-color);
    flex-shrink: 0;
  }

  .model-name {
    flex: 1;
    font-size: var(--app-text-md);
    color: var(--td-text-color-primary);
  }

  .model-size {
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
    margin-left: auto;
  }

  &.download {
    .model-name {
      color: var(--td-brand-color);
      font-weight: 500;
    }
  }
}

// Download progress suffix styles
.download-suffix {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 4px;

  .spinning {
    animation: wk-spin 1s linear infinite;
    font-size: var(--app-text-base);
    color: var(--td-brand-color);
  }

  .progress-text {
    font-size: var(--app-text-sm);
    font-weight: 500;
    color: var(--td-brand-color);
  }
}

// Progress bar effect on the picker while downloading
:deep(.t-select.downloading) {
  .t-input {
    position: relative;
    overflow: hidden;

    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      width: var(--progress, 0%);
      background: linear-gradient(90deg, color-mix(in srgb, var(--td-brand-color) 8%, transparent), color-mix(in srgb, var(--td-brand-color) 15%, transparent));
      transition: width var(--app-motion-slow) ease;
      z-index: 0;
      border-radius: 5px 0 0 5px;
    }

    .t-input__inner,
    input {
      position: relative;
      z-index: 1;
      background: transparent !important;
    }
  }
}

.model-select-row {
  display: flex;
  align-items: center;
  gap: 8px;

  .t-select {
    flex: 1;
  }
}

.refresh-btn {
  flex-shrink: 0;
}

// Dimension control styles
.dimension-control {
  display: flex;
  align-items: center;
  gap: 8px;

  :deep(.t-input) {
    flex: 1;
  }
}

.dimension-check-btn {
  flex-shrink: 0;
}

.dimension-hint {
  margin: 8px 0 0 0;
  font-size: var(--app-text-md);
  line-height: 1.5;
  color: var(--td-error-color);

  &.success {
    color: var(--td-brand-color);
  }
}

// Custom HTTP header area
.custom-headers-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.custom-headers-desc {
  margin: 0 0 10px 0;
  font-size: var(--app-text-sm);
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

.custom-headers-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.custom-header-row {
  display: flex;
  align-items: center;
  gap: 8px;

  .custom-header-key {
    flex: 0 0 38%;
  }

  .custom-header-value {
    flex: 1;
  }

  // Ghost icon button - matches the model-card "more" affordance: invisible
  // until hover/focus, then a subtle background pops in. Avoids painting a
  // permanent red splotch next to every header row.
  .custom-header-remove {
    flex-shrink: 0;
    width: 32px;
    height: 32px;
    padding: 0;
    color: var(--td-text-color-placeholder);
    border-radius: var(--app-radius-sm);
    transition: all 0.18s ease;

    &:hover {
      background: var(--td-error-color-light);
      color: var(--td-error-color);
    }
  }
}

.form-desc {
  margin: 4px 0 0 0;
  font-size: var(--app-text-sm);
  line-height: 1.5;
  color: var(--td-text-color-placeholder);

  // Inline with switches/checkboxes - drops the top margin so the label and
  // helper text sit on the same baseline.
  &--inline {
    margin: 0;
  }

  &--recommend {
    color: var(--td-brand-color);
  }

  &--warn {
    color: var(--td-warning-color);
  }

  &--error {
    color: var(--td-error-color);
  }
}

// Selected vendor: icon + name, inside the t-select valueDisplay
.provider-value {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  max-width: 100%;

  &__name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.provider-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  border-radius: 4px;
  object-fit: contain;

  &--placeholder {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: var(--app-text-xs);
    font-weight: 600;
    color: var(--td-text-color-secondary);
    background: var(--td-bg-color-secondarycontainer);
  }
}

// Subdued badges such as in-catalog / reasoning / vision (same tone as the chips on cards)
.catalog-badge {
  display: inline-flex;
  align-items: center;
  padding: 0 6px;
  height: 18px;
  border-radius: 4px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-xs);
  line-height: 18px;
  white-space: nowrap;

  & + & {
    margin-left: 4px;
  }

  &--accent {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
  }
}

// Access diagnostics panel: read-only, light background, so it is not confused with editable fields
.resolved-panel {
  padding: 10px 12px;
  border: 1px dashed var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container-hover);

  &__header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 6px;
  }

  &__icon {
    font-size: var(--app-text-base);
    color: var(--td-text-color-secondary);
  }

  &__title {
    font-size: var(--app-text-sm);
    font-weight: 500;
    color: var(--td-text-color-secondary);
  }

  &__loading {
    font-size: var(--app-text-md);
    color: var(--td-text-color-placeholder);
    animation: spin 1s linear infinite;
  }

  &__grid {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    column-gap: 12px;
    row-gap: 4px;
    margin: 0;
    font-size: var(--app-text-sm);

    dt {
      color: var(--td-text-color-placeholder);
      white-space: nowrap;
      line-height: 18px;
    }

    dd {
      margin: 0;
      min-width: 0;
      color: var(--td-text-color-primary);
      line-height: 18px;
      overflow-wrap: anywhere;
    }

    code {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: var(--app-text-sm);
    }
  }
}

// Advanced collapse: same look as the advanced-toggle in the knowledge base chunking settings
.advanced-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 0;
  margin: 0;
  background: transparent;
  border: none;
  cursor: pointer;
  font-family: inherit;
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-secondary);
  user-select: none;

  &:hover {
    color: var(--td-text-color-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color-focus);
    outline-offset: 2px;
    border-radius: 4px;
  }

  .toggle-arrow {
    font-size: var(--app-text-base);
    transition: transform 0.15s ease;

    &.open {
      transform: rotate(90deg);
    }
  }
}

.compat-textarea {
  :deep(textarea) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: var(--app-text-sm);
  }
}

.vision-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
}

// Ollama unavailable notice styles
.ollama-unavailable-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 12px;
  background: var(--td-error-color-light);
  border: 1px solid var(--td-error-color-focus);
  border-radius: var(--app-radius-md);
  font-size: var(--app-text-md);

  .tip-icon {
    color: var(--td-error-color);
    font-size: var(--app-text-xl);
    flex-shrink: 0;
    margin-right: 2px;

    &.info {
      color: var(--td-brand-color);
    }
  }

  .tip-text {
    color: var(--td-error-color);
    flex: 1;
    line-height: 1.5;
  }

  :deep(.tip-link) {
    color: var(--td-brand-color);
    font-size: var(--app-text-md);
    font-weight: 500;
    padding: 4px 6px 4px 10px !important;
    min-height: auto !important;
    height: auto !important;
    line-height: 1.4 !important;
    text-decoration: none;
    white-space: nowrap;
    display: inline-flex !important;
    align-items: center !important;
    gap: 1px;
    border-radius: var(--app-radius-xs);
    transition: all var(--app-motion-base) ease;

    &:hover {
      background: color-mix(in srgb, var(--td-brand-color) 8%, transparent) !important;
      color: var(--td-brand-color-active) !important;
    }

    &:active {
      background: color-mix(in srgb, var(--td-brand-color) 12%, transparent) !important;
    }

    .t-icon {
      font-size: var(--app-text-base) !important;
      margin: 0 !important;
      line-height: 1 !important;
      display: inline-flex !important;
      align-items: center !important;
    }
  }
}

// Destructive-action checkbox for "Remove this credential". Styled to match
// the pattern used in McpServiceDialog so the two dialogs read identically.
.clear-credential {
  display: inline-flex;
  margin-top: 8px;

  :deep(.t-checkbox__label) {
    color: var(--td-error-color);
    font-size: var(--app-text-md);
  }
}
</style>

<!-- Non-scoped styles: the t-select popup renders under body, where scoped styles cannot reach -->
<style lang="less">
.catalog-model-select-popup {
  padding: 4px;

  .t-select-option {
    height: auto !important;
    padding: 8px 10px;
    border-radius: var(--app-radius-sm);
    margin: 2px 0;
  }
}

.catalog-model-option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  width: 100%;

  &__name {
    font-size: var(--app-text-md);
    color: var(--td-text-color-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__id {
    font-size: var(--app-text-xs);
    color: var(--td-text-color-placeholder);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__badges {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    flex-shrink: 0;

    .catalog-badge {
      display: inline-flex;
      align-items: center;
      padding: 0 6px;
      height: 18px;
      border-radius: 4px;
      background: var(--td-bg-color-secondarycontainer);
      color: var(--td-text-color-secondary);
      font-size: var(--app-text-xs);
      line-height: 18px;
      white-space: nowrap;

      &--accent {
        background: var(--td-brand-color-light);
        color: var(--td-brand-color);
      }
    }
  }
}

.provider-select-popup {
  // Give the container some room, so options do not touch the popup's rounded corners
  padding: 4px;

  // TDesign hard-codes max-height: 300px on the select popup, and the options here are two lines
  // (about 56px), so exactly five show; there are twenty-odd more vendors below, but macOS overlay scrollbars
  // stay invisible until scrolled, so it looks like "there are only these few". Raised to show eight or nine on one screen, with the scrollbar
  // always visible, so it is clear the list continues. &.wk-popover is only there to beat TDesign's
  // own two-level selector.
  &.wk-popover .t-popup__content {
    max-height: min(480px, 60vh);
    // Scrolling itself is restored for every skinned select in
    // assets/theme/tdesign-overrides.less; this only makes the bar visible,
    // because macOS overlay scrollbars stay hidden until something moves and
    // a capped list then looks complete.
    scrollbar-color: var(--td-component-border) transparent;
    scrollbar-width: thin;
  }

  // TDesign by default attaches an overflow tooltip to t-select-option (floating on the right
  // with the full label). Our options are laid out as "main name + secondary description" on two lines and never
  // truncate, so the tooltip is only visual noise -> hide the popup's built-in hint.
  + .t-popup .t-tooltip,
  ~ .t-popup .t-tooltip {
    display: none !important;
  }

  .t-select-option {
    height: auto !important;
    padding: 8px 10px;
    border-radius: var(--app-radius-sm);
    margin: 2px 0;
    outline: none;
    transition: background-color var(--app-motion-fast) ease;

    &:focus,
    &:focus-visible {
      outline: none;
    }


  }

  // The hit state is not defined here: assets/theme/tdesign-overrides.less already replaced TDesign's
  // default full-row --td-brand-color-light with a neutral background + a small theme-color
  // check mark, consistent across the site. Another version here would only make this dropdown disagree with the "Model type / Model source"
  // filter rows. The main name of the selected row still gets a touch of the theme color, as the anchor in the two-line layout.
  .t-select-option.t-is-selected .provider-name {
    color: var(--td-brand-color);
  }

  .provider-option {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-width: 0;

    .provider-icon {
      width: 18px;
      height: 18px;
      flex-shrink: 0;
      border-radius: 4px;
      object-fit: contain;

      &--placeholder {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        font-size: var(--app-text-xs);
        font-weight: 600;
        color: var(--td-text-color-secondary);
        background: var(--td-bg-color-secondarycontainer);
      }
    }

    &__text {
      display: flex;
      flex-direction: column;
      gap: 2px;
      // Allow shrinking so the description can truncate within the popup width (the popup width is pinned by matchTriggerWidth).
      min-width: 0;
      flex: 1;
    }

    .provider-name {
      font-size: var(--app-text-md);
      font-weight: 500;
      color: var(--td-text-color-primary);
      line-height: 20px;
    }

    .provider-desc {
      font-size: var(--app-text-sm);
      color: var(--td-text-color-placeholder);
      line-height: 18px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
  }
}
</style>
