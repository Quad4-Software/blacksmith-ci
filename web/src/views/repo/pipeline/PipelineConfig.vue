<template>
  <div class="flex flex-col gap-y-4">
    <Panel
      v-for="pipelineConfig in pipelineConfigsDecoded"
      :key="pipelineConfig.hash"
      :collapsable="pipelineConfigsDecoded && pipelineConfigsDecoded.length > 1"
      :open="isPanelOpen(pipelineConfig.name)"
      :title="showTitles ? pipelineConfig.name : ''"
      @update:open="(value) => setPanelOpen(pipelineConfig.name, value)"
    >
      <template v-if="showTitles" #title>
        <span class="truncate font-mono text-sm">{{ pipelineConfig.name }}</span>
        <span class="flex-1" />
        <Badge v-if="pipelineConfig.generated_from" variant="default">
          {{ $t('repo.pipeline.config_generated') }}
        </Badge>
        <Badge
          v-if="pipelineConfig.generated_from"
          variant="outline"
          class="font-mono font-normal"
          :title="$t('repo.pipeline.config_generated_from', { file: pipelineConfig.generated_from })"
        >
          {{ $t('repo.pipeline.config_generated_from_short', { file: baseName(pipelineConfig.generated_from) }) }}
        </Badge>
        <Badge v-else-if="isLibsonnet(pipelineConfig.name)" variant="outline">
          {{ $t('repo.pipeline.config_import_only') }}
        </Badge>
        <Badge v-else-if="isJsonnet(pipelineConfig.name)" variant="outline">
          {{ $t('repo.pipeline.config_source') }}
        </Badge>
      </template>
      <div class="overflow-auto">
        <table class="w-full border-collapse font-mono text-sm">
          <tbody>
            <tr
              v-for="(line, index) in getLines(pipelineConfig.data)"
              :id="`${pipelineConfig.name}-L${index + 1}`"
              :key="index"
              :ref="(el) => registerLineRef(pipelineConfig.name, index + 1, el as HTMLElement | null)"
              class="hover:bg-muted/50"
              :class="{ 'bg-primary/10': isHighlightedLine(pipelineConfig.name, index + 1) }"
            >
              <td
                class="select-none pr-4 pl-2 text-right text-muted-foreground w-[1%] whitespace-nowrap align-top border-r border-border"
              >
                {{ index + 1 }}
              </td>
              <td class="pl-4 whitespace-pre">
                <SyntaxHighlight language="yaml" :code="line || ' '" class="inline" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </Panel>
  </div>
</template>

<script lang="ts" setup>
import { decode } from 'js-base64';
import { computed, nextTick, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';

import SyntaxHighlight from '~/components/atomic/SyntaxHighlight';
import Panel from '~/components/layout/Panel.vue';
import { Badge } from '~/components/ui/badge';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useCrowTitle } from '~/compositions/useCrowTitle';

const YAML_NAME_REGEX = /^name:\s*["']?([^"'\n]+)["']?/m;

const route = useRoute();
const repo = requiredInject('repo');
const pipeline = requiredInject('pipeline');
const pipelineConfigs = requiredInject('pipeline-configs');

const pipelineConfigsDecoded = computed(
  () =>
    pipelineConfigs.value?.map((i) => ({
      ...i,
      data: decode(i.data),
    })) ?? [],
);

const showTitles = computed(() => pipelineConfigsDecoded.value.length > 1);
const hasGeneratedConfigs = computed(() => pipelineConfigsDecoded.value.some((c) => !!c.generated_from));

function isJsonnet(fileName: string): boolean {
  return fileName.endsWith('.jsonnet');
}

function isLibsonnet(fileName: string): boolean {
  return fileName.endsWith('.libsonnet');
}

function baseName(filePath: string): string {
  return filePath.split('/').pop() ?? filePath;
}

// Track open state for each panel - use reactive object for proper Vue reactivity
const panelOpenState = ref<Record<string, boolean>>({});

// Query params for line highlighting
const targetFile = computed(() => (route.query.file as string) || null);
const targetLine = computed(() => {
  const line = route.query.line;
  return line ? Number(line) : null;
});

// Check if this config should be the target (for expanding and highlighting)
function isTargetConfig(configName: string): boolean {
  // If there's a target line specified
  if (!targetLine.value) return false;

  // If only one config, it's always the target
  if (pipelineConfigsDecoded.value.length === 1) return true;

  // Pipeline errors report the file the step builder worked on, which for a
  // single-document Jsonnet is still the .jsonnet source. Send the link to the
  // YAML generated from it instead — that is where the line number is valid.
  const generatedFromTarget = pipelineConfigsDecoded.value.find((c) => c.generated_from === targetFile.value);
  if (generatedFromTarget) {
    return configName === generatedFromTarget.name;
  }

  // If no file specified in query, target the first config as fallback
  if (!targetFile.value) {
    return configName === pipelineConfigsDecoded.value[0]?.name;
  }

  // Try exact match on config name first
  if (configName === targetFile.value) return true;

  // Try matching just the filename part (in case of path differences)
  const configBaseName = configName.split('/').pop();
  const targetBaseName = targetFile.value.split('/').pop();
  if (configBaseName === targetBaseName) return true;

  // Try matching by workflow name inside the YAML content
  // The targetFile might be a workflow name (e.g., "myname") rather than a filename
  const config = pipelineConfigsDecoded.value.find((c) => c.name === configName);
  if (config) {
    // Check if the YAML contains a "name:" field matching the target
    const nameMatch = config.data.match(YAML_NAME_REGEX);
    if (nameMatch && nameMatch[1].trim() === targetFile.value) {
      return true;
    }
  }

  // Fallback: if no config matches by name or content, use the first config
  const target = targetFile.value;
  const anyMatch = pipelineConfigsDecoded.value.some((c) => {
    if (c.name === target || c.name.split('/').pop() === target.split('/').pop()) return true;
    // Also check workflow name in content
    const nameMatch = c.data.match(YAML_NAME_REGEX);
    return nameMatch && nameMatch[1].trim() === target;
  });
  if (!anyMatch) {
    return configName === pipelineConfigsDecoded.value[0]?.name;
  }

  return false;
}

function isHighlightedLine(configName: string, lineNumber: number): boolean {
  return isTargetConfig(configName) && targetLine.value === lineNumber;
}

function getLines(code: string): string[] {
  return code.split('\n');
}

// Line refs for scrolling
const lineRefs = new Map<string, HTMLElement>();

function registerLineRef(fileName: string, lineNumber: number, el: HTMLElement | null) {
  const key = `${fileName}-L${lineNumber}`;
  if (el) {
    lineRefs.set(key, el);
  } else {
    lineRefs.delete(key);
  }
}

// Get panel open state.
// Jsonnet sources and libraries start collapsed while the YAML they evaluate to
// starts expanded: when a repo generates its workflows, the generated document is
// what you came to this tab to read.
function isPanelOpen(configName: string): boolean {
  if (!(configName in panelOpenState.value)) {
    if (pipelineConfigsDecoded.value.length <= 1) {
      return true;
    }
    return hasGeneratedConfigs.value && !isJsonnet(configName) && !isLibsonnet(configName);
  }
  return panelOpenState.value[configName];
}

function setPanelOpen(configName: string, value: boolean) {
  panelOpenState.value = { ...panelOpenState.value, [configName]: value };
}

function expandAndScrollToTarget() {
  if (!targetLine.value) return;
  if (pipelineConfigsDecoded.value.length === 0) return;

  // Find the target config
  const targetConfig = pipelineConfigsDecoded.value.find((config) => isTargetConfig(config.name));
  if (!targetConfig) return;

  // Expand the target panel
  setPanelOpen(targetConfig.name, true);

  // Wait for panel to expand and DOM to update, then scroll
  // Use retry mechanism since collapsed panel content takes time to render
  const scrollToLine = (retries = 5) => {
    const key = `${targetConfig.name}-L${targetLine.value}`;
    const el = lineRefs.get(key);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    } else if (retries > 0) {
      setTimeout(scrollToLine, 100, retries - 1);
    }
  };

  nextTick(() => {
    setTimeout(scrollToLine, 50);
  });
}

// Watch for route query changes
watch([targetFile, targetLine], () => {
  expandAndScrollToTarget();
});

// Watch for configs to load (important when navigating directly to this tab)
watch(
  pipelineConfigsDecoded,
  (configs) => {
    if (configs.length > 0 && targetLine.value) {
      expandAndScrollToTarget();
    }
  },
  { immediate: true },
);

const { t } = useI18n();
useCrowTitle(
  computed(() => [
    t('repo.pipeline.config'),
    t('repo.pipeline.pipeline', { pipelineId: pipeline.value.number }),
    repo.value.full_name,
  ]),
);
</script>
