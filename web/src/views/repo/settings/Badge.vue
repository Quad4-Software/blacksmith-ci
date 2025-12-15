<template>
  <Settings :title="$t('repo.settings.badge.badge')">
    <template #titleActions>
      <a v-if="badgeUrl" :href="badgeUrl" target="_blank"> <img :src="badgeUrl" /> </a>
    </template>
    <FieldGroup>
      <Field>
        <FieldLabel for="badge-type">{{ $t('repo.settings.badge.type') }}</FieldLabel>
        <Select id="badge-type" v-model="badgeType" required>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="url">{{ $t('repo.settings.badge.type_url') }}</SelectItem>
            <SelectItem value="markdown">{{ $t('repo.settings.badge.type_markdown') }}</SelectItem>
            <SelectItem value="html">{{ $t('repo.settings.badge.type_html') }}</SelectItem>
          </SelectContent>
        </Select>
      </Field>

      <Field>
        <FieldLabel for="badge-branch">{{ $t('repo.settings.badge.branch') }}</FieldLabel>
        <Select id="badge-branch" v-model="branch">
          <SelectTrigger>
            <SelectValue :placeholder="defaultBranch ? `${defaultBranch} (default)` : 'Select branch'" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="option in branches" :key="option.value" :value="option.value">
              {{ option.text }}
            </SelectItem>
          </SelectContent>
        </Select>
      </Field>

      <div v-if="badgeContent" class="flex flex-col gap-2">
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium text-muted-foreground">{{ $t('repo.settings.badge.badge') }}</span>
          <Button variant="outline" size="sm" class="gap-2" @click="copyToClipboard">
            <Icon name="content-copy" size="1rem" />
            {{ copied ? $t('copied') : $t('copy') }}
          </Button>
        </div>
        <pre class="code-box">{{ badgeContent }}</pre>
      </div>
    </FieldGroup>
  </Settings>
</template>

<script lang="ts" setup>
import { useStorage } from '@vueuse/core';
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import type { SelectOption } from '~/components/form/form.types';
import Settings from '~/components/layout/Settings.vue';
import { Button } from '~/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '~/components/ui/field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import useApiClient from '~/compositions/useApiClient';
import useConfig from '~/compositions/useConfig';
import { requiredInject } from '~/compositions/useInjectProvide';
import { usePaginate } from '~/compositions/usePaginate';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import useNotifications from '~/compositions/useNotifications';

const apiClient = useApiClient();
const repo = requiredInject('repo');
const notifications = useNotifications();
const { t } = useI18n();

const badgeType = useStorage('crow:last-badge-type', 'markdown');
const copied = ref(false);

const defaultBranch = computed(() => repo.value.default_branch);
const branches = ref<SelectOption[]>([]);
const branch = ref<string | undefined>(undefined);

async function loadBranches() {
  branches.value = (await usePaginate((page) => apiClient.getRepoBranches(repo.value.id, { page })))
    .map((b) => ({
      value: b,
      text: b,
    }))
    .filter((b) => b.value !== defaultBranch.value);
  branches.value.unshift({
    value: defaultBranch.value,
    text: `${defaultBranch.value} (default)`,
  });
}

const baseUrl = `${window.location.protocol}//${window.location.hostname}${
  window.location.port ? `:${window.location.port}` : ''
}`;
const { rootPath } = useConfig();
const badgeUrl = computed(() => {
  const isDefaultBranch = !branch.value || branch.value === defaultBranch.value;
  return `${rootPath}/api/v1/badges/${repo.value.id}/status.svg${!isDefaultBranch ? `?branch=${branch.value}` : ''}`;
});
const repoUrl = computed(() => {
  const isDefaultBranch = !branch.value || branch.value === defaultBranch.value;
  return `${rootPath}/repos/${repo.value.id}${!isDefaultBranch && branch.value ? `/branches/${encodeURIComponent(branch.value)}` : ''}`;
});

const badgeContent = computed(() => {
  if (badgeType.value === 'url') {
    return `${baseUrl}${badgeUrl.value}`;
  }

  if (badgeType.value === 'markdown') {
    return `[![status-badge](${baseUrl}${badgeUrl.value})](${baseUrl}${repoUrl.value})`;
  }

  if (badgeType.value === 'html') {
    return `<a href="${baseUrl}${repoUrl.value}" target="_blank">\n  <img src="${baseUrl}${badgeUrl.value}" alt="status-badge" />\n</a>`;
  }

  return '';
});

async function copyToClipboard() {
  if (!badgeContent.value) {
    return;
  }

  try {
    await navigator.clipboard.writeText(badgeContent.value);
    copied.value = true;
    notifications.notify({
      title: t('notify_title_success'),
      text: 'Copied to clipboard',
      type: 'success',
      duration: 3000,
    });
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: 'Failed to copy to clipboard',
      type: 'error',
      duration: 5000,
    });
  }
}

onMounted(() => {
  loadBranches();
});

watch(repo, () => {
  loadBranches();
});

useCrowTitle(computed(() => [t('repo.settings.badge.badge'), repo.value.full_name]));
</script>
