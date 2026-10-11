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
          <div v-if="!isEditing && !lockScope" class="space-y-4">
            <!-- Global Checkbox (secrets:write only) -->
            <div v-if="can('secrets:write')" class="flex items-center gap-2">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <div class="flex items-center gap-2">
                      <Checkbox
                        id="scope-global"
                        :checked="isGlobal"
                        @update:checked="(v) => (isGlobal = v === true)"
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
                    <span v-else class="text-muted-foreground">{{ $t('secrets.scope.personal') }}</span>
                    <Icon name="chevron-down" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-[400px] p-0" align="start">
                  <Command v-model:search-term="searchTerm">
                    <CommandInput :placeholder="$t('secrets.scope.search_target')" />
                    <CommandList>
                      <CommandEmpty>{{ $t('no_search_results') }}</CommandEmpty>
                      <!-- Organizations -->
                      <CommandGroup v-if="orgs.length > 0" :heading="$t('secrets.scope.organizations')">
                        <!-- Column headers when multi-forge -->
                        <div
                          v-if="showForgeInfo"
                          class="grid grid-cols-2 gap-2 px-2 py-1 text-xs text-muted-foreground border-b"
                        >
                          <span>{{ $t('name') }}</span>
                          <span>{{ $t('forge') }}</span>
                        </div>
                        <CommandItem
                          v-for="org in orgs"
                          :key="`org-${org.id}`"
                          :value="
                            showForgeInfo && getOrgForgeInfo(org)
                              ? `${org.name} ${getOrgForgeInfo(org)?.hostname}`
                              : org.name
                          "
                          @select="
                            () => {
                              selectedTarget = `org-${org.id}`;
                              comboboxOpen = false;
                            }
                          "
                        >
                          <div v-if="showForgeInfo" class="grid grid-cols-2 gap-2 w-full items-center">
                            <span class="truncate">{{ org.name }}</span>
                            <span
                              v-if="getOrgForgeInfo(org)"
                              class="flex items-center gap-1 text-xs text-muted-foreground"
                            >
                              <img
                                v-if="getOrgForgeInfo(org)?.customUrl"
                                :src="getOrgForgeInfo(org)?.customUrl!"
                                class="h-3 w-3 shrink-0"
                                alt=""
                              />
                              <Icon v-else :name="getOrgForgeInfo(org)?.icon || 'repo'" class="h-3 w-3 shrink-0" />
                              <span class="truncate">{{ getOrgForgeInfo(org)?.hostname }}</span>
                            </span>
                          </div>
                          <span v-else>{{ org.name }}</span>
                        </CommandItem>
                      </CommandGroup>
                      <!-- Repositories -->
                      <CommandGroup v-if="repos.length > 0" :heading="$t('secrets.scope.repositories')">
                        <!-- Column headers when multi-forge -->
                        <div
                          v-if="showForgeInfo"
                          class="grid grid-cols-2 gap-2 px-2 py-1 text-xs text-muted-foreground border-b"
                        >
                          <span>{{ $t('scope.repo') }}</span>
                          <span>{{ $t('forge') }}</span>
                        </div>
                        <CommandItem
                          v-for="repo in repos"
                          :key="`repo-${repo.id}`"
                          :value="
                            showForgeInfo && getRepoForgeInfo(repo)
                              ? `${repo.owner}/${repo.name} ${getRepoForgeInfo(repo)?.hostname}`
                              : `${repo.owner}/${repo.name}`
                          "
                          @select="
                            () => {
                              selectedTarget = `repo-${repo.id}`;
                              comboboxOpen = false;
                            }
                          "
                        >
                          <div v-if="showForgeInfo" class="grid grid-cols-2 gap-2 w-full items-center">
                            <span class="truncate">{{ `${repo.owner}/${repo.name}` }}</span>
                            <span
                              v-if="getRepoForgeInfo(repo)"
                              class="flex items-center gap-1 text-xs text-muted-foreground"
                            >
                              <img
                                v-if="getRepoForgeInfo(repo)?.customUrl"
                                :src="getRepoForgeInfo(repo)?.customUrl!"
                                class="h-3 w-3 shrink-0"
                                alt=""
                              />
                              <Icon v-else :name="getRepoForgeInfo(repo)?.icon || 'repo'" class="h-3 w-3 shrink-0" />
                              <span class="truncate">{{ getRepoForgeInfo(repo)?.hostname }}</span>
                            </span>
                          </div>
                          <span v-else>{{ `${repo.owner}/${repo.name}` }}</span>
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
              <div class="flex items-center justify-between">
                <FormLabel>{{ $t('secrets.value') }}</FormLabel>
                <Button
                  v-if="isEditing && canViewAbbreviated"
                  type="button"
                  variant="ghost"
                  size="sm"
                  :disabled="isLoadingAbbreviatedValue"
                  @click="toggleAbbreviatedValue"
                >
                  <Icon v-if="isLoadingAbbreviatedValue" name="spinner" class="h-4 w-4 animate-spin mr-2" />
                  <Icon v-else :name="showAbbreviatedValue ? 'ph:eye-slash' : 'ph:eye'" class="h-4 w-4 mr-2" />
                  {{ showAbbreviatedValue ? $t('secrets.hide_value') : $t('secrets.show_value') }}
                </Button>
              </div>
              <FormControl>
                <div class="relative">
                  <Textarea
                    :placeholder="
                      isEditing
                        ? showAbbreviatedValue && abbreviatedValue
                          ? abbreviatedValue
                          : `${$t('secrets.value')} (leave empty to keep current value)`
                        : $t('secrets.value')
                    "
                    rows="5"
                    :name="componentField.name"
                    :model-value="componentField.modelValue"
                    :disabled="showAbbreviatedValue"
                    :class="{ 'bg-muted': showAbbreviatedValue }"
                    @blur="componentField.onBlur"
                    @input="componentField.onInput"
                    @change="componentField.onChange"
                    @update:model-value="componentField['onUpdate:modelValue']"
                  />
                  <div
                    v-if="showAbbreviatedValue && abbreviatedValue"
                    class="absolute inset-0 flex items-center justify-center pointer-events-none"
                  >
                    <code class="text-sm font-mono bg-muted/50 px-2 py-1 rounded">{{ abbreviatedValue }}</code>
                  </div>
                </div>
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
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <Button type="button" size="icon" variant="outline" @click="copyImage(image)">
                            <Icon :name="copiedImages[image] ? 'ph:check' : 'ph:copy'" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>{{ copiedImages[image] ? $t('copied') : $t('copy') }}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
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
                      <Icon name="ph:arrow-elbow-down-left" />
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

          <!-- Target Management (edit mode only) -->
          <SecretTargetManager
            v-if="isEditing && secret?.id"
            :secret-id="Number(secret.id)"
            :owner-name="getOwnerName()"
            :targets="secretTargets"
            :org-names="orgNameMap"
            :repo-names="repoNameMap"
            @add-target="handleAddTarget"
            @remove-target="handleRemoveTarget"
            @make-owner="handleMakeOwner"
          />

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
import { computed, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import * as z from 'zod';

import Icon from '~/components/atomic/Icon.vue';
import SecretTargetManager from './SecretTargetManager.vue';
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
import usePermissions from '~/compositions/usePermissions';
import { useForges } from '~/compositions/useForges';
import useNotifications from '~/compositions/useNotifications';
import { WebhookEvents } from '~/lib/api/types';
import type { CheckboxOption } from '~/components/form/form.types';
import type { IconNames } from '~/components/atomic/Icon.types';
import type { Org, Repo, Secret, SecretTarget } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    secret?: Partial<Secret>;
    isSaving?: boolean;
    open?: boolean;
    hideTrigger?: boolean;
    /**
     * Hide the scope picker and keep the scope of the passed secret. The repo,
     * org and admin settings views create secrets for exactly one scope.
     */
    lockScope?: boolean;
  }>(),
  {
    hideTrigger: false,
    lockScope: false,
  },
);

const emit = defineEmits<{
  (event: 'save', value: Partial<Secret>): void;
  (event: 'cancel'): void;
  (event: 'update:open', value: boolean): void;
}>();

const { t } = useI18n();
const apiClient = useApiClient();
// Global secrets and registries are managed with secrets:write.
const { can } = usePermissions();
const notifications = useNotifications();
const { forges, getForgeById } = useForges();

// Show forge info when multiple forges are configured
const showForgeInfo = computed(() => forges.value.length > 1);

// Helper to check if a string is a valid URL
function isValidUrl(str: string): boolean {
  try {
    return Boolean(new URL(str));
  } catch {
    return false;
  }
}

// Helper to get forge icon name from forge info
function getForgeIconName(forgeIcon?: string, forgeType?: string, forgeName?: string): IconNames {
  // Check for custom icon first
  if (forgeIcon && !isValidUrl(forgeIcon)) {
    const customIcon = forgeIcon.toLowerCase();
    const validIcons: Record<string, IconNames> = {
      codeberg: 'codeberg',
      codefloe: 'codefloe',
      github: 'github',
      gitlab: 'gitlab',
      gitea: 'gitea',
      forgejo: 'forgejo',
      bitbucket: 'bitbucket',
      repo: 'repo',
    };
    if (validIcons[customIcon]) {
      return validIcons[customIcon];
    }
  }

  // Check forgeName for special hosts
  if (forgeName) {
    const lowerName = forgeName.toLowerCase();
    if (lowerName.includes('codeberg')) return 'codeberg';
    if (lowerName.includes('codefloe')) return 'codefloe';
  }

  // Fall back to forge type
  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forgeType || ''] || 'repo';
}

// Helper to get custom icon URL if forge_icon is a URL
function getForgeCustomIconUrl(forgeIcon?: string): string | null {
  if (forgeIcon && isValidUrl(forgeIcon)) {
    return forgeIcon;
  }
  return null;
}

// Helper to get forge display info for an org
function getOrgForgeInfo(org: Org): { icon: IconNames; customUrl: string | null; hostname: string } | null {
  if (!showForgeInfo.value) return null;
  const forge = getForgeById(org.forge_id);
  if (!forge) return null;
  try {
    const url = new URL(forge.url);
    return {
      icon: getForgeIconName(forge.icon, forge.type, url.hostname),
      customUrl: getForgeCustomIconUrl(forge.icon),
      hostname: url.hostname,
    };
  } catch {
    return null;
  }
}

// Helper to get forge display info for a repo
function getRepoForgeInfo(repo: Repo): { icon: IconNames; customUrl: string | null; hostname: string } | null {
  if (!showForgeInfo.value) return null;
  const forge = getForgeById(repo.forge_id);
  if (!forge) return null;
  try {
    const url = new URL(forge.url);
    return {
      icon: getForgeIconName(forge.icon, forge.type, url.hostname),
      customUrl: getForgeCustomIconUrl(forge.icon),
      hostname: url.hostname,
    };
  } catch {
    return null;
  }
}

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

// Abbreviated value state
const showAbbreviatedValue = ref(false);
const abbreviatedValue = ref<string | null>(null);
const isLoadingAbbreviatedValue = ref(false);

// Permission check for viewing abbreviated values
// Users with write access can view abbreviated values
const canViewAbbreviated = computed(() => {
  if (!isEditing.value || !props.secret) return false;
  // The secret must exist (have an ID) to view its abbreviated value
  // Permission is checked server-side when fetching
  return true;
});
const newImage = ref('');
const isGlobal = ref(false);
const secretTargets = ref<SecretTarget[]>([]);
const orgNameMap = ref<Record<number, string>>({});
const repoNameMap = ref<Record<number, string>>({});

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
    const [reposData, orgsData] = await Promise.all([
      apiClient.getActiveRepos({ silent: true }),
      apiClient.getUserOrgs({ silent: true }),
    ]);
    repos.value = reposData || [];
    orgs.value = orgsData || [];
  } catch {
    // Silent failure - orgs and repos will remain empty
  }
}

// Load data when dialog opens/closes
watch(isOpen, (open) => {
  if (open && !isEditing.value && !props.lockScope) {
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
  } else if (open && isEditing.value) {
    loadTargets();
  } else if (!open) {
    // Reset state when dialog closes
    searchTerm.value = '';
    showAbbreviatedValue.value = false;
    abbreviatedValue.value = null;
    isLoadingAbbreviatedValue.value = false;
    secretTargets.value = [];
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

function removeImage(index: string | number, componentField: any) {
  const currentImages = componentField.modelValue || [];
  const idx = typeof index === 'string' ? Number.parseInt(index, 10) : index;
  const newImages = currentImages.filter((_: string, i: number) => i !== idx);
  componentField['onUpdate:modelValue'](newImages);
}

// Toggle abbreviated value visibility
async function toggleAbbreviatedValue() {
  if (showAbbreviatedValue.value) {
    // Hide the value
    showAbbreviatedValue.value = false;
    return;
  }

  // Show the value - need to fetch it first if not already loaded
  if (abbreviatedValue.value) {
    showAbbreviatedValue.value = true;
    return;
  }

  isLoadingAbbreviatedValue.value = true;
  try {
    const secret = props.secret as any;
    let fetchedSecret;

    // Determine which API to call based on secret scope
    if (secret.repo_id) {
      fetchedSecret = await apiClient.getSecretAbbreviated(secret.repo_id, secret.name);
    } else if (secret.org_id) {
      fetchedSecret = await apiClient.getOrgSecretAbbreviated(secret.org_id, secret.name);
    } else {
      // Global secret
      fetchedSecret = await apiClient.getGlobalSecretAbbreviated(secret.name);
    }

    abbreviatedValue.value = fetchedSecret.value;
    showAbbreviatedValue.value = true;
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('secrets.error_viewing_value'),
      type: 'error',
    });
  } finally {
    isLoadingAbbreviatedValue.value = false;
  }
}

async function loadTargets() {
  if (!props.secret?.id) return;
  try {
    secretTargets.value = (await apiClient.getSecretTargets(Number(props.secret.id), { silent: true })) || [];
  } catch {
    // Silent failure
  }
}

function getOwnerName(): string {
  const secret = props.secret as any;
  if (secret?.repo_name) return secret.repo_name;
  if (secret?.org_name) return secret.org_name;
  return 'Global';
}

async function handleAddTarget(type: 'org' | 'repo', id: number) {
  try {
    await apiClient.createSecretTarget(Number(props.secret!.id!), {
      org_id: type === 'org' ? id : 0,
      repo_id: type === 'repo' ? id : 0,
    });
    await loadTargets();
  } catch (err: any) {
    notifications.notify({ title: t('notify_title_error'), text: err.message, type: 'error' });
  }
}

async function handleRemoveTarget(target: SecretTarget) {
  try {
    await apiClient.deleteSecretTarget(Number(props.secret!.id!), target.id);
    await loadTargets();
  } catch (err: any) {
    notifications.notify({ title: t('notify_title_error'), text: err.message, type: 'error' });
  }
}

async function handleMakeOwner(target: SecretTarget) {
  try {
    await apiClient.transferSecretOwner(Number(props.secret!.id!), {
      org_id: target.org_id,
      repo_id: target.repo_id,
      keep_as_target: true,
    });
    await loadTargets();
  } catch (err: any) {
    notifications.notify({ title: t('notify_title_error'), text: err.message, type: 'error' });
  }
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
    // Add scope information (only when creating with a free choice of scope)
    ...(isEditing.value || props.lockScope
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

  // Note: If no target is selected and not global, the secret will be user-scoped (personal)

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
