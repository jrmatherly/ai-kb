<template>
  <div class="integrations-settings">
    <div class="integrations-settings__body">
      <div v-if="tab === 'im'" class="section">
        <div class="section-header">
          <h2>{{ $t('agentEditor.im.title') }}</h2>
          <p class="section-description">
            {{ imDescription }}
            <a
              :href="docsUrl('imIntegration')"
              target="_blank"
              rel="noopener noreferrer"
              class="doc-link"
            >
              {{ $t('agentEditor.im.docLink') }}
              <t-icon name="link" class="link-icon" />
            </a>
          </p>
        </div>
        <IMChannelPanel v-model:filter-agent-id="filterAgentId" />
      </div>

      <div v-if="tab === 'embed'" class="section">
        <div class="section-header">
          <h2>{{ $t('agentEditor.embed.title') }}</h2>
          <p class="section-description">{{ $t('agentEditor.embed.description') }}</p>
        </div>
        <AgentEmbedChannelPanel v-model:filter-agent-id="filterAgentId" />
      </div>

      <div v-if="tab === 'api'" class="section">
        <div class="section-header">
          <h2>{{ $t('integrations.api.title') }}</h2>
          <p class="section-description">{{ $t('integrations.api.subtitle') }}</p>
        </div>
        <ApiIntegrationSettings />
      </div>

      <div v-if="tab === 'mcpserver'" class="section">
        <div class="section-header">
          <h2>{{ $t('integrations.mcpserver.title') }}</h2>
          <p class="section-description">{{ $t('integrations.mcpserver.subtitle') }}</p>
        </div>
        <McpServerIntegrationSettings />
      </div>

      <AgentSkillLanding v-if="tab === 'claw'" />
      <CliIntegrationLanding v-if="tab === 'cli'" />
      <BrowserExtensionLanding v-if="tab === 'browser'" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'
import IMChannelPanel from '@/components/IMChannelPanel.vue'
import AgentEmbedChannelPanel from '@/components/AgentEmbedChannelPanel.vue'
import ApiIntegrationSettings from '@/views/integrations/ApiIntegrationSettings.vue'
import McpServerIntegrationSettings from '@/views/integrations/McpServerIntegrationSettings.vue'
import AgentSkillLanding from '@/views/integrations/AgentSkillLanding.vue'
import CliIntegrationLanding from '@/views/integrations/CliIntegrationLanding.vue'
import BrowserExtensionLanding from '@/views/integrations/BrowserExtensionLanding.vue'
import type { IntegrationTab } from '@/config/integrations'
import { docsUrl } from '@/utils/docsUrl'

const filterAgentId = ref('')
const { t, te } = useI18n()
const deploymentCapabilities = useDeploymentCapabilitiesStore()

const imDescription = computed(() => {
  const platforms = deploymentCapabilities.imPlatforms
  if (platforms === undefined) return t('agentEditor.im.descriptionAll')
  if (platforms.length === 0) return t('agentEditor.im.descriptionNone')
  const label = (p: string) => (te(`agentEditor.im.${p}`) ? t(`agentEditor.im.${p}`) : p)
  return t('agentEditor.im.description', { platforms: platforms.map(label).join(', ') })
})

defineProps<{
  tab: IntegrationTab
}>()

const route = useRoute()

function applyAgentFilterFromRoute() {
  filterAgentId.value = (route.query.agentId as string) || ''
}

watch(
  () => route.query.agentId,
  applyAgentFilterFromRoute,
  { immediate: true },
)
</script>

<style scoped lang="less">
@import (reference) '@/components/css/settings-section.less';

.integrations-settings {
  display: flex;
  flex-direction: column;
}

.integrations-settings__body {
  min-width: 0;
}

.section-header {
  .settings-section-header();
}

.doc-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  margin-left: 6px;
  color: var(--td-brand-color);
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}

.link-icon {
  font-size: var(--app-text-md);
}
</style>
