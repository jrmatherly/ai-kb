<template>
  <IntegrationLandingLayout
    :title="$t('integrations.cli.title')"
    :subtitle="$t('integrations.cli.subtitle')"
    variant="cli"
  >
    <template #actions>
      <IntegrationExternalCta
        :label="$t('integrations.cli.docs')"
        :hint="$t('integrations.cli.docsHint')"
        @click="openExternal(CLI_README_URL)"
      >
        <template #icon><t-icon name="code" /></template>
      </IntegrationExternalCta>
    </template>

    <template #main>
      <div class="landing-group">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t('integrations.cli.quickstart') }}</h4>
          <ol class="landing-steps">
            <li v-for="(step, index) in steps" :key="step.key" class="landing-step">
              <span class="landing-step-num">{{ index + 1 }}</span>
              <div class="landing-step-body">
                <div class="landing-step-title">{{ $t(`integrations.cli.${step.key}Title`) }}</div>
                <p class="landing-step-desc">{{ $t(`integrations.cli.${step.key}Desc`) }}</p>
                <div class="landing-step-embed code-toolbar">
                  <pre class="code-toolbar__code">{{ step.command }}</pre>
                  <t-button
                    class="code-toolbar__copy"
                    size="small"
                    variant="text"
                    shape="square"
                    :title="$t('integrations.cli.copy')"
                    :aria-label="$t('integrations.cli.copy')"
                    @click="copy(step.command)"
                  >
                    <t-icon name="file-copy" size="16px" />
                  </t-button>
                </div>
              </div>
            </li>
          </ol>
        </section>
      </div>
    </template>

    <template #aside>
      <div class="landing-group">
        <section v-for="example in examples" :key="example.key" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t(`integrations.cli.${example.key}Title`) }}</h4>
          <p class="field-desc">{{ $t(`integrations.cli.${example.key}Desc`) }}</p>
          <div class="code-toolbar">
            <pre class="code-toolbar__code">{{ example.command }}</pre>
            <t-button
              class="code-toolbar__copy"
              size="small"
              variant="text"
              shape="square"
              :title="$t('integrations.cli.copy')"
              :aria-label="$t('integrations.cli.copy')"
              @click="copy(example.command)"
            >
              <t-icon name="file-copy" size="16px" />
            </t-button>
          </div>
        </section>
      </div>
    </template>
  </IntegrationLandingLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useApiBaseUrlDisplay } from '@/composables/useApiBaseUrlDisplay'
import { copyWithToast } from '@/utils/clipboard'
import IntegrationLandingLayout from './IntegrationLandingLayout.vue'
import IntegrationExternalCta from './IntegrationExternalCta.vue'
import { buildCLIConnectCommand } from './cliIntegration'
import { useIntegrationNav } from './useIntegrationNav'

const CLI_README_URL = 'https://github.com/jrmatherly/ai-kb/blob/main/cli/README.md'
const { openExternal } = useIntegrationNav()

const { apiBaseUrlDisplay } = useApiBaseUrlDisplay()
const steps = computed(() => [
  {
    key: 'install',
    command: 'brew install jrmatherly/ai-kb/aikb\naikb --version',
  },
  { key: 'connect', command: buildCLIConnectCommand(apiBaseUrlDisplay.value, window.location.origin) },
  { key: 'verify', command: 'aikb doctor\naikb kb list' },
])
const examples = [
  {
    key: 'commands',
    command: 'aikb doc upload ./document.pdf --kb "KB_ID"\naikb search chunks "query" --kb "KB_ID"\naikb chat "question" --kb "KB_ID" --format text\naikb agent list',
  },
  {
    key: 'mcp',
    command: JSON.stringify({
      mcpServers: {
        aikb: { command: 'aikb', args: ['--profile', 'aikb', 'mcp', 'serve'] },
      },
    }, null, 2),
  },
]

const copy = (command: string) => copyWithToast(command, 'integrations.cli.copied')
</script>
