import type { Agent } from '~/lib/api/types';

type Translator = (key: string) => string;

export function formatBytes(bytes: number, t: Translator): string {
  if (bytes === 0) return t('admin.settings.agents.unlimited');
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${Math.round((bytes / k ** i) * 100) / 100} ${sizes[i]}`;
}

export function formatCPUQuota(quota: number, t: Translator): string {
  // CPU quota is in microseconds per 100ms period.
  // 100000 = 1 CPU core, 200000 = 2 CPU cores, etc.
  if (quota === 0) return t('admin.settings.agents.unlimited');
  const cores = quota / 100000;
  return cores >= 1 ? `${cores}` : `${quota}µs`;
}

export function getAgentDisplayName(agent: Agent): string {
  return agent.name || `Agent ${agent.id}`;
}

export function isAgentVersionMismatch(
  agentVersion: string | undefined | null,
  serverVersion: string | undefined | null,
): boolean {
  if (agentVersion == null || agentVersion === '') return false;
  if (serverVersion == null || serverVersion === '') return false;
  return agentVersion !== serverVersion;
}

export function getAgentStatusColor(agent: Agent): string {
  if (agent.no_schedule) return 'bg-muted-foreground';
  if (!agent.last_contact || agent.last_contact === 0) return 'bg-red-500';
  if (!agent.workflow_polling_healthy) return 'bg-red-500';
  const secsSince = Date.now() / 1000 - agent.last_contact;
  if (secsSince < 60) return 'bg-green-500';
  if (secsSince < 300) return 'bg-yellow-500';
  return 'bg-red-500';
}

export function getAgentStatusTitle(agent: Agent, t: Translator): string {
  if (agent.no_schedule) return t('admin.settings.agents.status.disabled');
  if (!agent.last_contact || agent.last_contact === 0) return t('admin.settings.agents.status.never_connected');
  if (!agent.workflow_polling_healthy) return t('admin.settings.agents.status.polling_unhealthy');
  const secsSince = Date.now() / 1000 - agent.last_contact;
  if (secsSince < 60) return t('admin.settings.agents.status.healthy');
  if (secsSince < 300) return t('admin.settings.agents.status.unstable');
  return t('admin.settings.agents.status.offline');
}

export function getAgentStatusAnimation(agent: Agent): string {
  if (agent.no_schedule || !agent.last_contact || agent.last_contact === 0) return '';
  const secsSince = Date.now() / 1000 - agent.last_contact;
  return secsSince < 60 ? 'animate-pulse-slow' : '';
}
