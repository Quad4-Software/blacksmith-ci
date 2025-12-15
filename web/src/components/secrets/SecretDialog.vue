<template>
  <Dialog v-model:open="isOpen">
    <DialogTrigger v-if="showTrigger" as-child>
      <slot name="trigger">
        <Button size="sm">
          <Icon name="plus" class="mr-2 h-4 w-4" />
          {{ $t('secrets.add') }}
        </Button>
      </slot>
    </DialogTrigger>
    <DialogContent class="sm:max-w-[600px] max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{{ isEditing ? $t('secrets.edit') : $t('secrets.add') }}</DialogTitle>
        <DialogDescription>
          {{ isEditing ? $t('secrets.edit_description') : $t('secrets.add_description') }}
        </DialogDescription>
      </DialogHeader>

      <Form v-slot="{ handleSubmit }" as="" keep-values :validation-schema="formSchema" :initial-values="initialValues">
        <form
          id="secretForm"
          class="space-y-4"
          @submit.prevent="
            (e) => {
              handleSubmit(e, onSubmit);
            }
          "
        >
          <!-- Name Field -->
          <FormField v-slot="{ componentField }" name="name">
            <FormItem>
              <div class="flex items-center gap-2">
                <FormLabel>{{ $t('secrets.name') }}</FormLabel>
                <FormMessage class="text-xs" />
              </div>
              <FormControl>
                <Input
                  type="text"
                  :placeholder="$t('secrets.name')"
                  :disabled="isEditing"
                  :name="componentField.name"
                  :model-value="componentField.modelValue"
                  @blur="componentField.onBlur"
                  @input="componentField.onInput"
                  @change="componentField.onChange"
                  @update:model-value="componentField['onUpdate:modelValue']"
                />
              </FormControl>
            </FormItem>
          </FormField>

          <!-- Scope Selection -->
          <div v-if="!isEditing" class="space-y-4">
            <!-- Global Checkbox (Admin only) -->
            <div v-if="user?.admin" class="flex items-center gap-2">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <div class="flex items-center gap-2">
                      <Checkbox
                        id="scope-global"
                        :checked="isGlobal"
                        @update:checked="isGlobal = $event"
                        @click="isGlobal = !isGlobal"
                      />
                      <label for="scope-global" class="text-sm cursor-pointer">
                        {{ $t('scope.global') }}
                      </label>
                    </div>
                  </TooltipTrigger>
                  <TooltipContent class="max-w-lg">
                    <div class="space-y-1">
                      <p class="font-medium">{{ $t('admin.settings.secrets.desc') }}</p>
                      <!-- <p class="text-xs text-muted-foreground">{{ $t('admin.settings.secrets.warning') }}</p> -->
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>

            <!-- Organization/Repository Selection -->
            <div class="space-y-2">
              <label class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                {{ $t('secrets.scope.select_target') }}
              </label>
              <!-- Show disabled button when global is checked -->
              <Button v-if="isGlobal" variant="outline" disabled class="w-full justify-between">
                <span class="text-muted-foreground">{{ $t('secrets.scope.global_selected') }}</span>
                <Icon name="chevron-down" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
              <!-- Show interactive popover when global is not checked -->
              <Popover v-else v-model:open="comboboxOpen">
                <PopoverTrigger as-child>
                  <Button variant="outline" role="combobox" class="w-full justify-between">
                    <span v-if="selectedTargetDisplay" class="flex items-center gap-2 flex-1 justify-between">
                      <span>{{ selectedTargetDisplay }}</span>
                      <button
                        type="button"
                        class="h-4 w-4 shrink-0 opacity-50 hover:opacity-100 flex items-center justify-center"
                        @click.stop.prevent="clearSelection"
                      >
                        <Icon name="close" class="h-4 w-4" />
                      </button>
                    </span>
                    <span v-else class="text-muted-foreground">{{ $t('secrets.scope.search_target') }}</span>
                    <Icon name="chevron-down" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-[200px] p-0" align="start">
                  <Command v-model:search-term="searchTerm">
                    <CommandInput :placeholder="$t('secrets.scope.search_target')" />
                    <CommandList>
                      <CommandEmpty>{{ $t('no_search_results') }}</CommandEmpty>
                      <!-- Organizations -->
                      <CommandGroup v-if="orgs.length > 0" :heading="$t('secrets.scope.organizations')">
                        <CommandItem
                          v-for="org in orgs"
                          :key="`org-${org.id}`"
                          :value="org.name"
                          @select="
                            () => {
                              selectedTarget = `org-${org.id}`;
                              comboboxOpen = false;
                            }
                          "
                        >
                          {{ org.name }}
                        </CommandItem>
                      </CommandGroup>
                      <!-- Repositories -->
                      <CommandGroup v-if="repos.length > 0" :heading="$t('secrets.scope.repositories')">
                        <CommandItem
                          v-for="repo in repos"
                          :key="`repo-${repo.id}`"
                          :value="`${repo.owner}/${repo.name}`"
                          @select="
                            () => {
                              selectedTarget = `repo-${repo.id}`;
                              comboboxOpen = false;
                            }
                          "
                        >
                          {{ `${repo.owner}/${repo.name}` }}
                        </CommandItem>
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            </div>
          </div>

          <!-- Value Field -->
          <FormField v-slot="{ componentField }" name="value">
            <FormItem>
              <FormLabel>{{ $t('secrets.value') }}</FormLabel>
              <FormControl>
                <Textarea
                  :placeholder="
                    isEditing ? `${$t('secrets.value')} (leave empty to keep current value)` : $t('secrets.value')
                  "
                  rows="5"
                  :name="componentField.name"
                  :model-value="componentField.modelValue"
                  @blur="componentField.onBlur"
                  @input="componentField.onInput"
                  @change="componentField.onChange"
                  @update:model-value="componentField['onUpdate:modelValue']"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <!-- Images Field -->
          <FormField v-slot="{ componentField, value: imagesValue }" name="images">
            <FormItem>
              <FormLabel>{{ $t('secrets.plugins.images') }}</FormLabel>
              <FormDescription>{{ $t('secrets.plugins.desc') }}</FormDescription>
              <FormControl>
                <div class="flex flex-col gap-2">
                  <!-- Existing images -->
                  <div v-for="(image, index) in imagesValue" :key="index" class="flex gap-2">
                    <Input :model-value="image" disabled />
                    <Button type="button" size="icon" @click="removeImage(index, componentField)">
                      <Icon name="trash" />
                    </Button>
                  </div>

                  <!-- Add new image -->
                  <div class="flex gap-2">
                    <Input
                      v-model="newImage"
                      :placeholder="$t('secrets.plugins.add_image')"
                      @keydown.enter.prevent="addImage(componentField)"
                    />
                    <Button type="button" size="icon" @click="addImage(componentField)">
                      <Icon name="plus" />
                    </Button>
                  </div>
                </div>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <!-- Events Field -->
          <FormField v-slot="{ componentField }" name="events">
            <FormItem>
              <FormLabel>{{ $t('secrets.events.events') }}</FormLabel>
              <Alert variant="destructive" class="mb-4">
                <Icon name="alert" class="h-4 w-4" />
                <AlertDescription>
                  {{ $t('secrets.events.warning') }}
                </AlertDescription>
              </Alert>
              <FormControl>
                <div class="flex flex-col gap-2">
                  <div v-for="option in secretEventsOptions" :key="option.value" class="flex items-start gap-3">
                    <Checkbox
                      :id="`event-${option.value}`"
                      :checked="(componentField.modelValue || []).includes(option.value)"
                      @update:checked="
                        (checked: boolean | 'indeterminate' | undefined) =>
                          toggleEventCheckbox(option.value, checked, componentField)
                      "
                    />
                    <label :for="`event-${option.value}`" class="text-sm font-normal leading-none cursor-pointer">
                      {{ option.text }}
                    </label>
                  </div>
                </div>
              </FormControl>
              <FormMessage />
            </FormItem>
          </FormField>

          <div class="flex justify-end gap-2 pt-4">
            <Button type="button" variant="outline" @click="isOpen = false">
              {{ $t('cancel') }}
            </Button>
            <Button type="submit" :disabled="isSaving">
              <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
              {{ isEditing ? $t('secrets.save') : $t('secrets.add') }}
            </Button>
          </div>
        </form>
      </Form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { toTypedSchema } from '@vee-validate/zod';
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import * as z from 'zod';

import Icon from '~/components/atomic/Icon.vue';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '~/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '~/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { Textarea } from '~/components/ui/textarea';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import useNotifications from '~/compositions/useNotifications';
import { WebhookEvents } from '~/lib/api/types';
import type { CheckboxOption } from '~/components/form/form.types';
import type { Org, Repo, Secret } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    secret?: Partial<Secret>;
    isSaving?: boolean;
    open?: boolean;
    hideTrigger?: boolean;
  }>(),
  {
    hideTrigger: false,
  },
);

const emit = defineEmits<{
  (event: 'save', value: Partial<Secret>): void;
  (event: 'cancel'): void;
  (event: 'update:open', value: boolean): void;
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const { user } = useConfig();
const notifications = useNotifications();

const isOpen = computed({
  get: () => props.open ?? false,
  set: (value) => {
    emit('update:open', value);
    if (!value) {
      emit('cancel');
    }
  },
});

const showTrigger = computed(() => !props.hideTrigger);
const isEditing = computed(() => !!props.secret?.id);
const newImage = ref('');
const isGlobal = ref(false);
const selectedTarget = ref<string>(''); // Format: "org-123" or "repo-456"
const comboboxOpen = ref(false);
const searchTerm = ref('');

// Fetch repos and orgs
const repos = ref<Repo[]>([]);
const orgs = ref<Org[]>([]);

// Computed property to get the display name for selected target
const selectedTargetDisplay = computed(() => {
  if (!selectedTarget.value) return '';

  const [type, idStr] = selectedTarget.value.split('-');
  const id = Number.parseInt(idStr, 10);

  if (type === 'org') {
    const org = orgs.value.find((o) => o.id === id);
    return org ? org.name : '';
  } else if (type === 'repo') {
    const repo = repos.value.find((r) => r.id === id);
    return repo ? `${repo.owner}/${repo.name}` : '';
  }

  return '';
});

function clearSelection() {
  selectedTarget.value = '';
  searchTerm.value = '';
}

async function loadReposAndOrgs() {
  try {
    const [reposData, orgsData] = await Promise.all([apiClient.getRepoList(), apiClient.getOrgs()]);
    repos.value = reposData || [];
    orgs.value = orgsData || [];
  } catch {
    // Silent failure - orgs and repos will remain empty
  }
}

// Load data when dialog opens/closes
watch(isOpen, (open) => {
  if (open && !isEditing.value) {
    loadReposAndOrgs();
    const secretWithScope = props.secret as any;
    isGlobal.value = secretWithScope?.scope_type === 'global';

    // Set selectedTarget based on scope_type
    if (secretWithScope?.scope_type === 'org' && secretWithScope?.org_id) {
      selectedTarget.value = `org-${secretWithScope.org_id}`;
    } else if (secretWithScope?.scope_type === 'repo' && secretWithScope?.repo_id) {
      selectedTarget.value = `repo-${secretWithScope.repo_id}`;
    } else {
      selectedTarget.value = '';
    }
  } else if (!open) {
    // Reset search term when dialog closes
    searchTerm.value = '';
  }
});

// Clear selected target and close popover when global is checked
watch(isGlobal, (global) => {
  if (global) {
    selectedTarget.value = '';
    searchTerm.value = '';
    comboboxOpen.value = false;
  }
});

// Initial form values
const initialValues = computed(() => ({
  name: props.secret?.name || '',
  value: props.secret?.value || '',
  images: props.secret?.images || [],
  events: props.secret?.events || [WebhookEvents.Push],
}));

// Form schema with zod
const formSchema = toTypedSchema(
  z.object({
    name: z.string().min(1, t('secrets.validation.name_required')),
    value: z.string().optional(),
    images: z.array(z.string()),
    events: z.array(z.nativeEnum(WebhookEvents)).min(1, t('secrets.validation.events_required')),
  }),
);

// Webhook events options - computed for reactivity
const secretEventsOptions = computed<CheckboxOption[]>(() => [
  { value: WebhookEvents.Push, text: t('repo.pipeline.event.push') },
  { value: WebhookEvents.Tag, text: t('repo.pipeline.event.tag') },
  { value: WebhookEvents.Release, text: t('repo.pipeline.event.release') },
  { value: WebhookEvents.PullRequest, text: t('repo.pipeline.event.pr') },
  { value: WebhookEvents.Deploy, text: t('repo.pipeline.event.deploy') },
  { value: WebhookEvents.Cron, text: t('repo.pipeline.event.cron') },
  { value: WebhookEvents.Manual, text: t('repo.pipeline.event.manual') },
]);

// Image management
function addImage(componentField: any) {
  if (!newImage.value) return;

  const currentImages = componentField.modelValue || [];
  componentField['onUpdate:modelValue']([...currentImages, newImage.value]);
  newImage.value = '';
}

function removeImage(index: number, componentField: any) {
  const currentImages = componentField.modelValue || [];
  const newImages = currentImages.filter((_: string, i: number) => i !== index);
  componentField['onUpdate:modelValue'](newImages);
}

// Event checkbox management
function toggleEventCheckbox(eventValue: string, checked: boolean | 'indeterminate' | undefined, componentField: any) {
  const currentEvents = componentField.modelValue || [];
  let newEvents: string[];

  if (checked === true) {
    if (!currentEvents.includes(eventValue)) {
      newEvents = [...currentEvents, eventValue];
    } else {
      newEvents = currentEvents;
    }
  } else {
    newEvents = currentEvents.filter((e: string) => e !== eventValue);
  }

  componentField['onUpdate:modelValue'](newEvents);
}

// Form submission
function onSubmit(values: any) {
  // Parse selectedTarget to get scope_type, org_id, and repo_id
  let scope_type: 'user' | 'org' | 'repo' | 'global' = 'user';
  let org_id: number | undefined;
  let repo_id: number | undefined;

  if (isGlobal.value) {
    scope_type = 'global';
  } else if (selectedTarget.value) {
    const [type, idStr] = selectedTarget.value.split('-');
    const id = Number.parseInt(idStr, 10);

    if (type === 'org') {
      scope_type = 'org';
      org_id = id;
    } else if (type === 'repo') {
      scope_type = 'repo';
      repo_id = id;
    }
  }

  const secretData: Partial<Secret> = {
    // Start with all properties from the original secret (scope_type, org_id, repo_id, etc.)
    ...props.secret,
    // Override with form values
    name: values.name,
    value: values.value || props.secret?.value || '',
    images: values.images || [],
    events: values.events || [],
    source: props.secret?.source || 'internal',
    // Add scope information (only when creating, not editing)
    ...(isEditing.value
      ? {}
      : {
          scope_type,
          org_id,
          repo_id,
        }),
  };

  // Validation
  if (!secretData.name) {
    return;
  }

  // Validate scope selection - must select a target unless global
  if (!isEditing.value && !isGlobal.value && !selectedTarget.value) {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('secrets.validation.target_required'),
      type: 'error',
    });
    return;
  }

  emit('save', secretData);
}

// Watch for prop changes to reset form
watch(
  () => props.secret,
  () => {
    newImage.value = '';
  },
  { deep: true },
);
</script>
