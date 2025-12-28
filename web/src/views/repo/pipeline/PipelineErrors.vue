<template>
  <Panel>
    <div class="flex flex-col gap-y-4">
      <template v-for="(error, _index) in pipeline!.errors" :key="_index">
        <div
          class="rounded-lg border p-4"
          :class="{
            'border-yellow-500/30 bg-yellow-500/5': error.is_warning,
            'border-destructive/30 bg-destructive/5': !error.is_warning,
          }"
        >
          <!-- Header row with error type badge and location -->
          <div class="flex flex-wrap items-center gap-2 mb-3">
            <!-- Error type badge -->
            <span
              class="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium"
              :class="{
                'bg-yellow-500/20 text-yellow-700 dark:text-yellow-400': error.is_warning,
                'bg-destructive/20 text-destructive': !error.is_warning,
              }"
            >
              <Icon
                name="alert"
                class="size-3.5"
                :class="{
                  'text-yellow-600 dark:text-yellow-400': error.is_warning,
                  'text-destructive': !error.is_warning,
                }"
              />
              {{ getErrorTypeLabel(error.type) }}
            </span>

            <!-- File and field/step location -->
            <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
            <span
              v-if="hasLocationData(error)"
              class="inline-flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs font-mono"
            >
              <template v-if="getErrorData(error)?.file">
                <span class="text-muted-foreground">{{ getErrorData(error)?.file }}</span>
                <span v-if="getFieldOrStep(error)" class="text-muted-foreground/50">:</span>
              </template>
              <span v-if="getFieldOrStep(error)" class="font-semibold text-foreground">{{
                getFieldOrStep(error)
              }}</span>
              <template v-if="getLinterData(error)?.line">
                <span class="text-muted-foreground/50">@</span>
                <span class="text-primary">{{
                  t('repo.pipeline.error_location.line', { line: getLinterData(error)?.line })
                }}</span>
              </template>
            </span>
            <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->

            <!-- Docs link for deprecations and bad habits -->
            <DocsLink
              v-if="(isDeprecationError(error) || isBadHabitError(error)) && getDocsUrl(error)"
              :topic="getFieldOrStep(error) || ''"
              :url="getDocsUrl(error) || ''"
              class="ml-auto"
            />
          </div>

          <!-- Error message -->
          <div class="pl-0.5">
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
import type {
  BadHabitErrorData,
  CompilerErrorData,
  DeprecationErrorData,
  LinterErrorData,
  PipelineError,
  PipelineErrorType,
} from '~/lib/api/types';

const repo = requiredInject('repo');
const pipeline = requiredInject('pipeline');

type ErrorData = LinterErrorData | DeprecationErrorData | BadHabitErrorData | CompilerErrorData;

function isLinterError(error: PipelineError): error is PipelineError<LinterErrorData> {
  return error.type === 'linter';
}

function isDeprecationError(error: PipelineError): error is PipelineError<DeprecationErrorData> {
  return error.type === 'deprecation';
}

function isBadHabitError(error: PipelineError): error is PipelineError<BadHabitErrorData> {
  return error.type === 'Best practice';
}

function isCompilerError(error: PipelineError): error is PipelineError<CompilerErrorData> {
  return error.type === 'compiler';
}

function hasLocationData(error: PipelineError): boolean {
  if (isLinterError(error) || isDeprecationError(error) || isBadHabitError(error)) {
    return true;
  }
  if (isCompilerError(error)) {
    const data = error.data;
    return !!(data?.file || data?.step);
  }
  return false;
}

function getErrorData(error: PipelineError): ErrorData | undefined {
  if (isLinterError(error) || isDeprecationError(error) || isBadHabitError(error) || isCompilerError(error)) {
    return error.data as ErrorData;
  }
  return undefined;
}

function getLinterData(error: PipelineError): LinterErrorData | undefined {
  if (isLinterError(error)) {
    return error.data;
  }
  return undefined;
}

function getFieldOrStep(error: PipelineError): string | undefined {
  if (isLinterError(error) || isDeprecationError(error) || isBadHabitError(error)) {
    return (error.data as LinterErrorData | DeprecationErrorData | BadHabitErrorData)?.field;
  }
  if (isCompilerError(error)) {
    return error.data?.step;
  }
  return undefined;
}

function getDocsUrl(error: PipelineError): string | undefined {
  if (isDeprecationError(error)) {
    return error.data?.docs;
  }
  if (isBadHabitError(error)) {
    return error.data?.docs;
  }
  return undefined;
}

const { t } = useI18n();

function getErrorTypeLabel(type: PipelineErrorType): string {
  const labels: Record<PipelineErrorType, string> = {
    linter: t('repo.pipeline.error_types.schema'),
    deprecation: t('repo.pipeline.error_types.deprecation'),
    compiler: t('repo.pipeline.error_types.compiler'),
    generic: t('repo.pipeline.error_types.generic'),
    'Best practice': t('repo.pipeline.error_types.best_practice'),
  };
  return labels[type] || type;
}
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
  margin-top: 0.75rem;
}

:deep(.error-markdown-content h3) {
  font-size: 1.125rem;
  font-weight: 600;
  color: hsl(var(--foreground));
  margin-bottom: 0.5rem;
}

:deep(.error-markdown-content h4) {
  font-size: 1rem;
  font-weight: 600;
  color: hsl(var(--foreground));
  margin-top: 0.75rem;
  margin-bottom: 0.375rem;
}

:deep(.error-markdown-content p) {
  color: hsl(var(--foreground));
  line-height: 1.625;
  margin-top: 0.375rem;
  margin-bottom: 0.375rem;
}

:deep(.error-markdown-content ul) {
  list-style-type: disc;
  padding-left: 1.5rem;
  margin-top: 0.375rem;
  margin-bottom: 0.375rem;
}

:deep(.error-markdown-content li) {
  color: hsl(var(--foreground));
  margin-top: 0.125rem;
}

:deep(.error-markdown-content code) {
  background-color: hsl(var(--muted));
  color: hsl(var(--foreground));
  padding: 0.125rem 0.375rem;
  border-radius: 0.25rem;
  font-size: 0.875rem;
  font-family: ui-monospace, monospace;
}

:deep(.error-markdown-content strong) {
  font-weight: 600;
  color: hsl(var(--foreground));
}

:deep(.error-markdown-content table) {
  width: 100%;
  border-collapse: collapse;
  margin-top: 0.75rem;
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
}

:deep(.error-markdown-content thead) {
  background-color: hsl(var(--card));
}

:deep(.error-markdown-content th) {
  text-align: left;
  padding: 0.5rem 1rem;
  font-weight: 600;
  color: hsl(var(--foreground));
  border: 1px solid hsl(var(--border));
}

:deep(.error-markdown-content td) {
  padding: 0.5rem 1rem;
  color: hsl(var(--foreground));
  border: 1px solid hsl(var(--border));
}

:deep(.error-markdown-content tbody tr:nth-child(even)) {
  background-color: hsl(var(--background));
}

:deep(.error-markdown-content tbody tr:hover) {
  background-color: hsl(var(--card));
}
</style>
