<template>
  <Dialog v-model:open="cronDialog.isOpen.value">
    <DialogContent class="sm:max-w-[500px]">
      <DialogHeader>
        <DialogTitle>{{ $t('repo.settings.crons.add') }}</DialogTitle>
        <DialogDescription>
          {{ $t('repo.settings.crons.desc') }}
        </DialogDescription>
      </DialogHeader>

      <form class="space-y-4" @submit.prevent="handleSave">
        <!-- Repository Selection -->
        <div class="space-y-2">
          <Label>{{ $t('repo.settings.crons.repo') }}</Label>
          <Popover v-model:open="repoPopoverOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" role="combobox" class="w-full justify-between">
                <span v-if="selectedRepoDisplay" class="flex items-center gap-2 flex-1 justify-between">
                  <span>{{ selectedRepoDisplay }}</span>
                  <button
                    type="button"
                    class="h-4 w-4 shrink-0 opacity-50 hover:opacity-100 flex items-center justify-center"
                    @click.stop.prevent="clearRepoSelection"
                  >
                    <Icon name="close" class="h-4 w-4" />
                  </button>
                </span>
                <span v-else class="text-muted-foreground">{{ $t('repo.settings.crons.select_repo') }}</span>
                <Icon name="chevron-down" class="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-[400px] p-0" align="start">
              <Command v-model:search-term="repoSearchTerm">
                <CommandInput :placeholder="$t('repo.settings.crons.search_repo')" />
                <CommandList>
                  <CommandEmpty>{{ $t('no_search_results') }}</CommandEmpty>
                  <CommandGroup>
                    <!-- Column headers when multi-forge -->
                    <div
                      v-if="showForgeInfo"
                      class="grid grid-cols-2 gap-2 px-2 py-1 text-xs text-muted-foreground border-b"
                    >
                      <span>{{ $t('scope.repo') }}</span>
                      <span>{{ $t('forge') }}</span>
                    </div>
                    <CommandItem
                      v-for="repo in filteredRepos"
                      :key="repo.id"
                      :value="
                        showForgeInfo && getRepoForgeInfo(repo)
                          ? `${repo.owner}/${repo.name} ${getRepoForgeInfo(repo)?.hostname}`
                          : `${repo.owner}/${repo.name}`
                      "
                      @select="() => selectRepo(repo)"
                    >
                      <div v-if="showForgeInfo" class="grid grid-cols-2 gap-2 w-full items-center">
                        <span class="truncate">{{ `${repo.owner}/${repo.name}` }}</span>
                        <span
                          v-if="getRepoForgeInfo(repo)"
                          class="flex items-center gap-1 text-xs text-muted-foreground"
                        >
                          <img
                            v-if="getRepoForgeInfo(repo)?.customUrl"
                            :src="getRepoForgeInfo(repo)?.customUrl!"
                            class="h-3 w-3 shrink-0"
                            alt=""
                          />
                          <Icon v-else :name="getRepoForgeInfo(repo)?.icon || 'repo'" class="h-3 w-3 shrink-0" />
                          <span class="truncate">{{ getRepoForgeInfo(repo)?.hostname }}</span>
                        </span>
                      </div>
                      <span v-else>{{ `${repo.owner}/${repo.name}` }}</span>
                    </CommandItem>
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
        </div>

        <!-- Name Field -->
        <div class="space-y-2">
          <Label for="cron-name">{{ $t('repo.settings.crons.name.name') }}</Label>
          <Input
            id="cron-name"
            v-model="cronForm.name"
            :placeholder="$t('repo.settings.crons.name.placeholder')"
            required
          />
        </div>

        <!-- Branch Field -->
        <div class="space-y-2">
          <Label for="cron-branch">{{ $t('repo.settings.crons.branch.title') }}</Label>
          <Input
            id="cron-branch"
            v-model="cronForm.branch"
            :placeholder="$t('repo.settings.crons.branch.placeholder')"
          />
        </div>

        <!-- Schedule Field -->
        <div class="space-y-2">
          <div class="flex items-center gap-2">
            <Label for="cron-schedule">{{ $t('repo.settings.crons.schedule.title') }}</Label>
            <a
              href="https://pkg.go.dev/github.com/gdgvda/cron#hdr-CRON_Expression_Format"
              target="_blank"
              rel="noopener noreferrer"
              class="text-xs text-muted-foreground hover:text-foreground"
            >
              <Icon name="lucide:external-link" class="h-3 w-3" />
            </a>
          </div>
          <Input
            id="cron-schedule"
            v-model="cronForm.schedule"
            :placeholder="$t('repo.settings.crons.schedule.placeholder')"
            required
          />
          <p class="text-xs text-muted-foreground">
            {{ $t('repo.settings.crons.timezone_note') }}
          </p>
        </div>

        <div class="flex justify-end gap-2 pt-4">
          <Button type="button" variant="outline" @click="handleCancel">
            {{ $t('cancel') }}
          </Button>
          <Button type="submit" :disabled="isSaving || !selectedRepoId">
            <Icon v-if="isSaving" name="spinner" class="size-4 animate-spin mr-2" />
            {{ $t('repo.settings.crons.add') }}
          </Button>
        </div>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';

import Icon from '~/components/atomic/Icon.vue';
import { Button } from '~/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '~/components/ui/command';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '~/components/ui/popover';
import useApiClient from '~/compositions/useApiClient';
import { useAsyncAction } from '~/compositions/useAsyncAction';
import { useForges } from '~/compositions/useForges';
import { useGlobalDialogs } from '~/compositions/useGlobalDialogs';
import useNotifications from '~/compositions/useNotifications';
import type { IconNames } from '~/components/atomic/Icon.types';
import type { Cron, Repo } from '~/lib/api/types';

const { t } = useI18n();
const apiClient = useApiClient();
const notifications = useNotifications();
const { cronDialog } = useGlobalDialogs();
const { forges, getForgeById } = useForges();

// Show forge info when multiple forges are configured
const showForgeInfo = computed(() => forges.value.length > 1);

// Helper to check if a string is a valid URL
function isValidUrl(str: string): boolean {
  try {
    return Boolean(new URL(str));
  } catch {
    return false;
  }
}

// Helper to get forge icon name from forge info
function getForgeIconName(forgeIcon?: string, forgeType?: string, forgeName?: string): IconNames {
  if (forgeIcon && !isValidUrl(forgeIcon)) {
    const customIcon = forgeIcon.toLowerCase();
    const validIcons: Record<string, IconNames> = {
      codeberg: 'codeberg',
      codefloe: 'codefloe',
      github: 'github',
      gitlab: 'gitlab',
      gitea: 'gitea',
      forgejo: 'forgejo',
      bitbucket: 'bitbucket',
      repo: 'repo',
    };
    if (validIcons[customIcon]) {
      return validIcons[customIcon];
    }
  }
  if (forgeName) {
    const lowerName = forgeName.toLowerCase();
    if (lowerName.includes('codeberg')) return 'codeberg';
    if (lowerName.includes('codefloe')) return 'codefloe';
  }
  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forgeType || ''] || 'repo';
}

// Helper to get custom icon URL if forge_icon is a URL
function getForgeCustomIconUrl(forgeIcon?: string): string | null {
  if (forgeIcon && isValidUrl(forgeIcon)) {
    return forgeIcon;
  }
  return null;
}

// Helper to get forge display info for a repo
function getRepoForgeInfo(repo: Repo): { icon: IconNames; customUrl: string | null; hostname: string } | null {
  if (!showForgeInfo.value) return null;
  const forge = getForgeById(repo.forge_id);
  if (!forge) return null;
  try {
    const url = new URL(forge.url);
    return {
      icon: getForgeIconName(forge.icon, forge.type, url.hostname),
      customUrl: getForgeCustomIconUrl(forge.icon),
      hostname: url.hostname,
    };
  } catch {
    return null;
  }
}

const repos = ref<Repo[]>([]);
const selectedRepoId = ref<number | null>(null);
const repoPopoverOpen = ref(false);
const repoSearchTerm = ref('');

const cronForm = ref<Partial<Cron>>({
  name: '',
  branch: '',
  schedule: '@daily',
});

// Computed property for selected repo display
const selectedRepoDisplay = computed(() => {
  if (!selectedRepoId.value) return '';
  const repo = repos.value.find((r) => r.id === selectedRepoId.value);
  return repo ? `${repo.owner}/${repo.name}` : '';
});

// Filter repos based on search term
const filteredRepos = computed(() => {
  if (!repoSearchTerm.value) return repos.value;
  const search = repoSearchTerm.value.toLowerCase();
  return repos.value.filter((repo) => `${repo.owner}/${repo.name}`.toLowerCase().includes(search));
});

function selectRepo(repo: Repo) {
  selectedRepoId.value = repo.id;
  repoPopoverOpen.value = false;
  repoSearchTerm.value = '';
}

function clearRepoSelection() {
  selectedRepoId.value = null;
  repoSearchTerm.value = '';
}

// Load repos when dialog opens
async function loadRepos() {
  try {
    const reposData = await apiClient.getActiveRepos();
    repos.value = reposData || [];
  } catch {
    // Silent fail - repos will remain empty
  }
}

// Watch for dialog open/close
watch(
  () => cronDialog.isOpen.value,
  (open) => {
    if (open) {
      loadRepos();
    } else {
      // Reset form when dialog closes
      resetForm();
    }
  },
);

function resetForm() {
  cronForm.value = {
    name: '',
    branch: '',
    schedule: '@daily',
  };
  selectedRepoId.value = null;
  repoSearchTerm.value = '';
}

const { doSubmit: handleSave, isLoading: isSaving } = useAsyncAction(async () => {
  // Safety checks (button is disabled when these fail, but validate anyway)
  if (!selectedRepoId.value || !cronForm.value.name || !cronForm.value.schedule) {
    return;
  }

  await apiClient.createCron(selectedRepoId.value, cronForm.value);

  notifications.notify({
    title: t('notify_title_success'),
    text: t('repo.settings.crons.notify_text_created'),
    type: 'success',
  });

  resetForm();
  cronDialog.close();
});

function handleCancel() {
  resetForm();
  cronDialog.close();
}
</script>
