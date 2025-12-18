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
              <Icon :name="copiedCreated ? 'lucide:check' : 'lucide:copy'" class="h-4 w-4" />
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
              :model-value="form.no_schedule || false"
              @update:model-value="form.no_schedule = $event === true"
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

          <!-- Read-only fields when editing -->
          <template v-if="isEditing && agent">
            <!-- Token field for admins -->
            <div v-if="isAdmin && agent.token" class="space-y-2">
              <label class="text-sm font-medium">{{ $t('admin.settings.agents.token') }}</label>
              <div class="flex gap-2">
                <Input :model-value="agent.token" readonly class="font-mono text-xs" />
                <Button type="button" variant="outline" size="icon" :title="$t('copy')" @click="copyAgentToken">
                  <Icon :name="copiedAgent ? 'lucide:check' : 'lucide:copy'" class="h-4 w-4" />
                </Button>
              </div>
            </div>

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
import { computed, ref, watch } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Checkbox } from '~/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { useDate } from '~/compositions/useDate';
import type { Agent } from '~/lib/api/types';

const props = withDefaults(
  defineProps<{
    open?: boolean;
    agent?: Agent | null;
    isSaving?: boolean;
    isAdmin?: boolean;
  }>(),
  {
    open: false,
    agent: null,
    isSaving: false,
    isAdmin: false,
  },
);

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void;
  (event: 'save', value: Partial<Agent>): void;
  (event: 'created', value: Agent): void;
  (event: 'cancel'): void;
}>();

const date = useDate();

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

const createdAgent = ref<Agent | null>(null);
const copiedCreated = ref(false);
const copiedAgent = ref(false);

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
  },
  { immediate: true },
);

// Reset when dialog closes
watch(isOpen, (open) => {
  if (!open) {
    createdAgent.value = null;
    copiedCreated.value = false;
    copiedAgent.value = false;
    form.value = {
      name: '',
      no_schedule: false,
      priority: 0,
    };
  }
});

function handleSave() {
  emit('save', {
    ...form.value,
    id: props.agent?.id,
    org_id: props.agent?.org_id,
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

async function copyAgentToken() {
  if (!props.agent?.token) return;

  try {
    await navigator.clipboard.writeText(props.agent.token);
    copiedAgent.value = true;
    setTimeout(() => {
      copiedAgent.value = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

// Expose method to show created agent with token
function showCreatedAgent(agent: Agent) {
  createdAgent.value = agent;
}

defineExpose({
  showCreatedAgent,
});
</script>
