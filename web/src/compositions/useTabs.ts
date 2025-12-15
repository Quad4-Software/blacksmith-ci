import { ref } from 'vue';
import type { RouteLocationRaw } from 'vue-router';

import type { IconNames } from '~/components/atomic/Icon.types';

import { provide, requiredInject } from './useInjectProvide';

export interface Tab {
  to: RouteLocationRaw;
  title: string;
  count?: number;
  icon?: IconNames;
  iconSize?: string;
  iconClass?: string;
  matchChildren?: boolean;
}

export function useTabsProvider() {
  const tabs = ref<Tab[]>([]);
  provide('tabs', tabs);
}

export function useTabsClient() {
  const tabs = requiredInject('tabs');
  return { tabs };
}
