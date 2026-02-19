<template>
  <div class="space-y-3">
    <Card v-for="i in count" :key="i">
      <div class="flex items-center gap-3 p-2">
        <!-- Name and badges -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <Skeleton class="h-5 w-32" />
            <Skeleton class="h-5 w-5 rounded-md" />
          </div>
          <!-- Event badges -->
          <div class="hidden md:flex items-center gap-2 mt-2">
            <Skeleton class="h-5 w-14 rounded-full" />
            <Skeleton class="h-5 w-20 rounded-full" />
          </div>
        </div>
        <!-- Action buttons -->
        <div class="flex items-center gap-1 shrink-0">
          <Skeleton class="h-8 w-8 rounded-md" />
          <Skeleton class="h-8 w-8 rounded-md" />
          <Skeleton class="h-8 w-8 rounded-md" />
        </div>
      </div>
      <!-- Mobile event badges -->
      <div class="flex md:hidden items-center gap-2 px-2 pb-2 pt-0">
        <Skeleton class="h-5 w-14 rounded-full" />
        <Skeleton class="h-5 w-20 rounded-full" />
      </div>
    </Card>
  </div>
</template>

<script lang="ts" setup>
import Card from '~/components/ui/card/Card.vue';
import { Skeleton } from '~/components/ui/skeleton';

withDefaults(
  defineProps<{
    count?: number;
  }>(),
  {
    count: 3,
  },
);
</script>
