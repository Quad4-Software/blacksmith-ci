<template>
  <div v-if="innerValue">
    <form @submit.prevent="save">
      <FieldGroup>
        <Field>
          <FieldLabel for="secret-name">{{ $t('secrets.name') }}</FieldLabel>
          <Input
            id="secret-name"
            v-model="innerValue.name"
            :placeholder="$t('secrets.name')"
            required
            :disabled="isEditingSecret"
          />
        </Field>

        <Field>
          <div class="flex items-center justify-between">
            <FieldLabel for="secret-value">{{ $t('secrets.value') }}</FieldLabel>
            <Button
              v-if="isEditingSecret"
              type="button"
              variant="ghost"
              size="sm"
              :disabled="isLoadingAbbreviatedValue"
              @click="toggleAbbreviatedValue"
            >
              <Icon v-if="isLoadingAbbreviatedValue" name="spinner" class="h-4 w-4 animate-spin mr-2" />
              <Icon v-else :name="showAbbreviatedValue ? 'lucide:eye-off' : 'lucide:eye'" class="h-4 w-4 mr-2" />
              {{ showAbbreviatedValue ? $t('secrets.hide_value') : $t('secrets.show_value') }}
            </Button>
          </div>
          <div class="relative">
            <Textarea
              id="secret-value"
              v-model="innerValue.value"
              :placeholder="
                isEditingSecret
                  ? showAbbreviatedValue && abbreviatedValue
                    ? abbreviatedValue
                    : `${$t('secrets.value')} (leave empty to keep current value)`
                  : $t('secrets.value')
              "
              rows="5"
              :required="!isEditingSecret"
              :disabled="showAbbreviatedValue"
              :class="{ 'bg-muted': showAbbreviatedValue }"
            />
            <div
              v-if="showAbbreviatedValue && abbreviatedValue"
              class="absolute inset-0 flex items-center justify-center pointer-events-none"
            >
              <code class="text-sm font-mono bg-muted/50 px-2 py-1 rounded">{{ abbreviatedValue }}</code>
            </div>
          </div>
        </Field>

        <Field>
          <FieldLabel for="secret-images">{{ $t('secrets.plugins.images') }}</FieldLabel>
          <FieldDescription>{{ $t('secrets.plugins.desc') }}</FieldDescription>
          <div class="flex flex-col gap-2">
            <div v-for="image in innerValue.images" :key="image" class="flex gap-2">
              <Input :model-value="image" disabled />
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button type="button" size="icon" variant="outline" @click="copyImage(image)">
                      <Icon :name="copiedImages[image] ? 'lucide:check' : 'lucide:copy'" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{{ copiedImages[image] ? $t('copied') : $t('copy') }}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
              <Button type="button" size="icon" @click="removeImage(image)">
                <Icon name="trash" />
              </Button>
            </div>

            <div class="flex gap-2">
              <Input id="secret-images" v-model="newImage" @keydown.enter.prevent="addNewImage" />
              <Button type="button" size="icon" @click="addNewImage">
                <Icon name="plus" />
              </Button>
            </div>
          </div>
        </Field>

        <Field>
          <FieldLabel>{{ $t('secrets.events.events') }}</FieldLabel>
          <Alert variant="destructive" class="mb-4">
            <Icon name="alert" class="h-4 w-4" />
            <AlertDescription>
              {{ $t('secrets.events.warning') }}
            </AlertDescription>
          </Alert>
          <div class="flex flex-col gap-2">
            <div v-for="option in secretEventsOptions" :key="option.value" class="flex items-start gap-3">
              <Checkbox
                :id="`event-${option.value}`"
                :checked="innerValue.events?.includes(option.value as WebhookEvents)"
                @update:checked="
                  (checked: boolean | 'indeterminate') => toggleEvent(option.value as WebhookEvents, checked === true)
                "
              />
              <label :for="`event-${option.value}`" class="text-sm font-normal leading-none cursor-pointer">{{
                option.text
              }}</label>
            </div>
          </div>
        </Field>
      </FieldGroup>

      <div class="mt-6 flex gap-2">
        <Button type="button" variant="outline" @click="$emit('cancel')">
          {{ $t('cancel') }}
        </Button>
        <Button type="submit" :disabled="isSaving">
          <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
          {{ isEditingSecret ? $t('secrets.save') : $t('secrets.add') }}
        </Button>
      </div>
    </form>
  </div>
</template>

<script lang="ts" setup>
import { computed, reactive, ref, toRef } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import type { CheckboxOption } from '~/components/form/form.types';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Input } from '~/components/ui/input';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import { Textarea } from '~/components/ui/textarea';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import { WebhookEvents } from '~/lib/api/types';
import type { Secret } from '~/lib/api/types';

const props = defineProps<{
  modelValue: Partial<Secret>;
  isSaving: boolean;
}>();

const emit = defineEmits<{
  (event: 'update:modelValue', value: Partial<Secret> | undefined): void;
  (event: 'save', value: Partial<Secret>): void;
  (event: 'cancel'): void;
}>();

const i18n = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();

const modelValue = toRef(props, 'modelValue');
const innerValue = computed({
  get: () => modelValue.value,
  set: (value) => {
    emit('update:modelValue', value);
  },
});
const isEditingSecret = computed(() => !!innerValue.value?.id);

// Abbreviated value state
const showAbbreviatedValue = ref(false);
const abbreviatedValue = ref<string | null>(null);
const isLoadingAbbreviatedValue = ref(false);

async function toggleAbbreviatedValue() {
  if (showAbbreviatedValue.value) {
    showAbbreviatedValue.value = false;
    return;
  }

  if (abbreviatedValue.value) {
    showAbbreviatedValue.value = true;
    return;
  }

  isLoadingAbbreviatedValue.value = true;
  try {
    const secret = innerValue.value;
    let fetchedSecret;

    // Determine which API to call based on secret scope
    if (secret.repo_id && secret.repo_id !== 0) {
      fetchedSecret = await apiClient.getSecretAbbreviated(secret.repo_id, secret.name!);
    } else if (secret.org_id && secret.org_id !== 0) {
      fetchedSecret = await apiClient.getOrgSecretAbbreviated(secret.org_id, secret.name!);
    } else {
      fetchedSecret = await apiClient.getGlobalSecretAbbreviated(secret.name!);
    }

    abbreviatedValue.value = fetchedSecret.value;
    showAbbreviatedValue.value = true;
  } catch {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      text: i18n.t('secrets.error_viewing_value'),
      type: 'error',
    });
  } finally {
    isLoadingAbbreviatedValue.value = false;
  }
}

const newImage = ref('');

// Track copied state for images
const copiedImages = reactive<Record<string, boolean>>({});

async function copyImage(image: string) {
  try {
    await navigator.clipboard.writeText(image);
    copiedImages[image] = true;
    setTimeout(() => {
      copiedImages[image] = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

function addNewImage() {
  if (!newImage.value) {
    return;
  }
  const currentImages = innerValue.value.images ?? [];
  innerValue.value = { ...innerValue.value, images: [...currentImages, newImage.value] };
  newImage.value = '';
}
function removeImage(image: string) {
  const currentImages = innerValue.value.images ?? [];
  innerValue.value = { ...innerValue.value, images: currentImages.filter((i) => i !== image) };
}

const secretEventsOptions: CheckboxOption[] = [
  { value: WebhookEvents.Push, text: i18n.t('repo.pipeline.event.push') },
  { value: WebhookEvents.Tag, text: i18n.t('repo.pipeline.event.tag') },
  { value: WebhookEvents.Release, text: i18n.t('repo.pipeline.event.release') },
  { value: WebhookEvents.PullRequest, text: i18n.t('repo.pipeline.event.pr') },
  { value: WebhookEvents.Deploy, text: i18n.t('repo.pipeline.event.deploy') },
  { value: WebhookEvents.Cron, text: i18n.t('repo.pipeline.event.cron') },
  { value: WebhookEvents.Manual, text: i18n.t('repo.pipeline.event.manual') },
];

function toggleEvent(eventValue: WebhookEvents, checked: boolean) {
  const currentEvents = innerValue.value.events ?? [];

  let newEvents: WebhookEvents[];
  if (checked) {
    if (!currentEvents.includes(eventValue)) {
      newEvents = [...currentEvents, eventValue];
    } else {
      newEvents = currentEvents;
    }
  } else {
    newEvents = currentEvents.filter((e) => e !== eventValue);
  }

  // Emit updated object to trigger proper v-model reactivity
  innerValue.value = { ...innerValue.value, events: newEvents };
}

function save() {
  if (!innerValue.value) {
    return;
  }

  if (newImage.value) {
    innerValue.value.images?.push(newImage.value);
  }

  emit('save', innerValue.value);
}
</script>
