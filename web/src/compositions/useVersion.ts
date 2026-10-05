import semverCoerce from 'semver/functions/coerce';
import semverGt from 'semver/functions/gt';
import { onMounted, ref } from 'vue';

import useAuthentication from './useAuthentication';
import useConfig from './useConfig';

interface VersionEntry {
  latest: string;
  dev: string;
  rc: string;
}

const version = ref<{
  latest: string | undefined;
  current: string;
  currentShort: string;
  needsUpdate: boolean;
  usesDev: boolean;
}>();

async function fetchVersion(): Promise<{ latest?: string; rc?: string; dev?: string }> {
  try {
    const resp = await fetch('https://docs.crowci.com/crow-versions.json');
    const json = (await resp.json()) as VersionEntry[];
    const entry = json[0];
    return {
      latest: entry.latest ?? undefined,
      rc: entry.rc ?? undefined,
      dev: entry.dev ?? undefined,
    };
  } catch (error) {
    console.error('Failed to fetch version info', error);
    return {};
  }
}

const isInitialized = ref(false);

export function useVersion() {
  if (isInitialized.value) {
    return version;
  }
  isInitialized.value = true;

  const config = useConfig();
  const current = config.version as string;
  const currentSemver = semverCoerce(current);
  const usesDev = current.startsWith('dev');

  const { user } = useAuthentication();
  if (config.skipVersionCheck || user?.admin !== true) {
    version.value = {
      latest: undefined,
      current,
      currentShort: current,
      needsUpdate: false,
      usesDev,
    };
    return version;
  }

  if (current === 'dev') {
    version.value = {
      latest: undefined,
      current,
      currentShort: current,
      needsUpdate: false,
      usesDev,
    };
    return version;
  }

  onMounted(async () => {
    const versionInfo = await fetchVersion();

    let latest;
    const hasVersionInfo = Boolean(versionInfo.latest ?? versionInfo.rc ?? versionInfo.dev);
    if (hasVersionInfo) {
      if (usesDev) {
        latest = versionInfo.dev;
      } else if (current.includes('rc')) {
        latest = versionInfo.rc;
      } else {
        latest = versionInfo.latest;
      }
    }

    let needsUpdate = false;
    if (usesDev) {
      needsUpdate = latest !== current;
    } else if (latest !== undefined && currentSemver !== null) {
      needsUpdate = semverGt(latest, currentSemver);
    }

    version.value = {
      latest,
      current,
      currentShort: current,
      needsUpdate,
      usesDev,
    };
  });

  return version;
}
