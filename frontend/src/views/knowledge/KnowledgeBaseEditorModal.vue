<template>
  <SettingsModalShell :visible="visible"
    :title="editorMode === 'create' ? $t('knowledgeEditor.titleCreate') : $t('knowledgeEditor.titleEdit')"
    v-model="currentSection" :nav-groups="navGroups" :loading="loading" :z-index="1000"
    nav-guide="kb-editor-sidebar" nav-item-guide-prefix="kb-editor-nav" @close="modalShell.requestClose">
    <div class="content-wrapper">
      <!-- Basic info -->
      <div v-show="currentSection === 'basic'" class="section">
        <div v-if="formData" class="section-content">
          <div class="section-header">
            <h3 class="section-title">{{ $t('knowledgeEditor.basic.title') }}</h3>
            <p class="section-desc">{{ $t('knowledgeEditor.basic.description') }}</p>
          </div>
          <div class="section-body">
            <div v-if="editorMode === 'edit' && activeKbId" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.basic.kbId') }}</label>
              <p class="form-tip">{{ isPostCreateSession ? $t('knowledgeEditor.postCreateHint.followUpDesc') : $t('knowledgeEditor.basic.kbIdDesc') }}</p>
              <div class="kb-id-field">
                <code class="kb-id-value" :title="activeKbId">{{ activeKbId }}</code>
                <t-tooltip :content="$t('common.copy')" placement="top">
                  <t-button theme="default" size="small" variant="text" class="kb-id-copy"
                    @click="copyKbId">
                    <t-icon name="file-copy" />
                  </t-button>
                </t-tooltip>
              </div>
            </div>

            <div class="form-item">
              <label class="form-label required">{{ $t('knowledgeEditor.basic.typeLabel') }}</label>
              <t-radio-group
                v-model="formData.type"
                :disabled="editorMode === 'edit'"
                data-guide="kb-create-type"
              >
                <t-radio-button value="document">{{ $t('knowledgeEditor.basic.typeDocument') }}</t-radio-button>
                <t-radio-button value="faq">{{ $t('knowledgeEditor.basic.typeFAQ') }}</t-radio-button>
              </t-radio-group>
              <p class="form-tip">{{ $t('knowledgeEditor.basic.typeDescription') }}</p>
            </div>

            <!-- Indexing strategy (right after the type selector) -->
            <div v-if="!isFAQ" class="form-item">
              <label class="form-label required">{{ $t('knowledgeEditor.indexing.title') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.indexing.description') }}</p>
              <div class="indexing-checks" :class="{ 'is-locked': isIndexingLocked }"
                data-guide="kb-create-indexing">
                <div
                  class="indexing-check-item"
                  :class="{ 'is-checked': formData.indexingStrategy.vectorEnabled, 'is-disabled': isIndexingLocked }"
                  @click="toggleVectorIndexing"
                >
                  <t-checkbox
                    :checked="formData.indexingStrategy.vectorEnabled"
                    :disabled="isIndexingLocked"
                    class="indexing-check-box"
                  >{{ $t('knowledgeEditor.indexing.searchTitle') }}</t-checkbox>
                  <p class="indexing-check-desc">{{ $t('knowledgeEditor.indexing.searchDesc') }}</p>
                </div>
                <div
                  class="indexing-check-item"
                  :class="{ 'is-checked': formData.indexingStrategy.wikiEnabled, 'is-disabled': isIndexingLocked }"
                  @click="toggleWikiIndexing"
                >
                  <t-checkbox
                    :checked="formData.indexingStrategy.wikiEnabled"
                    :disabled="isIndexingLocked"
                    class="indexing-check-box"
                  >
                    <span class="indexing-check-title">
                      {{ $t('knowledgeEditor.indexing.wikiTitle') }}
                      <span class="indexing-new-badge">NEW</span>
                    </span>
                  </t-checkbox>
                  <p class="indexing-check-desc">{{ $t('knowledgeEditor.indexing.wikiDesc') }}</p>
                </div>
              </div>
              <p v-if="isIndexingLocked" class="locked-tip">
                <t-icon name="lock-on" class="locked-tip-icon" />
                <span>{{ $t('knowledgeEditor.indexing.lockedTip') }}</span>
              </p>
            </div>

            <!-- Wiki extraction granularity (only when Wiki is enabled) -->
            <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.wiki.extractionGranularityLabel') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.wiki.extractionGranularityTip') }}</p>
              <t-radio-group
                :value="resolvedGranularity"
                class="granularity-radio-group"
                @change="handleGranularityChange"
              >
                <t-radio-button value="focused">
                  {{ $t('knowledgeEditor.wiki.granularityFocused') }}
                </t-radio-button>
                <t-radio-button value="standard">
                  {{ $t('knowledgeEditor.wiki.granularityStandard') }}
                </t-radio-button>
                <t-radio-button value="exhaustive">
                  {{ $t('knowledgeEditor.wiki.granularityExhaustive') }}
                </t-radio-button>
              </t-radio-group>
              <p class="form-tip granularity-hint">{{ granularityHint }}</p>
            </div>

            <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.wiki.contentInstructionsLabel') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.wiki.contentInstructionsTip') }}</p>
              <t-textarea
                v-model="formData.wikiConfig.contentInstructions"
                :placeholder="$t('knowledgeEditor.wiki.contentInstructionsPlaceholder')"
                :maxlength="4000"
                :autosize="{ minRows: 3, maxRows: 8 }"
              />
            </div>

            <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.wiki.extractionInstructionsLabel') }}</label>
              <p class="form-tip">{{ $t('knowledgeEditor.wiki.extractionInstructionsTip') }}</p>
              <t-textarea
                v-model="formData.wikiConfig.extractionInstructions"
                :placeholder="$t('knowledgeEditor.wiki.extractionInstructionsPlaceholder')"
                :maxlength="4000"
                :autosize="{ minRows: 3, maxRows: 8 }"
              />
            </div>

            <div class="form-item" data-guide="kb-create-name">
              <label class="form-label required">{{ $t('knowledgeEditor.basic.nameLabel') }}</label>
              <t-input
                v-model="formData.name"
                :placeholder="$t('knowledgeEditor.basic.namePlaceholder')"
                :maxlength="50"
              />
            </div>
            <div class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.basic.descriptionLabel') }}</label>
              <t-textarea
                v-model="formData.description"
                :placeholder="$t('knowledgeEditor.basic.descriptionPlaceholder')"
                :maxlength="200"
                :autosize="{ minRows: 3, maxRows: 6 }"
              />
            </div>

            <!-- AI-generated knowledge base description (edit mode, document knowledge bases only) -->
            <div v-if="editorMode === 'edit' && !isFAQ" class="form-item">
              <label class="form-label">{{ $t('knowledgeEditor.basic.profile.title') }}</label>
              <div class="kb-profile-card" :class="{ 'is-empty': !generatedProfileHasText }">
                <template v-if="generatedProfileHasText">
                  <p v-if="generatedProfile?.gist" class="kb-profile-gist">{{ generatedProfile.gist }}</p>
                  <div v-if="generatedProfile?.topics?.length" class="kb-profile-topics">
                    <t-tag
                      v-for="topic in generatedProfile.topics"
                      :key="topic"
                      size="small"
                      theme="primary"
                      variant="light"
                    >{{ topic }}</t-tag>
                  </div>
                  <div v-if="generatedProfile?.typical_questions?.length" class="kb-profile-questions">
                    <p class="kb-profile-subtitle">{{ $t('knowledgeEditor.basic.profile.questions') }}</p>
                    <ul>
                      <li v-for="q in generatedProfile.typical_questions" :key="q">{{ q }}</li>
                    </ul>
                  </div>
                  <p v-if="generatedProfile?.status === 'failed'" class="kb-profile-error">
                    {{ $t('knowledgeEditor.basic.profile.failed', { error: generatedProfile.error || '' }) }}
                  </p>
                  <div class="kb-profile-footer">
                    <span class="kb-profile-meta">{{ generatedProfileMeta }}</span>
                    <div class="kb-profile-actions">
                      <t-button
                        v-if="generatedProfile?.gist"
                        size="small"
                        variant="text"
                        theme="primary"
                        @click="handleAdoptProfileGist"
                      >
                        <template #icon><t-icon name="enter" /></template>
                        {{ $t('knowledgeEditor.basic.profile.adopt') }}
                      </t-button>
                      <t-button
                        size="small"
                        variant="text"
                        :loading="generatingProfile"
                        @click="handleGenerateProfile"
                      >
                        <template #icon><t-icon name="refresh" /></template>
                        {{ $t('knowledgeEditor.basic.profile.regenerate') }}
                      </t-button>
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div class="kb-profile-empty">
                    <span class="kb-profile-empty-icon"><t-icon name="ai-edit-1" /></span>
                    <div class="kb-profile-empty-text">
                      <p>
                        {{ generatedProfile?.status === 'empty'
                          ? $t('knowledgeEditor.basic.profile.noDocuments')
                          : $t('knowledgeEditor.basic.profile.empty') }}
                      </p>
                      <p v-if="generatedProfile?.status === 'failed'" class="kb-profile-error">
                        {{ $t('knowledgeEditor.basic.profile.failed', { error: generatedProfile.error || '' }) }}
                      </p>
                    </div>
                    <t-button
                      size="small"
                      theme="primary"
                      variant="outline"
                      :loading="generatingProfile"
                      @click="handleGenerateProfile"
                    >
                      <template #icon><t-icon name="ai-edit-1" /></template>
                      {{ $t('knowledgeEditor.basic.profile.generate') }}
                    </t-button>
                  </div>
                </template>
              </div>
              <p class="form-tip">{{ $t('knowledgeEditor.basic.profile.hint') }}</p>
            </div>

            <!-- The Wiki synthesis model lives on the model settings page -->
          </div>
        </div>
      </div>

      <!-- Models -->
      <div v-show="currentSection === 'models'" class="section">
        <KBModelConfig
          ref="modelConfigRef"
          v-if="formData"
          :config="formData.modelConfig"
          :has-files="hasFiles"
          :wiki-enabled="formData.indexingStrategy?.wikiEnabled"
          :rag-enabled="formData.indexingStrategy?.vectorEnabled || formData.indexingStrategy?.keywordEnabled"
          :all-models="allModels"
          @update:config="handleModelConfigUpdate"
        />
      </div>

      <!-- VectorStore binding -->
      <div v-show="currentSection === 'vectorStore'" class="section">
        <KBVectorStoreSettings
          v-if="formData"
          :mode="editorMode"
          :vector-store-id="formData.vectorStoreId"
          :bound-source="formData.vectorStoreInfo?.source"
          :bound-name="formData.vectorStoreInfo?.name"
          :bound-engine-type="formData.vectorStoreInfo?.engineType"
          :bound-status="formData.vectorStoreInfo?.status"
          @update:vector-store-id="handleVectorStoreIdUpdate"
        />
      </div>

      <!-- FAQ -->
      <div v-if="isFAQ && formData" v-show="currentSection === 'faq'" class="section">
        <div class="section-content">
          <div class="section-header">
            <h3 class="section-title">{{ $t('knowledgeEditor.faq.title') }}</h3>
            <p class="section-desc">{{ $t('knowledgeEditor.faq.description') }}</p>
          </div>
          <div class="section-body">
            <div class="form-item">
              <label class="form-label required">{{ $t('knowledgeEditor.faq.indexModeLabel') }}</label>
              <t-radio-group
                v-model="formData.faqConfig.indexMode"
              >
                <t-radio-button value="question_only">{{ $t('knowledgeEditor.faq.modes.questionOnly') }}</t-radio-button>
                <t-radio-button value="question_answer">{{ $t('knowledgeEditor.faq.modes.questionAnswer') }}</t-radio-button>
              </t-radio-group>
              <p class="form-tip">{{ $t('knowledgeEditor.faq.indexModeDescription') }}</p>
            </div>
            <div class="form-item">
              <label class="form-label required">{{ $t('knowledgeEditor.faq.questionIndexModeLabel') }}</label>
              <t-radio-group
                v-model="formData.faqConfig.questionIndexMode"
              >
                <t-radio-button value="combined">{{ $t('knowledgeEditor.faq.modes.combined') }}</t-radio-button>
                <t-radio-button value="separate">{{ $t('knowledgeEditor.faq.modes.separate') }}</t-radio-button>
              </t-radio-group>
              <p class="form-tip">{{ $t('knowledgeEditor.faq.questionIndexModeDescription') }}</p>
            </div>
            <div class="faq-guide">
              <p>{{ $t('knowledgeEditor.faq.entryGuide') }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Parser engine -->
      <div v-if="!isFAQ && formData && currentSection === 'parser'" class="section">
        <KBParserSettings
          :parser-engine-rules="formData.chunkingConfig.parserEngineRules"
          @update:parser-engine-rules="handleParserEngineRulesUpdate"
        />
      </div>

      <!-- Storage engine -->
      <div v-if="!isFAQ && formData && currentSection === 'storage'" class="section">
        <KBStorageSettings
          :storage-backend-id="formData.storageBackendId"
          :storage-provider="formData.storageProvider"
          :has-files="editorMode === 'edit' && hasFiles"
          @update:storage-backend-id="handleStorageBackendUpdate"
          @update:storage-provider="handleStorageProviderUpdate"
        />
      </div>

      <!-- Chunking -->
      <div v-if="!isFAQ" v-show="currentSection === 'chunking'" class="section">
        <KBChunkingSettings
          v-if="formData"
          :config="formData.chunkingConfig"
          @update:config="handleChunkingConfigUpdate"
        />
      </div>

      <!-- Multimodal -->
      <div v-if="!isFAQ" v-show="currentSection === 'multimodal'" class="section">
        <div v-if="formData" class="kb-multimodal-settings">
          <div class="section-header">
            <h2>{{ $t('knowledgeEditor.multimodal.title') }}</h2>
            <p class="section-description">{{ $t('knowledgeEditor.multimodal.description') }}</p>
          </div>

          <div class="settings-group">
            <!-- Multimodal switch -->
            <div class="setting-row" data-guide="kb-create-multimodal-toggle">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.label') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.description') }}</p>
              </div>
              <div class="setting-control">
                <t-switch
                  v-model="formData.multimodalConfig.enabled"
                  @change="handleMultimodalToggle"
                  size="medium"
                />
              </div>
            </div>

            <!-- VLM model selection (when multimodal is enabled) -->
            <div v-if="formData.multimodalConfig.enabled" class="setting-row"
              data-guide="kb-create-multimodal-vllm">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.vllmLabel') }} <span class="required">*</span></label>
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.vllmDescription') }}</p>
              </div>
              <div class="setting-control">
                <ModelSelector
                  model-type="VLLM"
                  :selected-model-id="formData.multimodalConfig.vllmModelId"
                  :all-models="allModels"
                  @update:selected-model-id="handleMultimodalVLLMChange"
                  @add-model="handleAddVLLMModel"
                  :placeholder="$t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')"
                />
              </div>
            </div>

            <div v-if="formData.multimodalConfig.enabled" class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.descriptionLanguageLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.descriptionLanguageDescription') }}</p>
              </div>
              <div class="setting-control">
                <t-select v-model="formData.multimodalConfig.descriptionLanguage" clearable
                  :placeholder="$t('knowledgeEditor.advanced.multimodal.descriptionLanguageAuto')">
                  <t-option value="Chinese" :label="$t('language.zhCN')" />
                  <t-option value="English" :label="$t('language.enUS')" />
                  <t-option value="Korean" :label="$t('language.koKR')" />
                  <t-option value="Russian" :label="$t('language.ruRU')" />
                </t-select>
              </div>
            </div>

            <div v-if="formData.multimodalConfig.enabled" class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.customInstructionsLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.customInstructionsDescription') }}</p>
              </div>
              <div class="setting-control setting-control-full">
                <t-textarea v-model="formData.multimodalConfig.customInstructions"
                  :placeholder="$t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')"
                  :maxlength="4000" :autosize="{ minRows: 3, maxRows: 8 }" />
              </div>
            </div>

            <!-- Image embeddings: off by default; only when enabled are images embedded separately and recalled at retrieval -->
            <div v-if="formData.multimodalConfig.enabled" class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.imageVectorLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageVectorDescription') }}</p>
                <p v-if="formData.imageVectorEnabled && !embeddingTakesImages" class="desc desc-warning">
                  {{ $t('knowledgeEditor.advanced.multimodal.imageVectorModelUnsupported') }}
                </p>
              </div>
              <div class="setting-control">
                <t-switch v-model="formData.imageVectorEnabled" size="medium" />
              </div>
            </div>

            <div v-if="formData.multimodalConfig.enabled" class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsDescription') }}</p>
              </div>
              <t-switch v-model="formData.imageAttrsEnabled" size="medium" />
            </div>

            <div v-if="formData.multimodalConfig.enabled && formData.imageAttrsEnabled"
              class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsSchemaLabel') }}</label>
                <!-- Implementation note (not UI): the panel is driven entirely by the backend attribute registry. Names, descriptions
                     and value meanings come from the schema endpoint; the frontend only overrides translations by name, so adding an
                     attribute is one backend line and the frontend follows without changes here. -->
                <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsSchemaDescription') }}</p>
              </div>
              <div class="image-attr-panel">
                <ul v-if="imageAttrDisplays.length" class="image-attr-list">
                  <li v-for="attr in imageAttrDisplays" :key="attr.name" class="image-attr-row">
                    <div class="image-attr-head">
                      <span class="image-attr-label">{{ attr.label }}</span>
                      <code class="image-attr-name">{{ attr.name }}</code>
                    </div>
                    <p v-if="attr.description" class="image-attr-desc">{{ attr.description }}</p>
                    <ul class="image-attr-value-list">
                      <li v-for="v in attr.values" :key="v.value" class="image-attr-value">
                        <code>{{ v.value }}</code>
                        <span class="image-attr-value-label">{{ v.label }}</span>
                      </li>
                    </ul>
                  </li>
                </ul>
                <div class="image-attr-section">
                  <div class="setting-info">
                    <label>{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsOcrConditions') }}</label>
                    <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsOcrConditionsDesc') }}</p>
                  </div>
                  <ul class="image-attr-condition-list">
                    <li v-for="(cond, i) in imageAttrConditionDisplays" :key="i" class="image-attr-condition">
                      <span class="image-attr-condition-label">{{ cond.label }}</span>
                      <code class="image-attr-condition-raw">{{ cond.raw }}</code>
                    </li>
                  </ul>
                </div>
              </div>

              <!-- Unobserved fallback: a sibling of the switch block above, not nested in the attribute panel.
                   The switch stays in setting-control to share the other switch rows'
                   right-aligned, vertically centered, reserved-right-column layout instead of jumping to the row start -->
              <div class="setting-row">
                <div class="setting-info">
                  <label>{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsOcrOnUnobserved') }}</label>
                  <p class="desc">{{ $t('knowledgeEditor.advanced.multimodal.imageAttrsOcrOnUnobservedDesc') }}</p>
                </div>
                <div class="setting-control">
                  <t-switch v-model="formData.imageActions.ocr.on_unobserved" size="medium" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Audio processing (ASR) -->
      <div v-if="!isFAQ" v-show="currentSection === 'asr'" class="section">
        <div v-if="formData" class="kb-multimodal-settings">
          <div class="section-header">
            <h2>{{ $t('knowledgeEditor.asr.title') }}</h2>
            <p class="section-description">{{ $t('knowledgeEditor.asr.description') }}</p>
          </div>

          <div class="settings-group">
            <!-- ASR switch -->
            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.asr.label') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.asr.desc') }}</p>
              </div>
              <div class="setting-control">
                <t-switch
                  v-model="formData.asrConfig.enabled"
                  size="medium"
                />
              </div>
            </div>

            <!-- ASR model selection -->
            <div v-if="formData.asrConfig.enabled" class="setting-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.asr.modelLabel') }} <span class="required">*</span></label>
                <p class="desc">{{ $t('knowledgeEditor.asr.modelDescription') }}</p>
              </div>
              <div class="setting-control">
                <ModelSelector
                  model-type="ASR"
                  :selected-model-id="formData.asrConfig.modelId"
                  :all-models="allModels"
                  @update:selected-model-id="(val: string) => { if (formData) formData.asrConfig.modelId = val }"
                  @add-model="handleAddASRModel"
                  :placeholder="$t('knowledgeEditor.asr.modelPlaceholder')"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Knowledge graph -->
      <div v-if="!isFAQ && currentSection === 'graph'" class="section">
        <GraphSettings
          v-if="formData"
          :graph-extract="formData.nodeExtractConfig"
          :model-id="formData.modelConfig.llmModelId"
          :all-models="allModels"
          @update:graphExtract="handleNodeExtractUpdate"
        />
      </div>

      <!-- Advanced -->
      <div v-if="!isFAQ" v-show="currentSection === 'advanced'" class="section">
        <KBAdvancedSettings
          ref="advancedSettingsRef"
          v-if="formData"
          :question-generation="formData.questionGenerationConfig"
          :auto-tag="formData.autoTagConfig"
                    :profile-config="formData.profileConfig"
          :rag-enabled="formData.indexingStrategy?.vectorEnabled || formData.indexingStrategy?.keywordEnabled"
          :all-models="allModels"
          :table-metadata-instructions="formData.chunkingConfig.tableMetadataInstructions"
          @update:question-generation="handleQuestionGenerationUpdate"
          @update:auto-tag="(value) => { if (formData) formData.autoTagConfig = value }"
                    @update:profile-config="(value) => { if (formData) formData.profileConfig = value }"
          @update:table-metadata-instructions="(value: string) => { if (formData) formData.chunkingConfig.tableMetadataInstructions = value }"
        />
      </div>

      <!-- Data sources (edit mode only) -->
      <div v-if="editorMode === 'edit' && activeKbId && currentSection === 'datasource'" class="section">
        <DataSourceSettings :kb-id="activeKbId" @count="dsCount = $event" />
      </div>

      <!-- Sharing (edit mode only) -->
      <div v-if="editorMode === 'edit' && activeKbId && currentSection === 'share'" class="section">
        <KBShareSettings :kb-id="activeKbId" :can-share="canShareKB" />
      </div>

      <!-- Activity (edit mode only, Owner/Admin of the KB's workspace) -->
      <div v-if="editorMode === 'edit' && activeKbId && canViewActivity && currentSection === 'activity'" class="section">
        <KnowledgeBaseActivitySettings :kb-id="activeKbId" :active="currentSection === 'activity'" />
      </div>
    </div>

    <template #footer-note>
      <p v-if="isPostCreateSession" class="settings-footer-note">
        <t-icon name="check-circle-filled" class="settings-footer-note__icon" />
        <span>
          <strong>{{ $t('knowledgeEditor.postCreateHint.title') }}</strong>
          {{ $t('knowledgeEditor.postCreateHint.footer') }}
        </span>
      </p>
      <p v-if="isInstantSection" class="settings-footer-note">
        <t-icon name="info-circle-filled" class="settings-footer-note__icon" />
        <span>{{ $t('knowledgeEditor.footer.instantEffect') }}</span>
      </p>
    </template>
    <template #footer>
      <t-button v-if="isInstantSection" theme="default" variant="outline" @click="modalShell.requestClose">
        {{ $t('common.close') }}
      </t-button>
      <template v-else>
        <t-button theme="default" variant="outline" @click="modalShell.requestClose">
          {{ $t('common.cancel') }}
        </t-button>
        <t-button theme="primary" data-guide="kb-create-submit" @click="handleSubmit" :loading="saving"
          :disabled="loading">
          {{ saveButtonLabel }}
        </t-button>
      </template>
    </template>
  </SettingsModalShell>

  <KbCreateContextualGuide :when="visible && editorMode === 'create'" :is-faq="isFAQ"
    :needs-embedding="kbCreateNeedsEmbedding" />
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import KbCreateContextualGuide from '@/components/KbCreateContextualGuide.vue'
import { KB_EDITOR_FOCUS_SECTION_EVENT, markContextualGuideDone } from '@/config/contextualGuides'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useModalShell } from '@/composables/useModalShell'
import SettingsModalShell from '@/components/SettingsModalShell.vue'
import {
  createKnowledgeBase,
  getKnowledgeBaseById,
  listKnowledgeFiles,
  updateKnowledgeBase,
  rebuildKBIndex,
  generateKnowledgeBaseProfile,
  mergeImageActions,
  fetchImageAttrSchema,
  FALLBACK_IMAGE_ATTR_SCHEMA,
  type ImageActionsConfig,
  type ImageAttrSchema,
  type KnowledgeBaseProfile,
} from '@/api/knowledge-base'
import { buildImageProcessingConfig } from '@/utils/imageProcessingConfig'
import { imageAttrDisplay, imageAttrConditionDisplay } from '@/utils/imageAttrDisplay'

// The image-attribute registry, fetched from the backend (single source of
// truth). Falls back to the static registry until the endpoint answers.
const imageAttrSchema = ref<ImageAttrSchema | null>(null)
const displaySchema = computed<ImageAttrSchema>(
  () => imageAttrSchema.value ?? FALLBACK_IMAGE_ATTR_SCHEMA,
)
async function loadImageAttrSchema(kbId: string) {
  try {
    imageAttrSchema.value = await fetchImageAttrSchema(kbId)
  } catch {
    imageAttrSchema.value = null
  }
}
import { updateKBConfig, type KBModelConfigRequest } from '@/api/initialization'
import { useChatResourcesStore } from '@/stores/chatResources'
import { selectInitialModelId } from '@/utils/modelDefaults'
import { copyWithToast } from '@/utils/clipboard'
import { useEditorResourcesStore } from '@/stores/editorResources'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import KBModelConfig from './settings/KBModelConfig.vue'
import KBParserSettings from './settings/KBParserSettings.vue'
import KBStorageSettings from './settings/KBStorageSettings.vue'
import KBChunkingSettings from './settings/KBChunkingSettings.vue'
import KBVectorStoreSettings from './settings/KBVectorStoreSettings.vue'
import KBAdvancedSettings from './settings/KBAdvancedSettings.vue'
import ModelSelector from '@/components/ModelSelector.vue'
import GraphSettings from './settings/GraphSettings.vue'
import KBShareSettings from './settings/KBShareSettings.vue'
import DataSourceSettings from './settings/DataSourceSettings.vue'
import KnowledgeBaseActivitySettings from './settings/KnowledgeBaseActivitySettings.vue'
import { useI18n } from 'vue-i18n'

const uiStore = useUIStore()
const authStore = useAuthStore()
const chatResources = useChatResourcesStore()
const editorResources = useEditorResourcesStore()
const { t, te } = useI18n()

// The attribute panel and the OCR conditions are both rendered from the
// registry, in the operator's language: the registry supplies the wording and
// the i18n overlay translates it, so a new backend attribute shows up here
// without a frontend change.
const imageAttrDisplays = computed(() =>
  displaySchema.value.attributes.map((attr) => imageAttrDisplay(attr, t, te)),
)
// The conditions the knowledge base actually runs with: a list customised through the
// API is shown as is, otherwise mergeImageActions filled in the default.
const imageAttrConditionDisplays = computed(() =>
  (formData.value.imageActions as ImageActionsConfig).ocr.on.map((cond) =>
    imageAttrConditionDisplay(cond, displaySchema.value, t, te),
  ),
)

// Props
const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  kbId?: string
  initialType?: 'document' | 'faq'
}>()

// Emits
const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'success', kbId: string): void
}>()

/** After the first save creates the KB, stay in the modal to configure sharing and the rest */
const savedKbId = ref<string | null>(null)
const editorMode = computed(() => (savedKbId.value ? 'edit' : props.mode))
const activeKbId = computed(() => savedKbId.value ?? props.kbId)
const isPostCreateSession = computed(() => !!savedKbId.value)
const saveButtonLabel = computed(() =>
  editorMode.value === 'create'
    ? t('knowledgeEditor.buttons.create')
    : t('knowledgeEditor.buttons.saveAndClose')
)

const copyKbId = async () => {
  await copyWithToast(activeKbId.value, 'common.copied')
}

const currentSection = ref<string>('basic')

const onKbEditorFocusSection = (event: Event) => {
  const section = (event as CustomEvent<{ section?: string }>).detail?.section
  if (section) {
    currentSection.value = section
  }
}

onMounted(() => {
  window.addEventListener(KB_EDITOR_FOCUS_SECTION_EVENT, onKbEditorFocusSection)
})

onBeforeUnmount(() => {
  window.removeEventListener(KB_EDITOR_FOCUS_SECTION_EVENT, onKbEditorFocusSection)
})
const saving = ref(false)
const loading = ref(false)
const allModels = ref<any[]>([])
const hasFiles = ref(false)
// AI-generated knowledge-base description (edit mode only). Kept outside
// formData because it is never submitted: the backend owns it.
const generatedProfile = ref<KnowledgeBaseProfile | null>(null)
const generatingProfile = ref(false)
const generatedProfileHasText = computed(() => {
  const p = generatedProfile.value
  return !!(p && (p.gist || p.topics?.length || p.typical_questions?.length))
})
const generatedProfileMeta = computed(() => {
  const p = generatedProfile.value
  if (!p || !p.generated_at) return ''
  const when = new Date(p.generated_at)
  const stamp = isNaN(when.getTime()) ? p.generated_at : when.toLocaleString()
  return t('knowledgeEditor.basic.profile.generatedAt', {
    time: stamp,
    count: p.stats?.document_count ?? 0,
  })
})
const initialStorageProvider = ref<string>('')
/** Tenant-wide default from Settings → Storage engine (used when creating a KB). */
const tenantDefaultStorageProvider = ref('local')
const initialIndexingStrategy = ref<any>(null)
const dsCount = ref(0)
// Identifier of the user who created this KB. Empty for older rows
// that predate per-KB ownership tracking; those KBs have no "owner" and
// only tenant Admin+ can mutate their share settings.
const kbCreatorId = ref<string>('')
const kbTenantId = ref<number>(0)

// Backend gate for /knowledge-bases/:id/shares (POST/PUT/DELETE) is
// g.OwnedKBOrAdmin(): only the KB creator or tenant Admin+ may mutate
// shares. Org-admins on a shared KB do NOT pass this guard, so they
// would only see 403s if we let them try. Mirror the matrix here so
// the buttons disappear instead of failing.
const canShareKB = computed(() => {
  if (!activeKbId.value) return false
  const userId = authStore.user?.id || ''
  if (kbCreatorId.value && userId && kbCreatorId.value === userId) return true
  return authStore.hasRole('admin')
})

const isKbOwner = computed(() => {
  const userId = authStore.user?.id || ''
  return Boolean(kbCreatorId.value && userId && kbCreatorId.value === userId)
})

const canViewActivity = computed(() => {
  if (editorMode.value !== 'edit' || !activeKbId.value) return false
  if (Number(kbTenantId.value || 0) !== Number(authStore.currentTenantId || 0)) return false
  return isKbOwner.value || authStore.hasRole('admin')
})
// Whether the user changed any chunking value. Once true, the indexing strategy no longer adjusts chunking defaults.
const chunkingDirty = ref(false)

// Chunking preset for Wiki-only indexing: larger chunks, no overlap, no parent-child chunking.
// It applies only in create mode before the user touches chunking, so an existing KB keeps its settings.
const WIKI_ONLY_CHUNKING_PRESET = {
  chunkSize: 2048,
  chunkOverlap: 0,
  enableParentChild: false,
} as const

// Non-Wiki-only fallback. Mirrors chunker.DefaultChunkSize and
// DefaultChunkOverlap on the backend so a freshly created KB uses
// the same numbers whether the editor sets them or the splitter
// falls back to its package defaults.
const DEFAULT_CHUNKING_PRESET = {
  chunkSize: 512,
  chunkOverlap: 80,
  enableParentChild: true,
} as const

// These sections (sharing / data sources / activity) apply on click, not through the footer Save,
// so the footer shows only Close with a hint, lest users think they must save or that Cancel undoes them.
const INSTANT_SECTIONS = new Set(['datasource', 'share', 'activity'])
const isInstantSection = computed(() => INSTANT_SECTIONS.has(currentSection.value))

const navItems = computed(() => {
  const items: { key: string; icon: string; label: string; badge?: number }[] = [
    { key: 'basic', icon: 'info-circle', label: t('knowledgeEditor.sidebar.basic') },
    { key: 'models', icon: 'control-platform', label: t('knowledgeEditor.sidebar.models') },
    // VectorStore binding section - present in both create and edit
    // modes. Create mode shows a dropdown; edit mode shows the bound
    // store read-only with an immutability hint.
    { key: 'vectorStore', icon: 'data-base', label: t('knowledgeEditor.sidebar.vectorStore') }
  ]
  if (formData.value?.type === 'faq') {
    items.push({ key: 'faq', icon: 'help-circle', label: t('knowledgeEditor.sidebar.faq') })
  } else {
    items.push(
      { key: 'parser', icon: 'file-search', label: t('settings.parserEngine') },
      { key: 'multimodal', icon: 'image', label: t('knowledgeEditor.sidebar.multimodal') },
      { key: 'asr', icon: 'sound', label: t('knowledgeEditor.sidebar.asr') },
      { key: 'storage', icon: 'cloud', label: t('knowledgeEditor.sidebar.storage') },
      { key: 'chunking', icon: 'file-copy', label: t('knowledgeEditor.sidebar.chunking') },
      { key: 'graph', icon: 'chart-bubble', label: t('knowledgeEditor.sidebar.graph') },
      { key: 'advanced', icon: 'setting', label: t('knowledgeEditor.sidebar.advanced') }
    )
    if (editorMode.value === 'edit' && activeKbId.value) {
      items.push({ key: 'datasource', icon: 'cloud-download', label: t('knowledgeEditor.sidebar.datasource'), badge: dsCount.value || undefined })
    }
  }
  if (editorMode.value === 'edit' && activeKbId.value && !authStore.isLiteMode) {
    items.push({ key: 'share', icon: 'share', label: t('knowledgeEditor.sidebar.share') })
  }
  if (canViewActivity.value) {
    items.push({ key: 'activity', icon: 'history', label: t('knowledgeEditor.sidebar.activity') })
  }
  return items
})

// Left nav groups (matches AgentEditorModal)
const navGroups = computed(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]))
  const pickItems = (keys: string[]) =>
    keys.map((key) => itemMap.get(key)).filter(Boolean) as typeof navItems.value
  return [
    {
      key: 'basic',
      label: t('knowledgeEditor.navGroups.basic'),
      items: pickItems(['basic', 'models', 'vectorStore', 'faq']),
    },
    {
      key: 'processing',
      label: t('knowledgeEditor.navGroups.processing'),
      items: pickItems(['parser', 'chunking', 'multimodal', 'asr', 'graph', 'advanced']),
    },
    {
      key: 'data',
      label: t('knowledgeEditor.navGroups.data'),
      items: pickItems(['storage', 'datasource']),
    },
    {
      key: 'integration',
      label: t('knowledgeEditor.navGroups.integration'),
      items: pickItems(['share']),
    },
    {
      key: 'management',
      label: t('knowledgeEditor.navGroups.management'),
      items: pickItems(['activity']),
    },
  ].filter((group) => group.items.length > 0)
})

const modelConfigRef = ref<InstanceType<typeof KBModelConfig>>()
const advancedSettingsRef = ref<InstanceType<typeof KBAdvancedSettings>>()

const formData = ref<any>(null)
// Image embeddings only work when the embedding model handles images; when the model is missing from the list
// (not loaded, shared across workspaces) do not warn, the backend checks the actual model.
const embeddingTakesImages = computed(() => {
  const id = formData.value?.modelConfig.embeddingModelId
  const model = id ? allModels.value.find((m) => m.id === id) : undefined
  return !model || !!model.capabilities?.input?.includes('image')
})
const isFAQ = computed(() => formData.value?.type === 'faq')

const kbCreateNeedsEmbedding = computed(() => {
  if (!formData.value || formData.value.type === 'faq') return false
  const s = formData.value.indexingStrategy
  return Boolean(s?.vectorEnabled || s?.keywordEnabled)
})

const applyDefaultModelsIfEmpty = () => {
  if (!formData.value || editorMode.value !== 'create') return
  const chatModelId = selectInitialModelId(allModels.value, 'KnowledgeQA')
  const embeddingModelId = selectInitialModelId(allModels.value, 'Embedding')
  if (!formData.value.modelConfig.llmModelId && chatModelId) {
    formData.value.modelConfig.llmModelId = chatModelId
  }
  if (!formData.value.modelConfig.embeddingModelId && embeddingModelId) {
    formData.value.modelConfig.embeddingModelId = embeddingModelId
  }
}

watch(
  () => formData.value?.type,
  (newType, oldType) => {
    if (!formData.value) return
    if (newType === 'faq') {
      if (!formData.value.faqConfig) {
        formData.value.faqConfig = { indexMode: 'question_only', questionIndexMode: 'separate' }
      }
      if (!['basic', 'models', 'faq'].includes(currentSection.value)) {
        currentSection.value = 'faq'
      }
    } else if (oldType === 'faq' && currentSection.value === 'faq') {
      currentSection.value = 'basic'
    }
  }
)

const initFormData = (type: 'document' | 'faq' = 'document') => {
  return {
    type,
    name: '',
    description: '',
    faqConfig: {
      indexMode: 'question_only',
      questionIndexMode: 'separate'
    },
    modelConfig: {
      llmModelId: '',
      embeddingModelId: '',
      wikiSynthesisModelId: '',
    },
    chunkingConfig: {
      chunkSize: 512,
      // 80 ≈ 15% of chunkSize - community-recommended sweet spot.
      // Aligned with chunker.DefaultChunkOverlap on the backend.
      chunkOverlap: 80,
      // CJK full stop, exclamation, question mark and semicolon (\u3002 \uff01 \uff1f \uff1b) split Chinese text.
      separators: ['\n\n', '\n', '\u3002', '\uff01', '\uff1f', ';', '\uff1b'],
      parserEngineRules: undefined as any,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      // New KBs default to the adaptive auto-strategy. User can change in the UI.
      strategy: 'auto' as string,
      tokenLimit: 0,
      languages: [] as string[],
      tableMetadataInstructions: ''
    },
    storageBackendId: '' as string,
    storageProvider: '' as string,
    multimodalConfig: {
      enabled: false,
      vllmModelId: '',
      descriptionLanguage: '',
      customInstructions: ''
    },
    // Image attribute observation pipeline: the UI edits the switch and the on_unobserved fallback; the rest
    // (model_id etc.) is sent back from the load-time snapshot so settings written via the API survive.
    // Create mode must also start from the full default actions: imageActions.ocr.on_unobserved
    // is bound to the switch directly, and a missing value crashes rendering when the switch is turned on.
    imageAttrsEnabled: false,
    // Image embeddings default to off: image-capable embedding models are often used for text only.
    imageVectorEnabled: false,
    imageActions: mergeImageActions(),
    imageProcessingConfigSnapshot: null as Record<string, unknown> | null,
    asrConfig: {
      enabled: false,
      modelId: '',
      language: ''
    },
    nodeExtractConfig: {
      enabled: false,
      text: '',
      tags: [] as string[],
      nodes: [] as Array<{
        name: string
        attributes: string[]
      }>,
      relations: [] as Array<{
        node1: string
        node2: string
        type: string
      }>,
      customInstructions: ''
    },
    questionGenerationConfig: {
      enabled: true,
      questionCount: 3,
      customInstructions: ''
    },
    autoTagConfig: {
      enabled: false,
      modelId: '',
      maxTags: 3,
      skipIfTagged: true
    },
    profileConfig: {
      enabled: false,
      modelId: '',
      customInstructions: ''
    },
    wikiConfig: {
      synthesisModelId: '',
      maxPagesPerIngest: 0,
      extractionGranularity: 'standard' as 'focused' | 'standard' | 'exhaustive',
      contentInstructions: '',
      extractionInstructions: '',
    },
    indexingStrategy: {
      vectorEnabled: true,
      keywordEnabled: true,
      wikiEnabled: false,
      graphEnabled: false,
    },
    // Vector-store binding. Empty string means "use the env-configured
    // store"; create mode defaults to that, edit mode loads the
    // existing binding from the KB response below.
    vectorStoreId: '' as string,
    vectorStoreInfo: {
      source: undefined as string | undefined,
      name: undefined as string | undefined,
      engineType: undefined as string | undefined,
      status: undefined as string | undefined,
    },
  }
}

const loadAllModels = async (force = false) => {
  try {
    await chatResources.ensureModels(force)
    allModels.value = chatResources.allModels || []
  } catch (error) {
    console.error('Failed to load model list:', error)
    MessagePlugin.error(t('knowledgeEditor.messages.loadModelsFailed'))
    allModels.value = []
  }
}

let kbEditorLoadGeneration = 0

const isCurrentKBLoad = (generation: number, kbId: string) => (
  generation === kbEditorLoadGeneration
  && props.visible
  && activeKbId.value === kbId
)

// Load the knowledge base (edit mode)
const loadKBData = async (
  kbIdOverride?: string,
  generation = kbEditorLoadGeneration,
) => {
  const kbId = kbIdOverride ?? activeKbId.value
  if (editorMode.value !== 'edit' || !kbId) return
  
  loading.value = true
  try {
    const [kbInfo, filesResult] = await Promise.all([
      getKnowledgeBaseById(kbId),
      listKnowledgeFiles(kbId, { page: 1, page_size: 1 })
    ])

    if (!isCurrentKBLoad(generation, kbId)) return
    
    if (!kbInfo || !kbInfo.data) {
      throw new Error(t('knowledgeEditor.messages.notFound'))
    }

    const kb = kbInfo.data
    hasFiles.value = (filesResult as any)?.total > 0
    generatedProfile.value = (kb as any).generated_profile || null
    kbCreatorId.value = (kb as any).creator_id || ''
    kbTenantId.value = Number((kb as any).tenant_id || 0)

    const kbType = (kb.type as 'document' | 'faq') || 'document'
    formData.value = {
      type: kbType,
      name: kb.name || '',
      description: kb.description || '',
      faqConfig: {
        indexMode: kb.faq_config?.index_mode || 'question_only',
        questionIndexMode: kb.faq_config?.question_index_mode || 'separate'
      },
      modelConfig: {
        llmModelId: kb.summary_model_id || '',
        embeddingModelId: kb.embedding_model_id || '',
        wikiSynthesisModelId: kb.wiki_config?.synthesis_model_id || ''
      },
      chunkingConfig: {
        chunkSize: kb.chunking_config?.chunk_size || 512,
        // Fallback only used when the loaded KB has no chunk_overlap stored.
        // Aligned with chunker.DefaultChunkOverlap on the backend.
        chunkOverlap: kb.chunking_config?.chunk_overlap || 80,
        // CJK full stop, exclamation, question mark and semicolon (\u3002 \uff01 \uff1f \uff1b) split Chinese text.
        separators: kb.chunking_config?.separators || ['\n\n', '\n', '\u3002', '\uff01', '\uff1f', ';', '\uff1b'],
        parserEngineRules: kb.chunking_config?.parser_engine_rules || undefined,
        enableParentChild: kb.chunking_config?.enable_parent_child || false,
        parentChunkSize: kb.chunking_config?.parent_chunk_size || 4096,
        childChunkSize: kb.chunking_config?.child_chunk_size || 384,
        // Existing KBs without strategy field render as empty (= legacy behavior).
        // The user has to actively pick a value to opt in to the new tiers.
        strategy: kb.chunking_config?.strategy || '',
        tokenLimit: kb.chunking_config?.token_limit || 0,
        languages: kb.chunking_config?.languages || [],
        tableMetadataInstructions: kb.chunking_config?.table_metadata_instructions || ''
      },
      storageBackendId: (kb.storage_backend_id || '') as string,
      storageProvider: (kb.storage_provider_config?.provider || kb.storage_config?.provider || 'local') as string,
      multimodalConfig: {
        enabled: !!kb.vlm_config?.enabled,
        vllmModelId: kb.vlm_config?.model_id || '',
        descriptionLanguage: kb.vlm_config?.description_language || '',
        customInstructions: kb.vlm_config?.custom_instructions || ''
      },
      imageAttrsEnabled:
        !!(kb as Record<string, any>).image_processing_config?.image_attrs_enabled,
      imageVectorEnabled:
        !!(kb as Record<string, any>).image_processing_config?.image_vector_enabled,
      imageActions: mergeImageActions(
        (kb as Record<string, any>).image_processing_config?.image_actions,
      ),
      imageProcessingConfigSnapshot:
        (kb as Record<string, any>).image_processing_config || null,
      asrConfig: {
        enabled: !!kb.asr_config?.enabled,
        modelId: kb.asr_config?.model_id || '',
        language: kb.asr_config?.language || ''
      },
      nodeExtractConfig: {
        enabled: kb.extract_config?.enabled || false,
        text: kb.extract_config?.text || '',
        tags: kb.extract_config?.tags || [],
        nodes: (kb.extract_config?.nodes || []).map((node: any) => ({
          name: node.name,
          attributes: node.attributes || []
        })),
        relations: kb.extract_config?.relations || [],
        customInstructions: kb.extract_config?.custom_instructions || ''
      },
      questionGenerationConfig: {
        enabled: kb.question_generation_config?.enabled || false,
        questionCount: kb.question_generation_config?.question_count || 3,
        customInstructions: kb.question_generation_config?.custom_instructions || ''
      },
      autoTagConfig: {
        enabled: kb.auto_tag_config?.enabled || false,
        modelId: kb.auto_tag_config?.model_id || '',
        maxTags: kb.auto_tag_config?.max_tags || 3,
        // Absent on knowledge bases saved before the toggle existed; the
        // backend treats that as "skip", so mirror it here.
        skipIfTagged: kb.auto_tag_config?.skip_if_tagged ?? true
      },
      profileConfig: {
        enabled: kb.profile_config?.enabled || false,
        modelId: kb.profile_config?.model_id || '',
        customInstructions: kb.profile_config?.custom_instructions || ''
      },
      wikiConfig: {
        synthesisModelId: kb.wiki_config?.synthesis_model_id || '',
        maxPagesPerIngest: kb.wiki_config?.max_pages_per_ingest || 0,
        extractionGranularity: (
          kb.wiki_config?.extraction_granularity === 'focused' ||
          kb.wiki_config?.extraction_granularity === 'exhaustive'
            ? kb.wiki_config.extraction_granularity
            : 'standard'
        ) as 'focused' | 'standard' | 'exhaustive',
        contentInstructions: kb.wiki_config?.content_instructions || '',
        extractionInstructions: kb.wiki_config?.extraction_instructions || '',
      },
      indexingStrategy: {
        vectorEnabled: kb.indexing_strategy?.vector_enabled ?? true,
        keywordEnabled: kb.indexing_strategy?.keyword_enabled ?? true,
        wikiEnabled: kb.indexing_strategy?.wiki_enabled ?? false,
        graphEnabled: kb.indexing_strategy?.graph_enabled ?? false,
      },
      // Vector-store binding. vectorStoreId is editor-only state; it
      // is only included in the create request, never the update
      // request, because the binding is immutable after creation.
      // vectorStoreInfo carries the read-only display fields that the
      // edit view renders below; they come straight from the KB
      // response.
      vectorStoreId: '',
      vectorStoreInfo: {
        source: kb.vector_store_source,
        name: kb.vector_store_name,
        engineType: kb.vector_store_engine_type,
        status: kb.vector_store_status,
      },
    }
    // Fetch the backend attribute registry that renders the attribute panel (edit mode has a kbId).
    loadImageAttrSchema(kbId)
    initialStorageProvider.value = formData.value.storageProvider
    initialIndexingStrategy.value = { ...formData.value.indexingStrategy }
  } catch (error) {
    if (!isCurrentKBLoad(generation, kbId)) return
    console.error('Failed to load knowledge base data:', error)
    MessagePlugin.error(t('knowledgeEditor.messages.loadDataFailed'))
    handleClose()
  } finally {
    if (isCurrentKBLoad(generation, kbId)) {
      loading.value = false
      modalShell.markClean()
    }
  }
}

const handleModelConfigUpdate = (config: any) => {
  if (formData.value) {
    formData.value.modelConfig = { ...config }
  }
}

// Granularity selector: read and normalize formData.wikiConfig, falling back to 'standard' for unknown values,
// matching the backend WikiExtractionGranularity.Normalize() contract.
const resolvedGranularity = computed<'focused' | 'standard' | 'exhaustive'>(() => {
  const g = formData.value?.wikiConfig?.extractionGranularity
  if (g === 'focused' || g === 'standard' || g === 'exhaustive') {
    return g
  }
  return 'standard'
})

const granularityHint = computed<string>(() => {
  switch (resolvedGranularity.value) {
    case 'focused':
      return t('knowledgeEditor.wiki.granularityFocusedHint')
    case 'exhaustive':
      return t('knowledgeEditor.wiki.granularityExhaustiveHint')
    default:
      return t('knowledgeEditor.wiki.granularityStandardHint')
  }
})

const handleGranularityChange = (value: string | number | boolean) => {
  if (!formData.value) return
  const next: 'focused' | 'standard' | 'exhaustive' =
    value === 'focused' || value === 'exhaustive'
      ? (value as 'focused' | 'exhaustive')
      : 'standard'
  formData.value.wikiConfig = {
    ...formData.value.wikiConfig,
    extractionGranularity: next,
  }
}

const isIndexingLocked = computed(() => editorMode.value === 'edit' && hasFiles.value)

const toggleVectorIndexing = () => {
  if (!formData.value) return
  if (isIndexingLocked.value) return
  const next = !formData.value.indexingStrategy.vectorEnabled
  formData.value.indexingStrategy.vectorEnabled = next
  formData.value.indexingStrategy.keywordEnabled = next
}

const toggleWikiIndexing = () => {
  if (!formData.value) return
  if (isIndexingLocked.value) return
  formData.value.indexingStrategy.wikiEnabled = !formData.value.indexingStrategy.wikiEnabled
}

const handleChunkingConfigUpdate = (config: any) => {
  if (formData.value) {
    formData.value.chunkingConfig = { ...config }
    // The user touched chunking, so later indexing strategy changes no longer override it
    chunkingDirty.value = true
  }
}

// Wiki-only indexing: Wiki on, vector and keyword retrieval off
const isWikiOnlyStrategy = computed(() => {
  const s = formData.value?.indexingStrategy
  if (!s) return false
  return !!s.wikiEnabled && !s.vectorEnabled && !s.keywordEnabled
})

// Apply or revert the Wiki-only preset with the indexing strategy, only in create mode before the user touches chunking.
// Edit mode never changes the existing backend config.
watch(isWikiOnlyStrategy, (wikiOnly) => {
  if (editorMode.value !== 'create') return
  if (!formData.value) return
  if (chunkingDirty.value) return
  const preset = wikiOnly ? WIKI_ONLY_CHUNKING_PRESET : DEFAULT_CHUNKING_PRESET
  formData.value.chunkingConfig = {
    ...formData.value.chunkingConfig,
    ...preset,
  }
})

const handleParserEngineRulesUpdate = (rules: any[]) => {
  if (formData.value) {
    formData.value.chunkingConfig.parserEngineRules = rules?.length ? rules : undefined
  }
}

const handleMultimodalToggle = () => {
  if (formData.value && !formData.value.multimodalConfig.enabled) {
    formData.value.multimodalConfig.vllmModelId = ''
  }
}

const handleMultimodalVLLMChange = (modelId: string) => {
  if (formData.value) {
    formData.value.multimodalConfig.vllmModelId = modelId
  }
}

const handleAddVLLMModel = () => {
  uiStore.openSettings('models', 'vllm')
}

const handleAddASRModel = () => {
  uiStore.openSettings('models', 'asr')
}

const handleAddWikiModel = () => {
  uiStore.openSettings('models', 'knowledgeqa')
}

const handleStorageProviderUpdate = (value: string) => {
  if (formData.value) {
    formData.value.storageProvider = editorMode.value === 'create'
      ? editorResources.resolveUsableStorageProvider(value || tenantDefaultStorageProvider.value)
      : (value || tenantDefaultStorageProvider.value || 'local')
  }
}

const handleStorageBackendUpdate = (value: string) => {
  if (formData.value) {
    formData.value.storageBackendId = value
  }
}

async function loadTenantDefaultStorageProvider(force = false) {
  try {
    await editorResources.ensureStorageEngine(force)
    tenantDefaultStorageProvider.value = editorResources.resolveUsableStorageProvider(
      editorResources.storageConfig?.default_provider,
    )
  } catch {
    tenantDefaultStorageProvider.value = editorResources.resolveUsableStorageProvider()
  }
}

/** Resolved storage provider for create payload (never silently default to local before tenant config loads). */
function resolvedStorageProvider(): string {
  const explicit = formData.value?.storageProvider?.trim()
  if (editorMode.value === 'create') {
    return editorResources.resolveUsableStorageProvider(explicit || tenantDefaultStorageProvider.value)
  }
  if (explicit) return explicit
  return tenantDefaultStorageProvider.value || 'local'
}

const handleVectorStoreIdUpdate = (id: string) => {
  if (formData.value) {
    // Empty string here means "use system default" (env-store fallback).
    // The create-payload assembly below converts this back to `omit` so
    // the backend stores NULL - keeping the wire shape identical to
    // pre-Phase-2 clients.
    formData.value.vectorStoreId = id || ''
  }
}

const handleQuestionGenerationUpdate = (config: any) => {
  if (formData.value) {
    formData.value.questionGenerationConfig = { ...config }
  }
}

// Regenerate the AI description now (synchronous: one aggregation + one
// small model call). The result replaces the card but never the manual
// description; "adopt" copies the gist over explicitly.
const handleGenerateProfile = async () => {
  const kbId = activeKbId.value
  if (!kbId || generatingProfile.value) return
  generatingProfile.value = true
  try {
    const result: any = await generateKnowledgeBaseProfile(kbId)
    if (!result?.success) {
      throw new Error(result?.message || t('knowledgeEditor.basic.profile.generateFailed'))
    }
    generatedProfile.value = result.data || null
    MessagePlugin.success(t('knowledgeEditor.basic.profile.generated'))
  } catch (error: any) {
    console.error('Generate knowledge base profile failed:', error)
    MessagePlugin.error(error?.message || t('knowledgeEditor.basic.profile.generateFailed'))
  } finally {
    generatingProfile.value = false
  }
}

const handleAdoptProfileGist = () => {
  const gist = generatedProfile.value?.gist
  if (!gist || !formData.value) return
  formData.value.description = gist.slice(0, 200)
  MessagePlugin.success(t('knowledgeEditor.basic.profile.adopted'))
}

const handleNodeExtractUpdate = (config: any) => {
  if (formData.value) {
    formData.value.nodeExtractConfig = { ...config }
  }
}

const validateForm = (): boolean => {
  if (!formData.value) return false

  if (!formData.value.name || !formData.value.name.trim()) {
    MessagePlugin.warning(t('knowledgeEditor.messages.nameRequired'))
    currentSection.value = 'basic'
    return false
  }

  // Document knowledge bases need at least one indexing strategy
  if (formData.value.type !== 'faq') {
    const s = formData.value.indexingStrategy
    if (s && !s.vectorEnabled && !s.keywordEnabled && !s.wikiEnabled && !s.graphEnabled) {
      MessagePlugin.warning(t('knowledgeEditor.indexing.atLeastOne'))
      currentSection.value = 'basic'
      return false
    }
  }

  // The embedding model is required only when retrieval indexing is enabled
  const needsEmbedding = formData.value.indexingStrategy?.vectorEnabled || formData.value.indexingStrategy?.keywordEnabled
  if (needsEmbedding && !formData.value.modelConfig.embeddingModelId) {
    MessagePlugin.warning(t('knowledgeEditor.indexing.embeddingRequired'))
    currentSection.value = 'models'
    return false
  }

  if (!formData.value.modelConfig.llmModelId) {
    MessagePlugin.warning(t('knowledgeEditor.messages.summaryRequired'))
    currentSection.value = 'models'
    return false
  }

  if (formData.value.multimodalConfig.enabled && !formData.value.multimodalConfig.vllmModelId) {
    MessagePlugin.warning(t('knowledgeEditor.messages.multimodalInvalid'))
    currentSection.value = 'multimodal'
    return false
  }

  if (formData.value.type === 'faq' && !formData.value.faqConfig?.indexMode) {
    MessagePlugin.warning(t('knowledgeEditor.messages.indexModeRequired'))
    currentSection.value = 'faq'
    return false
  }

  return true
}

const buildSubmitData = () => {
  if (!formData.value) return null

  const data: any = {
    name: formData.value.name,
    description: formData.value.description,
    type: formData.value.type,
    chunking_config: {
      chunk_size: formData.value.chunkingConfig.chunkSize,
      chunk_overlap: formData.value.chunkingConfig.chunkOverlap,
      separators: formData.value.chunkingConfig.separators,
      enable_parent_child: formData.value.chunkingConfig.enableParentChild,
      parent_chunk_size: formData.value.chunkingConfig.parentChunkSize,
      child_chunk_size: formData.value.chunkingConfig.childChunkSize,
      // Adaptive chunking fields are always sent (empty/zero values
      // included) so the user can clear them - backend uses pointer DTOs
      // to distinguish "not in payload" from "explicitly empty".
      strategy: formData.value.chunkingConfig.strategy ?? '',
      token_limit: formData.value.chunkingConfig.tokenLimit ?? 0,
      languages: formData.value.chunkingConfig.languages ?? [],
      table_metadata_instructions: formData.value.chunkingConfig.tableMetadataInstructions || '',
      ...(formData.value.chunkingConfig.parserEngineRules?.length
        ? { parser_engine_rules: formData.value.chunkingConfig.parserEngineRules }
        : {})
    },
    embedding_model_id: formData.value.modelConfig.embeddingModelId,
    summary_model_id: formData.value.modelConfig.llmModelId
  }

  // Vector-store binding. Only attach the field when the user actively
  // selected a non-default store. The server treats an empty string as
  // NULL, but keeping the field absent on the wire matches what a
  // client that doesn't know about this binding would send - which
  // makes A/B response diffs easier to read.
  if (formData.value.vectorStoreId) {
    data.vector_store_id = formData.value.vectorStoreId
  }

  data.vlm_config = {
    enabled: formData.value.multimodalConfig.enabled,
    model_id: formData.value.multimodalConfig.enabled
      ? (formData.value.multimodalConfig.vllmModelId || '')
      : '',
    description_language: formData.value.multimodalConfig.descriptionLanguage || '',
    custom_instructions: formData.value.multimodalConfig.customInstructions || ''
  }

  // Image attribute observation: the backend replaces it as a whole (field absent = unchanged).
  // The UI edits only the switch and on_unobserved; an `on` customized via the API is kept from the snapshot,
  // otherwise the registry default applies. Merge rules: buildImageProcessingConfig.
  {
    const built = buildImageProcessingConfig(formData.value.imageProcessingConfigSnapshot, {
      imageAttrsEnabled: formData.value.imageAttrsEnabled,
      imageVectorEnabled: formData.value.imageVectorEnabled,
      onUnobserved: formData.value.imageActions.ocr.on_unobserved,
      defaultOn: displaySchema.value.default_actions.ocr.on,
    })
    if (built) {
      data.image_processing_config = built
    }
  }

  data.asr_config = {
    enabled: formData.value.asrConfig?.enabled || false,
    model_id: formData.value.asrConfig?.enabled
      ? (formData.value.asrConfig?.modelId || '')
      : '',
    language: formData.value.asrConfig?.language || ''
  }

  // storage_backend_id is authoritative. Keep provider projection for old clients
  // and for rolling upgrades where a node has not picked up the new schema yet.
  if (formData.value.storageBackendId) {
    data.storage_backend_id = formData.value.storageBackendId
  }
  const storageProvider = resolvedStorageProvider()
  data.storage_provider_config = {
    provider: storageProvider
  }
  data.storage_config = {
    provider: storageProvider
  }

  // Knowledge graph config is synced via indexingStrategy.graphEnabled
  // extract_config is sent below along with indexing_strategy

  if (formData.value.questionGenerationConfig?.enabled) {
    data.question_generation_config = {
      enabled: true,
      question_count: formData.value.questionGenerationConfig.questionCount || 3,
      custom_instructions: formData.value.questionGenerationConfig.customInstructions || ''
    }
  } else {
    data.question_generation_config = {
      enabled: false,
      question_count: 3,
      custom_instructions: formData.value.questionGenerationConfig?.customInstructions || ''
    }
  }

  data.auto_tag_config = {
    enabled: formData.value.autoTagConfig?.enabled || false,
    model_id: formData.value.autoTagConfig?.modelId || '',
    max_tags: formData.value.autoTagConfig?.maxTags || 3,
    skip_if_tagged: formData.value.autoTagConfig?.skipIfTagged ?? true
  }

  data.profile_config = {
    enabled: formData.value.profileConfig?.enabled || false,
    model_id: formData.value.profileConfig?.modelId || '',
    custom_instructions: formData.value.profileConfig?.customInstructions || ''
  }

  if (formData.value.type === 'faq') {
    data.faq_config = {
      index_mode: formData.value.faqConfig?.indexMode || 'question_only',
      question_index_mode: formData.value.faqConfig?.questionIndexMode || 'separate'
    }
  }

  // Wiki enablement is carried solely by indexing_strategy.wiki_enabled.
  // wiki_config only holds wiki-specific tunables.
  if (formData.value.type !== 'faq') {
    data.wiki_config = {
      synthesis_model_id: formData.value.modelConfig?.wikiSynthesisModelId || '',
      max_pages_per_ingest: formData.value.wikiConfig?.maxPagesPerIngest || 0,
      extraction_granularity: formData.value.wikiConfig?.extractionGranularity || 'standard',
      content_instructions: formData.value.wikiConfig?.contentInstructions || '',
      extraction_instructions: formData.value.wikiConfig?.extractionInstructions || '',
    }
  }

  // Send indexing strategy
  if (formData.value.type !== 'faq') {
    data.indexing_strategy = {
      vector_enabled: formData.value.indexingStrategy?.vectorEnabled ?? true,
      keyword_enabled: formData.value.indexingStrategy?.keywordEnabled ?? true,
      wiki_enabled: formData.value.indexingStrategy?.wikiEnabled ?? false,
      graph_enabled: formData.value.indexingStrategy?.graphEnabled ?? false,
    }
  }

  // Always persist extract_config so the toggle state from GraphSettings is saved,
  // regardless of whether the graph indexing strategy is currently enabled.
  if (formData.value.nodeExtractConfig) {
    data.extract_config = {
      enabled: !!formData.value.nodeExtractConfig.enabled,
      text: formData.value.nodeExtractConfig.text || '',
      tags: formData.value.nodeExtractConfig.tags || [],
      nodes: formData.value.nodeExtractConfig.nodes || [],
      relations: formData.value.nodeExtractConfig.relations || [],
      custom_instructions: formData.value.nodeExtractConfig.customInstructions || ''
    }
  }

  return data
}

const handleSubmit = async () => {
  if (!validateForm()) {
    return
  }

  // In edit mode, confirm when files exist and the storage engine changed
  if (
    editorMode.value === 'edit' &&
    hasFiles.value &&
    formData.value &&
    initialStorageProvider.value &&
    formData.value.storageProvider !== initialStorageProvider.value
  ) {
    const dialog = DialogPlugin.confirm({
      header: t('common.confirm'),
      body: t('knowledgeEditor.messages.storageChangeConfirm'),
      confirmBtn: t('common.confirm'),
      cancelBtn: t('common.cancel'),
      onConfirm: () => {
        dialog.destroy()
        doSubmit()
      },
      onCancel: () => {
        dialog.destroy()
      },
    })
    return
  }

  doSubmit()
}

const doSubmit = async () => {
  saving.value = true
  try {
    const data = buildSubmitData()
    if (!data) {
      throw new Error(t('knowledgeEditor.messages.buildDataFailed'))
    }

    if (editorMode.value === 'create') {
      // Create mode: create the knowledge base with all settings in one call
      const result: any = await createKnowledgeBase(data)
      if (!result.success || !result.data?.id) {
        throw new Error(result.message || t('knowledgeEditor.messages.createFailed'))
      }
      const createdKbId = result.data.id as string
      savedKbId.value = createdKbId
      currentSection.value = 'basic'
      await loadKBData(createdKbId)
      MessagePlugin.success(t('knowledgeEditor.messages.createSuccess'))
      markContextualGuideDone('kbCreate')
      emit('success', createdKbId)
    } else {
      // Edit mode: update basic info and config separately
      const kbId = activeKbId.value
      if (!kbId) {
        throw new Error(t('knowledgeEditor.messages.missingId'))
      }

      // 1. Basic info (name, description) and FAQ/Wiki config
      const updateConfig: any = {}
      if (formData.value.type === 'faq' && formData.value.faqConfig) {
        updateConfig.faq_config = {
          index_mode: formData.value.faqConfig.indexMode || 'question_only',
          question_index_mode: formData.value.faqConfig.questionIndexMode || 'separate'
        }
      }
      if (formData.value.wikiConfig && formData.value.type !== 'faq') {
        updateConfig.wiki_config = {
          synthesis_model_id: formData.value.modelConfig?.wikiSynthesisModelId || '',
          max_pages_per_ingest: formData.value.wikiConfig.maxPagesPerIngest || 0,
          extraction_granularity: formData.value.wikiConfig.extractionGranularity || 'standard',
          content_instructions: formData.value.wikiConfig.contentInstructions || '',
          extraction_instructions: formData.value.wikiConfig.extractionInstructions || '',
        }
      }
      if (formData.value.type !== 'faq') {
        updateConfig.auto_tag_config = data.auto_tag_config
        updateConfig.profile_config = data.profile_config
        updateConfig.indexing_strategy = {
          vector_enabled: formData.value.indexingStrategy?.vectorEnabled ?? true,
          keyword_enabled: formData.value.indexingStrategy?.keywordEnabled ?? true,
          wiki_enabled: formData.value.indexingStrategy?.wikiEnabled ?? false,
          graph_enabled: formData.value.indexingStrategy?.graphEnabled ?? false,
        }
      }
      // Image classification config: buildSubmitData emits it only when it differs from the snapshot, and sending it
      // replaces it whole (backend: field absent = unchanged). Edit mode used to omit it, so image
      // classification edits in the KB editor were silently dropped after creation.
      if (data.image_processing_config) {
        updateConfig.image_processing_config = data.image_processing_config
      }
      await updateKnowledgeBase(kbId, {
        name: data.name,
        description: data.description,
        config: updateConfig
      })

      // 2. Full config (models, chunking, multimodal, storage engine, knowledge graph, ...)
      const config: KBModelConfigRequest = {
        llmModelId: data.summary_model_id,
        embeddingModelId: data.embedding_model_id,
        vlm_config: data.vlm_config,
        asr_config: data.asr_config,
        documentSplitting: {
          chunkSize: data.chunking_config.chunk_size,
          chunkOverlap: data.chunking_config.chunk_overlap,
          separators: data.chunking_config.separators,
          parserEngineRules: data.chunking_config.parser_engine_rules || undefined,
          enableParentChild: data.chunking_config.enable_parent_child || false,
          parentChunkSize: data.chunking_config.parent_chunk_size || 4096,
          childChunkSize: data.chunking_config.child_chunk_size || 384,
          // Always send strategy / tokenLimit / languages - backend treats
          // empty/0/[] as a valid clear, so we must include them in the
          // payload to let users reset back to defaults.
          strategy: formData.value?.chunkingConfig.strategy ?? '',
          tokenLimit: formData.value?.chunkingConfig.tokenLimit ?? 0,
          languages: formData.value?.chunkingConfig.languages ?? [],
          tableMetadataInstructions: formData.value?.chunkingConfig.tableMetadataInstructions ?? ''
        },
        multimodal: {
          enabled: !!data.vlm_config?.enabled
        },
        storageBackendId: formData.value?.storageBackendId || '',
        storageProvider: data.storage_provider_config?.provider || data.storage_config?.provider || 'local',
        nodeExtract: {
          enabled: data.extract_config?.enabled || false,
          text: data.extract_config?.text || '',
          tags: data.extract_config?.tags || [],
          nodes: data.extract_config?.nodes || [],
          relations: data.extract_config?.relations || [],
          customInstructions: data.extract_config?.custom_instructions || ''
        },
        questionGeneration: {
          enabled: data.question_generation_config?.enabled || false,
          questionCount: data.question_generation_config?.question_count || 3,
          customInstructions: data.question_generation_config?.custom_instructions || ''
        }
      }

      await updateKBConfig(kbId, config)
      MessagePlugin.success(t('knowledgeEditor.messages.updateSuccess'))

      // Check if indexing strategy changed and offer rebuild
      if (hasFiles.value && initialIndexingStrategy.value && formData.value) {
        const curr = formData.value.indexingStrategy
        const prev = initialIndexingStrategy.value
        const strategyChanged = (
          curr.vectorEnabled !== prev.vectorEnabled ||
          curr.keywordEnabled !== prev.keywordEnabled ||
          curr.wikiEnabled !== prev.wikiEnabled ||
          curr.graphEnabled !== prev.graphEnabled
        )
        if (strategyChanged) {
          const dialog = DialogPlugin.confirm({
            header: t('knowledgeEditor.indexing.rebuildConfirmTitle'),
            body: t('knowledgeEditor.indexing.rebuildConfirmBody', { count: '...' }),
            confirmBtn: t('common.confirm'),
            cancelBtn: t('common.cancel'),
            onConfirm: async () => {
              dialog.destroy()
              try {
                const result: any = await rebuildKBIndex(kbId)
                const count = result?.data?.document_count ?? 0
                MessagePlugin.success(t('knowledgeEditor.indexing.rebuildSuccess', { count }))
              } catch (e) {
                console.error('Rebuild index failed:', e)
              }
            },
            onCancel: () => {
              dialog.destroy()
              MessagePlugin.info(t('knowledgeEditor.indexing.rebuildSkip'))
            },
          })
        }
      }

      emit('success', kbId)
      handleClose()
    }
  } catch (error: any) {
    console.error('Knowledge base operation failed:', error)
    // Vector-store-binding error codes from the server. Both indicate
    // the selected store cannot be used: 2200 is "the binding itself
    // is invalid" (e.g. unknown id, foreign tenant), 2201 is "the
    // store is currently unreachable". For either, swap in a localized
    // message and jump the user back to the Vector Store section so
    // they can pick a different store or fall back to the system
    // default.
    const code = error?.error?.code ?? error?.code
    if (code === 2200) {
      MessagePlugin.error(t('knowledgeEditor.errors.vectorStoreBindingInvalid'))
      currentSection.value = 'vectorStore'
    } else if (code === 2201) {
      MessagePlugin.error(t('knowledgeEditor.errors.vectorStoreUnavailable'))
      currentSection.value = 'vectorStore'
    } else {
      MessagePlugin.error(error?.message || t('common.operationFailed'))
    }
  } finally {
    saving.value = false
  }
}

const resetState = () => {
  savedKbId.value = null
  currentSection.value = 'basic'
  formData.value = null
  hasFiles.value = false
  initialStorageProvider.value = ''
  tenantDefaultStorageProvider.value = 'local'
  initialIndexingStrategy.value = null
  saving.value = false
  loading.value = false
  chunkingDirty.value = false
  kbCreatorId.value = ''
  kbTenantId.value = 0
}

const handleClose = () => {
  emit('update:visible', false)
  setTimeout(() => {
    if (props.visible) return
    resetState()
  }, 300)
}

const modalShell = useModalShell({
  visible: () => props.visible,
  close: handleClose,
  snapshot: () => formData.value,
})

watch(() => props.visible, async (newVal) => {
  const generation = ++kbEditorLoadGeneration
  if (newVal) {
    resetState()
    modalShell.markClean()
    loading.value = true
    const targetKbId = props.kbId
    
    if (uiStore.kbEditorInitialSection) {
      currentSection.value = uiStore.kbEditorInitialSection
    }
    
    // Load models and the workspace default storage engine (create mode uses it even if the Storage engine tab is never opened)
    await Promise.all([loadAllModels(), loadTenantDefaultStorageProvider()])

    if (generation !== kbEditorLoadGeneration || !props.visible) return
    
    if (props.mode === 'edit' && targetKbId) {
      await loadKBData(targetKbId, generation)
    } else {
      // Create mode: empty form prefilled with the workspace default storage engine
      formData.value = initFormData(props.initialType || 'document')
      formData.value.storageProvider = tenantDefaultStorageProvider.value
      hasFiles.value = false
      applyDefaultModelsIfEmpty()
      loading.value = false
      modalShell.markClean()
    }
  } else {
    // On close, reset after the animation ends
    setTimeout(() => {
      if (props.visible) return
      resetState()
      currentSection.value = 'basic'
    }, 300)
  }
})

// Refresh the model list after the global settings modal closes
watch(
  () => uiStore.showSettingsModal,
  async (visible, previous) => {
    if (!visible && previous && props.visible) {
      await loadAllModels(true)
    }
  }
)

watch(() => chatResources.allModels, (list) => {
  if (props.visible) {
    allModels.value = list || []
  }
})
</script>

<style scoped lang="less">
// Image attribute panel (observable attributes / OCR triggers / unobserved fallback).
// All text comes from the backend attribute registry (display name + meaning of each value); the frontend only overrides translations,
// so new attributes need no change here. It sits in its settings section without its own background.
.image-attr-panel {
  margin-top: 4px;
}
.image-attr-list,
.image-attr-value-list,
.image-attr-condition-list {
  margin: 0;
  padding: 0;
  list-style: none;
}
.image-attr-row + .image-attr-row {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--td-component-border);
}
.image-attr-head {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
}
.image-attr-label {
  font-weight: 600;
  color: var(--td-text-color-primary);
}
// The raw attribute name is smaller and in parentheses, to match the processing trace without distracting
.image-attr-name {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);

  &::before {
    content: '(';
  }

  &::after {
    content: ')';
  }
}
.image-attr-desc {
  margin: 2px 0 0;
  font-size: var(--app-text-sm);
  line-height: 20px;
  color: var(--td-text-color-secondary);
}
// One line per value: value (monospace, left column) + colon + explanation
.image-attr-value-list {
  display: grid;
  grid-template-columns: max-content 1fr;
  column-gap: 2px;
  row-gap: 2px;
  margin-top: 6px;
  font-size: var(--app-text-sm);
}
// display: contents puts value and explanation in the two columns above, so values align in a column
.image-attr-value {
  display: contents;

  code {
    font-family: ui-monospace, Menlo, Consolas, monospace;
    color: var(--td-text-color-primary);

    // The colon follows the value with no gap, as in "none: no text"
    &::after {
      content: ':';
    }
  }
}
.image-attr-value-label {
  color: var(--td-text-color-secondary);
}
.image-attr-section {
  margin-top: 14px;
}
// An OCR condition takes two lines: plain words on top, the raw property = value below
.image-attr-condition-list {
  margin-top: 6px;
  font-size: var(--app-text-sm);
}
.image-attr-condition + .image-attr-condition {
  margin-top: 6px;
}
.image-attr-condition-label {
  display: block;
  color: var(--td-text-color-primary);
}
.image-attr-condition-raw {
  display: block;
  margin-top: 1px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);
}

/* Left nav: matches AgentEditorModal */
.content-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
}

.section {
  margin-bottom: 32px;

  &:last-child {
    margin-bottom: 0;
  }
}

.section-content {
  .section-header {
    margin-bottom: 16px;
  }

  .section-title {
    margin: 0 0 6px 0;
    font-family: var(--app-font-family);
    font-size: var(--app-text-3xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .section-desc {
    margin: 0;
    font-family: var(--app-font-family);
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
    line-height: 22px;
  }

  .section-body {
    background: var(--td-bg-color-container);
  }
}

.form-item {
  margin-bottom: 16px;

  &:last-child {
    margin-bottom: 0;
  }
}

.form-label {
  display: block;
  margin-bottom: 8px;
  font-family: var(--app-font-family);
  font-size: var(--app-text-lg);
  font-weight: 500;
  color: var(--td-text-color-primary);

  &.required::after {
    content: '*';
    color: var(--td-error-color);
    margin-left: 4px;
  }
}

.form-tip {
  margin-top: 6px;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
}

.kb-id-field {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  max-width: 480px;
  margin-top: 8px;
  padding: 6px 8px 6px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);

  .kb-id-value {
    flex: 1;
    min-width: 0;
    margin: 0;
    padding: 0;
    background: none;
    border: none;
    font-family: var(--app-font-family-mono);
    font-size: var(--app-text-md);
    line-height: 1.5;
    color: var(--td-text-color-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kb-id-copy {
    flex-shrink: 0;
    color: var(--td-text-color-secondary);

    &:hover {
      color: var(--td-brand-color);
    }
  }
}

.granularity-radio-group {
  margin-top: 4px;
}

.granularity-hint {
  margin-top: 8px;
  line-height: 1.6;
  color: var(--td-text-color-secondary);
  white-space: normal;
  word-break: break-word;
}

.indexing-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
  margin-top: 10px;
}

.indexing-check-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);
  cursor: pointer;
  user-select: none;
  transition: border-color var(--app-motion-base) ease, background var(--app-motion-base) ease;

  &:hover {
    border-color: var(--td-brand-color);
  }

  &.is-checked {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }

  &.is-disabled {
    cursor: not-allowed;
    background: var(--td-bg-color-secondarycontainer);

    &:hover {
      border-color: var(--td-component-stroke);
    }

    &.is-checked {
      border-color: var(--td-brand-color-focus);
      background: var(--td-brand-color-light);
    }

    &:not(.is-checked) {
      opacity: 0.6;
    }
  }

  :deep(.t-checkbox__label) {
    font-weight: 500;
    color: var(--td-text-color-primary);
  }
}

.locked-tip {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 8px 0 0;
  font-size: var(--app-text-sm);
  line-height: 18px;
  color: var(--td-text-color-secondary);

  .locked-tip-icon {
    flex-shrink: 0;
    margin-top: 2px;
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
  }
}

// Disable the inner checkbox's own clicks; the card handles them
.indexing-check-box {
  pointer-events: none;
}

.indexing-check-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.indexing-new-badge {
  display: inline-flex;
  align-items: center;
  padding: 0 6px;
  height: 16px;
  border-radius: 3px;
  font-size: var(--app-text-2xs);
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0.4px;
  color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.indexing-check-desc {
  margin: 0;
  padding-left: 24px;
  font-size: var(--app-text-sm);
  line-height: 18px;
  color: var(--td-text-color-placeholder);
}

.faq-guide {
  margin-top: 20px;
  padding: 12px 16px;
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-md);
  line-height: 20px;
}

// Inline multimodal styles (consistent with KBStorageSettings/KBAdvancedSettings)
.kb-multimodal-settings {
  width: 100%;

  .section-header {
    margin-bottom: 20px;

    h2 {
      font-size: var(--app-text-3xl);
      font-weight: 600;
      color: var(--td-text-color-primary);
      margin: 0 0 6px 0;
    }

    .section-description {
      font-size: var(--app-text-base);
      color: var(--td-text-color-secondary);
      margin: 0;
      line-height: 1.5;
    }
  }

  .settings-group {
    display: flex;
    flex-direction: column;
  }

  .setting-row {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: 16px 0;
    border-bottom: 1px solid var(--td-component-stroke);

    &:last-child {
      border-bottom: none;
    }
  }

  // Vertical row (category OCR policy table): the description spans the row, the table wraps below
  .setting-row-vertical {
    flex-direction: column;
    align-items: stretch;

    > .setting-info {
      max-width: 100%;
      padding-right: 0;
    }
  }

  .setting-info {
    flex: 1;
    max-width: 65%;
    padding-right: 24px;

    label {
      font-size: var(--app-text-lg);
      font-weight: 500;
      color: var(--td-text-color-primary);
      display: block;
      margin-bottom: 4px;
    }

    .desc {
      font-size: var(--app-text-md);
      color: var(--td-text-color-secondary);
      margin: 0;
      line-height: 1.5;
    }

    .desc-warning {
      margin-top: 4px;
      color: var(--td-warning-color);
    }
  }

  .setting-control {
    flex-shrink: 0;
    min-width: 280px;
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .required {
    color: var(--td-error-color);
    margin-left: 2px;
    font-weight: 500;
  }
}

.kb-profile-card {
  padding: 12px 14px 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);

  &.is-empty {
    padding: 12px 14px;
    border-style: dashed;
    background: var(--td-bg-color-secondarycontainer);
  }

  .kb-profile-gist {
    margin: 0 0 10px;
    font-size: var(--app-text-base);
    line-height: 1.6;
    color: var(--td-text-color-primary);
  }

  .kb-profile-topics {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 10px;
  }

  .kb-profile-subtitle {
    margin: 0 0 4px;
    font-size: var(--app-text-sm);
    font-weight: 500;
    color: var(--td-text-color-secondary);
  }

  .kb-profile-questions ul {
    margin: 0 0 4px;
    padding-left: 18px;
    font-size: var(--app-text-md);
    line-height: 1.7;
    color: var(--td-text-color-secondary);
  }

  .kb-profile-error {
    margin: 4px 0 0;
    font-size: var(--app-text-sm);
    color: var(--td-error-color);
  }

  .kb-profile-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--td-component-stroke);
  }

  .kb-profile-meta {
    min-width: 0;
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kb-profile-actions {
    display: flex;
    flex-shrink: 0;
    gap: 4px;
    align-items: center;
  }

  .kb-profile-empty {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .kb-profile-empty-icon {
    display: inline-flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: var(--app-radius-md);
    font-size: var(--app-text-2xl);
    color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }

  .kb-profile-empty-text {
    flex: 1;
    min-width: 0;

    p {
      margin: 0;
      font-size: var(--app-text-md);
      line-height: 1.5;
      color: var(--td-text-color-secondary);
    }
  }
}
</style>
