<!-- cSpell:ignore radiobox timelapse -->
<template>
  <Icon v-if="lucideIcon" :icon="`lucide:${lucideIcon}`" :width="size || '1em'" :height="size || '1em'" />
  <Icon
    v-else-if="iconifyIcon"
    :icon="iconifyIcon"
    :width="size || '1.3rem'"
    :height="size || '1.3rem'"
    :style="iconifyIconStyle"
  />
  <img
    v-else-if="forgeIcon"
    :src="getForgeIcon(forgeIcon)"
    :style="{ width: size || '1.3rem', height: size || '1.3rem' }"
    :alt="forgeIcon"
  />
  <div
    v-else-if="name === 'status-running' || name === 'status-started'"
    :style="{ width: size || '1.3rem', height: size || '1.3rem' }"
  >
    <PipelineProgressSpinner />
  </div>
  <svg v-else-if="name === 'spinner'" width="24" height="24" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
    <path
      fill="currentColor"
      d="M12,1A11,11,0,1,0,23,12,11,11,0,0,0,12,1Zm0,19a8,8,0,1,1,8-8A8,8,0,0,1,12,20Z"
      opacity=".25"
    />

    <path
      fill="currentColor"
      d="M12,4a8,8,0,0,1,7.89,6.7A1.53,1.53,0,0,0,21.38,12h0a1.5,1.5,0,0,0,1.48-1.75,11,11,0,0,0-21.72,0A1.5,1.5,0,0,0,2.62,12h0a1.53,1.53,0,0,0,1.49-1.3A8,8,0,0,1,12,4Z"
    >
      <animateTransform
        attributeName="transform"
        type="rotate"
        dur="0.75s"
        values="0 12 12;360 12 12"
        repeatCount="indefinite"
      />
    </path>
  </svg>
  <div v-else-if="name === 'blank'" class="h-6 w-6" />
</template>

<script lang="ts" setup>
import { computed } from 'vue';
import { Icon } from '@iconify/vue';

import PipelineProgressSpinner from '~/components/repo/pipeline/PipelineProgressSpinner.vue';
import { useTheme } from '~/compositions/useTheme';

import type { IconNames } from './Icon.types';

// Preload critical forge icons
import githubLight from '~/assets/forge-icons/github-light.png';
import githubDark from '~/assets/forge-icons/github-dark.png';
import gitlabLight from '~/assets/forge-icons/gitlab-light.png';
import gitlabDark from '~/assets/forge-icons/gitlab-dark.png';
import bitbucketLight from '~/assets/forge-icons/bitbucket-light.png';
import bitbucketDark from '~/assets/forge-icons/bitbucket-dark.png';
import forgejoLight from '~/assets/forge-icons/forgejo-light.png';
import forgejoDark from '~/assets/forge-icons/forgejo-dark.png';
import codebergLight from '~/assets/forge-icons/codeberg-light.png';
import codebergDark from '~/assets/forge-icons/codeberg-dark.png';
import codefloeLight from '~/assets/forge-icons/codefloe-light.png';
import codefloeDark from '~/assets/forge-icons/codefloe-dark.png';

const props = defineProps<{
  name: IconNames;
  size?: string;
}>();

const { theme } = useTheme();

// Mapping of icon names to Lucide icon names
const iconMap: Record<string, string> = {
  duration: 'timer',
  startDate: 'clock',
  branch: 'git-branch',
  'pull-request': 'git-pull-request',
  'pull-request-closed': 'git-pull-request-closed',
  'pull-request-merged': 'git-merge',
  'pull-request-edited': 'pencil',
  'manual-pipeline': 'hand',
  tag: 'tag',
  deployment: 'package',
  commit: 'git-commit',
  back: 'arrow-left',
  next: 'arrow-right',
  repo: 'folder-code',
  settings: 'settings',
  settings2: 'settings-2',
  'settings-outline': 'settings-2',
  trash: 'trash-2',
  'status-blocked': 'play',
  'status-declined': 'square',
  'status-killed': 'square',
  folder: 'folder',
  'folder-open': 'folder-open',
  file: 'file',
  filter: 'list-filter',
  columns: 'columns-3',
  'status-failure': 'x',
  'status-error': 'x',
  'status-skipped': 'minus',
  'status-success': 'check',
  'status-pending': 'circle',
  pipelines: 'list-checks',
  secret: 'key-round',
  console: 'code',
  agent: 'bot',
  autoscaler: 'scale',
  worker: 'hard-hat',
  info: 'info',
  'info-outline': 'info',
  user: 'user',
  org: 'users',
  cron: 'calendar-clock',
  toolbox: 'wrench',
  alert: 'triangle-alert',
  question: 'info',
  plus: 'plus',
  list: 'list',
  heal: 'settings-2',
  'turn-off': 'power',
  'chevron-left': 'chevron-left',
  'chevron-right': 'chevron-right',
  'chevron-up': 'chevron-up',
  'chevron-down': 'chevron-down',
  'sort-ascending': 'arrow-up',
  'sort-descending': 'arrow-down',
  close: 'x',
  edit: 'pencil',
  'content-save-outline': 'save',
  download: 'download',
  stopwatch: 'alarm-clock',
  'auto-scroll': 'eye',
  'auto-scroll-off': 'eye-off',
  pause: 'pause',
  play: 'play',
  'play-outline': 'play',
  'progress-wrench-outline': 'wrench',
  dots: 'more-vertical',
  tasks: 'list-tree',
  'file-cog-outline': 'file-code',
  'file-edit-outline': 'file-pen-line',
  'bug-outline': 'bug',
  docker: 'package',
  'visibility-private': 'lock',
  'visibility-internal': 'lock-open',
  'mdi-wind-power-outline': 'wind',
  'email-outline': 'mail',
  graph: 'chart-bar',
  spinner: 'loader-2',
  logout: 'log-out',
  calendar: 'calendar',
};

const forgeIcons = {
  github: { light: githubLight, dark: githubDark },
  gitlab: { light: gitlabLight, dark: gitlabDark },
  bitbucket: { light: bitbucketLight, dark: bitbucketDark },
  forgejo: { light: forgejoLight, dark: forgejoDark },
  codeberg: { light: codebergLight, dark: codebergDark },
  codefloe: { light: codefloeLight, dark: codefloeDark },
};

// Forge icons that use PNG images
const forgeIconNames = ['github', 'gitlab', 'bitbucket', 'bitbucket-dc', 'forgejo', 'codeberg', 'codefloe'];

// Forge icons that use Iconify (for icons that need proper SVG scaling)
const iconifyForgeIcons: Record<string, string> = {
  gitea: 'devicon:gitea',
};

const lucideIcon = computed(() => {
  if (props.name === 'status-running' || props.name === 'status-started' || props.name === 'blank') {
    return null;
  }
  if (forgeIconNames.includes(props.name) || props.name in iconifyForgeIcons) {
    return null;
  }
  // Handle direct lucide icon names
  if (props.name.startsWith('lucide:')) {
    return props.name.replace('lucide:', '');
  }
  return iconMap[props.name] || null;
});

const iconifyIcon = computed(() => {
  if (iconifyForgeIcons[props.name]) return iconifyForgeIcons[props.name];
  // Handle any iconify icon name with a colon prefix (e.g. "solar:restart-bold")
  if (props.name.includes(':') && !props.name.startsWith('lucide:')) return props.name;
  return null;
});

// Brand colors for Iconify forge icons (devicon icons are already colored)
const iconifyIconColors: Record<string, string> = {};

const iconifyIconStyle = computed(() => {
  const color = iconifyIconColors[props.name];
  return color ? { color } : {};
});

const forgeIcon = computed(() => {
  if (forgeIconNames.includes(props.name)) {
    return props.name === 'bitbucket-dc' ? 'bitbucket' : props.name;
  }
  return null;
});

function getForgeIcon(forgeName: string): string {
  // Fallback theme detection if useTheme is not ready
  const themeMode = theme.value === 'dark' ? 'dark' : 'light';
  const fallbackTheme =
    typeof document !== 'undefined' && document?.documentElement?.classList.contains('dark') ? 'dark' : 'light';
  const finalTheme = theme.value ? themeMode : fallbackTheme;

  return (
    forgeIcons[forgeName as keyof typeof forgeIcons]?.[finalTheme] ||
    forgeIcons[forgeName as keyof typeof forgeIcons]?.light ||
    ''
  );
}
</script>
