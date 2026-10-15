<template>
  <SettingsModalShell :visible="visible"
    :title="editorMode === 'create' ? $t('agent.editor.createTitle') : $t('agent.editor.editTitle')"
    v-model="currentSection" :nav-groups="navGroups" :loading="editorInitializing" :z-index="1000"
    nav-guide="agent-editor-sidebar" nav-item-guide-prefix="agent-editor-nav" @close="modalShell.requestClose">
    <div ref="contentWrapperRef" class="content-wrapper" :class="{ 'content-wrapper--prompts': currentSection === 'prompts' }">
      <!-- Basic settings -->
      <div v-show="currentSection === 'basic'" class="section">
        <div class="section-header">
          <div class="section-header-title">
            <h2>{{ $t('agent.editor.basicInfo') }}</h2>
            <t-tooltip v-if="isBuiltinAgent" :content="$t('agentEditor.builtinHint')" placement="top">
              <span class="builtin-agent-hint" tabindex="0" role="img"
                :aria-label="$t('agentEditor.builtinHint')">
                <t-icon name="info-circle" />
              </span>
            </t-tooltip>
          </div>
          <p class="section-description">{{ $t('agent.editor.basicInfoDesc') }}</p>
        </div>

        <div class="settings-group">
          <!-- Agent ID (for API integration) -->
          <div v-if="editorMode === 'edit' && editorAgent?.id" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.agentId') }}</label>
              <p class="desc">{{ $t('agent.editor.agentIdDesc') }}</p>
            </div>
            <div class="setting-control">
              <div class="agent-id-field">
                <code class="agent-id-value" :title="editorAgent.id">{{ editorAgent.id }}</code>
                <t-tooltip :content="$t('common.copy')" placement="top">
                  <t-button theme="default" size="small" variant="text" class="agent-id-copy"
                    @click="copyAgentId">
                    <t-icon name="file-copy" />
                  </t-button>
                </t-tooltip>
              </div>
            </div>
          </div>

          <!-- Integration channel status (edit mode; configured in the integration center) -->
          <div v-if="editorMode === 'edit' && editorAgent?.id" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('integrations.agentEditor.label') }}</label>
              <p class="desc">{{ isPostCreateSession ? $t('agent.editor.postCreateHint.integrationDesc') : $t('integrations.agentEditor.desc') }}</p>
            </div>
            <div class="setting-control">
              <div class="integration-inline">
                <button type="button" class="integration-inline__stat integration-inline__link" @click="gotoIntegrations('im')">
                  <span>{{ $t('integrations.tabs.im') }} · {{ agentIMChannelCount }}</span>
                  <t-icon name="chevron-right" size="14px" />
                </button>
                <span class="integration-inline__sep" aria-hidden="true">|</span>
                <button type="button" class="integration-inline__stat integration-inline__link" @click="gotoIntegrations('embed')">
                  <span>{{ $t('integrations.tabs.embed') }} · {{ agentEmbedChannelCount }}</span>
                  <t-icon name="chevron-right" size="14px" />
                </button>
              </div>
            </div>
          </div>

          <!-- Run mode (chosen first) -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.mode') }} <span class="required">*</span></label>
              <p class="desc">{{ agentMode === 'smart-reasoning' ? $t('agent.editor.agentDesc') :
                $t('agent.editor.normalDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-radio-group v-model="agentMode" :disabled="isBuiltinAgent" data-guide="agent-create-mode">
                <t-radio-button value="quick-answer">
                  {{ $t('agent.type.normal') }}
                </t-radio-button>
                <t-radio-button value="smart-reasoning">
                  {{ $t('agent.type.agent') }}
                </t-radio-button>
              </t-radio-group>
            </div>
          </div>

          <!-- Agent type (shown only in Smart Reasoning mode) -->
          <div v-if="isAgentMode && agentTypePresets.length > 0" class="setting-row setting-row--emphasize"
            data-guide="agent-create-agent-type">
            <div class="setting-info">
              <label>{{ $t('agentEditor.agentType.label') }}</label>
              <p class="desc">{{ $t('agentEditor.agentType.desc') }}</p>
              <p v-if="activeAgentTypePreset" class="desc agent-type-preset-desc">{{
                agentTypePresetDescription(activeAgentTypePreset) }}</p>
            </div>
            <div class="setting-control">
              <t-select :value="agentType" @change="onAgentTypeChange" :disabled="isBuiltinAgent"
                :placeholder="$t('agentEditor.agentType.label')" :options="agentTypeSelectOptions"
                :popup-props="{ overlayClassName: 'agent-type-popup' }" class="agent-type-select">
                <template #option="{ option }">
                  <div class="agent-type-option">
                    <span class="agent-type-option-label">{{ option.label }}</span>
                    <span v-if="option.desc" class="agent-type-option-desc">{{ option.desc }}</span>
                  </div>
                </template>
              </t-select>
            </div>
          </div>

          <!-- Name -->
          <div class="setting-row" data-guide="agent-create-name">
            <div class="setting-info">
              <label>{{ $t('agent.editor.name') }} <span v-if="!isBuiltinAgent"
                  class="required">*</span></label>
              <p class="desc">{{ $t('agentEditor.desc.name') }}</p>
            </div>
            <div class="setting-control">
              <div class="name-input-wrapper">
                <!-- Built-in agents use a simple icon -->
                <div v-if="isBuiltinAgent" class="builtin-avatar" :class="isAgentMode ? 'agent' : 'normal'">
                  <t-icon :name="isAgentMode ? 'control-platform' : 'chat'" size="24px" />
                </div>
                <!-- Custom agents use AgentAvatar -->
                <AgentAvatar v-else :name="formData.name || '?'" size="medium" />
                <t-input v-model="formData.name" :placeholder="$t('agent.editor.namePlaceholder')"
                  class="name-input" :disabled="isBuiltinAgent" />
              </div>
            </div>
          </div>

          <!-- Description -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.description') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.description') }}</p>
            </div>
            <div class="setting-control">
              <t-textarea v-model="formData.description"
                :placeholder="$t('agent.editor.descriptionPlaceholder')"
                :autosize="{ minRows: 2, maxRows: 4 }" :disabled="isBuiltinAgent" />
            </div>
          </div>

          <!-- Long-term memory. It stays in the basic info group rather than "Multi-turn conversation" because it is
               unrelated to the multi-turn history window, and Smart Reasoning is exactly the mode that needs this switch most.
               This switch can only turn memory "off": when the workspace or personal setting is off, turning it on here has no effect. -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.memoryEnabled') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.memoryEnabled') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.memory_enabled" />
            </div>
          </div>

        </div>
      </div>

      <!-- Prompts -->
      <div v-show="currentSection === 'prompts'" class="section section--prompts">
        <div class="prompts-panel">
          <div class="prompts-panel__header">
            <div class="section-header section-header--compact">
              <h2>{{ $t('agent.editor.promptsConfig') }}</h2>
              <p class="section-description">{{ $t('agent.editor.promptsConfigDesc') }}</p>
            </div>

            <nav v-if="promptNavItems.length > 1" class="prompts-outline"
              :aria-label="$t('agentEditor.promptNav.ariaLabel')">
              <button v-for="item in promptNavItems" :key="item.key" type="button"
                class="prompts-outline__pill"
                :class="{ 'prompts-outline__pill--active': activePromptAnchor === item.key }"
                @click="activePromptAnchor = item.key">
                <span>{{ item.label }}</span>
                <span v-if="item.customized" class="prompts-outline__dot"
                  :title="$t('agentEditor.intentPrompts.customized')" />
              </button>
            </nav>
          </div>

          <div class="prompts-panel__body">
            <div class="settings-group">
              <!-- System prompt -->
              <div v-show="activePromptAnchor === 'system'"
                class="setting-row setting-row-vertical prompts-panel__pane">
            <div class="setting-info">
              <label>{{ $t('agent.editor.systemPrompt') }} <span v-if="!isBuiltinAgent"
                  class="required">*</span></label>
              <p class="desc">{{ $t('agentEditor.desc.systemPrompt') }}{{ isBuiltinAgent ?
                $t('agentEditor.desc.leaveEmptyDefault') : '' }}</p>
              <p class="desc">{{ $t('agentEditor.desc.promptInheritance') }}</p>
              <div class="placeholder-tags">
                <span class="placeholder-label">{{ $t('agentEditor.placeholders.available') }}</span>
                <t-tooltip v-for="placeholder in availablePlaceholders" :key="placeholder.name"
                  :content="placeholder.description + $t('agentEditor.placeholders.clickToInsert')"
                  placement="top">
                  <span class="placeholder-tag" @click="handlePlaceholderClick('system', placeholder.name)"
                    v-text="'{{' + placeholder.name + '}}'"></span>
                </t-tooltip>
                <span class="placeholder-hint">{{ $t('agentEditor.placeholders.hint') }}</span>
              </div>
            </div>
            <div class="setting-control setting-control-full" style="position: relative;">
              <!-- Agent mode: one unified prompt (the {{web_search_status}} placeholder controls behavior dynamically) -->
              <div v-if="isAgentMode" class="textarea-with-template">
                <t-textarea ref="promptTextareaRef" v-model="formData.config.system_prompt"
                  :placeholder="systemPromptPlaceholder" :autosize="{ minRows: 10, maxRows: 25 }"
                  @input="handlePromptInput" class="system-prompt-textarea" />
                <PromptTemplateSelector type="agentSystemPrompt" position="corner"
                  :hasKnowledgeBase="hasKnowledgeBase" @select="handleSystemPromptTemplateSelect"
                  @reset-default="handleAgentSystemPromptResetDefault" />
              </div>
              <!-- Normal mode: a single prompt -->
              <div v-else class="textarea-with-template">
                <t-textarea ref="promptTextareaRef" v-model="formData.config.system_prompt"
                  :placeholder="systemPromptPlaceholder" :autosize="{ minRows: 10, maxRows: 25 }"
                  @input="handlePromptInput" class="system-prompt-textarea" />
                <PromptTemplateSelector type="systemPrompt" position="corner"
                  :hasKnowledgeBase="hasKnowledgeBase" @select="handleSystemPromptTemplateSelect"
                  @reset-default="handleSystemPromptTemplateSelect" />
              </div>
              <!-- Placeholder hint dropdown -->
              <Teleport to="body">
                <div v-if="showPlaceholderPopup && filteredPlaceholders.length > 0"
                  class="placeholder-popup-wrapper" :style="popupStyle">
                  <div class="placeholder-popup">
                    <div v-for="(placeholder, index) in filteredPlaceholders" :key="placeholder.name"
                      class="placeholder-item" :class="{ active: selectedPlaceholderIndex === index }"
                      @mousedown.prevent="insertPlaceholder(placeholder.name, true)"
                      @mouseenter="selectedPlaceholderIndex = index">
                      <div class="placeholder-name">
                        <code v-html="`{{${placeholder.name}}}`"></code>
                      </div>
                      <div class="placeholder-desc">{{ placeholder.description }}</div>
                    </div>
                  </div>
                </div>
              </Teleport>
            </div>
          </div>

          <!-- Context template (normal mode only) -->
          <div v-if="!isAgentMode" v-show="activePromptAnchor === 'context'"
            class="setting-row setting-row-vertical prompts-panel__pane">
            <div class="setting-info">
              <label>{{ $t('agent.editor.contextTemplate') }} <span v-if="!isBuiltinAgent"
                  class="required">*</span></label>
              <p class="desc">{{ $t('agentEditor.desc.contextTemplate') }}{{ isBuiltinAgent ?
                $t('agentEditor.desc.leaveEmptyDefault') : '' }}</p>
              <div class="placeholder-tags">
                <span class="placeholder-label">{{ $t('agentEditor.placeholders.available') }}</span>
                <t-tooltip v-for="placeholder in contextTemplatePlaceholders" :key="placeholder.name"
                  :content="placeholder.description + $t('agentEditor.placeholders.clickToInsert')"
                  placement="top">
                  <span class="placeholder-tag" @click="handlePlaceholderClick('context', placeholder.name)"
                    v-text="'{{' + placeholder.name + '}}'"></span>
                </t-tooltip>
                <span class="placeholder-hint">{{ $t('agentEditor.placeholders.hint') }}</span>
              </div>
            </div>
            <div class="setting-control setting-control-full" style="position: relative;">
              <div class="textarea-with-template">
                <t-textarea ref="contextTemplateTextareaRef" v-model="formData.config.context_template"
                  :placeholder="contextTemplatePlaceholder" :autosize="{ minRows: 8, maxRows: 20 }"
                  @input="handleContextTemplateInput" class="system-prompt-textarea" />
                <PromptTemplateSelector type="contextTemplate" position="corner"
                  :hasKnowledgeBase="hasKnowledgeBase" @select="handleContextTemplateSelect"
                  @reset-default="handleContextTemplateSelect" />
              </div>
              <!-- Context template placeholder hint dropdown -->
              <Teleport to="body">
                <div v-if="showContextPlaceholderPopup && filteredContextPlaceholders.length > 0"
                  class="placeholder-popup-wrapper" :style="contextPopupStyle">
                  <div class="placeholder-popup">
                    <div v-for="(placeholder, index) in filteredContextPlaceholders" :key="placeholder.name"
                      class="placeholder-item" :class="{ active: selectedContextPlaceholderIndex === index }"
                      @mousedown.prevent="insertContextPlaceholder(placeholder.name, true)"
                      @mouseenter="selectedContextPlaceholderIndex = index">
                      <div class="placeholder-name">
                        <code v-html="`{{${placeholder.name}}}`"></code>
                      </div>
                      <div class="placeholder-desc">{{ placeholder.description }}</div>
                    </div>
                  </div>
                </div>
              </Teleport>
            </div>
          </div>

          <!-- Intent prompt (normal mode only) -->
          <div v-if="!isAgentMode" v-show="activePromptAnchor === 'intent'"
            class="setting-row setting-row-vertical prompts-panel__pane">
            <div class="setting-info">
              <label>{{ $t('agentEditor.intentPrompts.title') }}</label>
              <p class="desc">{{ $t('agentEditor.intentPrompts.sectionDesc') }}</p>
            </div>
            <div class="setting-control setting-control-full">
              <div class="intent-prompts-editor">
                <div v-if="intentPromptTemplates.length === 0" class="prompt-disabled-hint">
                  {{ $t('agentEditor.intentPrompts.empty') }}
                </div>
                <template v-else>
                  <div class="intent-toggle-group" role="tablist"
                    :aria-label="$t('agentEditor.intentPrompts.intentLabel')">
                    <t-button v-for="template in intentPromptTemplates" :key="template.id" theme="default"
                      variant="outline" size="small" class="intent-toggle-btn"
                      :class="{ 'intent-toggle-btn--active': selectedIntent === template.id }"
                      :disabled="props.readOnly" @click="selectedIntent = template.id">
                      <span class="intent-toggle-label">
                        {{ template.name || template.id }}
                        <t-tooltip v-if="isIntentCustomized(template.id)"
                          :content="$t('agentEditor.intentPrompts.customized')" placement="top">
                          <span class="intent-toggle-dot" />
                        </t-tooltip>
                      </span>
                    </t-button>
                  </div>
                  <p v-if="currentIntentTemplateDesc" class="intent-active-desc">{{ currentIntentTemplateDesc
                  }}</p>

                  <div v-if="placeholderData.system_prompt.length > 0" class="placeholder-tags">
                    <span class="placeholder-label">{{ $t('agentEditor.placeholders.available') }}</span>
                    <t-tooltip v-for="placeholder in placeholderData.system_prompt" :key="placeholder.name"
                      :content="placeholder.description + $t('agentEditor.placeholders.clickToInsert')"
                      placement="top">
                      <span class="placeholder-tag"
                        @click="handlePlaceholderClick('intent', placeholder.name)"
                        v-text="'{{' + placeholder.name + '}}'" />
                    </t-tooltip>
                    <span class="placeholder-hint">{{ $t('agentEditor.placeholders.hint') }}</span>
                  </div>

                  <div class="textarea-with-template">
                    <t-textarea ref="intentPromptTextareaRef" v-model="intentEditorValue"
                      class="system-prompt-textarea" :autosize="{ minRows: 10, maxRows: 25 }"
                      :disabled="props.readOnly || !selectedIntent"
                      :placeholder="currentIntentTemplate?.content || $t('agentEditor.intentPrompts.promptPlaceholder')"
                      @input="handleIntentPromptInput" />
                    <PromptTemplateSelector type="intentPrompt" position="corner" :intent-id="selectedIntent"
                      :show-template-picker="false" @reset-default="resetCurrentIntentPrompt" />
                  </div>

                  <Teleport to="body">
                    <div v-if="intentPromptPopup.show && filteredIntentPlaceholders.length > 0"
                      class="placeholder-popup-wrapper" :style="intentPromptPopup.style">
                      <div class="placeholder-popup">
                        <div v-for="(placeholder, index) in filteredIntentPlaceholders" :key="placeholder.name"
                          class="placeholder-item"
                          :class="{ active: intentPromptPopup.selectedIndex === index }"
                          @mousedown.prevent="insertGenericPlaceholder('intent', placeholder.name, true)"
                          @mouseenter="intentPromptPopup.selectedIndex = index">
                          <div class="placeholder-name">
                            <code v-html="`{{${placeholder.name}}}`" />
                          </div>
                          <div class="placeholder-desc">{{ placeholder.description }}</div>
                        </div>
                      </div>
                    </div>
                  </Teleport>
                </template>
              </div>
            </div>
          </div>

          <!-- Rewrite prompt (multi-turn conversation + question rewrite on) -->
          <template
            v-if="!isAgentMode && formData.config.multi_turn_enabled && formData.config.enable_rewrite">
            <div v-show="activePromptAnchor === 'rewrite-system'"
              class="setting-row setting-row-vertical prompts-panel__pane">
              <div class="setting-info">
                <label>{{ $t('agent.editor.rewritePromptSystem') }}</label>
                <p class="desc">{{ $t('agentEditor.desc.rewriteSystemPrompt') }}</p>
                <div class="placeholder-tags" v-if="rewriteSystemPlaceholders.length > 0">
                  <span class="placeholder-label">{{ $t('agentEditor.placeholders.available') }}</span>
                  <t-tooltip v-for="placeholder in rewriteSystemPlaceholders" :key="placeholder.name"
                    :content="placeholder.description + $t('agentEditor.placeholders.clickToInsert')"
                    placement="top">
                    <span class="placeholder-tag"
                      @click="handlePlaceholderClick('rewriteSystem', placeholder.name)"
                      v-text="'{{' + placeholder.name + '}}'"></span>
                  </t-tooltip>
                  <span class="placeholder-hint">{{ $t('agentEditor.placeholders.hint') }}</span>
                </div>
              </div>
              <div class="setting-control setting-control-full" style="position: relative;">
                <div class="textarea-with-template">
                  <t-textarea ref="rewriteSystemTextareaRef" v-model="formData.config.rewrite_prompt_system"
                    :placeholder="defaultRewritePromptSystem || $t('agent.editor.rewritePromptSystemPlaceholder')"
                    :autosize="{ minRows: 4, maxRows: 10 }" @input="handleRewriteSystemInput" />
                  <PromptTemplateSelector type="rewrite" position="corner" @select="handleRewriteTemplateSelect"
                    @reset-default="handleRewriteTemplateSelect" />
                </div>
                <Teleport to="body">
                  <div v-if="rewriteSystemPopup.show && filteredRewriteSystemPlaceholders.length > 0"
                    class="placeholder-popup-wrapper" :style="rewriteSystemPopup.style">
                    <div class="placeholder-popup">
                      <div v-for="(placeholder, index) in filteredRewriteSystemPlaceholders"
                        :key="placeholder.name" class="placeholder-item"
                        :class="{ active: rewriteSystemPopup.selectedIndex === index }"
                        @mousedown.prevent="insertGenericPlaceholder('rewriteSystem', placeholder.name, true)"
                        @mouseenter="rewriteSystemPopup.selectedIndex = index">
                        <div class="placeholder-name">
                          <code v-html="`{{${placeholder.name}}}`"></code>
                        </div>
                        <div class="placeholder-desc">{{ placeholder.description }}</div>
                      </div>
                    </div>
                  </div>
                </Teleport>
              </div>
            </div>

            <div v-show="activePromptAnchor === 'rewrite-user'"
              class="setting-row setting-row-vertical prompts-panel__pane">
              <div class="setting-info">
                <label>{{ $t('agent.editor.rewritePromptUser') }}</label>
                <p class="desc">{{ $t('agentEditor.desc.rewriteUserPrompt') }}</p>
                <div class="placeholder-tags" v-if="rewritePlaceholders.length > 0">
                  <span class="placeholder-label">{{ $t('agentEditor.placeholders.available') }}</span>
                  <t-tooltip v-for="placeholder in rewritePlaceholders" :key="placeholder.name"
                    :content="placeholder.description + $t('agentEditor.placeholders.clickToInsert')"
                    placement="top">
                    <span class="placeholder-tag"
                      @click="handlePlaceholderClick('rewriteUser', placeholder.name)"
                      v-text="'{{' + placeholder.name + '}}'"></span>
                  </t-tooltip>
                  <span class="placeholder-hint">{{ $t('agentEditor.placeholders.hint') }}</span>
                </div>
              </div>
              <div class="setting-control setting-control-full" style="position: relative;">
                <div class="textarea-with-template">
                  <t-textarea ref="rewriteUserTextareaRef" v-model="formData.config.rewrite_prompt_user"
                    :placeholder="defaultRewritePromptUser || $t('agent.editor.rewritePromptUserPlaceholder')"
                    :autosize="{ minRows: 4, maxRows: 10 }" @input="handleRewriteUserInput" />
                  <PromptTemplateSelector type="rewrite" position="corner" @select="handleRewriteTemplateSelect"
                    @reset-default="handleRewriteTemplateSelect" />
                </div>
                <Teleport to="body">
                  <div v-if="rewriteUserPopup.show && filteredRewriteUserPlaceholders.length > 0"
                    class="placeholder-popup-wrapper" :style="rewriteUserPopup.style">
                    <div class="placeholder-popup">
                      <div v-for="(placeholder, index) in filteredRewriteUserPlaceholders"
                        :key="placeholder.name" class="placeholder-item"
                        :class="{ active: rewriteUserPopup.selectedIndex === index }"
                        @mousedown.prevent="insertGenericPlaceholder('rewriteUser', placeholder.name, true)"
                        @mouseenter="rewriteUserPopup.selectedIndex = index">
                        <div class="placeholder-name">
                          <code v-html="`{{${placeholder.name}}}`"></code>
                        </div>
                        <div class="placeholder-desc">{{ placeholder.description }}</div>
                      </div>
                    </div>
                  </div>
                </Teleport>
              </div>
            </div>
          </template>

          <!-- Retrieval fallback (normal mode + knowledge base enabled) -->
          <div v-if="!isAgentMode && hasKnowledgeBase" v-show="activePromptAnchor === 'fallback'"
            class="prompts-panel__pane prompts-panel__pane--stack">
            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('agent.editor.fallbackStrategy') }}</label>
                <p class="desc">{{ $t('agentEditor.desc.fallbackStrategy') }}</p>
              </div>
              <div class="setting-control">
                <t-radio-group v-model="formData.config.fallback_strategy">
                  <t-radio-button value="fixed">{{ $t('agentEditor.fallback.fixed') }}</t-radio-button>
                  <t-radio-button value="model">{{ $t('agentEditor.fallback.model') }}</t-radio-button>
                </t-radio-group>
              </div>
            </div>

            <div v-if="formData.config.fallback_strategy === 'fixed'"
              class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('agent.editor.fallbackResponse') }}</label>
                <p class="desc">{{ $t('agentEditor.desc.fallbackResponse') }}</p>
              </div>
              <div class="setting-control setting-control-full">
                <div class="textarea-with-template">
                  <t-textarea v-model="formData.config.fallback_response"
                    :placeholder="defaultFallbackResponse || $t('agent.editor.fallbackResponsePlaceholder')"
                    :autosize="{ minRows: 2, maxRows: 6 }" />
                  <PromptTemplateSelector type="fallback" position="corner" fallbackMode="fixed"
                    @select="handleFallbackResponseTemplateSelect"
                    @reset-default="handleFallbackResponseTemplateSelect" />
                </div>
              </div>
            </div>

            <div v-if="formData.config.fallback_strategy === 'model'"
              class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('agent.editor.fallbackPrompt') }}</label>
                <p class="desc">{{ $t('agentEditor.desc.fallbackPrompt') }}</p>
                <div class="placeholder-tags" v-if="fallbackPlaceholders.length > 0">
                  <span class="placeholder-label">{{ $t('agentEditor.placeholders.available') }}</span>
                  <t-tooltip v-for="placeholder in fallbackPlaceholders" :key="placeholder.name"
                    :content="placeholder.description + $t('agentEditor.placeholders.clickToInsert')"
                    placement="top">
                    <span class="placeholder-tag"
                      @click="handlePlaceholderClick('fallback', placeholder.name)"
                      v-text="'{{' + placeholder.name + '}}'"></span>
                  </t-tooltip>
                  <span class="placeholder-hint">{{ $t('agentEditor.placeholders.hint') }}</span>
                </div>
              </div>
              <div class="setting-control setting-control-full" style="position: relative;">
                <div class="textarea-with-template">
                  <t-textarea ref="fallbackPromptTextareaRef" v-model="formData.config.fallback_prompt"
                    :placeholder="defaultFallbackPrompt || $t('agent.editor.fallbackPromptPlaceholder')"
                    :autosize="{ minRows: 4, maxRows: 10 }" @input="handleFallbackPromptInput" />
                  <PromptTemplateSelector type="fallback" position="corner" fallbackMode="model"
                    @select="handleFallbackPromptTemplateSelect"
                    @reset-default="handleFallbackPromptTemplateSelect" />
                </div>
                <Teleport to="body">
                  <div v-if="fallbackPromptPopup.show && filteredFallbackPlaceholders.length > 0"
                    class="placeholder-popup-wrapper" :style="fallbackPromptPopup.style">
                    <div class="placeholder-popup">
                      <div v-for="(placeholder, index) in filteredFallbackPlaceholders"
                        :key="placeholder.name" class="placeholder-item"
                        :class="{ active: fallbackPromptPopup.selectedIndex === index }"
                        @mousedown.prevent="insertGenericPlaceholder('fallback', placeholder.name, true)"
                        @mouseenter="fallbackPromptPopup.selectedIndex = index">
                        <div class="placeholder-name">
                          <code v-html="`{{${placeholder.name}}}`"></code>
                        </div>
                        <div class="placeholder-desc">{{ placeholder.description }}</div>
                      </div>
                    </div>
                  </div>
                </Teleport>
              </div>
            </div>
          </div>

            </div>
          </div>
        </div>
      </div>

      <!-- Model config -->
      <div v-show="currentSection === 'model'" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.modelConfig') }}</h2>
          <p class="section-description">{{ $t('agent.editor.modelConfigDesc') }}</p>
        </div>

        <div class="settings-group">
          <!-- Model selection -->
          <div
            class="setting-row"
            data-guide="agent-create-model"
            data-agent-field="summary_model"
            :class="{ 'setting-row--field-highlight': highlightedField === 'summary_model' }"
          >
            <div class="setting-info">
              <label>{{ $t('agent.editor.model') }} <span class="required">*</span></label>
              <p class="desc">{{ $t('agentEditor.desc.model') }}</p>
            </div>
            <div class="setting-control">
              <ModelSelector model-type="KnowledgeQA" :selected-model-id="formData.config.model_id"
                :all-models="allModels"
                @update:selected-model-id="(val: string) => formData.config.model_id = val"
                @add-model="handleAddModel('llm')" :placeholder="chatModelPlaceholder" />
            </div>
          </div>

          <!-- Temperature -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.temperature') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.temperature') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.temperature" :min="0" :max="1" :step="0.1" />
                <span class="slider-value">{{ formData.config.temperature }}</span>
              </div>
            </div>
          </div>

          <!-- Max generated tokens: 0 follows the system default; a custom value is saved as entered -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.maxCompletionTokens') }}</label>
              <p class="desc">{{ isAgentMode ? $t('agentEditor.desc.maxTokensAgent') :
                $t('agentEditor.desc.maxTokens') }}
              </p>
            </div>
            <div class="setting-control max-tokens-control">
              <t-radio-group v-model="maxCompletionTokensMode">
                <t-radio-button value="default">{{ $t('agent.editor.maxCompletionTokensDefault')
                  }}</t-radio-button>
                <t-radio-button value="custom">{{ $t('agent.editor.maxCompletionTokensCustom')
                  }}</t-radio-button>
              </t-radio-group>
              <span v-if="maxCompletionTokensMode === 'default'" class="max-tokens-value">
                {{ effectiveDefaultMaxCompletionTokens }}
              </span>
              <t-input-number v-else v-model="formData.config.max_completion_tokens" :min="100" :max="100000"
                :step="100" theme="column" />
            </div>
          </div>

          <!-- Reasoning effort: off / auto + the levels the selected chat model's catalog entry reports -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.thinking') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.thinking') }}</p>
              <p v-if="selectedChatModel && !selectedChatModelCanThink" class="desc">
                {{ $t('agent.editor.reasoningEffortUnsupported') }}
              </p>
              <p v-else-if="selectedChatModelAlwaysThinks" class="desc">
                {{ $t('agent.editor.reasoningEffortAlwaysOn') }}
              </p>
            </div>
            <div class="setting-control">
              <t-select v-model="reasoningEffortLevel" class="reasoning-effort-select"
                :popup-props="{ overlayClassName: 'reasoning-level-select-popup' }">
                <t-option v-for="level in reasoningEffortOptions" :key="level" :value="level"
                  :label="$t(levelLabelKey(level))" :show-overflow-tooltip="false">
                  <div class="reasoning-level-option">
                    <span class="reasoning-level-option__title">{{ $t(levelLabelKey(level)) }}</span>
                    <span class="reasoning-level-option__hint">{{ $t(levelDescriptionKey(level)) }}</span>
                  </div>
                </t-option>
              </t-select>
            </div>
          </div>

          <!-- Source citations -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.citationEnabled') }}</label>
              <p class="desc">{{ $t('agent.editor.citationEnabledDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.citation_enabled" />
            </div>
          </div>

          <!-- ReRank model (shown when a knowledge base or the search_knowledge tool is enabled) -->
          <div
            v-if="showRerankModelField"
            class="setting-row"
            data-agent-field="rerank_model"
            :class="{ 'setting-row--field-highlight': highlightedField === 'rerank_model' }"
          >
            <div class="setting-info">
              <label>
                {{ $t('agent.editor.rerankModel') }}
                <span v-if="needsRerankModel" class="required">*</span>
              </label>
              <p class="desc">
                {{ $t('agent.editor.rerankModelDesc') }}
                <template v-if="!needsRerankModel">
                  <br />
                  <span class="hint">{{ $t('agent.editor.rerankModelOptionalHint') }}</span>
                </template>
              </p>
            </div>
            <div class="setting-control">
              <ModelSelector model-type="Rerank" :selected-model-id="formData.config.rerank_model_id"
                :all-models="allModels"
                :clearable="!needsRerankModel"
                @update:selected-model-id="(val: string) => formData.config.rerank_model_id = val"
                @add-model="handleAddModel('rerank')"
                :placeholder="rerankModelPlaceholder" />
            </div>
          </div>

          <!-- Question understanding model (for multi-turn rewrite; empty reuses the main chat model) -->
          <div
            v-if="!isAgentMode && formData.config.multi_turn_enabled && formData.config.enable_rewrite"
            class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.queryUnderstandModel') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.queryUnderstandModel') }}</p>
            </div>
            <div class="setting-control">
              <ModelSelector model-type="KnowledgeQA"
                :selected-model-id="formData.config.query_understand_model_id" :all-models="allModels"
                clearable
                @update:selected-model-id="(val: string) => formData.config.query_understand_model_id = val"
                @add-model="handleAddModel('llm')"
                :placeholder="$t('agent.editor.queryUnderstandModelPlaceholder')" />
            </div>
          </div>

          <!-- Max iterations (Agent mode): a positive number is the limit, -1 means unlimited -->
          <div v-if="isAgentMode" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.maxIterations') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.maxIterations') }}</p>
            </div>
            <div class="setting-control max-tokens-control">
              <t-radio-group v-model="maxIterationsMode">
                <t-radio-button value="limit">{{ $t('agent.editor.maxIterationsLimit') }}</t-radio-button>
                <t-radio-button value="unlimited">{{ $t('agent.editor.maxIterationsUnlimited') }}</t-radio-button>
              </t-radio-group>
              <t-input-number v-if="maxIterationsMode === 'limit'" v-model="formData.config.max_iterations"
                :min="2" :max="50" theme="column" />
            </div>
          </div>

          <!-- LLM call timeout (Agent mode) -->
          <div v-if="isAgentMode" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.llmCallTimeout.label') }}</label>
              <p class="desc">{{ $t('agentEditor.llmCallTimeout.desc') }}</p>
              <p class="desc-hint">{{ $t('agentEditor.llmCallTimeout.hint') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.llm_call_timeout" :min="0" :max="3600" theme="column"
                :placeholder="$t('agentEditor.llmCallTimeout.placeholder')" clearable />
            </div>
          </div>

        </div>
      </div>

      <!-- Attachment upload -->
      <div v-show="currentSection === 'multimodal'" class="section">
        <div class="section-header">
          <h2>{{ $t('agentEditor.imageUpload.sectionTitle') }}</h2>
          <p class="section-description">{{ $t('agentEditor.imageUpload.sectionDesc') }}</p>
        </div>

        <div class="settings-group">
          <!-- Image upload -->
          <div class="setting-row" data-guide="agent-create-multimodal">
            <div class="setting-info">
              <label>{{ $t('agentEditor.imageUpload.label') }}</label>
              <p class="desc">{{ $t('agentEditor.imageUpload.desc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.image_upload_enabled" />
            </div>
          </div>

          <!-- VLM model (when image upload is enabled) -->
          <div v-if="formData.config.image_upload_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.imageUpload.vlmModel') }} <span class="required">*</span></label>
              <p class="desc">{{ $t('agentEditor.imageUpload.vlmModelDesc') }}</p>
            </div>
            <div class="setting-control">
              <ModelSelector model-type="VLLM" :selected-model-id="formData.config.vlm_model_id"
                :all-models="allModels"
                @update:selected-model-id="(val: string) => formData.config.vlm_model_id = val"
                @add-model="handleAddModel('vllm')"
                :placeholder="$t('agentEditor.imageUpload.vlmModelPlaceholder')" />
            </div>
          </div>

          <!-- Attachment image understanding / scanned-document OCR (when image upload is enabled) -->
          <div v-if="formData.config.image_upload_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.imageUpload.imageUnderstandingLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.imageUpload.imageUnderstandingDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.attachment_image_understanding" />
            </div>
          </div>

          <!-- Max OCR pages for scanned documents (when attachment image understanding is on) -->
          <div v-if="formData.config.image_upload_enabled && formData.config.attachment_image_understanding"
            class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.imageUpload.ocrMaxPagesLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.imageUpload.ocrMaxPagesDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.attachment_ocr_max_pages" :min="0" :max="64"
                :step="1" theme="normal" style="width: 160px;"
                :placeholder="$t('agentEditor.imageUpload.useGlobalDefault')" />
            </div>
          </div>

          <!-- Image storage provider (when image upload is enabled) -->
          <div v-if="formData.config.image_upload_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.imageUpload.storageProvider') }}</label>
              <p class="desc">{{ $t('agentEditor.imageUpload.storageProviderDesc') }}</p>
            </div>
            <div class="setting-control" style="flex-direction: column; align-items: flex-end;">
              <t-select v-model="formData.config.image_storage_provider" style="width: 280px;"
                :placeholder="$t('agentEditor.imageUpload.storageProviderPlaceholder')" clearable>
                <t-option value="" :label="$t('agentEditor.imageUpload.storageDefault')" />
                <t-option v-for="opt in imageStorageOptions" :key="opt.value" :value="opt.value"
                  :label="opt.label" :disabled="opt.disabled">
                  <span class="select-option-with-tag">
                    <span>{{ opt.label }}</span>
                    <t-tag v-if="opt.disabled" theme="warning" variant="light" size="small">{{
                      $t('agentEditor.imageUpload.notConfigured') }}</t-tag>
                  </span>
                </t-option>
              </t-select>
              <a href="javascript:void(0)" class="go-settings-link"
                @click.prevent="uiStore.openSettings('storage')">
                {{ $t('agentEditor.imageUpload.goStorageSettings') }}
              </a>
            </div>
          </div>

          <!-- Audio upload switch -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.audioUpload.label') }}</label>
              <p class="desc">{{ $t('agentEditor.audioUpload.desc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.audio_upload_enabled" />
            </div>
          </div>

          <!-- ASR model (when audio upload is enabled) -->
          <div v-if="formData.config.audio_upload_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.audioUpload.asrModel') }}</label>
              <p class="desc">{{ $t('agentEditor.audioUpload.asrModelDesc') }}</p>
            </div>
            <div class="setting-control">
              <ModelSelector model-type="ASR" :selected-model-id="formData.config.asr_model_id"
                :all-models="allModels"
                clearable
                @update:selected-model-id="(val: string) => formData.config.asr_model_id = val"
                @add-model="handleAddModel('asr')"
                :placeholder="$t('agentEditor.audioUpload.asrModelPlaceholder')" />
            </div>
          </div>

          <!-- Per-turn timeout waiting for attachment parsing (seconds) -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.chatParser.waitTimeoutLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.chatParser.waitTimeoutDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.attachment_parse_wait_timeout_sec" :min="0" :max="600"
                :step="10" theme="normal" style="width: 160px;"
                :placeholder="$t('agentEditor.imageUpload.useGlobalDefault')" />
            </div>
          </div>

          <!-- Chat attachment parsing strategy -->
          <div class="parser-policy-block">
            <div class="parser-policy-block__header">
              <label>{{ $t('agentEditor.chatParser.label') }}</label>
              <p class="desc">{{ $t('agentEditor.chatParser.desc') }}</p>
            </div>
            <KBParserSettings
              embedded
              :parser-engine-rules="formData.config.chat_parser_engine_rules"
              :relevant-extensions="CHAT_PARSER_EXTENSIONS"
              @update:parser-engine-rules="(val: any) => formData.config.chat_parser_engine_rules = val"
            />
          </div>

        </div>
      </div>

      <!-- Multi-turn conversation. Both modes keep this group: in Agent mode it explains that history is managed automatically
           by the context window and carries keeping retrieval results across turns; the switch is forced on by EnsureDefaults,
           so it is shown only in normal mode. -->
      <div v-show="currentSection === 'conversation'" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.conversationSettings') }}</h2>
          <p class="section-description">{{ conversationSectionDesc }}</p>
        </div>

        <div class="settings-group">
          <!-- Multi-turn conversation switch (normal mode only: Agent mode is forced on by EnsureDefaults,
               and a switch that can be turned off would only be changed back by the server) -->
          <div v-if="!isAgentMode" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.multiTurn') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.multiTurn') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.multi_turn_enabled" />
            </div>
          </div>

          <!-- Turns kept (normal mode only: Agent mode loads history by context window and compresses the overflow into
               a summary, see session_agent_qa.go -> LoadAgentHistory, without reading history_turns) -->
          <div v-if="!isAgentMode && formData.config.multi_turn_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.historyTurns') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.historyRounds') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.history_turns" :min="1" :max="100" theme="column" />
            </div>
          </div>

          <!-- Keep retrieval results across turns. Only the agent path reads this value (internal/agent/observe.go),
               and it rewrites only the history results of the eight KB/Wiki tools, so it is hidden without a knowledge base. -->
          <div v-if="isAgentMode && hasKnowledgeBase" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.retainRetrievalHistory') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.retainRetrievalHistory') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.retain_retrieval_history" />
            </div>
          </div>

          <!-- Question rewrite (shown only with multi-turn conversation on, in normal mode) -->
          <div v-if="formData.config.multi_turn_enabled && !isAgentMode" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.enableRewrite') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.rewrite') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.enable_rewrite" />
            </div>
          </div>
        </div>
      </div>

      <!-- Conversation question suggestions -->
      <div v-show="currentSection === 'suggestions'" class="section">
        <div class="section-header">
          <h2>{{ $t('agentEditor.questionSuggestions.title') }}</h2>
          <p class="section-description">{{ $t('agentEditor.questionSuggestions.description') }}</p>
        </div>

        <t-tabs v-model="suggestionTab" class="suggestion-tabs">
          <t-tab-panel value="starters"
            :label="$t('agentEditor.questionSuggestions.startersTitle')" />
          <t-tab-panel value="followUps"
            :label="$t('agentEditor.questionSuggestions.followUpsTitle')" />
        </t-tabs>

        <div v-show="suggestionTab === 'starters'" class="settings-group">
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.questionSuggestions.enableStarters') }}</label>
              <p class="desc">{{ $t('agentEditor.questionSuggestions.enableStartersDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.question_suggestions.starters.enabled"
                :aria-label="$t('agentEditor.questionSuggestions.enableStarters')" />
            </div>
          </div>

          <div v-if="formData.config.question_suggestions.starters.enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.questionSuggestions.sourceMode') }}</label>
            </div>
            <div class="setting-control">
              <t-select v-model="formData.config.question_suggestions.starters.mode"
                :options="starterSuggestionModeOptions" />
            </div>
          </div>

          <div v-if="formData.config.question_suggestions.starters.enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.questionSuggestions.count') }}</label>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.question_suggestions.starters.count"
                :min="1" :max="8" theme="column" />
            </div>
          </div>

          <div
            v-if="formData.config.question_suggestions.starters.enabled && ['curated', 'hybrid'].includes(formData.config.question_suggestions.starters.mode)"
            class="setting-row setting-row-vertical">
            <div class="setting-info">
              <div class="setting-info-header setting-info-header--inline">
                <label>{{ $t('agentEditor.questionSuggestions.curatedItems') }}</label>
                <span class="curated-items-count">
                  {{ formData.config.question_suggestions.starters.items.length }}/8
                </span>
              </div>
              <p class="desc">{{ $t('agentEditor.questionSuggestions.curatedItemsDesc') }}</p>
            </div>
            <div class="setting-control setting-control-full">
              <div class="suggested-prompts-list">
                <div v-for="(_prompt, index) in formData.config.question_suggestions.starters.items"
                  :key="index" class="prompt-item">
                  <t-input v-model="formData.config.question_suggestions.starters.items[index]"
                    :maxlength="200" />
                  <t-button variant="text" theme="danger" shape="square"
                    :aria-label="$t('common.delete')" @click="removeStarterSuggestion(Number(index))">
                    <t-icon name="delete" />
                  </t-button>
                </div>
                <t-button variant="dashed"
                  :disabled="formData.config.question_suggestions.starters.items.length >= 8"
                  @click="addStarterSuggestion">
                  <template #icon><t-icon name="add" /></template>
                  {{ $t('agentEditor.questionSuggestions.addItem') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>

        <div v-show="suggestionTab === 'followUps'" class="settings-group">
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.questionSuggestions.enableFollowUps') }}</label>
              <p class="desc">{{ $t('agentEditor.questionSuggestions.enableFollowUpsDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.question_suggestions.follow_ups.enabled"
                :aria-label="$t('agentEditor.questionSuggestions.enableFollowUps')" />
            </div>
          </div>

          <template v-if="formData.config.question_suggestions.follow_ups.enabled">
            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.sourceMode') }}</label>
              </div>
              <div class="setting-control">
                <t-select v-model="formData.config.question_suggestions.follow_ups.mode"
                  :options="followUpSuggestionModeOptions" />
              </div>
            </div>

            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.count') }}</label>
              </div>
              <div class="setting-control">
                <t-input-number v-model="formData.config.question_suggestions.follow_ups.count"
                  :min="1" :max="5" theme="column" />
              </div>
            </div>

            <div v-if="formData.config.question_suggestions.follow_ups.mode !== 'knowledge'"
              class="setting-row">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.model') }}</label>
                <p class="desc">{{ $t('agentEditor.questionSuggestions.modelDesc') }}</p>
              </div>
              <div class="setting-control">
                <ModelSelector model-type="KnowledgeQA"
                  :selected-model-id="formData.config.question_suggestions.follow_ups.model_id"
                  :all-models="allModels"
                  clearable
                  @update:selected-model-id="(val: string) => formData.config.question_suggestions.follow_ups.model_id = val"
                  @add-model="handleAddModel('summary')" />
              </div>
            </div>

            <div class="suggestion-advanced-divider">
              <span>{{ $t('agentEditor.questionSuggestions.advancedSettings') }}</span>
            </div>

            <div class="setting-row">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.contextTurns') }}</label>
              </div>
              <div class="setting-control">
                <t-input-number
                  v-model="formData.config.question_suggestions.follow_ups.max_context_turns"
                  :min="1" :max="5" theme="column" />
              </div>
            </div>

            <div class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.categories') }}</label>
              </div>
              <div class="setting-control setting-control-full">
                <t-checkbox-group v-model="formData.config.question_suggestions.follow_ups.categories"
                  :options="followUpCategoryOptions" />
              </div>
            </div>

            <div class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.instruction') }}</label>
              </div>
              <div class="setting-control setting-control-full">
                <t-textarea
                  v-model="formData.config.question_suggestions.follow_ups.additional_instruction"
                  :placeholder="$t('agentEditor.questionSuggestions.instructionPlaceholder')"
                  :maxlength="2000" :autosize="{ minRows: 3, maxRows: 8 }" />
              </div>
            </div>

            <div class="setting-row setting-row-vertical">
              <div class="setting-info">
                <label>{{ $t('agentEditor.questionSuggestions.displayRules') }}</label>
              </div>
              <div class="setting-control setting-control-full">
                <div class="suggestion-checkboxes">
                  <t-checkbox v-model="formData.config.question_suggestions.follow_ups.suppress_on_fallback">{{ $t('agentEditor.questionSuggestions.suppressFallback') }}</t-checkbox>
                  <t-checkbox v-model="formData.config.question_suggestions.follow_ups.suppress_when_answer_asks_question">{{ $t('agentEditor.questionSuggestions.suppressQuestion') }}</t-checkbox>
                  <t-checkbox v-model="formData.config.question_suggestions.follow_ups.knowledge_fallback">{{ $t('agentEditor.questionSuggestions.knowledgeFallback') }}</t-checkbox>
                  <t-checkbox v-model="formData.config.question_suggestions.follow_ups.allow_regenerate">{{ $t('agentEditor.questionSuggestions.allowRegenerate') }}</t-checkbox>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>

      <!-- Tool config (Agent mode only) -->
      <div v-show="currentSection === 'tools' && isAgentMode" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.toolsConfig') }}</h2>
          <p class="section-description">{{ $t('agent.editor.toolsConfigDesc') }}</p>
        </div>

        <!-- Combined panel: capability status + preset switch -->
        <div class="tools-overview">
          <div class="tools-overview-row">
            <div class="tools-status-chip">
              <t-icon name="folder" />
              <template v-if="kbSelectionMode === 'none'">
                <span>{{ $t('agentEditor.tools.statusNoKb') }}</span>
              </template>
              <template v-else>
                <span class="tools-status-metric">
                  <strong>{{ ragKbCount }}</strong> {{ $t('agentEditor.tools.kbMetricRag') }}
                </span>
                <span class="tools-status-sep">·</span>
                <span class="tools-status-metric">
                  <strong>{{ wikiKbCount }}</strong> {{ $t('agentEditor.tools.kbMetricWiki') }}
                </span>
              </template>
            </div>
            <div v-if="inactiveToolCount > 0" class="tools-status-chip tools-status-chip--warn">
              <t-icon name="error-circle" />
              <span>{{ $t('agentEditor.tools.statusInactive', { count: inactiveToolCount }) }}</span>
            </div>
          </div>
        </div>

        <div class="settings-group">
          <!-- Allowed tools (rendered by group, one uniform grid) -->
          <div
            class="setting-row setting-row-vertical"
            data-agent-field="allowed_tools"
            :class="{ 'setting-row--field-highlight': highlightedField === 'allowed_tools' }"
          >
            <div class="setting-info">
              <label>{{ $t('agent.editor.allowedTools') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.selectTools') }}</p>
            </div>
            <div class="setting-control setting-control-full">
              <t-checkbox-group v-model="formData.config.allowed_tools" class="tool-groups">
                <section v-for="group in groupedAvailableTools" :key="group.key"
                  :class="['tool-group', `tool-group--${group.key}`]">
                  <header class="tool-group-header">
                    <span class="tool-group-bar" />
                    <span class="tool-group-title">{{ group.label }}</span>
                    <span class="tool-group-count">{{ group.tools.length }}</span>
                    <span v-if="group.key === 'wiki_edit'" class="tool-group-warning">
                      <t-icon name="error-circle" />
                      {{ $t('agentEditor.tools.writeWarning') }}
                    </span>
                  </header>
                  <div class="tool-grid">
                    <t-checkbox v-for="tool in group.tools" :key="tool.value" :value="tool.value"
                      :disabled="tool.disabled"
                      :class="['tool-card', { 'tool-card--disabled': tool.disabled, 'tool-card--danger': tool.danger }]">
                      <div class="tool-card-body">
                        <div class="tool-card-head">
                          <span class="tool-card-name">{{ tool.label }}</span>
                          <span v-if="tool.danger" class="tool-card-badge">
                            {{ $t('agentEditor.tools.dangerTag') }}
                          </span>
                        </div>
                        <span v-if="tool.description" class="tool-card-desc">{{ tool.description }}</span>
                        <span v-if="tool.disabled && tool.disabledReason" class="tool-card-hint">
                          {{ tool.disabledReason }}
                        </span>
                      </div>
                    </t-checkbox>
                  </div>
                </section>
              </t-checkbox-group>
            </div>
          </div>

          <!-- Effective tools preview: what you see is what you get -->
          <div class="setting-row setting-row-vertical">
            <div class="setting-info">
              <label>{{ $t('agentEditor.tools.effectiveLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.tools.effectiveDesc') }}</p>
            </div>
            <div class="setting-control setting-control-full">
              <div class="effective-tools">
                <template v-if="effectiveTools.length === 0">
                  <div class="effective-tools-empty">
                    {{ $t('agentEditor.tools.effectiveEmpty') }}
                  </div>
                </template>
                <template v-else>
                  <span v-for="item in effectiveTools" :key="item.value"
                    :class="['effective-chip', { 'effective-chip--inactive': !item.active }]"
                    :title="item.reason || ''">
                    <span class="effective-chip-label">{{ item.label }}</span>
                    <span v-if="!item.active" class="effective-chip-reason">{{ item.reason }}</span>
                  </span>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- MCP service config (Agent mode only) -->
      <div v-show="currentSection === 'mcp' && isAgentMode" class="section">
        <div class="section-header">
          <h2>{{ $t('agentEditor.mcp.label') }}</h2>
          <p class="section-description">{{ $t('agentEditor.mcp.desc') }}</p>
        </div>

        <div class="settings-group">
          <!-- MCP service selection -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.mcp.label') }}</label>
              <p class="desc">{{ $t('agentEditor.mcp.desc') }}</p>
            </div>
            <div class="setting-control">
              <t-radio-group v-model="mcpSelectionMode">
                <t-radio-button value="all">{{ $t('agentEditor.selection.all') }}</t-radio-button>
                <t-radio-button value="selected">{{ $t('agentEditor.selection.selected') }}</t-radio-button>
                <t-radio-button value="none">{{ $t('agentEditor.selection.disabled') }}</t-radio-button>
              </t-radio-group>
            </div>
          </div>

          <!-- Select specific MCP services -->
          <div v-if="mcpSelectionMode === 'selected' && showMcpServiceSelect" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.mcp.selectLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.mcp.selectDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-select v-model="formData.config.mcp_services" multiple
                :placeholder="$t('agentEditor.mcp.selectPlaceholder')" filterable>
                <t-option v-for="mcp in mcpOptions" :key="mcp.value" :value="mcp.value" :label="mcp.label"
                  :disabled="mcp.disabled" />
              </t-select>
            </div>
          </div>

          <!-- Authorization wait timeout: seconds to wait when an OAuth authorization fires during a chat -->
          <div v-if="mcpSelectionMode !== 'none'" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.mcp.authWaitTimeout') }}</label>
              <p class="desc">{{ $t('agentEditor.mcp.authWaitTimeoutDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.mcp_auth_wait_timeout" :min="5" :max="3600"
                theme="column" :placeholder="$t('agentEditor.mcp.authWaitTimeoutPlaceholder')" />
            </div>
          </div>
        </div>
      </div>

      <!-- Skills: scripts run in the selected sandbox, and the available list also comes from that config -->
      <div v-show="currentSection === 'skills' && isAgentMode" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.skillsConfig') }}</h2>
          <p class="section-description">{{ hostOnly ? $t('agent.editor.hostSkillsConfigDesc') : $t('agent.editor.skillsConfigDesc') }}</p>
        </div>

        <div class="settings-group">
          <div v-if="!hostOnly" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.sandboxBackend') }}</label>
              <p class="desc">{{ $t('agent.editor.sandboxBackendHint') }}</p>
            </div>
            <div class="setting-control sandbox-select-control">
              <t-select
                v-model="formData.config.sandbox_config_id"
                :placeholder="$t('agent.editor.sandboxBackendDefault')"
                class="sandbox-config-select"
                filterable
                :popup-props="{ overlayClassName: 'sandbox-config-select-popup' }"
              >
                <t-option value="" :label="$t('agent.editor.sandboxBackendDefault')" />
                <t-option
                  v-for="cfg in sandboxConfigOptions"
                  :key="cfg.id"
                  :value="cfg.id"
                  :label="cfg.name"
                >
                  <div class="sandbox-option">
                    <div class="sandbox-option__row">
                      <span class="sandbox-option__name">{{ cfg.name }}</span>
                      <span v-if="cfg.sandbox_type" class="sandbox-option__type">{{ backendLabel(cfg.sandbox_type) }}</span>
                    </div>
                    <div v-if="sandboxTargetLine(cfg)" class="sandbox-option__target">{{ sandboxTargetLine(cfg) }}</div>
                  </div>
                </t-option>
              </t-select>
              <p v-if="selectedSandboxSummary" class="sandbox-selected-meta">{{ selectedSandboxSummary }}</p>
              <div class="sandbox-select-links">
                <a href="javascript:void(0)" class="go-settings-link"
                  @click.prevent="uiStore.openSettings('sandbox')">
                  {{ $t('agent.editor.goSandboxSettings') }}
                </a>
                <template v-if="hasSandboxSelected && canInstallSkills">
                  <span class="sandbox-select-links__sep" aria-hidden="true">·</span>
                  <a
                    href="javascript:void(0)"
                    class="go-settings-link"
                    @click.prevent="openSkillSettings"
                  >
                    {{ $t('agent.editor.goSkillSettings') }}
                  </a>
                </template>
              </div>
              <p v-if="sandboxConfigOptions.length === 0" class="desc empty-hint">
                {{ $t('agent.editor.sandboxNoConfigs') }}
              </p>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.skillsSelection') }}</label>
              <p class="desc">{{ skillsSelectionHint }}</p>
            </div>
            <div class="setting-control sandbox-select-control">
              <t-radio-group v-model="skillsSelectionMode">
                <t-radio-button value="all" :disabled="!canEnableSkills">{{ $t('agent.editor.skillsAll') }}</t-radio-button>
                <t-radio-button value="selected" :disabled="!canEnableSkills">{{ $t('agent.editor.skillsSelected') }}</t-radio-button>
                <t-radio-button value="none">{{ $t('agent.editor.skillsNone') }}</t-radio-button>
              </t-radio-group>
              <p v-if="!hasSandboxSelected && sandboxConfigOptions.length > 1" class="desc empty-hint">
                {{ $t('agent.editor.skillsNeedSandbox') }}
              </p>
              <p v-else-if="hasSandboxSelected && skillCatalog.length === 0" class="desc empty-hint">
                <span>{{ $t('agent.editor.noSkillsAvailable') }}</span>
                <a
                  v-if="canInstallSkills"
                  href="javascript:void(0)"
                  class="go-settings-link"
                  @click.prevent="openSkillSettings"
                >
                  {{ $t('agent.editor.goSkillSettings') }}
                </a>
              </p>
            </div>
          </div>

          <div v-if="showCatalogSkillList" class="setting-row setting-row-vertical">
            <div class="setting-control setting-control-full">
              <t-checkbox-group
                v-model="formData.config.selected_skills"
                class="skill-pick-list"
              >
                <section
                  v-for="group in catalogSkillGroups"
                  :key="group.key"
                  class="skill-pick-group"
                  :class="`skill-pick-group--${group.key}`"
                >
                  <header class="skill-pick-group__header">
                    <span class="skill-pick-group__bar" />
                    <span class="skill-pick-group__title">{{ group.label }}</span>
                    <span class="skill-pick-group__count">{{ group.skills.length }}</span>
                  </header>
                  <article
                    v-for="skill in group.skills"
                    :key="skill.name"
                    class="skill-pick"
                    :class="{
                      'skill-pick--ready': skill.selectable,
                      'skill-pick--pending': !skill.selectable,
                      'skill-pick--busy': isSkillBusy(skill),
                    }"
                  >
                    <t-checkbox
                      v-if="skillsSelectionMode === 'selected'"
                      :value="skill.name"
                      :disabled="!skill.selectable"
                      class="skill-pick__check"
                    />
                    <div class="skill-pick__badge" aria-hidden="true">
                      <t-icon :name="SKILL_ICON" size="16px" />
                    </div>
                    <div class="skill-pick__body">
                      <div class="skill-pick__title-row">
                        <span class="skill-name" :title="skill.name">{{ skill.name }}</span>
                        <span
                          v-if="!skill.selectable"
                          class="skill-pick__hint"
                          :class="{ 'skill-pick__hint--busy': isSkillBusy(skill) }"
                        >
                          <t-icon :name="skillStatusIcon(skill)" size="14px" />
                          {{ skillStatusHint(skill) }}
                        </span>
                        <span
                          v-if="skill.selectable && skill.servedNote"
                          class="skill-pick__hint"
                          :class="{ 'skill-pick__hint--busy': isSkillBusy(skill) }"
                        >
                          <t-icon :name="isSkillBusy(skill) ? 'refresh' : 'error-circle'" size="14px" />
                          {{ skill.servedNote }}
                        </span>
                        <span
                          v-if="canUpgradeSkillRow(skill)"
                          class="skill-pick__hint skill-pick__hint--upgrade"
                        >
                          <t-icon name="arrow-up" size="14px" />
                          {{ skillUpgradeHint(skill) }}
                        </span>
                      </div>
                      <p
                        v-if="skill.description"
                        class="skill-desc"
                        :title="skill.description"
                      >{{ skill.description }}</p>
                    </div>
                    <t-button
                      v-if="canInstallSkillRow(skill)"
                      size="small"
                      variant="text"
                      theme="primary"
                      :loading="installingCatalogId === skill.id"
                      :title="installsAnUpgrade(skill) ? (hostOnly ? $t('agent.editor.hostUpgradeOnThisComputer') : $t('agent.editor.upgradeOnThisSandbox')) : (hostOnly ? $t('agent.editor.hostInstallToThisComputer') : $t('agent.editor.installToThisSandbox'))"
                      @click.stop="installCatalogToCurrent(skill)"
                    >
                      {{ installsAnUpgrade(skill) ? $t('settings.skills.upgrade') : $t('agent.editor.installShort') }}
                    </t-button>
                    <t-button
                      v-else-if="canUpgradeSkillRow(skill)"
                      size="small"
                      variant="text"
                      theme="primary"
                      :loading="installingCatalogId === skill.id"
                      :title="hostOnly ? $t('agent.editor.hostUpgradeOnThisComputer') : $t('agent.editor.upgradeOnThisSandbox')"
                      @click.stop="installCatalogToCurrent(skill)"
                    >
                      {{ $t('settings.skills.upgrade') }}
                    </t-button>
                    <t-button
                      v-else-if="canInstallSkills && isSkillBusy(skill)"
                      size="small"
                      variant="text"
                      theme="primary"
                      :title="$t('agent.editor.viewInstallProgress')"
                      @click.stop="openSkillInstallProgress(skill)"
                    >
                      {{ $t('agent.editor.viewInstallProgress') }}
                    </t-button>
                  </article>
                </section>
              </t-checkbox-group>
            </div>
          </div>
        </div>
      </div>

      <!-- Knowledge base config -->
      <div v-show="currentSection === 'knowledge'" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.knowledgeConfig') }}</h2>
          <p class="section-description">{{ $t('agent.editor.knowledgeConfigDesc') }}</p>
        </div>

        <div class="settings-group">
          <!-- Linked knowledge bases -->
          <div class="setting-row" data-guide="agent-create-knowledge">
            <div class="setting-info">
              <label>{{ $t('agent.editor.knowledgeBases') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.kbScope') }}</p>
            </div>
            <div class="setting-control">
              <t-radio-group v-model="kbSelectionMode">
                <t-radio-button value="all">{{ $t('agent.editor.allKnowledgeBases') }}</t-radio-button>
                <t-radio-button value="selected">{{ $t('agent.editor.selectedKnowledgeBases')
                  }}</t-radio-button>
                <t-radio-button value="none">{{ $t('agent.editor.noKnowledgeBase') }}</t-radio-button>
              </t-radio-group>
            </div>
          </div>

          <!-- Select specific knowledge bases (shown only when "Specific knowledge bases" is chosen) -->
          <div v-if="kbSelectionMode === 'selected'" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.selectKnowledgeBases') }}</label>
              <p class="desc">{{ $t('agent.editor.selectKnowledgeBasesDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-select v-model="formData.config.knowledge_bases" multiple
                :placeholder="$t('agent.editor.selectKnowledgeBases')" filterable :min-collapsed-num="3">
                <t-option-group v-if="filteredMyKbOptions.length"
                  :label="$t('agent.editor.myKnowledgeBases')">
                  <t-option v-for="kb in filteredMyKbOptions" :key="kb.value" :value="kb.value"
                    :label="kb.label" :disabled="kb.disabled">
                    <div class="kb-option-item" :title="kb.disabled ? kb.disabledReason : ''">
                      <span class="kb-option-icon" :class="kb.type === 'faq' ? 'faq-icon' : 'doc-icon'">
                        <t-icon :name="kb.type === 'faq' ? 'chat-bubble-help' : 'folder'" />
                      </span>
                      <span class="kb-option-label">{{ kb.label }}</span>
                      <span v-if="kb.ragEnabled" class="kb-option-tag tag-rag">RAG</span>
                      <span v-if="kb.wikiEnabled" class="kb-option-tag tag-wiki">Wiki</span>
                      <span class="kb-option-count">{{ kb.count || 0 }}</span>
                      <span v-if="kb.disabled" class="kb-option-disabled-hint">{{ kb.disabledReason }}</span>
                    </div>
                  </t-option>
                </t-option-group>
                <t-option-group v-if="filteredSharedKbOptions.length"
                  :label="$t('agent.editor.sharedKnowledgeBases')">
                  <t-option v-for="kb in filteredSharedKbOptions" :key="kb.value" :value="kb.value"
                    :label="kb.label" :disabled="kb.disabled">
                    <div class="kb-option-item" :title="kb.disabled ? kb.disabledReason : ''">
                      <span class="kb-option-icon" :class="kb.type === 'faq' ? 'faq-icon' : 'doc-icon'">
                        <t-icon :name="kb.type === 'faq' ? 'chat-bubble-help' : 'folder'" />
                      </span>
                      <span class="kb-option-label">{{ kb.label }}</span>
                      <span v-if="kb.ragEnabled" class="kb-option-tag tag-rag">RAG</span>
                      <span v-if="kb.wikiEnabled" class="kb-option-tag tag-wiki">Wiki</span>
                      <span v-if="kb.orgName" class="kb-option-org">{{ kb.orgName }}</span>
                      <span class="kb-option-count">{{ kb.count || 0 }}</span>
                      <span v-if="kb.disabled" class="kb-option-disabled-hint">{{ kb.disabledReason }}</span>
                    </div>
                  </t-option>
                </t-option-group>
              </t-select>
            </div>
          </div>

          <!-- Supported file types (limits the file types the user can choose) -->
          <div v-if="hasKnowledgeBase" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.fileTypes.label') }}</label>
              <p class="desc">{{ $t('agentEditor.fileTypes.desc') }}</p>
            </div>
            <div class="setting-control">
              <t-select v-model="formData.config.supported_file_types" multiple
                :placeholder="$t('agentEditor.fileTypes.allTypes')" :min-collapsed-num="3" clearable>
                <t-option v-for="ft in availableFileTypes" :key="ft.value" :value="ft.value"
                  :label="ft.label" />
              </t-select>
            </div>
          </div>

          <!-- Search the knowledge base only when mentioned (shown when knowledge bases are configured) -->
          <div v-if="hasKnowledgeBase" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.retrieveKBOnlyWhenMentioned') }}</label>
              <p class="desc">{{ $t('agent.editor.retrieveKBOnlyWhenMentionedDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.retrieve_kb_only_when_mentioned" />
            </div>
          </div>

        </div>
      </div>

      <!-- Web search config -->
      <div v-show="currentSection === 'websearch'" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.webSearchConfig') }}</h2>
          <p class="section-description">{{ $t('agent.editor.webSearchConfigDesc') }}</p>
        </div>

        <div class="settings-group">
          <!-- Web search -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.webSearch') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.webSearch') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.web_search_enabled" />
            </div>
          </div>

          <!-- Web search max results -->
          <div v-if="formData.config.web_search_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.webSearchProvider') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.webSearchProvider') }}</p>
            </div>
            <div class="setting-control">
              <t-select v-model="formData.config.web_search_provider_id" clearable
                :placeholder="$t('agent.editor.webSearchProviderPlaceholder')" style="width: 240px;">
                <t-option v-for="p in webSearchProviderList" :key="p.id" :value="p.id" :label="p.name">
                  <span>{{ p.name }}</span>
                  <t-tag v-if="p.is_default" theme="primary" size="small" style="margin-left: 6px;">{{
                    $t('common.default')
                    }}</t-tag>
                </t-option>
              </t-select>
            </div>
          </div>

          <!-- Web search max results -->
          <div v-if="formData.config.web_search_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.webSearchMaxResults') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.webSearchMaxResults') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.web_search_max_results" :min="1" :max="10" />
                <span class="slider-value">{{ formData.config.web_search_max_results }}</span>
              </div>
            </div>
          </div>

          <!-- Fetch page content automatically -->
          <div v-if="formData.config.web_search_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.webFetchEnabled') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.webFetchEnabled') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.web_fetch_enabled" />
            </div>
          </div>

          <!-- Pages to fetch -->
          <div v-if="formData.config.web_search_enabled && formData.config.web_fetch_enabled"
            class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.webFetchTopN') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.webFetchTopN') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.web_fetch_top_n" :min="1" :max="10" />
                <span class="slider-value">{{ formData.config.web_fetch_top_n }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Retrieval strategy (shown only in normal mode with knowledge base capability; Agent mode's search_knowledge
           uses the global retrieval config, so these settings do not apply to it) -->
      <div v-show="currentSection === 'retrieval' && hasKnowledgeBase && !isAgentMode" class="section">
        <div class="section-header">
          <h2>{{ $t('agent.editor.retrievalStrategy') }}</h2>
          <p class="section-description">{{ $t('agentEditor.desc.retrievalSection') }}</p>
        </div>

        <div class="settings-group">
          <!-- Query expansion (normal mode only) -->
          <div v-if="!isAgentMode" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.enableQueryExpansion') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.queryExpansion') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.enable_query_expansion" />
            </div>
          </div>

          <!-- Vector recall TopK -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.embeddingTopK') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.embeddingTopK') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.embedding_top_k" :min="1" :max="50" theme="column" />
            </div>
          </div>

          <!-- Keyword threshold -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.keywordThreshold') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.keywordThreshold') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.keyword_threshold" :min="0" :max="1" :step="0.01" />
                <span class="slider-value">{{ formData.config.keyword_threshold?.toFixed(2) }}</span>
              </div>
            </div>
          </div>

          <!-- Vector threshold -->
          <div class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.vectorThreshold') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.vectorThreshold') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.vector_threshold" :min="0" :max="1" :step="0.01" />
                <span class="slider-value">{{ formData.config.vector_threshold?.toFixed(2) }}</span>
              </div>
            </div>
          </div>

          <!-- Rerank TopK (shown only when a Rerank model is configured) -->
          <div v-if="formData.config.rerank_model_id" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.rerankTopK') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.rerankTopK') }}</p>
            </div>
            <div class="setting-control">
              <t-input-number v-model="formData.config.rerank_top_k" :min="1" :max="20" theme="column" />
            </div>
          </div>

          <!-- Rerank threshold (shown only when a Rerank model is configured) -->
          <div v-if="formData.config.rerank_model_id" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agent.editor.rerankThreshold') }}</label>
              <p class="desc">{{ $t('agentEditor.desc.rerankThreshold') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.rerank_threshold" :min="-10" :max="10" :step="0.01" />
                <span class="slider-value">{{ formData.config.rerank_threshold?.toFixed(1) }}</span>
              </div>
            </div>
          </div>

          <!-- FAQ priority strategy (shown when a FAQ-type knowledge base is linked) -->
          <div v-if="hasFaqKnowledgeBase" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.faq.enableLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.faq.enableDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.faq_priority_enabled" />
            </div>
          </div>

          <div v-if="hasFaqKnowledgeBase && formData.config.faq_priority_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.faq.thresholdLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.faq.thresholdDesc') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.faq_direct_answer_threshold" :min="0.7" :max="1"
                  :step="0.05" />
                <span class="slider-value">{{ formData.config.faq_direct_answer_threshold?.toFixed(2)
                  }}</span>
              </div>
            </div>
          </div>

          <div v-if="hasFaqKnowledgeBase && formData.config.faq_priority_enabled" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.faq.boostLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.faq.boostDesc') }}</p>
            </div>
            <div class="setting-control">
              <div class="slider-wrapper">
                <t-slider v-model="formData.config.faq_score_boost" :min="1" :max="2" :step="0.1" />
                <span class="slider-value">{{ formData.config.faq_score_boost?.toFixed(1) }}x</span>
              </div>
            </div>
          </div>

          <!-- Table data analysis (normal mode only; a CSV/Excel hit adds one LLM call to generate SQL) -->
          <div v-if="!isAgentMode" class="setting-row">
            <div class="setting-info">
              <label>{{ $t('agentEditor.dataAnalysis.enableLabel') }}</label>
              <p class="desc">{{ $t('agentEditor.dataAnalysis.enableDesc') }}</p>
            </div>
            <div class="setting-control">
              <t-switch v-model="formData.config.data_analysis_enabled" />
            </div>
          </div>
        </div>
      </div>

      <!-- Sharing management (edit mode only, and not for built-in agents) -->
      <div v-if="editorMode === 'edit' && editorAgent?.id && !editorAgent?.is_builtin"
        v-show="currentSection === 'share'" class="section">
        <AgentShareSettings :agent-id="editorAgent.id" :agent="editorAgent" />
      </div>
    </div>

    <template #footer-note>
      <p v-if="isPostCreateSession" class="settings-footer-note">
        <t-icon name="check-circle-filled" class="settings-footer-note__icon" />
        <span>
          <strong>{{ $t('agent.editor.postCreateHint.title') }}</strong>
          {{ $t('agent.editor.postCreateHint.footer') }}
        </span>
      </p>
    </template>
    <template #footer>
      <t-button variant="outline" @click="modalShell.requestClose">
        {{ props.readOnly ? $t('common.close') : $t('common.cancel') }}
      </t-button>
      <t-button v-if="!props.readOnly" theme="primary" data-guide="agent-create-submit" :loading="saving"
        :disabled="editorInitializing" @click="handleSave">
        {{ saveButtonLabel }}
      </t-button>
    </template>
  </SettingsModalShell>

  <SettingDrawer
    v-model:visible="showSkillProgress"
    :title="skillProgressTitle"
    :description="skillProgressDesc"
    :icon="SKILL_ICON"
    width="680px"
    :min-width="560"
    :max-width="920"
    storage-key="setting-drawer:width:skill-catalog-manage"
    :hide-footer="true"
  >
    <SandboxSkillsPanel
      v-if="showSkillProgress && skillProgressRecord && skillProgressId"
      :record="skillProgressRecord"
      mode="list"
      hide-add
      :focus-skill-id="skillProgressId"
      @updated="onSkillProgressUpdated"
      @skills-changed="onSkillProgressChanged"
    />
  </SettingDrawer>

  <AgentCreateContextualGuide :when="visible && editorMode === 'create'" :is-agent-mode="isAgentMode" />
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import AgentCreateContextualGuide from '@/components/AgentCreateContextualGuide.vue';
import {
  AGENT_EDITOR_FOCUS_SECTION_EVENT,
  markContextualGuideDone,
} from '@/config/contextualGuides';
import { useI18n } from 'vue-i18n';
import { resolveBuiltinDefaultModelId, selectInitialModelId } from '@/utils/modelDefaults';
import { hydrateAgentPromptRefs, serializeAgentPrompts } from '@/utils/agentPromptTemplates';
import { copyWithToast } from '@/utils/clipboard';
import { MessagePlugin } from 'tdesign-vue-next';
import { useModalShell } from '@/composables/useModalShell'
import SettingsModalShell from '@/components/SettingsModalShell.vue'
import {
  createAgent,
  updateAgent,
  listIMChannels,
  type CustomAgent,
  type PlaceholderDefinition,
  type AgentTypePreset,
  type AgentType,
  type AgentTypeKBFilter,
  type KBCapabilities,
} from '@/api/agent';
import { type ModelConfig } from '@/api/model';
import { type AgentNotReadyReasonKey, agentRequiresRerankModel } from '@/utils/agent-readiness';
import { normalizeLegacyToolNames } from '@/utils/legacy-tool-names';
import { installSkillCatalog, type SkillCatalogItem } from '@/api/skill';
import { installUpgradable, servedPreviousText, upgradeVersions } from '@/utils/skillUpgrade';
import { type WebSearchProviderEntity } from '@/api/web-search-provider';
import {
  isNamedSandboxBackend,
  type SandboxConfigRecord,
  type StorageEngineStatusItem,
  type PromptTemplate,
  type PromptTemplatesConfig,
} from '@/api/system';
import { useUIStore } from '@/stores/ui';
import { useAuthStore } from '@/stores/auth';
import { useOrganizationStore } from '@/stores/organization';
import { useChatResourcesStore } from '@/stores/chatResources';
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities';
import { HOST_SKILL_TARGET_ID, hostSkillTargetRecord, hostSkillsOnly } from '@/utils/skillTarget';
import { useEditorResourcesStore } from '@/stores/editorResources';
import AgentAvatar from '@/components/AgentAvatar.vue';
import PromptTemplateSelector from '@/components/PromptTemplateSelector.vue';
import ModelSelector from '@/components/ModelSelector.vue';
import SandboxSkillsPanel from '@/components/SandboxSkillsPanel.vue';
import SettingDrawer from '@/components/settings/SettingDrawer.vue';
import KBParserSettings, { type ParserEngineRule } from '@/views/knowledge/settings/KBParserSettings.vue';
import AgentShareSettings from '@/components/AgentShareSettings.vue';
import { SKILL_ICON } from '@/types/mention';
import { listEmbedChannels } from '@/api/embed';
import { getRootZoom, rectToCssPx } from '@/utils/zoom';
import { integrationSectionKey } from '@/config/settingsRoute';
import { toolboxLocation } from '@/config/toolbox';
import {
  evaluateToolRequirement,
  deriveKbFilterFromTools,
  type RequirementMissKind,
  type ScopeCapabilities,
} from '@/utils/tool-capabilities';
import {
  clampLevel,
  levelDescriptionKey,
  levelEnablesThinking,
  levelFromLegacy,
  levelLabelKey,
  modelCanThink,
  modelCannotDisableThinking,
  optionsFor,
  type ReasoningLevel,
} from '@/utils/reasoningEffort';

// File extensions offered in the agent-level chat attachment parsing policy.
const CHAT_PARSER_EXTENSIONS = [
  'pdf', 'doc', 'docx', 'xls', 'xlsx', 'xlsb', 'ppt', 'pptx', 'epub', 'mhtml',
  'txt', 'md', 'markdown', 'csv', 'json', 'xml', 'html', 'yaml', 'yml', 'log',
  'jpg', 'jpeg', 'png', 'gif', 'bmp', 'tiff', 'webp',
];

const uiStore = useUIStore();
const authStore = useAuthStore();
const deploymentCapabilities = useDeploymentCapabilitiesStore();
const router = useRouter();
const orgStore = useOrganizationStore();
const chatResources = useChatResourcesStore();
const editorResources = useEditorResourcesStore();

const { t, locale: i18nLocale } = useI18n();

const props = defineProps<{
  visible: boolean;
  mode: 'create' | 'edit';
  agent?: CustomAgent | null;
  initialSection?: string;
  initialHighlightField?: string;
  // readOnly hides the save button so a Viewer who clicks an agent
  // card to inspect its config doesn't see an "OK" that 403s on the
  // backend update endpoint. Field-level disable is intentionally NOT
  // wired here yet (the modal has 3000+ lines of form inputs); instead
  // we just remove the only mutation surface - the footer button.
  readOnly?: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:visible', visible: boolean): void;
  (e: 'success', agent?: CustomAgent): void;
}>();

/** After the first save creates the agent, stay in the modal and switch to edit mode with local state to show the IM / embed entries */
const savedAgent = ref<CustomAgent | null>(null);
const editorMode = computed(() => (savedAgent.value ? 'edit' : props.mode));
const editorAgent = computed(() => savedAgent.value ?? props.agent ?? null);
const isPostCreateSession = computed(() => !!savedAgent.value);
const saveButtonLabel = computed(() =>
  editorMode.value === 'create'
    ? t('agent.editor.buttons.create')
    : t('agent.editor.buttons.saveAndClose')
);

const copyAgentId = async () => {
  await copyWithToast(editorAgent.value?.id, 'common.copied');
};

// The old entry split the skill sandbox into its own tab; after the merge, section=sandbox is still accepted.
const AGENT_EDITOR_SECTION_ALIASES: Record<string, string> = {
  sandbox: 'skills',
};

function resolveEditorSection(section?: string | null): string {
  const key = section || 'basic';
  return AGENT_EDITOR_SECTION_ALIASES[key] || key;
}

const currentSection = ref(resolveEditorSection(props.initialSection));
const suggestionTab = ref<'starters' | 'followUps'>('starters');
const contentWrapperRef = ref<HTMLElement | null>(null);
const highlightedField = ref<AgentNotReadyReasonKey | null>(null);
let highlightClearTimer: ReturnType<typeof setTimeout> | null = null;

const VALID_HIGHLIGHT_FIELDS: AgentNotReadyReasonKey[] = ['summary_model', 'rerank_model', 'allowed_tools'];

const sectionForHighlightField = (field: AgentNotReadyReasonKey): string => {
  if (field === 'allowed_tools') return 'tools';
  return 'model';
};

const FIELD_FLASH_DURATION_MS = 2400;

const clearFieldHighlight = () => {
  if (highlightClearTimer) {
    clearTimeout(highlightClearTimer);
    highlightClearTimer = null;
  }
  highlightedField.value = null;
};

const applyInitialFieldHighlight = async (field: string) => {
  if (!VALID_HIGHLIGHT_FIELDS.includes(field as AgentNotReadyReasonKey)) return;

  const targetField = field as AgentNotReadyReasonKey;
  currentSection.value = sectionForHighlightField(targetField);

  await nextTick();
  await new Promise<void>((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
  });

  clearFieldHighlight();
  highlightedField.value = null;

  const wrapper = contentWrapperRef.value;
  const row = wrapper?.querySelector(`[data-agent-field="${targetField}"]`) as HTMLElement | null;
  if (row && wrapper) {
    const rowTop = row.offsetTop;
    const scrollTarget = rowTop - wrapper.clientHeight / 2 + row.clientHeight / 2;
    wrapper.scrollTo({ top: Math.max(0, scrollTarget), behavior: 'auto' });

    await nextTick();
    await new Promise<void>((resolve) => {
      requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
    });
  }

  highlightedField.value = targetField;

  if (row) {
    const focusTarget = row.querySelector('.t-input, .t-select-input, input, .t-checkbox') as HTMLElement | null;
    focusTarget?.focus({ preventScroll: true });
  }

  highlightClearTimer = setTimeout(() => {
    if (highlightedField.value === targetField) {
      highlightedField.value = null;
    }
    highlightClearTimer = null;
  }, FIELD_FLASH_DURATION_MS);
};

const onAgentEditorFocusSection = (event: Event) => {
  const section = (event as CustomEvent<{ section?: string }>).detail?.section
  if (!section) return
  const resolved = resolveEditorSection(section)
  if (navItems.value.some((item) => item.key === resolved)) {
    currentSection.value = resolved
  }
}

onMounted(() => {
  window.addEventListener(AGENT_EDITOR_FOCUS_SECTION_EVENT, onAgentEditorFocusSection)
})

onBeforeUnmount(() => {
  window.removeEventListener(AGENT_EDITOR_FOCUS_SECTION_EVENT, onAgentEditorFocusSection)
})

const saving = ref(false);
const editorInitializing = ref(false);
const allModels = ref<ModelConfig[]>([]);
const kbOptions = ref<{ label: string; value: string; type?: 'document' | 'faq'; count?: number; shared?: boolean; orgName?: string; ragEnabled?: boolean; wikiEnabled?: boolean; capabilities?: KBCapabilities }[]>([]);

// Agent type presets (shown only in smart-reasoning mode)
const agentTypePresets = ref<AgentTypePreset[]>([]);
// Agent system prompt template cache (used to resolve the actual text from system_prompt_id when switching agent types)
const agentSystemPromptTemplates = ref<PromptTemplate[]>([]);
const promptTemplates = ref<PromptTemplatesConfig | null>(null);
const intentPromptTemplates = ref<PromptTemplate[]>([]);
type McpSelectOption = { label: string; value: string; disabled?: boolean };

const mcpOptions = computed<McpSelectOption[]>(() => {
  const services = editorResources.mcpServices || [];
  const selectedIds = new Set<string>(formData.value.config.mcp_services || []);
  const serviceById = new Map(services.map((mcp) => [mcp.id, mcp]));
  const options: McpSelectOption[] = [];

  for (const mcp of services) {
    if (mcp.enabled) {
      options.push({ label: mcp.name, value: mcp.id });
    }
  }

  for (const id of selectedIds) {
    const mcp = serviceById.get(id);
    if (mcp && !mcp.enabled) {
      options.push({
        label: `${mcp.name} (${t('mcpSettings.disabled')})`,
        value: mcp.id,
        disabled: true,
      });
    } else if (!mcp) {
      options.push({
        label: t('agentEditor.mcp.unavailableService'),
        value: id,
        disabled: true,
      });
    }
  }

  return options;
});

const showMcpServiceSelect = computed(() =>
  mcpOptions.value.length > 0 || (formData.value.config.mcp_services?.length ?? 0) > 0,
);
const webSearchProviderList = ref<WebSearchProviderEntity[]>([]);
const skillCatalog = ref<SkillCatalogItem[]>([]);
const catalogReady = ref(false);
const installingCatalogId = ref('');
const skillsSelectionMode = ref<'all' | 'selected' | 'none'>('none');
const hostOnly = computed(() => hostSkillsOnly(
  deploymentCapabilities.isSupported('settings.sandbox.remote'),
  deploymentCapabilities.isSupported('settings.sandbox.host'),
));
// Where this agent's skills install and run.
const skillTargetId = computed(() =>
  hostOnly.value ? HOST_SKILL_TARGET_ID : (formData.value.config.sandbox_config_id || ''),
);
const hasSandboxSelected = computed(() => !!skillTargetId.value);
const canEnableSkills = computed(() =>
  hostOnly.value || hasSandboxSelected.value || namedSandboxConfigs().length === 1,
);
const canInstallSkills = computed(() => authStore.hasRole('admin'));

type CatalogSkillRow = SkillCatalogItem & {
  installed: boolean
  selectable: boolean
  installStatus: string
  installEnabled: boolean
  // The install on this sandbox is still on an archive the catalog has moved past.
  upgradable: boolean
  installVersion: string
  // Set while a newer install runs or after it failed: the sandbox still runs
  // the previous version, so the skill stays usable.
  servedNote: string
}

const catalogSkillRows = computed<CatalogSkillRow[]>(() => {
  const sandboxId = skillTargetId.value
  return skillCatalog.value.map((item) => {
    const inst = sandboxId
      ? (item.installations || []).find((row) => row.sandbox_config_id === sandboxId)
      : undefined
    const installStatus = inst?.status || ''
    const installEnabled = Boolean(inst?.enabled)
    const installed = Boolean(inst) && installStatus !== 'removed'
    const servedNote = inst ? servedPreviousText(t, inst) : ''
    const selectable = installEnabled && (installStatus === 'ready' || Boolean(servedNote))
    const upgradable = Boolean(inst && installUpgradable(item, inst))
    return {
      ...item, installed, selectable, installStatus, installEnabled,
      upgradable, installVersion: inst?.version || '', servedNote,
    }
  })
})

const showCatalogSkillList = computed(() =>
  skillsSelectionMode.value !== 'none'
  && hasSandboxSelected.value
  && catalogSkillRows.value.length > 0,
)

const skillsSelectionHint = computed(() => {
  if (hostOnly.value) {
    if (skillsSelectionMode.value === 'all') return t('agent.editor.hostSkillsAllListHint')
    if (skillsSelectionMode.value === 'selected') return t('agent.editor.hostSelectSkillsDesc')
    return t('agent.editor.hostSkillsSelectionDesc')
  }
  if (skillsSelectionMode.value === 'all') return t('agent.editor.skillsAllListHint')
  if (skillsSelectionMode.value === 'selected') return t('agent.editor.selectSkillsDesc')
  return t('agent.editor.skillsSelectionDesc')
})

const catalogSkillGroups = computed(() => {
  const ready = catalogSkillRows.value.filter((skill) => skill.selectable)
  const pending = catalogSkillRows.value.filter((skill) => !skill.selectable)
  const groups: { key: 'ready' | 'pending'; label: string; skills: CatalogSkillRow[] }[] = []
  if (ready.length) {
    groups.push({
      key: 'ready',
      label: t('agent.editor.skillsGroupAvailable'),
      skills: ready,
    })
  }
  if (pending.length) {
    groups.push({
      key: 'pending',
      label: t('agent.editor.skillsGroupUnavailable'),
      skills: pending,
    })
  }
  return groups
})

function skillStatusHint(skill: CatalogSkillRow): string {
  if (!skill.installed) return t('agent.editor.skillNotInstalled')
  if (skill.installStatus === 'installing') return t('settings.sandbox.skillStatusInstalling')
  if (skill.installStatus === 'failed') return t('settings.sandbox.skillStatusFailed')
  if (skill.installStatus === 'removing') return t('settings.sandbox.skillStatusRemoving')
  if (skill.installStatus === 'ready' && !skill.installEnabled) {
    return hostOnly.value ? t('agent.editor.hostSkillDisabled') : t('agent.editor.skillDisabledOnSandbox')
  }
  return t('agent.editor.skillNotReady')
}

function skillStatusIcon(skill: CatalogSkillRow): string {
  if (!skill.installed || skill.installStatus === 'failed') return 'download'
  if (skill.installStatus === 'installing' || skill.installStatus === 'removing') return 'refresh'
  if (skill.installStatus === 'ready' && !skill.installEnabled) return 'close-circle'
  return 'time'
}

function isSkillBusy(skill: CatalogSkillRow): boolean {
  return skill.installStatus === 'installing' || skill.installStatus === 'removing'
}

function canInstallSkillRow(skill: CatalogSkillRow): boolean {
  if (!canInstallSkills.value || !hasSandboxSelected.value) return false
  return !skill.installed || skill.installStatus === 'failed'
}

// Upgrading writes the sandbox image through the same admin-only catalog
// install, so it is offered, and even mentioned, only to those who can run it.
function canUpgradeSkillRow(skill: CatalogSkillRow): boolean {
  return canInstallSkills.value && hasSandboxSelected.value && skill.upgradable
}

// Installing the catalog version over what this sandbox has is an upgrade:
// over an outdated install, or over a failed upgrade whose previous version
// still runs. Only a skill the sandbox has never carried is a plain install.
function installsAnUpgrade(skill: CatalogSkillRow): boolean {
  return skill.upgradable || Boolean(skill.servedNote)
}

function skillUpgradeHint(skill: CatalogSkillRow): string {
  const versions = upgradeVersions(skill, { version: skill.installVersion })
  return versions
    ? t('settings.skills.upgradeFromTo', versions)
    : t('settings.skills.upgradeAvailable')
}

function namedSandboxConfigs(): SandboxConfigRecord[] {
  return chatResources.sandboxConfigs.filter((cfg) => isNamedSandboxBackend(cfg.sandbox_type))
}

function autoBindSoleSandbox() {
  if (hostOnly.value) return
  if (skillsSelectionMode.value === 'none') return
  if (formData.value.config.sandbox_config_id) return
  const configs = namedSandboxConfigs()
  if (configs.length === 1) {
    formData.value.config.sandbox_config_id = configs[0].id
  }
}

function openSkillSettings() {
  const configId = skillTargetId.value
  modalShell.requestClose(() => {
    void router.push(toolboxLocation('skills', configId || undefined))
  })
}

const showSkillProgress = ref(false)
const skillProgressRecord = ref<SandboxConfigRecord | null>(null)
const skillProgressId = ref('')
const skillProgressTitle = ref('')
const skillProgressDesc = computed(() => {
  const record = skillProgressRecord.value
  if (!record) return ''
  return t('settings.skills.manageDrawerDesc', { name: record.name })
})

function sandboxRecordById(configId: string): SandboxConfigRecord | undefined {
  if (hostOnly.value && configId === HOST_SKILL_TARGET_ID) return hostSkillTargetRecord(t('settings.skills.hostTarget'))
  return chatResources.sandboxConfigs.find((cfg) => cfg.id === configId)
}

function installOnCurrentSandbox(skill: CatalogSkillRow, configId: string) {
  return (skill.installations || []).find((row) => row.sandbox_config_id === configId)
}

async function openSkillInstallProgress(skill: CatalogSkillRow) {
  const configId = skillTargetId.value
  const record = sandboxRecordById(configId)
  if (!record) {
    openSkillSettings()
    return
  }
  let inst = installOnCurrentSandbox(skill, configId)
  if (!inst?.skill_id) {
    await syncInstalledSkills(true)
    const latest = catalogSkillRows.value.find((row) => row.id === skill.id)
    inst = latest ? installOnCurrentSandbox(latest, configId) : undefined
  }
  if (!inst?.skill_id) {
    openSkillSettings()
    return
  }
  skillProgressRecord.value = record
  skillProgressId.value = inst.skill_id
  skillProgressTitle.value = skill.name
  showSkillProgress.value = true
}

function onSkillProgressUpdated() {
  void syncInstalledSkills(true)
}

function onSkillProgressChanged() {
  void syncInstalledSkills(true)
}

function pruneSelectedSkills() {
  if (!catalogReady.value) return
  // A skill being upgraded is briefly not ready, and dropping it here would
  // silently unselect it for good once the agent is saved.
  const names = new Set(catalogSkillRows.value
    .filter((skill) => skill.selectable || (skill.installed && isSkillBusy(skill)))
    .map((skill) => skill.name))
  const selected: string[] = formData.value.config.selected_skills || []
  const kept = selected.filter((name: string) => names.has(name))
  if (kept.length !== selected.length) {
    formData.value.config.selected_skills = kept
  }
}

async function syncInstalledSkills(force = false) {
  autoBindSoleSandbox()
  const configId = skillTargetId.value
  // The editor only edits this workspace's agents, so the sandbox config is
  // local and needs no source-workspace scope.
  await editorResources.ensureSkills(configId, undefined, force)
  try {
    await editorResources.ensureSkillCatalog(force)
    skillCatalog.value = [...editorResources.skillCatalog]
    catalogReady.value = true
  } catch {
    catalogReady.value = false
  }
  pruneSelectedSkills()
}

async function installCatalogToCurrent(skill: CatalogSkillRow) {
  const configId = skillTargetId.value
  if (!configId || installingCatalogId.value) return
  installingCatalogId.value = skill.id
  const upgrading = installsAnUpgrade(skill)
  try {
    const res = await installSkillCatalog(skill.id, [configId])
    const failed = Object.keys(res?.data?.errors || {}).length
    if (failed > 0) {
      MessagePlugin.warning(t('settings.skills.installPartial', { failed }))
    } else {
      MessagePlugin.success(t(upgrading ? 'settings.skills.upgradeAccepted' : 'settings.skills.installAccepted'))
    }
    await syncInstalledSkills(true)
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('settings.sandbox.skillUploadFailed'))
  } finally {
    installingCatalogId.value = ''
  }
}
// The named sandbox backend configs in the workspace. Always includes the currently selected one, even if it was deleted;
// otherwise the dropdown would silently show "No sandbox" and hide that the agent points at a config that no longer exists.
const sandboxConfigOptions = computed(() => {
  const configs = chatResources.sandboxConfigs.filter((cfg) => isNamedSandboxBackend(cfg.sandbox_type));
  const selected = formData.value.config.sandbox_config_id;
  if (!selected || configs.some((cfg) => cfg.id === selected)) return configs;
  return [
    ...configs,
    { id: selected, name: t('agent.editor.sandboxBackendMissing'), sandbox_type: '' } as SandboxConfigRecord,
  ];
});
const backendLabel = (type: string) =>
  type ? t(`settings.sandbox.backends.${type}`) : t('common.error');

function sandboxTargetLine(cfg: SandboxConfigRecord): string {
  if (cfg.sandbox_type === 'docker') {
    return cfg.config?.docker?.image?.trim() || ''
  }
  const remote = cfg.config?.e2b || cfg.config?.cube
  const raw = remote?.api_url?.trim() || ''
  if (!raw) return ''
  try {
    return new URL(raw).host
  } catch {
    return raw
  }
}

const selectedSandboxSummary = computed(() => {
  const id = formData.value.config.sandbox_config_id
  const cfg = sandboxConfigOptions.value.find((item) => item.id === id)
  if (!cfg?.sandbox_type) return ''
  const parts = [backendLabel(cfg.sandbox_type), sandboxTargetLine(cfg)]
  const desc = cfg.description?.trim()
  if (desc) parts.push(desc)
  return parts.filter(Boolean).join(' · ')
})
// Storage engine availability (for choosing the image storage provider)
const storageEngineStatus = ref<StorageEngineStatusItem[]>([]);
const imageStorageOptions = computed(() => {
  const statusMap: Record<string, boolean> = {};
  for (const e of storageEngineStatus.value) {
    statusMap[e.name] = e.available;
  }
  return [
    { value: 'local', label: t('settings.storage.engineLocal'), disabled: false },
    { value: 'minio', label: 'MinIO', disabled: statusMap.minio === false },
    { value: 'cos', label: t('settings.storage.engineCos'), disabled: statusMap.cos === false },
    { value: 'tos', label: t('settings.storage.engineTos'), disabled: statusMap.tos === false },
    { value: 's3', label: 'Amazon S3', disabled: statusMap.s3 === false },
    { value: 'oss', label: t('settings.storage.engineOss'), disabled: statusMap.oss === false },
  ];
});

// System default config (for showing the default prompts of built-in agents)
// The default system prompt for Agent (smart-reasoning) mode. Picked directly from the prompt-templates
// agent_system_prompt array as the entry with mode==='rag' && default,
// the same data source as the backend's agent.GetProgressiveRAGSystemPrompt.
const defaultAgentSystemPrompt = ref('');
const defaultNormalSystemPrompt = ref('');  // default system prompt for normal mode (from the prompt-templates default entry)
const defaultContextTemplate = ref('');
const defaultRewritePromptSystem = ref('');
const defaultRewritePromptUser = ref('');
const defaultFallbackPrompt = ref('');
const defaultFallbackResponse = ref('');
// Default retrieval parameters
const defaultEmbeddingTopK = ref(10);
const defaultKeywordThreshold = ref(0.3);
const defaultVectorThreshold = ref(0.5);
const defaultRerankTopK = ref(5);
const defaultRerankThreshold = ref(0.5);
const defaultQuickAnswerMaxCompletionTokens = 2048;
const defaultSmartReasoningMaxCompletionTokens = 4096;
const defaultSandboxWriteMaxCompletionTokens = 24576;
const defaultTemperature = ref(0.7);

const defaultMaxCompletionTokensFor = (mode: string, sandboxConfigId?: string) => {
  if (mode === 'smart-reasoning') {
    return sandboxConfigId
      ? defaultSandboxWriteMaxCompletionTokens
      : defaultSmartReasoningMaxCompletionTokens;
  }
  return defaultQuickAnswerMaxCompletionTokens;
};

// Knowledge base tools (for seeding default tools when watch(hasKnowledgeBase) goes from "none" to "some")
const knowledgeBaseTools = ['search_knowledge', 'read_document', 'list_documents'];

// Wiki read tools (for seeding default tools when watch(agentMode) switches to smart-reasoning)
const wikiReadTools = ['wiki_search', 'wiki_read_page', 'read_document', 'wiki_flag_issue'];

// Initialization flag, so watches do not add tools automatically during initialization
const isInitializing = ref(false);

// Knowledge base selection mode: all=all, selected=specific, none=not used
const kbSelectionMode = ref<'all' | 'selected' | 'none'>('none');

// MCP service selection mode: all=all, selected=specific, none=not used
const mcpSelectionMode = ref<'all' | 'selected' | 'none'>('none');

// Available tools (kept in sync with the backend internal/agent/tools/definitions.go)
// group decides the UI grouping: base / rag / wiki_read / wiki_edit / wiki_issue / data
// danger: destructive write tools, flagged prominently in the UI
// The tools' KB capability dependencies are declared once in `@/utils/tool-capabilities`;
// `availableTools` reads them through `evaluateToolRequirement`, so they are not maintained here twice.
const allTools = computed(() => [
  // Basic thinking
  { value: 'thinking', label: t('agentEditor.tools.thinking'), description: t('agentEditor.tools.thinkingDesc'), group: 'base' },
  { value: 'todo_write', label: t('agentEditor.tools.todoWrite'), description: t('agentEditor.tools.todoWriteDesc'), group: 'base' },
  // Knowledge base search / document reading (the old grep_chunks / knowledge_search / list_knowledge_chunks /
  // get_document_info were merged; normalizeLegacyToolNames maps them to the new names when an old config loads)
  { value: 'search_knowledge', label: t('agentEditor.tools.searchKnowledge'), description: t('agentEditor.tools.searchKnowledgeDesc'), group: 'rag' },
  { value: 'read_document', label: t('agentEditor.tools.readDocument'), description: t('agentEditor.tools.readDocumentDesc'), group: 'rag' },
  { value: 'list_documents', label: t('agentEditor.tools.listDocuments'), description: t('agentEditor.tools.listDocumentsDesc'), group: 'rag' },
  { value: 'query_knowledge_graph', label: t('agentEditor.tools.queryGraph'), description: t('agentEditor.tools.queryGraphDesc'), group: 'rag' },
  { value: 'database_query', label: t('agentEditor.tools.dbQuery'), description: t('agentEditor.tools.dbQueryDesc'), group: 'rag' },
  // Wiki read (read, search, flag issues)
  { value: 'wiki_search', label: t('agentEditor.tools.wikiSearch'), description: t('agentEditor.tools.wikiSearchDesc'), group: 'wiki_read' },
  { value: 'wiki_read_page', label: t('agentEditor.tools.wikiReadPage'), description: t('agentEditor.tools.wikiReadPageDesc'), group: 'wiki_read' },
  { value: 'wiki_flag_issue', label: t('agentEditor.tools.wikiFlagIssue'), description: t('agentEditor.tools.wikiFlagIssueDesc'), group: 'wiki_read' },
  // Wiki edit (modifies wiki content directly)
  { value: 'wiki_write_page', label: t('agentEditor.tools.wikiWritePage'), description: t('agentEditor.tools.wikiWritePageDesc'), group: 'wiki_edit', danger: true },
  { value: 'wiki_replace_text', label: t('agentEditor.tools.wikiReplaceText'), description: t('agentEditor.tools.wikiReplaceTextDesc'), group: 'wiki_edit', danger: true },
  { value: 'wiki_rename_page', label: t('agentEditor.tools.wikiRenamePage'), description: t('agentEditor.tools.wikiRenamePageDesc'), group: 'wiki_edit', danger: true },
  { value: 'wiki_delete_page', label: t('agentEditor.tools.wikiDeletePage'), description: t('agentEditor.tools.wikiDeletePageDesc'), group: 'wiki_edit', danger: true },
  // Wiki inspection
  { value: 'wiki_read_issue', label: t('agentEditor.tools.wikiReadIssue'), description: t('agentEditor.tools.wikiReadIssueDesc'), group: 'wiki_issue' },
  { value: 'wiki_update_issue', label: t('agentEditor.tools.wikiUpdateIssue'), description: t('agentEditor.tools.wikiUpdateIssueDesc'), group: 'wiki_issue' },
  // Data analysis
  { value: 'data_analysis', label: t('agentEditor.tools.dataAnalysis'), description: t('agentEditor.tools.dataAnalysisDesc'), group: 'data' },
  { value: 'data_schema', label: t('agentEditor.tools.dataSchema'), description: t('agentEditor.tools.dataSchemaDesc'), group: 'data' },
]);

// Tool group metadata
const toolGroups = computed(() => [
  { key: 'base', label: t('agentEditor.tools.groupBase') },
  { key: 'rag', label: t('agentEditor.tools.groupRag') },
  { key: 'wiki_read', label: t('agentEditor.tools.groupWikiRead') },
  { key: 'wiki_edit', label: t('agentEditor.tools.groupWikiEdit') },
  { key: 'wiki_issue', label: t('agentEditor.tools.groupWikiIssue') },
  { key: 'data', label: t('agentEditor.tools.groupData') },
]);

// Knowledge base groups: mine vs shared
const myKbOptions = computed(() => kbOptions.value.filter(kb => !kb.shared));
const sharedKbOptions = computed(() => kbOptions.value.filter(kb => kb.shared));

// Whether there is knowledge base capability, computed from the knowledge base config
const hasKnowledgeBase = computed(() => {
  return kbSelectionMode.value !== 'none';
});

const showRerankModelField = computed(() => {
  if (!isAgentMode.value) return hasKnowledgeBase.value;
  return hasKnowledgeBase.value || agentRequiresRerankModel(formData.value.config);
});

// The knowledge bases in the agent's scope under the current config
// Note: the user may have chosen knowledge_bases (per base) or knowledge_ids (per document);
// this is only for tool availability in the UI, so it counts per base
const kbsInScope = computed(() => {
  if (kbSelectionMode.value === 'none') return [];
  if (kbSelectionMode.value === 'all') return kbOptions.value;
  const selectedIds = formData.value.config.knowledge_bases || [];
  return kbOptions.value.filter(kb => selectedIds.includes(kb.value));
});

// Whether at least one knowledge base has RAG capability enabled (vector or keyword)
const hasRagKnowledgeBase = computed(() => {
  return kbsInScope.value.some(kb => kb.ragEnabled);
});

// Whether at least one knowledge base has wiki capability enabled
const hasWikiKnowledgeBase = computed(() => {
  return kbsInScope.value.some(kb => kb.wikiEnabled);
});

// RAG/wiki knowledge base counts in scope (for the top status bar)
const ragKbCount = computed(() => kbsInScope.value.filter(kb => kb.ragEnabled).length);
const wikiKbCount = computed(() => kbsInScope.value.filter(kb => kb.wikiEnabled).length);

// Detect whether the selected knowledge bases include the FAQ type
const hasFaqKnowledgeBase = computed(() => {
  if (kbSelectionMode.value === 'none') return false;
  if (kbSelectionMode.value === 'all') {
    // All knowledge bases mode: check whether any knowledge base is of FAQ type
    return kbOptions.value.some(kb => kb.type === 'faq');
  }
  // Specific knowledge bases mode: check whether a selected knowledge base is of FAQ type
  const selectedKbIds = formData.value.config.knowledge_bases || [];
  return kbOptions.value.some(kb => selectedKbIds.includes(kb.value) && kb.type === 'faq');
});

// Aggregate the "in-scope KB capabilities" into one ScopeCapabilities object for
// `evaluateToolRequirement` to decide; every availability hint in the UI should come from here.
const scopeCapabilities = computed<ScopeCapabilities>(() => {
  const scope: ScopeCapabilities = { vector: false, keyword: false, wiki: false, graph: false, faq: false };
  for (const kb of kbsInScope.value) {
    const caps = kb.capabilities;
    if (caps) {
      if (caps.vector) scope.vector = true;
      if (caps.keyword) scope.keyword = true;
      if (caps.wiki) scope.wiki = true;
      if (caps.graph) scope.graph = true;
      if (caps.faq) scope.faq = true;
    } else {
      // Backward compatibility: while capabilities are not loaded yet, infer from ragEnabled/wikiEnabled
      if (kb.ragEnabled) { scope.vector = true; scope.keyword = true; }
      if (kb.wikiEnabled) scope.wiki = true;
      if (kb.type === 'faq') scope.faq = true;
    }
  }
  return scope;
});

// Map the missKind returned by evaluateToolRequirement to i18n text.
// The new needsGraph / needsFaq reuse the requiresRagKb text ("Requires a RAG knowledge base") for now;
// separate i18n keys can be added later as needed.
const missKindToReason = (kind: RequirementMissKind): string | undefined => {
  switch (kind) {
    case 'needsKb': return t('agentEditor.tools.requiresKb');
    case 'needsWiki': return t('agentEditor.tools.requiresWikiKb');
    case 'needsRag':
    case 'needsGraph':
    case 'needsFaq': return t('agentEditor.tools.requiresRagKb');
    case 'none':
    default: return undefined;
  }
};

const availableTools = computed(() => {
  const scope = scopeCapabilities.value;
  const hasAnyKb = hasKnowledgeBase.value;
  return allTools.value.map(tool => {
    const { ok, missKind } = evaluateToolRequirement(tool.value, scope, hasAnyKb);
    return {
      ...tool,
      disabled: !ok,
      disabledReason: ok ? undefined : missKindToReason(missKind),
    };
  });
});

// The tool list sliced by group, for grouped rendering in the template
const groupedAvailableTools = computed(() => {
  const map: Record<string, typeof availableTools.value> = {};
  for (const tool of availableTools.value) {
    const g = tool.group || 'base';
    if (!map[g]) map[g] = [];
    map[g].push(tool);
  }
  return toolGroups.value
    .map(g => ({
      ...g,
      tools: map[g.key] || [],
    }))
    .filter(g => g.tools.length > 0);
});

// ==================== Effective tools preview ====================
// The set of tools the agent can actually use at runtime (preview only)
// Rules: filtered from allowed_tools
//   1) Checked tools that lack the matching capability (no KB / no wiki-capable KB) are grayed out/hidden
//   2) Whether checked or not, web_search / web_fetch appear with web_search_enabled
//   3) When kb_selection_mode === 'none', the RAG/wiki tools all count as unavailable
const effectiveTools = computed(() => {
  const chosen = new Set(formData.value.config.allowed_tools || []);
  const items: Array<{ value: string; label: string; reason?: string; active: boolean }> = [];
  for (const tool of availableTools.value) {
    const picked = chosen.has(tool.value);
    if (!picked) continue;
    if (tool.disabled) {
      items.push({ value: tool.value, label: tool.label, active: false, reason: tool.disabledReason });
    } else {
      items.push({ value: tool.value, label: tool.label, active: true });
    }
  }
  if (formData.value.config.web_search_enabled) {
    items.push({ value: 'web_search', label: t('agentEditor.tools.webSearch'), active: true });
    items.push({ value: 'web_fetch', label: t('agentEditor.tools.webFetch'), active: true });
  }
  return items;
});

// Number of tools that are checked but cannot take effect under the current config (for the top status hint)
const inactiveToolCount = computed(() => effectiveTools.value.filter(i => !i.active).length);

// Available file types
const availableFileTypes = [
  { value: 'pdf', label: 'PDF', description: t('agentEditor.fileTypes.pdf') },
  { value: 'docx', label: 'Word', description: t('agentEditor.fileTypes.word') },
  { value: 'txt', label: t('agentEditor.fileTypes.textLabel'), description: t('agentEditor.fileTypes.text') },
  { value: 'md', label: 'Markdown', description: t('agentEditor.fileTypes.markdown') },
  { value: 'csv', label: 'CSV', description: t('agentEditor.fileTypes.csv') },
  { value: 'xlsx', label: 'Excel', description: t('agentEditor.fileTypes.excel') },
  { value: 'jpg', label: t('agentEditor.fileTypes.imageLabel'), description: t('agentEditor.fileTypes.image') },
];

// Placeholders, fetched from the API
const placeholderData = ref<{
  system_prompt: PlaceholderDefinition[];
  agent_system_prompt: PlaceholderDefinition[];
  context_template: PlaceholderDefinition[];
  rewrite_system_prompt: PlaceholderDefinition[];
  rewrite_prompt: PlaceholderDefinition[];
  fallback_prompt: PlaceholderDefinition[];
}>({
  system_prompt: [],
  agent_system_prompt: [],
  context_template: [],
  rewrite_system_prompt: [],
  rewrite_prompt: [],
  fallback_prompt: [],
});

// System prompt placeholders (chosen dynamically by mode)
const availablePlaceholders = computed(() => {
  return isAgentMode.value ? placeholderData.value.agent_system_prompt : placeholderData.value.system_prompt;
});

// Context template placeholders
const contextTemplatePlaceholders = computed(() => placeholderData.value.context_template);

// Rewrite system prompt placeholders
const rewriteSystemPlaceholders = computed(() => placeholderData.value.rewrite_system_prompt);

// Rewrite user prompt placeholders
const rewritePlaceholders = computed(() => placeholderData.value.rewrite_prompt);

// Fallback prompt placeholders
const fallbackPlaceholders = computed(() => placeholderData.value.fallback_prompt);

const promptTextareaRef = ref<any>(null);
const showPlaceholderPopup = ref(false);
const selectedPlaceholderIndex = ref(0);
const placeholderPrefix = ref('');
const popupStyle = ref({ top: '0px', left: '0px' });
let placeholderPopupTimer: any = null;

// Context template placeholder state
const contextTemplateTextareaRef = ref<any>(null);
const showContextPlaceholderPopup = ref(false);
const selectedContextPlaceholderIndex = ref(0);
const contextPlaceholderPrefix = ref('');
const contextPopupStyle = ref({ top: '0px', left: '0px' });
let contextPlaceholderPopupTimer: any = null;

// Intent prompt editing state
const selectedIntent = ref('');
const intentEditorValue = ref('');
const intentPromptsSyncing = ref(false);
const intentPromptTextareaRef = ref<any>(null);

// Generic placeholder popup state (for the rewrite and fallback prompts)
interface PlaceholderPopupState {
  show: boolean;
  selectedIndex: number;
  prefix: string;
  style: { top: string; left: string };
  timer: any;
  fieldKey: string;
  placeholders: PlaceholderDefinition[];
}

const intentPromptPopup = ref<PlaceholderPopupState>({
  show: false, selectedIndex: 0, prefix: '', style: { top: '0px', left: '0px' }, timer: null, fieldKey: 'intent_prompt', placeholders: []
});

const rewriteSystemPopup = ref<PlaceholderPopupState>({
  show: false, selectedIndex: 0, prefix: '', style: { top: '0px', left: '0px' }, timer: null, fieldKey: 'rewrite_prompt_system', placeholders: []
});
const rewriteUserPopup = ref<PlaceholderPopupState>({
  show: false, selectedIndex: 0, prefix: '', style: { top: '0px', left: '0px' }, timer: null, fieldKey: 'rewrite_prompt_user', placeholders: []
});
const fallbackPromptPopup = ref<PlaceholderPopupState>({
  show: false, selectedIndex: 0, prefix: '', style: { top: '0px', left: '0px' }, timer: null, fieldKey: 'fallback_prompt', placeholders: []
});

const rewriteSystemTextareaRef = ref<any>(null);
const rewriteUserTextareaRef = ref<any>(null);
const fallbackPromptTextareaRef = ref<any>(null);

const navItems = computed(() => {
  const items: { key: string; icon: string; label: string; badge?: number }[] = [
    { key: 'basic', icon: 'info-circle', label: t('agent.editor.basicInfo') },
    { key: 'prompts', icon: 'file-paste', label: t('agent.editor.promptsConfig'), badge: promptNavItems.value.length > 1 ? promptNavItems.value.length : undefined },
    { key: 'model', icon: 'control-platform', label: t('agent.editor.modelConfig') },
    { key: 'suggestions', icon: 'help-circle', label: t('agentEditor.questionSuggestions.navLabel') },
  ];
  // Multi-turn conversation (both modes need it: in Agent mode it explains automatic history management and keeping retrieval results)
  items.push({ key: 'conversation', icon: 'chat', label: t('agent.editor.conversationSettings') });
  // Knowledge base and retrieval
  items.push({ key: 'knowledge', icon: 'folder', label: t('agent.editor.knowledgeConfig') });
  // The retrieval strategy only applies to the normal-mode Q&A flow; Agent mode's retrieval tools do not read these settings,
  // so showing them would only let users tune parameters with no effect.
  if (hasKnowledgeBase.value && !isAgentMode.value) {
    items.push({ key: 'retrieval', icon: 'search', label: t('agent.editor.retrievalStrategy') });
  }
  items.push({ key: 'websearch', icon: 'internet', label: t('agent.editor.webSearchConfig') });
  items.push({ key: 'multimodal', icon: 'attach', label: t('agentEditor.imageUpload.navLabel') });
  // Agent mode capabilities
  if (isAgentMode.value) {
    items.push({ key: 'tools', icon: 'tools', label: t('agent.editor.toolsConfig') });
    items.push({ key: 'mcp', icon: 'server', label: t('agentEditor.mcp.label') });
    items.push({ key: 'skills', icon: SKILL_ICON, label: t('agent.editor.skillsConfig') });
  }
  // Publishing (edit mode only)
  if (editorMode.value === 'edit' && editorAgent.value?.id && !editorAgent.value?.is_builtin && !authStore.isLiteMode) {
    items.push({ key: 'share', icon: 'share', label: t('knowledgeEditor.sidebar.share') });
  }
  return items;
});

// Left nav groups (modeled on the grouping in "Avatar → Settings")
const navGroups = computed(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]));
  const pickItems = (keys: string[]) =>
    keys.map((key) => itemMap.get(key)).filter(Boolean) as typeof navItems.value;
  return [
    {
      key: 'basic',
      label: t('agentEditor.navGroups.basic'),
      items: pickItems(['basic', 'prompts', 'model', 'conversation', 'suggestions']),
    },
    {
      key: 'knowledge',
      label: t('agentEditor.navGroups.knowledge'),
      items: pickItems(['knowledge', 'retrieval', 'websearch']),
    },
    {
      key: 'capability',
      label: t('agentEditor.navGroups.capability'),
      items: pickItems(['multimodal', 'tools', 'mcp', 'skills']),
    },
    {
      key: 'integration',
      label: t('agentEditor.navGroups.integration'),
      items: pickItems(['share']),
    },
  ].filter((group) => group.items.length > 0);
});

// Initial data
const defaultFormData = {
  name: '',
  description: '',
  is_builtin: false,
  config: {
    // Basic settings
    agent_mode: 'smart-reasoning' as 'quick-answer' | 'smart-reasoning',
    system_prompt: '',
    context_template: '',
    // Model settings
    model_id: '',
    rerank_model_id: '',
    temperature: 0.7,
    max_completion_tokens: 0,
    thinking: false, // thinking mode disabled by default
    reasoning_effort: 'off', // reasoning effort; kept in sync with the thinking boolean
    citation_enabled: true, // cite knowledge base/web sources by default
    // Agent mode settings
    max_iterations: 10,
    llm_call_timeout: 120,  // 120 seconds
    allowed_tools: [] as string[],
    reflection_enabled: false,
    // MCP service settings
    mcp_selection_mode: 'none' as 'all' | 'selected' | 'none',
    mcp_services: [] as string[],
    // Wait timeout (seconds) when an OAuth authorization fires during a chat, default 600
    mcp_auth_wait_timeout: 600,
    // Skills settings
    skills_selection_mode: 'none' as 'all' | 'selected' | 'none',
    selected_skills: [] as string[],
    // Which workspace sandbox config skill scripts run on. Empty disables script execution.
    sandbox_config_id: '' as string,
    // Knowledge base settings: a new agent defaults to "All knowledge bases",
    // so users can start without first checking KBs; change to "selected" / "none" as needed.
    kb_selection_mode: 'all' as 'all' | 'selected' | 'none',
    knowledge_bases: [] as string[],
    retrieve_kb_only_when_mentioned: false,
    // Type preset under Smart Reasoning: a new agent defaults to RAG Q&A (the most common case).
    // When editing an existing agent, the agent_type it saved overrides this.
    agent_type: 'rag-qa' as AgentType,
    system_prompt_id: '' as string,
    // Attachment upload settings
    image_upload_enabled: false,
    vlm_model_id: '',
    image_storage_provider: '',
    // Attachment image understanding / scanned-document OCR switch (off by default, to avoid longer parsing)
    attachment_image_understanding: false,
    // Max OCR pages for scanned documents (0 = use the global default)
    attachment_ocr_max_pages: 0,
    // Longest time (seconds) a single Q&A turn waits for attachment parsing to finish (0 = use the global default)
    attachment_parse_wait_timeout_sec: 0,
    // Chat attachment parsing engine strategy (engine chosen per file type)
    chat_parser_engine_rules: [] as ParserEngineRule[],
    // File type limits
    supported_file_types: [] as string[],
    // Data analysis stage switch (off by default, to avoid an extra LLM call generating SQL on normal Q&A)
    data_analysis_enabled: false,
    // FAQ strategy settings
    faq_priority_enabled: true, // whether the FAQ priority strategy is enabled
    faq_direct_answer_threshold: 0.9, // FAQ direct answer threshold (above this similarity the FAQ answer is used directly)
    faq_score_boost: 1.2, // FAQ score boost factor
    // Web search settings
    web_search_enabled: false,
    web_search_max_results: 5,
    // Multi-turn conversation settings
    multi_turn_enabled: false,
    history_turns: 5,
    retain_retrieval_history: false,
    // Long-term memory: follows the workspace setting by default. Writing true is the same as omitting it; only false
    // makes this agent alone skip reading memory.
    memory_enabled: true,
    // Retrieval strategy settings
    embedding_top_k: 10,
    keyword_threshold: 0.3,
    vector_threshold: 0.5,
    rerank_top_k: 5,
    rerank_threshold: 0.5,
    // Advanced settings (normal mode)
    enable_query_expansion: true,
    enable_rewrite: true,
    query_understand_model_id: '',
    rewrite_prompt_system: '',
    rewrite_prompt_user: '',
    fallback_strategy: 'model' as 'fixed' | 'model',
    fallback_response: '',
    fallback_prompt: '',
    question_suggestions: {
      starters: {
        enabled: true,
        mode: 'hybrid' as 'curated' | 'knowledge' | 'hybrid',
        items: [] as string[],
        count: 6,
      },
      follow_ups: {
        enabled: false,
        mode: 'hybrid' as 'generated' | 'knowledge' | 'hybrid',
        count: 3,
        model_id: '',
        additional_instruction: '',
        categories: ['clarify', 'deepen', 'action'] as Array<'clarify' | 'deepen' | 'action'>,
        max_context_turns: 2,
        suppress_on_fallback: true,
        suppress_when_answer_asks_question: true,
        knowledge_fallback: true,
        allow_regenerate: false,
      },
    },
    // Deprecated fields (kept for compatibility)
    welcome_message: '',
  }
};

const formData = ref(JSON.parse(JSON.stringify(defaultFormData)));

const starterSuggestionModeOptions = computed(() => [
  { value: 'curated', label: t('agentEditor.questionSuggestions.modeCurated') },
  { value: 'knowledge', label: t('agentEditor.questionSuggestions.modeKnowledge') },
  { value: 'hybrid', label: t('agentEditor.questionSuggestions.modeHybrid') },
]);
const followUpSuggestionModeOptions = computed(() => [
  { value: 'generated', label: t('agentEditor.questionSuggestions.modeGenerated') },
  { value: 'knowledge', label: t('agentEditor.questionSuggestions.modeKnowledge') },
  { value: 'hybrid', label: t('agentEditor.questionSuggestions.modeHybrid') },
]);
const followUpCategoryOptions = computed(() => [
  { value: 'clarify', label: t('agentEditor.questionSuggestions.categoryClarify') },
  { value: 'deepen', label: t('agentEditor.questionSuggestions.categoryDeepen') },
  { value: 'action', label: t('agentEditor.questionSuggestions.categoryAction') },
]);

const addStarterSuggestion = () => {
  const items = formData.value.config.question_suggestions.starters.items;
  if (items.length < 8) items.push('');
};

const removeStarterSuggestion = (index: number) => {
  formData.value.config.question_suggestions.starters.items.splice(index, 1);
};

const applyDefaultModelsIfEmpty = () => {
  if (props.mode !== 'create' || !formData.value) return
  const chatModelId = selectInitialModelId(allModels.value, 'KnowledgeQA')
  const rerankModelId = selectInitialModelId(allModels.value, 'Rerank')
  if (!formData.value.config.model_id && chatModelId) {
    formData.value.config.model_id = chatModelId
  }
  if (!formData.value.config.rerank_model_id && rerankModelId) {
    formData.value.config.rerank_model_id = rerankModelId
  }
}

const agentMode = computed({
  get: () => formData.value.config.agent_mode,
  set: (val: 'quick-answer' | 'smart-reasoning') => { formData.value.config.agent_mode = val; }
});

const isAgentMode = computed(() => agentMode.value === 'smart-reasoning');

const effectiveDefaultMaxCompletionTokens = computed(() =>
  defaultMaxCompletionTokensFor(agentMode.value, formData.value.config.sandbox_config_id),
);

const maxCompletionTokensMode = computed({
  get: () => (formData.value.config.max_completion_tokens > 0 ? 'custom' : 'default'),
  set: (mode: 'default' | 'custom') => {
    if (mode === 'default') {
      formData.value.config.max_completion_tokens = 0;
      return;
    }
    if (!formData.value.config.max_completion_tokens) {
      formData.value.config.max_completion_tokens = effectiveDefaultMaxCompletionTokens.value;
    }
  },
});

const lastFiniteMaxIterations = ref(10);
const maxIterationsMode = computed({
  get: () => (formData.value.config.max_iterations < 0 ? 'unlimited' : 'limit'),
  set: (mode: 'limit' | 'unlimited') => {
    if (mode === 'unlimited') {
      if (formData.value.config.max_iterations > 1) {
        lastFiniteMaxIterations.value = formData.value.config.max_iterations;
      }
      formData.value.config.max_iterations = -1;
      return;
    }
    const restored = lastFiniteMaxIterations.value > 1 ? lastFiniteMaxIterations.value : 10;
    formData.value.config.max_iterations = restored;
  },
});

const currentIntentTemplate = computed(() =>
  intentPromptTemplates.value.find((template) => template.id === selectedIntent.value),
);

const currentIntentTemplateDesc = computed(() =>
  currentIntentTemplate.value?.description || '',
);

const isIntentCustomized = (intentId: string) => {
  const overrides = formData.value.config.intent_prompts || {};
  const override = overrides[intentId];
  if (!override?.trim()) return false;
  const template = intentPromptTemplates.value.find((item) => item.id === intentId);
  return override.trim() !== (template?.content || '').trim();
};

const activePromptAnchor = ref('system');

const hasAnyIntentCustomized = computed(() =>
  intentPromptTemplates.value.some((item) => isIntentCustomized(item.id)),
);

// In Agent mode this group only has "Turns kept" (plus retrieval retention when there is a knowledge base),
// and the default text's "Question rewrite" is not shown, so the text is split by mode.
const conversationSectionDesc = computed(() =>
  isAgentMode.value
    ? t('agentEditor.desc.conversationSectionAgent')
    : t('agentEditor.desc.conversationSection'),
);

const showRewritePrompts = computed(() =>
  !isAgentMode.value
  && formData.value.config.multi_turn_enabled
  && formData.value.config.enable_rewrite,
);

const promptNavItems = computed(() => {
  type PromptNavItem = { key: string; label: string; customized?: boolean };
  const items: PromptNavItem[] = [
    {
      key: 'system',
      label: t('agentEditor.promptNav.system'),
      customized: !!formData.value.config.system_prompt?.trim(),
    },
  ];
  if (!isAgentMode.value) {
    items.push({
      key: 'context',
      label: t('agentEditor.promptNav.context'),
      customized: !!formData.value.config.context_template?.trim(),
    });
    items.push({
      key: 'intent',
      label: t('agentEditor.promptNav.intent'),
      customized: hasAnyIntentCustomized.value,
    });
    if (showRewritePrompts.value) {
      items.push(
        {
          key: 'rewrite-system',
          label: t('agentEditor.promptNav.rewriteSystem'),
          customized: !!formData.value.config.rewrite_prompt_system?.trim(),
        },
        {
          key: 'rewrite-user',
          label: t('agentEditor.promptNav.rewriteUser'),
          customized: !!formData.value.config.rewrite_prompt_user?.trim(),
        },
      );
    }
    if (hasKnowledgeBase.value) {
      items.push({
        key: 'fallback',
        label: t('agentEditor.promptNav.fallback'),
      });
    }
  }
  return items;
});

const syncActivePromptAnchor = () => {
  const items = promptNavItems.value;
  if (!items.length) return;
  if (!items.some((item) => item.key === activePromptAnchor.value)) {
    activePromptAnchor.value = items[0].key;
  }
};

watch(promptNavItems, syncActivePromptAnchor);

watch(currentSection, (section) => {
  if (section === 'prompts') {
    syncActivePromptAnchor();
  }
});

const agentIMChannelCount = ref(0);
const agentEmbedChannelCount = ref(0);

async function loadAgentIntegrationCounts(agentId: string) {
  try {
    const [imResp, embedResp] = await Promise.all([
      listIMChannels(agentId),
      listEmbedChannels(agentId),
    ]);
    agentIMChannelCount.value = imResp?.data?.length ?? 0;
    agentEmbedChannelCount.value = embedResp?.data?.length ?? 0;
  } catch {
    agentIMChannelCount.value = 0;
    agentEmbedChannelCount.value = 0;
  }
}

function gotoIntegrations(tab: 'im' | 'embed') {
  const agentId = editorAgent.value?.id;
  if (!agentId) return;
  handleClose();
  router.push({ path: '/platform/settings', query: { section: integrationSectionKey(tab), agentId } });
}

const filteredIntentPlaceholders = computed(() => {
  if (!intentPromptPopup.value.prefix) {
    return placeholderData.value.system_prompt;
  }
  const prefix = intentPromptPopup.value.prefix.toLowerCase();
  return placeholderData.value.system_prompt.filter(p => p.name.toLowerCase().startsWith(prefix));
});

const syncIntentEditorFromSelection = () => {
  const key = selectedIntent.value;
  if (!key) {
    intentEditorValue.value = '';
    return;
  }
  const overrides = formData.value.config.intent_prompts || {};
  intentEditorValue.value = overrides[key] ?? currentIntentTemplate.value?.content ?? '';
};

watch(selectedIntent, () => {
  intentPromptPopup.value.show = false;
  intentPromptPopup.value.prefix = '';
  syncIntentEditorFromSelection();
});

watch(
  () => intentPromptTemplates.value,
  (templates) => {
    if (!selectedIntent.value && templates.length > 0) {
      selectedIntent.value = templates[0].id;
    } else if (selectedIntent.value) {
      syncIntentEditorFromSelection();
    }
  },
  { immediate: true },
);

watch(intentEditorValue, (value) => {
  const key = selectedIntent.value;
  if (!key || intentPromptsSyncing.value) return;
  const defaultContent = currentIntentTemplate.value?.content || '';
  const next = value.trim();
  if (!next || next === defaultContent.trim()) {
    if (formData.value.config.intent_prompts) {
      const { [key]: _removed, ...rest } = formData.value.config.intent_prompts;
      if (Object.keys(rest).length === 0) {
        delete formData.value.config.intent_prompts;
      } else {
        formData.value.config.intent_prompts = rest;
      }
    }
    return;
  }
  formData.value.config.intent_prompts = {
    ...(formData.value.config.intent_prompts || {}),
    [key]: value,
  };
});

watch(
  () => formData.value.config.intent_prompts,
  () => {
    intentPromptsSyncing.value = true;
    syncIntentEditorFromSelection();
    intentPromptsSyncing.value = false;
  },
  { deep: true },
);

const resetCurrentIntentPrompt = () => {
  const key = selectedIntent.value;
  if (!key || !formData.value.config.intent_prompts) return;
  const { [key]: _removed, ...rest } = formData.value.config.intent_prompts;
  if (Object.keys(rest).length === 0) {
    delete formData.value.config.intent_prompts;
  } else {
    formData.value.config.intent_prompts = rest;
  }
  syncIntentEditorFromSelection();
};

// ============================================================================
// Agent type presets (visible only in smart-reasoning mode)
// Choosing a type fills system_prompt_id / allowed_tools and so on automatically;
// choosing "custom" or a type with no matching preset overrides nothing.
// ============================================================================

const agentType = computed({
  get: () => (formData.value.config.agent_type as AgentType) || 'custom',
  set: (val: AgentType) => { formData.value.config.agent_type = val; },
});

// The currently active preset object (for KB filtering / UI badges)
const activeAgentTypePreset = computed<AgentTypePreset | null>(() => {
  if (!isAgentMode.value) return null;
  const id = agentType.value;
  if (!id || id === 'custom') return null;
  return agentTypePresets.value.find(p => p.id === id) || null;
});

// Pick the i18n label for the current locale
const agentTypePresetLabel = (p: AgentTypePreset): string => {
  const locale = i18nLocale.value || 'default';
  return p.i18n?.[locale]?.label || p.i18n?.default?.label || p.id;
};
const agentTypePresetDescription = (p: AgentTypePreset): string => {
  const locale = i18nLocale.value || 'default';
  return p.i18n?.[locale]?.description || p.i18n?.default?.description || '';
};

// Options data for t-select: label goes to TDesign itself (for the selected display), desc goes through the custom option slot
const agentTypeSelectOptions = computed(() => {
  return agentTypePresets.value.map(p => ({
    value: p.id,
    label: agentTypePresetLabel(p),
    desc: agentTypePresetDescription(p),
  }));
});

// Generate a default name "My <label>" for each preset, so the user can save in one click
// The custom type's default name is empty (the user picks one)
const getPresetDefaultName = (preset: AgentTypePreset | null): string => {
  if (!preset || preset.id === 'custom') return '';
  return t('agentEditor.agentType.defaultNamePattern', { label: agentTypePresetLabel(preset) });
};
const getPresetDefaultDescription = (preset: AgentTypePreset | null): string => {
  if (!preset) return '';
  return agentTypePresetDescription(preset);
};

// Whether the current name/description was filled in by the system (any preset's default value, or empty)
// Used to overwrite only values "not edited by the user" when switching types, so user input is never overwritten
const isNameSystemGenerated = (name: string): boolean => {
  if (!name) return true;
  return agentTypePresets.value.some(p => getPresetDefaultName(p) === name);
};
const isDescriptionSystemGenerated = (desc: string): boolean => {
  if (!desc) return true;
  return agentTypePresets.value.some(p => getPresetDefaultDescription(p) === desc);
};

// Return the user-facing incompatibility reason text for a preset id.
// Do not pass low-level capability names such as "vector / keyword / wiki" back to the user;
// users do not care about the implementation, they only want to know "why can't I use this knowledge base".
const presetKbMismatchKeyMap: Record<string, string> = {
  'rag-qa': 'ragQa',
  'wiki-qa': 'wikiQa',
  'hybrid-rag-wiki': 'hybridRagWiki',
  'data-analysis': 'dataAnalysis',
};
const presetKbMismatchReason = (preset: AgentTypePreset): string => {
  const subKey = presetKbMismatchKeyMap[preset.id];
  if (subKey) return t(`agentEditor.agentType.kbMismatch.${subKey}`);
  return t('agentEditor.agentType.kbMismatch.generic');
};

// Compute a preset's "effective KB filter": derived from the tools + incremental YAML overlay.
//
// Design principles:
//   - tools → any_of ("the KB must be usable by at least one of the tools") is
//     computed automatically by `deriveKbFilterFromTools`;
//   - the YAML `kb_filter` only carries business rules **the tools cannot derive** (such as
//     data-analysis's `none_of: ["faq"]`), merged incrementally rather than replacing everything;
//   - `all_of` / `none_of` are inherited directly from the YAML (the tools do not express these constraints).
//
// This way rag-qa / wiki-qa / hybrid write no `kb_filter` in the YAML at all,
// data-analysis only declares the extra `none_of`, and the "tool → capability" mapping is maintained
// once, in `@/utils/tool-capabilities`.
const effectiveKbFilter = (preset: AgentTypePreset | null): AgentTypeKBFilter | null => {
  if (!preset) return null;
  const derived = deriveKbFilterFromTools(preset.config?.allowed_tools || []);
  const yaml = preset.kb_filter;

  // When the YAML provides any_of it replaces the derivation entirely (leaving room for explicit control); otherwise use the derived one
  const anyOf = (yaml?.any_of && yaml.any_of.length > 0) ? yaml.any_of : (derived?.any_of ?? []);
  const allOf = yaml?.all_of ?? [];
  const noneOf = yaml?.none_of ?? [];
  if (anyOf.length === 0 && allOf.length === 0 && noneOf.length === 0) return null;
  return { any_of: anyOf, all_of: allOf, none_of: noneOf };
};

// Evaluate whether a single KB satisfies the given preset's kb_filter
const kbSatisfiesPresetFilter = (kb: { capabilities?: KBCapabilities; ragEnabled?: boolean; wikiEnabled?: boolean; type?: string }, preset: AgentTypePreset | null): { ok: boolean; reason: string } => {
  const filter = effectiveKbFilter(preset);
  if (!preset || !filter) return { ok: true, reason: '' };
  const caps = kb.capabilities || {
    vector: !!kb.ragEnabled,
    keyword: !!kb.ragEnabled,
    wiki: !!kb.wikiEnabled,
    graph: false,
    faq: kb.type === 'faq',
  };
  const has = (name: string): boolean => {
    switch (name) {
      case 'vector': return !!caps.vector;
      case 'keyword': return !!caps.keyword;
      case 'wiki': return !!caps.wiki;
      case 'graph': return !!caps.graph;
      case 'faq': return !!caps.faq;
      default: return false;
    }
  };
  const reason = presetKbMismatchReason(preset);
  if (filter.all_of && filter.all_of.length > 0) {
    for (const n of filter.all_of) {
      if (!has(n)) return { ok: false, reason };
    }
  }
  if (filter.any_of && filter.any_of.length > 0) {
    if (!filter.any_of.some(n => has(n))) {
      return { ok: false, reason };
    }
  }
  if (filter.none_of && filter.none_of.length > 0) {
    for (const n of filter.none_of) {
      if (has(n)) return { ok: false, reason };
    }
  }
  return { ok: true, reason: '' };
};

// The implicit requirement "Quick Answer / RAG mode" puts on a KB: it must have a vector or keyword index.
// This is decoupled from `activeAgentTypePreset`: quick-answer has no agent_type,
// so the preset chain is always null, but a wiki-only KB always returns empty retrieval in RAG mode,
// so the UI must disable it + explain, to keep users from choosing it in vain.
const kbSatisfiesQuickAnswerMode = (kb: { capabilities?: KBCapabilities; ragEnabled?: boolean }): { ok: boolean; reason: string } => {
  if (agentMode.value !== 'quick-answer') return { ok: true, reason: '' };
  const hasRag = kb.capabilities
    ? (!!kb.capabilities.vector || !!kb.capabilities.keyword)
    : !!kb.ragEnabled;
  if (hasRag) return { ok: true, reason: '' };
  return { ok: false, reason: t('agentEditor.agentType.kbMismatch.quickAnswer') };
};

// KB options after filtering (for the "Specific knowledge bases" dropdown); ones that do not qualify stay but are marked disabled + tooltip
const filteredKbOptionsForPreset = computed(() => {
  const preset = activeAgentTypePreset.value;
  return kbOptions.value.map(kb => {
    const presetResult = kbSatisfiesPresetFilter(kb, preset);
    const modeResult = kbSatisfiesQuickAnswerMode(kb);
    const ok = presetResult.ok && modeResult.ok;
    const reason = !presetResult.ok ? presetResult.reason : (!modeResult.ok ? modeResult.reason : '');
    return { ...kb, disabled: !ok, disabledReason: reason };
  });
});
const filteredMyKbOptions = computed(() => filteredKbOptionsForPreset.value.filter(kb => !kb.shared));
const filteredSharedKbOptions = computed(() => filteredKbOptionsForPreset.value.filter(kb => kb.shared));

// How many of the selected KBs would be disabled under the new preset / mode (for a hint before saving).
// In quick-answer mode the preset is always null, but a wiki-only KB still counts as "disabled",
// so this no longer depends on whether a preset exists and looks directly for disabled selected items.
const incompatibleSelectedKbCount = computed(() => {
  if (kbSelectionMode.value !== 'selected') return 0;
  const selected = new Set(formData.value.config.knowledge_bases || []);
  return filteredKbOptionsForPreset.value.filter(kb => selected.has(kb.value) && kb.disabled).length;
});

// Apply a preset's config to formData.config (overwriting only the fields the preset sets explicitly, leaving the rest)
const applyAgentTypePreset = (preset: AgentTypePreset | null) => {
  if (!preset || !preset.config) return;
  const c = preset.config;
  const target = formData.value.config;
  if (c.system_prompt_id !== undefined) {
    target.system_prompt_id = c.system_prompt_id;
    // Look up the text by system_prompt_id in the loaded template list and fill it into the user-visible textarea
    const tmpl = agentSystemPromptTemplates.value.find(t => t.id === c.system_prompt_id);
    if (tmpl && typeof tmpl.content === 'string') {
      target.system_prompt = tmpl.content;
    } else {
      // The template list has not loaded yet / or the preset references an id that does not exist: clear it so the user notices the change
      target.system_prompt = '';
      if (c.system_prompt_id) {
        console.warn(`[AgentType] system_prompt_id "${c.system_prompt_id}" not found in agent_system_prompt templates`);
      }
    }
  }
  if (typeof c.temperature === 'number') target.temperature = c.temperature;
  if (typeof c.max_iterations === 'number') target.max_iterations = c.max_iterations;
  if (Array.isArray(c.allowed_tools)) target.allowed_tools = normalizeLegacyToolNames(c.allowed_tools);
  if (typeof c.retain_retrieval_history === 'boolean') target.retain_retrieval_history = c.retain_retrieval_history;
  if (typeof c.faq_priority_enabled === 'boolean') target.faq_priority_enabled = c.faq_priority_enabled;
  if (typeof c.web_search_enabled === 'boolean') target.web_search_enabled = c.web_search_enabled;
  // supported_file_types uses "strong sync" semantics: only data-analysis needs to restrict to csv/xlsx,
  // and switching to any other type must clear it, or leftovers carry over from the previous type.
  if (Array.isArray(c.supported_file_types)) {
    target.supported_file_types = [...c.supported_file_types];
  } else {
    target.supported_file_types = [];
  }
  // Sync kb_selection_mode to formData and the UI state (both must change, or the radio buttons do not update)
  if (c.kb_selection_mode) {
    target.kb_selection_mode = c.kb_selection_mode;
    kbSelectionMode.value = c.kb_selection_mode;
  }
};

// The user switches the type manually → apply the preset
const onAgentTypeChange = (val: AgentType) => {
  // Before switching, capture whether the name/description can be safely overwritten
  // User input that has been edited (not equal to any preset default) is never overwritten
  const canOverrideName = isNameSystemGenerated(formData.value.name);
  const canOverrideDesc = isDescriptionSystemGenerated(formData.value.description);

  agentType.value = val;
  const preset = agentTypePresets.value.find(p => p.id === val) || null;
  if (val !== 'custom') {
    applyAgentTypePreset(preset);
  }

  // Refresh the auto-filled fields with the new preset's default name/description
  if (canOverrideName) {
    formData.value.name = getPresetDefaultName(preset);
  }
  if (canOverrideDesc) {
    formData.value.description = getPresetDefaultDescription(preset);
  }

  // If the new preset conflicts with the currently selected KBs, show a soft hint (without forcing removal)
  if (incompatibleSelectedKbCount.value > 0) {
    MessagePlugin.warning(
      t('agentEditor.agentType.kbIncompatibleWarn', { count: incompatibleSelectedKbCount.value }),
      4000,
    );
  }
};

// Reasoning effort: reasoning_effort is authoritative; old data with only the thinking boolean derives true→auto / false→off.
// On write the thinking boolean is kept in sync, so old backends / old read paths keep working.
const selectedChatModel = computed(() =>
  allModels.value.find(model => model.id === formData.value.config.model_id),
);
const selectedChatModelCanThink = computed(() => modelCanThink(selectedChatModel.value?.capabilities));
// When the selected model cannot turn thinking off (deepseek-reasoner / qwq-plus / gemini-3 and so on), show a hint,
// otherwise a dropdown with no "Off" looks like a bug.
const selectedChatModelAlwaysThinks = computed(
  () => modelCannotDisableThinking(selectedChatModel.value?.capabilities),
);
// When the catalog reports capabilities, the options follow thinking_levels strictly (including the fact that there is "no off");
// only models without capabilities (local / Ollama / model list not loaded) fall back to the generic ladder.
const reasoningEffortOptions = computed<ReasoningLevel[]>(() => optionsFor(selectedChatModel.value?.capabilities));
const reasoningEffortLevel = computed<ReasoningLevel>({
  get: () => levelFromLegacy(formData.value.config.thinking, formData.value.config.reasoning_effort),
  set: (level: ReasoningLevel) => {
    formData.value.config.reasoning_effort = level;
    formData.value.config.thinking = levelEnablesThinking(level);
  },
});
// The saved level may not be in the selected model's available set (model changed, or an old agent loaded):
// clamp it onto the available set and sync the thinking boolean (the setter handles it), so the UI does not show "Off"
// while the backend sends no switch at all and the model keeps thinking.
//
// Clamp only after the model has actually resolved: while the model list loads asynchronously capabilities is still undefined,
// and the generic ladder would wrongly downgrade a saved max/xhigh to auto.
const clampReasoningEffortToModel = () => {
  if (!selectedChatModel.value) return;
  const clamped = clampLevel(reasoningEffortLevel.value, reasoningEffortOptions.value);
  if (clamped !== reasoningEffortLevel.value) reasoningEffortLevel.value = clamped;
};
watch(
  () => [
    editorInitializing.value,
    formData.value.config.model_id,
    reasoningEffortOptions.value.join(','),
  ].join('|'),
  () => {
    if (!editorInitializing.value) clampReasoningEffortToModel();
  },
  { immediate: true },
);

// Whether this is a built-in agent
const isBuiltinAgent = computed(() => {
  return formData.value.is_builtin === true;
});

// A built-in agent's empty model slot runs on the declared default (backend
// agentModelSlot). Show it as the placeholder; never write it into the form.
const builtinDefaultModel = (type: 'KnowledgeQA' | 'Rerank') => {
  if (!isBuiltinAgent.value) return undefined;
  const id = resolveBuiltinDefaultModelId(allModels.value, type);
  return id ? allModels.value.find(model => model.id === id) : undefined;
};
const workspaceDefaultPlaceholder = (type: 'KnowledgeQA' | 'Rerank', fallbackKey: string) => {
  const model = builtinDefaultModel(type);
  return model
    ? t('agent.editor.workspaceDefaultModel', { name: model.display_name?.trim() || model.name })
    : t(fallbackKey);
};
const chatModelPlaceholder = computed(() => workspaceDefaultPlaceholder('KnowledgeQA', 'agent.editor.modelPlaceholder'));
const rerankModelPlaceholder = computed(() => workspaceDefaultPlaceholder('Rerank', 'agent.editor.rerankModelPlaceholder'));

// System prompt placeholder
const systemPromptPlaceholder = computed(() => {
  return t('agent.editor.systemPromptPlaceholder');
});

// Context template placeholder
const contextTemplatePlaceholder = computed(() => {
  return t('agent.editor.contextTemplatePlaceholder');
});

// Whether a ReRank model is needed (only when the linked knowledge bases include a RAG type)
const needsRerankModel = computed(() => {
  if (!hasKnowledgeBase.value) return false;
  const mode = kbSelectionMode.value;
  if (mode === 'all') {
    // In "All" mode, it is needed as soon as any RAG knowledge base exists
    return kbOptions.value.some(kb => kb.ragEnabled);
  }
  if (mode === 'selected') {
    const selectedIds = formData.value.config.knowledge_bases || [];
    return kbOptions.value.some(kb => selectedIds.includes(kb.value) && kb.ragEnabled);
  }
  return false;
});

let editorInitializationGeneration = 0;

// Watch visibility changes and reset the form
watch(() => props.visible, async (val) => {
  const generation = ++editorInitializationGeneration;
  if (val) {
    void deploymentCapabilities.ensureLoaded();
    editorInitializing.value = true;
    try {
    savedAgent.value = null;
    currentSection.value = resolveEditorSection(props.initialSection);
    // Load the dependency data first (including the default config)
    await loadDependencies();
    if (generation !== editorInitializationGeneration || !props.visible) return;

    if (props.mode === 'edit' && props.agent) {
      // Deep copy the object to avoid reference problems
      const agentData = JSON.parse(JSON.stringify(props.agent));

      // Make sure the config object exists
      if (!agentData.config) {
        agentData.config = JSON.parse(JSON.stringify(defaultFormData.config));
      }

      // Fill in fields that may be missing
      agentData.config = { ...defaultFormData.config, ...agentData.config };
      if (agentData.config.thinking == null) {
        agentData.config.thinking = false;
      }
      // Legacy rows carry only the boolean: derive the graded level once so
      // the selector and the persisted config agree (true → auto, false → off).
      agentData.config.reasoning_effort = levelFromLegacy(agentData.config.thinking, agentData.config.reasoning_effort);
      agentData.config.thinking = levelEnablesThinking(agentData.config.reasoning_effort);

      agentData.config.question_suggestions = {
        starters: {
          ...defaultFormData.config.question_suggestions.starters,
          ...(agentData.config.question_suggestions?.starters || {}),
          items: agentData.config.question_suggestions?.starters?.items || [],
        },
        follow_ups: {
          ...defaultFormData.config.question_suggestions.follow_ups,
          ...(agentData.config.question_suggestions?.follow_ups || {}),
          categories: agentData.config.question_suggestions?.follow_ups?.categories
            || [...defaultFormData.config.question_suggestions.follow_ups.categories],
        },
      };
      // Make sure the array fields exist
      if (!agentData.config.knowledge_bases) agentData.config.knowledge_bases = [];
      // Old configs may still carry merged tool names (knowledge_search / grep_chunks /
      // list_knowledge_chunks / get_document_info / wiki_read_source_doc);
      // map them to the new names and dedupe, or the checkboxes will not match allTools.
      agentData.config.allowed_tools = normalizeLegacyToolNames(agentData.config.allowed_tools);
      if (!agentData.config.mcp_services) agentData.config.mcp_services = [];
      // Authorization wait timeout: default to 600 seconds when old data lacks it
      if (agentData.config.mcp_auth_wait_timeout == null || agentData.config.mcp_auth_wait_timeout <= 0) {
        agentData.config.mcp_auth_wait_timeout = 600;
      }
      if (!agentData.config.selected_skills) agentData.config.selected_skills = [];
      if (!agentData.config.supported_file_types) agentData.config.supported_file_types = [];
      if (!agentData.config.chat_parser_engine_rules) agentData.config.chat_parser_engine_rules = [];
      // Attachment parsing tuning fields: set to 0 when old data lacks them (meaning use the global default)
      if (agentData.config.attachment_ocr_max_pages == null) agentData.config.attachment_ocr_max_pages = 0;
      if (agentData.config.attachment_parse_wait_timeout_sec == null) agentData.config.attachment_parse_wait_timeout_sec = 0;
      if (agentData.config.max_completion_tokens == null) agentData.config.max_completion_tokens = 0;
      // Long-term memory: the backend uses omitempty, so an agent following the workspace setting carries no such field.
      // Without filling it in as true the switch would show "off", and a casual save would really turn memory off.
      if (agentData.config.memory_enabled == null) agentData.config.memory_enabled = true;

      // Old data compatibility: without an agent_mode field, infer it from allowed_tools
      if (!agentData.config.agent_mode) {
        const isAgent = agentData.config.max_iterations < 0 || agentData.config.max_iterations > 1 || (agentData.config.allowed_tools && agentData.config.allowed_tools.length > 0);
        agentData.config.agent_mode = isAgent ? 'smart-reasoning' : 'quick-answer';
      }

      // Set the initialization flag so watches do not add tools automatically
      isInitializing.value = true;
      agentData.config = hydrateAgentPromptRefs(agentData.config, promptTemplates.value);
      formData.value = agentData;
      if (agentData.config.max_iterations > 1) {
        lastFiniteMaxIterations.value = agentData.config.max_iterations;
      }
      // Initialize the knowledge base selection mode
      initKbSelectionMode();
      initMcpSelectionMode();
      initSkillsSelectionMode();
      // Reset the flag after initialization
      nextTick(() => {
        isInitializing.value = false;
      });
      // Display inherited defaults for all agents without persisting a copy.
      fillBuiltinAgentDefaults();
      void loadAgentIntegrationCounts(agentData.id);
    } else {
      // Create a new agent with the system defaults
      const newFormData = JSON.parse(JSON.stringify(defaultFormData));
      // Apply the system default retrieval parameters
      newFormData.config.embedding_top_k = defaultEmbeddingTopK.value;
      newFormData.config.keyword_threshold = defaultKeywordThreshold.value;
      newFormData.config.vector_threshold = defaultVectorThreshold.value;
      newFormData.config.rerank_top_k = defaultRerankTopK.value;
      newFormData.config.rerank_threshold = defaultRerankThreshold.value;
      newFormData.config.max_completion_tokens = 0;
      newFormData.config.temperature = defaultTemperature.value;
      // Apply the system default prompt (filled by mode)
      const isAgent = newFormData.config.agent_mode === 'smart-reasoning';
      if (isAgent) {
        // Agent mode uses the default system prompt from agent-config
        if (defaultAgentSystemPrompt.value) {
          newFormData.config.system_prompt = defaultAgentSystemPrompt.value;
        }
      } else {
        // Quick Answer mode: the default prompt comes from the prompt-templates default entry
        if (defaultNormalSystemPrompt.value) {
          newFormData.config.system_prompt = defaultNormalSystemPrompt.value;
        }
        if (defaultContextTemplate.value) {
          newFormData.config.context_template = defaultContextTemplate.value;
        }
        if (defaultRewritePromptSystem.value) {
          newFormData.config.rewrite_prompt_system = defaultRewritePromptSystem.value;
        }
        if (defaultRewritePromptUser.value) {
          newFormData.config.rewrite_prompt_user = defaultRewritePromptUser.value;
        }
        if (defaultFallbackPrompt.value) {
          newFormData.config.fallback_prompt = defaultFallbackPrompt.value;
        }
        if (defaultFallbackResponse.value) {
          newFormData.config.fallback_response = defaultFallbackResponse.value;
        }
      }
      formData.value = newFormData;
      // New agent: knowledge bases default to "All", MCP / Skills still default to "Not used".
      kbSelectionMode.value = 'all';
      mcpSelectionMode.value = 'none';
      skillsSelectionMode.value = 'none';

      // When creating a Smart Reasoning agent, apply the default agent_type preset right away
      // (filling in system_prompt / allowed_tools / kb_selection_mode and so on),
      // otherwise the "default form" the user sees as the modal opens would not match the type shown in the dropdown.
      if (newFormData.config.agent_mode === 'smart-reasoning') {
        const defaultTypeId = newFormData.config.agent_type as AgentType;
        const preset = agentTypePresets.value.find(p => p.id === defaultTypeId) || null;
        if (defaultTypeId && defaultTypeId !== 'custom') {
          applyAgentTypePreset(preset);
        }
        // Give the new form the default name "My XXX" + the preset description, so the user can save directly;
        // values the user typed are never overwritten (this is the create case, so the fields are always empty).
        if (!formData.value.name) {
          formData.value.name = getPresetDefaultName(preset);
        }
        if (!formData.value.description) {
          formData.value.description = getPresetDefaultDescription(preset);
        }
      }
      applyDefaultModelsIfEmpty()
    }

    await syncInstalledSkills()
    if (generation !== editorInitializationGeneration || !props.visible) return;

    if (props.initialHighlightField) {
      await applyInitialFieldHighlight(props.initialHighlightField);
      if (generation !== editorInitializationGeneration || !props.visible) return;
    }
    } catch (error) {
      console.error('Failed to initialize agent editor', error);
    } finally {
      if (generation === editorInitializationGeneration && props.visible) {
        // Model normalization and queued form watchers belong to initialization,
        // so capture the baseline only after they have settled.
        clampReasoningEffortToModel();
        await nextTick();
        if (generation !== editorInitializationGeneration || !props.visible) return;
        modalShell.markClean();
        editorInitializing.value = false;
      }
    }
  } else {
    editorInitializing.value = false;
    clearFieldHighlight();
    agentIMChannelCount.value = 0;
    agentEmbedChannelCount.value = 0;
    showSkillProgress.value = false;
    skillProgressRecord.value = null;
    skillProgressId.value = '';
  }
});

// Initialize the knowledge base selection mode
const initKbSelectionMode = () => {
  if (formData.value.config.kb_selection_mode) {
    // If there is a saved mode, use it directly
    kbSelectionMode.value = formData.value.config.kb_selection_mode;
  } else if (formData.value.config.knowledge_bases?.length > 0) {
    // There are specific knowledge bases
    kbSelectionMode.value = 'selected';
  } else {
    kbSelectionMode.value = 'none';
  }
};

// Initialize the MCP selection mode
const initMcpSelectionMode = () => {
  if (formData.value.config.mcp_selection_mode) {
    // If there is a saved mode, use it directly
    mcpSelectionMode.value = formData.value.config.mcp_selection_mode;
  } else if (formData.value.config.mcp_services?.length > 0) {
    // There are specific MCP services
    mcpSelectionMode.value = 'selected';
  } else {
    mcpSelectionMode.value = 'none';
  }
};

// Initialize the Skills selection mode
const initSkillsSelectionMode = () => {
  if (formData.value.config.skills_selection_mode) {
    // If there is a saved mode, use it directly
    skillsSelectionMode.value = formData.value.config.skills_selection_mode;
  } else if (formData.value.config.selected_skills?.length > 0) {
    // There are specific Skills
    skillsSelectionMode.value = 'selected';
  } else {
    skillsSelectionMode.value = 'none';
  }
  autoBindSoleSandbox();
};

// Built-in agent: fill in the system defaults
const fillBuiltinAgentDefaults = () => {
  const config = formData.value.config;
  const isAgent = config.agent_mode === 'smart-reasoning';

  if (isAgent) {
    // Agent mode: use the default prompt from agent-config
    if (!config.system_prompt && defaultAgentSystemPrompt.value) {
      config.system_prompt = defaultAgentSystemPrompt.value;
    }
  } else {
    // Normal mode: the default system prompt, context template and so on come from the prompt-templates default entry
    if (!config.system_prompt && defaultNormalSystemPrompt.value) {
      config.system_prompt = defaultNormalSystemPrompt.value;
    }
    if (!config.context_template && defaultContextTemplate.value) {
      config.context_template = defaultContextTemplate.value;
    }
  }

  // Generic defaults
  if (!config.rewrite_prompt_system && defaultRewritePromptSystem.value) {
    config.rewrite_prompt_system = defaultRewritePromptSystem.value;
  }
  if (!config.rewrite_prompt_user && defaultRewritePromptUser.value) {
    config.rewrite_prompt_user = defaultRewritePromptUser.value;
  }
  if (!config.fallback_prompt && defaultFallbackPrompt.value) {
    config.fallback_prompt = defaultFallbackPrompt.value;
  }
  if (!config.fallback_response && defaultFallbackResponse.value) {
    config.fallback_response = defaultFallbackResponse.value;
  }
};

// Watch the knowledge base selection mode
watch(kbSelectionMode, (mode) => {
  formData.value.config.kb_selection_mode = mode;
  if (mode === 'none') {
    // No knowledge base: clear the related config
    formData.value.config.knowledge_bases = [];
  } else if (mode === 'all') {
    // All knowledge bases: clear the specific list
    formData.value.config.knowledge_bases = [];
  }
  // selected mode keeps knowledge_bases unchanged
});

// Watch the MCP selection mode
watch(mcpSelectionMode, (mode) => {
  formData.value.config.mcp_selection_mode = mode;
  if (mode === 'none') {
    // No MCP: clear the related config
    formData.value.config.mcp_services = [];
  } else if (mode === 'all') {
    // All MCP: clear the specific list
    formData.value.config.mcp_services = [];
  }
  // selected mode keeps mcp_services unchanged
});

watch(() => formData.value.config.sandbox_config_id, async () => {
  if (!props.visible) return
  await syncInstalledSkills()
})

let catalogPollTimer: number | null = null

function stopCatalogPoll() {
  if (catalogPollTimer != null) {
    window.clearInterval(catalogPollTimer)
    catalogPollTimer = null
  }
}

watch(
  [() => props.visible, catalogSkillRows],
  () => {
    const busy = catalogSkillRows.value.some((skill) =>
      skill.installStatus === 'installing' || skill.installStatus === 'removing',
    )
    if (!props.visible || !busy) {
      stopCatalogPoll()
      return
    }
    if (catalogPollTimer != null) return
    catalogPollTimer = window.setInterval(() => {
      void syncInstalledSkills(true)
    }, 2500)
  },
  { flush: 'post' },
)

// Watch the Skills selection mode
watch(skillsSelectionMode, (mode) => {
  formData.value.config.skills_selection_mode = mode;
  if (mode === 'none') {
    // No Skills: clear the related config
    formData.value.config.selected_skills = [];
  } else if (mode === 'all') {
    // All Skills: clear the specific list
    formData.value.config.selected_skills = [];
    autoBindSoleSandbox()
  } else {
    autoBindSoleSandbox()
  }
  // selected mode keeps selected_skills unchanged
});

// Watch mode changes and adjust the config automatically
watch(agentMode, (val, _oldVal) => {
  if (val === 'smart-reasoning') {
    // Switching to Agent mode: enable tools based on the knowledge base config.
    // Note: thinking / todo_write are not injected by default; they are for explicit reflection or multi-step plans
    // and raise token use noticeably, so users check them by hand as needed.
    if (formData.value.config.allowed_tools.length === 0) {
      const tools: string[] = [];
      if (hasRagKnowledgeBase.value) {
        tools.push(...knowledgeBaseTools);
      }
      if (hasWikiKnowledgeBase.value) {
        tools.push(...wikiReadTools);
      }
      formData.value.config.allowed_tools = tools;
    }
    if (formData.value.config.max_iterations >= 0 && formData.value.config.max_iterations <= 1) {
      formData.value.config.max_iterations = 10;
    }
    // When switching to Agent mode, if the system prompt is the Quick Answer default or empty, replace it with the Agent default prompt
    if (defaultAgentSystemPrompt.value) {
      const isDefaultNormalPrompt = formData.value.config.system_prompt === defaultNormalSystemPrompt.value;
      if (!formData.value.config.system_prompt || isDefaultNormalPrompt) {
        formData.value.config.system_prompt = defaultAgentSystemPrompt.value;
      }
    }
  } else {
    // Switching to normal mode: clear the tools
    formData.value.config.allowed_tools = [];
    formData.value.config.max_iterations = 1; // 1 means single-turn RAG
    // When switching to Quick Answer mode, if the system prompt is the Agent default or empty, replace it with the Quick Answer default prompt
    if (defaultNormalSystemPrompt.value) {
      const isDefaultAgentPrompt = formData.value.config.system_prompt === defaultAgentSystemPrompt.value;
      if (!formData.value.config.system_prompt || isDefaultAgentPrompt) {
        formData.value.config.system_prompt = defaultNormalSystemPrompt.value;
      }
    }
    // Other prompts are filled only when empty
    if (!formData.value.config.context_template && defaultContextTemplate.value) {
      formData.value.config.context_template = defaultContextTemplate.value;
    }
    if (!formData.value.config.rewrite_prompt_system && defaultRewritePromptSystem.value) {
      formData.value.config.rewrite_prompt_system = defaultRewritePromptSystem.value;
    }
    if (!formData.value.config.rewrite_prompt_user && defaultRewritePromptUser.value) {
      formData.value.config.rewrite_prompt_user = defaultRewritePromptUser.value;
    }
    if (!formData.value.config.fallback_prompt && defaultFallbackPrompt.value) {
      formData.value.config.fallback_prompt = defaultFallbackPrompt.value;
    }
    if (!formData.value.config.fallback_response && defaultFallbackResponse.value) {
      formData.value.config.fallback_response = defaultFallbackResponse.value;
    }
  }
});

// Watch the knowledge base enabled state:
//   - from "none" to "some": fill in the basic RAG tools automatically, so it works out of the box (seed only);
//   - from "some" to "none": tools are **no longer** wiped automatically; unmet dependencies are grayed out by `availableTools`
//     + filtered by the runtime tool registry. `allowed_tools` represents user intent and should change only on explicit user actions
//     (switching agent_type / switching agent_mode / checking tools by hand).
// Background: older versions wiped the KB/wiki tools when KB capability disappeared, so a user switching
// `kb_selection_mode` to "selected" without checking specific KBs yet silently lost tools in that transition,
// which was especially fatal for the built-in "Wiki Q&A" agent whose default tools are all wiki_*.
watch(hasKnowledgeBase, (hasKB, oldHasKB) => {
  // If the current page is the retrieval strategy page but knowledge base capability is gone, switch to basic settings
  if (!hasKB && currentSection.value === 'retrieval') {
    currentSection.value = 'basic';
  }

  // Do not adjust tools automatically during initialization or outside Agent mode
  if (isInitializing.value || !isAgentMode.value) return;

  if (hasKB && !oldHasKB) {
    // From no knowledge base to a knowledge base: seed the default RAG tools (filling in only unchecked ones)
    const currentTools = formData.value.config.allowed_tools || [];
    const toolsToAdd = knowledgeBaseTools.filter((tool: string) => !currentTools.includes(tool));
    formData.value.config.allowed_tools = [...currentTools, ...toolsToAdd];
  }
});

// Watch run mode changes and switch pages automatically
watch(isAgentMode, (isAgent) => {
  // If the current page is advanced settings or retrieval strategy but the mode switched to Agent, switch to basic settings
  if (isAgent && (currentSection.value === 'advanced' || currentSection.value === 'retrieval')) {
    currentSection.value = 'basic';
  }
  if (!isAgent && (currentSection.value === 'skills' || currentSection.value === 'sandbox')) {
    currentSection.value = 'basic';
  }
});

// Watch the settings modal closing and refresh the model list
watch(() => uiStore.showSettingsModal, async (visible, prevVisible) => {
  if (prevVisible && !visible && props.visible) {
    try {
      await Promise.all([
        chatResources.ensureModels(true),
        editorResources.ensureStorageEngine(true),
        chatResources.ensureSandboxConfigs(true),
      ]);
      if (chatResources.allModels.length > 0) {
        allModels.value = chatResources.allModels;
      }
      if (editorResources.storageStatus.length > 0) {
        storageEngineStatus.value = editorResources.storageStatus;
      }
      await syncInstalledSkills(true);
    } catch (e) {
      console.warn('Failed to refresh data after settings closed', e);
    }
  }
});

watch(() => chatResources.allModels, (list) => {
  if (props.visible) {
    allModels.value = list;
  }
});

const mapKbToOption = (kb: any, shared: boolean, orgName?: string) => {
  const strategy = kb.indexing_strategy;
  const caps: KBCapabilities | undefined = kb.capabilities;
  return {
    label: kb.name,
    value: kb.id,
    type: kb.type || 'document',
    count: kb.type === 'faq' ? (kb.chunk_count || 0) : (kb.knowledge_count || 0),
    shared,
    orgName,
    ragEnabled: caps ? (caps.vector || caps.keyword) : (!strategy || strategy.vector_enabled || strategy.keyword_enabled),
    wikiEnabled: caps ? caps.wiki : (strategy?.wiki_enabled || false),
    capabilities: caps,
  };
};

const applyPromptTemplateDefaults = (cfg: PromptTemplatesConfig | null) => {
  promptTemplates.value = cfg;
  if (!cfg) return;
  if (cfg.agent_system_prompt && Array.isArray(cfg.agent_system_prompt)) {
    agentSystemPromptTemplates.value = cfg.agent_system_prompt;
    const ragDefault =
      cfg.agent_system_prompt.find(t => t.mode === 'rag' && t.default) ||
      cfg.agent_system_prompt.find(t => t.mode === 'rag');
    if (ragDefault?.content) {
      defaultAgentSystemPrompt.value = ragDefault.content;
    }
  }
  const pickDefault = (arr?: PromptTemplate[]): PromptTemplate | undefined =>
    Array.isArray(arr) ? arr.find(t => t.default) : undefined;
  const sysPrompt = pickDefault(cfg.system_prompt);
  if (sysPrompt?.content) defaultNormalSystemPrompt.value = sysPrompt.content;
  const ctxTmpl = pickDefault(cfg.context_template);
  if (ctxTmpl?.content) defaultContextTemplate.value = ctxTmpl.content;
  const rewriteTmpl = pickDefault(cfg.rewrite);
  if (rewriteTmpl?.content) defaultRewritePromptSystem.value = rewriteTmpl.content;
  if (rewriteTmpl?.user) defaultRewritePromptUser.value = rewriteTmpl.user;
  const fallbackList = Array.isArray(cfg.fallback) ? cfg.fallback : [];
  const fixedFallback = fallbackList.find(t => t.default && t.mode !== 'model');
  if (fixedFallback?.content) defaultFallbackResponse.value = fixedFallback.content;
  const modelFallback = fallbackList.find(t => t.mode === 'model' && t.default) || fallbackList.find(t => t.mode === 'model');
  if (modelFallback?.content) defaultFallbackPrompt.value = modelFallback.content;
  if (Array.isArray(cfg.intent_prompts)) {
    intentPromptTemplates.value = cfg.intent_prompts;
  }
};

// Load the dependency data (reusing the workspace-level cache to avoid repeat requests)
const loadDependencies = async () => {
  try {
    await Promise.all([
      chatResources.ensureModels(),
      chatResources.ensureKnowledgeBases(),
      chatResources.ensureWebSearchProviders(),
      chatResources.ensureSandboxConfigs(),
      editorResources.prefetchAgentEditorDeps(),
    ]);

    if (chatResources.allModels.length > 0) {
      allModels.value = chatResources.allModels;
    }

    const myKbs = chatResources.rawKnowledgeBases.map((kb: any) => mapKbToOption(kb, false));
    const myKbIds = new Set(myKbs.map(kb => kb.value));
    const sharedKbs = (orgStore.sharedKnowledgeBases || [])
      .filter((shared: any) => shared.knowledge_base && !myKbIds.has(shared.knowledge_base.id))
      .map((shared: any) => mapKbToOption(shared.knowledge_base, true, shared.org_name));
    kbOptions.value = [...myKbs, ...sharedKbs];

    agentTypePresets.value = editorResources.agentTypePresets as AgentTypePreset[];
    applyPromptTemplateDefaults(editorResources.promptTemplates);

    storageEngineStatus.value = editorResources.storageStatus;

    webSearchProviderList.value = chatResources.webSearchProviders as WebSearchProviderEntity[];

    if (editorResources.placeholders) {
      placeholderData.value = editorResources.placeholders;
    }

    const rc = editorResources.tenantRetrievalConfig as Record<string, number> | null;
    if (rc?.embedding_top_k) defaultEmbeddingTopK.value = rc.embedding_top_k;
    if (rc?.keyword_threshold !== undefined) defaultKeywordThreshold.value = rc.keyword_threshold;
    if (rc?.vector_threshold !== undefined) defaultVectorThreshold.value = rc.vector_threshold;
    if (rc?.rerank_top_k) defaultRerankTopK.value = rc.rerank_top_k;
    if (rc?.rerank_threshold !== undefined) defaultRerankThreshold.value = rc.rerank_threshold;
  } catch (e) {
    console.error('Failed to load dependencies', e);
  }
};

// Go to the model management page to add a model
const handleAddModel = (subSection: string) => {
  uiStore.openSettings('models', subSection);
};

const handleClose = () => {
  showPlaceholderPopup.value = false;
  showContextPlaceholderPopup.value = false;
  intentPromptPopup.value.show = false;
  rewriteSystemPopup.value.show = false;
  rewriteUserPopup.value.show = false;
  fallbackPromptPopup.value.show = false;
  emit('update:visible', false);
};

const modalShell = useModalShell({
  visible: () => props.visible,
  loading: () => editorInitializing.value,
  close: handleClose,
  snapshot: () => formData.value,
  ignoreEscape: () =>
    showPlaceholderPopup.value ||
    showContextPlaceholderPopup.value ||
    intentPromptPopup.value.show ||
    rewriteSystemPopup.value.show ||
    rewriteUserPopup.value.show ||
    fallbackPromptPopup.value.show,
});

// Filtered placeholder list
const filteredPlaceholders = computed(() => {
  if (!placeholderPrefix.value) {
    return availablePlaceholders.value;
  }
  const prefix = placeholderPrefix.value.toLowerCase();
  return availablePlaceholders.value.filter(p =>
    p.name.toLowerCase().startsWith(prefix)
  );
});

// Filtered context template placeholder list
const filteredContextPlaceholders = computed(() => {
  if (!contextPlaceholderPrefix.value) {
    return contextTemplatePlaceholders.value;
  }
  const prefix = contextPlaceholderPrefix.value.toLowerCase();
  return contextTemplatePlaceholders.value.filter(p =>
    p.name.toLowerCase().startsWith(prefix)
  );
});

// Filtered rewrite system prompt placeholder list
const filteredRewriteSystemPlaceholders = computed(() => {
  if (!rewriteSystemPopup.value.prefix) {
    return rewriteSystemPlaceholders.value;
  }
  const prefix = rewriteSystemPopup.value.prefix.toLowerCase();
  return rewriteSystemPlaceholders.value.filter(p =>
    p.name.toLowerCase().startsWith(prefix)
  );
});

// Filtered rewrite user prompt placeholder list
const filteredRewriteUserPlaceholders = computed(() => {
  if (!rewriteUserPopup.value.prefix) {
    return rewritePlaceholders.value;
  }
  const prefix = rewriteUserPopup.value.prefix.toLowerCase();
  return rewritePlaceholders.value.filter(p =>
    p.name.toLowerCase().startsWith(prefix)
  );
});

// Filtered fallback prompt placeholder list
const filteredFallbackPlaceholders = computed(() => {
  if (!fallbackPromptPopup.value.prefix) {
    return fallbackPlaceholders.value;
  }
  const prefix = fallbackPromptPopup.value.prefix.toLowerCase();
  return fallbackPlaceholders.value.filter(p =>
    p.name.toLowerCase().startsWith(prefix)
  );
});

// Get the textarea element
const getTextareaElement = (): HTMLTextAreaElement | null => {
  if (promptTextareaRef.value) {
    if (promptTextareaRef.value.$el) {
      return promptTextareaRef.value.$el.querySelector('textarea');
    }
    if (promptTextareaRef.value instanceof HTMLTextAreaElement) {
      return promptTextareaRef.value;
    }
  }
  return null;
};

// Compute the caret position
const calculateCursorPosition = (textarea: HTMLTextAreaElement) => {
  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = formData.value.config.system_prompt.substring(0, cursorPos);

  const style = window.getComputedStyle(textarea);
  // Placeholder popup is `position: fixed` under the root zoom; normalize the
  // visual-pixel rect to CSS pixels so the popup actually lands on the caret.
  const textareaRect = rectToCssPx(textarea.getBoundingClientRect(), getRootZoom());

  const lineHeight = parseFloat(style.lineHeight) || 20;
  const paddingTop = parseFloat(style.paddingTop) || 0;
  const paddingLeft = parseFloat(style.paddingLeft) || 0;

  // Compute the current line number
  const lines = textBeforeCursor.split('\n');
  const currentLine = lines.length - 1;
  const currentLineText = lines[currentLine];

  // Create a temporary span to measure the text width
  const span = document.createElement('span');
  span.style.font = style.font;
  span.style.visibility = 'hidden';
  span.style.position = 'absolute';
  span.style.whiteSpace = 'pre';
  span.textContent = currentLineText;
  document.body.appendChild(span);
  const textWidth = span.offsetWidth;
  document.body.removeChild(span);

  const scrollTop = textarea.scrollTop;
  const top = textareaRect.top + paddingTop + (currentLine * lineHeight) - scrollTop + lineHeight + 4;
  const scrollLeft = textarea.scrollLeft;
  const left = textareaRect.left + paddingLeft + textWidth - scrollLeft;

  return { top, left };
};

// Check for and show the placeholder hint
const checkAndShowPlaceholderPopup = () => {
  const textarea = getTextareaElement();
  if (!textarea) return;

  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = formData.value.config.system_prompt.substring(0, cursorPos);

  // Find the nearest {{ position
  let lastOpenPos = -1;
  for (let i = textBeforeCursor.length - 1; i >= 1; i--) {
    if (textBeforeCursor[i] === '{' && textBeforeCursor[i - 1] === '{') {
      const textAfterOpen = textBeforeCursor.substring(i + 1);
      if (!textAfterOpen.includes('}}')) {
        lastOpenPos = i - 1;
        break;
      }
    }
  }

  if (lastOpenPos === -1) {
    showPlaceholderPopup.value = false;
    placeholderPrefix.value = '';
    return;
  }

  const textAfterOpen = textBeforeCursor.substring(lastOpenPos + 2);
  placeholderPrefix.value = textAfterOpen;

  const filtered = filteredPlaceholders.value;
  if (filtered.length > 0) {
    nextTick(() => {
      const position = calculateCursorPosition(textarea);
      popupStyle.value = {
        top: `${position.top}px`,
        left: `${position.left}px`
      };
      showPlaceholderPopup.value = true;
      selectedPlaceholderIndex.value = 0;
    });
  } else {
    showPlaceholderPopup.value = false;
  }
};

// Handle input
const handlePromptInput = () => {
  if (placeholderPopupTimer) {
    clearTimeout(placeholderPopupTimer);
  }
  placeholderPopupTimer = setTimeout(() => {
    checkAndShowPlaceholderPopup();
  }, 50);
};

// Insert a placeholder
const insertPlaceholder = (placeholderName: string, fromPopup: boolean = false) => {
  const textarea = getTextareaElement();
  if (!textarea) return;

  showPlaceholderPopup.value = false;
  placeholderPrefix.value = '';
  selectedPlaceholderIndex.value = 0;

  nextTick(() => {
    const cursorPos = textarea.selectionStart;
    const currentValue = formData.value.config.system_prompt || '';
    const textBeforeCursor = currentValue.substring(0, cursorPos);
    const textAfterCursor = currentValue.substring(cursorPos);

    // Only look up and replace {{ when chosen from the dropdown
    if (fromPopup) {
      let lastOpenPos = -1;
      for (let i = textBeforeCursor.length - 1; i >= 1; i--) {
        if (textBeforeCursor[i] === '{' && textBeforeCursor[i - 1] === '{') {
          lastOpenPos = i - 1;
          break;
        }
      }

      if (lastOpenPos !== -1) {
        const textBeforeOpen = currentValue.substring(0, lastOpenPos);
        const newValue = textBeforeOpen + `{{${placeholderName}}}` + textAfterCursor;
        formData.value.config.system_prompt = newValue;

        nextTick(() => {
          const newCursorPos = textBeforeOpen.length + placeholderName.length + 4;
          textarea.setSelectionRange(newCursorPos, newCursorPos);
          textarea.focus();
        });
        return;
      }
    }

    // Insert the full placeholder directly at the caret
    const newValue = textBeforeCursor + `{{${placeholderName}}}` + textAfterCursor;
    formData.value.config.system_prompt = newValue;

    nextTick(() => {
      const newCursorPos = cursorPos + placeholderName.length + 4;
      textarea.setSelectionRange(newCursorPos, newCursorPos);
      textarea.focus();
    });
  });
};

// Get the context template textarea element
const getContextTemplateTextareaElement = (): HTMLTextAreaElement | null => {
  if (contextTemplateTextareaRef.value) {
    if (contextTemplateTextareaRef.value.$el) {
      return contextTemplateTextareaRef.value.$el.querySelector('textarea');
    }
    if (contextTemplateTextareaRef.value instanceof HTMLTextAreaElement) {
      return contextTemplateTextareaRef.value;
    }
  }
  return null;
};

// Compute the context template caret position
const calculateContextCursorPosition = (textarea: HTMLTextAreaElement) => {
  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = formData.value.config.context_template.substring(0, cursorPos);

  const style = window.getComputedStyle(textarea);
  // See `calculateCursorPosition` for the zoom rationale.
  const textareaRect = rectToCssPx(textarea.getBoundingClientRect(), getRootZoom());

  const lineHeight = parseFloat(style.lineHeight) || 20;
  const paddingTop = parseFloat(style.paddingTop) || 0;
  const paddingLeft = parseFloat(style.paddingLeft) || 0;

  const lines = textBeforeCursor.split('\n');
  const currentLine = lines.length - 1;
  const currentLineText = lines[currentLine];

  const span = document.createElement('span');
  span.style.font = style.font;
  span.style.visibility = 'hidden';
  span.style.position = 'absolute';
  span.style.whiteSpace = 'pre';
  span.textContent = currentLineText;
  document.body.appendChild(span);
  const textWidth = span.offsetWidth;
  document.body.removeChild(span);

  const scrollTop = textarea.scrollTop;
  const top = textareaRect.top + paddingTop + (currentLine * lineHeight) - scrollTop + lineHeight + 4;
  const scrollLeft = textarea.scrollLeft;
  const left = textareaRect.left + paddingLeft + textWidth - scrollLeft;

  return { top, left };
};

// Check for and show the context template placeholder hint
const checkAndShowContextPlaceholderPopup = () => {
  const textarea = getContextTemplateTextareaElement();
  if (!textarea) return;

  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = formData.value.config.context_template.substring(0, cursorPos);

  let lastOpenPos = -1;
  for (let i = textBeforeCursor.length - 1; i >= 1; i--) {
    if (textBeforeCursor[i] === '{' && textBeforeCursor[i - 1] === '{') {
      const textAfterOpen = textBeforeCursor.substring(i + 1);
      if (!textAfterOpen.includes('}}')) {
        lastOpenPos = i - 1;
        break;
      }
    }
  }

  if (lastOpenPos === -1) {
    showContextPlaceholderPopup.value = false;
    contextPlaceholderPrefix.value = '';
    return;
  }

  const textAfterOpen = textBeforeCursor.substring(lastOpenPos + 2);
  contextPlaceholderPrefix.value = textAfterOpen;

  const filtered = filteredContextPlaceholders.value;
  if (filtered.length > 0) {
    nextTick(() => {
      const position = calculateContextCursorPosition(textarea);
      contextPopupStyle.value = {
        top: `${position.top}px`,
        left: `${position.left}px`
      };
      showContextPlaceholderPopup.value = true;
      selectedContextPlaceholderIndex.value = 0;
    });
  } else {
    showContextPlaceholderPopup.value = false;
  }
};

// Handle context template input
const handleContextTemplateInput = () => {
  if (contextPlaceholderPopupTimer) {
    clearTimeout(contextPlaceholderPopupTimer);
  }
  contextPlaceholderPopupTimer = setTimeout(() => {
    checkAndShowContextPlaceholderPopup();
  }, 50);
};

// Insert a context template placeholder
const insertContextPlaceholder = (placeholderName: string, fromPopup: boolean = false) => {
  const textarea = getContextTemplateTextareaElement();
  if (!textarea) return;

  showContextPlaceholderPopup.value = false;
  contextPlaceholderPrefix.value = '';
  selectedContextPlaceholderIndex.value = 0;

  nextTick(() => {
    const cursorPos = textarea.selectionStart;
    const currentValue = formData.value.config.context_template || '';
    const textBeforeCursor = currentValue.substring(0, cursorPos);
    const textAfterCursor = currentValue.substring(cursorPos);

    // Only look up and replace {{ when chosen from the dropdown
    if (fromPopup) {
      let lastOpenPos = -1;
      for (let i = textBeforeCursor.length - 1; i >= 1; i--) {
        if (textBeforeCursor[i] === '{' && textBeforeCursor[i - 1] === '{') {
          lastOpenPos = i - 1;
          break;
        }
      }

      if (lastOpenPos !== -1) {
        const textBeforeOpen = currentValue.substring(0, lastOpenPos);
        const newValue = textBeforeOpen + `{{${placeholderName}}}` + textAfterCursor;
        formData.value.config.context_template = newValue;

        nextTick(() => {
          const newCursorPos = textBeforeOpen.length + placeholderName.length + 4;
          textarea.setSelectionRange(newCursorPos, newCursorPos);
          textarea.focus();
        });
        return;
      }
    }

    // Insert the full placeholder directly at the caret
    const newValue = textBeforeCursor + `{{${placeholderName}}}` + textAfterCursor;
    formData.value.config.context_template = newValue;

    nextTick(() => {
      const newCursorPos = cursorPos + placeholderName.length + 4;
      textarea.setSelectionRange(newCursorPos, newCursorPos);
      textarea.focus();
    });
  });
};

type GenericPlaceholderType = 'rewriteSystem' | 'rewriteUser' | 'fallback' | 'intent';

const genericPlaceholderFieldKeyMap: Record<Exclude<GenericPlaceholderType, 'intent'>, keyof typeof formData.value.config> = {
  rewriteSystem: 'rewrite_prompt_system',
  rewriteUser: 'rewrite_prompt_user',
  fallback: 'fallback_prompt',
};

const getGenericPlaceholderFieldValue = (type: GenericPlaceholderType): string => {
  if (type === 'intent') return intentEditorValue.value || '';
  return String(formData.value.config[genericPlaceholderFieldKeyMap[type]] || '');
};

const setGenericPlaceholderFieldValue = (type: GenericPlaceholderType, value: string) => {
  if (type === 'intent') {
    intentEditorValue.value = value;
    return;
  }
  (formData.value.config as any)[genericPlaceholderFieldKeyMap[type]] = value;
};

// Generic: get the textarea element
const getGenericTextareaElement = (type: GenericPlaceholderType): HTMLTextAreaElement | null => {
  const refMap = {
    rewriteSystem: rewriteSystemTextareaRef,
    rewriteUser: rewriteUserTextareaRef,
    fallback: fallbackPromptTextareaRef,
    intent: intentPromptTextareaRef,
  };
  const ref = refMap[type];
  if (ref.value) {
    if (ref.value.$el) {
      return ref.value.$el.querySelector('textarea');
    }
    if (ref.value instanceof HTMLTextAreaElement) {
      return ref.value;
    }
  }
  return null;
};

// Generic: compute the caret position
const calculateGenericCursorPosition = (textarea: HTMLTextAreaElement, fieldValue: string) => {
  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = fieldValue.substring(0, cursorPos);
  const lines = textBeforeCursor.split('\n');
  const currentLine = lines.length - 1;
  const currentLineText = lines[currentLine];

  // See `calculateCursorPosition` for the zoom rationale.
  const textareaRect = rectToCssPx(textarea.getBoundingClientRect(), getRootZoom());
  const style = window.getComputedStyle(textarea);
  const lineHeight = parseFloat(style.lineHeight) || 20;
  const paddingTop = parseFloat(style.paddingTop) || 0;
  const paddingLeft = parseFloat(style.paddingLeft) || 0;

  const span = document.createElement('span');
  span.style.font = style.font;
  span.style.visibility = 'hidden';
  span.style.position = 'absolute';
  span.style.whiteSpace = 'pre';
  span.textContent = currentLineText;
  document.body.appendChild(span);
  const textWidth = span.offsetWidth;
  document.body.removeChild(span);

  const scrollTop = textarea.scrollTop;
  const top = textareaRect.top + paddingTop + (currentLine * lineHeight) - scrollTop + lineHeight + 4;
  const scrollLeft = textarea.scrollLeft;
  const left = textareaRect.left + paddingLeft + textWidth - scrollLeft;

  return { top, left };
};

// Generic: check for and show the placeholder popup
const checkAndShowGenericPlaceholderPopup = (
  type: GenericPlaceholderType,
  popup: typeof rewriteSystemPopup,
  filteredPlaceholders: PlaceholderDefinition[]
) => {
  const textarea = getGenericTextareaElement(type);
  if (!textarea) return;

  const cursorPos = textarea.selectionStart;
  const fieldValue = getGenericPlaceholderFieldValue(type);
  const textBeforeCursor = fieldValue.substring(0, cursorPos);

  let lastOpenPos = -1;
  for (let i = textBeforeCursor.length - 1; i >= 1; i--) {
    if (textBeforeCursor[i] === '{' && textBeforeCursor[i - 1] === '{') {
      const textAfterOpen = textBeforeCursor.substring(i + 1);
      if (!textAfterOpen.includes('}}')) {
        lastOpenPos = i - 1;
        break;
      }
    }
  }

  if (lastOpenPos === -1) {
    popup.value.show = false;
    popup.value.prefix = '';
    return;
  }

  const textAfterOpen = textBeforeCursor.substring(lastOpenPos + 2);
  popup.value.prefix = textAfterOpen;

  if (filteredPlaceholders.length > 0) {
    nextTick(() => {
      const position = calculateGenericCursorPosition(textarea, fieldValue);
      popup.value.style = {
        top: `${position.top}px`,
        left: `${position.left}px`
      };
      popup.value.show = true;
      popup.value.selectedIndex = 0;
    });
  } else {
    popup.value.show = false;
  }
};

// Handle rewrite system prompt input
const handleRewriteSystemInput = () => {
  if (rewriteSystemPopup.value.timer) {
    clearTimeout(rewriteSystemPopup.value.timer);
  }
  rewriteSystemPopup.value.timer = setTimeout(() => {
    checkAndShowGenericPlaceholderPopup('rewriteSystem', rewriteSystemPopup, filteredRewriteSystemPlaceholders.value);
  }, 50);
};

// Handle rewrite user prompt input
const handleRewriteUserInput = () => {
  if (rewriteUserPopup.value.timer) {
    clearTimeout(rewriteUserPopup.value.timer);
  }
  rewriteUserPopup.value.timer = setTimeout(() => {
    checkAndShowGenericPlaceholderPopup('rewriteUser', rewriteUserPopup, filteredRewriteUserPlaceholders.value);
  }, 50);
};

// Handle fallback prompt input
const handleFallbackPromptInput = () => {
  if (fallbackPromptPopup.value.timer) {
    clearTimeout(fallbackPromptPopup.value.timer);
  }
  fallbackPromptPopup.value.timer = setTimeout(() => {
    checkAndShowGenericPlaceholderPopup('fallback', fallbackPromptPopup, filteredFallbackPlaceholders.value);
  }, 50);
};

// Handle intent prompt input
const handleIntentPromptInput = () => {
  if (intentPromptPopup.value.timer) {
    clearTimeout(intentPromptPopup.value.timer);
  }
  intentPromptPopup.value.timer = setTimeout(() => {
    checkAndShowGenericPlaceholderPopup('intent', intentPromptPopup, filteredIntentPlaceholders.value);
  }, 50);
};

// Generic: insert a placeholder
const insertGenericPlaceholder = (type: GenericPlaceholderType, placeholderName: string, fromPopup: boolean = false) => {
  const textarea = getGenericTextareaElement(type);
  if (!textarea) return;

  const popupMap = {
    rewriteSystem: rewriteSystemPopup,
    rewriteUser: rewriteUserPopup,
    fallback: fallbackPromptPopup,
    intent: intentPromptPopup,
  };

  const popup = popupMap[type];

  popup.value.show = false;
  popup.value.prefix = '';
  popup.value.selectedIndex = 0;

  nextTick(() => {
    const cursorPos = textarea.selectionStart;
    const currentValue = getGenericPlaceholderFieldValue(type);
    const textBeforeCursor = currentValue.substring(0, cursorPos);
    const textAfterCursor = currentValue.substring(cursorPos);

    // Only look up and replace {{ when chosen from the dropdown
    if (fromPopup) {
      let lastOpenPos = -1;
      for (let i = textBeforeCursor.length - 1; i >= 1; i--) {
        if (textBeforeCursor[i] === '{' && textBeforeCursor[i - 1] === '{') {
          lastOpenPos = i - 1;
          break;
        }
      }

      if (lastOpenPos !== -1) {
        const textBeforeOpen = currentValue.substring(0, lastOpenPos);
        const newValue = textBeforeOpen + `{{${placeholderName}}}` + textAfterCursor;
        setGenericPlaceholderFieldValue(type, newValue);

        nextTick(() => {
          const newCursorPos = textBeforeOpen.length + placeholderName.length + 4;
          textarea.setSelectionRange(newCursorPos, newCursorPos);
          textarea.focus();
        });
        return;
      }
    }

    // Insert the full placeholder directly at the caret
    const newValue = textBeforeCursor + `{{${placeholderName}}}` + textAfterCursor;
    setGenericPlaceholderFieldValue(type, newValue);

    nextTick(() => {
      const newCursorPos = cursorPos + placeholderName.length + 4;
      textarea.setSelectionRange(newCursorPos, newCursorPos);
      textarea.focus();
    });
  });
};

// Set up the context template textarea event listeners
const setupContextTemplateEventListeners = () => {
  nextTick(() => {
    const textarea = getContextTemplateTextareaElement();
    if (textarea) {
      textarea.addEventListener('keydown', (e: KeyboardEvent) => {
        if (showContextPlaceholderPopup.value && filteredContextPlaceholders.value.length > 0) {
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            e.stopPropagation();
            if (selectedContextPlaceholderIndex.value < filteredContextPlaceholders.value.length - 1) {
              selectedContextPlaceholderIndex.value++;
            } else {
              selectedContextPlaceholderIndex.value = 0;
            }
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            e.stopPropagation();
            if (selectedContextPlaceholderIndex.value > 0) {
              selectedContextPlaceholderIndex.value--;
            } else {
              selectedContextPlaceholderIndex.value = filteredContextPlaceholders.value.length - 1;
            }
          } else if (e.key === 'Enter' || e.key === 'Tab') {
            e.preventDefault();
            e.stopPropagation();
            const selected = filteredContextPlaceholders.value[selectedContextPlaceholderIndex.value];
            if (selected) {
              insertContextPlaceholder(selected.name, true);
            }
          } else if (e.key === 'Escape') {
            e.preventDefault();
            e.stopPropagation();
            showContextPlaceholderPopup.value = false;
            contextPlaceholderPrefix.value = '';
          }
        }
      }, true);
    }
  });
};

// Set up the textarea event listeners
const setupTextareaEventListeners = () => {
  nextTick(() => {
    const textarea = getTextareaElement();
    if (textarea) {
      textarea.addEventListener('keydown', (e: KeyboardEvent) => {
        if (showPlaceholderPopup.value && filteredPlaceholders.value.length > 0) {
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            e.stopPropagation();
            if (selectedPlaceholderIndex.value < filteredPlaceholders.value.length - 1) {
              selectedPlaceholderIndex.value++;
            } else {
              selectedPlaceholderIndex.value = 0;
            }
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            e.stopPropagation();
            if (selectedPlaceholderIndex.value > 0) {
              selectedPlaceholderIndex.value--;
            } else {
              selectedPlaceholderIndex.value = filteredPlaceholders.value.length - 1;
            }
          } else if (e.key === 'Enter' || e.key === 'Tab') {
            e.preventDefault();
            e.stopPropagation();
            const selected = filteredPlaceholders.value[selectedPlaceholderIndex.value];
            if (selected) {
              insertPlaceholder(selected.name, true);
            }
          } else if (e.key === 'Escape') {
            e.preventDefault();
            e.stopPropagation();
            showPlaceholderPopup.value = false;
            placeholderPrefix.value = '';
          }
        }
      }, true);
    }
  });
};

// Generic: set up the textarea event listeners
const setupGenericTextareaEventListeners = (
  type: GenericPlaceholderType,
  popup: typeof rewriteSystemPopup,
  filteredPlaceholders: () => PlaceholderDefinition[]
) => {
  nextTick(() => {
    const textarea = getGenericTextareaElement(type);
    if (textarea) {
      textarea.addEventListener('keydown', (e: KeyboardEvent) => {
        const filtered = filteredPlaceholders();
        if (popup.value.show && filtered.length > 0) {
          if (e.key === 'ArrowDown') {
            e.preventDefault();
            e.stopPropagation();
            if (popup.value.selectedIndex < filtered.length - 1) {
              popup.value.selectedIndex++;
            } else {
              popup.value.selectedIndex = 0;
            }
          } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            e.stopPropagation();
            if (popup.value.selectedIndex > 0) {
              popup.value.selectedIndex--;
            } else {
              popup.value.selectedIndex = filtered.length - 1;
            }
          } else if (e.key === 'Enter' || e.key === 'Tab') {
            e.preventDefault();
            e.stopPropagation();
            const selected = filtered[popup.value.selectedIndex];
            if (selected) {
              insertGenericPlaceholder(type, selected.name, true);
            }
          } else if (e.key === 'Escape') {
            e.preventDefault();
            e.stopPropagation();
            popup.value.show = false;
            popup.value.prefix = '';
          }
        }
      }, true);
    }
  });
};

// Handle a click on a placeholder tag
const handlePlaceholderClick = (type: 'system' | 'context' | 'rewriteSystem' | 'rewriteUser' | 'fallback' | 'intent', placeholderName: string) => {
  if (type === 'system') {
    insertPlaceholder(placeholderName);
  } else if (type === 'context') {
    insertContextPlaceholder(placeholderName);
  } else {
    insertGenericPlaceholder(type, placeholderName);
  }
};

// Watch visible to set up the event listeners
watch(() => props.visible, (val) => {
  if (val) {
    nextTick(() => {
      setupTextareaEventListeners();
      setupContextTemplateEventListeners();
      setupGenericTextareaEventListeners('intent', intentPromptPopup, () => filteredIntentPlaceholders.value);
      setupGenericTextareaEventListeners('rewriteSystem', rewriteSystemPopup, () => filteredRewriteSystemPlaceholders.value);
      setupGenericTextareaEventListeners('rewriteUser', rewriteUserPopup, () => filteredRewriteUserPlaceholders.value);
      setupGenericTextareaEventListeners('fallback', fallbackPromptPopup, () => filteredFallbackPlaceholders.value);
    });
  }
});

// Template selection handler
const handleSystemPromptTemplateSelect = (template: PromptTemplate) => {
  formData.value.config.system_prompt = template.content;
  formData.value.config.system_prompt_id = template.id;
};

// "Restore default" for the Agent system prompt:
// when a non-custom agent type is selected, the "default" should be the prompt bound to that type's preset
// (for example Wiki Q&A → wiki_researcher), not the global default: true entry in the
// agent_system_prompt template table. Only when the type is custom or no template is bound to the preset
// does it fall back to the global default template passed by PromptTemplateSelector.
const handleAgentSystemPromptResetDefault = (fallback: PromptTemplate) => {
  const typeId = agentType.value;
  if (typeId && typeId !== 'custom') {
    const preset = agentTypePresets.value.find(p => p.id === typeId);
    const presetPromptId = preset?.config?.system_prompt_id;
    if (presetPromptId) {
      const tmpl = agentSystemPromptTemplates.value.find(t => t.id === presetPromptId);
      if (tmpl && typeof tmpl.content === 'string') {
        formData.value.config.system_prompt = tmpl.content;
        formData.value.config.system_prompt_id = tmpl.id;
        return;
      }
    }
  }
  // Fallback: no suitable type preset, so use the global default PromptTemplateSelector found
  formData.value.config.system_prompt = fallback.content;
  formData.value.config.system_prompt_id = fallback.id;
};

const handleContextTemplateSelect = (template: PromptTemplate) => {
  formData.value.config.context_template = template.content;
  formData.value.config.context_template_id = template.id;
};

const handleRewriteTemplateSelect = (template: PromptTemplate) => {
  // Rewrite templates contain both content (system) and user fields
  formData.value.config.rewrite_prompt_system = template.content;
  if (template.user) {
    formData.value.config.rewrite_prompt_user = template.user;
  }
};

const handleFallbackResponseTemplateSelect = (template: PromptTemplate) => {
  formData.value.config.fallback_response = template.content;
};

const handleFallbackPromptTemplateSelect = (template: PromptTemplate) => {
  formData.value.config.fallback_prompt = template.content;
};

// Helper: check whether a prompt contains the given placeholder
const hasPlaceholder = (text: string | undefined, placeholder: string): boolean => {
  if (!text) return false;
  return text.includes(`{{${placeholder}}}`);
};

const handleSave = async () => {
  // Validate required fields (built-in agents skip name and system prompt validation)
  if (!isBuiltinAgent.value) {
    if (!formData.value.name || !formData.value.name.trim()) {
      MessagePlugin.error(t('agent.editor.nameRequired'));
      currentSection.value = 'basic';
      return;
    }

    // A custom agent must have a system prompt
    if (!formData.value.config.system_prompt || !formData.value.config.system_prompt.trim()) {
      MessagePlugin.error(t('agent.editor.systemPromptRequired'));
      currentSection.value = 'prompts';
      return;
    }

    // A custom agent in normal mode must have a context template
    if (!isAgentMode.value && (!formData.value.config.context_template || !formData.value.config.context_template.trim())) {
      MessagePlugin.error(t('agent.editor.contextTemplateRequired'));
      currentSection.value = 'prompts';
      return;
    }
  }

  // Validate placeholders (normal mode + multi-turn conversation rewrite on)
  if (!isAgentMode.value && formData.value.config.multi_turn_enabled && formData.value.config.enable_rewrite) {
    const rewritePrompt = formData.value.config.rewrite_prompt_user || '';
    // Validate only when the user customized the rewrite prompt
    if (rewritePrompt.trim()) {
      if (!hasPlaceholder(rewritePrompt, 'query')) {
        MessagePlugin.error(t('agent.editor.queryMissingInRewrite'));
        currentSection.value = 'prompts';
        return;
      }
    }
  }

  // Validate placeholders (when the fallback strategy is model-generated)
  if (!isAgentMode.value && formData.value.config.fallback_strategy === 'model') {
    const fallbackPrompt = formData.value.config.fallback_prompt || '';
    // Validate only when the user customized the fallback prompt
    if (fallbackPrompt.trim() && !hasPlaceholder(fallbackPrompt, 'query')) {
      MessagePlugin.error(t('agent.editor.queryMissingInFallback'));
      currentSection.value = 'prompts';
      return;
    }
  }

  if (!formData.value.config.model_id && !builtinDefaultModel('KnowledgeQA')) {
    MessagePlugin.error(t('agent.editor.modelRequired'));
    currentSection.value = 'model';
    return;
  }

  // Validate the VLM model (required when image upload is enabled)
  if (formData.value.config.image_upload_enabled && !formData.value.config.vlm_model_id) {
    MessagePlugin.error(t('agentEditor.imageUpload.vlmModelRequired'));
    currentSection.value = 'multimodal';
    return;
  }

  // The ReRank model is used on demand by run scope: it is not needed when the knowledge base scope is none or
  // search_knowledge is not enabled; otherwise the chat entry gives a clear hint before use.

  formData.value.config.question_suggestions.starters.items =
    formData.value.config.question_suggestions.starters.items
      .map((p: string) => p.trim())
      .filter(Boolean);

  if (!formData.value.config.intent_prompts || Object.keys(formData.value.config.intent_prompts).length === 0) {
    delete formData.value.config.intent_prompts;
  }

  pruneSelectedSkills()

  if (hostOnly.value) formData.value.config.sandbox_config_id = ''

  const payload = { ...formData.value, config: serializeAgentPrompts(formData.value.config, promptTemplates.value) };
  saving.value = true;
  try {
    if (editorMode.value === 'create') {
      const result: any = await createAgent(payload);
      const created = result?.data as CustomAgent | undefined;
      if (!created?.id) {
        throw new Error(result?.message || t('agent.messages.saveFailed'));
      }
      savedAgent.value = created;
      formData.value.id = created.id;
      modalShell.markClean();
      markContextualGuideDone('agentCreate')
      currentSection.value = 'basic';
      void loadAgentIntegrationCounts(created.id);
      MessagePlugin.success(t('agent.messages.created'));
      emit('success', created);
    } else {
      await updateAgent(formData.value.id, payload);
      MessagePlugin.success(t('agent.messages.updated'));
      emit('success');
      handleClose();
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('agent.messages.saveFailed'));
  } finally {
    saving.value = false;
  }
};
</script>

<style scoped lang="less">
// Reuse the create knowledge base styles
/* Left nav: aligned with the "Avatar → Settings" modal */
.section--prompts {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.prompts-panel {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.prompts-panel__header {
  flex-shrink: 0;
  margin: 0 0 0;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
}

.prompts-panel__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  margin: 0 -4px;
  padding: 4px 4px 8px;
}

.prompts-panel__pane {
  &.setting-row:last-child {
    border-bottom: none;
  }
}

.prompts-panel__pane--stack {
  .setting-row:last-child {
    border-bottom: none;
  }
}

.section-header--compact {
  margin-bottom: 0;

  h2 {
    margin-bottom: 4px;
  }

  .section-description {
    font-size: var(--app-text-md);
  }
}

.prompts-outline {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
  min-width: 0;

  &__pill {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 5px 12px;
    border: none;
    border-radius: var(--app-radius-sm);
    background: var(--td-bg-color-secondarycontainer);
    font: inherit;
    font-size: var(--app-text-md);
    line-height: 1.4;
    color: var(--td-text-color-secondary);
    cursor: pointer;
    transition: color var(--app-motion-fast) ease, background var(--app-motion-fast) ease;

    &:hover,
    &:focus-visible {
      color: var(--td-brand-color);
      background: color-mix(in srgb, var(--td-brand-color) 8%, var(--td-bg-color-secondarycontainer));
      outline: none;
    }

    &--active {
      background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
      color: var(--td-brand-color);
      font-weight: 500;
    }
  }

  &__dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--td-brand-color);
    flex-shrink: 0;
  }
}

.content-wrapper {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
  padding: 28px 40px 48px;
  box-sizing: border-box;
  scroll-padding-bottom: 24px;

  &--prompts {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding-bottom: 28px;
  }
}

.section {
  width: 100%;
  animation: sectionFadeIn 0.25s ease;
}

@keyframes sectionFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.section-header {
  margin-bottom: 20px;

  .section-header-title {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;

    h2 {
      margin: 0;
    }
  }

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

    .doc-link {
      margin-left: 8px;
    }
  }
}

// settings-group styles, consistent with the knowledge base settings
.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.parser-policy-block {
  padding: 16px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &__header {
    margin-bottom: 12px;

    label {
      display: block;
      font-size: var(--app-text-lg);
      font-weight: 500;
      color: var(--td-text-color-primary);
      margin-bottom: 4px;
    }

    .desc {
      margin: 0;
      font-size: var(--app-text-md);
      color: var(--td-text-color-secondary);
      line-height: 1.5;
    }
  }
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 16px 0;
  border-bottom: 1px solid var(--td-component-stroke);
  min-width: 0;

  &:last-child {
    border-bottom: none;
  }

  &.setting-row-vertical {
    flex-direction: column;
    gap: 12px;

    .setting-info {
      max-width: 100%;
      padding-right: 0;
    }
  }

  // Emphasized row: for key configs such as "Agent type" that affect users most and need to stand out.
  // Visually only a very light emphasis: a 3px brand-color bar on the left + a bold label.
  &.setting-row--emphasize {
    position: relative;
    padding-left: 14px;

    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 18px;
      bottom: 18px;
      width: 3px;
      border-radius: 2px;
      background: var(--td-brand-color);
    }

    .setting-info label {
      font-weight: 600;
    }
  }

  &.setting-row--field-highlight {
    border-radius: var(--app-radius-sm);
    animation: agent-field-flash 0.8s ease-in-out 3;
  }
}

@keyframes agent-field-flash {
  0%,
  100% {
    background-color: transparent;
    box-shadow: none;
  }

  50% {
    background-color: var(--td-warning-color-light);
    box-shadow: inset 0 0 0 1px rgba(237, 123, 47, 0.35);
  }
}

.setting-info {
  flex: 0 0 42%;
  max-width: 42%;
  min-width: 0;
  padding-right: 0;

  &.full-width {
    max-width: 100%;
    padding-right: 0;
  }

  .setting-info-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 4px;

    label {
      margin-bottom: 0;
    }
  }

  label {
    font-size: var(--app-text-lg);
    font-weight: 500;
    color: var(--td-text-color-primary);
    display: block;
    margin-bottom: 4px;

    .required {
      color: var(--td-error-color);
      margin-left: 2px;
    }
  }

  .desc {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;

    .hint {
      color: var(--td-warning-color);
    }
  }
}

.setting-control {
  flex: 1 1 58%;
  min-width: 0;
  max-width: 58%;
  display: flex;
  justify-content: flex-end;
  align-items: flex-start;
  overflow: hidden;

  .reasoning-effort-select {
    width: 100%;
    max-width: 220px;
  }

  &.setting-control-full {
    width: 100%;
    min-width: 100%;
    max-width: 100%;
    justify-content: flex-start;
  }

  &.max-tokens-control {
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;

    :deep(.t-input-number) {
      width: 140px;
    }
  }

  // Make select and input fill the control area
  :deep(.t-select),
  :deep(.t-input),
  :deep(.t-textarea) {
    width: 100%;
    min-width: 0;
  }

  :deep(.t-select-input) {
    min-width: 0;
  }

  :deep(.t-select .t-tag) {
    max-width: 160px;
  }

  :deep(.t-select .t-tag__text) {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  :deep(.t-input-number) {
    width: 120px;
  }
}

.integration-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;

  &__stat {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);

    &.integration-inline__link {
      display: inline-flex;
      align-items: center;
      gap: 2px;
      padding: 0;
      border: none;
      background: transparent;
      line-height: 1;
      color: var(--td-brand-color);
      cursor: pointer;

      &:hover {
        opacity: 0.85;
      }
    }
  }

  &__sep {
    color: var(--td-component-stroke);
    font-size: var(--app-text-sm);
  }

  &__link {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    margin-left: 4px;
    padding: 0;
    border: none;
    background: transparent;
    font-size: var(--app-text-md);
    line-height: 1;
    color: var(--td-brand-color);
    cursor: pointer;

    &:hover {
      opacity: 0.85;
    }

    :deep(.t-icon) {
      display: block;
    }
  }
}

.select-option-with-tag {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 8px;
}

.go-settings-link {
  font-size: var(--app-text-sm);
  color: var(--td-brand-color);
  margin-top: 0;
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}

.sandbox-select-links {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}

.sandbox-select-links__sep {
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
}

.sandbox-select-control {
  flex-direction: column;
  align-items: flex-end;
}

.sandbox-config-select {
  width: 280px;
}

.sandbox-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  min-width: 0;
}

.sandbox-option__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
}

.sandbox-option__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sandbox-option__type {
  flex-shrink: 0;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
}

.sandbox-option__target {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
  line-height: 1.35;
}

.sandbox-selected-meta {
  margin: 6px 0 0;
  max-width: 280px;
  font-size: var(--app-text-sm);
  line-height: 1.45;
  color: var(--td-text-color-secondary);
  word-break: break-word;
}

// Name input with an avatar preview
.name-input-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;

  .name-input {
    flex: 1;
  }
}

.agent-id-field {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
  padding: 6px 8px 6px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);

  .agent-id-value {
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

  .agent-id-copy {
    flex-shrink: 0;
    color: var(--td-text-color-secondary);

    &:hover {
      color: var(--td-brand-color);
    }
  }
}

/* Scrollbar: consistent with the settings modal */
.content-wrapper::-webkit-scrollbar {
  width: 6px;
}

.content-wrapper::-webkit-scrollbar-track {
  background: var(--td-bg-color-container);
}

.content-wrapper::-webkit-scrollbar-thumb {
  background: var(--td-gray-color-5);
  border-radius: 3px;
}

.content-wrapper::-webkit-scrollbar-thumb:hover {
  background: var(--td-gray-color-6);
}

// Mode hint styles
.mode-hint {
  display: flex;
  align-items: center;
  padding: 10px 14px;
  background: var(--td-success-color-light);
  border-radius: var(--app-radius-sm);
  border: 1px solid var(--td-success-color-focus);
  color: var(--td-brand-color);
  font-size: var(--app-text-md);
  line-height: 1.5;
}

// Transition animation
// Slider styles
.slider-wrapper {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;

  :deep(.t-slider) {
    flex: 1;
  }
}

.slider-value {
  width: 40px;
  text-align: right;
  font-family: var(--app-font-family-mono);
  font-size: var(--app-text-base);
  color: var(--td-text-color-primary);
}

.max-tokens-value {
  font-family: var(--app-font-family-mono);
  font-variant-numeric: tabular-nums;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
}

// Suggested question list
.suggested-prompts-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.prompt-item {
  display: flex;
  align-items: center;
  gap: 8px;

  :deep(.t-input) {
    flex: 1;
  }
}

// Opening / after-answer suggestions are split by top tabs (modeled on model management), avoiding a whole surrounding box
.suggestion-tabs {
  margin-bottom: 4px;

  :deep(.t-tabs__nav-item) {
    font-size: var(--app-text-base);
  }

  :deep(.t-tabs__operations) {
    display: none;
  }

  // Tabs are only for navigation; the content renders itself below
  :deep(.t-tabs__content) {
    display: none;
  }
}

.suggestion-advanced-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 0 2px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
  line-height: 18px;

  &::before,
  &::after {
    content: '';
    flex: 1;
    height: 1px;
    background: var(--td-component-stroke);
  }

  span {
    flex-shrink: 0;
    color: var(--td-text-color-secondary);
    font-weight: 500;
  }
}

// The count badge sits right next to the label, so space-between does not fling it away in a full-width row
// It needs the same specificity as the base `.setting-info .setting-info-header` (space-between) to override it
.setting-info-header.setting-info-header--inline {
  justify-content: flex-start;
  gap: 8px;
}

.curated-items-count {
  flex-shrink: 0;
  padding: 0 8px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-secondarycontainer);
  font-size: var(--app-text-sm);
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-secondary);
}

.suggestion-checkboxes {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

// ===== Tool config: overview panel =====
.tools-overview {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 12px;
  padding: 12px 14px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-lg);
  border: 1px solid var(--td-component-stroke);
}

.tools-overview-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 12px;

  &--preset {
    border-top: 1px dashed var(--td-component-stroke);
    padding-top: 10px;
    justify-content: space-between;
  }
}

.tools-status-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-pill);
  border: 1px solid var(--td-component-stroke);

  .t-icon {
    color: var(--td-text-color-secondary);
    font-size: var(--app-text-base);
  }

  .tools-status-metric {
    display: inline-flex;
    align-items: baseline;
    gap: 4px;

    strong {
      font-size: var(--app-text-base);
      font-weight: 600;
      color: var(--td-text-color-primary);
    }
  }

  .tools-status-sep {
    color: var(--td-text-color-placeholder);
  }

  &--warn {
    color: var(--td-warning-color);
    background: var(--td-warning-color-1);
    border-color: var(--td-warning-color-light);

    .t-icon {
      color: var(--td-warning-color);
    }
  }
}

// ===== Card grid by group =====
.tool-groups {
  display: flex;
  flex-direction: column;
  gap: 22px;
  width: 100%;
}

.tool-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tool-group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 2px;

  .tool-group-bar {
    display: inline-block;
    width: 3px;
    height: 14px;
    border-radius: 2px;
    background: var(--td-brand-color);
  }

  .tool-group-title {
    font-size: var(--app-text-md);
    font-weight: 600;
    color: var(--td-text-color-primary);
    letter-spacing: 0.2px;
  }

  .tool-group-count {
    min-width: 20px;
    padding: 0 6px;
    font-size: var(--app-text-xs);
    color: var(--td-text-color-secondary);
    background: var(--td-bg-color-secondarycontainer);
    border-radius: var(--app-radius-pill);
    text-align: center;
    line-height: 18px;
  }

  .tool-group-warning {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    font-size: var(--app-text-sm);
    color: var(--td-warning-color);
    background: var(--td-warning-color-1);
    border: 1px solid var(--td-warning-color-light);
    border-radius: var(--app-radius-pill);

    .t-icon {
      font-size: var(--app-text-md);
    }
  }
}

// Left color bar per group
.tool-group--base .tool-group-bar {
  background: var(--td-gray-color-6);
}

.tool-group--rag .tool-group-bar {
  background: var(--td-brand-color);
}

.tool-group--wiki_read .tool-group-bar {
  background: var(--td-success-color);
}

.tool-group--wiki_edit .tool-group-bar {
  background: var(--td-warning-color);
}

.tool-group--wiki_issue .tool-group-bar {
  background: var(--td-purple-5, #8e56dd);
}

.tool-group--data .tool-group-bar {
  background: var(--td-cyan-6, #09a3b7);
}

// A uniform two-column grid; single column on small screens
.tool-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  width: 100%;

  @media (max-width: 720px) {
    grid-template-columns: 1fr;
  }
}

// ===== Tool cards (based on the t-checkbox label structure) =====
.tool-card {
  margin: 0; // clear TDesign checkbox's default margin
  padding: 12px 14px;
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-md);
  border: 1px solid var(--td-component-stroke);
  transition: border-color var(--app-motion-base), background var(--app-motion-base);
  cursor: pointer;
  overflow: hidden;

  &:hover:not(.tool-card--disabled) {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-1);
  }

  // Reshape the checkbox's check box + label
  :deep(.t-checkbox__input) {
    margin-top: 2px;
    flex-shrink: 0;
  }

  :deep(.t-checkbox__label) {
    flex: 1;
    min-width: 0;
    padding-left: 10px;
  }

  &.t-is-checked {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-1);
  }

  &--disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }

  &--danger {
    border-color: var(--td-warning-color-light);

    &:hover:not(.tool-card--disabled) {
      border-color: var(--td-warning-color);
      background: var(--td-warning-color-1);
    }

    &.t-is-checked {
      border-color: var(--td-warning-color);
      background: var(--td-warning-color-1);
    }
  }
}

.tool-card-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.tool-card-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.tool-card-name {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 0 1 auto;
  min-width: 0;
}

.tool-card-badge {
  flex: 0 0 auto;
  font-size: 10.5px;
  line-height: 1;
  padding: 3px 6px;
  color: var(--td-warning-color);
  background: transparent;
  border: 1px solid var(--td-warning-color-light);
  border-radius: var(--app-radius-xs);
  letter-spacing: 0.3px;
}

.tool-card-desc {
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.tool-card-hint {
  font-size: var(--app-text-xs);
  color: var(--td-warning-color);
  font-style: italic;
  line-height: 1.4;
}

.tool-card--disabled {

  .tool-card-name,
  .tool-card-desc {
    color: var(--td-text-color-placeholder);
  }
}

// ===== Effective tools preview (chip group) =====
.effective-tools {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 12px;
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-md);
  border: 1px dashed var(--td-component-stroke);
  min-height: 52px;
  align-items: flex-start;
}

.effective-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  font-size: var(--app-text-sm);
  line-height: 18px;
  color: var(--td-brand-color);
  background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--td-brand-color) 22%, transparent);
  border-radius: var(--app-radius-pill);
  max-width: 100%;
}

.effective-chip-label {
  font-weight: 500;
}

.effective-chip-reason {
  font-size: var(--app-text-xs);
  color: var(--td-warning-color);
  font-style: normal;

  &::before {
    content: "· ";
    color: var(--td-text-color-placeholder);
    margin-right: 2px;
  }
}

.effective-chip--inactive {
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-secondarycontainer);
  border-color: var(--td-component-stroke);

  .effective-chip-label {
    text-decoration: line-through;
  }
}

.effective-tools-empty {
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
  font-style: italic;
}

.skill-pick-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
}

.skill-pick-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skill-pick-group__header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 2px;
}

.skill-pick-group__bar {
  display: inline-block;
  width: 3px;
  height: 14px;
  border-radius: 2px;
  background: var(--td-brand-color);
}

.skill-pick-group--pending .skill-pick-group__bar {
  background: var(--td-text-color-placeholder);
}

.skill-pick-group__title {
  font-size: var(--app-text-md);
  font-weight: 600;
  color: var(--td-text-color-primary);
  letter-spacing: 0.2px;
}

.skill-pick-group__count {
  min-width: 20px;
  padding: 0 6px;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-pill);
  text-align: center;
  line-height: 18px;
}

.skill-pick {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 8px 10px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
}

.skill-pick--pending {
  background: var(--td-bg-color-secondarycontainer);
}

.skill-pick__check {
  flex-shrink: 0;
}

.skill-pick__badge {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border-radius: 7px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
}

.skill-pick--ready .skill-pick__badge {
  background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
  color: var(--td-brand-color);
}

.skill-pick--pending .skill-pick__badge {
  background: var(--td-bg-color-container);
  color: var(--td-text-color-placeholder);
}

.skill-pick__body {
  flex: 1;
  min-width: 0;
}

.skill-pick__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.skill-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.skill-pick__hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);

  .t-icon {
    flex-shrink: 0;
  }
}

.skill-pick__hint--upgrade {
  color: var(--td-warning-color);
}

.skill-pick__hint--busy {
  color: var(--td-brand-color);

  .t-icon {
    animation: wk-spin 1s linear infinite;
  }
}

.skill-desc {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  margin: 2px 0 0;
  overflow: hidden;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
  line-height: 1.45;
  white-space: pre-line;
  word-break: break-word;
}

.hint-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border: none;
  border-radius: var(--app-radius-xs);
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: help;
  line-height: 1;

  &:hover,
  &:focus-visible {
    color: var(--td-brand-color);
    outline: none;
  }
}

.hint-popover {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 340px;
}

.hint-popover__title {
  margin: 0;
  color: var(--td-text-color-primary);
  font-size: var(--app-text-md);
  font-weight: 600;
}

.hint-popover__text {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
  line-height: 1.55;
}

.empty-hint {
  color: var(--td-text-color-placeholder);
  font-style: italic;

  .go-settings-link {
    display: inline-block;
    font-style: normal;
  }
}

// textarea and template selector container
.textarea-with-template {
  position: relative;
  width: 100%;
}

.intent-prompts-editor {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.intent-toggle-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.intent-toggle-group :deep(.intent-toggle-btn--active) {
  background-color: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
  border-color: var(--td-brand-color);
  color: var(--td-brand-color);
  font-weight: 500;

  &:hover,
  &:focus-visible {
    background-color: color-mix(in srgb, var(--td-brand-color) 14%, transparent);
    border-color: var(--td-brand-color);
    color: var(--td-brand-color);
  }
}

.intent-toggle-btn {
  max-width: 100%;
}

.intent-toggle-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.intent-toggle-dot {
  flex-shrink: 0;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
}

.intent-active-desc {
  margin: 0;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
  line-height: 1.5;
}

// System prompt input styles
.system-prompt-textarea {
  width: 100%;
  font-family: var(--app-font-family-mono);
  font-size: var(--app-text-md);

  :deep(textarea) {
    resize: vertical !important;
    min-height: 200px;
  }
}

// Placeholder tag group styles
.placeholder-tags {
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: var(--app-text-sm);
  line-height: 1.4;
  overflow-x: auto;
  white-space: nowrap;
  padding-bottom: 4px;

  // Hide the scrollbar but keep it scrollable
  scrollbar-width: thin;

  &::-webkit-scrollbar {
    height: 4px;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(0, 0, 0, 0.1);
    border-radius: 2px;
  }

  .placeholder-label {
    color: var(--td-text-color-secondary);
    flex-shrink: 0;
  }

  .placeholder-hint {
    color: var(--td-text-color-placeholder);
    font-size: var(--app-text-xs);
    user-select: none;
    flex-shrink: 0;
  }

  .placeholder-tag {
    display: inline-flex;
    align-items: center;
    padding: 1px 5px;
    border-radius: 3px;
    font-family: var(--app-font-family-mono);
    font-size: var(--app-text-xs);
    color: var(--td-text-color-primary);
    background-color: var(--td-bg-color-secondarycontainer);
    cursor: pointer;
    transition: all var(--app-motion-base);
    user-select: none;
    border: 1px solid transparent;
    flex-shrink: 0;

    &:hover {
      color: var(--td-brand-color);
      background-color: var(--td-brand-color-light);
      border-color: var(--td-brand-color-focus);
    }

    &:active {
      background-color: var(--td-brand-color-focus);
    }
  }
}

.placeholder-popup-wrapper {
  position: fixed;
  z-index: 10001;
  pointer-events: auto;
}

.placeholder-popup {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
  max-width: 320px;
  max-height: 240px;
  overflow-y: auto;
  padding: 4px;
}

.placeholder-item {
  padding: 6px 10px;
  cursor: pointer;
  transition: background-color var(--app-motion-fast);
  border-radius: var(--app-radius-xs);

  &:hover,
  &.active {
    background-color: var(--td-bg-color-container-hover);
  }

  .placeholder-name {
    margin-bottom: 2px;

    code {
      background: var(--td-bg-color-container-hover);
      padding: 2px 5px;
      border-radius: 3px;
      font-family: var(--app-font-family-mono);
      font-size: var(--app-text-xs);
      color: var(--td-brand-color);
    }
  }

  .placeholder-desc {
    font-size: var(--app-text-xs);
    color: var(--td-text-color-secondary);
  }
}

.builtin-agent-hint {
  display: inline-flex;
  align-items: center;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-2xl);
  line-height: 1;
  cursor: help;
  transition: color var(--app-motion-base);

  &:hover,
  &:focus-visible {
    color: var(--td-warning-color);
    outline: none;
  }

  &:focus-visible {
    border-radius: 2px;
    box-shadow: 0 0 0 2px var(--td-warning-color-focus);
  }
}

// Built-in agent avatar
.builtin-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: var(--app-radius-xl);
  flex-shrink: 0;

  &.normal {
    background: linear-gradient(135deg, color-mix(in srgb, var(--td-brand-color) 15%, transparent) 0%, color-mix(in srgb, var(--td-brand-color) 8%, transparent) 100%);
    color: var(--td-brand-color-active);
  }

  &.agent {
    background: linear-gradient(135deg, rgba(124, 77, 255, 0.15) 0%, rgba(124, 77, 255, 0.08) 100%);
    color: var(--td-brand-color);
  }
}

// Prompt switch
.prompt-toggle {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 12px;

  .prompt-toggle-label {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);
  }
}

// Prompt disabled hint
.prompt-disabled-hint {
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-md);
  font-style: italic;
  padding: 12px 16px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-sm);
}

// System prompt tabs
.system-prompt-tabs {
  width: 100%;

  .prompt-variant-tabs {
    :deep(.t-tabs__nav) {
      margin-bottom: 12px;
    }
  }
}

// Knowledge base option styles
.kb-option-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 2px 0;
}

.kb-option-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  border-radius: var(--app-radius-sm);
  font-size: var(--app-text-base);

  // Document KB
  &.doc-icon {
    background: rgba(16, 185, 129, 0.1);
    color: var(--td-success-color);
  }

  // FAQ KB
  &.faq-icon {
    background: rgba(0, 82, 217, 0.1);
    color: var(--td-brand-color);
  }
}

.kb-option-label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--app-text-md);
  color: var(--td-text-color-primary);
}

.kb-option-org {
  flex-shrink: 0;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-secondarycontainer);
  padding: 1px 6px;
  border-radius: var(--app-radius-xs);
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-option-disabled-hint {
  flex-shrink: 0;
  font-size: var(--app-text-xs);
  color: var(--td-warning-color-6);
  background: var(--td-warning-color-1);
  padding: 1px 6px;
  border-radius: var(--app-radius-xs);
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-type-preset-desc {
  margin-top: 4px;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
  line-height: 1.5;
}

.agent-type-select {
  width: 100%;
  min-width: 240px;
  max-width: 360px;
}

.kb-option-count {
  flex-shrink: 0;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-secondarycontainer);
  padding: 1px 6px;
  border-radius: var(--app-radius-xs);
}

.kb-option-tag {
  flex-shrink: 0;
  font-size: var(--app-text-2xs);
  font-weight: 500;
  padding: 0 5px;
  border-radius: 3px;
  line-height: 18px;
}

.tag-rag {
  color: #165dff;
  background: rgba(22, 93, 255, 0.1);
}

.tag-wiki {
  color: #00b42a;
  background: rgba(0, 180, 42, 0.1);
}

</style>

<!-- Non-scoped styles: TDesign teleports the popup outside this component, so
     scoped selectors can't reach .agent-type-popup .t-select-option. -->
<style lang="less">
.reasoning-level-select-popup {
  padding: 4px;

  .t-select-option {
    height: auto !important;
    padding: 6px 10px;
    border-radius: 6px;
    margin: 2px 0;
    white-space: normal;
  }
}

.reasoning-level-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.35;
  min-width: 0;

  &__title {
    font-size: var(--app-text-md);
    color: var(--td-text-color-primary);
  }

  &__hint {
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
    word-break: break-word;
  }
}

.agent-type-popup {
  .t-select-option {
    // The default option is a 32px single line; we want two lines, so drop the fixed height and loosen the padding
    height: auto !important;
    min-height: 48px;
    line-height: 1.4;
    padding: 8px 12px;
    white-space: normal;
  }
}

.sandbox-config-select-popup {
  .t-select-option {
    height: auto;
    padding: 6px 10px;
  }
}

.agent-type-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  width: 100%;
}

.agent-type-option-label {
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.4;
}

.agent-type-option-desc {
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
  line-height: 1.4;
  white-space: normal;
  overflow-wrap: break-word;
}
</style>
