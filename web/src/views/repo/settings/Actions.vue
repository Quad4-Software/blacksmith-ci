<template>
  <Settings :title="$t('repo.settings.actions.actions')">
    <ActionsButtons />
  </Settings>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import Settings from '~/components/layout/Settings.vue';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useCrowTitle } from '~/compositions/useCrowTitle';

import ActionsButtons from './ActionsButtons.vue';

const i18n = useI18n();
const repo = requiredInject('repo');

useCrowTitle(computed(() => [i18n.t('repo.settings.actions.actions'), repo.value.full_name]));
</script>
