<template>
  <Dialog v-model:open="isOpen">
    <DialogContent class="max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ $t('keyboard_shortcuts.title') }}</DialogTitle>
        <DialogDescription>{{ $t('keyboard_shortcuts.description') }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 max-h-[60vh] overflow-y-auto pr-1">
        <div v-for="group in shortcutGroups" :key="group.label">
          <h3 class="text-sm font-medium text-muted-foreground mb-2">{{ group.label }}</h3>
          <div class="space-y-1">
            <div
              v-for="shortcut in group.shortcuts"
              :key="shortcut.label"
              class="flex items-center justify-between py-1.5 px-2 rounded-md hover:bg-muted/50"
            >
              <span class="text-sm text-foreground">{{ shortcut.label }}</span>
              <KbdGroup>
                <Kbd v-for="(key, i) in shortcut.keys" :key="i">{{ key }}</Kbd>
              </KbdGroup>
            </div>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Kbd } from '~/components/ui/kbd';
import KbdGroup from '~/components/ui/kbd/KbdGroup.vue';
import { useKeyboardShortcutsDialog } from '~/compositions/useKeyboardShortcutsDialog';

const { t } = useI18n();
const { isOpen } = useKeyboardShortcutsDialog();

const isMac = navigator.userAgent.includes('Mac');
const metaKey = isMac ? '⌘' : 'Ctrl';

const shortcutGroups = computed(() => [
  {
    label: t('keyboard_shortcuts.general'),
    shortcuts: [
      { label: t('command_palette.title'), keys: [metaKey, 'K'] },
      { label: t('keyboard_shortcuts.show_shortcuts'), keys: ['?'] },
    ],
  },
  {
    label: t('keyboard_shortcuts.navigation'),
    shortcuts: [
      { label: t('repositories.title'), keys: ['Alt', 'R'] },
      { label: t('admin.settings.agents.agents'), keys: ['Alt', 'A'] },
      { label: t('admin.settings.autoscalers.autoscalers'), keys: ['Alt', 'U'] },
      { label: t('secrets.secrets'), keys: ['Alt', 'S'] },
      { label: t('registries.registries'), keys: ['Alt', 'Shift', 'R'] },
      { label: t('access_tokens.title'), keys: ['Alt', 'T'] },
      { label: t('repo.settings.crons.crons'), keys: ['Alt', 'C'] },
      { label: t('metrics.title'), keys: ['Alt', 'M'] },
      { label: t('integrations.title'), keys: ['Alt', 'I'] },
    ],
  },
]);
</script>
