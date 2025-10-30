<template>
  <Panel>
    <div class="flex flex-col gap-y-6">
      <template v-for="(error, _index) in pipeline!.errors" :key="_index">
        <div class="flex flex-col gap-y-3">
          <div class="flex items-center gap-x-2">
            <Icon
              name="alert"
              class="shrink-0"
              :class="{
                'text-wp-state-warn-100': error.is_warning,
                'text-wp-error-100': !error.is_warning,
              }"
            />
            <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
            <span>
              <code>{{ error.type }}</code>
            </span>
            <span
              v-if="isLinterError(error) || isDeprecationError(error) || isBadHabitError(error)"
              class="flex items-center gap-x-2 whitespace-nowrap"
            >
              <span>
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                <span v-if="error.data?.file" class="font-bold">{{ error.data?.file }}<Text>:</Text></span>
                <span>{{ error.data?.field }}</span>
              </span>
              <DocsLink
                v-if="isDeprecationError(error) || isBadHabitError(error)"
                :topic="error.data?.field || ''"
                :url="error.data?.docs || ''"
              />
            </span>
          </div>

          <div class="w-full">
            <RenderMarkdown :content="error.message" class="error-markdown-content" />
          </div>
        </div>
      </template>
    </div>
  </Panel>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import DocsLink from '~/components/atomic/DocsLink.vue';
import Icon from '~/components/atomic/Icon.vue';
import RenderMarkdown from '~/components/atomic/RenderMarkdown.vue';
import Panel from '~/components/layout/Panel.vue';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { PipelineError } from '~/lib/api/types';

const repo = requiredInject('repo');
const pipeline = requiredInject('pipeline');

function isLinterError(error: PipelineError): error is PipelineError<{ file?: string; field: string }> {
  return error.type === 'linter';
}

function isDeprecationError(
  error: PipelineError,
): error is PipelineError<{ file: string; field: string; docs: string }> {
  return error.type === 'deprecation';
}

function isBadHabitError(error: PipelineError): error is PipelineError<{ file?: string; field: string; docs: string }> {
  return error.type === 'Best practice';
}

const { t } = useI18n();
useCrowTitle(
  computed(() => [
    pipeline.value.errors?.some((e) => !e.is_warning) ? t('repo.pipeline.errors') : t('repo.pipeline.warnings'),
    t('repo.pipeline.pipeline', { pipelineId: pipeline.value.number }),
    repo.value.full_name,
  ]),
);
</script>

<style scoped>
:deep(.error-markdown-content) > * + * {
  margin-top: 1rem;
}

:deep(.error-markdown-content h3) {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--wp-text-100);
  margin-bottom: 0.75rem;
}

:deep(.error-markdown-content h4) {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--wp-text-100);
  margin-top: 1rem;
  margin-bottom: 0.5rem;
}

:deep(.error-markdown-content p) {
  color: var(--wp-text-100);
  line-height: 1.625;
  margin-top: 0.5rem;
  margin-bottom: 0.5rem;
}

:deep(.error-markdown-content ul) {
  list-style-type: disc;
  padding-left: 1.5rem;
  margin-top: 0.5rem;
  margin-bottom: 0.5rem;
}

:deep(.error-markdown-content li) {
  color: var(--wp-text-100);
  margin-top: 0.25rem;
}

:deep(.error-markdown-content code) {
  background-color: var(--wp-code-bg);
  color: var(--wp-code-text);
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-size: 0.875rem;
  font-family: ui-monospace, monospace;
}

:deep(.error-markdown-content strong) {
  font-weight: 600;
  color: var(--wp-text-100);
}

:deep(.error-markdown-content table) {
  width: 100%;
  border-collapse: collapse;
  margin-top: 1rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}

:deep(.error-markdown-content thead) {
  background-color: var(--wp-background-200);
}

:deep(.error-markdown-content th) {
  text-align: left;
  padding: 0.5rem 1rem;
  font-weight: 600;
  color: var(--wp-text-100);
  border: 1px solid var(--wp-background-300);
}

:deep(.error-markdown-content td) {
  padding: 0.5rem 1rem;
  color: var(--wp-text-100);
  border: 1px solid var(--wp-background-300);
}

:deep(.error-markdown-content tbody tr:nth-child(even)) {
  background-color: var(--wp-background-100);
}

:deep(.error-markdown-content tbody tr:hover) {
  background-color: var(--wp-background-200);
}
</style>
