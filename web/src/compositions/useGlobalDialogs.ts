import { ref } from 'vue';
import type { Ref } from 'vue';

interface DialogState {
  isOpen: Ref<boolean>;
  open: () => void;
  close: () => void;
}

// Global dialog states - singleton pattern
const secretDialogOpen = ref(false);
const registryDialogOpen = ref(false);
const agentDialogOpen = ref(false);
const repositoryDialogOpen = ref(false);
const cronDialogOpen = ref(false);

function createDialogState(openRef: Ref<boolean>): DialogState {
  return {
    isOpen: openRef,
    open: () => {
      openRef.value = true;
    },
    close: () => {
      openRef.value = false;
    },
  };
}

export function useGlobalDialogs() {
  return {
    secretDialog: createDialogState(secretDialogOpen),
    registryDialog: createDialogState(registryDialogOpen),
    agentDialog: createDialogState(agentDialogOpen),
    repositoryDialog: createDialogState(repositoryDialogOpen),
    cronDialog: createDialogState(cronDialogOpen),
  };
}
