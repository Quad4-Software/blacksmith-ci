<template>
  <Icon
    v-if="indicator"
    :name="indicator.icon"
    class="h-4 w-4 shrink-0"
    :class="indicator.color"
    :title="indicator.title"
    :aria-label="indicator.title"
    data-testid="agent-compatibility"
    :data-compatibility="compatibility"
  />
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import useConfig from '~/compositions/useConfig';
import { getAgentCompatibility } from '~/lib/agent-utils';
import type { Agent } from '~/lib/api/types';

const props = defineProps<{
  agent: Pick<Agent, 'version' | 'protocol_version'>;
}>();

const { t } = useI18n();
const config = useConfig();

const compatibility = computed(() => getAgentCompatibility(props.agent, config.version, config.agentProtocol));

const indicator = computed(() => {
  const params = {
    agent: props.agent.protocol_version,
    server: config.agentProtocol?.version,
    min: config.agentProtocol?.min_agent,
  };
  switch (compatibility.value) {
    case 'incompatible':
      return {
        icon: 'alert' as const,
        color: 'text-red-500',
        title: t('admin.settings.agents.version.incompatible', params),
      };
    case 'outdated':
      return {
        icon: 'alert' as const,
        color: 'text-yellow-500',
        title: t('admin.settings.agents.version.outdated', params),
      };
    case 'release_differs':
      return {
        icon: 'info' as const,
        color: 'text-muted-foreground',
        title: t('admin.settings.agents.version.mismatch'),
      };
    default:
      return null;
  }
});
</script>
