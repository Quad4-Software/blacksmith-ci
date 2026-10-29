<template>
  <div class="flex h-full w-full flex-col items-center justify-center gap-6 p-4">
    <img :src="logoSrc" alt="Blacksmith CI" class="h-16 w-16 opacity-40" />
    <div class="text-center">
      <h1 class="text-4xl font-bold text-foreground">{{ $t('not_found.heading') }}</h1>
      <p class="mt-2 text-muted-foreground">{{ $t('not_found.not_found') }}</p>
    </div>
    <router-link
      replace
      :to="{ name: 'home' }"
      class="inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
    >
      {{ $t('not_found.back_home') }}
    </router-link>
  </div>
</template>

<script lang="ts" setup>
import { computed } from 'vue';

import LogoLight from '~/assets/logo-dark.png';
import LogoDark from '~/assets/logo.png';
import { useTheme } from '~/compositions/useTheme';

const { theme } = useTheme();

const logoSrc = computed(() => (theme.value === 'dark' ? LogoLight : LogoDark));
</script>
