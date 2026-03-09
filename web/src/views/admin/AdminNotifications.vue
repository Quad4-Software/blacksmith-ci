<template>
  <Settings :title="$t('admin.settings.notifications.smtp_status')">
    <div v-if="!smtpStatus || !smtpStatus.configured" class="flex flex-col gap-4">
      <p class="text-sm text-muted-foreground">{{ $t('admin.settings.notifications.not_configured') }}</p>
    </div>

    <div v-else class="flex flex-col gap-6">
      <p class="text-sm text-muted-foreground">{{ $t('admin.settings.notifications.configured') }}</p>

      <div class="grid gap-2 text-sm">
        <div class="flex gap-2">
          <span class="font-medium w-28">{{ $t('admin.settings.notifications.host') }}</span>
          <span class="text-muted-foreground">{{ smtpStatus.host }}</span>
        </div>
        <div class="flex gap-2">
          <span class="font-medium w-28">{{ $t('admin.settings.notifications.port') }}</span>
          <span class="text-muted-foreground">{{ smtpStatus.port }}</span>
        </div>
        <div class="flex gap-2">
          <span class="font-medium w-28">{{ $t('admin.settings.notifications.from') }}</span>
          <span class="text-muted-foreground">{{ smtpStatus.from }}</span>
        </div>
      </div>

      <div>
        <Button :disabled="isTesting" @click="sendTestEmail">
          <Icon v-if="isTesting" name="spinner" class="size-4 animate-spin mr-2" />
          {{ isTesting ? $t('admin.settings.notifications.testing') : $t('admin.settings.notifications.test') }}
        </Button>
      </div>

      <div v-if="testResult" class="text-sm" :class="testResult.success ? 'text-green-600' : 'text-red-600'">
        {{
          testResult.success
            ? $t('admin.settings.notifications.test_success')
            : $t('admin.settings.notifications.test_failed')
        }}
        <span v-if="!testResult.success && testResult.message" class="block mt-1 text-muted-foreground">
          {{ testResult.message }}
        </span>
      </div>
    </div>
  </Settings>
</template>

<script lang="ts" setup>
import { onMounted, ref } from 'vue';

import { Button } from '~/components/ui/button';
import Icon from '~/components/atomic/Icon.vue';
import Settings from '~/components/layout/Settings.vue';
import useApiClient from '~/compositions/useApiClient';
import type { SMTPStatus, SMTPTestResult } from '~/lib/api/types';

const apiClient = useApiClient();

const smtpStatus = ref<SMTPStatus | null>(null);
const isTesting = ref(false);
const testResult = ref<SMTPTestResult | null>(null);

async function loadStatus() {
  try {
    smtpStatus.value = await apiClient.getAdminSMTPStatus();
  } catch {
    // Leave as null
  }
}

async function sendTestEmail() {
  isTesting.value = true;
  testResult.value = null;
  try {
    testResult.value = await apiClient.testAdminSMTP();
  } catch (error) {
    testResult.value = { success: false, message: String(error) };
  } finally {
    isTesting.value = false;
  }
}

onMounted(loadStatus);
</script>
