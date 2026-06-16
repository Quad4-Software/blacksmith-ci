<template>
  <Dialog v-model:open="isOpen">
    <DialogTrigger v-if="showTrigger" as-child>
      <slot name="trigger">
        <Button size="sm">
          <Icon name="plus" class="mr-2 h-4 w-4" />
          {{ $t('registries.add') }}
        </Button>
      </slot>
    </DialogTrigger>
    <DialogContent class="sm:max-w-[500px] max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{{ isEditing ? $t('registries.edit') : $t('registries.add') }}</DialogTitle>
        <DialogDescription>
          {{ isEditing ? $t('registries.edit_description') : $t('registries.add_description') }}
        </DialogDescription>
      </DialogHeader>

      <form id="registryForm" class="space-y-4" @submit.prevent="onSubmit">
        <!-- Address Field -->
        <div class="space-y-2">
          <Label for="registry-address">{{ $t('registries.address.address') }}</Label>
          <Input
            id="registry-address"
            v-model="formData.address"
            :placeholder="$t('registries.address.desc')"
            required
            :disabled="isEditing"
          />
        </div>

        <!-- Scope Selection (only when creating) -->
        <div v-if="!isEditing" class="space-y-4">
          <!-- Global Checkbox (Admin only) -->
          <div v-if="user?.admin" class="flex items-center gap-2">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger as-child>
                  <div class="flex items-center gap-2">
                    <Checkbox id="scope-global" :checked="isGlobal" @update:checked="(v) => (isGlobal = v === true)" />
                    <label for="scope-global" class="text-sm cursor-pointer">
                      {{ $t('scope.global') }}
                    </label>
                  </div>
                </TooltipTrigger>
                <TooltipContent class="max-w-lg">
                  <div class="space-y-1">
                    <p class="font-medium">{{ $t('admin.settings.registries.desc') }}</p>
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>

          <!-- Organization Selection -->
          <div class="space-y-2">
            <Label>{{ $t('registries.scope.select_target') }}</Label>
            <!-- Show disabled button when global is checked -->
            <Button v-if="isGlobal" variant="outline" disabled class="w-full justify-between">
              <span class="text-muted-foreground">{{ $t('registries.scope.global_selected') }}</span>
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
                  <span v-else class="text-muted-foreground">{{ $t('registries.scope.search_target') }}</span>
                  <Icon name="chevron-down" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-[400px] p-0" align="start">
                <Command v-model:search-term="searchTerm">
                  <CommandInput :placeholder="$t('registries.scope.search_target')" />
                  <CommandList>
                    <CommandEmpty>{{ $t('no_search_results') }}</CommandEmpty>
                    <!-- Organizations -->
                    <CommandGroup v-if="orgs.length > 0" :heading="$t('registries.scope.organizations')">
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
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </div>
        </div>

        <!-- Username Field -->
        <div class="space-y-2">
          <Label for="registry-username">{{ $t('username') }}</Label>
          <Input id="registry-username" v-model="formData.username" :placeholder="$t('username')" required />
        </div>

        <!-- Password Field -->
        <div class="space-y-2">
          <Label for="registry-password">{{ $t('password') }}</Label>
          <Input
            id="registry-password"
            v-model="formData.password"
            type="password"
            :placeholder="isEditing ? $t('registries.password_placeholder_edit') : $t('password')"
            :required="!isEditing"
          />
        </div>

        <div class="flex justify-end gap-2 pt-4">
          <Button type="button" variant="outline" @click="isOpen = false">
            {{ $t('cancel') }}
          </Button>
          <Button type="submit" :disabled="isSaving">
            <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
            {{ isEditing ? $t('registries.save') : $t('registries.add') }}
          </Button>
        </div>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '~/components/ui/command';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import { useForges } from '~/compositions/useForges';
import useNotifications from '~/compositions/useNotifications';
import type { IconNames } from '~/components/atomic/Icon.types';
import type { Org, Registry } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    registry?: Partial<Registry>;
    isSaving?: boolean;
    open?: boolean;
    hideTrigger?: boolean;
  }>(),
  {
    hideTrigger: false,
  },
);

const emit = defineEmits<{
  (event: 'save', value: Partial<Registry>): void;
  (event: 'cancel'): void;
  (event: 'update:open', value: boolean): void;
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const { user } = useConfig();
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
  if (forgeName) {
    const lowerName = forgeName.toLowerCase();
    if (lowerName.includes('codeberg')) return 'codeberg';
    if (lowerName.includes('codefloe')) return 'codefloe';
  }
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
const isEditing = computed(() => !!props.registry?.id);

// Form data
const formData = ref({
  address: '',
  username: '',
  password: '',
});

// Scope selection state
const isGlobal = ref(false);
const selectedTarget = ref<string>('');
const comboboxOpen = ref(false);
const searchTerm = ref('');

// Orgs for selection
const orgs = ref<Org[]>([]);

// Computed property to get the display name for selected target
const selectedTargetDisplay = computed(() => {
  if (!selectedTarget.value) return '';

  const [type, idStr] = selectedTarget.value.split('-');
  const id = Number.parseInt(idStr, 10);

  if (type === 'org') {
    const org = orgs.value.find((o) => o.id === id);
    return org ? org.name : '';
  }

  return '';
});

function clearSelection() {
  selectedTarget.value = '';
  searchTerm.value = '';
}

async function loadOrgs() {
  try {
    const orgsData = await apiClient.getUserOrgs({ silent: true });
    orgs.value = orgsData || [];
  } catch {
    // Silent failure - orgs will remain empty
  }
}

// Load data when dialog opens/closes
watch(isOpen, (open) => {
  if (open) {
    // Initialize form data from props
    formData.value = {
      address: props.registry?.address || '',
      username: props.registry?.username || '',
      password: '',
    };

    if (!isEditing.value) {
      loadOrgs();
      const registryWithScope = props.registry as any;
      isGlobal.value = registryWithScope?.scope_type === 'global';

      // Set selectedTarget based on scope_type
      if (registryWithScope?.scope_type === 'org' && registryWithScope?.org_id) {
        selectedTarget.value = `org-${registryWithScope.org_id}`;
      } else {
        selectedTarget.value = '';
      }
    }
  } else {
    // Reset state when dialog closes
    searchTerm.value = '';
    isGlobal.value = false;
    selectedTarget.value = '';
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

// Form submission
function onSubmit() {
  // Validate required fields
  if (!formData.value.address || !formData.value.username) {
    return;
  }

  if (!isEditing.value && !formData.value.password) {
    return;
  }

  // Validate scope selection - must select a target unless global
  if (!isEditing.value && !isGlobal.value && !selectedTarget.value) {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('registries.validation.target_required'),
      type: 'error',
    });
    return;
  }

  // Parse selectedTarget to get scope_type and org_id
  let scope_type: 'user' | 'org' | 'global' = 'user';
  let org_id: number | undefined;

  if (isGlobal.value) {
    scope_type = 'global';
  } else if (selectedTarget.value) {
    const [type, idStr] = selectedTarget.value.split('-');
    const id = Number.parseInt(idStr, 10);

    if (type === 'org') {
      scope_type = 'org';
      org_id = id;
    }
  }

  const registryData: Partial<Registry> = {
    ...props.registry,
    address: formData.value.address,
    username: formData.value.username,
    ...(formData.value.password ? { password: formData.value.password } : {}),
    ...(isEditing.value
      ? {}
      : {
          scope_type,
          org_id,
        }),
  };

  emit('save', registryData);
}
</script>
