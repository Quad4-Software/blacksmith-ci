<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-start gap-3">
      <Checkbox
        id="watch-in-app"
        :checked="modelValue.in_app"
        data-testid="watch-in-app"
        @update:checked="(v) => update({ in_app: v === true })"
      />
      <div class="flex flex-col">
        <Label for="watch-in-app">{{ $t('repo.watch.in_app') }}</Label>
        <p class="text-sm text-muted-foreground">{{ $t('repo.watch.in_app_desc') }}</p>
      </div>
    </div>

    <div class="flex items-start gap-3">
      <Checkbox
        id="watch-email"
        :checked="emailOn"
        :disabled="!emailAvailable"
        data-testid="watch-email"
        @update:checked="(v) => update({ email: v === true })"
      />
      <div class="flex flex-col">
        <Label for="watch-email">{{ $t('repo.watch.email') }}</Label>
        <p class="text-sm text-muted-foreground">{{ $t('repo.watch.email_desc') }}</p>
        <p v-if="!smtpConfigured" class="text-sm text-muted-foreground" data-testid="watch-email-hint">
          {{ $t('repo.watch.smtp_not_configured') }}
        </p>
        <p v-else-if="!hasEmail" class="text-sm text-muted-foreground" data-testid="watch-email-hint">
          {{ $t('repo.watch.no_user_email') }}
        </p>
      </div>
    </div>

    <div v-if="emailOn" class="ml-7 flex flex-col gap-2">
      <div v-for="ev in emailEvents" :key="ev.value" class="flex items-start gap-3">
        <Checkbox
          :id="`watch-event-${ev.value}`"
          :checked="modelValue.email_events.includes(ev.value)"
          :data-testid="`watch-event-${ev.value}`"
          @update:checked="(v) => toggleEvent(ev.value, v === true)"
        />
        <Label :for="`watch-event-${ev.value}`" class="font-normal">{{ ev.label }}</Label>
      </div>
    </div>

    <p v-if="!isValid" class="text-sm text-destructive" data-testid="watch-invalid">
      {{ $t('repo.watch.channel_required') }}
    </p>
  </div>
</template>

<script lang="ts" setup>
import { computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import { Checkbox } from '~/components/ui/checkbox';
import { Label } from '~/components/ui/label';
import useConfig from '~/compositions/useConfig';
import type { NotificationEvent, WatchSettings } from '~/lib/api/types';
import { useAuthStore } from '~/store/auth';

const props = defineProps<{ modelValue: WatchSettings }>();
const emit = defineEmits<{
  'update:modelValue': [value: WatchSettings];
  valid: [valid: boolean];
}>();

const { t } = useI18n();
const smtpConfigured = useConfig().smtpConfigured;
const hasEmail = computed(() => !!useAuthStore().user?.email);
const emailAvailable = computed(() => smtpConfigured && hasEmail.value);
// Without SMTP or an address no email is sent, so a stored email channel is
// shown as off. The stored value is left alone until the user saves.
const emailOn = computed(() => emailAvailable.value && props.modelValue.email);

const emailEvents = computed<{ value: NotificationEvent; label: string }[]>(() => [
  { value: 'failure', label: t('repo.watch.event_failure') },
  { value: 'success', label: t('repo.watch.event_success') },
  { value: 'status_change', label: t('repo.watch.event_status_change') },
]);

const isValid = computed(() => props.modelValue.in_app || emailOn.value);

watch(isValid, (v) => emit('valid', v), { immediate: true });

function update(patch: Partial<WatchSettings>) {
  emit('update:modelValue', { ...props.modelValue, ...patch });
}

function toggleEvent(event: NotificationEvent, on: boolean) {
  const events = on
    ? [...new Set([...props.modelValue.email_events, event])]
    : props.modelValue.email_events.filter((e) => e !== event);
  update({ email_events: events });
}
</script>
