<template>
  <Scaffold enable-tabs>
    <template #title>{{ $t('user.settings.settings') }}</template>
    <template #headerActions><Button :text="$t('logout')" :to="`${address}/logout`" /></template>
    <Tab icon="settings-outline" icon-size="1rem" :to="{ name: 'user' }" :title="$t('user.settings.general.general')" />
    <Tab icon="secret" icon-size="1rem" :to="{ name: 'user-secrets' }" :title="$t('secrets.secrets')" />
    <Tab icon="docker" :to="{ name: 'user-registries' }" :title="$t('registries.registries')" />
    <Tab
      icon="console"
      icon-size="1rem"
      :to="{ name: 'user-cli-and-api' }"
      :title="$t('user.settings.cli_and_api.cli_and_api')"
    />
    <Tab
      v-if="useConfig().userRegisteredAgents"
      icon="agent"
      icon-size="1rem"
      :to="{ name: 'user-agents' }"
      :title="$t('admin.settings.agents.agents')"
    />
    <Tab icon="info" icon-size="1rem" :to="{ name: 'user-info' }" :title="$t('info')" />
    <router-view />
  </Scaffold>
</template>

<script lang="ts" setup>
import Button from '~/components/atomic/Button.vue';
import Scaffold from '~/components/layout/scaffold/Scaffold.vue';
import Tab from '~/components/layout/scaffold/Tab.vue';
import useConfig from '~/compositions/useConfig';

const address = `${window.location.protocol}//${window.location.host}${useConfig().rootPath}`; // port is included in location.host
</script>
