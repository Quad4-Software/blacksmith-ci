<template>
  <div class="space-y-3 text-foreground">
    <Card v-for="registry in registries" :key="registry.id" class="hover:shadow-md transition-shadow duration-200">
      <div class="flex items-center gap-3 p-2">
        <span class="font-medium flex-1 min-w-0">{{ registry.address }}</span>
        <div class="flex items-center gap-1 shrink-0">
          <Button
            variant="ghost"
            size="icon"
            :title="registry.readonly ? $t('registries.view') : $t('registries.edit')"
            @click="editRegistry(registry)"
          >
            <Icon :name="registry.readonly ? 'chevron-right' : 'edit'" class="h-4 w-4" />
          </Button>
          <AlertDialog v-if="!registry.readonly">
            <AlertDialogTrigger as-child>
              <Button variant="ghost" size="icon" :title="$t('registries.delete')" :disabled="isDeleting">
                <Icon v-if="!isDeleting" name="trash" class="h-4 w-4 text-destructive" />
                <Icon v-else name="loading" class="h-4 w-4 animate-spin" />
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{{ $t('registries.delete') }}</AlertDialogTitle>
                <AlertDialogDescription>
                  {{ $t('registries.delete_confirm') }}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{{ $t('cancel') }}</AlertDialogCancel>
                <AlertDialogAction @click="deleteRegistry(registry)">
                  {{ $t('delete') }}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>
    </Card>

    <!-- Empty State -->
    <div v-if="registries?.length === 0" class="text-center py-12 text-muted-foreground">
      <Icon name="docker" class="h-12 w-12 mx-auto mb-3 opacity-50" />
      <p class="text-base">{{ $t('registries.none') }}</p>
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
import { Button } from '~/components/ui/button';
import Card from '~/components/ui/card/Card.vue';
import type { Registry } from '~/lib/api/types';

const props = defineProps<{
  modelValue: (Registry & { edit?: boolean })[];
  isDeleting: boolean;
}>();

const emit = defineEmits<{
  (event: 'edit', registry: Registry): void;
  (event: 'delete', registry: Registry): void;
}>();

const registries = toRef(props, 'modelValue');

function editRegistry(registry: Registry) {
  emit('edit', registry);
}

function deleteRegistry(registry: Registry) {
  emit('delete', registry);
}
</script>
