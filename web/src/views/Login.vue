<template>
  <div class="min-h-screen w-full flex flex-col items-center justify-center bg-white dark:bg-black text-wp-text-100">
    <div
      class="font-manrope shadow-2xl rounded-3xl py-8 md:py-12 px-8 md:px-16 flex flex-col items-center gap-4 max-w-lg border border-black/10 dark:border-white/30 bg-white dark:bg-[#383c4a] text-wp-text-100 md:my-24 my-12 md:mx-24 mx-6"
    >
      <component :is="isDark ? CrowLogo : CrowLogoDark" class="h-20 w-20 mb-2" />
      <h1 class="text-3xl font-extrabold text-center">{{ $t('welcome_to_crow') }}</h1>
      <p class="text-center text-lg mb-2">{{ $t('login_oauth_message') }}</p>
      <div class="flex flex-col gap-2 w-full items-center">
        <Button
          v-for="(forge, idx) in forges"
          :key="forge.id"
          :ref="(el) => (buttonRefs[idx] = el)"
          class="justify-center dark:!bg-black text-wp-text-100"
          :style="maxButtonWidth ? { width: `${maxButtonWidth}px` } : {}"
          :disabled="loading === forge.id"
          color="default"
          :aria-label="`${$t('login_oauth_message')} ${getHostFromUrl(forge)}`"
          @click="doLogin(forge.id)"
        >
          <span v-if="loading === forge.id" class="loader" aria-hidden="true"></span>
          <span v-else class="flex items-center gap-2">
            <img
              :src="getFaviconUrl(forge)"
              alt=""
              class="h-6 w-6 rounded bg-white dark:bg-black object-contain"
              :data-forge-id="forge.id"
              @error="onFaviconError($event)"
            />
            {{ getHostFromUrl(forge) }}
          </span>
        </Button>
      </div>
      <div
        v-if="errorMessage"
        class="w-full text-center text-red-500 bg-red-50 dark:bg-red-900/30 rounded-lg py-2 px-3 mt-2 text-wp-text-100"
      >
        <span class="whitespace-pre-line">{{ errorMessage }}</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, nextTick, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import { useTheme } from '~/compositions/useTheme';

import CrowLogo from '~/assets/logo.svg?component';
import CrowLogoDark from '~/assets/logo-dark.svg?component';
import Button from '~/components/atomic/Button.vue';
import useApiClient from '~/compositions/useApiClient';
import useAuthentication from '~/compositions/useAuthentication';
import { useCrowTitle } from '~/compositions/useCrowTitle';
import type { Forge } from '~/lib/api/types';

const route = useRoute();
const router = useRouter();
const authentication = useAuthentication();
const i18n = useI18n();
const apiClient = useApiClient();
const { theme } = useTheme();

const isDark = computed(() => theme.value === 'dark');

const forges = ref<Forge[]>([]);
const loading = ref<number | null>(null);

// Track favicon extension state per forge
const faviconExts = ['svg', 'ico', 'png'] as const;
const faviconState = ref<Record<number, number>>({}); // forge.id -> ext index
const faviconUrlOverride = ref<Record<number, string>>({}); // forge.id -> custom favicon url

async function fetchFaviconFromHead(forge: Forge) {
  try {
    const url = new URL(forge.url);
    const res = await fetch(url.origin, { mode: 'cors' });
    if (!res.ok) return null; // fallback if fetch fails
    const html = await res.text();
    const doc = new DOMParser().parseFromString(html, 'text/html');
    // Collect all <link rel="icon" ...> or similar
    const iconLinks = Array.from(
      doc.querySelectorAll('link[rel~="icon" i], link[rel~="shortcut icon" i], link[rel~="apple-touch-icon" i]'),
    );
    // Prefer by type attribute (svg, png, ico), then by href extension
    let href = '';
    const typePrefs = [
      { type: 'svg', mime: 'svg' },
      { type: 'png', mime: 'png' },
      { type: 'ico', mime: 'ico' },
    ];
    for (const pref of typePrefs) {
      const found = iconLinks.find((link) => {
        const typeAttr = (link.getAttribute('type') || '').toLowerCase();
        const hrefAttr = (link.getAttribute('href') || '').toLowerCase();
        return typeAttr.includes(pref.mime) || hrefAttr.endsWith(`.${pref.type}`);
      });
      if (found) {
        href = found.getAttribute('href')!;
        break;
      }
    }
    // If none matched, fallback to first icon link
    if (!href && iconLinks.length > 0 && iconLinks[0].getAttribute('href')) {
      href = iconLinks[0].getAttribute('href')!;
    }
    if (href) {
      // Resolve relative URLs
      if (!href.startsWith('http')) {
        href = url.origin + (href.startsWith('/') ? href : `/${href}`);
      }
      faviconUrlOverride.value[forge.id] = href;
      return href;
    }
  } catch {
    // If fetch fails (e.g., CORS), do not set override, let fallback logic run
    return null;
  }
  return null;
}

function getFaviconUrl(forge: Forge) {
  // If we have an override from <head>, use it
  if (faviconUrlOverride.value[forge.id]) {
    return faviconUrlOverride.value[forge.id];
  }
  try {
    const url = new URL(forge.url);
    const extIdx = faviconState.value[forge.id] ?? 0;
    const ext = faviconExts[extIdx];
    return `${url.origin}/favicon.${ext}`;
  } catch {
    return '';
  }
}

async function ensureFavicon(forge: Forge) {
  if (!faviconUrlOverride.value[forge.id]) {
    await fetchFaviconFromHead(forge);
  }
}

function getHostFromUrl(forge: Forge) {
  if (!forge.url) {
    return forge.type.charAt(0).toUpperCase() + forge.type.slice(1);
  }
  const url = new URL(forge.url);
  return url.hostname;
}

function doLogin(forgeId?: number) {
  loading.value = forgeId ?? null;
  const url = typeof route.query.url === 'string' ? route.query.url : '';
  authentication.authenticate(url, forgeId);
}

const authErrorMessages = {
  oauth_error: i18n.t('oauth_error'),
  internal_error: i18n.t('internal_error'),
  registration_closed: i18n.t('registration_closed'),
  access_denied: i18n.t('access_denied'),
  invalid_state: i18n.t('invalid_state'),
  org_access_denied: i18n.t('org_access_denied'),
};

const errorMessage = ref<string>();

const buttonRefs = ref<any[]>([]);
const maxButtonWidth = ref<number | null>(null);

function updateButtonWidths() {
  nextTick(() => {
    maxButtonWidth.value = Math.max(
      ...buttonRefs.value.map((el) => {
        if (!el) return 0;
        // If it's a Vue component, get the underlying element
        if (el.$el) return el.$el.offsetWidth || 0;
        return el.offsetWidth || 0;
      }),
      0,
    );
  });
}

watch(forges, updateButtonWidths, { immediate: true });
watch(() => i18n.locale, updateButtonWidths); // update on language change

// Ensure favicon is checked on mount for all forges
onMounted(async () => {
  if (authentication.isAuthenticated) {
    await router.replace({ name: 'home' });
    return;
  }

  forges.value = (await apiClient.getForges()) ?? [];

  // Remove the dummy placeholder for another forge login for UI testing
  // forges.value.push({
  //   id: 9999,
  //   type: 'gitea',
  //   url: 'https://codefloe.com',
  // });

  // Try to fetch favicons from <head> for all forges
  await Promise.all(forges.value.map(ensureFavicon));

  if (route.query.error) {
    const error = route.query.error as keyof typeof authErrorMessages;
    errorMessage.value = authErrorMessages[error] ?? error;
  }
  await nextTick();
  updateButtonWidths();
});

function onFaviconError(event: Event) {
  const target = event.target as HTMLImageElement;
  const forgeId = Number(target.dataset.forgeId);
  // If we were using a <head> favicon, clear it and try fallback
  if (faviconUrlOverride.value[forgeId]) {
    delete faviconUrlOverride.value[forgeId];
    target.src = getFaviconUrl({ ...forges.value.find((f) => f.id === forgeId)! });
    return;
  }
  const currentIdx = faviconState.value[forgeId] ?? 0;
  if (currentIdx < faviconExts.length - 1) {
    faviconState.value[forgeId] = currentIdx + 1;
    // Force reload with new src
    target.src = getFaviconUrl({ ...forges.value.find((f) => f.id === forgeId)! });
  } else {
    target.style.display = 'none';
  }
}

useCrowTitle(computed(() => [i18n.t('login')]));
</script>

<style scoped>
.loader {
  border: 2px solid #f3f3f3;
  border-top: 2px solid #f6a434;
  border-radius: 50%;
  width: 1.2em;
  height: 1.2em;
  animation: spin 1s linear infinite;
  display: inline-block;
}
@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}
</style>
