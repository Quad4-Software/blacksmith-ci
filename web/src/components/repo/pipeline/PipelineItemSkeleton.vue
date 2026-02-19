<template>
  <div class="flex w-full overflow-hidden rounded-md border border-border bg-card p-0 text-card-foreground shadow-xs">
    <!-- Status icon placeholder -->
    <div class="flex w-11 shrink-0 flex-col items-center justify-center py-1">
      <Skeleton class="mx-2 md:mx-3 h-6 w-6 rounded-full" />
    </div>

    <div class="flex min-w-0 grow flex-wrap px-0 lg:px-4 py-2 pl-0.5! md:flex-nowrap">
      <!-- Avatar placeholder (desktop) -->
      <div class="hidden shrink-0 items-center md:flex">
        <Skeleton class="h-8 w-8 rounded-full" />
      </div>

      <!-- Title area -->
      <div class="flex min-w-0 flex-1 items-center md:mx-4">
        <Skeleton class="hidden md:block h-4 w-10 mr-2 shrink-0" />
        <Skeleton class="h-4 w-48 max-w-full" />
      </div>

      <!-- Metadata grid -->
      <div
        class="pipeline-meta grid w-full min-w-0 grid-cols-2 grid-rows-2 gap-x-3 gap-y-1 md:gap-y-2 py-2 text-xs md:text-sm md:ml-auto md:w-85 md:shrink-0"
      >
        <Skeleton class="h-4 w-24" />
        <Skeleton class="h-4 w-20" />
        <Skeleton class="h-4 w-16" />
        <Skeleton class="h-4 w-28" />
      </div>
    </div>

    <!-- Action buttons placeholder -->
    <div class="flex items-center pr-2 md:pr-4 py-2 shrink-0">
      <Skeleton class="h-7 w-7 rounded-md" />
    </div>
  </div>
</template>

<script lang="ts" setup>
import { Skeleton } from '~/components/ui/skeleton';
</script>

<style scoped>
@media (min-width: 1280px) {
  .pipeline-meta {
    grid-template-columns: 1.5fr 1fr 1fr 2fr;
    grid-template-rows: 1fr;
    width: 40rem;
  }
}
</style>
