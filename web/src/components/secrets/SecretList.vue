<template>
  <div class="space-y-3 text-foreground">
    <Card v-for="secret in secrets" :key="secret.id" class="hover:shadow-md transition-shadow duration-200">
      <div class="flex items-center gap-3 p-2">
        <!-- Secret Name, Value, and Badges -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="font-medium">{{ secret.name }}</span>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button variant="ghost" size="icon-sm" class="h-6 w-6" @click.stop="copySecretName(secret.name)">
                    <Icon :name="copiedSecrets[secret.name] ? 'lucide:check' : 'lucide:copy'" class="h-3 w-3" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{{ copiedSecrets[secret.name] ? $t('copied') : $t('copy') }}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
            <Badge v-if="secret.edit === false" variant="secondary">
              {{ secret.org_id === 0 ? $t('global_level_secret') : $t('org_level_secret') }}
            </Badge>
            <!-- Abbreviated value display -->
            <code
              v-if="visibleSecretId === secret.id && secret.id && abbreviatedValues[secret.id]"
              class="text-xs font-mono bg-muted px-2 py-0.5 rounded"
            >
              {{ abbreviatedValues[secret.id] }}
            </code>
          </div>
          <!-- Events badges (desktop) -->
          <div class="hidden md:flex items-center gap-2 mt-2">
            <Badge v-for="event in secret.events" :key="event" variant="outline">
              {{ event }}
            </Badge>
          </div>
        </div>

        <!-- Action Buttons -->
        <div v-if="secret.edit !== false" class="flex items-center gap-1 shrink-0">
          <Button
            variant="ghost"
            size="icon"
            :title="visibleSecretId === secret.id ? $t('secrets.hide_value') : $t('secrets.show_value')"
            :disabled="loadingSecretId === secret.id"
            @click="toggleAbbreviatedValue(secret)"
          >
            <Icon v-if="loadingSecretId === secret.id" name="spinner" class="h-4 w-4 animate-spin" />
            <Icon v-else :name="visibleSecretId === secret.id ? 'lucide:eye-off' : 'lucide:eye'" class="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" :title="$t('secrets.edit')" @click="editSecret(secret)">
            <Icon name="edit" class="h-4 w-4" />
          </Button>
          <AlertDialog>
            <AlertDialogTrigger as-child>
              <Button variant="ghost" size="icon" :title="$t('secrets.delete')" :disabled="isDeleting">
                <Icon v-if="!isDeleting" name="trash" class="h-4 w-4 text-destructive" />
                <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{{ $t('secrets.delete') }}</AlertDialogTitle>
                <AlertDialogDescription>
                  {{ $t('secrets.delete_confirm') }}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
                <AlertDialogAction @click="deleteSecret(secret)">
                  {{ $t('delete') }}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <!-- Events badges (mobile) -->
      <div class="flex md:hidden items-center gap-2 px-2 pb-2 pt-0 flex-wrap">
        <Badge v-for="event in secret.events" :key="event" variant="outline">
          {{ event }}
        </Badge>
      </div>
    </Card>

    <!-- Empty State -->
    <div v-if="secrets?.length === 0" class="text-center py-12 text-muted-foreground">
      <Icon name="secret" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">{{ $t('secrets.none') }}</p>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { reactive, ref, toRef } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import useApiClient from '~/compositions/useApiClient';
import useNotifications from '~/compositions/useNotifications';
import type { Secret } from '~/lib/api/types';

const props = defineProps<{
  modelValue: (Secret & { edit?: boolean })[];
  isDeleting: boolean;
}>();

const emit = defineEmits<{
  (event: 'edit', secret: Secret): void;
  (event: 'delete', secret: Secret): void;
}>();

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();

const secrets = toRef(props, 'modelValue');

// Track copied state for secret names
const copiedSecrets = reactive<Record<string, boolean>>({});

async function copySecretName(name: string) {
  try {
    await navigator.clipboard.writeText(name);
    copiedSecrets[name] = true;
    setTimeout(() => {
      copiedSecrets[name] = false;
    }, 2000);
  } catch {
    // Failed to copy
  }
}

// Track which secret is showing its abbreviated value
const visibleSecretId = ref<string | number | null>(null);
const abbreviatedValues = ref<Record<string | number, string>>({});
const loadingSecretId = ref<string | number | null>(null);

async function toggleAbbreviatedValue(secret: Secret) {
  // If already showing, hide it
  if (visibleSecretId.value === secret.id) {
    visibleSecretId.value = null;
    return;
  }

  // If we already have the value cached, just show it
  if (secret.id && abbreviatedValues.value[secret.id]) {
    visibleSecretId.value = secret.id;
    return;
  }

  // Fetch the abbreviated value
  loadingSecretId.value = secret.id ?? null;
  try {
    let fetchedSecret;

    // Determine which API to call based on secret scope
    if (secret.repo_id && secret.repo_id !== 0) {
      fetchedSecret = await apiClient.getSecretAbbreviated(secret.repo_id, secret.name);
    } else if (secret.org_id && secret.org_id !== 0) {
      fetchedSecret = await apiClient.getOrgSecretAbbreviated(secret.org_id, secret.name);
    } else {
      fetchedSecret = await apiClient.getGlobalSecretAbbreviated(secret.name);
    }

    if (secret.id) {
      abbreviatedValues.value[secret.id] = fetchedSecret.value;
      visibleSecretId.value = secret.id;
    }
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('secrets.error_viewing_value'),
      type: 'error',
    });
  } finally {
    loadingSecretId.value = null;
  }
}

function editSecret(secret: Secret) {
  emit('edit', secret);
}

function deleteSecret(secret: Secret) {
  emit('delete', secret);
}
</script>
