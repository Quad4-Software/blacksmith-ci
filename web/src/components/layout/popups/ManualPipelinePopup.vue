<template>
  <Dialog :open="open" @update:open="(val) => !val && $emit('close')">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ $t('repo.manual_pipeline.title') }}</DialogTitle>
        <DialogDescription>
          {{ $t('repo.manual_pipeline.desc') }}
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="spinner" class="h-6 w-6 animate-spin" />
      </div>

      <form v-else class="space-y-4" @submit.prevent="triggerManualPipeline">
        <div class="space-y-2">
          <Label for="branch">{{ $t('repo.manual_pipeline.select_branch') }}</Label>
          <Select v-model="payload.branch" required>
            <SelectTrigger id="branch">
              <SelectValue :placeholder="$t('repo.manual_pipeline.select_branch')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="branch in branches" :key="branch" :value="branch">
                {{ branch }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="space-y-2">
          <Label>{{ $t('repo.manual_pipeline.variables.title') }}</Label>
          <p class="text-sm text-muted-foreground">{{ $t('repo.manual_pipeline.variables.desc') }}</p>
          <KeyValueEditor
            v-model="payload.variables"
            :key-placeholder="$t('repo.manual_pipeline.variables.name')"
            :value-placeholder="$t('repo.manual_pipeline.variables.value')"
            :delete-title="$t('repo.manual_pipeline.variables.delete')"
            @update:is-valid="isVariablesValid = $event"
          />
        </div>

        <DialogFooter class="sm:justify-start">
          <Button type="submit" :disabled="!isFormValid || isSubmitting">
            <Icon v-if="isSubmitting" name="spinner" class="mr-1 h-4 w-4 animate-spin" />
            <Icon v-else name="play" class="mr-1 h-4 w-4" />
            {{ $t('repo.manual_pipeline.trigger') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import KeyValueEditor from '~/components/form/KeyValueEditor.vue';
import { Button } from '~/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Label } from '~/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { PipelineFilteredError } from '~/lib/api/client';
import { usePaginate } from '~/compositions/usePaginate';

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  (event: 'close'): void;
}>();

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();
const router = useRouter();

const repo = requiredInject('repo');

const branches = ref<string[]>([]);
const payload = ref<{ branch: string; variables: Record<string, string> }>({
  branch: '',
  variables: {},
});

const isVariablesValid = ref(true);
const loading = ref(false);
const isSubmitting = ref(false);
const hasFetchedBranches = ref(false);

const isFormValid = computed(() => {
  return payload.value.branch !== '' && isVariablesValid.value;
});

// Fetch branches when dialog opens
watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen && !hasFetchedBranches.value) {
      loading.value = true;
      try {
        const data = await usePaginate((page) => apiClient.getRepoBranches(repo.value.id, { page }));
        branches.value = data;
        // Set default branch if available
        if (data.length > 0) {
          // Try to find 'main' or 'master', otherwise use first branch
          payload.value.branch = data.find((b) => b === 'main') || data.find((b) => b === 'master') || data[0];
        }
        hasFetchedBranches.value = true;
      } catch (error) {
        console.error('Failed to fetch branches:', error);
        notifications.notify({
          title: i18n.t('notify_title_error'),
          text: i18n.t('repo.manual_pipeline.fetch_branches_failed'),
          type: 'error',
        });
      } finally {
        loading.value = false;
      }
    }
  },
  { immediate: true },
);

async function triggerManualPipeline() {
  isSubmitting.value = true;

  try {
    const pipeline = await apiClient.createPipeline(repo.value.id, {
      branch: payload.value.branch,
      variables: payload.value.variables,
    });

    emit('close');

    // Reset form for next use
    payload.value.variables = {};

    await router.push({
      name: 'repo-pipeline',
      params: {
        pipelineId: pipeline.number,
      },
    });
  } catch (error: unknown) {
    if (error instanceof PipelineFilteredError) {
      // All steps were filtered out by `when` conditions
      const helpUrl = 'https://crowci.dev/dev/usage/workflow-syntax/#when-step';
      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        duration: 9000,
        text: `${i18n.t('repo.manual_pipeline.steps_filtered_out')} <a href='${helpUrl}' target='_blank' class='text-primary underline'>${i18n.t('repo.manual_pipeline.when_condition_help')}</a>`,
      });
    } else {
      // Generic error - likely manual event not enabled
      const helpUrl = 'https://crowci.dev/dev/usage/workflow-syntax/#event';
      const baseMessage = i18n.t('repo.manual_pipeline.notify_text_failed');
      const helpText = i18n.t('repo.manual_pipeline.manual_event_help');

      notifications.notify({
        title: i18n.t('notify_title_error'),
        type: 'error',
        duration: 9000,
        text: `${baseMessage} <a href='${helpUrl}' target='_blank' class='text-primary underline'>${helpText}</a>`,
      });
    }
    console.error('Failed to trigger pipeline:', error);
  } finally {
    isSubmitting.value = false;
  }
}
</script>
