<template>
  <Scaffold v-if="org" enable-tabs :go-back="goBack">
    <template #title>
      <span>
        <router-link :to="{ name: 'org' }" class="hover:underline hover:decoration-primary">{{
          org.name
          /* eslint-disable @intlify/vue-i18n/no-raw-text */
        }}</router-link
        ><Text>/</Text>{{ $t('settings') }}
      </span>
    </template>

    <Tab icon="secret" icon-size="1rem" :to="{ name: 'org-settings-secrets' }" :title="$t('secrets.secrets')" />
    <Tab
      icon="docker"
      icon-size="1rem"
      :to="{ name: 'org-settings-registries' }"
      :title="$t('registries.registries')"
    />
    <Tab
      v-if="useConfig().userRegisteredAgents"
      icon="agent"
      icon-size="1rem"
      :to="{ name: 'org-settings-agents' }"
      :title="$t('admin.settings.agents.agents')"
    />
    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import { onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import useConfig from '~/compositions/useConfig';
import { requiredInject } from '~/compositions/useInjectProvide';
import useNotifications from '~/compositions/useNotifications';
import { useRouteBack } from '~/compositions/useRouteBack';

const notifications = useNotifications();
const router = useRouter();
const i18n = useI18n();

const org = requiredInject('org');
const orgPermissions = requiredInject('org-permissions');

onMounted(async () => {
  if (!orgPermissions.value?.admin) {
    notifications.notify({
      title: i18n.t('notify_title_error'),
      type: 'error',
      duration: 9000,
      text: i18n.t('org.settings.notify_text_not_allowed'),
    });
    await router.replace({ name: 'home' });
  }
});

const goBack = useRouteBack({ name: 'org' });
</script>
