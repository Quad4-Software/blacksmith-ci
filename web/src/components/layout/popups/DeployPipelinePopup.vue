<template>
  <Dialog :open="open" @update:open="(val) => !val && $emit('close')">
    <DialogContent class="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
      <DialogHeader class="text-left">
        <DialogTitle>{{ $t('repo.deploy_pipeline.title', { pipelineId: pipelineNumber }) }}</DialogTitle>
      </DialogHeader>

      <form class="space-y-4" @submit.prevent="triggerDeployPipeline">
        <InputField v-slot="{ id }" :label="$t('repo.deploy_pipeline.enter_target.title')">
          <span class="mb-2 text-sm text-muted-foreground">{{ $t('repo.deploy_pipeline.enter_target.desc') }}</span>
          <Input :id="id" v-model="payload.environment" required />
        </InputField>
        <InputField v-slot="{ id }" :label="$t('repo.deploy_pipeline.enter_task.title')">
          <span class="mb-2 text-sm text-muted-foreground">{{ $t('repo.deploy_pipeline.enter_task.desc') }}</span>
          <Input :id="id" v-model="payload.task" />
        </InputField>
        <InputField v-slot="{ id }" :label="$t('repo.deploy_pipeline.variables.title')">
          <span class="mb-2 text-sm text-muted-foreground">{{ $t('repo.deploy_pipeline.variables.desc') }}</span>
          <KeyValueEditor
            :id="id"
            v-model="payload.variables"
            :key-placeholder="$t('repo.deploy_pipeline.variables.name')"
            :value-placeholder="$t('repo.deploy_pipeline.variables.value')"
            :delete-title="$t('repo.deploy_pipeline.variables.delete')"
            @update:is-valid="isVariablesValid = $event"
          />
        </InputField>
        <DialogFooter class="sm:justify-start">
          <Button type="submit" :disabled="!isFormValid || isSubmitting">
            {{ $t('repo.deploy_pipeline.trigger') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, toRef } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import InputField from '~/components/form/InputField.vue';
import KeyValueEditor from '~/components/form/KeyValueEditor.vue';
import { Button } from '~/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import { useLoadingOverlay } from '~/compositions/useLoadingOverlay';

const props = defineProps<{
  open: boolean;
  pipelineNumber: string;
}>();

const emit = defineEmits<{
  (event: 'close'): void;
}>();

const apiClient = useApiClient();
const overlay = useLoadingOverlay();
const i18n = useI18n();
const repo = requiredInject('repo');
const router = useRouter();

const payload = ref<{
  id: string;
  environment: string;
  task: string;
  variables: Record<string, string>;
}>({
  id: '',
  environment: '',
  task: '',
  variables: {},
});

const isVariablesValid = ref(true);
const isSubmitting = ref(false);

const isFormValid = computed(() => {
  return payload.value.environment !== '' && isVariablesValid.value;
});

const pipelineOptions = computed(() => ({
  ...payload.value,
  variables: payload.value.variables,
}));

const pipelineNumber = toRef(props, 'pipelineNumber');
async function triggerDeployPipeline() {
  isSubmitting.value = true;
  try {
    const newPipeline = await overlay.withOverlay(i18n.t('loading_overlay.deploying_pipeline'), () =>
      apiClient.deployPipeline(repo.value.id, pipelineNumber.value, pipelineOptions.value),
    );

    emit('close');

    await router.push({
      name: 'repo-pipeline',
      params: {
        pipelineId: newPipeline.number,
      },
    });
  } catch {
    emit('close');
  } finally {
    isSubmitting.value = false;
  }
}
</script>
