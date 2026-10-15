<template>
  <form class="flex w-full flex-col gap-3" @submit.prevent="submit">
    <div class="flex flex-col gap-1.5">
      <Label for="ldap-username">{{ $t('ldap.username') }}</Label>
      <Input
        id="ldap-username"
        v-model="username"
        type="text"
        name="username"
        autocomplete="username"
        required
        :disabled="submitting"
      />
    </div>
    <div class="flex flex-col gap-1.5">
      <Label for="ldap-password">{{ $t('ldap.password') }}</Label>
      <Input
        id="ldap-password"
        v-model="password"
        type="password"
        name="password"
        autocomplete="current-password"
        required
        :disabled="submitting"
      />
    </div>
    <div
      v-if="errorMessage"
      role="alert"
      class="w-full rounded-lg bg-red-50 px-3 py-2 text-center text-red-500 dark:bg-red-900/30"
    >
      {{ errorMessage }}
    </div>
    <Button type="submit" class="w-full justify-center" :disabled="submitting">
      <Icon v-if="submitting" name="spinner" class="h-5 w-5 animate-spin" aria-hidden="true" />
      <span v-else>{{ $t('ldap.sign_in', { name }) }}</span>
    </Button>
  </form>
</template>

<script lang="ts" setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import useConfig from '~/compositions/useConfig';
import useUserConfig from '~/compositions/useUserConfig';
import { ldapErrorMessage, ldapLogin } from '~/lib/ldapLogin';

defineProps<{ name: string }>();

const { t } = useI18n();
const config = useConfig();
const route = useRoute();

const username = ref('');
const password = ref('');
const submitting = ref(false);
const errorMessage = ref('');

async function submit() {
  if (submitting.value) return;
  submitting.value = true;
  errorMessage.value = '';
  try {
    const result = await ldapLogin(config.rootPath, username.value, password.value);
    if (result.ok) {
      // Like the forge and OIDC flows, return to the page that asked for sign-in
      // once the reload has picked up the session. The form stays disabled
      // until the page is gone.
      const url = typeof route.query.url === 'string' ? route.query.url : '';
      if (url) {
        useUserConfig().setUserConfig('redirectUrl', url);
      }
      window.location.href = `${config.rootPath}/`;
      return;
    }
    errorMessage.value = ldapErrorMessage(t, result.code, result.retryAfter);
    password.value = '';
    submitting.value = false;
  } catch (error) {
    submitting.value = false;
    throw error;
  }
}
</script>
