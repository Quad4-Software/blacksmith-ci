import type { Component } from 'vue';
import { createRouter, createWebHistory } from 'vue-router';
import type { RouteRecordRaw } from 'vue-router';

import useConfig from '~/compositions/useConfig';
import useUserConfig from '~/compositions/useUserConfig';
import { useAuthStore } from '~/store/auth';

const { rootPath } = useConfig();
const routes: RouteRecordRaw[] = [
  {
    path: `${rootPath}/`,
    name: 'home',
    redirect: `${rootPath}/repos`,
  },
  {
    path: `${rootPath}/agents`,
    name: 'agents',
    component: (): Component => import('~/views/Agents.vue'),
    meta: { authentication: 'required' },
  },
  {
    path: `${rootPath}/autoscalers`,
    name: 'autoscalers',
    component: (): Component => import('~/views/Autoscalers.vue'),
    meta: { authentication: 'required' },
  },
  {
    path: `${rootPath}/repos`,
    component: (): Component => import('~/views/RouterView.vue'),
    children: [
      {
        path: '',
        name: 'repos',
        component: (): Component => import('~/views/Repos.vue'),
        // No authentication required - anonymous users can browse public repos
      },
      {
        path: 'add',
        name: 'repo-add',
        redirect: { name: 'repos' },
      },
      {
        path: ':repoId',
        name: 'repo-wrapper',
        component: (): Component => import('~/views/repo/RepoWrapper.vue'),
        props: true,
        children: [
          {
            path: '',
            name: 'repo',
            component: (): Component => import('~/views/repo/RepoPipelines.vue'),
            meta: { repoHeader: true },
          },
          {
            path: 'pipeline/:pipelineId',
            component: (): Component => import('~/views/repo/pipeline/PipelineWrapper.vue'),
            props: true,
            children: [
              {
                path: ':stepId?',
                name: 'repo-pipeline',
                component: (): Component => import('~/views/repo/pipeline/Pipeline.vue'),
                props: true,
              },
              {
                path: 'graph',
                name: 'repo-pipeline-graph',
                component: (): Component => import('~/views/repo/pipeline/PipelineGraph.vue'),
              },
              {
                path: 'changed-files',
                name: 'repo-pipeline-changed-files',
                component: (): Component => import('~/views/repo/pipeline/PipelineChangedFiles.vue'),
              },
              {
                path: 'config',
                name: 'repo-pipeline-config',
                component: (): Component => import('~/views/repo/pipeline/PipelineConfig.vue'),
                props: true,
              },
              {
                path: 'errors',
                name: 'repo-pipeline-errors',
                component: (): Component => import('~/views/repo/pipeline/PipelineErrors.vue'),
                props: true,
              },
              {
                path: 'metadata',
                name: 'repo-pipeline-metadata',
                component: (): Component => import('~/views/repo/pipeline/PipelineMetadata.vue'),
                props: true,
              },
            ],
          },
          {
            path: 'settings',
            component: (): Component => import('~/views/repo/settings/RepoSettings.vue'),
            meta: { authentication: 'required' },
            props: true,
            children: [
              {
                path: '',
                name: 'repo-settings',
                component: (): Component => import('~/views/repo/settings/General.vue'),
                props: true,
              },
              {
                path: 'secrets',
                name: 'repo-settings-secrets',
                component: (): Component => import('~/views/repo/settings/Secrets.vue'),
                props: true,
              },
              {
                path: 'registries',
                name: 'repo-settings-registries',
                component: (): Component => import('~/views/repo/settings/Registries.vue'),
                props: true,
              },
              {
                path: 'crons',
                name: 'repo-settings-crons',
                component: (): Component => import('~/views/repo/settings/Crons.vue'),
                props: true,
              },
              {
                path: 'badge',
                name: 'repo-settings-badge',
                component: (): Component => import('~/views/repo/settings/Badge.vue'),
                props: true,
              },
              {
                path: 'actions',
                name: 'repo-settings-actions',
                component: (): Component => import('~/views/repo/settings/Actions.vue'),
                props: true,
              },
            ],
          },
        ],
      },
      {
        path: ':repoOwner/:repoName/:pathMatch(.*)*',
        component: (): Component => import('~/views/repo/RepoDeprecatedRedirect.vue'),
        props: true,
      },
    ],
  },
  {
    path: `${rootPath}/orgs/:orgId`,
    component: (): Component => import('~/views/org/OrgWrapper.vue'),
    props: true,
    children: [
      {
        path: '',
        name: 'org',
        component: (): Component => import('~/views/org/OrgRepos.vue'),
        props: true,
      },
      {
        path: 'settings',
        name: 'org-settings',
        component: (): Component => import('~/views/org/settings/OrgSettingsWrapper.vue'),
        meta: { authentication: 'required' },
        props: true,
        children: [
          {
            path: 'secrets',
            name: 'org-settings-secrets',
            component: (): Component => import('~/views/org/settings/OrgSecrets.vue'),
            props: true,
          },
          {
            path: 'registries',
            name: 'org-settings-registries',
            component: (): Component => import('~/views/org/settings/OrgRegistries.vue'),
            props: true,
          },
          {
            path: 'agents',
            name: 'org-settings-agents',
            component: (): Component => import('~/views/org/settings/OrgAgents.vue'),
            props: true,
          },
        ],
      },
    ],
  },
  {
    path: `${rootPath}/org/:orgName/:pathMatch(.*)*`,
    component: (): Component => import('~/views/org/OrgDeprecatedRedirect.vue'),
    props: true,
  },
  {
    path: `${rootPath}/user`,
    component: (): Component => import('~/views/user/UserWrapperSidebar.vue'),
    meta: { authentication: 'required' },
    props: true,
    children: [
      {
        path: '',
        name: 'user',
        component: (): Component => import('~/views/user/UserGeneral.vue'),
        props: true,
      },
      {
        path: 'secrets',
        name: 'user-secrets',
        component: (): Component => import('~/views/user/UserSecrets.vue'),
        props: true,
      },
      {
        path: 'registries',
        name: 'user-registries',
        component: (): Component => import('~/views/user/UserRegistries.vue'),
        props: true,
      },
      {
        path: 'crons',
        name: 'user-crons',
        component: (): Component => import('~/views/user/UserCrons.vue'),
        props: true,
      },
      {
        path: 'cli-and-api',
        name: 'user-cli-and-api',
        component: (): Component => import('~/views/user/UserCLIAndAPI.vue'),
        props: true,
      },
      {
        path: 'access-tokens',
        name: 'user-access-tokens',
        component: (): Component => import('~/views/user/UserAccessTokens.vue'),
        props: true,
      },
      {
        path: 'agents',
        name: 'user-agents',
        component: (): Component => import('~/views/user/UserAgents.vue'),
        props: true,
      },
      {
        path: 'global-agents',
        name: 'user-global-agents',
        component: (): Component => import('~/views/user/GlobalAgents.vue'),
        props: true,
      },
      {
        path: 'metrics',
        name: 'user-metrics',
        component: (): Component => import('~/views/user/UserMetrics.vue'),
        props: true,
      },
      {
        path: 'integrations',
        name: 'user-integrations',
        component: (): Component => import('~/views/user/UserIntegrations.vue'),
        props: true,
      },
      {
        path: 'info',
        name: 'user-info',
        redirect: { name: 'user' },
      },
    ],
  },

  {
    path: `${rootPath}/admin`,
    component: (): Component => import('~/views/user/UserWrapperSidebar.vue'),
    props: true,
    meta: { authentication: 'required' },
    children: [
      {
        path: '',
        name: 'admin-settings',
        redirect: { name: 'admin-settings-maintenance' },
      },
      {
        path: 'secrets',
        name: 'admin-settings-secrets',
        component: (): Component => import('~/views/admin/AdminSecrets.vue'),
        props: true,
      },
      {
        path: 'registries',
        name: 'admin-settings-registries',
        component: (): Component => import('~/views/admin/AdminRegistries.vue'),
        props: true,
      },
      {
        path: 'repos',
        name: 'admin-settings-repos',
        component: (): Component => import('~/views/admin/AdminRepos.vue'),
        props: true,
      },
      {
        path: 'entities',
        name: 'admin-settings-entities',
        component: (): Component => import('~/views/admin/AdminEntities.vue'),
        props: true,
      },
      {
        path: 'agents',
        name: 'admin-settings-agents',
        redirect: { name: 'agents' },
      },
      {
        path: 'autoscalers',
        name: 'admin-settings-autoscalers',
        component: (): Component => import('~/views/admin/AdminAutoscalers.vue'),
        props: true,
      },
      {
        path: 'queue',
        name: 'admin-settings-queue',
        component: (): Component => import('~/views/admin/AdminQueue.vue'),
        props: true,
      },
      {
        path: 'metrics',
        name: 'admin-settings-metrics',
        redirect: { name: 'user-metrics' },
      },
      {
        path: 'maintenance',
        name: 'admin-settings-maintenance',
        component: (): Component => import('~/views/admin/AdminMaintenance.vue'),
        props: true,
      },
    ],
  },
  {
    path: `${rootPath}/login`,
    name: 'login',
    component: (): Component => import('~/views/Login.vue'),
    meta: { blank: true },
    props: true,
  },
  {
    path: `${rootPath}/cli/auth`,
    component: (): Component => import('~/views/cli/Auth.vue'),
    meta: { authentication: 'required' },
  },

  // TODO: deprecated routes => remove after some time
  {
    path: `${rootPath}/:ownerOrOrgId`,
    redirect: (route) => ({ name: 'org', params: route.params }),
  },
  {
    path: `${rootPath}/:repoOwner/:repoName/:pathMatch(.*)*`,
    component: (): Component => import('~/views/repo/RepoDeprecatedRedirect.vue'),
    props: true,
  },

  // not found handler
  {
    path: `${rootPath}/:pathMatch(.*)*`,
    name: 'not-found',
    component: (): Component => import('~/views/NotFound.vue'),
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(async (to, _, next) => {
  const config = useUserConfig();
  const { redirectUrl } = config.userConfig.value;
  if (redirectUrl !== '' && to.name !== 'login') {
    config.setUserConfig('redirectUrl', '');

    // Reset auth store session state after successful login
    const authStore = useAuthStore();
    authStore.resetSessionState();

    // Clear any existing notifications when redirecting after successful login
    window.dispatchEvent(new CustomEvent('clearNotifications'));
    next(redirectUrl);
    return;
  }

  const authStore = useAuthStore();
  const authenticationRequired = to.matched.some((record) => record.meta.authentication === 'required');
  if (authenticationRequired && !authStore.isAuthenticated) {
    next({ name: 'login', query: { url: to.fullPath } });
    return;
  }

  next();
});

// Handle stale chunk errors after deployments - when JS chunks have been replaced
// but user still has old tab open, dynamic imports fail. Auto-reload to get new assets.
// eslint-disable-next-line promise/prefer-await-to-callbacks -- Vue Router API requires callback
router.onError((error: Error, to) => {
  const isChunkError =
    error.message.includes('dynamically imported module') ||
    error.message.includes('Failed to fetch dynamically imported module') ||
    error.message.includes('Loading chunk') ||
    error.message.includes('Loading CSS chunk');

  if (isChunkError) {
    // Reload to the target route to get fresh assets
    window.location.href = to.fullPath;
  }
});

export default router;
