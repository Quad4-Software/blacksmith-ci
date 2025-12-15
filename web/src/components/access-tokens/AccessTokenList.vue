<template>
  <div class="space-y-4">
    <!-- Empty State -->
    <div v-if="tokens.length === 0" class="text-center py-12 text-muted-foreground">
      <Icon name="secret" class="h-12 w-12 mx-auto mb-4 opacity-50" />
      <p>{{ $t('access_tokens.empty') }}</p>
      <p class="text-sm mt-1">{{ $t('access_tokens.empty_description') }}</p>
    </div>

    <!-- Token List -->
    <div v-else class="space-y-3">
      <div v-for="token in tokens" :key="token.id" class="border rounded-lg p-4 flex items-start justify-between gap-4">
        <div class="space-y-2 flex-1 min-w-0">
          <!-- Name and Last Used -->
          <div class="flex items-center gap-2 flex-wrap">
            <h4 class="font-medium">{{ token.name }}</h4>
            <Badge v-if="isExpired(token)" variant="destructive">
              {{ $t('access_tokens.expired') }}
            </Badge>
            <Badge
              v-else-if="isExpiringSoon(token)"
              variant="outline"
              class="border-amber-500/50 text-amber-700 dark:text-amber-400"
            >
              {{ $t('access_tokens.expiring_soon') }}
            </Badge>
          </div>

          <!-- Scopes -->
          <div class="flex flex-wrap gap-1">
            <Badge v-for="scope in token.scopes" :key="scope" variant="secondary" class="text-xs">
              {{ scope }}
            </Badge>
          </div>

          <!-- Metadata -->
          <!-- eslint-disable @intlify/vue-i18n/no-raw-text -->
          <div class="text-xs text-muted-foreground space-y-0.5">
            <p>
              {{ $t('access_tokens.created') }}:
              <time :datetime="formatDatetime(token.created_at)">{{ formatDate(token.created_at) }}</time>
            </p>
            <p v-if="token.last_used">
              {{ $t('access_tokens.last_used') }}:
              <time :datetime="formatDatetime(token.last_used)">{{ formatRelativeTime(token.last_used) }}</time>
            </p>
            <p v-else>
              {{ $t('access_tokens.never_used') }}
            </p>
            <p v-if="token.expires_at">
              {{ $t('access_tokens.expires') }}:
              <time :datetime="formatDatetime(token.expires_at)">{{ formatDate(token.expires_at) }}</time>
            </p>
            <p v-else>
              {{ $t('access_tokens.no_expiration') }}
            </p>
          </div>
          <!-- eslint-enable @intlify/vue-i18n/no-raw-text -->
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-2">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon"
                  class="text-destructive hover:text-destructive hover:bg-destructive/10"
                  :disabled="isDeleting"
                  @click="emit('delete', token.id)"
                >
                  <Icon v-if="isDeleting" name="spinner" class="h-4 w-4 animate-spin" />
                  <Icon v-else name="trash" class="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ $t('access_tokens.delete') }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import type { AccessToken } from '~/lib/api/types';

defineProps<{
  tokens: AccessToken[];
  isDeleting?: boolean;
}>();

const emit = defineEmits<{
  (event: 'delete', tokenId: number): void;
}>();

const { t, d } = useI18n();

function formatDate(timestamp: number): string {
  if (!timestamp) return '';
  return d(new Date(timestamp * 1000), 'short');
}

function formatDatetime(timestamp: number): string {
  if (!timestamp) return '';
  return new Date(timestamp * 1000).toISOString();
}

function formatRelativeTime(timestamp: number): string {
  if (!timestamp) return '';

  const now = Math.floor(Date.now() / 1000);
  const diff = now - timestamp;

  if (diff < 60) {
    return t('time.just_now');
  } else if (diff < 3600) {
    const minutes = Math.floor(diff / 60);
    return t('time.minutes_ago', { count: minutes });
  } else if (diff < 86400) {
    const hours = Math.floor(diff / 3600);
    return t('time.hours_ago', { count: hours });
  } else if (diff < 604800) {
    const days = Math.floor(diff / 86400);
    return t('time.days_ago', { count: days });
  } else {
    return formatDate(timestamp);
  }
}

function isExpired(token: AccessToken): boolean {
  if (!token.expires_at) return false;
  return token.expires_at < Math.floor(Date.now() / 1000);
}

function isExpiringSoon(token: AccessToken): boolean {
  if (!token.expires_at) return false;
  const now = Math.floor(Date.now() / 1000);
  const sevenDays = 7 * 24 * 60 * 60;
  return token.expires_at > now && token.expires_at < now + sevenDays;
}
</script>
