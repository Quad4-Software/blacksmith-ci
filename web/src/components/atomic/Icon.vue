<!-- cSpell:ignore radiobox timelapse -->
<template>
  <component :is="phosphorIcon" v-if="phosphorIcon" :size="size || '1em'" />
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

import { phosphorIcons } from './phosphor-icons';

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

// Map semantic names to bundled Phosphor icons.
const iconMap: Record<string, keyof typeof phosphorIcons> = {
  duration: 'ph:timer',
  startDate: 'ph:clock',
  branch: 'ph:git-branch',
  'pull-request': 'ph:git-pull-request',
  'pull-request-closed': 'ph:git-pull-request',
  'pull-request-merged': 'ph:git-merge',
  'pull-request-edited': 'ph:pencil-simple',
  'manual-pipeline': 'ph:hand',
  tag: 'ph:tag',
  deployment: 'ph:package',
  commit: 'ph:git-commit',
  back: 'ph:arrow-left',
  next: 'ph:arrow-right',
  repo: 'ph:folder-simple',
  settings: 'ph:gear',
  settings2: 'ph:sliders-horizontal',
  'settings-outline': 'ph:sliders-horizontal',
  trash: 'ph:trash',
  'status-blocked': 'ph:play',
  'status-declined': 'ph:square',
  'status-killed': 'ph:square',
  folder: 'ph:folder',
  'folder-open': 'ph:folder-open',
  file: 'ph:file',
  filter: 'ph:funnel',
  columns: 'ph:columns',
  'status-failure': 'ph:x',
  'status-error': 'ph:x',
  'status-skipped': 'ph:minus',
  'status-success': 'ph:check',
  'status-pending': 'ph:circle',
  pipelines: 'ph:list-checks',
  secret: 'ph:key',
  console: 'ph:code',
  agent: 'ph:robot',
  autoscaler: 'ph:scales',
  worker: 'ph:hard-hat',
  info: 'ph:info',
  'info-outline': 'ph:info',
  user: 'ph:user',
  org: 'ph:users',
  cron: 'ph:calendar-dots',
  toolbox: 'ph:wrench',
  alert: 'ph:warning',
  question: 'ph:info',
  plus: 'ph:plus',
  list: 'ph:list',
  heal: 'ph:sliders-horizontal',
  'turn-off': 'ph:power',
  'chevron-left': 'ph:caret-left',
  'chevron-right': 'ph:caret-right',
  'chevron-up': 'ph:caret-up',
  'chevron-down': 'ph:caret-down',
  'sort-ascending': 'ph:arrow-up',
  'sort-descending': 'ph:arrow-down',
  close: 'ph:x',
  edit: 'ph:pencil-simple',
  'content-save-outline': 'ph:floppy-disk',
  download: 'ph:download',
  stopwatch: 'ph:alarm',
  'auto-scroll': 'ph:eye',
  'auto-scroll-off': 'ph:eye-slash',
  pause: 'ph:pause',
  play: 'ph:play',
  'play-outline': 'ph:play',
  'progress-wrench-outline': 'ph:wrench',
  dots: 'ph:dots-three-vertical',
  tasks: 'ph:tree-structure',
  'file-cog-outline': 'ph:file-code',
  'file-edit-outline': 'ph:note-pencil',
  'bug-outline': 'ph:bug',
  docker: 'ph:package',
  'visibility-private': 'ph:lock',
  'visibility-internal': 'ph:lock-open',
  dependency: 'ph:git-merge',
  'mdi-wind-power-outline': 'ph:wind',
  'email-outline': 'ph:envelope',
  graph: 'ph:chart-bar',
  spinner: 'ph:spinner-gap',
  logout: 'ph:sign-out',
  calendar: 'ph:calendar',
  cache: 'ph:database',
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

const phosphorIcon = computed(() => {
  const name = iconMap[props.name] ?? props.name;
  return phosphorIcons[name as keyof typeof phosphorIcons] ?? null;
});

const iconifyIcon = computed(() => {
  if (iconifyForgeIcons[props.name]) return iconifyForgeIcons[props.name];
  // Handle any iconify icon name with a colon prefix (e.g. "solar:restart-bold")
  if (props.name.includes(':') && !props.name.startsWith('ph:')) return props.name;
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
