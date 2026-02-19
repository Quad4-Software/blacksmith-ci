import { onMounted, ref } from 'vue';
import type { Router } from 'vue-router';

import { useKeyboardShortcutsDialog } from '~/compositions/useKeyboardShortcutsDialog';

export interface Command {
  id: string;
  label: string;
  description?: string;
  icon: any; // Using any to avoid Icon type dependency
  keywords?: string[];
  action: () => void;
}

// Global state - singleton pattern
const isOpen = ref(false);
const commands = ref<Command[]>([]);
let isListenerRegistered = false;
let globalRouter: Router | null = null;

function handleKeyDown(e: KeyboardEvent) {
  // Don't trigger if user is typing in an input/textarea
  const target = e.target as HTMLElement;
  if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
    return;
  }

  // ? key -> Keyboard shortcuts help
  if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault();
    const { toggle: toggleShortcuts } = useKeyboardShortcutsDialog();
    toggleShortcuts();
    return;
  }

  // Cmd+K on Mac, Ctrl+K on Windows/Linux
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault();
    isOpen.value = !isOpen.value;
    return;
  }

  // Navigation shortcuts (Alt key combinations)
  // Use event.code instead of event.key for physical key detection
  if (e.altKey && !e.ctrlKey && !e.metaKey && globalRouter) {
    // Alt+R -> Repos
    if (e.code === 'KeyR' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'repos' });
      return;
    }

    // Alt+S -> Secrets
    if (e.code === 'KeyS' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'user-secrets' });
      return;
    }

    // Alt+Shift+R -> Registries
    if (e.code === 'KeyR' && e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'user-registries' });
      return;
    }

    // Alt+A -> Agents
    if (e.code === 'KeyA' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'agents' });
      return;
    }

    // Alt+M -> Metrics
    if (e.code === 'KeyM' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'user-metrics' });
      return;
    }

    // Alt+C -> Crons
    if (e.code === 'KeyC' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'user-crons' });
      return;
    }

    // Alt+U -> Autoscalers
    if (e.code === 'KeyU' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'autoscalers' });
      return;
    }

    // Alt+T -> Access Tokens
    if (e.code === 'KeyT' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'user-access-tokens' });
      return;
    }

    // Alt+I -> Integrations
    if (e.code === 'KeyI' && !e.shiftKey) {
      e.preventDefault();
      e.stopPropagation();
      void globalRouter.push({ name: 'user-integrations' });
    }
  }
}

export function useCommandPalette() {
  function open() {
    isOpen.value = true;
  }

  function close() {
    isOpen.value = false;
  }

  function toggle() {
    isOpen.value = !isOpen.value;
  }

  function registerCommand(command: Command) {
    // Remove if already exists (by id)
    const existingIndex = commands.value.findIndex((c) => c.id === command.id);
    if (existingIndex !== -1) {
      commands.value.splice(existingIndex, 1);
    }
    commands.value.push(command);
  }

  function executeCommand(commandId: string) {
    const command = commands.value.find((c) => c.id === commandId);
    if (command) {
      close();
      // Use setTimeout to ensure palette closes before action executes
      setTimeout(() => {
        command.action();
      }, 100);
    }
  }

  function initializeRouter(router: Router) {
    globalRouter = router;
  }

  // Register global keyboard listener only once
  onMounted(() => {
    if (!isListenerRegistered) {
      window.addEventListener('keydown', handleKeyDown);
      isListenerRegistered = true;
    }
  });

  return {
    isOpen,
    commands,
    open,
    close,
    toggle,
    registerCommand,
    executeCommand,
    initializeRouter,
  };
}
