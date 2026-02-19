<template>
  <Card>
    <div class="p-2">
      <!-- Header Row -->
      <div class="flex items-start justify-between mb-3">
        <div class="flex items-center gap-3 flex-1 min-w-0">
          <!-- Status dot -->
          <Skeleton class="shrink-0 w-3 h-3 rounded-full" />
          <!-- Name and metadata -->
          <div class="min-w-0 flex-1">
            <Skeleton class="h-5 w-40 mb-2" />
            <Skeleton class="h-3 w-28" />
          </div>
        </div>
        <!-- Action buttons placeholder -->
        <div class="hidden md:flex items-center gap-1 shrink-0 ml-3">
          <Skeleton class="h-8 w-8 rounded-md" />
          <Skeleton class="h-8 w-8 rounded-md" />
        </div>
      </div>

      <!-- Info Grid -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-3">
        <div v-for="i in 4" :key="i" class="flex flex-col">
          <Skeleton class="h-3 w-16 mb-1.5" />
          <Skeleton class="h-4 w-24" />
        </div>
      </div>

      <!-- Labels section -->
      <Separator class="my-3" />
      <div class="flex flex-wrap items-center gap-2">
        <Skeleton class="h-3 w-12 mr-1" />
        <Skeleton class="h-6 w-16 rounded-full" />
        <Skeleton class="h-6 w-24 rounded-full" />
        <Skeleton class="h-6 w-20 rounded-full" />
      </div>
    </div>
  </Card>
</template>

<script lang="ts" setup>
import Card from '~/components/ui/card/Card.vue';
import { Separator } from '~/components/ui/separator';
import { Skeleton } from '~/components/ui/skeleton';
</script>
