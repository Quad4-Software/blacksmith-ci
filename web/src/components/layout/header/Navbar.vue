<template>
  <nav
    class="text-neutral-content flex border-wp-background-100 bg-wp-background-secondary-800 p-3 text-wp-primary-text-100"
  >
    <div class="flex items-center space-x-2">
      <router-link :to="{ name: 'home' }" class="-my-2 flex flex-col px-2">
        <CrowLogo class="h-8 w-8" />
        <!-- <span class="text-center text-xs" :title="version?.current">{{ version?.currentShort }}</span> -->
      </router-link>
      <router-link v-if="user" :to="{ name: 'repos' }" class="navbar-clickable navbar-link">
        <span class="flex md:hidden">{{ $t('repos') }}</span>
        <span class="hidden md:flex">{{ $t('repositories.title') }}</span>
      </router-link>
      <a href="https://crowci.dev/" target="_blank" class="navbar-clickable navbar-link hidden md:flex">{{
        $t('docs')
      }}</a>
      <a v-if="enableSwagger" :href="apiUrl" target="_blank" class="navbar-clickable navbar-link hidden md:flex">{{
        $t('api')
      }}</a>
    </div>

    <div class="-m-1.5 ml-auto flex items-center space-x-2">
      <IconButton
        v-if="user?.admin"
        class="navbar-icon relative"
        :title="$t('settings')"
        :to="{ name: 'admin-settings' }"
      >
        <Icon name="settings" />
        <div v-if="version?.needsUpdate" class="absolute right-2 top-2 h-3 w-3 rounded-full bg-wp-error-100" />
      </IconButton>
      <ActivePipelines v-if="user" class="navbar-icon p-1.5!" />
      <IconButton v-if="user" :to="{ name: 'user' }" :title="$t('user.settings.settings')" class="navbar-icon p-1.5!">
        <img v-if="user && user.avatar_url" class="rounded-md" :src="`${user.avatar_url}`" />
      </IconButton>
      <Button
        v-else
        :text="$t('login')"
        :to="`/login?url=${route.fullPath}`"
        class="navbar-link !text-wp-primary-text-100 bg-wp-custom-highlight-100 !border-transparent !shadow-none"
      />
    </div>
  </nav>
</template>

<script lang="ts" setup>
import { useRoute } from 'vue-router';

import CrowLogo from '~/assets/logo.svg?component';
import Button from '~/components/atomic/Button.vue';
import Icon from '~/components/atomic/Icon.vue';
import IconButton from '~/components/atomic/IconButton.vue';
import useAuthentication from '~/compositions/useAuthentication';
import useConfig from '~/compositions/useConfig';
import { useVersion } from '~/compositions/useVersion';

import ActivePipelines from './ActivePipelines.vue';

const version = useVersion();
const config = useConfig();
const route = useRoute();
const authentication = useAuthentication();
const { user } = authentication;
const apiUrl = `${config.rootPath ?? ''}/swagger/index.html`;

const { enableSwagger } = config;
</script>

<style scoped>
.navbar-icon {
  height: 2.75rem;
  width: 2.75rem;
  border-radius: 0.375rem;
  padding: 0.625rem;
  position: relative;
  display: flex;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background-color: transparent;
  transition-property: color, background-color, border-color, text-decoration-color, fill, stroke;
  transition-duration: 100ms;
}

.navbar-icon:hover {
  background-color: rgba(0, 0, 0, 0.1);
}

.dark .navbar-icon:hover {
  background-color: rgba(255, 255, 255, 0.05);
}

.navbar-icon :deep(svg) {
  height: 100%;
  width: 100%;
}

.navbar-link {
  margin-top: -0.25rem;
  margin-bottom: -0.25rem;
  border-radius: 0.375rem;
  padding-left: 0.75rem;
  padding-right: 0.75rem;
  padding-top: 0.5rem;
  padding-bottom: 0.5rem;
  transition-property: color, background-color, border-color, text-decoration-color, fill, stroke;
  transition-duration: 100ms;
}

.navbar-link:hover {
  background-color: rgba(0, 0, 0, 0.1);
}

.dark .navbar-link:hover {
  background-color: rgba(255, 255, 255, 0.05);
}
</style>
