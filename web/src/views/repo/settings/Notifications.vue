<template>
  <Settings :title="$t('repo.settings.notifications.notifications')">
    <p v-if="smtpStatus && !smtpStatus.configured" class="text-sm text-muted-foreground mb-4">
      {{ $t('repo.settings.notifications.smtp_not_configured') }}
    </p>
    <form v-else class="flex flex-col gap-6" @submit.prevent="saveSettings">
      <div class="grid gap-2">
        <div class="flex items-start gap-3">
          <Checkbox :checked="isEnabled" @update:checked="(v) => (isEnabled = v === true)" />
          <div class="flex flex-col">
            <label class="text-sm font-medium">{{ $t('repo.settings.notifications.enable_email') }}</label>
            <p class="text-sm text-muted-foreground">{{ $t('repo.settings.notifications.enable_email_desc') }}</p>
          </div>
        </div>
      </div>

      <div v-if="isEnabled" class="grid gap-2">
        <label class="text-sm font-medium text-foreground">{{ $t('repo.settings.notifications.events') }}</label>

        <div class="flex items-start gap-3">
          <Checkbox :checked="events.includes('failure')" @update:checked="(v) => toggleEvent('failure', v === true)" />
          <label class="text-sm font-normal">{{ $t('repo.settings.notifications.event_failure') }}</label>
        </div>
        <div class="flex items-start gap-3">
          <Checkbox :checked="events.includes('success')" @update:checked="(v) => toggleEvent('success', v === true)" />
          <label class="text-sm font-normal">{{ $t('repo.settings.notifications.event_success') }}</label>
        </div>
        <div class="flex items-start gap-3">
          <Checkbox
            :checked="events.includes('status_change')"
            @update:checked="(v) => toggleEvent('status_change', v === true)"
          />
          <div class="flex flex-col">
            <label class="text-sm font-normal">{{ $t('repo.settings.notifications.event_status_change') }}</label>
            <p class="text-sm text-muted-foreground">
              {{ $t('repo.settings.notifications.event_status_change_desc') }}
            </p>
          </div>
        </div>
      </div>

      <Button type="submit" class="mr-auto" :disabled="isSaving">
        <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
        {{ $t('repo.settings.notifications.save') }}
      </Button>
    </form>
  </Settings>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';

import { Checkbox } from '~/components/ui/checkbox';
import { Button } from '~/components/ui/button';
import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
import useApiClient from '~/compositions/useApiClient';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import type { NotificationEvent, SMTPStatus } from '~/lib/api/types';

const apiClient = useApiClient();
const notifications = useNotifications();
const i18n = useI18n();
const repo = requiredInject('repo');

const isEnabled = ref(false);
const events = ref<NotificationEvent[]>(['failure']);
const isSaving = ref(false);
const smtpStatus = ref<SMTPStatus | null>(null);

function toggleEvent(event: NotificationEvent, checked: boolean) {
  if (checked) {
    if (!events.value.includes(event)) {
      events.value.push(event);
    }
  } else {
    events.value = events.value.filter((e) => e !== event);
  }
}

async function loadConfig() {
  try {
    smtpStatus.value = await apiClient.getAdminSMTPStatus();
    const config = await apiClient.getRepoNotificationConfig(repo.value.id);
    isEnabled.value = config.is_enabled;
    if (config.events && config.events.length > 0) {
      events.value = config.events;
    }
  } catch {
    // Default values are fine
  }
}

async function saveSettings() {
  isSaving.value = true;
  try {
    await apiClient.saveRepoNotificationConfig(repo.value.id, {
      is_enabled: isEnabled.value,
      events: events.value,
    });
    notifications.notify({
      title: i18n.t('repo.settings.notifications.saved'),
      type: 'success',
    });
  } catch (error) {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      type: 'error',
      text: String(error),
    });
  } finally {
    isSaving.value = false;
  }
}

onMounted(loadConfig);
</script>
