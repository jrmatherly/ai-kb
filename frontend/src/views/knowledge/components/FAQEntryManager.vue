<template>
  <div class="faq-manager">
    <div class="faq-content">
      <!-- Header -->
      <div class="faq-header">
        <div class="faq-header-title">
          <div class="faq-title-row">
            <h2 class="faq-breadcrumb">
              <button type="button" class="breadcrumb-link" @click="handleNavigateToKbList">
                {{ $t('menu.knowledgeBase') }}
              </button>
              <t-icon name="chevron-right" class="breadcrumb-separator" />
              <KBSwitcherDropdown
                v-if="knowledgeList.length"
                :kb-list="knowledgeList"
                :current-kb-id="props.kbId"
                @select="(id) => handleKnowledgeDropdownSelect({ value: id })"
              >
                <button type="button" class="breadcrumb-link dropdown" :disabled="!props.kbId">
                  <template v-if="!kbInfo">
                    <t-skeleton animation="gradient" :row-col="[{ width: '120px', height: '20px' }]" />
                  </template>
                  <template v-else>
                    <span>{{ kbInfo.name }}</span>
                    <t-icon name="chevron-down" />
                  </template>
                </button>
              </KBSwitcherDropdown>
              <button v-else type="button" class="breadcrumb-link" :disabled="!props.kbId"
                @click="handleNavigateToCurrentKB">
                <template v-if="!kbInfo">
                  <t-skeleton animation="gradient" :row-col="[{ width: '120px', height: '20px' }]" />
                </template>
                <template v-else>
                  {{ kbInfo.name }}
                </template>
              </button>
              <t-icon name="chevron-right" class="breadcrumb-separator" />
              <span class="breadcrumb-current">{{ $t('knowledgeEditor.faq.title') }}</span>
            </h2>
            <div class="kb-title-actions">
              <KBInfoPopover
                v-if="kbInfo && !authStore.isLiteMode"
                :kb-info="kbInfo"
              />
              <t-tooltip v-if="canManage" :content="$t('knowledgeBase.settings')" placement="top">
                <button type="button" class="kb-settings-button" @click="handleOpenKBSettings">
                  <t-icon name="setting" size="16px" />
                </button>
              </t-tooltip>
              <!-- Import result: icon only by default, details expand on hover / click -->
              <div v-if="showImportResultBadge" class="faq-import-host"
                :class="{ 'is-expanded': importResultExpanded }">
                <button type="button" class="faq-import-trigger"
                  :aria-label="$t('faqManager.import.recentResult')"
                  @click.stop="importResultExpanded = !importResultExpanded">
                  <t-icon name="check-circle-filled" size="16px" />
                </button>
                <div class="faq-import-panel">
                  <div class="faq-import-strip faq-import-strip--result faq-import-strip--panel">
                    <span class="faq-import-strip__text">{{ importResultSummary }}</span>
                    <t-tag size="small" variant="light"
                      :theme="importResult!.import_mode === 'append' ? 'primary' : 'warning'">
                      {{ importResult!.import_mode === 'append' ? $t('faqManager.import.appendMode') :
                        $t('faqManager.import.replaceMode') }}
                    </t-tag>
                    <t-button v-if="importResult!.failed_entries_url && importResult!.failed_count > 0"
                      variant="text" theme="danger" size="small" class="faq-import-strip__link"
                      @click="downloadFailedEntries">
                      {{ $t('faqManager.import.downloadReasons') }}
                    </t-button>
                    <span class="faq-import-strip__time">{{ formatImportTime(importResult!.imported_at) }}</span>
                    <button type="button" class="faq-import-strip__close" :aria-label="$t('common.close')"
                      @click="closeImportResult">
                      <t-icon name="close" size="14px" />
                    </button>
                  </div>
                </div>
              </div>
              <!-- Import in progress -->
              <div v-else-if="isImportInProgress && importState.taskStatus"
                class="faq-import-strip faq-import-strip--in-title"
                :class="`faq-import-strip--${importState.taskStatus.status}`">
                <t-icon :name="importProgressIcon" size="16px" class="faq-import-strip__icon"
                  :class="{ 'is-spinning': importState.taskStatus.status === 'running' }" />
                <span class="faq-import-strip__text">{{ importProgressText }}</span>
                <div class="faq-import-strip__bar">
                  <div class="faq-import-strip__bar-fill" :style="{ width: `${importState.taskStatus.progress}%` }" />
                </div>
                <span class="faq-import-strip__count">{{ importState.taskStatus.processed }}/{{
                  importState.taskStatus.total }}</span>
              </div>
            </div>
          </div>
          <p class="faq-subtitle">{{ $t('knowledgeEditor.faq.subtitle') }}</p>
        </div>
      </div>

      <div class="faq-main">
        <div class="faq-card-area">
          <!-- Search bar and tag filter -->
          <div class="faq-filter-bar">
            <t-input v-model.trim="entrySearchKeyword" :placeholder="$t('knowledgeEditor.faq.searchPlaceholder')"
              clearable class="faq-search-input" @clear="loadEntries()" @enter="loadEntries()">
              <template #prefix-icon>
                <t-icon name="search" size="16px" />
              </template>
            </t-input>
            <div class="faq-filter-bar__filters">
              <KnowledgeTagFilter v-model:search="tagSearchQuery" v-model:cleared="tagFilterCleared"
                :tags="tagList" :selected-ids="selectedTagIds" :total="tagTotal"
                :loading="tagLoading" :loading-more="tagLoadingMore" :has-more="tagHasMore"
                :can-manage="canEdit" variant="faq" @change="handleTagFilterChange"
                @load-more="loadTags()" @manage="openTagManageDrawer" />
            </div>
            <div class="faq-filter-bar__trailing">
              <!-- New: new entry / import -->
              <template v-if="faqCreateOptions.length">
                <t-tooltip :content="$t('knowledgeEditor.faq.createGroup')" placement="top">
                  <t-dropdown :options="faqCreateOptions" trigger="click" placement="bottom-right"
                    @click="handleFaqAction">
                    <t-button variant="text" theme="default" class="content-bar-icon-btn" size="small">
                      <template #icon><t-icon name="add" size="16px" /></template>
                    </t-button>
                  </t-dropdown>
                </t-tooltip>
              </template>
              <!-- Export -->
              <t-dropdown :options="faqExportOptions" trigger="click" placement="bottom-right"
                @click="handleFaqAction">
                <t-tooltip :content="$t('knowledgeEditor.faqExport.exportButton')" placement="top">
                  <t-button variant="text" theme="default" class="content-bar-icon-btn" size="small"
                    :loading="exportLoading">
                    <template #icon><t-icon name="download" size="16px" /></template>
                  </t-button>
                </t-tooltip>
              </t-dropdown>
              <!-- Search -->
              <t-tooltip :content="$t('knowledgeEditor.faq.searchTest')" placement="top">
                <t-button variant="text" theme="default" class="content-bar-icon-btn" size="small"
                  @click="handleFaqAction({ value: 'search' })">
                  <template #icon><t-icon name="search" size="16px" /></template>
                </t-button>
              </t-tooltip>
            </div>
          </div>
          <!-- Card List Container with Scroll -->
          <div ref="scrollContainer" class="faq-scroll-container"
            :class="{ 'has-batch-bar': selectedRowKeys.length > 0 && canSelectEntries }" @scroll="handleScroll">
            <!-- FAQ skeleton -->
            <div v-if="loading && entries.length === 0" class="faq-skeleton-grid">
              <div v-for="n in 6" :key="'faq-skel-' + n" class="faq-card faq-card-skeleton">
                <div class="faq-card-header">
                  <t-skeleton animation="gradient" :row-col="[{ width: '80%', height: '16px' }]" />
                </div>
                <div class="faq-card-body">
                  <t-skeleton animation="gradient"
                    :row-col="[{ width: '100%', height: '13px' }, { width: '90%', height: '13px' }, { width: '60%', height: '13px' }]" />
                </div>
                <div class="faq-skel-footer">
                  <t-skeleton animation="gradient"
                    :row-col="[[{ width: '50px', height: '18px', type: 'rect' }, { width: '60px', height: '18px', type: 'rect' }]]" />
                </div>
              </div>
            </div>
            <!-- Card List -->
            <template v-else-if="entries.length > 0">
              <div ref="cardListRef" class="faq-card-list">
                <div v-for="entry in entries" :key="entry.id" class="faq-card"
                  :class="{ 'selected': selectedRowKeys.includes(entry.id), 'is-selectable': canSelectEntries }"
                  @click="handleCardSelect(entry.id, !selectedRowKeys.includes(entry.id))">
                  <!-- Card Header -->
                  <div class="faq-card-header">
                    <div class="faq-header-top">
                      <div class="faq-question" :title="entry.standard_question">
                        {{ entry.standard_question }}
                      </div>
                      <div class="faq-card-actions">
                        <t-popup v-if="canManage" v-model="entry.showMore" overlayClassName="card-more-popup"
                          trigger="click" destroy-on-close placement="bottom-right"
                          @visible-change="(visible: boolean) => (entry.showMore = visible)">
                          <div class="card-more-btn" @click.stop>
                            <img class="more-icon" src="@/assets/img/more.png" alt="" />
                          </div>
                          <template #content>
                            <div class="popup-menu" @click.stop>
                              <div class="popup-menu-item" @click.stop="handleMenuEdit(entry)">
                                <t-icon class="menu-icon" name="edit" />
                                <span>{{ $t('common.edit') }}</span>
                              </div>
                              <div class="popup-menu-item delete" @click.stop="handleMenuDelete(entry)">
                                <t-icon class="menu-icon" name="delete" />
                                <span>{{ $t('common.delete') }}</span>
                              </div>
                            </div>
                          </template>
                        </t-popup>
                      </div>
                    </div>
                  </div>

                  <!-- Card Body -->
                  <div class="faq-card-body">
                    <!-- Similar Questions Section -->
                    <div v-if="entry.similar_questions?.length" class="faq-section similar">
                      <div class="faq-section-label clickable"
                        @click.stop="entry.similarCollapsed = !entry.similarCollapsed">
                        <span>{{ $t('knowledgeEditor.faq.similarQuestions') }}</span>
                        <span class="section-count">
                          ({{ entry.similar_questions.length }})
                        </span>
                        <t-icon :name="entry.similarCollapsed ? 'chevron-right' : 'chevron-down'"
                          class="collapse-icon" />
                      </div>
                      <Transition name="slide-down">
                        <div v-if="!entry.similarCollapsed" class="faq-tags">
                          <FAQTagTooltip v-for="question in entry.similar_questions" :key="question" :content="question"
                            type="similar" placement="top">
                            <t-tag size="small" variant="light-outline" class="question-tag">
                              {{ question }}
                            </t-tag>
                          </FAQTagTooltip>
                        </div>
                      </Transition>
                    </div>

                    <!-- Negative Questions Section -->
                    <div v-if="entry.negative_questions?.length" class="faq-section negative">
                      <div class="faq-section-label clickable"
                        @click.stop="entry.negativeCollapsed = !entry.negativeCollapsed">
                        <span>{{ $t('knowledgeEditor.faq.negativeQuestions') }}</span>
                        <span class="section-count">
                          ({{ entry.negative_questions.length }})
                        </span>
                        <t-icon :name="entry.negativeCollapsed ? 'chevron-right' : 'chevron-down'"
                          class="collapse-icon" />
                      </div>
                      <Transition name="slide-down">
                        <div v-if="!entry.negativeCollapsed" class="faq-tags">
                          <FAQTagTooltip v-for="question in entry.negative_questions" :key="question"
                            :content="question" type="negative" placement="top">
                            <t-tag size="small" theme="warning" variant="light-outline" class="question-tag">
                              {{ question }}
                            </t-tag>
                          </FAQTagTooltip>
                        </div>
                      </Transition>
                    </div>

                    <!-- Answers Section -->
                    <div class="faq-section answers">
                      <div class="faq-section-label clickable"
                        @click.stop="entry.answersCollapsed = !entry.answersCollapsed">
                        <span>{{ $t('knowledgeEditor.faq.answers') }}</span>
                        <span v-if="entry.answers?.length" class="section-count">
                          ({{ entry.answers.length }})
                        </span>
                        <t-icon :name="entry.answersCollapsed ? 'chevron-right' : 'chevron-down'"
                          class="collapse-icon" />
                      </div>
                      <Transition name="slide-down">
                        <div v-if="!entry.answersCollapsed" class="faq-tags">
                          <FAQTagTooltip v-for="answer in entry.answers" :key="answer" :content="answer" type="answer"
                            placement="top">
                            <t-tag size="small" theme="success" variant="light-outline" class="question-tag">
                              {{ answer }}
                            </t-tag>
                          </FAQTagTooltip>
                        </div>
                      </Transition>
                    </div>
                  </div>

                  <!-- Card Footer -->
                  <div class="faq-card-footer">
                    <div class="faq-card-tag" @click.stop>
                      <template v-if="canEdit && tagList.length">
                        <t-dropdown :options="tagDropdownOptions" trigger="click"
                          @click="(data: any) => handleEntryTagChange(entry.id, data.value as string)">
                          <t-tag size="small" variant="light-outline" class="faq-tag-chip">
                            <span class="tag-text">{{ getTagName(entry.tag_id) || $t('knowledgeBase.untagged') }}</span>
                          </t-tag>
                        </t-dropdown>
                      </template>
                      <template v-else>
                        <t-tag size="small" variant="light-outline" class="faq-tag-chip">
                          <span class="tag-text">{{ getTagName(entry.tag_id) || $t('knowledgeBase.untagged') }}</span>
                        </t-tag>
                      </template>
                    </div>
                    <div class="faq-card-status" @click.stop>
                      <t-tooltip
                        :content="entry.is_enabled ? $t('knowledgeEditor.faq.statusEnabled') : $t('knowledgeEditor.faq.statusDisabled')"
                        placement="top">
                        <div class="status-item-compact">
                          <t-switch :key="`${entry.id}-${entry.is_enabled}`" size="small" :value="entry.is_enabled"
                            :loading="!!entryStatusLoading[entry.id]"
                            :disabled="!!entryStatusLoading[entry.id] || !canEdit" @click.stop
                            @change="(value: boolean) => handleEntryStatusChange(entry, value)" />
                        </div>
                      </t-tooltip>
                    </div>
                  </div>
                </div>
              </div>
            </template>
            <template v-else-if="!loading">
              <div class="faq-empty-state">
                <div class="empty-content">
                  <t-icon name="file-add" size="48px" class="empty-icon" />
                  <div class="empty-text">{{ $t('knowledgeEditor.faq.emptyTitle') }}</div>
                  <div class="empty-desc">{{ $t('knowledgeEditor.faq.emptyDesc') }}</div>
                </div>
              </div>
            </template>
            <div v-if="loadingMore" class="faq-load-more">
              <t-loading size="small" :text="$t('common.loading')" />
            </div>
            <div v-if="hasMore === false && entries.length > 0" class="faq-no-more">
              {{ $t('common.noMoreData') }}
            </div>
          </div>
          <div class="faq-batch-bar-anchor">
            <FAQBatchBar :count="selectedRowKeys.length" :enabled-count="selectedEnabledCount"
              :disabled-count="selectedDisabledCount" :can-edit="canEdit" :can-manage="canManage"
              :tag-loading="batchTagLoading" :status-action="batchStatusAction"
              :delete-loading="batchDeleteLoading" @cancel="clearFAQSelection" @batch-tag="openBatchTagDialog"
              @enable="handleBatchStatusChange(true)" @disable="handleBatchStatusChange(false)"
              @delete="handleBatchDelete" />
          </div>
        </div>
      </div>
    </div>
    <!-- Editor Drawer -->
    <t-drawer v-model:visible="editorVisible"
      :header="editorMode === 'create' ? $t('knowledgeEditor.faq.editorCreate') : $t('knowledgeEditor.faq.editorEdit')"
      :close-btn="true" size="520px" placement="right" class="faq-editor-drawer" @close="handleEditorClose">
      <div class="faq-editor-drawer-content">
        <t-form ref="editorFormRef" :data="editorForm" :rules="editorRules" layout="vertical" :label-width="0"
          class="faq-editor-form">
          <div class="settings-group">
            <!-- Standard question -->
            <div class="setting-row vertical setting-row-primary">
              <div class="setting-info">
                <label class="required-label">
                  {{ $t('knowledgeEditor.faq.standardQuestion') }}
                  <span class="required-mark">*</span>
                </label>
                <p class="desc">{{ $t('knowledgeEditor.faq.standardQuestionDesc') }}</p>
              </div>
              <div class="setting-control">
                <t-input v-model="editorForm.standard_question" :maxlength="200" class="full-width-input" />
              </div>
            </div>

            <!-- Similar questions -->
            <div class="setting-row vertical setting-row-optional setting-row-similar">
              <div class="setting-info">
                <label class="optional-label">{{ $t('knowledgeEditor.faq.similarQuestions') }}</label>
                <p class="desc optional-desc">{{ $t('knowledgeEditor.faq.similarQuestionsDesc') }}</p>
              </div>
              <div class="setting-control">
                <div class="full-width-input-wrapper">
                  <t-input v-model="similarInput" :placeholder="$t('knowledgeEditor.faq.similarPlaceholder')"
                    @enter="addSimilar" class="full-width-input" />
                  <t-button theme="primary" variant="outline"
                    :disabled="!similarInput.trim() || editorForm.similar_questions.length >= 10" @click="addSimilar"
                    class="add-item-btn" size="small">
                    <t-icon name="add" size="16px" />
                  </t-button>
                </div>
                <div v-if="editorForm.similar_questions.length > 0" class="item-list">
                  <div v-for="(question, index) in editorForm.similar_questions" :key="index" class="item-row">
                    <div class="item-content">{{ question }}</div>
                    <t-button theme="default" variant="text" size="small" @click="removeSimilar(index)"
                      class="remove-item-btn">
                      <t-icon name="close" size="16px" />
                    </t-button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Negative examples -->
            <div class="setting-row vertical setting-row-optional setting-row-negative">
              <div class="setting-info">
                <label class="optional-label">{{ $t('knowledgeEditor.faq.negativeQuestions') }}</label>
                <p class="desc optional-desc">{{ $t('knowledgeEditor.faq.negativeQuestionsDesc') }}</p>
              </div>
              <div class="setting-control">
                <div class="full-width-input-wrapper">
                  <t-input v-model="negativeInput" :placeholder="$t('knowledgeEditor.faq.negativePlaceholder')"
                    @enter="addNegative" class="full-width-input" />
                  <t-button theme="primary" variant="outline"
                    :disabled="!negativeInput.trim() || editorForm.negative_questions.length >= 10" @click="addNegative"
                    class="add-item-btn" size="small">
                    <t-icon name="add" size="16px" />
                  </t-button>
                </div>
                <div v-if="editorForm.negative_questions.length > 0" class="item-list">
                  <div v-for="(question, index) in editorForm.negative_questions" :key="index"
                    class="item-row negative">
                    <div class="item-content">{{ question }}</div>
                    <t-button theme="default" variant="text" size="small" @click="removeNegative(index)"
                      class="remove-item-btn">
                      <t-icon name="close" size="16px" />
                    </t-button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Answers -->
            <div class="setting-row vertical setting-row-primary setting-row-answer">
              <div class="setting-info">
                <label class="required-label">
                  {{ $t('knowledgeEditor.faq.answers') }}
                  <span class="required-mark">*</span>
                </label>
                <p class="desc">{{ $t('knowledgeEditor.faq.answersDesc') }}</p>
              </div>
              <div class="setting-control">
                <div class="textarea-container">
                  <div class="full-width-input-wrapper textarea-wrapper">
                    <t-textarea v-model="answerInput" :placeholder="$t('knowledgeEditor.faq.answerPlaceholder')"
                      :autosize="{ minRows: 3, maxRows: 6 }" class="full-width-textarea" @keydown.ctrl.enter="addAnswer"
                      @keydown.meta.enter="addAnswer" />
                    <t-button theme="primary" variant="outline"
                      :disabled="!answerInput.trim() || editorForm.answers.length >= 5" @click="addAnswer"
                      class="add-item-btn" size="small">
                      <t-icon name="add" size="16px" />
                    </t-button>
                  </div>
                  <div class="item-count">{{ editorForm.answers.length }}/5</div>
                </div>
                <div v-if="editorForm.answers.length > 0" class="item-list">
                  <div v-for="(answer, index) in editorForm.answers" :key="index" class="item-row answer-row">
                    <div class="item-content">{{ answer }}</div>
                    <t-button theme="default" variant="text" size="small" @click="removeAnswer(index)"
                      class="remove-item-btn">
                      <t-icon name="close" size="16px" />
                    </t-button>
                  </div>
                </div>
              </div>
            </div>

            <div class="setting-row vertical">
              <div class="setting-info">
                <label>{{ $t('knowledgeBase.tagLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.faq.tagDesc') }}</p>
              </div>
              <div class="setting-control">
                <t-select v-model="editorForm.tag_id" class="full-width-input" :options="tagSelectOptions" clearable
                  :placeholder="$t('knowledgeEditor.faq.tagPlaceholder')" />
              </div>
            </div>
          </div>
        </t-form>
      </div>

      <template #footer>
        <div class="faq-editor-drawer-footer">
          <t-button theme="default" variant="outline" @click="editorVisible = false">
            {{ $t('common.cancel') }}
          </t-button>
          <t-button theme="primary" @click="handleSubmitEntry" :loading="savingEntry">
            {{ editorMode === 'create' ? $t('knowledgeEditor.faq.editorCreate') : $t('common.save') }}
          </t-button>
        </div>
      </template>
    </t-drawer>

    <!-- Import Dialog -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="importVisible" class="faq-import-overlay" @click.self="importVisible = false">
          <div class="faq-import-modal">
            <!-- Close button -->
            <button class="close-btn" @click="importVisible = false" :aria-label="$t('general.close')">
              <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
              </svg>
            </button>

            <div class="faq-import-container">
              <div class="faq-import-header">
                <h2 class="import-title">{{ $t('knowledgeEditor.faqImport.title') }}</h2>
              </div>

              <div class="faq-import-content">
                <!-- Import mode selection -->
                <div class="import-form-item">
                  <label class="import-form-label required">{{ $t('knowledgeEditor.faqImport.modeLabel') }}</label>
                  <t-radio-group v-model="importState.mode" class="import-radio-group">
                    <t-radio-button value="append">{{ $t('knowledgeEditor.faqImport.appendMode') }}</t-radio-button>
                    <t-radio-button value="replace">{{ $t('knowledgeEditor.faqImport.replaceMode') }}</t-radio-button>
                  </t-radio-group>
                </div>

                <!-- File upload area -->
                <div class="import-form-item">
                  <div class="file-label-row">
                    <label class="import-form-label required">{{ $t('knowledgeEditor.faqImport.fileLabel') }}</label>
                    <t-dropdown :options="downloadExampleOptions" placement="bottom-right" trigger="click"
                      @click="handleDownloadExample" class="download-example-dropdown">
                      <t-button theme="default" variant="outline" size="small" class="download-example-btn">
                        <t-icon name="download" size="16px" />
                        <span>{{ $t('knowledgeEditor.faqImport.downloadExample') }}</span>
                      </t-button>
                    </t-dropdown>
                  </div>
                  <div class="file-upload-wrapper">
                    <input ref="fileInputRef" type="file" accept=".json,.csv,.xlsx,.xls" @change="handleFileChange"
                      class="file-input-hidden" />
                    <div class="file-upload-area" :class="{ 'has-file': importState.file }"
                      @click="fileInputRef?.click()" @dragover.prevent @dragenter.prevent
                      @drop.prevent="handleFileDrop">
                      <div class="file-upload-content">
                        <t-icon name="upload" size="32px" class="upload-icon" />
                        <div class="upload-text">
                          <span v-if="!importState.file" class="upload-primary-text">
                            {{ $t('knowledgeEditor.faqImport.clickToUpload') }}
                          </span>
                          <span v-else class="upload-file-name">
                            {{ importState.file.name }}
                          </span>
                          <span v-if="!importState.file" class="upload-secondary-text">
                            {{ $t('knowledgeEditor.faqImport.dragDropTip') }}
                          </span>
                        </div>
                      </div>
                    </div>
                    <p class="import-form-tip">{{ $t('knowledgeEditor.faqImport.fileTip') }}</p>
                  </div>
                </div>

                <!-- Preview area -->
                <div v-if="importState.preview.length" class="import-preview">
                  <div class="preview-header">
                    <t-icon name="file-view" size="16px" class="preview-icon" />
                    <span class="preview-title">
                      {{ $t('knowledgeEditor.faqImport.previewCount', { count: importState.preview.length }) }}
                    </span>
                  </div>
                  <div class="preview-list">
                    <div v-for="(item, index) in importState.preview.slice(0, 5)" :key="index" class="preview-item">
                      <span class="preview-index">{{ index + 1 }}</span>
                      <span class="preview-question">{{ item.standard_question }}</span>
                    </div>
                  </div>
                  <p v-if="importState.preview.length > 5" class="preview-more">
                    {{ $t('knowledgeEditor.faqImport.previewMore', { count: importState.preview.length - 5 }) }}
                  </p>
                </div>

              </div>

              <div class="faq-import-footer">
                <t-button theme="default" variant="outline" @click="handleCancelImport"
                  :disabled="importState.importing && importState.taskStatus?.status === 'running'">
                  {{ $t('common.cancel') }}
                </t-button>
                <t-button theme="primary" @click="handleImport" :loading="importState.importing && !importState.taskId"
                  :disabled="importState.taskStatus?.status === 'running'">
                  {{ importState.taskStatus?.status === 'success' ? $t('common.close') :
                    importState.taskStatus?.status === 'failed' ? $t('common.retry') :
                      $t('knowledgeEditor.faqImport.importButton') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Batch Tag Dialog -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="batchTagDialogVisible" class="batch-tag-overlay" @click.self="batchTagDialogVisible = false">
          <div class="batch-tag-modal">
            <!-- Close button -->
            <button class="batch-tag-close-btn" @click="batchTagDialogVisible = false"
              :aria-label="$t('general.close')">
              <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
              </svg>
            </button>

            <div class="batch-tag-container">
              <div class="batch-tag-header">
                <h2 class="batch-tag-title">{{ $t('knowledgeEditor.faq.batchUpdateTag') }}</h2>
              </div>

              <div class="batch-tag-content">
                <div class="batch-tag-tip">
                  <t-icon name="info-circle" size="16px" class="tip-icon" />
                  <span>{{ $t('knowledgeEditor.faq.batchUpdateTagTip', { count: selectedRowKeys.length }) }}</span>
                </div>
                <t-form layout="vertical" class="batch-tag-form">
                  <t-form-item :label="$t('knowledgeBase.tagLabel')">
                    <t-select v-model="batchTagValue" :options="tagSelectOptions"
                      :placeholder="$t('knowledgeBase.tagPlaceholder')" clearable filterable class="batch-tag-select">
                      <template #empty>
                        <div class="tag-select-empty">
                          {{ $t('knowledgeBase.noTags') }}
                        </div>
                      </template>
                    </t-select>
                  </t-form-item>
                </t-form>
              </div>

              <div class="batch-tag-footer">
                <t-button theme="default" variant="outline" @click="batchTagDialogVisible = false">
                  {{ $t('common.cancel') }}
                </t-button>
                <t-button theme="primary" :loading="batchTagLoading" :disabled="batchActionLoading"
                  @click="handleBatchTag">
                  {{ $t('common.confirm') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Search Test Drawer -->
    <t-drawer v-model:visible="searchDrawerVisible" :header="$t('knowledgeEditor.faq.searchTestTitle')"
      :close-btn="true" size="420px" placement="right" class="faq-search-drawer">
      <div class="search-test-content">
        <t-form layout="vertical" class="search-form" :label-width="0">
          <div class="settings-group">
            <!-- Query text -->
            <div class="setting-row vertical search-first-row">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.faq.queryLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.faq.queryPlaceholder') }}</p>
              </div>
              <div class="setting-control">
                <t-input v-model="searchForm.query" :placeholder="$t('knowledgeEditor.faq.queryPlaceholder')"
                  @enter="handleSearch" class="full-width-input" />
              </div>
            </div>

            <!-- Similarity threshold -->
            <div class="setting-row vertical">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.faq.similarityThresholdLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.faq.vectorThresholdDesc') }}</p>
              </div>
              <div class="setting-control">
                <div class="slider-wrapper">
                  <t-slider v-model="searchForm.vectorThreshold" :min="0" :max="1" :step="0.1" :show-tooltip="true"
                    :format-tooltip="(val: number) => val.toFixed(2)" />
                  <div class="slider-value">{{ searchForm.vectorThreshold.toFixed(2) }}</div>
                </div>
              </div>
            </div>

            <!-- Match count -->
            <div class="setting-row vertical">
              <div class="setting-info">
                <label>{{ $t('knowledgeEditor.faq.matchCountLabel') }}</label>
                <p class="desc">{{ $t('knowledgeEditor.faq.matchCountDesc') }}</p>
              </div>
              <div class="setting-control">
                <div class="slider-wrapper">
                  <t-slider v-model="searchForm.matchCount" :min="1" :max="50" :step="1" :show-tooltip="true" />
                  <div class="slider-value">{{ searchForm.matchCount }}</div>
                </div>
              </div>
            </div>

            <!-- Search button -->
            <div class="setting-row vertical">
              <div class="setting-control">
                <t-button theme="primary" block :loading="searching" @click="handleSearch" class="search-button">
                  {{ searching ? $t('knowledgeEditor.faq.searching') : $t('knowledgeEditor.faq.searchButton') }}
                </t-button>
              </div>
            </div>
          </div>
        </t-form>

        <!-- Search Results -->
        <div v-if="searchResults.length > 0 || hasSearched" class="search-results">
          <div class="results-header">
            <span>{{ $t('knowledgeEditor.faq.searchResults') }} ({{ searchResults.length }})</span>
          </div>
          <div v-if="searchResults.length === 0" class="no-results">
            {{ $t('knowledgeEditor.faq.noResults') }}
          </div>
          <div v-else class="results-list">
            <div v-for="(result, index) in searchResults" :key="result.id" class="result-card"
              :class="{ 'expanded': result.expanded }">
              <div class="result-header" @click="toggleResult(result)">
                <div class="result-question-wrapper">
                  <div class="result-main">
                    <div class="result-question">
                      <span class="result-index">{{ index + 1 }}.</span>
                      {{ result.standard_question }}
                    </div>
                    <div v-if="result.matched_question && result.matched_question !== result.standard_question"
                      class="matched-question">
                      <span class="matched-label">{{ $t('knowledgeEditor.faq.matchedQuestion') }}:</span>
                      <span class="matched-text">{{ result.matched_question }}</span>
                    </div>
                  </div>
                  <div class="result-meta">
                    <t-tag size="small" variant="light-outline" class="score-tag">
                      {{ (result.score || 0).toFixed(3) }}
                    </t-tag>
                  </div>
                  <t-icon :name="result.expanded ? 'chevron-up' : 'chevron-down'" class="expand-icon" />
                </div>
              </div>
              <Transition name="slide-down">
                <div v-if="result.expanded" class="result-body">
                  <div v-if="result.answers?.length" class="result-section">
                    <div class="section-label">{{ $t('knowledgeEditor.faq.answers') }}</div>
                    <div class="result-tags">
                      <t-tooltip v-for="answer in result.answers" :key="answer" :content="answer" placement="top">
                        <t-tag size="small" theme="success" variant="light" class="answer-tag">
                          {{ answer }}
                        </t-tag>
                      </t-tooltip>
                    </div>
                  </div>
                  <div v-if="result.similar_questions?.length" class="result-section">
                    <div class="section-label">{{ $t('knowledgeEditor.faq.similarQuestions') }}</div>
                    <div class="result-tags">
                      <t-tooltip v-for="question in result.similar_questions" :key="question" :content="question"
                        placement="top">
                        <t-tag size="small" variant="light-outline" class="question-tag">
                          {{ question }}
                        </t-tag>
                      </t-tooltip>
                    </div>
                  </div>
                </div>
              </Transition>
            </div>
          </div>
        </div>
      </div>
    </t-drawer>

    <KbTagManageDrawer
      v-model:visible="tagManageDrawerVisible"
      :kb-id="props.kbId"
      :is-faq="true"
      @changed="onTagManageChanged"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed, nextTick, onUnmounted, h } from 'vue'
import KnowledgeTagFilter from './KnowledgeTagFilter.vue'
import { MessagePlugin, Icon as TIcon } from 'tdesign-vue-next'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'
import type { FormRules, FormInstanceFunctions } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationStore } from '@/stores/organization'
import { useChatResourcesStore } from '@/stores/chatResources'
import {
  listFAQEntries,
  upsertFAQEntries,
  createFAQEntry,
  updateFAQEntry,
  updateFAQEntryFieldsBatch,
  deleteFAQEntries,
  searchFAQEntries,
  exportFAQEntries,
  listKnowledgeTags,
  updateFAQEntryTagBatch,
  getKnowledgeBaseById,
  getFAQImportProgress,
  updateFAQImportResultDisplayStatus,
} from '@/api/knowledge-base'
import * as XLSX from 'xlsx'
import {
  type FAQEntryPayload,
  parseFAQImportCSV,
  parseFAQImportJSON,
  parseFAQImportWorkbook,
} from '@/utils/faqImport'
import FAQTagTooltip from '@/components/FAQTagTooltip.vue'
import KBInfoPopover from '@/components/KBInfoPopover.vue'
import KBSwitcherDropdown from '@/components/KBSwitcherDropdown.vue'
import FAQBatchBar from './FAQBatchBar.vue'
import KbTagManageDrawer from './KbTagManageDrawer.vue'
import { useUIStore } from '@/stores/ui'

interface FAQEntry {
  id: number
  chunk_id: string
  knowledge_id: string
  knowledge_base_id: string
  tag_id?: number
  is_enabled: boolean
  is_recommended: boolean
  standard_question: string
  similar_questions: string[]
  negative_questions: string[]
  answers: string[]
  updated_at: string
  showMore?: boolean
  score?: number
  match_type?: string
  matched_question?: string
  expanded?: boolean
  similarCollapsed?: boolean
  negativeCollapsed?: boolean
  answersCollapsed?: boolean
}

const props = defineProps<{
  kbId: string
}>()

const { t } = useI18n()
const confirmDelete = useConfirmDelete()
const router = useRouter()
const uiStore = useUIStore()
const authStore = useAuthStore()
const orgStore = useOrganizationStore()
const chatResources = useChatResourcesStore()

// Permission control: check if current user owns this KB or has edit/manage permission.
//
// isOwner used to compare kbInfo.tenant_id against the user's effective tenant id,
// which silently treated "any KB visible to me in my current tenant" as "I created
// it" — Viewer / Contributor in their home tenant ended up showing every FAQ
// CRUD entry on every KB and 403'ing when they clicked. Mirror the rule we settled
// on in KnowledgeBase.vue: explicit creator_id match, with role / org-share fallbacks
// inside canEdit / canManage. Legacy KBs with empty creator_id stay tenant-owned
// (Admin+ may manage).
const isOwner = computed(() => {
  if (!kbInfo.value) return false
  const creatorId = (kbInfo.value as any).creator_id || ''
  const userId = authStore.user?.id || ''
  if (!creatorId) return false
  return creatorId === userId
})

// Current KB's shared record (when accessed via organization share)
const currentSharedKb = computed(() =>
  orgStore.sharedKnowledgeBases.find((s) => s.knowledge_base?.id === props.kbId) ?? null,
)

// Accessed via organization share: presence in the sharedKnowledgeBases list
// means we reached this KB through a shared space, so the user's local tenant
// role is irrelevant — only the share grant counts. tenant_id comparison
// alone is unreliable (a user can be a member of both source and receiving
// tenants); share-list presence is the authoritative signal.
const isViaShare = computed(() => !!currentSharedKb.value)

// Can edit: when accessed via an organization share, ONLY the share grant
// counts — even if the current user happens to be the original creator of
// the KB. The backend's RBAC middleware authorizes based on the active
// tenant, not on creator_id, so a creator viewing their own KB from a
// different tenant context will be 403'd on write. Otherwise: KB creator
// (any role) or tenant Admin+ in the home tenant.
const canEdit = computed(() => {
  if (isViaShare.value) return orgStore.canEditKB(props.kbId, false)
  if (isOwner.value) return true
  if (authStore.hasRole('admin')) return true
  return orgStore.canEditKB(props.kbId, false)
})

// Can manage (delete, settings, share): same isViaShare-first rule. For
// shared KBs only an 'admin' share grant qualifies — editor/viewer (and
// even being the creator viewed via share) never grant delete/settings.
const canManage = computed(() => {
  if (isViaShare.value) return orgStore.canManageKB(props.kbId, false)
  if (isOwner.value) return true
  if (authStore.hasRole('admin')) return true
  return orgStore.canManageKB(props.kbId, false)
})

const canSelectEntries = computed(() => canEdit.value || canManage.value)

const faqExportOptions = computed(() => [
  { content: t('knowledgeEditor.faqExport.exportCSV'), value: 'export_csv' },
  { content: t('knowledgeEditor.faqExport.exportJSON'), value: 'export_json' },
])

// FAQ actions: the new group (new entry + import)
const faqCreateOptions = computed(() => {
  if (!canEdit.value) return []
  return [
    { content: t('knowledgeEditor.faq.editorCreate'), value: 'create', prefixIcon: () => h(TIcon, { name: 'add', size: '16px' }) },
    { content: t('knowledgeEditor.faqImport.importButton'), value: 'import', prefixIcon: () => h(TIcon, { name: 'upload', size: '16px' }) },
  ]
})

// Handle FAQ actions
const handleFaqAction = (data: { value: string }) => {
  switch (data.value) {
    case 'create':
      openEditor()
      break
    case 'import':
      openImportDialog()
      break
    case 'search':
      searchDrawerVisible.value = true
      break
    case 'export_csv':
      handleExportCSV()
      break
    case 'export_json':
      handleExportJSON()
      break
    case 'export':
      handleExportCSV()
      break
  }
}

const loading = ref(true)
const loadingMore = ref(false)
const entries = ref<FAQEntry[]>([])
const entryStatusLoading = reactive<Record<number, boolean>>({})
const selectedRowKeys = ref<number[]>([])
const batchDeleteLoading = ref(false)
const batchTagLoading = ref(false)
const batchStatusAction = ref<'enable' | 'disable' | null>(null)
const selectedEntries = computed(() => {
  const selectedIds = new Set(selectedRowKeys.value)
  return entries.value.filter(entry => selectedIds.has(entry.id))
})
const selectedEnabledCount = computed(() => (
  selectedEntries.value.filter(entry => entry.is_enabled !== false).length
))
const selectedDisabledCount = computed(() => selectedEntries.value.length - selectedEnabledCount.value)
const batchActionLoading = computed(() => (
  batchDeleteLoading.value
  || batchTagLoading.value
  || batchStatusAction.value != null
))
const scrollContainer = ref<HTMLElement | null>(null)
const cardListRef = ref<HTMLElement | null>(null)
const hasMore = ref(true)
const pageSize = 20
let currentPage = 1
const entrySearchKeyword = ref('')
let entrySearchDebounce: number | null = null

const tagList = ref<any[]>([])
const tagLoading = ref(false)
const selectedTagIds = ref<string[]>([])
const tagFilterCleared = ref(false)
const tagManageDrawerVisible = ref(false)
const overallFAQTotal = ref(0)
const tagSearchQuery = ref('')
const TAG_PAGE_SIZE = 20
const tagPage = ref(1)
const tagHasMore = ref(false)
const tagLoadingMore = ref(false)
const tagTotal = ref(0)
let tagSearchDebounce: number | null = null

// tagMapBySeqId uses seq_id as key for looking up by entry.tag_id
const tagMapBySeqId = computed<Record<number, any>>(() => {
  const map: Record<number, any> = {}
  tagList.value.forEach((tag) => {
    map[tag.seq_id] = tag
  })
  return map
})

const regularTags = computed(() => tagList.value)
const tagDropdownOptions = computed(() =>
  regularTags.value.map((tag: any) => ({ content: tag.name, value: String(tag.seq_id) })),
)
const tagSelectOptions = computed(() =>
  regularTags.value.map((tag: any) => ({ label: tag.name, value: tag.seq_id })),
)

const kbInfo = ref<any>(null)
const knowledgeList = ref<Array<{ id: string; name: string; type?: string }>>([])

const loadKnowledgeInfo = async (kbId: string) => {
  if (!kbId) {
    kbInfo.value = null
    return
  }
  try {
    const res: any = await getKnowledgeBaseById(kbId)
    kbInfo.value = res?.data || null
    return kbInfo.value
  } catch (error) {
    console.error('Failed to load knowledge base info:', error)
    kbInfo.value = null
    return null
  }
}

const loadKnowledgeList = async () => {
  try {
    await chatResources.ensureKnowledgeBases()
    const myKbs: typeof knowledgeList.value = chatResources.rawKnowledgeBases.map((item: any) => ({
      id: String(item.id),
      name: item.name,
      type: item.type,
    }))

    // Also include shared knowledge bases from orgStore
    const sharedKbs: typeof knowledgeList.value = (orgStore.sharedKnowledgeBases || [])
      .filter(s => s.knowledge_base != null)
      .map(s => ({
        id: String(s.knowledge_base.id),
        name: s.knowledge_base.name,
        type: s.knowledge_base.type,
      }))

    // Merge and deduplicate by id (my KBs take precedence)
    const myKbIds = new Set(myKbs.map(kb => kb.id))
    const uniqueSharedKbs = sharedKbs.filter(kb => !myKbIds.has(kb.id))

    knowledgeList.value = [...myKbs, ...uniqueSharedKbs]
  } catch (error) {
    console.error('Failed to load knowledge bases:', error)
  }
}

const editorVisible = ref(false)
const editorMode = ref<'create' | 'edit'>('create')
const currentEntryId = ref<number | null>(null)
const editorForm = reactive<FAQEntryPayload>({
  standard_question: '',
  similar_questions: [],
  negative_questions: [],
  answers: [],
  tag_id: undefined,
})
const editorFormRef = ref<FormInstanceFunctions>()
const savingEntry = ref(false)

// Input state
const answerInput = ref('')
const similarInput = ref('')
const negativeInput = ref('')

const importVisible = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const importState = reactive({
  mode: 'append' as 'append' | 'replace',
  file: null as File | null,
  preview: [] as FAQEntryPayload[],
  importing: false,
  taskId: null as string | null,
  taskStatus: null as {
    status: string
    progress: number
    total: number
    processed: number
    message?: string
    error?: string
  } | null,
  pollingInterval: null as ReturnType<typeof setInterval> | null,
})

// FAQ import result state (persisted)
type FAQImportResultView = {
  total_entries: number
  success_count: number
  failed_count: number
  skipped_count: number
  partial_failed_count: number
  merged_count: number
  added_count: number
  import_mode: string
  imported_at: string
  task_id: string
  processing_time: number
  message?: string
  failed_entries_url?: string
  success_entries?: Array<{
    index: number
    seq_id: number
    tag_id?: number
    tag_name?: string
    standard_question: string
  }>
  display_status: string
}

const importResult = ref<FAQImportResultView | null>(null)
const importResultExpanded = ref(false)

const showImportResultBadge = computed(() => (
  !!importResult.value
  && importResult.value.display_status === 'open'
  && !importState.taskId
))

const isImportInProgress = computed(() => {
  const status = importState.taskStatus?.status
  return !!importState.taskId && (status === 'running' || status === 'pending')
})

const importResultSummary = computed(() => {
  const result = importResult.value
  if (!result) return ''
  if (result.message?.trim()) {
    return result.message.trim()
  }
  const parts: string[] = []
  parts.push(`${t('faqManager.import.totalData')} ${result.total_entries}`)
  if (result.merged_count > 0) {
    if (result.added_count > 0) {
      parts.push(`${t('faqManager.import.added')} ${result.added_count}`)
    }
    parts.push(`${t('faqManager.import.merged')} ${result.merged_count}`)
  } else if (result.success_count > 0) {
    parts.push(`${t('faqManager.import.success')} ${result.success_count}`)
  }
  if (result.partial_failed_count > 0) {
    parts.push(`${t('faqManager.import.partialFailed')} ${result.partial_failed_count}`)
  }
  if (result.failed_count > 0) {
    parts.push(`${t('faqManager.import.failed')} ${result.failed_count}`)
  }
  if (result.skipped_count > 0) {
    parts.push(`${t('faqManager.import.skipped')} ${result.skipped_count}`)
  }
  return parts.join(' · ')
})

const importProgressTitle = computed(() => {
  const status = importState.taskStatus?.status
  if (status === 'running') return t('faqManager.import.importing')
  if (status === 'success') return t('faqManager.import.importDone')
  if (status === 'failed') return t('faqManager.import.importFailed')
  return t('faqManager.import.waiting')
})

const importProgressIcon = computed(() => {
  const status = importState.taskStatus?.status
  if (status === 'running') return 'loading'
  if (status === 'success') return 'check-circle-filled'
  if (status === 'failed') return 'error-circle-filled'
  return 'time-filled'
})

const importProgressText = computed(() => {
  const status = importState.taskStatus
  if (!status) return ''
  if (status.error) return status.error
  if (status.message?.trim()) return status.message.trim()
  return importProgressTitle.value
})

// Search test state
const searchDrawerVisible = ref(false)
const searching = ref(false)
const hasSearched = ref(false)
const searchResults = ref<FAQEntry[]>([])
const searchForm = reactive({
  query: '',
  vectorThreshold: 0.7,
  matchCount: 10,
})

const getTagName = (tagId?: number) => {
  if (!tagId) return t('knowledgeBase.untagged')
  return tagMapBySeqId.value[tagId]?.name || t('knowledgeBase.untagged')
}

const handleTagFilterChange = (tagIds: string[]) => {
  selectedTagIds.value = tagIds
  uiStore.clearSelectedTagIds()
  tagIds.forEach((id) => uiStore.toggleSelectedTagId(id))
}

const openTagManageDrawer = () => {
  tagManageDrawerVisible.value = true
}

const onTagManageChanged = (payload?: { deletedTagId?: string }) => {
  if (!props.kbId) return
  void loadTags(true)
  if (payload?.deletedTagId && selectedTagIds.value.includes(payload.deletedTagId)) {
    selectedTagIds.value = selectedTagIds.value.filter((id) => id !== payload.deletedTagId)
    handleTagFilterChange([...selectedTagIds.value])
  }
  currentPage = 1
  entries.value = []
  selectedRowKeys.value = []
  void loadEntries()
}

const loadTags = async (reset = false) => {
  if (!props.kbId) {
    tagList.value = []
    tagTotal.value = 0
    tagHasMore.value = false
    tagPage.value = 1
    return
  }

  if (reset) {
    tagPage.value = 1
    tagList.value = []
    tagTotal.value = 0
    tagHasMore.value = false
  } else if (tagLoading.value || tagLoadingMore.value) {
    return
  }

  const currentTagPage = tagPage.value || 1
  tagLoading.value = currentTagPage === 1
  tagLoadingMore.value = currentTagPage > 1

  try {
    const res: any = await listKnowledgeTags(props.kbId, {
      page: currentTagPage,
      page_size: TAG_PAGE_SIZE,
      keyword: tagSearchQuery.value || undefined,
    })
    const pageData = (res?.data || {}) as {
      data?: any[]
      total?: number
    }
    const pageTags = (pageData.data || []).map((tag: any) => ({
      ...tag,
      id: String(tag.id),
    }))

    if (currentTagPage === 1) {
      tagList.value = pageTags
    } else {
      tagList.value = [...tagList.value, ...pageTags]
    }

    tagTotal.value = pageData.total || tagList.value.length
    tagHasMore.value = tagList.value.length < tagTotal.value
    if (tagHasMore.value) {
      tagPage.value = currentTagPage + 1
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    tagLoading.value = false
    tagLoadingMore.value = false
  }
}

const handleEntryTagChange = async (entryId: number, value?: string) => {
  if (!props.kbId) return
  const targetEntry = entries.value.find((item) => item.id === entryId)
  const previousTagId = targetEntry ? targetEntry.tag_id : undefined
  const normalizedValue = value ? Number(value) : null
  if (normalizedValue === previousTagId) {
    return
  }
  try {
    await updateFAQEntryTagBatch(props.kbId, { updates: { [entryId]: normalizedValue } })
    MessagePlugin.success(t('knowledgeEditor.messages.updateSuccess'))
    await loadEntries()
    await loadTags(true)
  } catch (error: any) {
    if (targetEntry) {
      targetEntry.tag_id = previousTagId
    }
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  }
}

const handleNavigateToKbList = () => {
  router.push('/platform/knowledge-bases')
}

const handleNavigateToCurrentKB = () => {
  if (!props.kbId) return
  router.push(`/platform/knowledge-bases/${props.kbId}`)
}

const handleOpenKBSettings = () => {
  if (!props.kbId) {
    MessagePlugin.warning(t('knowledgeEditor.messages.missingId'))
    return
  }
  uiStore.openKBSettings(props.kbId)
}

const handleKnowledgeDropdownSelect = (data: { value: string }) => {
  if (!data?.value || data.value === props.kbId) return
  router.push(`/platform/knowledge-bases/${data.value}`)
}

const handleEntryStatusChange = async (entry: FAQEntry, value: boolean) => {
  if (!props.kbId) {
    return
  }
  const entryIndex = entries.value.findIndex(e => e.id === entry.id)
  if (entryIndex === -1) {
    return
  }
  // Get the actual object reference from the array, to make sure the latest data is used
  const actualEntry = entries.value[entryIndex]
  const previous = actualEntry.is_enabled
  if (previous === value) {
    return
  }
  // Update the property directly; Vue 3 reactivity should detect it
  actualEntry.is_enabled = value
  entryStatusLoading[entry.id] = true
  try {
    await updateFAQEntryFieldsBatch(props.kbId, { by_id: { [entry.id]: { is_enabled: value } } })
    MessagePlugin.success(t(value ? 'knowledgeEditor.faq.statusEnableSuccess' : 'knowledgeEditor.faq.statusDisableSuccess'))
  } catch (error: any) {
    // Roll back on failure
    actualEntry.is_enabled = previous
    MessagePlugin.error(error?.message || t('knowledgeEditor.faq.statusUpdateFailed'))
  } finally {
    entryStatusLoading[entry.id] = false
  }
}

const editorRules: FormRules<FAQEntryPayload> = {
  standard_question: [
    { required: true, message: t('knowledgeEditor.messages.nameRequired') },
  ],
  answers: [
    {
      validator: (val: string[]) => Array.isArray(val) && val.length > 0,
      message: t('knowledgeEditor.faq.answerRequired'),
    },
  ],
}

const loadEntries = async (append = false) => {
  if (!props.kbId) return
  if (append) {
    loadingMore.value = true
  } else {
    loading.value = true
    currentPage = 1
    entries.value = []
    selectedRowKeys.value = []
    Object.keys(entryStatusLoading).forEach((key) => {
      delete entryStatusLoading[Number(key)]
    })
  }

  try {
    // If overallFAQTotal is not initialized, fetch it first (without tag_id filter)
    if (overallFAQTotal.value === 0 && !append) {
      const totalRes = await listFAQEntries(props.kbId, {
        page: 1,
        page_size: 1,
      })
      const totalData = (totalRes.data || {}) as { total: number }
      overallFAQTotal.value = totalData.total || 0
    }

    const res = await listFAQEntries(props.kbId, {
      page: currentPage,
      page_size: pageSize,
      tag_ids: selectedTagIds.value.length > 0 ? selectedTagIds.value.join(',') : undefined,
      keyword: entrySearchKeyword.value ? entrySearchKeyword.value.trim() : undefined,
    })
    const pageData = (res.data || {}) as {
      data: FAQEntry[]
      total: number
    }
    const newEntries = (pageData.data || []).map(entry => ({
      ...entry,
      showMore: false,
      similarCollapsed: true,  // similar questions collapsed by default
      negativeCollapsed: true,  // negative examples collapsed by default
      answersCollapsed: true,   // answers collapsed by default
      is_enabled: entry.is_enabled !== false,
    }))

    if (append) {
      entries.value = [...entries.value, ...newEntries]
    } else {
      entries.value = newEntries
    }
    // Whether there is more data
    hasMore.value = entries.value.length < (pageData.total || 0)
    currentPage++

    // Re-layout after the DOM updates
    await nextTick()
    arrangeCards()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    loading.value = false
    loadingMore.value = false

    // Check whether to keep loading to fill the visible area
    // Delayed so that arrangeCards' requestAnimationFrame finishes
    setTimeout(() => {
      checkAndLoadMore()
    }, 350)
  }
}

const handleScroll = () => {
  if (!scrollContainer.value || loadingMore.value || !hasMore.value) return

  const container = scrollContainer.value
  const scrollTop = container.scrollTop
  const scrollHeight = container.scrollHeight
  const clientHeight = container.clientHeight

  // Load more when scrolled within 200px of the bottom
  if (scrollTop + clientHeight >= scrollHeight - 200) {
    loadEntries(true)
  }
}

// Check whether the content fills the visible area; if not and there is more data, keep loading
const checkAndLoadMore = () => {
  if (!scrollContainer.value) return
  if (loadingMore.value || loading.value) return
  if (!hasMore.value) return

  const container = scrollContainer.value
  const scrollHeight = container.scrollHeight
  const clientHeight = container.clientHeight

  // If the content height is below the container height + a 50px buffer, there may be no scrollbar or it is near the bottom, so keep loading
  if (scrollHeight <= clientHeight + 50) {
    loadEntries(true)
  }
}

const handleCardSelect = (entryId: number, checked: boolean) => {
  if (!canSelectEntries.value || batchActionLoading.value) return
  if (checked) {
    if (!selectedRowKeys.value.includes(entryId)) {
      selectedRowKeys.value.push(entryId)
    }
  } else {
    const index = selectedRowKeys.value.indexOf(entryId)
    if (index > -1) {
      selectedRowKeys.value.splice(index, 1)
    }
  }
}

const clearFAQSelection = () => {
  if (batchActionLoading.value) return
  selectedRowKeys.value = []
}

const resetEditorForm = () => {
  editorForm.standard_question = ''
  editorForm.similar_questions = []
  editorForm.negative_questions = []
  editorForm.answers = []
  editorForm.tag_id = undefined
  answerInput.value = ''
  similarInput.value = ''
  negativeInput.value = ''
}

const openEditor = (entry?: FAQEntry) => {
  if (entry) {
    editorMode.value = 'edit'
    currentEntryId.value = entry.id
    editorForm.standard_question = entry.standard_question
    editorForm.similar_questions = [...(entry.similar_questions || [])]
    editorForm.negative_questions = [...(entry.negative_questions || [])]
    editorForm.answers = [...(entry.answers || [])]
    editorForm.tag_id = entry.tag_id || undefined
  } else {
    editorMode.value = 'create'
    currentEntryId.value = null
    resetEditorForm()
  }
  answerInput.value = ''
  similarInput.value = ''
  negativeInput.value = ''
  editorVisible.value = true
}

const handleEditorClose = () => {
  // Reset the form on close
  resetEditorForm()
  answerInput.value = ''
  similarInput.value = ''
  negativeInput.value = ''
  editorFormRef.value?.clearValidate?.()
}

// Add an answer
const addAnswer = () => {
  const trimmed = answerInput.value.trim()
  if (trimmed && editorForm.answers.length < 5 && !editorForm.answers.includes(trimmed)) {
    editorForm.answers.push(trimmed)
    answerInput.value = ''
  }
}

// Delete an answer
const removeAnswer = (index: number) => {
  editorForm.answers.splice(index, 1)
}

// Add a similar question
const addSimilar = () => {
  const trimmed = similarInput.value.trim()
  if (trimmed && editorForm.similar_questions.length < 10 && !editorForm.similar_questions.includes(trimmed)) {
    editorForm.similar_questions.push(trimmed)
    similarInput.value = ''
  }
}

// Delete a similar question
const removeSimilar = (index: number) => {
  editorForm.similar_questions.splice(index, 1)
}

// Add a negative example
const addNegative = () => {
  const trimmed = negativeInput.value.trim()
  if (trimmed && editorForm.negative_questions.length < 10 && !editorForm.negative_questions.includes(trimmed)) {
    editorForm.negative_questions.push(trimmed)
    negativeInput.value = ''
  }
}

// Delete a negative example
const removeNegative = (index: number) => {
  editorForm.negative_questions.splice(index, 1)
}

const handleSubmitEntry = async () => {
  if (!editorFormRef.value) return
  const result = await editorFormRef.value.validate?.()
  if (result !== true) return

  savingEntry.value = true
  try {
    const payload: FAQEntryPayload = {
      standard_question: editorForm.standard_question,
      similar_questions: [...editorForm.similar_questions],
      negative_questions: [...editorForm.negative_questions],
      answers: [...editorForm.answers],
      tag_id: editorForm.tag_id || undefined,
    }
    if (editorMode.value === 'create') {
      await createFAQEntry(props.kbId, payload)
      MessagePlugin.success(t('knowledgeEditor.messages.createSuccess'))
    } else if (currentEntryId.value) {
      await updateFAQEntry(props.kbId, currentEntryId.value, payload)
      MessagePlugin.success(t('knowledgeEditor.messages.updateSuccess'))
    }
    editorVisible.value = false
    await loadEntries()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    savingEntry.value = false
  }
}

const handleBatchDelete = async () => {
  if (!canManage.value || !selectedRowKeys.value.length || !props.kbId || batchActionLoading.value) return
  const selectedIds = [...selectedRowKeys.value]
  batchDeleteLoading.value = true
  try {
    await deleteFAQEntries(props.kbId, selectedIds)
    MessagePlugin.success(t('knowledgeEditor.faq.batchDeleteSuccess', { count: selectedIds.length }))
    selectedRowKeys.value = []
    await loadEntries()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    batchDeleteLoading.value = false
  }
}

// Batch status update dialog
const batchTagDialogVisible = ref(false)
const batchTagValue = ref<string>('')

const openBatchTagDialog = () => {
  if (!canEdit.value || !selectedRowKeys.value.length || batchActionLoading.value) return
  batchTagValue.value = ''
  batchTagDialogVisible.value = true
}

const handleBatchTag = async () => {
  if (!canEdit.value || !selectedRowKeys.value.length || !props.kbId || batchActionLoading.value) return
  const selectedIds = [...selectedRowKeys.value]
  batchTagLoading.value = true
  try {
    const updates: Record<number, number | null> = {}
    selectedIds.forEach(id => {
      updates[id] = batchTagValue.value ? Number(batchTagValue.value) : null
    })
    await updateFAQEntryTagBatch(props.kbId, { updates })
    MessagePlugin.success(t('knowledgeEditor.messages.updateSuccess'))
    batchTagDialogVisible.value = false
    selectedRowKeys.value = []
    await loadEntries()
    await loadTags(true)
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    batchTagLoading.value = false
  }
}

const handleBatchStatusChange = async (isEnabled: boolean) => {
  if (!canEdit.value || !selectedRowKeys.value.length || !props.kbId || batchActionLoading.value) return
  const selectedIds = [...selectedRowKeys.value]
  batchStatusAction.value = isEnabled ? 'enable' : 'disable'
  try {
    const by_id: Record<number, { is_enabled: boolean }> = {}
    selectedIds.forEach(id => {
      by_id[id] = { is_enabled: isEnabled }
    })
    await updateFAQEntryFieldsBatch(props.kbId, { by_id })
    MessagePlugin.success(t(isEnabled ? 'knowledgeEditor.faq.statusEnableSuccess' : 'knowledgeEditor.faq.statusDisableSuccess'))
    selectedRowKeys.value = []
    await loadEntries()
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
  } finally {
    batchStatusAction.value = null
  }
}

const handleMenuEdit = (entry: FAQEntry) => {
  entry.showMore = false
  openEditor(entry)
}

const handleMenuDelete = (entry: FAQEntry) => {
  entry.showMore = false
  confirmDelete({
    body: t('knowledgeEditor.faq.confirmDelete'),
    onConfirm: async () => {
      try {
        await deleteFAQEntries(props.kbId, [entry.id])
        MessagePlugin.success(t('knowledgeEditor.faqImport.deleteSuccess'))
        await loadEntries()
      } catch (error: any) {
        MessagePlugin.error(error?.message || t('common.operationFailed'))
      }
    },
  })
}

const openImportDialog = () => {
  // While an import is running, the import dialog cannot open
  if (importState.taskStatus?.status === 'running') {
    MessagePlugin.warning(t('faqManager.import.importInProgress'))
    return
  }
  stopPolling()
  importVisible.value = true
  importState.file = null
  importState.preview = []
  importState.mode = 'append'
  // Note: taskId and taskStatus are not cleared, so progress stays visible after the dialog closes
  importState.importing = false
}

const processFile = async (file: File) => {
  importState.file = file

  try {
    let parsed: FAQEntryPayload[] = []
    if (file.name.endsWith('.json')) {
      parsed = await parseJSONFile(file)
    } else if (file.name.endsWith('.csv')) {
      parsed = await parseCSVFile(file)
    } else if (file.name.endsWith('.xlsx') || file.name.endsWith('.xls')) {
      parsed = await parseExcelFile(file)
    } else {
      MessagePlugin.warning(t('knowledgeEditor.faqImport.unsupportedFormat'))
      importState.preview = []
      return
    }
    importState.preview = parsed
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('knowledgeEditor.faqImport.parseFailed'))
    importState.preview = []
  }
}

const handleFileChange = async (event: Event) => {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return
  await processFile(file)
}

const handleFileDrop = async (event: DragEvent) => {
  const file = event.dataTransfer?.files[0]
  if (!file) return
  await processFile(file)
}

const parseJSONFile = async (file: File): Promise<FAQEntryPayload[]> => {
  const parsed = parseFAQImportJSON(await file.text())
  if (!parsed) {
    throw new Error(t('knowledgeEditor.faqImport.invalidJSON'))
  }
  return parsed
}

const parseCSVFile = async (file: File) => parseFAQImportCSV(await file.text())

const parseExcelFile = async (file: File) => parseFAQImportWorkbook(await file.arrayBuffer())

const stopPolling = () => {
  if (importState.pollingInterval) {
    clearInterval(importState.pollingInterval)
    importState.pollingInterval = null
  }
}

const startPolling = (taskId: string) => {
  stopPolling()
  // Save the taskId to localStorage so it can be restored after a refresh
  saveTaskIdToStorage(taskId)

  // Remember the last processed count, to decide whether the list needs a refresh
  let lastProcessed = 0

  importState.pollingInterval = setInterval(async () => {
    try {
      const res: any = await getFAQImportProgress(taskId)
      const progressData = res?.data
      if (progressData) {
        // Extract the status from the Redis progress data
        // status: "pending" -> "pending", "processing" -> "running", "completed" -> "success", "failed" -> "failed"
        let status = progressData.status
        if (status === 'processing') {
          status = 'running'
        } else if (status === 'completed') {
          status = 'success'
        }

        const progress = progressData.progress || 0
        const total = progressData.total || 0
        const processed = progressData.processed || 0
        const error = progressData.error || ''
        const message = progressData.message || ''

        importState.taskStatus = {
          status: status,
          progress: progress,
          total: total,
          processed: processed,
          message: message,
          error: error,
        }

        // Refresh the FAQ list on progress updates (once per batch of new entries)
        if (processed > lastProcessed) {
          lastProcessed = processed
          await loadEntries()
          await loadTags(true)
        }

        // The task finished or failed: stop polling (but do not close the progress bar automatically; the user closes it)
        if (status === 'success' || status === 'failed') {
          stopPolling()
          if (status === 'success') {
            // Save the finished taskId for loading the result later
            if (importState.taskId) {
              saveLastCompletedTaskId(importState.taskId)
            }
            MessagePlugin.success(progressData.message || t('knowledgeEditor.faqImport.importSuccess'))
            // Clear the filters so the user sees all the newly imported data
            selectedTagIds.value = []
            tagFilterCleared.value = false
            uiStore.clearSelectedTagIds()
            entrySearchKeyword.value = ''
            overallFAQTotal.value = 0  // Reset to trigger re-fetch
            await loadEntries()
            await loadTags(true)
            await loadImportResult() // load the latest import result stats
            // After the task finishes, close the progress bar automatically after 3 seconds
            setTimeout(() => {
              if (importState.taskStatus?.status === 'success') {
                handleCloseProgress()
              }
            }, 3000)
          } else {
            MessagePlugin.error(error || t('common.operationFailed'))
            // Do not close automatically on failure, so the user sees the error
          }
        }
      }
    } catch (error: any) {
      console.error('Failed to poll task status:', error)
      // If the task does not exist or has expired, clear the storage
      if (error?.response?.status === 404 || error?.message?.includes('not found')) {
        clearTaskIdFromStorage()
        stopPolling()
        importState.taskId = null
        importState.taskStatus = null
      }
    }
  }, 3000) // poll every 3 seconds
}

const handleCancelImport = () => {
  stopPolling()
  importState.importing = false
  importState.taskId = null
  importState.taskStatus = null
  importVisible.value = false
  // Note: localStorage is not cleared, since the task may still be running
}

const handleCloseProgress = () => {
  stopPolling()
  importState.taskId = null
  importState.taskStatus = null
  clearTaskIdFromStorage()
}

// localStorage helpers
const getStorageKey = () => {
  return `faq_import_task_${props.kbId}`
}

const saveTaskIdToStorage = (taskId: string) => {
  if (!props.kbId) return
  try {
    localStorage.setItem(getStorageKey(), taskId)
  } catch (error) {
    console.error('Failed to save taskId to localStorage:', error)
  }
}

const getTaskIdFromStorage = (): string | null => {
  if (!props.kbId) return null
  try {
    return localStorage.getItem(getStorageKey())
  } catch (error) {
    console.error('Failed to get taskId from localStorage:', error)
    return null
  }
}

const clearTaskIdFromStorage = () => {
  if (!props.kbId) return
  try {
    localStorage.removeItem(getStorageKey())
  } catch (error) {
    console.error('Failed to clear taskId from localStorage:', error)
  }
}

// Restore the import task state (for restoring after a refresh)
const restoreImportTask = async () => {
  if (!props.kbId) return

  const savedTaskId = getTaskIdFromStorage()
  if (!savedTaskId) return

  try {
    // Query the progress status in Redis
    const res: any = await getFAQImportProgress(savedTaskId)
    const progressData = res?.data

    if (progressData) {
      // Extract the status from the Redis progress data
      let status = progressData.status
      if (status === 'processing') {
        status = 'running'
      } else if (status === 'completed') {
        status = 'success'
      }

      const progress = progressData.progress || 0
      const total = progressData.total || 0
      const processed = progressData.processed || 0
      const error = progressData.error || ''

      importState.taskId = savedTaskId
      importState.taskStatus = {
        status: status,
        progress: progress,
        total: total,
        processed: processed,
        message: progressData.message || '',
        error: error,
      }

      // If the task is still running, resume polling
      if (status === 'pending' || status === 'running') {
        startPolling(savedTaskId)
      } else {
        // The task finished or failed, clear the storage
        clearTaskIdFromStorage()
      }
    } else {
      // The task does not exist, clear the storage
      clearTaskIdFromStorage()
    }
  } catch (error: any) {
    console.error('Failed to restore import task:', error)
    // If the task does not exist or has expired, clear the storage
    if (error?.response?.status === 404 || error?.message?.includes('not found')) {
      clearTaskIdFromStorage()
    }
  }
}

// localStorage key for last completed task
const getLastCompletedTaskKey = () => {
  return `faq_import_last_completed_${props.kbId}`
}

const saveLastCompletedTaskId = (taskId: string) => {
  if (!props.kbId) return
  try {
    localStorage.setItem(getLastCompletedTaskKey(), taskId)
  } catch (error) {
    console.error('Failed to save last completed taskId:', error)
  }
}

const getLastCompletedTaskId = (): string | null => {
  if (!props.kbId) return null
  try {
    return localStorage.getItem(getLastCompletedTaskKey())
  } catch (error) {
    return null
  }
}

// Load the persisted import result stats
const loadImportResult = async () => {
  if (!props.kbId) return

  const lastTaskId = getLastCompletedTaskId()
  if (!lastTaskId) {
    importResult.value = null
    return
  }

  try {
    const res: any = await getFAQImportProgress(lastTaskId)
    const data = res?.data
    if (data && data.status === 'completed') {
      // Check the display_status the backend returns; hide it if it is close
      if (data.display_status === 'close') {
        importResult.value = null
        return
      }
      // Map progress fields to importResult format
      importResult.value = {
        total_entries: data.total,
        success_count: data.success_count || 0,
        failed_count: data.failed_count || 0,
        skipped_count: data.skipped_count || 0,
        partial_failed_count: data.partial_failed_count || 0,
        merged_count: data.merged_count || 0,
        added_count: data.added_count || 0,
        message: data.message || '',
        import_mode: data.import_mode || 'append',
        imported_at: data.imported_at,
        task_id: data.task_id,
        failed_entries_url: data.failed_entries_url,
        success_entries: data.success_entries,
        display_status: data.display_status || 'open',
        processing_time: data.processing_time || 0,
      }
    } else {
      importResult.value = null
    }
  } catch (error) {
    console.error('Failed to load FAQ import result:', error)
    importResult.value = null
  }
}

// Close the import result stats card
const closeImportResult = async () => {
  importResultExpanded.value = false
  if (!props.kbId) return
  try {
    await updateFAQImportResultDisplayStatus(props.kbId, 'close')
    if (importResult.value) {
      importResult.value.display_status = 'close'
    }
  } catch (error) {
    console.error('Failed to close import result:', error)
  }
}

// Download the failed entries with reasons
const downloadFailedEntries = () => {
  if (!importResult.value?.failed_entries_url) {
    MessagePlugin.warning(t('faqManager.import.noFailedRecords'))
    return
  }
  // Open the download link directly
  window.open(importResult.value.failed_entries_url, '_blank')
}

// Format the import time
const formatImportTime = (timeStr?: string) => {
  if (!timeStr) return ''
  try {
    const date = new Date(timeStr)
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
  } catch (e) {
    return timeStr
  }
}

const handleImport = async () => {
  if (!importState.file || !importState.preview.length) {
    MessagePlugin.warning(t('knowledgeEditor.faqImport.selectFile'))
    return
  }

  // If the task finished or failed, close the dialog
  if (importState.taskStatus?.status === 'success' || importState.taskStatus?.status === 'failed') {
    if (importState.taskStatus.status === 'success') {
      handleCancelImport()
    } else {
      // Retry on failure
      importState.taskId = null
      importState.taskStatus = null
      importState.importing = false
    }
    return
  }

  importState.importing = true
  try {
    const res: any = await upsertFAQEntries(props.kbId, {
      entries: importState.preview,
      mode: importState.mode,
    })

    const taskId = res?.data?.task_id
    if (taskId) {
      importState.taskId = taskId
      importState.taskStatus = {
        status: 'pending',
        progress: 0,
        total: importState.preview.length,
        processed: 0,
        message: t('faqManager.import.progressHint'),
      }
      // Start polling the task status
      startPolling(taskId)
      // Close the import dialog right away; progress shows at the top of the list page
      importVisible.value = false
      // Reset the import dialog state (but keep taskId and taskStatus for the progress display)
      importState.file = null
      importState.preview = []
      importState.importing = false
    } else {
      // No task ID returned: possibly an old API version, so use the synchronous path
      MessagePlugin.success(t('knowledgeEditor.faqImport.importSuccess'))
      importVisible.value = false
      await loadEntries()
      importState.importing = false
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
    importState.importing = false
    stopPolling()
  }
}

// Clean up polling when the component unmounts
onUnmounted(() => {
  stopPolling()
})

// Download example file options
const downloadExampleOptions = computed(() => [
  { content: t('knowledgeEditor.faqImport.downloadExampleJSON'), value: 'json' },
  { content: t('knowledgeEditor.faqImport.downloadExampleCSV'), value: 'csv' },
  { content: t('knowledgeEditor.faqImport.downloadExampleExcel'), value: 'excel' },
])

// Example data
const exampleData: FAQEntryPayload[] = [
  {
    standard_question: 'What is AI KB?',
    answers: ['AI KB is an intelligent knowledge base management system', 'It supports several knowledge base types and import methods'],
    similar_questions: ['What is AI KB about?', 'Tell me about AI KB'],
    negative_questions: ['This is not AI KB', 'Unrelated to AI KB'],
    tag_name: 'Product overview',
  },
  {
    standard_question: 'How do I create a knowledge base?',
    answers: ['Click the "New knowledge base" button', 'Choose a knowledge base type and fill in the details', 'Once it is created you can start using it'],
    similar_questions: ['How can I create a knowledge base?', 'How do I add a new knowledge base?'],
    negative_questions: [],
    tag_name: 'User guide',
  },
]

// Download an example file
const handleDownloadExample = (data: { value: string }) => {
  const { value } = data
  switch (value) {
    case 'json':
      downloadJSONExample()
      break
    case 'csv':
      downloadCSVExample()
      break
    case 'excel':
      downloadExcelExample()
      break
  }
}

// Download the JSON example
const downloadJSONExample = () => {
  const jsonStr = JSON.stringify(exampleData, null, 2)
  const blob = new Blob([jsonStr], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'faq_example.json'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

// Download the CSV example (headers match the server export in knowledge_faq.go)
const downloadCSVExample = () => {
  const headers = ['tag_name(required)', 'standard_question(required)', 'similar_questions(optional - separate multiple with ##)', 'negative_questions(optional - separate multiple with ##)', 'answers(required - separate multiple with ##)', 'answer_all(optional - default FALSE)', 'is_disabled(optional - default FALSE)', 'not_recommended(optional - default FALSE means recommendable)']
  const rows = exampleData.map((item) => {
    return [
      item.tag_name || '', // tag_name
      item.standard_question,
      item.similar_questions.join('##'),
      item.negative_questions.join('##'),
      item.answers.join('##'),
      'FALSE', // answer_all
      'FALSE', // is_disabled
      'FALSE', // not_recommended
    ]
  })
  const csvContent = [
    headers.join('\t'), // tab-separated
    ...rows.map((row) => row.map((cell) => {
      // Quote the cell if it contains a tab, newline or quote
      if (cell.includes('\t') || cell.includes('\n') || cell.includes('"')) {
        return `"${cell.replace(/"/g, '""')}"`
      }
      return cell
    }).join('\t')),
  ].join('\n')
  const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'faq_example.csv'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

// Download the Excel example
const downloadExcelExample = () => {
  const worksheet = XLSX.utils.json_to_sheet(
    exampleData.map((item) => ({
      'tag_name(required)': item.tag_name || '',
      'standard_question(required)': item.standard_question,
      'similar_questions(optional - separate multiple with ##)': item.similar_questions.join('##'),
      'negative_questions(optional - separate multiple with ##)': item.negative_questions.join('##'),
      'answers(required - separate multiple with ##)': item.answers.join('##'),
      'answer_all(optional - default FALSE)': 'FALSE',
      'is_disabled(optional - default FALSE)': 'FALSE',
      'not_recommended(optional - default FALSE means recommendable)': 'FALSE',
    })),
  )
  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, worksheet, 'FAQ')
  XLSX.writeFile(workbook, 'faq_example.xlsx')
}

// Export FAQ data
const exportLoading = ref(false)
const downloadExportBlob = (blob: Blob, ext: 'csv' | 'json') => {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `faq_export_${new Date().toISOString().slice(0, 10)}.${ext}`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
const handleExportFAQ = async (format: 'csv' | 'json') => {
  if (!props.kbId) {
    MessagePlugin.warning(t('knowledgeBase.selectKnowledgeBase'))
    return
  }

  exportLoading.value = true
  try {
    const blob = await exportFAQEntries(props.kbId, format)
    downloadExportBlob(blob, format)
    MessagePlugin.success(t('knowledgeEditor.faqExport.exportSuccess'))
  } catch (error: any) {
    console.error('Export failed:', error)
    MessagePlugin.error(t('knowledgeEditor.faqExport.exportFailed'))
  } finally {
    exportLoading.value = false
  }
}
const handleExportCSV = () => handleExportFAQ('csv')
const handleExportJSON = () => handleExportFAQ('json')

watch(
  () => props.kbId,
  async (newKbId) => {
    currentPage = 1
    hasMore.value = true
    selectedTagIds.value = []
    tagFilterCleared.value = false
    uiStore.clearSelectedTagIds()
    overallFAQTotal.value = 0
    tagSearchQuery.value = ''

    if (!newKbId) {
      kbInfo.value = null
      // Clear the previous task state when kbId changes
      stopPolling()
      importState.taskId = null
      importState.taskStatus = null
      clearTaskIdFromStorage()
      return
    }

    const info = await loadKnowledgeInfo(newKbId)
    if (!info || info.type !== 'faq') {
      return
    }

    loadEntries()
    loadTags(true)
    // Restore the import task state (if any)
    await restoreImportTask()
  },
  { immediate: true },
)

watch(selectedTagIds, (newVal, oldVal) => {
  if (oldVal === undefined) return
  if (newVal.join(',') !== oldVal.join(',')) {
    currentPage = 1
    entries.value = []
    selectedRowKeys.value = []
    loadEntries()
  }
})

watch(tagSearchQuery, (newVal, oldVal) => {
  if (newVal === oldVal) return
  if (tagSearchDebounce) {
    window.clearTimeout(tagSearchDebounce)
  }
  tagSearchDebounce = window.setTimeout(() => {
    loadTags(true)
  }, 300)
})

// Watch the FAQ search keyword
watch(entrySearchKeyword, (newVal, oldVal) => {
  if (newVal === oldVal) return
  if (entrySearchDebounce) {
    window.clearTimeout(entrySearchDebounce)
  }
  entrySearchDebounce = window.setTimeout(() => {
    loadEntries()
  }, 300)
})

const handleSearch = async () => {
  if (!searchForm.query.trim()) {
    MessagePlugin.warning(t('knowledgeEditor.faq.queryPlaceholder'))
    return
  }

  searching.value = true
  hasSearched.value = true
  try {
    const res = await searchFAQEntries(props.kbId, {
      query_text: searchForm.query.trim(),
      vector_threshold: searchForm.vectorThreshold,
      match_count: searchForm.matchCount,
    })
    const results = (res.data || []).map((entry: FAQEntry) => ({
      ...entry,
      similarCollapsed: true,  // similar questions collapsed by default
      negativeCollapsed: true,  // negative examples collapsed by default
      answersCollapsed: true,   // answers collapsed by default
      expanded: false,
    })) as FAQEntry[]

    // Sort by score, highest first
    searchResults.value = results.sort((a, b) => (b.score || 0) - (a.score || 0))
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('common.operationFailed'))
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

const toggleResult = (result: FAQEntry) => {
  result.expanded = !result.expanded
}

// Debounce function
let arrangeCardsTimer: ReturnType<typeof setTimeout> | null = null
const debounceArrangeCards = (delay = 100) => {
  if (arrangeCardsTimer) {
    clearTimeout(arrangeCardsTimer)
  }
  arrangeCardsTimer = setTimeout(() => {
    arrangeCards()
    arrangeCardsTimer = null
  }, delay)
}

// Masonry layout function, optimized to avoid flicker
const arrangeCards = () => {
  if (!cardListRef.value) return

  const cards = cardListRef.value.querySelectorAll('.faq-card') as NodeListOf<HTMLElement>
  if (cards.length === 0) return

  // Get the container width and column count
  const containerWidth = cardListRef.value.offsetWidth
  const gap = 12 // matches the CSS gap
  let columnCount = 1

  // Compute the column count from the container width (more cards per row)
  if (containerWidth >= 2560) columnCount = 12
  else if (containerWidth >= 1920) columnCount = 10
  else if (containerWidth >= 1536) columnCount = 8
  else if (containerWidth >= 1280) columnCount = 6
  else if (containerWidth >= 1024) columnCount = 5
  else if (containerWidth >= 768) columnCount = 4
  else if (containerWidth >= 640) columnCount = 3

  const columnWidth = (containerWidth - (gap * (columnCount - 1))) / columnCount

  // Initialize the height array for each column
  const columnHeights = new Array(columnCount).fill(0)

  // Use requestAnimationFrame for performance
  requestAnimationFrame(() => {
    // Set the width first, keeping the current position
    cards.forEach((card) => {
      // Make sure the card is absolutely positioned
      if (card.style.position !== 'absolute') {
        card.style.position = 'absolute'
      }
      // Set the width so the height computes correctly
      card.style.width = `${columnWidth}px`
    })

    // Wait for the browser to recompute layout
    requestAnimationFrame(() => {
      // Compute every card's height (without moving it)
      const cardHeights: number[] = []
      cards.forEach((card) => {
        const height = card.offsetHeight || card.getBoundingClientRect().height
        cardHeights.push(height)
      })

      // Compute the new positions
      const newPositions: Array<{ top: number; left: number }> = []
      cardHeights.forEach((height) => {
        const shortestColumnIndex = columnHeights.indexOf(Math.min(...columnHeights))
        const top = columnHeights[shortestColumnIndex]
        const left = shortestColumnIndex * (columnWidth + gap)

        newPositions.push({ top, left })
        columnHeights[shortestColumnIndex] += height + gap
      })

      // Update every card position in one batch, with a CSS transition for smooth movement
      cards.forEach((card, index) => {
        const { top, left } = newPositions[index]
        const currentTop = parseFloat(card.style.top) || 0
        const currentLeft = parseFloat(card.style.left) || 0

        // If the position changed, add the transition
        if (Math.abs(currentTop - top) > 1 || Math.abs(currentLeft - left) > 1) {
          // Hint the browser with will-change
          card.style.willChange = 'top, left'
          card.style.transition = 'top 0.3s cubic-bezier(0.4, 0, 0.2, 1), left 0.3s cubic-bezier(0.4, 0, 0.2, 1)'
        }

        card.style.position = 'absolute'
        card.style.top = `${top}px`
        card.style.left = `${left}px`
        card.style.width = `${columnWidth}px`
      })

      // Set the container height
      const maxHeight = Math.max(...columnHeights)
      if (cardListRef.value) {
        cardListRef.value.style.height = `${maxHeight}px`
        cardListRef.value.style.position = 'relative'
      }

      // After the animation, remove the transition and will-change so later interaction is unaffected
      setTimeout(() => {
        cards.forEach((card) => {
          card.style.transition = ''
          card.style.willChange = ''
        })
      }, 300)
    })
  })
}

// Watch window resizes (debounced)
let resizeTimer: ReturnType<typeof setTimeout> | null = null
const handleResize = () => {
  if (resizeTimer) {
    clearTimeout(resizeTimer)
  }
  resizeTimer = setTimeout(() => {
    arrangeCards()
    // A larger window may need more data; delayed so layout finishes first
    setTimeout(() => {
      checkAndLoadMore()
    }, 350)
    resizeTimer = null
  }, 150)
}

onMounted(async () => {
  // Ensure shared knowledge bases are loaded before loading the knowledge list
  orgStore.fetchSharedKnowledgeBases()
  loadKnowledgeList()
  window.addEventListener('resize', handleResize)
  // If there is already a kbId, restore the import task state
  if (props.kbId) {
    await restoreImportTask()
    await loadImportResult() // load the import result
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (arrangeCardsTimer) {
    clearTimeout(arrangeCardsTimer)
  }
  if (resizeTimer) {
    clearTimeout(resizeTimer)
  }
})

// Watch entries and re-layout
watch(() => entries.value.length, () => {
  nextTick(() => {
    arrangeCards()
  })
})

// Watch the collapse state and re-layout (debounced, with a callback after the animation)
watch(() => entries.value.map(e => ({
  id: e.id,
  similarCollapsed: e.similarCollapsed,
  negativeCollapsed: e.negativeCollapsed,
  answersCollapsed: e.answersCollapsed
})), () => {
  // nextTick makes sure the DOM has updated
  nextTick(() => {
    // Wait one render frame for the height change to take effect
    requestAnimationFrame(() => {
      // Wait one more render frame so the height measures accurately
      requestAnimationFrame(() => {
        // Lay out after the Transition animation finishes (slide-down takes about 200ms)
        // Debounced to avoid frequent calls
        debounceArrangeCards(250)
      })
    })
  })
}, { deep: true })
</script>

<style scoped lang="less">
.faq-manager {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--app-motion-fast) ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.faq-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  gap: 20px;
}

// Same as the list page: a light gray rounded area, with the filters on the left as a white card
.faq-main {
  display: flex;
  flex: 1;
  min-height: 0;
  background: transparent;
  border: none;
}

.faq-card-area {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0;
  border: none;
  overflow: hidden;
  background: transparent;
}

.faq-filter-bar {
  padding: 0 0 12px 0;
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;

  .faq-search-input {
    flex: 1 1 220px;
    min-width: 0;
    width: auto;
  }

  &__filters {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;

    :deep(.t-popup__reference) {
      display: block;
    }
  }

  &__trailing {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;

    :deep(.content-bar-icon-btn) {
      color: var(--td-text-color-secondary);
      background: transparent;
      border: none;

      &:hover {
        color: var(--td-brand-color);
        background: var(--td-bg-color-secondarycontainer);
      }
    }
  }

  @media (max-width: 767px) {
    .faq-search-input {
      flex: 1 1 100%;
    }

    &__filters {
      flex: 1 1 auto;
      min-width: 0;
    }

    &__trailing {
      flex: 0 0 auto;
      margin-left: auto;
    }
  }

  :deep(.t-input) {
    font-size: var(--app-text-md);
    background-color: var(--td-bg-color-secondarycontainer);
    border-color: transparent;
    border-radius: var(--app-radius-sm);
    box-shadow: none !important;

    &:hover,
    &:focus,
    &.t-is-focused {
      border-color: var(--td-brand-color);
      background-color: var(--td-bg-color-container);
      box-shadow: none !important;
    }
  }

  :deep(.t-input__prefix-icon) {
    margin-right: 0;
  }
}





.faq-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  flex-shrink: 0;

  .faq-header-title {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .faq-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    width: 100%;

    .faq-import-strip--in-title {
      margin-bottom: 0;
      flex: 0 1 auto;
      min-width: 0;
      max-width: min(420px, 40vw);

      .faq-import-strip__text {
        max-width: 220px;
      }
    }
  }

  .kb-title-actions {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .faq-breadcrumb {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0;
    font-size: var(--app-text-3xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .breadcrumb-link {
    border: none;
    background: transparent;
    padding: 4px 8px;
    margin: -4px -8px;
    font: inherit;
    color: var(--td-text-color-secondary);
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    border-radius: var(--app-radius-sm);
    transition: all var(--app-motion-instant) ease;

    &:hover:not(:disabled) {
      color: var(--td-success-color);
      background: var(--td-bg-color-container);
    }

    &:disabled {
      cursor: not-allowed;
      color: var(--td-text-color-placeholder);
    }

    &.dropdown {
      padding-right: 6px;

      :deep(.t-icon) {
        font-size: var(--app-text-base);
        transition: transform var(--app-motion-instant) ease;
      }

      &:hover:not(:disabled) {
        :deep(.t-icon) {
          transform: translateY(1px);
        }
      }
    }
  }

  .breadcrumb-separator {
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
  }

  .breadcrumb-current {
    color: var(--td-text-color-primary);
    font-weight: 600;
  }

  h2 {
    margin: 0;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: var(--app-text-4xl);
    font-weight: 600;
    line-height: 32px;
  }

  .faq-subtitle {
    margin: 0;
    color: var(--td-text-color-placeholder);
    font-family: var(--app-font-family);
    font-size: var(--app-text-base);
    font-weight: 400;
    line-height: 20px;
  }
}

// Import result entry: icon only by default, the overlay expands on hover / click
.faq-import-host {
  position: relative;
  flex-shrink: 0;

  .faq-import-trigger {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: transparent;
    padding: 2px;
    margin: 0;
    color: var(--td-success-color);
    cursor: pointer;
    line-height: 1;
    transition: opacity var(--app-motion-fast) ease;

    &:hover {
      opacity: 0.75;
    }
  }

  .faq-import-panel {
    position: absolute;
    top: calc(100% + 8px);
    left: 0;
    z-index: 200;
    opacity: 0;
    visibility: hidden;
    pointer-events: none;
    transform: translateY(-4px);
    transition: opacity var(--app-motion-fast) ease, transform var(--app-motion-fast) ease, visibility var(--app-motion-fast) ease;
  }

  &:hover .faq-import-panel,
  &.is-expanded .faq-import-panel,
  &:focus-within .faq-import-panel {
    opacity: 1;
    visibility: visible;
    pointer-events: auto;
    transform: translateY(0);
  }

  .faq-import-strip--panel {
    margin-bottom: 0;
    padding: 8px 10px;
    font-size: var(--app-text-sm);
    white-space: nowrap;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);

    .faq-import-strip__text {
      max-width: 360px;
    }
  }
}

// FAQ import hint bar (compact single line)
.faq-import-strip {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  gap: 8px;
  max-width: 100%;
  margin-bottom: 10px;
  padding: 4px 8px 4px 10px;
  border-radius: var(--app-radius-sm);
  font-size: var(--app-text-sm);
  line-height: 1.4;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);

  &__icon {
    flex-shrink: 0;
    color: var(--td-text-color-placeholder);

    &.is-spinning {
      animation: wk-spin 1s linear infinite;
    }
  }

  &__text {
    flex: 0 1 auto;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 520px;
  }

  &__bar {
    flex-shrink: 0;
    width: 72px;
    height: 4px;
    border-radius: 2px;
    background: rgba(0, 0, 0, 0.08);
    overflow: hidden;
  }

  &__bar-fill {
    height: 100%;
    border-radius: 2px;
    background: var(--td-brand-color);
    transition: width var(--app-motion-slow) ease;
  }

  &__count {
    flex-shrink: 0;
    font-size: var(--app-text-sm);
    font-variant-numeric: tabular-nums;
    color: var(--td-text-color-placeholder);
  }

  &__time {
    flex-shrink: 0;
    font-size: var(--app-text-sm);
    color: var(--td-text-color-placeholder);
    white-space: nowrap;
  }

  &__link {
    flex-shrink: 0;
    padding: 0 4px;
    height: auto;
    font-size: var(--app-text-sm);
  }

  &__close {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    margin: 0;
    padding: 0;
    border: none;
    border-radius: var(--app-radius-xs);
    background: transparent;
    color: var(--td-text-color-placeholder);
    cursor: pointer;
    transition: background var(--app-motion-fast) ease, color var(--app-motion-fast) ease;

    &:hover {
      background: rgba(0, 0, 0, 0.06);
      color: var(--td-text-color-secondary);
    }
  }

  &--result {
    .faq-import-strip__icon {
      color: var(--td-success-color);
    }
  }

  &--running {
    .faq-import-strip__icon {
      color: var(--td-brand-color);
    }
  }

  &--success {
    .faq-import-strip__icon {
      color: var(--td-success-color);
    }

    .faq-import-strip__bar-fill {
      background: var(--td-success-color);
    }
  }

  &--failed {
    border-color: rgba(227, 77, 89, 0.3);
    background: rgba(227, 77, 89, 0.06);

    .faq-import-strip__icon {
      color: var(--td-error-color);
    }

    .faq-import-strip__text {
      color: var(--td-error-color);
    }

    .faq-import-strip__bar-fill {
      background: var(--td-error-color);
    }
  }
}

.tag-filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;

  .tag-filter-label {
    color: var(--td-text-color-secondary);
    font-size: var(--app-text-base);
  }
}

.kb-settings-button {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 50%;
  background: var(--td-bg-color-secondarycontainer);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  transition: all var(--app-motion-base) ease;
  padding: 0;

  &:hover:not(:disabled) {
    background: var(--td-success-color-light);
    color: var(--td-brand-color);
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.4;
  }

  :deep(.t-icon) {
    font-size: var(--app-text-2xl);
  }
}

// Scroll container
.faq-scroll-container {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;

  &.has-batch-bar {
    padding-bottom: 76px;
  }
}

.faq-batch-bar-anchor {
  position: absolute;
  right: 0;
  bottom: 12px;
  left: 0;
  z-index: 6;
  display: flex;
  justify-content: center;
  padding: 0 16px;
  pointer-events: none;

  &>* {
    pointer-events: auto;
  }
}

@keyframes contentFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.faq-skeleton-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  width: 100%;
  animation: contentFadeIn 0.32s ease-out;
}

.faq-card-skeleton {
  cursor: default;
  height: auto;

  .faq-card-header {
    padding-bottom: 10px;
    border-bottom: 1px solid var(--td-component-stroke);
  }

  .faq-card-body {
    padding: 8px 0;
  }

  .faq-skel-footer {
    padding-top: 8px;
    border-top: 1px solid var(--td-component-stroke);
  }
}

// Card list styles: absolute positioning for a masonry layout, where the next row fills gaps in the previous one
.faq-card-list {
  position: relative;
  width: 100%;
  animation: contentFadeIn 0.32s ease-out;
  min-width: 0;
}

.faq-card {
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-container);
  padding: 10px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  cursor: default;
  transition: border-color var(--app-motion-base) ease, box-shadow var(--app-motion-base) ease, background-color var(--app-motion-base) ease;
  box-sizing: border-box;
  height: fit-content;

  &.is-selectable {
    cursor: pointer;

    &:hover {
      border-color: var(--app-selection-border);
    }
  }

  &.selected {
    border-color: var(--app-selection-border);
    background: var(--td-bg-color-container);
    box-shadow: none;
  }
}

.faq-card-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--td-component-stroke);
  position: relative;
}

.faq-header-top {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.faq-card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  flex-shrink: 0;
}

.faq-header-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding-top: 5px;
  border-top: 1px dashed var(--td-component-stroke);
}

.faq-meta-item {
  display: inline-flex;
  align-items: baseline;
  gap: 5px;
  padding: 3px 8px;
  border-radius: var(--app-radius-pill);
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);

  .meta-label {
    font-size: var(--app-text-xs);
    color: var(--td-text-color-secondary);
    font-weight: 500;
  }

  .meta-value {
    font-size: var(--app-text-sm);
    color: var(--td-text-color-primary);
    font-weight: 600;
  }
}

.faq-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 8px 12px;
  margin: 0 -10px -10px;
  background: rgba(48, 50, 54, 0.02);
  border-top: 1px solid var(--td-component-stroke);
  flex-wrap: nowrap;
}

.faq-card-status {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
  margin-left: auto;
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: var(--app-radius-pill);
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  font-size: var(--app-text-xs);
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);

  .status-icon {
    font-size: var(--app-text-md);
    color: var(--td-text-color-placeholder);

    &.warning {
      color: var(--td-warning-color);
    }

    &.success {
      color: var(--td-success-color);
    }
  }
}

.status-item-compact {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 4px;
  border-radius: var(--app-radius-xs);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: all var(--app-motion-base) ease;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  .status-icon {
    font-size: var(--app-text-xl);
    flex-shrink: 0;

    &.warning {
      color: var(--td-warning-color);
    }

    &.success {
      color: var(--td-success-color);
    }
  }

  :deep(.t-switch) {
    flex-shrink: 0;
  }
}

.faq-card-tag {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex: 1;
  min-width: 0;

  :deep(.t-tag) {
    display: inline-flex;
    align-items: center;
    cursor: pointer;
    max-width: 120px;
    height: 20px;
    border-radius: var(--app-radius-xs);
    border-color: var(--td-component-stroke);
    color: var(--td-text-color-disabled);
    padding: 0 6px;
    background: var(--td-bg-color-container-hover);
    font-size: var(--app-text-xs);
    font-weight: 400;
    font-family: var(--app-font-family);
    transition: all var(--app-motion-base) ease;

    &:hover {
      border-color: var(--td-brand-color);
      color: var(--td-brand-color-active);
      background: var(--td-success-color-light);
    }
  }
}

.faq-tag-chip {
  display: inline-flex;
  align-items: center;
  cursor: pointer;

  .tag-text {
    max-width: 100px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--app-text-xs);
    font-weight: 400;
    color: var(--td-text-color-disabled);
  }
}

.card-more-btn {
  display: flex;
  width: 28px;
  height: 28px;
  justify-content: center;
  align-items: center;
  border-radius: var(--app-radius-sm);
  cursor: pointer;
  flex-shrink: 0;
  opacity: 0.6;

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
    opacity: 1;
  }

  &.mobile {
    display: none;
  }

  .more-icon {
    width: 16px;
    height: 16px;
  }
}

/* card-menu styles are unified in @/assets/dropdown-menu.less, using the .popup-menu class */

.faq-question {
  flex: 1;
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family);
  font-size: var(--app-text-lg);
  font-weight: 600;
  line-height: 1.5;
  word-break: break-word;
  min-width: 0;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
}

.faq-card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  contain: layout;
}

.faq-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  overflow: hidden;

  .faq-section-label {
    color: var(--td-text-color-secondary);
    font-family: var(--app-font-family);
    font-size: var(--app-text-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    display: flex;
    align-items: center;
    gap: 5px;
    margin-bottom: 1px;

    &::before {
      content: '';
      width: 3px;
      height: 10px;
      background: var(--td-brand-color);
      border-radius: 2px;
      flex-shrink: 0;
    }

    &.clickable {
      cursor: pointer;
      user-select: none;
      padding: 2px 0;
      border-radius: var(--app-radius-xs);

      &:hover {
        color: var(--td-text-color-primary);
        background: var(--td-bg-color-container);
        padding-left: 4px;
        padding-right: 4px;
        margin-left: -4px;
        margin-right: -4px;
      }
    }

    .collapse-icon {
      font-size: var(--app-text-md);
      color: var(--td-text-color-placeholder);
      flex-shrink: 0;
      margin-left: auto; // align the arrow right
    }

    .section-count {
      color: var(--td-text-color-placeholder);
      font-weight: 400;
      margin-left: 4px;
    }
  }

  &.answers .faq-section-label::before {
    background: var(--td-brand-color);
  }

  &.similar .faq-section-label::before {
    background: var(--td-brand-color);
  }

  &.negative .faq-section-label::before {
    background: var(--td-warning-color);
  }
}

.faq-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  min-height: 18px;
  min-width: 0;
  width: 100%;
  overflow: hidden;
  contain: layout style paint; // rendering performance

  // Make sure every tag has a max width
  >* {
    max-width: 100%;
    min-width: 0;
    flex: 0 1 auto;
  }

  // Limit the max width when a tag is alone on a line
  >*:first-child:last-child {
    max-width: 100%;
  }
}

.question-tag {
  font-size: var(--app-text-xs);
  padding: 3px 8px;
  max-width: 100%;
  min-width: 0;
  border-radius: 5px;
  font-family: var(--app-font-family);
  flex: 0 1 auto;

  :deep(.t-tag) {
    max-width: 100% !important;
    min-width: 0 !important;
    width: auto !important;
    display: inline-flex !important;
    align-items: center;
    vertical-align: middle;
    overflow: hidden !important;
    box-sizing: border-box;
    background: var(--td-bg-color-container);
    border-color: var(--td-component-stroke);
    color: var(--td-text-color-primary);
  }

  // For the span inside a TDesign tag
  :deep(.t-tag span),
  :deep(.t-tag > span) {
    display: block !important;
    overflow: hidden !important;
    text-overflow: ellipsis !important;
    white-space: nowrap !important;
    max-width: 100% !important;
    width: auto !important;
    line-height: 1.4;
    min-width: 0 !important;
  }
}

// Make sure the tag itself does not overflow the container
.faq-tags :deep(.t-tag) {
  max-width: 100%;
  min-width: 0;
  flex-shrink: 1;
}

.faq-tags :deep(.faq-tag-wrapper) {
  max-width: 100%;
  min-width: 0;
  flex-shrink: 1;
}

.empty-tip {
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-sm);
  font-style: italic;
  padding: 8px 0;
  font-family: var(--app-font-family);
}

.faq-load-more,
.faq-no-more {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 24px 16px;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-md);
  font-family: var(--app-font-family);
}

.faq-no-more {
  color: var(--td-text-color-placeholder);
  font-style: italic;
}

// Empty state styles
.faq-empty-state {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
  padding: 60px 20px;

  .empty-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    text-align: center;
    max-width: 400px;
  }

  .empty-icon {
    color: var(--td-text-color-disabled);
    opacity: 0.6;
  }

  .empty-text {
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: var(--app-text-2xl);
    font-weight: 600;
    line-height: 28px;
  }

  .empty-desc {
    color: var(--td-text-color-secondary);
    font-family: var(--app-font-family);
    font-size: var(--app-text-base);
    font-weight: 400;
    line-height: 22px;
  }
}

// Import dialog styles, matching the create knowledge base dialog
.faq-import-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  backdrop-filter: blur(4px);
}

.faq-import-modal {
  position: relative;
  width: 100%;
  max-width: 600px;
  max-height: 90vh;
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-xl);
  box-shadow: 0 6px 28px rgba(15, 23, 42, 0.08);
  overflow: hidden;
  display: flex;
  flex-direction: column;

  .close-btn {
    position: absolute;
    top: 20px;
    right: 20px;
    width: 32px;
    height: 32px;
    border: none;
    background: var(--td-bg-color-secondarycontainer);
    border-radius: var(--app-radius-sm);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--td-text-color-secondary);
    transition: all var(--app-motion-base) ease;
    z-index: 10;

    &:hover {
      background: var(--td-bg-color-secondarycontainer);
      color: var(--td-text-color-primary);
    }
  }
}

.faq-import-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.faq-import-header {
  padding: 24px 24px 16px;
  border-bottom: 1px solid var(--td-component-stroke);
  flex-shrink: 0;

  .import-title {
    margin: 0;
    font-family: var(--app-font-family);
    font-size: var(--app-text-2xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.faq-import-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 24px;
  min-height: 0;
  max-height: calc(90vh - 140px); // minus the header and footer height

  // Custom scrollbar
  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: var(--td-bg-color-secondarycontainer);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: var(--td-bg-color-component-disabled);
    border-radius: 3px;
    transition: background var(--app-motion-base);

    &:hover {
      background: var(--td-brand-color);
    }
  }
}

.faq-import-footer {
  padding: 16px 24px;
  border-top: 1px solid var(--td-component-stroke);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  flex-shrink: 0;
}

// Import form item
.import-form-item {
  margin-bottom: 24px;

  &:last-child {
    margin-bottom: 0;
  }
}

// File label row
.file-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  gap: 12px;
}

// Download example button
.download-example-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--app-font-family);
  font-size: var(--app-text-md);
  font-weight: 500;
  padding: 6px 14px;
  border-radius: var(--app-radius-sm);
  border: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  transition: all var(--app-motion-base) ease;
  cursor: pointer;
  white-space: nowrap;

  &:hover {
    border-color: var(--td-brand-color);
    color: var(--td-brand-color);
    background: var(--td-success-color-light);
  }

  &:active {
    background: var(--td-success-color-light);
  }

  :deep(.t-icon) {
    font-size: var(--app-text-xl);
  }
}

// Import form label
.import-form-label {
  display: block;
  margin-bottom: 0;
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-primary);
  letter-spacing: -0.2px;
  flex: 1;

  &.required::after {
    content: '*';
    color: var(--td-error-color);
    margin-left: 4px;
    font-weight: 600;
  }
}

// File upload wrapper
.file-upload-wrapper {
  width: 100%;
}

// Hidden file input
.file-input-hidden {
  position: absolute;
  width: 0;
  height: 0;
  opacity: 0;
  overflow: hidden;
  pointer-events: none;
}

// File upload area
.file-upload-area {
  position: relative;
  width: 100%;
  min-height: 120px;
  border: 2px dashed var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-secondarycontainer);
  cursor: pointer;
  transition: all var(--app-motion-slow) ease;
  display: flex;
  align-items: center;
  justify-content: center;

  &:hover {
    border-color: var(--td-brand-color);
    background: var(--td-success-color-light);
  }

  &.has-file {
    border-color: var(--td-brand-color);
    background: var(--td-success-color-light);
    border-style: solid;
  }
}

// File upload content
.file-upload-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  text-align: center;
}

.upload-icon {
  color: var(--td-brand-color);
  transition: transform var(--app-motion-base) ease;
}

.file-upload-area:hover .upload-icon {
  transform: translateY(-2px);
}

.upload-text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.upload-primary-text {
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.upload-secondary-text {
  font-family: var(--app-font-family);
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
}

.upload-file-name {
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-brand-color);
  word-break: break-all;
}

// Import form hint
.import-form-tip {
  margin-top: 8px;
  font-family: var(--app-font-family);
  font-size: var(--app-text-sm);
  color: var(--td-text-color-disabled);
  line-height: 18px;
}

// Preview area
.import-preview {
  margin-top: 20px;
  padding: 16px;
  background: var(--td-bg-color-secondarycontainer);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
}

.preview-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.preview-icon {
  color: var(--td-brand-color);
  flex-shrink: 0;
}

.preview-title {
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.preview-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}

.preview-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 10px 12px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);
  transition: all var(--app-motion-base) ease;

  &:hover {
    border-color: var(--td-brand-color);
    box-shadow: 0 2px 4px color-mix(in srgb, var(--td-brand-color) 8%, transparent);
  }
}

.preview-index {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--td-brand-color) 0%, var(--td-brand-color-active) 100%);
  color: var(--td-text-color-anti);
  border-radius: var(--app-radius-xs);
  font-family: var(--app-font-family);
  font-size: var(--app-text-sm);
  font-weight: 600;
}

.preview-question {
  flex: 1;
  font-family: var(--app-font-family);
  font-size: var(--app-text-md);
  color: var(--td-text-color-primary);
  line-height: 1.5;
  word-break: break-word;
}

.preview-more {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--td-component-stroke);
  font-family: var(--app-font-family);
  font-size: var(--app-text-sm);
  color: var(--td-text-color-secondary);
  text-align: center;
}

// The responsive layout is computed in JavaScript, so no media queries are needed here

// Card menu popup styles are unified in @/assets/dropdown-menu.less

// FAQ editor drawer styles
:deep(.faq-editor-drawer) {
  .t-drawer__body {
    padding: 20px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .t-drawer__header {
    padding: 20px 24px;
    border-bottom: 1px solid var(--td-component-stroke);
    font-family: var(--app-font-family);
    font-size: var(--app-text-2xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .t-drawer__footer {
    padding: 16px 24px;
    border-top: 1px solid var(--td-component-stroke);
  }
}

.faq-editor-drawer-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;

  // Custom scrollbar
  &::-webkit-scrollbar {
    width: 6px;
  }

  &::-webkit-scrollbar-track {
    background: var(--td-bg-color-secondarycontainer);
    border-radius: 3px;
  }

  &::-webkit-scrollbar-thumb {
    background: var(--td-bg-color-component-disabled);
    border-radius: 3px;
    transition: background var(--app-motion-base);

    &:hover {
      background: var(--td-brand-color);
    }
  }

  .editor-form {
    width: 100%;
  }
}

.faq-editor-drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

// Full-width input wrapper, unified styles
.full-width-input-wrapper {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;

  .full-width-input {
    flex: 1;
    min-width: 0;
  }

  .full-width-textarea {
    flex: 1;
    min-width: 0;

    :deep(.t-textarea__inner) {
      min-height: 80px;
    }
  }

  // a textarea needs top alignment
  &.textarea-wrapper {
    align-items: flex-start;
  }

  .add-item-btn {
    flex-shrink: 0;
    width: 32px;
    height: 32px;
    min-width: 32px;
    padding: 0;
    font-family: var(--app-font-family);
    transition: all var(--app-motion-base) ease;
    border-radius: var(--app-radius-md);
  }

  :deep(.add-item-btn) {
    background: var(--td-brand-color) !important;
    border: 1px solid var(--td-brand-color) !important;
    border-radius: var(--app-radius-md) !important;
    color: var(--td-text-color-anti) !important;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover:not(:disabled) {
      background: var(--td-brand-color) !important;
      border-color: var(--td-brand-color-active) !important;
      transform: scale(1.05);
      box-shadow: 0 2px 8px color-mix(in srgb, var(--td-brand-color) 30%, transparent);
    }

    &:active:not(:disabled) {
      background: var(--td-brand-color-active) !important;
      border-color: var(--td-brand-color-active) !important;
      transform: scale(0.98);
    }

    &:disabled {
      background: var(--td-bg-color-component-disabled) !important;
      border-color: var(--td-component-stroke) !important;
      color: var(--td-text-color-placeholder) !important;
      cursor: not-allowed;
      opacity: 0.6;
    }

    .t-icon {
      font-size: var(--app-text-xl);
    }
  }
}

.textarea-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.item-count {
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);
  font-weight: 500;
  text-align: right;
  padding-right: 40px;
  line-height: 1;
}

.item-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  margin-top: 8px;
}

.item-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  transition: all var(--app-motion-base) ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  position: relative;

  &.answer-row {
    align-items: flex-start;
    padding: 12px 14px;
  }

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
    border-color: var(--td-brand-color);
    box-shadow: 0 2px 8px color-mix(in srgb, var(--td-brand-color) 12%, transparent);
    transform: translateY(-1px);
  }

  &.negative {
    background: var(--td-warning-color-light);
    border-color: var(--td-warning-color-focus);

    &:hover {
      background: var(--td-warning-color-light);
      border-color: var(--td-warning-color);
      box-shadow: 0 2px 8px rgba(251, 191, 36, 0.15);
    }
  }

  .item-content {
    flex: 1;
    font-size: var(--app-text-base);
    line-height: 1.6;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    white-space: pre-wrap;
    word-break: break-word;
    padding: 0;
    font-weight: 400;
  }

  .remove-item-btn {
    flex-shrink: 0;
    color: var(--td-text-color-placeholder);
    padding: 0;
    width: 24px;
    height: 24px;
    min-width: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--app-radius-sm);
    transition: all var(--app-motion-base) ease;
    background: transparent;
    border: none;
    cursor: pointer;

    &:hover {
      color: var(--td-error-color);
      background: var(--td-error-color-light);
    }

    &:active {
      background: var(--td-error-color-light);
    }

    :deep(.t-icon) {
      font-size: var(--app-text-base);
    }
  }

  &.answer-row .remove-item-btn {
    margin-top: 0;
  }
}

.form-tip {
  margin-top: 6px;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-disabled);
  font-family: var(--app-font-family);
}

// FAQ editor form styles, modeled entirely on the settings page
.faq-editor-form {
  width: 100%;

  // Hide the Form's default structure
  :deep(.t-form__label) {
    display: none !important;
    width: 0 !important;
    padding: 0 !important;
    margin: 0 !important;
  }

  :deep(.t-form__controls) {
    margin-left: 0 !important;
    width: 100% !important;
  }

  :deep(.t-form__controls-content) {
    margin: 0 !important;
    padding: 0 !important;
    width: 100% !important;
    display: block !important;
  }

  :deep(.t-form-item) {
    margin-bottom: 0 !important;
    padding: 0 !important;
  }
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 20px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child {
    border-bottom: none;
  }

  &.vertical {
    flex-direction: column;
    gap: 12px;

    .setting-control {
      width: 100%;
      max-width: 100%;
    }
  }

  // Emphasis for the main fields (standard question, answers)
  &.setting-row-primary {
    padding: 20px 0;
    padding-left: 12px;
    position: relative;

    // The first one (standard question) drops the top spacing
    &:first-child {
      padding-top: 0;
    }

    // Left color marker (green for both standard question and answers)
    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 20px;
      width: 3px;
      height: calc(100% - 40px);
      background: var(--td-brand-color);
      border-radius: 0 2px 2px 0;
    }

    &:first-child::before {
      top: 0;
      height: calc(100% - 20px);
    }
  }

  // Secondary style for the optional fields (similar questions, negative examples)
  &.setting-row-optional {
    padding-left: 12px;
    position: relative;

    // Left color marker
    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 20px;
      width: 3px;
      height: calc(100% - 40px);
      border-radius: 0 2px 2px 0;
    }

    .setting-info {
      .optional-label {
        color: var(--td-text-color-primary);
        font-weight: 500;
      }

      .optional-desc {
        color: var(--td-text-color-secondary);
      }
    }
  }

  // Blue marker for similar questions
  &.setting-row-similar::before {
    background: var(--td-brand-color);
  }

  // Orange marker for negative examples
  &.setting-row-negative::before {
    background: var(--td-warning-color);
  }

  // Answers drop the bottom border
  &.setting-row-answer {
    border-bottom: none;
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

  .required-label {
    font-size: var(--app-text-lg);
    font-weight: 600;
    color: var(--td-text-color-primary);
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-bottom: 4px;
  }

  .required-mark {
    color: var(--td-error-color);
    font-weight: 600;
    font-size: var(--app-text-base);
  }

  .optional-label {
    font-size: var(--app-text-lg);
    font-weight: 600;
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

  .optional-desc {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);
  }
}

.setting-row.vertical .setting-info {
  max-width: 100%;
  padding-right: 0;
  width: 100%;
}

.setting-control {
  flex-shrink: 0;
  min-width: 280px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.setting-row.vertical .setting-control {
  width: 100%;
  max-width: 100%;
  min-width: unset;
  justify-content: flex-start;
  align-items: flex-start;
  flex-direction: column;
}

// Inputs in the vertical layout are always full width
.setting-row.vertical .full-width-input {
  width: 100%;

  :deep(.t-input__wrap) {
    width: 100%;
  }
}

.setting-row.vertical .full-width-textarea {
  width: 100%;

  :deep(.t-textarea) {
    width: 100%;
  }
}

// Input component styles, matching the login page
:deep(.t-input) {
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);
  transition: all var(--app-motion-base) ease;

  &:hover {
    border-color: var(--td-brand-color);
  }

  &:focus-within {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--td-brand-color) 10%, transparent);
  }

  .t-input__inner {
    border: none !important;
    box-shadow: none !important;
    outline: none !important;
    background: transparent;
    font-size: var(--app-text-base);
    font-family: var(--app-font-family);
    padding: 6px 12px;
    color: var(--td-text-color-primary);

    &:focus {
      border: none !important;
      box-shadow: none !important;
      outline: none !important;
    }

    &::placeholder {
      color: var(--td-text-color-placeholder);
    }
  }

  .t-input__wrap {
    border: none !important;
    box-shadow: none !important;
  }
}

// Textarea component styles
:deep(.t-textarea) {
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);
  transition: all var(--app-motion-base) ease;

  &:hover {
    border-color: var(--td-brand-color);
  }

  &:focus-within {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--td-brand-color) 10%, transparent);
  }

  .t-textarea__inner {
    border: none !important;
    box-shadow: none !important;
    outline: none !important;
    background: transparent;
    font-size: var(--app-text-base);
    font-family: var(--app-font-family);
    line-height: 1.6;
    resize: vertical;
    padding: 6px 12px;
    color: var(--td-text-color-primary);

    &:focus {
      border: none !important;
      box-shadow: none !important;
      outline: none !important;
    }

    &::placeholder {
      color: var(--td-text-color-placeholder);
    }
  }
}

// Import dialog animation
.modal-enter-active,
.modal-leave-active {
  transition: opacity var(--app-motion-base) ease;
}

.modal-enter-active .faq-import-modal,
.modal-leave-active .faq-import-modal,
.modal-enter-active .batch-tag-modal,
.modal-leave-active .batch-tag-modal {
  transition: transform var(--app-motion-base) ease, opacity var(--app-motion-base) ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .faq-import-modal,
.modal-leave-to .faq-import-modal,
.modal-enter-from .batch-tag-modal,
.modal-leave-to .batch-tag-modal {
  transform: scale(0.95);
  opacity: 0;
}

// Tag style tweaks
.answer-tag {
  background: var(--td-brand-color)1a;
  color: var(--td-brand-color);
  border-color: var(--td-brand-color)33;
}

.question-tag {
  background: var(--td-bg-color-container);
  border-color: var(--td-component-stroke);
  color: var(--td-text-color-placeholder);
}

// Search test drawer styles, matching the editor drawer
:deep(.faq-search-drawer) {
  .t-drawer__body {
    padding: 20px;
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .t-drawer__header {
    padding: 20px 24px;
    border-bottom: 1px solid var(--td-component-stroke);
    font-family: var(--app-font-family);
    font-size: var(--app-text-2xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.search-test-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 0;

  // Hide the scrollbar but keep scrolling
  scrollbar-width: none; // Firefox
  -ms-overflow-style: none; // IE and Edge

  &::-webkit-scrollbar {
    display: none; // Chrome, Safari, Opera
  }
}

.search-form {
  flex-shrink: 0;

  :deep(.t-form__label) {
    display: none !important;
    width: 0 !important;
    padding: 0 !important;
    margin: 0 !important;
  }

  :deep(.t-form__controls) {
    margin-left: 0 !important;
    width: 100% !important;
  }

  :deep(.t-form__controls-content) {
    margin: 0 !important;
    padding: 0 !important;
    width: 100% !important;
    display: block !important;
  }

  :deep(.t-form-item) {
    margin-bottom: 0 !important;
    padding: 0 !important;
  }
}

.slider-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 2px 0;
}

.search-form .setting-row {
  padding: 16px 0;
  border-bottom: 1px solid var(--td-component-stroke);

  &.search-first-row {
    padding-top: 0;
  }

  &:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }

  .setting-info {
    max-width: 100%;
    padding-right: 0;
    margin-bottom: 8px;

    label {
      font-size: var(--app-text-base);
      font-weight: 500;
      color: var(--td-text-color-primary);
      display: block;
      margin-bottom: 4px;
    }

    .desc {
      font-size: var(--app-text-sm);
      color: var(--td-text-color-secondary);
      margin: 0;
      line-height: 1.4;
    }
  }

  .setting-control {
    width: 100%;
    max-width: 100%;
    min-width: unset;
    justify-content: flex-start;
    align-items: flex-start;
    flex-direction: column;
  }
}

:deep(.slider-wrapper .t-slider) {
  flex: 1;
  min-width: 0;
}

.slider-value {
  flex-shrink: 0;
  min-width: 50px;
  text-align: right;
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-primary);
  padding: 4px 8px;
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-sm);
}

.search-button {
  height: 36px;
  border-radius: var(--app-radius-md);
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 500;
  transition: all var(--app-motion-base) ease;

  &:hover:not(:disabled) {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px color-mix(in srgb, var(--td-brand-color) 30%, transparent);
  }

  &:active:not(:disabled) {
    transform: translateY(0);
  }
}

.search-results {
  display: flex;
  flex-direction: column;
  padding-top: 20px;
  padding-left: 0;
  width: 100%;
  box-sizing: border-box;
}

.results-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  margin-left: 0;
  margin-right: 0;
  padding-left: 0;
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 600;
  color: var(--td-text-color-primary);
  flex-shrink: 0;
  justify-content: flex-start;

  .t-icon {
    color: var(--td-brand-color);
  }
}

.no-results {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 16px;
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  text-align: center;
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-md);
  border: 1px dashed var(--td-component-stroke);
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.result-card {
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-md);
  background: var(--td-bg-color-container);
  padding: 14px;
  transition: border-color var(--app-motion-base) ease, box-shadow var(--app-motion-base) ease;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  width: 100%;
  box-sizing: border-box;
  min-width: 0;
  overflow: visible;
  position: relative;

  &:hover {
    border-color: var(--td-brand-color);
    box-shadow: 0 2px 8px color-mix(in srgb, var(--td-brand-color) 12%, transparent);
  }
}

.result-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 0;
  border-bottom: none;
  cursor: pointer;
  user-select: none;
  padding: 4px;
  margin: -4px;
  border-radius: var(--app-radius-sm);
  position: relative;

  &:hover {
    background-color: var(--td-bg-color-container);
  }
}

.result-card.expanded .result-header {
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--td-component-stroke);
  margin-left: -4px;
  margin-right: -4px;
  padding-left: 4px;
  padding-right: 4px;
}

.result-question-wrapper {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  width: 100%;
}

.result-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.result-question {
  font-family: var(--app-font-family);
  font-size: var(--app-text-base);
  font-weight: 600;
  color: var(--td-text-color-primary);
  line-height: 1.6;
  word-break: break-word;
  display: flex;
  align-items: flex-start;
  gap: 6px;

  .result-index {
    flex-shrink: 0;
    color: var(--td-brand-color);
    font-weight: 600;
  }
}

.matched-question {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  padding-left: 20px;
  font-size: var(--app-text-sm);
  line-height: 1.5;

  .matched-label {
    flex-shrink: 0;
    color: var(--td-warning-color);
    font-weight: 500;
  }

  .matched-text {
    color: var(--td-warning-color-active);
    background: linear-gradient(90deg, rgba(251, 191, 36, 0.15) 0%, rgba(251, 191, 36, 0.05) 100%);
    padding: 1px 6px;
    border-radius: var(--app-radius-xs);
    word-break: break-word;
  }
}

.result-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
  margin-left: auto;
}

.expand-icon {
  flex-shrink: 0;
  font-size: var(--app-text-2xl);
  color: var(--td-text-color-secondary);
  transition: transform var(--app-motion-base) ease;
  cursor: pointer;

  &:hover {
    color: var(--td-brand-color);
  }
}

.score-tag,
.match-type-tag {
  font-size: var(--app-text-sm);
  padding: 4px 8px;
  border-radius: var(--app-radius-sm);
  font-family: var(--app-font-family);
}

.result-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 12px;
  margin-top: 0;
  border-top: 1px solid var(--td-component-stroke);
  position: relative;
  width: 100%;
}

// Slide down animation, optimized for performance
.slide-down-enter-active {
  transition: opacity var(--app-motion-base) cubic-bezier(0.4, 0, 0.2, 1),
    transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  will-change: opacity, transform;
}

.slide-down-leave-active {
  transition: opacity var(--app-motion-base) cubic-bezier(0.4, 0, 0.2, 1),
    transform 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  will-change: opacity, transform;
}

.slide-down-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

.slide-down-enter-to {
  opacity: 1;
  transform: translateY(0);
}

.slide-down-leave-from {
  opacity: 1;
  transform: translateY(0);
}

.slide-down-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.result-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

// Batch tag dialog styles, matching the import dialog
.batch-tag-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  backdrop-filter: blur(4px);
}

.batch-tag-modal {
  position: relative;
  width: 100%;
  max-width: 480px;
  background: var(--td-bg-color-container);
  border-radius: var(--app-radius-xl);
  box-shadow: 0 6px 28px rgba(15, 23, 42, 0.08);
  overflow: hidden;
  display: flex;
  flex-direction: column;

  .batch-tag-close-btn {
    position: absolute;
    top: 20px;
    right: 20px;
    width: 32px;
    height: 32px;
    border: none;
    background: var(--td-bg-color-secondarycontainer);
    border-radius: var(--app-radius-sm);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--td-text-color-secondary);
    transition: all var(--app-motion-base) ease;
    z-index: 10;

    &:hover {
      background: var(--td-bg-color-secondarycontainer);
      color: var(--td-text-color-primary);
    }
  }
}

.batch-tag-container {
  display: flex;
  flex-direction: column;
  padding: 24px;
}

.batch-tag-header {
  margin-bottom: 24px;
  padding-right: 40px;

  .batch-tag-title {
    margin: 0;
    font-size: var(--app-text-3xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
    line-height: 1.4;
  }
}

.batch-tag-content {
  flex: 1;
  min-height: 0;
}

.batch-tag-tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px 16px;
  margin-bottom: 20px;
  background: var(--td-brand-color-light);
  border: 1px solid var(--td-brand-color-focus);
  border-radius: var(--app-radius-md);
  font-size: var(--app-text-base);
  color: var(--td-brand-color);
  line-height: 1.5;

  .tip-icon {
    flex-shrink: 0;
    margin-top: 2px;
    color: var(--td-brand-color);
  }
}

.batch-tag-form {
  margin-top: 0;

  :deep(.t-form-item) {
    margin-bottom: 0;
  }

  :deep(.t-form-item__label) {
    font-size: var(--app-text-base);
    font-weight: 500;
    color: var(--td-text-color-primary);
    margin-bottom: 8px;
  }
}

.batch-tag-select {
  width: 100%;
}

.batch-tag-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--td-component-stroke);
}

.tag-select-empty {
  padding: 8px 12px;
  text-align: center;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-base);
}

.section-label {
  font-family: var(--app-font-family);
  font-size: var(--app-text-sm);
  font-weight: 600;
  color: var(--td-text-color-secondary);
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.result-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  width: 100%;
  min-width: 0;
}

:deep(.result-tags .t-tag) {
  max-width: 100%;
  min-width: 0;
  word-break: break-word;
  overflow-wrap: break-word;
}

:deep(.result-tags .t-tag__text) {
  display: inline-block;
  max-width: 100%;
  word-break: break-word;
  overflow-wrap: break-word;
  white-space: normal;
  line-height: 1.4;
}
</style>
