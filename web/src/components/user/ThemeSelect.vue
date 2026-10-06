<template>
  <Select :id="id" v-model="model">
    <SelectTrigger class="w-full md:w-[200px]">
      <SelectValue />
    </SelectTrigger>
    <SelectContent>
      <SelectGroup>
        <SelectItem v-for="theme in bundledThemes" :key="theme.id" :value="theme.id">
          {{ theme.name }}
        </SelectItem>
      </SelectGroup>
      <template v-if="customThemes.length > 0">
        <SelectSeparator />
        <SelectGroup>
          <SelectLabel class="text-xs text-muted-foreground">
            {{ $t('user.settings.general.instance_themes') }}
          </SelectLabel>
          <SelectItem v-for="theme in customThemes" :key="theme.id" :value="theme.id">
            {{ theme.name }}
          </SelectItem>
        </SelectGroup>
      </template>
    </SelectContent>
  </Select>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from '~/components/ui/select';
import type { ThemeOption } from '~/lib/themes';

const props = defineProps<{
  id?: string;
  themes: ThemeOption[];
}>();

const model = defineModel<string>({ required: true });

// Admin-provided themes, including ones replacing a bundled theme, are listed
// separately so users can tell them apart from the themes Crow ships.
const bundledThemes = computed(() => props.themes.filter((theme) => theme.custom !== true));
const customThemes = computed(() => props.themes.filter((theme) => theme.custom === true));
</script>
