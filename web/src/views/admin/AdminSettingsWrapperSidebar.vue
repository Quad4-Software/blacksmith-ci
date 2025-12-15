<template>
  <div class="flex flex-1 flex-col gap-4 overflow-auto">
    <router-view />
  </div>
</template>

<script lang="ts" setup>
import { onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import useAuthentication from '~/compositions/useAuthentication';
import useNotifications from '~/compositions/useNotifications';

const router = useRouter();
const i18n = useI18n();
const { user } = useAuthentication();
const notifications = useNotifications();

onMounted(async () => {
  if (!user?.admin) {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      type: 'error',
      duration: 9000,
      text: i18n.t('admin.settings.notify_text_not_allowed'),
    });
    await router.replace({ name: 'home' });
  }
});
</script>
