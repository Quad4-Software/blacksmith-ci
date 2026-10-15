<template>
  <!-- Escape, an overlay click and the close button cannot dismiss the dialog while the check runs. -->
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ $t('user.settings.sso.link', { name }) }}</DialogTitle>
        <DialogDescription>{{ $t('ldap.link_description') }}</DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="submit">
        <div class="flex flex-col gap-1.5">
          <Label for="ldap-link-username">{{ $t('ldap.username') }}</Label>
          <Input
            id="ldap-link-username"
            v-model="username"
            type="text"
            name="username"
            autocomplete="username"
            required
            :disabled="submitting"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="ldap-link-password">{{ $t('ldap.password') }}</Label>
          <Input
            id="ldap-link-password"
            v-model="password"
            type="password"
            name="password"
            autocomplete="current-password"
            required
            :disabled="submitting"
          />
        </div>
        <p v-if="errorMessage" role="alert" class="text-sm text-destructive">{{ errorMessage }}</p>
        <DialogFooter>
          <Button variant="outline" type="button" :disabled="submitting" @click="onOpenChange(false)">
            {{ $t('cancel') }}
          </Button>
          <Button type="submit" :disabled="submitting">
            <Icon v-if="submitting" name="spinner" class="mr-1.5 h-4 w-4 animate-spin" />
            {{ $t('user.settings.sso.link', { name }) }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import useApiClient from '~/compositions/useApiClient';
import { ApiRequestError } from '~/lib/api/client';
import { ldapErrorMessage } from '~/lib/ldapLogin';

const props = defineProps<{ open: boolean; name: string }>();
const emit = defineEmits<{
  (e: 'update:open', value: boolean): void;
  (e: 'linked'): void;
}>();

const { t } = useI18n();
const apiClient = useApiClient();

const username = ref('');
const password = ref('');
const submitting = ref(false);
const errorMessage = ref('');

// A reopened dialog starts empty, so credentials never linger in the page.
watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      username.value = '';
      password.value = '';
      errorMessage.value = '';
    }
  },
);

function onOpenChange(value: boolean) {
  if (!submitting.value) {
    emit('update:open', value);
  }
}

async function submit() {
  if (submitting.value) return;
  submitting.value = true;
  errorMessage.value = '';
  try {
    await apiClient.linkLdapIdentity(username.value, password.value);
    password.value = '';
    emit('update:open', false);
    emit('linked');
  } catch (error) {
    // The server's error text is the error code.
    const code = error instanceof Error ? error.message : '';
    errorMessage.value = ldapErrorMessage(t, code, error instanceof ApiRequestError ? error.retryAfter : undefined);
    password.value = '';
  } finally {
    submitting.value = false;
  }
}
</script>
