<template>
  <Settings :title="$t('admin.settings.users.users')" :description="$t('admin.settings.users.desc')">
    <template #headerActions>
      <Button
        v-if="selectedUser"
        :text="$t('admin.settings.users.show')"
        start-icon="back"
        @click="selectedUser = undefined"
      />
      <Button v-else :text="$t('admin.settings.users.add')" start-icon="plus" @click="showAddUser" />
    </template>
    <div v-if="!selectedUser" class="space-y-4 text-wp-text-100">
      <!-- Filter controls -->
      <div v-if="users && users.length > 0" class="mb-4 space-y-2">
        <div class="flex flex-wrap gap-2">
          <!-- Search input -->
          <div class="flex-1 min-w-[200px]">
            <TextField
              v-model="searchQuery"
              :placeholder="$t('admin.settings.users.search')"
              type="text"
              class="h-10 shadow-xs"
            />
          </div>

          <!-- Admin filter -->
          <div class="min-w-[150px]">
            <SelectField
              v-model="adminFilter"
              :placeholder="$t('admin.settings.users.filter_admin')"
              :options="adminOptions"
              class="h-10"
            />
          </div>

          <!-- Clear filters button -->
          <Button
            v-if="hasActiveFilters"
            :text="$t('admin.settings.users.clear_filters')"
            color="gray"
            @click="clearFilters"
          />
        </div>
      </div>

      <ListItem
        v-for="user in filteredUsers"
        :key="user.id"
        class="items-center gap-2 bg-wp-background-200! dark:bg-wp-background-100!"
      >
        <img v-if="user.avatar_url" class="h-6 rounded-md" :src="user.avatar_url" /> <span>{{ user.login }}</span>
        <!-- Forge profile link with local icon -->
        <a
          v-if="primaryForge && user.login"
          :href="getUserForgeUrl(user)"
          :title="`View ${user.login} on ${primaryForge.url}`"
          target="_blank"
          class="inline-flex items-center justify-center h-6 w-6 hover:opacity-80 transition-opacity"
        >
          <Icon :name="getForgeIconName(primaryForge)" class="h-4 w-4 text-wp-text-100" />
        </a>
        <Badge
          v-if="user.admin"
          class="md:display-unset ml-auto hidden"
          :label="$t('admin.settings.users.admin.admin')"
        />
        <IconButton
          icon="edit"
          :title="$t('admin.settings.users.edit_user')"
          class="md:display-unset h-8 w-8"
          :class="{ 'ml-auto': !user.admin, 'ml-2': user.admin }"
          @click="editUser(user)"
        />
        <IconButton
          icon="trash"
          :title="$t('admin.settings.users.delete_user')"
          class="ml-2 h-8 w-8 hover:text-wp-error-100"
          :is-loading="isDeleting"
          @click="deleteUser(user)"
        />
      </ListItem>
      <div v-if="filteredUsers.length === 0 && users && users.length > 0" class="ml-2">
        {{ $t('admin.settings.users.no_match') }}
      </div>
      <div v-if="users?.length === 0" class="ml-2">{{ $t('admin.settings.users.none') }}</div>
    </div>

    <div v-else>
      <form @submit.prevent="saveUser">
        <InputField v-slot="{ id }" :label="$t('admin.settings.users.login')">
          <TextField :id="id" v-model="selectedUser.login" :disabled="isEditingUser" />
        </InputField>
        <InputField v-slot="{ id }" :label="$t('admin.settings.users.email')">
          <TextField :id="id" v-model="selectedUser.email" />
        </InputField>
        <InputField v-slot="{ id }" :label="$t('admin.settings.users.avatar_url')">
          <div class="flex gap-2">
            <img v-if="selectedUser.avatar_url" class="h-8 w-8 rounded-md" :src="selectedUser.avatar_url" />
            <TextField :id="id" v-model="selectedUser.avatar_url" />
          </div>
        </InputField>
        <InputField :label="$t('admin.settings.users.admin.admin')">
          <Checkbox
            :model-value="selectedUser.admin || false"
            :label="$t('admin.settings.users.admin.placeholder')"
            @update:model-value="selectedUser!.admin = $event"
          />
        </InputField>
        <div class="flex gap-2">
          <Button :text="$t('admin.settings.users.cancel')" @click="selectedUser = undefined" />
          <Button
            :is-loading="isSaving"
            type="submit"
            color="default"
            :text="isEditingUser ? $t('admin.settings.users.save') : $t('admin.settings.users.add')"
          />
        </div>
      </form>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { cloneDeep } from 'lodash';
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import Badge from '~/components/atomic/Badge.vue';
import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import ListItem from '~/components/atomic/ListItem.vue';
import Checkbox from '~/components/form/Checkbox.vue';
import InputField from '~/components/form/InputField.vue';
import SelectField from '~/components/form/SelectField.vue';
import TextField from '~/components/form/TextField.vue';
import Settings from '~/components/layout/Settings.vue';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import useNotifications from '~/compositions/useNotifications';
import { usePagination } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { User, Forge } from '~/lib/api/types';
import type { SelectOption } from '~/components/form/form.types';
import type { IconNames } from '~/components/atomic/Icon.vue';

const apiClient = useApiClient();
const notifications = useNotifications();
const { t } = useI18n();

// Load forges for forge URL construction
const forges = ref<Forge[]>([]);
const primaryForge = computed(() => forges.value[0] || null);

const selectedUser = ref<Partial<User>>();
const isEditingUser = computed(() => !!selectedUser.value?.id);

async function loadUsers(page: number): Promise<User[] | null> {
  return apiClient.getUsers({ page });
}

const { resetPage, data: users } = usePagination(loadUsers, () => !selectedUser.value);

// Load forges on component mount
onMounted(async () => {
  try {
    forges.value = (await apiClient.getForges()) ?? [];
    // Note: Skip fetchFaviconFromHead due to CORS issues, rely on fallback extensions
  } catch (error) {
    console.warn('Failed to load forges:', error);
  }
});

// Map forge types to local icon names
function getForgeIconName(forge: Forge): IconNames {
  // Handle special cases where forge URL might indicate specific forge instances
  if (forge.url.toLowerCase().includes('codeberg')) {
    return 'codeberg';
  }
  if (forge.url.toLowerCase().includes('codefloe')) {
    return 'codefloe';
  }

  // Map standard forge types to icon names
  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo', // fallback for unknown types
  };

  return typeToIcon[forge.type] || 'repo';
}

function getUserForgeUrl(user: User): string {
  if (!primaryForge.value?.url || !user.login) return '';
  try {
    const baseUrl = primaryForge.value.url.endsWith('/') ? primaryForge.value.url.slice(0, -1) : primaryForge.value.url;
    return `${baseUrl}/${user.login}`;
  } catch {
    return '';
  }
}

// Filter state
const searchQuery = ref('');
const adminFilter = ref('');

// Filter options
const adminOptions = computed<SelectOption[]>(() => [
  { value: '', text: t('admin.settings.users.filter_admin') },
  { value: 'admin', text: 'Admin' },
  { value: 'user', text: 'User' },
]);

// Check if any filters are active
const hasActiveFilters = computed(() => {
  return searchQuery.value !== '' || adminFilter.value !== '';
});

// Clear all filters
function clearFilters() {
  searchQuery.value = '';
  adminFilter.value = '';
}

// Filtered users based on all filters
const filteredUsers = computed(() => {
  if (!users.value) return [];

  return users.value.filter((user) => {
    // Admin filter
    if (adminFilter.value) {
      if (adminFilter.value === 'admin' && !user.admin) return false;
      if (adminFilter.value === 'user' && user.admin) return false;
    }

    // Search query - search in login and email
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase();
      const loginMatch = user.login?.toLowerCase().includes(query) || false;
      const emailMatch = user.email?.toLowerCase().includes(query) || false;

      if (!loginMatch && !emailMatch) {
        return false;
      }
    }

    return true;
  });
});

const { doSubmit: saveUser, isLoading: isSaving } = useAsyncAction(async () => {
  if (!selectedUser.value) {
    throw new Error("Unexpected: Can't get user");
  }

  if (isEditingUser.value) {
    await apiClient.updateUser(selectedUser.value);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.users.notify_text_saved'),
      type: 'success',
    });
  } else {
    selectedUser.value = await apiClient.createUser(selectedUser.value);
    notifications.notify({
      title: t('notify_title_success'),
      text: t('admin.settings.users.notify_text_created'),
      type: 'success',
    });
  }
  selectedUser.value = undefined;
  resetPage();
});

const { doSubmit: deleteUser, isLoading: isDeleting } = useAsyncAction(async (_user: User) => {
  // eslint-disable-next-line no-alert
  if (!confirm(t('admin.settings.users.delete_confirm'))) {
    return;
  }

  await apiClient.deleteUser(_user);
  notifications.notify({
    title: t('notify_title_success'),
    text: t('admin.settings.users.notify_text_deleted'),
    type: 'success',
    duration: 6000,
  });
  resetPage();
});

function editUser(user: User) {
  selectedUser.value = cloneDeep(user);
}

function showAddUser() {
  selectedUser.value = cloneDeep({ login: '' });
}

useCrowTitle(computed(() => [t('admin.settings.users.users'), t('admin.settings.settings')]));
</script>
