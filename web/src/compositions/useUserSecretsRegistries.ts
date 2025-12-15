import { computed, ref } from 'vue';

export interface ScopeFilters {
  user: boolean;
  org: boolean;
  global: boolean;
}

export interface GroupedItems<T> {
  orgName: string;
  scopeType: 'user' | 'org' | 'global';
  items: T[];
}

export interface WithOrgScope {
  org_name: string;
  scope_type: 'user' | 'org' | 'global';
}

export function useUserSecretsRegistries<T extends WithOrgScope>() {
  const scopeFilters = ref<ScopeFilters>({
    user: true,
    org: true,
    global: false,
  });

  const searchTerm = ref('');
  const expandedOrgs = ref<Set<string>>(new Set());

  // Computed scope query param for API
  const scopeParam = computed(() => {
    const scopes: string[] = [];
    if (scopeFilters.value.user) scopes.push('user');
    if (scopeFilters.value.org) scopes.push('org');
    if (scopeFilters.value.global) scopes.push('global');
    return scopes.join(',');
  });

  // Group items by organization
  function groupByOrg(items: T[]): GroupedItems<T>[] {
    const groups = new Map<string, GroupedItems<T>>();

    for (const item of items) {
      // Ensure org_name is never undefined
      const key = item.org_name || 'Unknown';
      if (!groups.has(key)) {
        groups.set(key, {
          orgName: key,
          scopeType: item.scope_type,
          items: [],
        });
      }
      groups.get(key)!.items.push(item);
    }

    // Sort: user first, then orgs alphabetically, then global
    return Array.from(groups.values()).sort((a, b) => {
      if (a.scopeType === 'user') return -1;
      if (b.scopeType === 'user') return 1;
      if (a.scopeType === 'global') return 1;
      if (b.scopeType === 'global') return -1;
      return a.orgName.localeCompare(b.orgName);
    });
  }

  function toggleOrg(orgName: string) {
    if (expandedOrgs.value.has(orgName)) {
      expandedOrgs.value.delete(orgName);
    } else {
      expandedOrgs.value.add(orgName);
    }
  }

  function expandAll(groups: GroupedItems<T>[]) {
    groups.forEach((g) => expandedOrgs.value.add(g.orgName));
  }

  function collapseAll() {
    expandedOrgs.value.clear();
  }

  // Initialize with all expanded
  function initializeExpanded(groups: GroupedItems<T>[]) {
    if (expandedOrgs.value.size === 0) {
      expandAll(groups);
    }
  }

  return {
    scopeFilters,
    scopeParam,
    searchTerm,
    expandedOrgs,
    groupByOrg,
    toggleOrg,
    expandAll,
    collapseAll,
    initializeExpanded,
  };
}
