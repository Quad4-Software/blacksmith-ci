<template>
  <CommandDialog v-model:open="isOpen">
    <template #default>
      <CommandInput :placeholder="$t('command_palette.search_placeholder')" />
      <CommandList>
        <CommandEmpty>{{ $t('command_palette.no_results') }}</CommandEmpty>

        <!-- Go To section -->
        <CommandGroup :heading="$t('command_palette.go_to')">
          <CommandItem
            v-for="repo in displayedRepos"
            :key="`repo-${repo.id}`"
            :value="`goto-repo-${repo.full_name}`"
            @select="() => goToRepo(repo)"
          >
            <Icon name="repo" class="mr-2 h-4 w-4" />
            <span>{{ repo.full_name }}</span>
          </CommandItem>
          <CommandItem
            v-for="org in displayedOrgs"
            :key="`org-${org.id}`"
            :value="`goto-org-${org.name}`"
            @select="() => goToOrg(org)"
          >
            <Icon name="org" class="mr-2 h-4 w-4" />
            <span>{{ org.name }}</span>
          </CommandItem>
        </CommandGroup>

        <!-- Actions section -->
        <CommandGroup :heading="$t('command_palette.actions')">
          <CommandItem
            v-for="command in commands"
            :key="command.id"
            :value="command.id"
            @select="() => executeCommand(command.id)"
          >
            <Icon :name="command.icon" class="mr-2 h-4 w-4" />
            <span>{{ command.label }}</span>
          </CommandItem>
        </CommandGroup>
      </CommandList>
      <SearchStateWatcher @search-change="handleSearchChange" />
    </template>
  </CommandDialog>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  useCommand,
} from '~/components/ui/command';
import useApiClient from '~/compositions/useApiClient';
import { useCommandPalette } from '~/compositions/useCommandPalette';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import { useTheme } from '~/compositions/useTheme';
import type { Org, Repo } from '~/lib/api/types';

// Helper component to watch search state changes
const SearchStateWatcher = defineComponent({
  emits: ['searchChange'],
  setup(_, { emit }) {
    const commandContext = useCommand();
    if (commandContext) {
      watch(
        () => commandContext.filterState.search,
        (newSearch) => {
          emit('searchChange', newSearch);
        },
      );
    }
    return () => h('span', { style: { display: 'none' } });
  },
});

const { t } = useI18n();
const router = useRouter();
const apiClient = useApiClient();
const { isOpen, commands, registerCommand, executeCommand, initializeRouter } = useCommandPalette();
const { secretDialog, registryDialog, agentDialog, repositoryDialog, cronDialog } = useGlobalDialogs();
const { storeTheme } = useTheme();

const repos = ref<Repo[]>([]);
const orgs = ref<Org[]>([]);
const searchTerm = ref('');

const MAX_SUGGESTIONS = 3;

// Handle search state changes from the command input
function handleSearchChange(search: string) {
  searchTerm.value = search;
}

// Sort repos by most recent pipeline activity
const sortedRepos = computed(() => {
  return repos.value.toSorted((a, b) => {
    const aTime = a.last_pipeline?.created ?? 0;
    const bTime = b.last_pipeline?.created ?? 0;
    return bTime - aTime; // Most recent first
  });
});

// Show limited items when not searching, all items when searching
const displayedRepos = computed(() => {
  if (searchTerm.value.length > 0) {
    return repos.value;
  }
  const reposToShow = Math.min(sortedRepos.value.length, Math.max(0, MAX_SUGGESTIONS - orgs.value.length));
  return sortedRepos.value.slice(0, reposToShow);
});

const displayedOrgs = computed(() => {
  if (searchTerm.value.length > 0) {
    return orgs.value;
  }
  const remainingSlots = MAX_SUGGESTIONS - displayedRepos.value.length;
  return orgs.value.slice(0, remainingSlots);
});

// Load repos and orgs when command palette opens
watch(isOpen, async (open) => {
  if (open) {
    // Reset search term when opening
    searchTerm.value = '';

    if (repos.value.length === 0) {
      try {
        const [reposData, orgsData] = await Promise.all([apiClient.getActiveRepos(), apiClient.getUserOrgs()]);
        repos.value = reposData || [];
        orgs.value = orgsData || [];
      } catch {
        // Silent fail - navigation will still work with empty lists
      }
    }
  }
});

function goToRepo(repo: Repo) {
  isOpen.value = false;
  router.push({ name: 'repo', params: { repoId: repo.id } });
}

function goToOrg(org: Org) {
  isOpen.value = false;
  router.push({ name: 'org', params: { orgId: org.id } });
}

// Computed command labels for reactivity
const commandLabels = computed(() => ({
  addSecret: t('secrets.add'),
  addRepository: t('repo.add'),
  addRegistry: t('registries.add'),
  addAgent: t('admin.settings.agents.add'),
  addCron: t('repo.settings.crons.add'),
  toggleTheme: t('command_palette.toggle_theme'),
}));

// Get current theme label for display
const currentThemeLabel = computed(() => {
  switch (storeTheme.value) {
    case 'light':
      return t('user.settings.general.theme.light');
    case 'dark':
      return t('user.settings.general.theme.dark');
    default:
      return t('user.settings.general.theme.auto');
  }
});

// Get icon for current theme
const themeIcon = computed(() => {
  switch (storeTheme.value) {
    case 'light':
      return 'lucide:sun';
    case 'dark':
      return 'lucide:moon';
    default:
      return 'lucide:sun-moon';
  }
});

// Toggle theme: light -> dark -> auto -> light
function toggleTheme() {
  if (storeTheme.value === 'light') {
    storeTheme.value = 'dark';
  } else if (storeTheme.value === 'dark') {
    storeTheme.value = 'auto';
  } else {
    storeTheme.value = 'light';
  }
}

// Register all global commands on mount
onMounted(() => {
  // Initialize router for keyboard shortcuts
  initializeRouter(router);

  registerCommand({
    id: 'add-secret',
    get label() {
      return commandLabels.value.addSecret;
    },
    icon: 'plus',
    keywords: ['secret', 'add', 'create'],
    action: () => secretDialog.open(),
  });

  registerCommand({
    id: 'add-repository',
    get label() {
      return commandLabels.value.addRepository;
    },
    icon: 'plus',
    keywords: ['repository', 'repo', 'add', 'activate'],
    action: () => repositoryDialog.open(),
  });

  registerCommand({
    id: 'add-registry',
    get label() {
      return commandLabels.value.addRegistry;
    },
    icon: 'plus',
    keywords: ['registry', 'docker', 'add', 'create'],
    action: () => registryDialog.open(),
  });

  registerCommand({
    id: 'add-agent',
    get label() {
      return commandLabels.value.addAgent;
    },
    icon: 'plus',
    keywords: ['agent', 'add', 'create'],
    action: () => agentDialog.open(),
  });

  registerCommand({
    id: 'add-cron',
    get label() {
      return commandLabels.value.addCron;
    },
    icon: 'plus',
    keywords: ['cron', 'schedule', 'scheduled', 'job', 'add', 'create'],
    action: () => cronDialog.open(),
  });

  registerCommand({
    id: 'toggle-theme',
    get label() {
      return `${commandLabels.value.toggleTheme} (${currentThemeLabel.value})`;
    },
    get icon() {
      return themeIcon.value;
    },
    keywords: ['theme', 'dark', 'light', 'auto', 'system', 'mode', 'toggle', 'appearance'],
    action: toggleTheme,
  });
});
</script>
