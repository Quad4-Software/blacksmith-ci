<template>
  <TooltipProvider>
    <Tooltip>
      <TooltipTrigger as-child>
        <span
          class="shrink-0 inline-flex text-muted-foreground"
          role="img"
          :aria-label="$t('repo.pipeline.cache_step_tooltip')"
          data-testid="cache-step-marker"
        >
          <Icon name="cache" :size="size" />
        </span>
      </TooltipTrigger>
      <TooltipContent>
        {{ $t('repo.pipeline.cache_step_tooltip') }}
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>

<script lang="ts" setup>
import Icon from '~/components/atomic/Icon.vue';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '~/components/ui/tooltip';

// Marks the steps the compiler adds for a workflow's `cache` section.
withDefaults(defineProps<{ size?: string }>(), { size: '0.85rem' });
</script>
