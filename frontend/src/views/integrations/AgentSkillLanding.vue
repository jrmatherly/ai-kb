<template>
  <IntegrationLandingLayout
    :title="$t('integrations.claw.title')"
    :subtitle="$t('integrations.claw.subtitle')"
  >
    <template #main>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ $t('integrations.claw.capabilitiesTitle') }}
            <span class="section-head-extra">{{ capabilityKeys.length }}</span>
          </h4>
          <div class="capability-grid">
            <div v-for="key in capabilityKeys" :key="key" class="capability-card">
              <div class="capability-card__icon">
                <t-icon :name="capabilityIcons[key]" />
              </div>
              <h5 class="capability-card__title">{{ $t(`integrations.claw.capabilities.${key}.title`) }}</h5>
              <p class="capability-card__desc">{{ $t(`integrations.claw.capabilities.${key}.desc`) }}</p>
            </div>
          </div>
        </section>
      </div>
    </template>

    <template #aside>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t('integrations.claw.stepsTitle') }}</h4>
          <ol class="landing-steps">
            <li v-for="(step, index) in stepKeys" :key="step" class="landing-step">
              <span class="landing-step-num">{{ index + 1 }}</span>
              <div class="landing-step-body">
                <div class="landing-step-title">{{ $t(`integrations.claw.steps.${step}.title`) }}</div>
                <p class="landing-step-desc">{{ $t(`integrations.claw.steps.${step}.desc`) }}</p>
                <t-button
                  v-if="step === 'cli'"
                  size="small"
                  variant="outline"
                  class="landing-step-action"
                  @click="openIntegrationTab('cli')"
                >
                  {{ $t('integrations.claw.openCli') }}
                </t-button>
                <div v-if="step === 'install'" class="landing-step-embed">
                  <div class="code-toolbar">
                    <pre class="code-toolbar__code">{{ installCommand }}</pre>
                    <t-button
                      class="code-toolbar__copy"
                      size="small"
                      variant="text"
                      shape="square"
                      :title="$t('integrations.claw.copy')"
                      @click="copyInstallCommand"
                    >
                      <t-icon name="file-copy" size="16px" />
                    </t-button>
                  </div>
                </div>
                <t-button
                  v-if="step === 'mcp'"
                  size="small"
                  variant="outline"
                  class="landing-step-action"
                  @click="openIntegrationTab('mcpserver')"
                >
                  {{ $t('integrations.claw.openMcp') }}
                </t-button>
              </div>
            </li>
          </ol>
        </section>
      </div>
    </template>

    <template #footer>
      <div class="landing-footer-bar" role="note">
        <p class="landing-footer-bar__note">{{ $t('integrations.claw.ecosystemNote') }}</p>
        <span class="landing-meta">{{ $t('integrations.claw.hubMeta') }}</span>
      </div>
    </template>
  </IntegrationLandingLayout>
</template>

<script setup lang="ts">
import { copyWithToast } from '@/utils/clipboard'
import IntegrationLandingLayout from './IntegrationLandingLayout.vue'
import { useIntegrationNav } from './useIntegrationNav'

const { openIntegrationTab } = useIntegrationNav()

const capabilityKeys = ['search', 'bases', 'answer'] as const
const stepKeys = ['cli', 'install', 'mcp', 'verify'] as const

const capabilityIcons: Record<(typeof capabilityKeys)[number], string> = {
  search: 'search',
  bases: 'view-list',
  answer: 'chat',
}

const installCommand = 'aikb skills install --dir ~/.claude/skills'

const copyInstallCommand = () => copyWithToast(installCommand, 'integrations.claw.copyCmdSuccess')
</script>
