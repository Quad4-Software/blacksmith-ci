<template>
  <Settings :title="$t('repo.settings.general.project')">
    <form class="flex flex-col gap-6" @submit.prevent="saveRepoSettings">
      <div class="grid gap-2">
        <label class="text-sm font-medium text-foreground">{{ $t('repo.settings.general.general') }}</label>

        <FormField v-slot="{ value, setValue }" name="allow_pr">
          <FormItem class="flex items-start gap-3 space-y-0">
            <FormControl>
              <Checkbox :checked="value" @update:checked="(v: boolean | 'indeterminate') => setValue(v === true)" />
            </FormControl>
            <div class="flex flex-col">
              <FormLabel class="font-normal">
                {{ $t('repo.settings.general.allow_pr.allow') }}
              </FormLabel>
              <FormDescription>{{ $t('repo.settings.general.allow_pr.desc') }}</FormDescription>
            </div>
          </FormItem>
        </FormField>

        <FormField v-slot="{ value, setValue }" name="allow_deploy">
          <FormItem class="flex items-start gap-3 space-y-0">
            <FormControl>
              <Checkbox :checked="value" @update:checked="(v) => setValue(v === true)" />
            </FormControl>
            <div class="flex flex-col">
              <FormLabel class="font-normal">
                {{ $t('repo.settings.general.allow_deploy.allow') }}
              </FormLabel>
              <FormDescription>{{ $t('repo.settings.general.allow_deploy.desc') }}</FormDescription>
            </div>
          </FormItem>
        </FormField>

        <FormField v-if="values.allow_deploy" v-slot="{ value, handleChange }" name="deploy_team">
          <FormItem class="ml-7">
            <FormLabel>{{ $t('repo.settings.general.deploy_team.team') }}</FormLabel>
            <FormControl>
              <Input
                :model-value="value"
                :placeholder="$t('repo.settings.general.deploy_team.placeholder')"
                @update:model-value="handleChange"
              />
            </FormControl>
            <FormDescription>{{ $t('repo.settings.general.deploy_team.desc') }}</FormDescription>
          </FormItem>
        </FormField>
      </div>
      <div class="grid gap-2">
        <label class="text-sm font-medium text-foreground">{{
          $t('repo.settings.general.netrc_only_trusted.netrc_only_trusted')
        }}</label>
        <div class="flex flex-col gap-2">
          <div v-for="image in values.netrc_trusted" :key="image" class="flex items-center gap-2">
            <Input :model-value="image" disabled />
            <Button type="button" size="icon" @click="removeImage(image)">
              <Icon name="trash" />
            </Button>
          </div>

          <div class="flex items-center gap-2">
            <Input v-model="newImage" @keydown.enter.prevent="addNewImage" />
            <Button type="button" size="icon" @click="addNewImage">
              <Icon name="plus" />
            </Button>
          </div>
        </div>
        <p class="text-sm text-muted-foreground">{{ $t('repo.settings.general.netrc_only_trusted.desc') }}</p>
      </div>
      <div v-if="user?.admin" class="grid gap-2">
        <label class="text-sm font-medium text-foreground">{{ $t('repo.settings.general.trusted.trusted') }}</label>

        <FormField v-slot="{ value, setValue }" name="trusted.network">
          <FormItem class="flex items-start gap-3 space-y-0">
            <FormControl>
              <Checkbox :checked="value" @update:checked="(v: boolean | 'indeterminate') => setValue(v === true)" />
            </FormControl>
            <div class="flex flex-col">
              <FormLabel class="font-normal">
                {{ $t('repo.settings.general.trusted.network.network') }}
              </FormLabel>
              <FormDescription>{{ $t('repo.settings.general.trusted.network.desc') }}</FormDescription>
            </div>
          </FormItem>
        </FormField>

        <FormField v-slot="{ value, setValue }" name="trusted.volumes">
          <FormItem class="flex items-start gap-3 space-y-0">
            <FormControl>
              <Checkbox :checked="value" @update:checked="(v: boolean | 'indeterminate') => setValue(v === true)" />
            </FormControl>
            <div class="flex flex-col">
              <FormLabel class="font-normal">
                {{ $t('repo.settings.general.trusted.volumes.volumes') }}
              </FormLabel>
              <FormDescription>{{ $t('repo.settings.general.trusted.volumes.desc') }}</FormDescription>
            </div>
          </FormItem>
        </FormField>

        <FormField v-slot="{ value, setValue }" name="trusted.security">
          <FormItem class="flex items-start gap-3 space-y-0">
            <FormControl>
              <Checkbox :checked="value" @update:checked="(v: boolean | 'indeterminate') => setValue(v === true)" />
            </FormControl>
            <div class="flex flex-col">
              <FormLabel class="font-normal">
                {{ $t('repo.settings.general.trusted.security.security') }}
              </FormLabel>
              <FormDescription>{{ $t('repo.settings.general.trusted.security.desc') }}</FormDescription>
            </div>
          </FormItem>
        </FormField>
      </div>
      <FormField v-slot="{ value, handleChange }" name="require_approval">
        <FormItem>
          <FormLabel>{{ $t('require_approval.require_approval_for') }}</FormLabel>
          <FormControl>
            <RadioGroup :model-value="value" @update:model-value="handleChange">
              <div class="flex items-start gap-3">
                <RadioGroupItem :value="RepoRequireApproval.None" />
                <div class="flex flex-col">
                  <FormLabel class="font-normal">{{ $t('require_approval.none') }}</FormLabel>
                  <FormDescription>{{ $t('require_approval.none_desc') }}</FormDescription>
                </div>
              </div>
              <div class="flex items-start gap-3">
                <RadioGroupItem :value="RepoRequireApproval.Forks" />
                <FormLabel class="font-normal">{{ $t('require_approval.forks') }}</FormLabel>
              </div>
              <div class="flex items-start gap-3">
                <RadioGroupItem :value="RepoRequireApproval.PullRequests" />
                <FormLabel class="font-normal">{{ $t('require_approval.pull_requests') }}</FormLabel>
              </div>
              <div class="flex items-start gap-3">
                <RadioGroupItem :value="RepoRequireApproval.AllEvents" />
                <FormLabel class="font-normal">{{ $t('require_approval.all_events') }}</FormLabel>
              </div>
            </RadioGroup>
          </FormControl>
          <FormDescription>{{ $t('require_approval.desc') }}</FormDescription>
        </FormItem>
      </FormField>
      <FormField v-slot="{ value, handleChange }" name="visibility">
        <FormItem>
          <FormLabel>{{ $t('repo.visibility.visibility') }}</FormLabel>
          <FormControl>
            <RadioGroup :model-value="value" @update:model-value="handleChange">
              <div v-for="option in projectVisibilityOptions" :key="option.value" class="flex items-start gap-3">
                <RadioGroupItem :value="option.value" />
                <div class="flex flex-col">
                  <FormLabel class="font-normal">{{ option.text }}</FormLabel>
                  <FormDescription v-if="option.description">{{ option.description }}</FormDescription>
                </div>
              </div>
            </RadioGroup>
          </FormControl>
        </FormItem>
      </FormField>
      <FormField v-slot="{ value, handleChange }" name="timeout">
        <FormItem>
          <FormLabel>{{ $t('repo.settings.general.timeout.timeout') }}</FormLabel>
          <FormControl>
            <div class="flex items-center">
              <NumberField :model-value="value" :min="0" class="max-w-[12rem]" @update:model-value="handleChange">
                <NumberFieldContent>
                  <NumberFieldDecrement />
                  <NumberFieldInput />
                  <NumberFieldIncrement />
                </NumberFieldContent>
              </NumberField>
              <span class="ml-4 text-muted-foreground">{{ $t('repo.settings.general.timeout.minutes') }}</span>
            </div>
          </FormControl>
        </FormItem>
      </FormField>
      <FormField v-slot="{ value, handleChange }" name="config_file">
        <FormItem>
          <FormLabel>{{ $t('repo.settings.general.pipeline_path.path') }}</FormLabel>
          <FormControl>
            <Input
              :model-value="value"
              :placeholder="$t('repo.settings.general.pipeline_path.default')"
              @update:model-value="handleChange"
            />
          </FormControl>
          <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
          <FormDescription>
            <i18n-t keypath="repo.settings.general.pipeline_path.desc">
              <span class="code-box-inline">{{ $t('repo.settings.general.pipeline_path.desc_path_example') }}</span>
              <span class="code-box-inline">/</span>
            </i18n-t>
          </FormDescription>
          <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
        </FormItem>
      </FormField>
      <div class="grid gap-2">
        <label class="text-sm font-medium text-foreground">{{ $t('repo.settings.general.cancel_prev.cancel') }}</label>
        <div v-for="option in cancelPreviousPipelineEventsOptions" :key="option.value" class="flex items-start gap-3">
          <Checkbox
            :checked="values.cancel_previous_pipeline_events?.includes(option.value)"
            @update:checked="(v) => toggleCancelEvent(option.value, v === true)"
          />
          <div class="flex flex-col">
            <label class="text-sm font-normal">{{ option.text }}</label>
            <p v-if="option.description" class="text-sm text-muted-foreground">{{ option.description }}</p>
          </div>
        </div>
        <p class="text-sm text-muted-foreground">{{ $t('repo.settings.general.cancel_prev.desc') }}</p>
      </div>
      <div class="grid gap-2">
        <label class="text-sm font-medium text-foreground">{{ $t('repo.settings.general.purge_logs.title') }}</label>
        <p class="text-sm text-muted-foreground">{{ $t('repo.settings.general.purge_logs.desc') }}</p>

        <FormField v-slot="{ value, handleChange }" name="logs_keep_duration">
          <FormItem>
            <FormControl>
              <div class="flex items-center">
                <Input :model-value="value" class="max-w-[6rem]" @update:model-value="handleChange" />
                <span class="ml-4 text-muted-foreground">{{ $t('repo.settings.general.purge_logs.duration') }}</span>
              </div>
            </FormControl>
          </FormItem>
        </FormField>

        <FormField v-slot="{ value, handleChange }" name="logs_keep_min">
          <FormItem>
            <FormControl>
              <div class="flex items-center">
                <NumberField :model-value="value" :min="0" class="max-w-[12rem]" @update:model-value="handleChange">
                  <NumberFieldContent>
                    <NumberFieldDecrement />
                    <NumberFieldInput />
                    <NumberFieldIncrement />
                  </NumberFieldContent>
                </NumberField>
                <span class="ml-4 text-muted-foreground">{{ $t('repo.settings.general.purge_logs.keep_min') }}</span>
              </div>
            </FormControl>
          </FormItem>
        </FormField>
      </div>
      <Button type="submit" class="mr-auto" :disabled="isSaving" @click.prevent="saveRepoSettings">
        <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
        {{ $t('repo.settings.general.save') }}
      </Button>
    </form>

    <!-- Actions Section -->
    <div class="border-t pt-6 pb-34">
      <h2 class="text-lg font-semibold mb-4">{{ $t('repo.settings.actions.actions') }}</h2>
      <ActionsButtons />
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { toTypedSchema } from '@vee-validate/zod';
import { useForm } from 'vee-validate';
import * as z from 'zod';

import { Checkbox } from '~/components/ui/checkbox';
import type { CheckboxOption, RadioOption } from '~/components/form/form.types';
import { Input } from '~/components/ui/input';
import { Button } from '~/components/ui/button';
import Icon from '~/components/atomic/Icon.vue';
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '~/components/ui/number-field';
import { RadioGroup, RadioGroupItem } from '~/components/ui/radio-group';
import { FormControl, FormDescription, FormField, FormItem, FormLabel } from '~/components/ui/form';
import Settings from '~/components/layout/Settings.vue';
import ActionsButtons from './ActionsButtons.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useAuthentication from '~/compositions/useAuthentication';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { RepoRequireApproval, RepoVisibility, WebhookEvents } from '~/lib/api/types';
import type { RepoSettings } from '~/lib/api/types';
import { useRepoStore } from '~/store/repos';

const apiClient = useApiClient();
const notifications = useNotifications();
const { user } = useAuthentication();
const repoStore = useRepoStore();
const i18n = useI18n();

const repo = requiredInject('repo');

// Define Zod schema for repo settings
const repoSettingsSchema = toTypedSchema(
  z.object({
    config_file: z.string(),
    timeout: z.number().min(0),
    visibility: z.nativeEnum(RepoVisibility),
    require_approval: z.nativeEnum(RepoRequireApproval),
    trusted: z.object({
      network: z.boolean(),
      volumes: z.boolean(),
      security: z.boolean(),
    }),
    allow_pr: z.boolean(),
    allow_deploy: z.boolean(),
    deploy_team: z.string(),
    cancel_previous_pipeline_events: z.array(z.string()),
    logs_keep_duration: z.string(),
    logs_keep_min: z.number().min(1),
    netrc_trusted: z.array(z.string()),
  }),
);

// Set up VeeValidate form
const { handleSubmit, setValues, values, setFieldValue } = useForm({
  validationSchema: repoSettingsSchema,
  initialValues: {
    config_file: '',
    timeout: 0,
    visibility: RepoVisibility.Public,
    require_approval: RepoRequireApproval.None,
    trusted: {
      network: false,
      volumes: false,
      security: false,
    },
    allow_pr: false,
    allow_deploy: false,
    deploy_team: '',
    cancel_previous_pipeline_events: [],
    logs_keep_duration: '',
    logs_keep_min: 90,
    netrc_trusted: [],
  },
});

function loadRepoSettings() {
  setValues({
    config_file: repo.value.config_file,
    timeout: repo.value.timeout,
    visibility: repo.value.visibility,
    require_approval: repo.value.require_approval,
    trusted: repo.value.trusted,
    allow_pr: repo.value.allow_pr,
    allow_deploy: repo.value.allow_deploy,
    deploy_team: repo.value.deploy_team || '',
    cancel_previous_pipeline_events: repo.value.cancel_previous_pipeline_events || [],
    logs_keep_duration: repo.value.logs_keep_duration ?? '',
    logs_keep_min: repo.value.logs_keep_min || 90,
    netrc_trusted: repo.value.netrc_trusted || [],
  });
}

async function loadRepo() {
  await repoStore.loadRepo(repo.value.id);
  loadRepoSettings();
}

const { doSubmit: saveRepoSettingsAction, isLoading: isSaving } = useAsyncAction(async () => {
  await apiClient.updateRepo(repo.value.id, values as RepoSettings);
  await loadRepo();
  notifications.notify({
    title: i18n.t('notify_title_success'),
    text: i18n.t('repo.settings.general.notify_text_updated'),
    type: 'success',
    duration: 6000,
  });
});

const saveRepoSettings = handleSubmit(() => {
  saveRepoSettingsAction();
});

onMounted(() => {
  loadRepoSettings();
});

const projectVisibilityOptions: RadioOption[] = [
  {
    value: RepoVisibility.Public,
    text: i18n.t('repo.visibility.public.public'),
    description: i18n.t('repo.visibility.public.desc'),
  },
  {
    value: RepoVisibility.Internal,
    text: i18n.t('repo.visibility.internal.internal'),
    description: i18n.t('repo.visibility.internal.desc'),
  },
  {
    value: RepoVisibility.Private,
    text: i18n.t('repo.visibility.private.private'),
    description: i18n.t('repo.visibility.private.desc'),
  },
];

const cancelPreviousPipelineEventsOptions: CheckboxOption[] = [
  { value: WebhookEvents.Push, text: i18n.t('repo.pipeline.event.push') },
  { value: WebhookEvents.Tag, text: i18n.t('repo.pipeline.event.tag') },
  {
    value: WebhookEvents.PullRequest,
    text: i18n.t('repo.pipeline.event.pr'),
  },
  { value: WebhookEvents.Deploy, text: i18n.t('repo.pipeline.event.deploy') },
];

const newImage = ref('');
function addNewImage() {
  if (!newImage.value) {
    return;
  }
  setFieldValue('netrc_trusted', [...(values.netrc_trusted || []), newImage.value]);
  newImage.value = '';
}
function removeImage(image: string) {
  setFieldValue(
    'netrc_trusted',
    (values.netrc_trusted || []).filter((i: string) => i !== image),
  );
}

function toggleCancelEvent(eventValue: string, checked: boolean) {
  if (checked) {
    if (!values.cancel_previous_pipeline_events?.includes(eventValue)) {
      setFieldValue('cancel_previous_pipeline_events', [...(values.cancel_previous_pipeline_events || []), eventValue]);
    }
  } else {
    setFieldValue(
      'cancel_previous_pipeline_events',
      values.cancel_previous_pipeline_events?.filter((e: string) => e !== eventValue) || [],
    );
  }
}

useCrowTitle(computed(() => [i18n.t('repo.settings.general.project'), repo.value.full_name]));
</script>
