<template>
  <Settings :title="$t('user.settings.general.general')">
    <!-- Profile Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.general.section_profile') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.section_profile_desc') }}
        </p>
      </div>
      <div class="flex flex-col md:flex-row gap-6 flex-wrap">
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.description')">
            <Textarea
              :id="id"
              v-model="userDescription"
              :placeholder="$t('user.settings.general.description_placeholder')"
              rows="3"
              class="resize-none w-full md:w-[250px]"
            />
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.timezone')">
            <Select :id="id" v-model="userTimezone">
              <SelectTrigger>
                <SelectValue :placeholder="$t('user.settings.general.timezone_placeholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="tz in timezoneOptions" :key="tz" :value="tz">
                  {{ tz }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.language')">
            <Select :id="id" v-model="selectedLocale">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in localeOptions" :key="option.value" :value="option.value">
                  {{ option.text }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
      </div>
      <Button @click="saveProfile">
        <Icon v-if="isSaving" name="loading" class="mr-2 h-4 w-4 animate-spin" />
        {{ $t('user.settings.general.save') }}
      </Button>
    </div>

    <Separator class="my-6" />

    <!-- Appearance Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.general.section_appearance') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.section_appearance_desc') }}
        </p>
      </div>
      <div class="flex flex-col md:flex-row gap-6 flex-wrap">
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.light_theme_label')">
            <ThemeSelect :id="id" v-model="lightThemeName" :themes="availableThemes" />
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.dark_theme_label')">
            <ThemeSelect :id="id" v-model="darkThemeName" :themes="availableThemes" />
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.color_mode')">
            <Select :id="id" v-model="storeTheme">
              <SelectTrigger class="w-full md:w-[200px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in colorModeOptions" :key="option.value" :value="option.value">
                  {{ option.text }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.font_size')">
            <Select :id="id" v-model="fontSize">
              <SelectTrigger class="w-full md:w-[200px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in FONT_SIZE_OPTIONS" :key="option.value" :value="option.value">
                  {{ option.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>
      </div>
      <InputField v-slot="{ id }" :label="$t('user.settings.general.theme_overrides')">
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.theme_overrides_desc') }}
        </p>
        <Textarea
          :id="id"
          v-model="themeOverridesDraft"
          :placeholder="themeOverridesPlaceholder"
          rows="8"
          spellcheck="false"
          class="max-h-96 w-full font-mono text-xs"
        />
        <div class="space-y-1 text-sm">
          <p v-if="themeOverridesDraft.trim() !== ''" class="text-muted-foreground">
            {{
              $t(
                'user.settings.general.theme_overrides_applied',
                { count: themeOverridesParsed.applied },
                themeOverridesParsed.applied,
              )
            }}
          </p>
          <p v-if="themeOverridesParsed.ignoredSelectors.length > 0" class="text-destructive">
            {{ $t('user.settings.general.theme_overrides_ignored_selectors') }}
            <span class="font-mono">{{ themeOverridesParsed.ignoredSelectors.join(', ') }}</span>
          </p>
          <p v-if="themeOverridesParsed.ignoredProperties.length > 0" class="text-destructive">
            {{ $t('user.settings.general.theme_overrides_ignored_properties') }}
            <span class="font-mono">{{ themeOverridesParsed.ignoredProperties.join(', ') }}</span>
          </p>
          <p v-if="themeOverridesParsed.invalidValues.length > 0" class="text-destructive">
            {{ $t('user.settings.general.theme_overrides_invalid_values') }}
            <span class="font-mono">{{ themeOverridesParsed.invalidValues.join('; ') }}</span>
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button :disabled="!themeOverridesDirty" @click="applyThemeOverrides">
            {{ $t('user.settings.general.theme_overrides_apply') }}
          </Button>
          <Button variant="outline" :disabled="!themeOverridesDirty" @click="revertThemeOverrides">
            {{ $t('user.settings.general.theme_overrides_revert') }}
          </Button>
          <Button variant="outline" @click="insertCurrentTheme">
            {{ $t('user.settings.general.theme_overrides_insert_current') }}
          </Button>
        </div>
      </InputField>
    </div>

    <Separator class="my-6" />

    <!-- Pipeline View Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.general.section_pipeline_view') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.general.section_pipeline_view_desc') }}
        </p>
      </div>
      <div class="flex flex-col md:flex-row gap-6 flex-wrap">
        <div>
          <InputField :label="$t('user.settings.general.pipeline_steps_width')">
            <div class="flex items-center gap-2">
              <NumberField v-model="pipelineSplitRatioInput" :min="25" :max="50" class="max-w-[8rem]">
                <NumberFieldContent>
                  <NumberFieldDecrement />
                  <NumberFieldInput />
                  <NumberFieldIncrement />
                </NumberFieldContent>
              </NumberField>
              <span class="text-sm text-muted-foreground">
                {{ $t('user.settings.general.split_ratio_range') }}
              </span>
            </div>
          </InputField>
        </div>

        <div>
          <InputField v-slot="{ id }" :label="$t('user.settings.general.log_theme')">
            <Select :id="id" v-model="logTheme">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in logThemeOptions" :key="option.value" :value="option.value">
                  {{ option.text }}
                </SelectItem>
              </SelectContent>
            </Select>
          </InputField>
        </div>

        <div class="mb-4 mt-2 flex flex-col">
          <label class="mb-2 text-foreground">{{ $t('user.settings.general.auto_follow') }}</label>
          <div class="flex items-center gap-3">
            <Switch v-model="autoFollowEnabled" />
            <span class="text-sm text-muted-foreground">
              {{ $t('user.settings.general.auto_follow_desc') }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <Separator class="my-6" />

    <!-- Linked Forges Section -->
    <div class="space-y-4">
      <div>
        <h3 class="text-lg text-foreground">{{ $t('user.settings.forges.linked_forges') }}</h3>
        <p class="text-sm text-muted-foreground">
          {{ $t('user.settings.forges.linked_forges_desc') }}
        </p>
      </div>

      <!-- Loading state -->
      <div v-if="forgesLoading" class="flex justify-center p-4">
        <Icon name="spinner" class="h-6 w-6 animate-spin text-muted-foreground" />
      </div>

      <!-- Linked forges list -->
      <div v-else-if="userForges.length > 0" class="space-y-2">
        <div
          v-for="userForge in userForges"
          :key="userForge.id"
          class="flex items-center gap-3 p-3 rounded-md border"
          :class="[userForge.status === 'expired' ? 'border-destructive/50 bg-destructive/5' : 'border-border']"
        >
          <img
            v-if="getForgeCustomIconUrl(userForge)"
            :src="getForgeCustomIconUrl(userForge)!"
            class="h-6 w-6 shrink-0"
            alt=""
          />
          <Icon v-else :name="getForgeIconName(userForge)" class="h-6 w-6 text-muted-foreground shrink-0" />
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="font-medium text-foreground">{{ userForge.forge_login }}</span>
              <Badge v-if="userForge.is_primary" variant="secondary" class="text-xs">
                {{ $t('user.settings.forges.primary') }}
              </Badge>
              <Badge v-if="userForge.status === 'expired'" variant="destructive" class="text-xs">
                {{ $t('user.settings.forges.status_expired') }}
              </Badge>
            </div>
            <div class="text-sm text-muted-foreground">
              {{ userForge.forge_name || userForge.forge_type }}
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <Button
              v-if="userForge.status === 'expired'"
              variant="outline"
              size="sm"
              as="a"
              :href="getReconnectUrl(userForge.forge_id)"
            >
              <Icon name="ph:arrow-clockwise" class="mr-1.5 h-3 w-3" />
              {{ $t('user.settings.forges.reconnect') }}
            </Button>
            <Button
              v-if="!userForge.is_primary && userForges.length > 1"
              variant="outline"
              size="sm"
              :disabled="unlinkingForgeId === userForge.forge_id"
              @click="unlinkForge(userForge)"
            >
              <Icon v-if="unlinkingForgeId === userForge.forge_id" name="spinner" class="mr-1.5 h-3 w-3 animate-spin" />
              {{ $t('user.settings.forges.unlink') }}
            </Button>
          </div>
        </div>
      </div>

      <!-- No forges -->
      <div v-else class="text-center p-4 text-muted-foreground border border-dashed border-border rounded-md">
        {{ $t('user.settings.forges.no_forges') }}
      </div>

      <!-- Link another forge -->
      <div v-if="availableForges.length > 0" class="pt-2">
        <h4 class="text-sm font-medium text-foreground mb-2">{{ $t('user.settings.forges.link_forge') }}</h4>
        <div class="flex flex-wrap gap-2">
          <Button
            v-for="forge in availableForges"
            :key="forge.id"
            variant="outline"
            size="sm"
            as="a"
            :href="getLinkUrl(forge.id)"
          >
            <Icon :name="getForgeIconNameFromType(forge.type, forge.url)" class="mr-1.5 h-4 w-4" />
            {{ forge.url.replace(/^https?:\/\//, '').replace(/\/$/, '') }}
          </Button>
        </div>
      </div>
    </div>
  </Settings>

  <!-- Unlink Forge Confirmation Dialog -->
  <AlertDialog v-model:open="showUnlinkDialog">
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ $t('user.settings.forges.unlink_confirm_title') }}</AlertDialogTitle>
        <AlertDialogDescription>
          <div class="space-y-3">
            <p>
              {{ $t('user.settings.forges.unlink_confirm_desc', { forge: unlinkDialogForge?.forge_login }) }}
            </p>

            <!-- Loading affected repos -->
            <div v-if="loadingAffectedRepos" class="flex items-center gap-2 text-muted-foreground">
              <Icon name="spinner" class="h-4 w-4 animate-spin" />
              {{ $t('user.settings.forges.loading_affected_repos') }}
            </div>

            <!-- Affected repos list -->
            <div v-else-if="affectedRepos.length > 0" class="space-y-2">
              <p class="font-medium text-destructive">
                {{ $t('user.settings.forges.affected_repos_warning', { count: affectedRepos.length }) }}
              </p>
              <ul class="max-h-[200px] overflow-y-auto space-y-1 rounded-md border border-border p-2 text-sm">
                <li v-for="repo in affectedRepos" :key="repo.id" class="flex items-center gap-2">
                  <Icon name="repo" class="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span class="truncate">{{ repo.full_name }}</span>
                </li>
              </ul>
              <p class="text-sm text-muted-foreground">
                {{ $t('user.settings.forges.affected_repos_note') }}
              </p>
            </div>

            <!-- No affected repos -->
            <div v-else class="text-muted-foreground">
              {{ $t('user.settings.forges.no_affected_repos') }}
            </div>
          </div>
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="cancelUnlink">{{ $t('cancel') }}</AlertDialogCancel>
        <AlertDialogAction
          :disabled="loadingAffectedRepos || unlinkingForgeId !== null"
          class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          @click="confirmUnlink"
        >
          <Icon v-if="unlinkingForgeId !== null" name="spinner" class="mr-1.5 h-4 w-4 animate-spin" />
          {{ $t('user.settings.forges.unlink') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>

<script lang="ts" setup>
import { useStorage } from '@vueuse/core';
import { SUPPORTED_LOCALES } from 'virtual:vue-i18n-supported-locales';
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

import Icon from '~/components/atomic/Icon.vue';
import type { IconNames } from '~/components/atomic/Icon.types';
import InputField from '~/components/form/InputField.vue';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog';
import { Badge } from '~/components/ui/badge';
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '~/components/ui/number-field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Separator } from '~/components/ui/separator';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { Button } from '~/components/ui/button';
import Settings from '~/components/layout/Settings.vue';
import ThemeSelect from '~/components/user/ThemeSelect.vue';
import { setI18nLanguage } from '~/compositions/useI18n';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import { FONT_SIZE_OPTIONS, useTheme } from '~/compositions/useTheme';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import useConfig from '~/compositions/useConfig';
import useNotifications from '~/compositions/useNotifications';
import type { AvailableForge, Repo, UserForge } from '~/lib/api/types';
import { getTheme, parseThemeOverrides, themesToCss } from '~/lib/themes';

const { locale, t } = useI18n();
const apiClient = useApiClient();
const { user } = useAuthentication();
const config = useConfig();
const notifications = useNotifications();
const route = useRoute();
const router = useRouter();

// Forge management
const userForges = ref<UserForge[]>([]);
const availableForges = ref<AvailableForge[]>([]);
const forgesLoading = ref(false);
const unlinkingForgeId = ref<number | null>(null);

// Unlink confirmation dialog
const showUnlinkDialog = ref(false);
const unlinkDialogForge = ref<UserForge | null>(null);
const affectedRepos = ref<Repo[]>([]);
const loadingAffectedRepos = ref(false);

function isValidUrl(str: string): boolean {
  try {
    return Boolean(new URL(str));
  } catch {
    return false;
  }
}

function getForgeIconName(userForge: UserForge): IconNames {
  // Check for custom icon first
  if (userForge.forge_icon) {
    // If it's not a URL, treat it as a predefined icon name
    if (!isValidUrl(userForge.forge_icon)) {
      const customIcon = userForge.forge_icon.toLowerCase();
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
  }

  const forgeType = userForge.forge_type?.toLowerCase() || '';
  const forgeName = userForge.forge_name?.toLowerCase() || '';

  if (forgeName.includes('codeberg')) return 'codeberg';
  if (forgeName.includes('codefloe')) return 'codefloe';

  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forgeType] || 'repo';
}

function getForgeCustomIconUrl(userForge: UserForge): string | null {
  if (userForge.forge_icon && isValidUrl(userForge.forge_icon)) {
    return userForge.forge_icon;
  }
  return null;
}

function getForgeIconNameFromType(forgeType: string, url: string): IconNames {
  if (url.toLowerCase().includes('codeberg')) return 'codeberg';
  if (url.toLowerCase().includes('codefloe')) return 'codefloe';

  const typeToIcon: Record<string, IconNames> = {
    github: 'github',
    gitlab: 'gitlab',
    gitea: 'gitea',
    forgejo: 'forgejo',
    bitbucket: 'bitbucket',
    'bitbucket-dc': 'bitbucket',
    addon: 'repo',
  };
  return typeToIcon[forgeType.toLowerCase()] || 'repo';
}

function getLinkUrl(forgeId: number): string {
  const rootPath = config.rootPath || '';
  return `${rootPath}/authorize?forge_id=${forgeId}&link=true`;
}

function getReconnectUrl(forgeId: number): string {
  const rootPath = config.rootPath || '';
  return `${rootPath}/authorize?forge_id=${forgeId}&reconnect=true`;
}

async function loadForges() {
  forgesLoading.value = true;
  try {
    const [forges, available] = await Promise.all([apiClient.getUserForges(), apiClient.getAvailableForges()]);
    userForges.value = forges;
    availableForges.value = available.filter((f) => !f.is_linked);
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('user.settings.forges.load_error'),
      type: 'error',
    });
  } finally {
    forgesLoading.value = false;
  }
}

async function unlinkForge(userForge: UserForge) {
  if (userForge.is_primary) {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('user.settings.forges.cannot_unlink_primary'),
      type: 'error',
    });
    return;
  }

  // Fetch affected repos and show confirmation dialog
  unlinkDialogForge.value = userForge;
  loadingAffectedRepos.value = true;
  showUnlinkDialog.value = true;

  try {
    affectedRepos.value = await apiClient.getForgeAffectedRepos(userForge.forge_id, { silent: true });
  } catch {
    affectedRepos.value = [];
  } finally {
    loadingAffectedRepos.value = false;
  }
}

async function confirmUnlink() {
  if (!unlinkDialogForge.value) return;

  unlinkingForgeId.value = unlinkDialogForge.value.forge_id;
  try {
    const result = await apiClient.unlinkForge(unlinkDialogForge.value.forge_id);
    notifications.notify({
      title: t('notify_title_success'),
      text:
        result.disabled_repos > 0
          ? t('user.settings.forges.unlink_success_with_repos', { count: result.disabled_repos })
          : t('user.settings.forges.unlink_success'),
      type: 'success',
    });
    await loadForges();
  } catch {
    notifications.notify({
      title: t('notify_title_error'),
      text: t('user.settings.forges.unlink_error'),
      type: 'error',
    });
  } finally {
    unlinkingForgeId.value = null;
    showUnlinkDialog.value = false;
    unlinkDialogForge.value = null;
    affectedRepos.value = [];
  }
}

function cancelUnlink() {
  showUnlinkDialog.value = false;
  unlinkDialogForge.value = null;
  affectedRepos.value = [];
}

// Theme settings
const { lightThemeName, darkThemeName, availableThemes, storeTheme, fontSize, themeOverridesCss } = useTheme();

// Theme overrides are edited as a draft and only stored on apply, so a
// half-typed value never restyles the page.
const themeOverridesDraft = ref(themeOverridesCss.value);
const themeOverridesParsed = computed(() => parseThemeOverrides(themeOverridesDraft.value));
const themeOverridesDirty = computed(() => themeOverridesDraft.value !== themeOverridesCss.value);
const themeOverridesPlaceholder = ':root {\n  --primary: #c2410c;\n}\n\n.dark {\n  --primary: #fb923c;\n}';

function applyThemeOverrides() {
  themeOverridesCss.value = themeOverridesDraft.value.trim() === '' ? '' : themeOverridesDraft.value;
  themeOverridesDraft.value = themeOverridesCss.value;
  notifications.notify({
    title: t('user.settings.general.theme_overrides_saved'),
    type: 'success',
  });
}

function revertThemeOverrides() {
  themeOverridesDraft.value = themeOverridesCss.value;
}

function insertCurrentTheme() {
  themeOverridesDraft.value = themesToCss(getTheme(lightThemeName.value), getTheme(darkThemeName.value));
}

const colorModeOptions = computed(() => [
  { value: 'light', text: t('user.settings.general.theme.light') },
  { value: 'dark', text: t('user.settings.general.theme.dark') },
  { value: 'auto', text: t('user.settings.general.theme.auto') },
]);

// Profile fields
const userDescription = ref(user?.description || '');
const userTimezone = ref(user?.timezone || '');
const isSaving = ref(false);

// Load timezones from API
const timezoneOptions = ref<string[]>([]);

onMounted(async () => {
  // Handle query params for forge link/reconnect notifications
  if (route.query.forge_linked === 'true') {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('user.settings.forges.link_success'),
      type: 'success',
    });
    // Clear the query param
    router.replace({ query: {} });
  } else if (route.query.forge_reconnected === 'true') {
    notifications.notify({
      title: t('notify_title_success'),
      text: t('user.settings.forges.reconnect_success'),
      type: 'success',
    });
    // Clear the query param
    router.replace({ query: {} });
  }

  // Load forges
  loadForges();

  // Load timezones
  try {
    const zones = await apiClient.getTimezones();
    timezoneOptions.value = zones;
  } catch {
    // Fallback to browser's built-in IANA timezone list
    timezoneOptions.value = Intl.supportedValuesOf('timeZone');
  }
});

async function saveProfile() {
  if (!user) {
    console.error('No user available');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      ...user,
      description: userDescription.value,
      timezone: userTimezone.value,
    };

    await apiClient.updateUser(payload);

    // Update window user object
    if (window.CROW_USER) {
      window.CROW_USER.description = userDescription.value;
      window.CROW_USER.timezone = userTimezone.value;
    }

    notifications.notify({
      title: t('user.settings.general.profile_saved'),
      type: 'success',
    });
  } catch (error) {
    console.error('Failed to save profile:', error);
    notifications.notify({
      title: t('user.settings.general.profile_save_error'),
      type: 'error',
    });
  } finally {
    isSaving.value = false;
  }
}

const localeOptions = computed(() =>
  SUPPORTED_LOCALES.map((supportedLocale) => ({
    value: supportedLocale,
    text: new Intl.DisplayNames(supportedLocale, { type: 'language' }).of(supportedLocale) || supportedLocale,
  })),
);

const storedLocale = useStorage('crow:locale', locale.value);
const selectedLocale = computed<string>({
  async set(_selectedLocale) {
    await setI18nLanguage(_selectedLocale);
    storedLocale.value = _selectedLocale;
  },
  get() {
    return storedLocale.value;
  },
});

// Pipeline split ratio input
const storedSplitRatio = useStorage('crow:pipeline-sidebar-size', 30);
const pipelineSplitRatioInput = computed<number>({
  set(value: number) {
    // Clamp value between 25 and 50
    const clampedValue = Math.max(25, Math.min(50, value));
    storedSplitRatio.value = clampedValue;
  },
  get() {
    return storedSplitRatio.value;
  },
});

// Log theme options
const logThemeOptions = computed(() => [
  { value: 'auto', text: t('user.settings.general.log_theme_system') },
  { value: 'light', text: t('user.settings.general.log_theme_light') },
  { value: 'dark', text: t('user.settings.general.log_theme_dark') },
]);

const logTheme = useStorage('crow:log-theme', 'auto');

const autoFollowEnabled = useStorage('crow:pipeline-auto-follow', true);

useCrowTitle(computed(() => [t('user.settings.general.general'), t('user.settings.settings')]));
</script>
