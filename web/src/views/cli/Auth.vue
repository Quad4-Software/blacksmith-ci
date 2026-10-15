<template>
  <div class="m-auto flex flex-col gap-4">
    <div class="text-center text-foreground">
      <CrowLogo preserveAspectRatio="xMinYMin slice" class="m-auto mb-8 w-32" />
      <template v-if="state === 'confirm'">
        <h1 class="text-4xl font-bold">{{ $t('login_to_cli') }}</h1>

        <p class="text-2xl">{{ $t('login_to_cli_description') }}</p>
      </template>
      <template v-else-if="state === 'success'">
        <h1 class="text-4xl font-bold">{{ $t('cli_login_success') }}</h1>

        <p class="text-2xl">{{ $t('return_to_cli') }}</p>
      </template>
      <template v-else-if="state === 'failed'">
        <h1 class="mt-4 text-4xl font-bold">{{ $t('cli_login_failed') }}</h1>

        <p class="text-2xl">{{ $t('return_to_cli') }}</p>
      </template>
      <template v-else-if="state === 'denied'">
        <h1 class="mt-4 text-4xl font-bold">{{ $t('cli_login_denied') }}</h1>

        <p class="text-2xl">{{ $t('return_to_cli') }}</p>
      </template>
    </div>

    <div v-if="state === 'confirm'" class="flex justify-center gap-4">
      <Button
        variant="outline"
        class="border-orange-400 hover:border-orange-500 hover:bg-orange-50 dark:hover:bg-orange-950/20"
        :disabled="loading"
        @click="sendToken(false)"
      >
        <Icon v-if="loading" name="spinner" class="h-5 w-5 animate-spin" aria-hidden="true" />
        <span>{{ $t('login_to_cli') }}</span>
      </Button>
      <Button variant="destructive" :disabled="loading" @click="abortLogin">{{ $t('abort') }}</Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRoute } from 'vue-router';

import CrowLogo from '~/assets/logo.svg?component';
import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import useApiClient from '~/compositions/useApiClient';
import { cliTokenUrl, sendCliToken } from '~/lib/cli-auth';

const apiClient = useApiClient();
const route = useRoute();
const state = ref<'confirm' | 'success' | 'failed' | 'denied'>('confirm');
const loading = ref(false);

async function sendToken(abort = false) {
  // Never send the token anywhere but a localhost port.
  const url = cliTokenUrl(route.query.port);
  if (!url) {
    state.value = abort ? 'denied' : 'failed';
    return;
  }

  loading.value = true;
  try {
    if (abort) {
      await sendCliToken(url, '');
      state.value = 'denied';
      window.close();
      return;
    }

    let token: string;
    try {
      token = await apiClient.getToken();
    } catch {
      state.value = 'failed';
      return;
    }
    state.value = (await sendCliToken(url, token)) ? 'success' : 'failed';
  } finally {
    loading.value = false;
  }
}

async function abortLogin() {
  await sendToken(true);
}
</script>
