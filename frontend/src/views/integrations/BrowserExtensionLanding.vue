<template>
  <IntegrationLandingLayout
    :title="$t('integrations.browser.title')"
    :subtitle="$t('integrations.browser.subtitle')"
  >
    <template #actions>
      <IntegrationExternalCta
        :label="$t('integrations.browser.installCta')"
        :hint="$t('integrations.browser.installHint')"
        @click="openExternal(EXTENSION_STORE_URL)"
      >
        <template #icon>
          <t-icon name="extension" size="18px" />
        </template>
      </IntegrationExternalCta>
    </template>

    <template #main>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ $t('integrations.browser.featuresTitle') }}
            <span class="section-head-extra">{{ featureKeys.length }}</span>
          </h4>
          <div class="capability-grid">
            <div v-for="key in featureKeys" :key="key" class="capability-card">
              <div class="capability-card__icon">
                <t-icon :name="featureIcons[key]" />
              </div>
              <h5 class="capability-card__title">{{ $t(`integrations.browser.features.${key}.title`) }}</h5>
              <p class="capability-card__desc">{{ $t(`integrations.browser.features.${key}.desc`) }}</p>
            </div>
          </div>
        </section>
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t('integrations.browser.capabilitiesTitle') }}</h4>
          <p class="landing-step-desc">{{ $t('integrations.browser.capabilitiesDesc') }}</p>
          <div class="capability-grid">
            <div v-for="key in capabilityKeys" :key="key" class="capability-card">
              <div class="capability-card__icon">
                <t-icon :name="capabilityIcons[key]" />
              </div>
              <h5 class="capability-card__title"><code>{{ key }}</code></h5>
              <p class="capability-card__desc">{{ $t(`integrations.browser.capabilities.${key}`) }}</p>
            </div>
          </div>
        </section>
      </div>
    </template>

    <template #aside>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t('integrations.browser.stepsTitle') }}</h4>
          <ol class="landing-steps">
            <li v-for="(step, index) in stepKeys" :key="step" class="landing-step">
              <span class="landing-step-num">{{ index + 1 }}</span>
              <div class="landing-step-body">
                <div class="landing-step-title">{{ $t(`integrations.browser.steps.${step}.title`) }}</div>
                <p class="landing-step-desc">{{ $t(`integrations.browser.steps.${step}.desc`) }}</p>
                <t-button
                  v-if="step === 'key'"
                  size="small"
                  variant="outline"
                  class="landing-step-action"
                  @click="openApiSettings"
                >
                  {{ $t('integrations.browser.openApiSettings') }}
                </t-button>
                <div v-if="step === 'connect'" class="landing-step-embed">
                  <div class="api-key-control landing-api-control">
                    <t-input :model-value="serverUrl" readonly />
                    <t-button
                      size="small"
                      variant="text"
                      :title="$t('integrations.browser.copy')"
                      @click="copyServerUrl"
                    >
                      <t-icon name="file-copy" size="16px" />
                    </t-button>
                  </div>
                </div>
              </div>
            </li>
          </ol>
        </section>
      </div>
    </template>

    <template #footer>
      <div class="landing-footer-bar" role="note">
        <p class="landing-footer-bar__note">{{ $t('integrations.browser.footerNote') }}</p>
        <a v-if="siteLinks.githubUrl" :href="`${siteLinks.githubUrl}/releases?q=extension`" target="_blank" rel="noopener noreferrer" class="landing-meta">
          {{ $t('integrations.browser.releases') }}
        </a>
        <a :href="docsUrl('browserExtension')" target="_blank" rel="noopener noreferrer" class="landing-meta">
          {{ $t('integrations.browser.docs') }}
        </a>
      </div>
    </template>
  </IntegrationLandingLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { EXTENSION_STORE_URL } from '@/config/integrations'
import { useSiteLinksStore } from '@/stores/siteLinks'
import { useApiBaseUrlDisplay } from '@/composables/useApiBaseUrlDisplay'
import { copyWithToast } from '@/utils/clipboard'
import { docsUrl } from '@/utils/docsUrl'
import IntegrationLandingLayout from './IntegrationLandingLayout.vue'
import IntegrationExternalCta from './IntegrationExternalCta.vue'
import { useIntegrationNav } from './useIntegrationNav'

const { openExternal, openApiSettings } = useIntegrationNav()
const siteLinks = useSiteLinksStore()
const { apiBaseUrlDisplay } = useApiBaseUrlDisplay()

const featureKeys = ['qa', 'clip', 'select', 'note', 'shortcuts'] as const
const capabilityKeys = ['chat', 'retrieve', 'ingest'] as const
const stepKeys = ['install', 'key', 'connect'] as const

const featureIcons: Record<(typeof featureKeys)[number], string> = {
  qa: 'chat-bubble',
  clip: 'download',
  select: 'file-copy',
  note: 'edit',
  shortcuts: 'keyboard',
}

const capabilityIcons: Record<(typeof capabilityKeys)[number], string> = {
  chat: 'chat-bubble',
  retrieve: 'search',
  ingest: 'upload',
}

// The extension takes the site root and adds /api/v1 itself.
const serverUrl = computed(() => apiBaseUrlDisplay.value.replace(/\/api\/v1$/, ''))

const copyServerUrl = () => copyWithToast(serverUrl.value, 'integrations.browser.copySuccess')
</script>
