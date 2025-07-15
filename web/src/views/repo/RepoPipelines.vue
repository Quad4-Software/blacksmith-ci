<template><PipelineList :repo="repo" :infinite-scroll="true" /></template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import PipelineList from '~/components/repo/pipeline/PipelineList.vue';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useCrowTitle } from '~/compositions/useCrowTitle';

const repo = requiredInject('repo');

const { t } = useI18n();
useCrowTitle(computed(() => [t('repo.activity'), repo.value.full_name]));
</script>
