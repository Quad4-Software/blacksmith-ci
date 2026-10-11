<template>
  <Dialog v-model:open="isOpen">
    <DialogContent class="sm:max-w-[600px] max-h-[90vh] overflow-y-auto">
      <!-- Token Created View -->
      <template v-if="createdAgent">
        <DialogHeader>
          <DialogTitle>{{ $t('admin.settings.agents.token_created_title') }}</DialogTitle>
          <DialogDescription>
            {{ $t('admin.settings.agents.token_created_desc') }}
          </DialogDescription>
        </DialogHeader>

        <Alert class="mt-4 border-amber-500/50 text-amber-700 dark:text-amber-400 [&>svg]:text-amber-500">
          <Icon name="alert" class="h-4 w-4" />
          <AlertDescription>
            {{ $t('admin.settings.agents.token_copy_warning') }}
          </AlertDescription>
        </Alert>

        <div class="mt-4 space-y-2">
          <label class="text-sm font-medium">{{ $t('admin.settings.agents.token') }}</label>
          <div class="flex gap-2">
            <Input :model-value="createdAgent.token" readonly class="font-mono text-xs" />
            <Button variant="outline" size="icon" :title="$t('copy')" @click="copyCreatedToken">
              <Icon :name="copiedCreated ? 'ph:check' : 'ph:copy'" class="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div class="mt-4 space-y-2">
          <label class="text-sm font-medium">{{ $t('admin.settings.agents.name.name') }}</label>
          <Input :model-value="createdAgent.name" readonly />
        </div>

        <div class="mt-4 space-y-2">
          <label class="text-sm font-medium">{{ $t('admin.settings.agents.id') }}</label>
          <Input :model-value="createdAgent.id?.toString()" readonly />
        </div>

        <div class="flex justify-end gap-2 pt-4">
          <Button @click="handleClose">
            {{ $t('done') }}
          </Button>
        </div>
      </template>

      <!-- Create/Edit Form View -->
      <template v-else>
        <DialogHeader>
          <DialogTitle>{{
            isEditing ? $t('admin.settings.agents.edit_agent') : $t('admin.settings.agents.add')
          }}</DialogTitle>
          <DialogDescription>
            {{ isEditing ? $t('admin.settings.agents.edit_desc') : $t('admin.settings.agents.add_desc') }}
          </DialogDescription>
        </DialogHeader>

        <form class="space-y-4 mt-4" @submit.prevent="handleSave">
          <div class="space-y-2">
            <label class="text-sm font-medium">{{ $t('admin.settings.agents.name.name') }}</label>
            <Input v-model="form.name" :placeholder="$t('admin.settings.agents.name.placeholder')" required />
          </div>

          <div class="flex items-center gap-3">
            <Checkbox
              id="no-schedule"
              :checked="form.no_schedule || false"
              @update:checked="form.no_schedule = $event === true"
            />
            <div class="grid gap-0.5 leading-none">
              <label for="no-schedule" class="text-sm font-medium cursor-pointer">
                {{ $t('admin.settings.agents.no_schedule.name') }}
              </label>
              <p class="text-xs text-muted-foreground">
                {{ $t('admin.settings.agents.no_schedule.placeholder') }}
              </p>
            </div>
          </div>

          <div class="space-y-2">
            <label class="text-sm font-medium">{{ $t('admin.settings.agents.priority.priority') }}</label>
            <p class="text-xs text-muted-foreground">{{ $t('admin.settings.agents.priority.desc') }}</p>
            <Input v-model.number="form.priority" type="number" min="0" max="2147483647" />
          </div>

          <div v-if="scopeEditable" class="space-y-2">
            <label class="text-sm font-medium">{{ $t('admin.settings.agents.scope.label') }}</label>
            <select v-model="scopeMode" class="w-full rounded-md border bg-background px-3 py-2 text-sm">
              <option value="global">{{ $t('admin.settings.agents.scope.global') }}</option>
              <option value="orgs">{{ $t('admin.settings.agents.scope.specific_orgs') }}</option>
            </select>
            <p v-if="scopeMode === 'global'" class="text-xs text-muted-foreground">
              {{ $t('admin.settings.agents.scope.global_desc') }}
            </p>
            <div v-else class="max-h-40 overflow-y-auto rounded-md border p-2 space-y-2">
              <p v-if="orgs.length === 0" class="text-xs text-muted-foreground">
                {{ $t('admin.settings.agents.scope.no_orgs') }}
              </p>
              <div v-for="org in orgs" :key="org.id" class="flex items-center gap-2">
                <Checkbox
                  :id="`scope-org-${org.id}`"
                  :checked="selectedOrgIds.includes(org.id)"
                  @update:checked="toggleOrg(org.id, $event === true)"
                />
                <label :for="`scope-org-${org.id}`" class="text-sm cursor-pointer">{{ org.name }}</label>
              </div>
            </div>
          </div>

          <!-- Read-only fields when editing -->
          <template v-if="isEditing && agent">
            <div class="space-y-2">
              <label class="text-sm font-medium">{{ $t('admin.settings.agents.id') }}</label>
              <Input :model-value="agent.id?.toString()" disabled />
            </div>

            <div v-if="agent.backend" class="space-y-2">
              <label class="text-sm font-medium">{{ $t('admin.settings.agents.backend.backend') }}</label>
              <Input :model-value="agent.backend" disabled />
            </div>

            <div v-if="agent.platform" class="space-y-2">
              <label class="text-sm font-medium">{{ $t('admin.settings.agents.platform.platform') }}</label>
              <Input :model-value="agent.platform" disabled />
            </div>

            <div v-if="agent.version" class="space-y-2">
              <label class="text-sm font-medium">{{ $t('admin.settings.agents.version.label') }}</label>
              <Input :model-value="agent.version" disabled />
            </div>

            <div class="space-y-2">
              <label class="text-sm font-medium">{{ $t('admin.settings.agents.last_contact.last_contact') }}</label>
              <Input
                :model-value="
                  agent.last_contact ? date.timeAgo(agent.last_contact * 1000) : $t('admin.settings.agents.never')
                "
                disabled
              />
            </div>
          </template>

          <div class="flex justify-end gap-2 pt-4">
            <Button type="button" variant="outline" @click="handleCancel">
              {{ $t('cancel') }}
            </Button>
            <Button type="submit" :disabled="isSaving || !form.name">
              <Icon v-if="isSaving" name="loading" class="size-4 animate-spin mr-2" />
              {{ isEditing ? $t('admin.settings.agents.save') : $t('admin.settings.agents.add') }}
            </Button>
          </div>
        </form>
      </template>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import useApiClient from '~/compositions/useApiClient';
import { useDate } from '~/compositions/useDate';
import type { Agent, AgentWithToken, Org } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    open?: boolean;
    agent?: Agent | null;
    isSaving?: boolean;
    isAdmin?: boolean;
    scopeEditable?: boolean;
  }>(),
  {
    open: false,
    agent: null,
    isSaving: false,
    isAdmin: false,
    scopeEditable: false,
  },
);

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void;
  (event: 'save', value: Partial<Agent>): void;
  (event: 'created', value: Agent): void;
  (event: 'cancel'): void;
}>();

const date = useDate();
const apiClient = useApiClient();

const isOpen = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
});

const isEditing = computed(() => !!props.agent?.id);

const form = ref<Partial<Agent>>({
  name: '',
  no_schedule: false,
  priority: 0,
});

const orgs = ref<Org[]>([]);
const scopeMode = ref<'global' | 'orgs'>('global');
const selectedOrgIds = ref<number[]>([]);

onMounted(async () => {
  if (props.scopeEditable) {
    const allOrgs: Org[] = [];
    let page = 1;
    let done = false;
    while (!done) {
      const batch = (await apiClient.getOrgs({ page, perPage: 50 })) ?? [];
      allOrgs.push(...batch);
      if (batch.length < 50) {
        done = true;
      } else {
        page += 1;
      }
    }
    orgs.value = allOrgs;
  }
});

function syncScopeFromAgent(agent: Agent | null) {
  const ids = agent?.org_ids ?? [];
  scopeMode.value = ids.length > 0 ? 'orgs' : 'global';
  selectedOrgIds.value = [...ids];
}

function toggleOrg(orgId: number, checked: boolean) {
  if (checked) {
    if (!selectedOrgIds.value.includes(orgId)) {
      selectedOrgIds.value = [...selectedOrgIds.value, orgId];
    }
  } else {
    selectedOrgIds.value = selectedOrgIds.value.filter((id) => id !== orgId);
  }
}

const createdAgent = ref<AgentWithToken | null>(null);
const copiedCreated = ref(false);

// Sync form with agent prop when editing
watch(
  () => props.agent,
  (newAgent) => {
    if (newAgent) {
      form.value = {
        name: newAgent.name || '',
        no_schedule: newAgent.no_schedule || false,
        priority: newAgent.priority || 0,
      };
    } else {
      form.value = {
        name: '',
        no_schedule: false,
        priority: 0,
      };
    }
    syncScopeFromAgent(newAgent ?? null);
  },
  { immediate: true },
);

// Reset when dialog closes
watch(isOpen, (open) => {
  if (!open) {
    createdAgent.value = null;
    copiedCreated.value = false;
    form.value = {
      name: '',
      no_schedule: false,
      priority: 0,
    };
    scopeMode.value = 'global';
    selectedOrgIds.value = [];
  }
});

function handleSave() {
  const orgIds = props.scopeEditable
    ? scopeMode.value === 'orgs'
      ? [...selectedOrgIds.value]
      : []
    : (props.agent?.org_ids ?? []);
  emit('save', {
    ...form.value,
    id: props.agent?.id,
    org_ids: orgIds,
  });
}

function handleCancel() {
  emit('cancel');
  isOpen.value = false;
}

function handleClose() {
  isOpen.value = false;
}

async function copyCreatedToken() {
  if (!createdAgent.value?.token) return;

  try {
    await navigator.clipboard.writeText(createdAgent.value.token);
    copiedCreated.value = true;
    setTimeout(() => {
      copiedCreated.value = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

// Expose method to show created agent with token
function showCreatedAgent(agent: AgentWithToken) {
  createdAgent.value = agent;
}

defineExpose({
  showCreatedAgent,
});
</script>
