<template>
  <div class="space-y-3 text-foreground">
    <Card v-for="secret in secrets" :key="secret.id" class="hover:shadow-md transition-shadow duration-200">
      <div class="flex items-center gap-3 p-2">
        <!-- Secret Name and Badges -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="font-medium">{{ secret.name }}</span>
            <Badge v-if="secret.edit === false" variant="secondary">
              {{ secret.org_id === 0 ? $t('global_level_secret') : $t('org_level_secret') }}
            </Badge>
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
import { toRef } from 'vue';

import Icon from '~/components/atomic/Icon.vue';
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
import type { Secret } from '~/lib/api/types';

const props = defineProps<{
  modelValue: (Secret & { edit?: boolean })[];
  isDeleting: boolean;
}>();

const emit = defineEmits<{
  (event: 'edit', secret: Secret): void;
  (event: 'delete', secret: Secret): void;
}>();

const secrets = toRef(props, 'modelValue');

function editSecret(secret: Secret) {
  emit('edit', secret);
}

function deleteSecret(secret: Secret) {
  emit('delete', secret);
}
</script>
