<!-- cSpell:ignore radiobox timelapse -->
<template>
  <SvgIcon v-if="name === 'duration'" :path="mdiTimerOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'startDate'" :path="mdiClockTimeEightOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'branch'" :path="mdiSourceBranch" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'pull-request'" :path="mdiSourcePull" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'pull-request-closed'" :path="mdiSourceMerge" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'pull-request-edited'" :path="mdiPencilOutline" size="1.3rem" />
  <SvgIcon v-else-if="name === 'manual-pipeline'" :path="mdiPlayCircleOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'tag'" :path="mdiTagOutline" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'deployment'" :path="mdiPackageVariant" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'commit'" :path="mdiSourceCommit" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'back'" :path="mdiArrowLeft" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'next'" :path="mdiArrowRight" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'github'" :path="mdiGithub" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'repo'" :path="mdiGit" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'settings'" :path="mdiCog" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'settings-outline'" :path="mdiCogOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'trash'" :path="mdiTrashCanOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'status-blocked'" :path="mdiPlay" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'status-declined'" :path="mdiStop" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'status-killed'" :path="mdiStop" :size="size || '1.3rem'" />
  <SvgIcon
    v-else-if="name === 'status-failure' || name === 'status-error'"
    type="mdi"
    :path="mdiClose"
    :size="size || '1.3rem'"
  />
  <SvgIcon
    v-else-if="name === 'status-running' || name === 'status-started'"
    type="mdi"
    :path="mdiRadioboxIndeterminateVariant"
    :size="size || '1.3rem'"
  />
  <SvgIcon v-else-if="name === 'status-skipped'" :path="mdiMinus" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'status-success'" :path="mdiCheck" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'status-pending'" :path="mdiRadioboxBlank" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'pipelines'" :path="mdiFormatListCheckbox" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'secret'" :path="mdiShieldKeyOutline" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'console'" :path="mdiConsole" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'agent'" :path="mdiPlayNetworkOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'info'" :path="mdiInformationSlabCircleOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'info-outline'" :path="mdiInformationOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'user'" :path="mdiAccountOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'org'" :path="mdiAccountGroupOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'cron'" :path="mdiCalendarClockOutline" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'toolbox'" :path="mdiToolboxOutline" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'alert'" :path="mdiAlertCircle" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'question'" :path="mdiInformationSlabCircleOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'plus'" :path="mdiPlus" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'list'" :path="mdiFormatListBulleted" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'heal'" :path="mdiWrenchCogOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'turn-off'" :path="mdiPower" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'chevron-right'" :path="mdiChevronRight" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'close'" :path="mdiClose" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'edit'" :path="mdiPencilOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'download'" :path="mdiDownloadOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'stopwatch'" :path="mdiAlarm" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'auto-scroll'" :path="mdiEyeOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'auto-scroll-off'" :path="mdiEyeOffOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'pause'" :path="mdiPause" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'play'" :path="mdiPlay" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'play-outline'" :path="mdiPlayOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'dots'" :path="mdiDotsVertical" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'tasks'" :path="mdiFormatListBulletedType" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'file-cog-outline'" :path="mdiFileCogOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'file-edit-outline'" :path="mdiFileEditOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'bug-outline'" :path="mdiBugOutline" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'docker'" :path="mdiDocker" :size="size || '1rem'" />
  <SvgIcon v-else-if="name === 'visibility-private'" :path="mdiLockOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'visibility-internal'" :path="mdiLockOpenOutline" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'forgejo'" :path="siForgejo.path" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'gitea'" :path="siGitea.path" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'gitlab'" :path="mdiGitlab" :size="size || '1.3rem'" />
  <SvgIcon v-else-if="name === 'bitbucket' || name === 'bitbucket-dc'" :path="mdiBitbucket" :size="size || '1.3rem'" />
  <Icon
    v-else-if="name === 'graph'"
    icon="hugeicons:workflow-square-01"
    :width="size || '1.3rem'"
    :height="size || '1.3rem'"
  />
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
import {
  mdiAccountGroupOutline,
  mdiAccountOutline,
  mdiAlarm,
  mdiAlertCircle,
  mdiArrowLeft,
  mdiArrowRight,
  mdiBitbucket,
  mdiCheck,
  mdiBugOutline,
  mdiCalendarClockOutline,
  mdiChevronRight,
  mdiClockTimeEightOutline,
  mdiClose,
  mdiCog,
  mdiCogOutline,
  mdiConsole,
  mdiDocker,
  mdiDotsVertical,
  mdiDownloadOutline,
  mdiEyeOffOutline,
  mdiEyeOutline,
  mdiFileCogOutline,
  mdiFileEditOutline,
  mdiFormatListBulleted,
  mdiGit,
  mdiGithub,
  mdiGitlab,
  mdiInformationOutline,
  mdiInformationSlabCircleOutline,
  mdiLockOpenOutline,
  mdiLockOutline,
  mdiMinus,
  mdiPackageVariant,
  mdiPause,
  mdiPencilOutline,
  mdiPlay,
  mdiPlayNetworkOutline,
  mdiPlayOutline,
  mdiPlus,
  mdiPower,
  mdiRadioboxBlank,
  mdiRadioboxIndeterminateVariant,
  mdiShieldKeyOutline,
  mdiSourceBranch,
  mdiSourceCommit,
  mdiSourceMerge,
  mdiSourcePull,
  mdiStop,
  mdiTagOutline,
  mdiTimerOutline,
  mdiToolboxOutline,
  mdiTrashCanOutline,
  mdiFormatListBulletedType,
  mdiWrenchCogOutline,
  mdiFormatListCheckbox,
  mdiPlayCircleOutline,
} from '@mdi/js';
import { siForgejo, siGitea } from 'simple-icons';

import { Icon } from '@iconify/vue';
import SvgIcon from './SvgIcon.vue';

export type IconNames =
  | 'duration'
  | 'startDate'
  | 'branch'
  | 'pull-request'
  | 'pull-request-closed'
  | 'pull-request-edited'
  | 'manual-pipeline'
  | 'tag'
  | 'deployment'
  | 'commit'
  | 'back'
  | 'next'
  | 'github'
  | 'repo'
  | 'settings'
  | 'settings-outline'
  | 'trash'
  | 'status-blocked'
  | 'status-declined'
  | 'status-error'
  | 'status-failure'
  | 'status-killed'
  | 'status-pending'
  | 'status-running'
  | 'status-skipped'
  | 'status-started'
  | 'status-success'
  | 'gitea'
  | 'gitlab'
  | 'bitbucket'
  | 'bitbucket-dc'
  | 'forgejo'
  | 'question'
  | 'list'
  | 'loading'
  | 'plus'
  | 'blank'
  | 'heal'
  | 'chevron-right'
  | 'turn-off'
  | 'close'
  | 'edit'
  | 'stopwatch'
  | 'download'
  | 'auto-scroll'
  | 'auto-scroll-off'
  | 'play'
  | 'play-outline'
  | 'pause'
  | 'alert'
  | 'spinner'
  | 'visibility-private'
  | 'visibility-internal'
  | 'dots'
  | 'tasks'
  | 'file-cog-outline'
  | 'file-edit-outline'
  | 'bug-outline'
  | 'pipelines'
  | 'secret'
  | 'docker'
  | 'console'
  | 'agent'
  | 'info'
  | 'info-outline'
  | 'repo'
  | 'user'
  | 'org'
  | 'cron'
  | 'toolbox'
  | 'graph';

defineProps<{
  name: IconNames;
  size?: string;
}>();
</script>
