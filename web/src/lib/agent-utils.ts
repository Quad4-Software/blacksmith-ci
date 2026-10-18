import type { AgentProtocol } from '~/compositions/useConfig';
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

export function formatHostCores(cores: number, t: Translator): string {
  return `${cores} ${t('admin.settings.agents.cores')}`;
}

// hasAgentResources reports whether there is anything to show in the resource
// section: either a host total reported by the agent, or the Docker limits that
// have always been displayed for the Docker backend.
export function hasAgentResources(agent: Agent): boolean {
  return agent.host_cpus > 0 || agent.host_mem > 0 || agent.backend === 'docker';
}

// formatAgentMemory renders the per-workflow memory limit against the host
// total, e.g. `4 GB / 64 GB`. Either side is dropped when unknown; with neither
// known it falls back to the limit, which renders as "unlimited".
export function formatAgentMemory(agent: Agent, t: Translator): string {
  const host = agent.host_mem > 0 ? formatBytes(agent.host_mem, t) : null;
  if (host === null) {
    return formatBytes(agent.limit_mem, t);
  }
  return agent.limit_mem > 0 ? `${formatBytes(agent.limit_mem, t)} / ${host}` : host;
}

// formatAgentCPU renders the per-workflow CPU quota against the host core
// count, e.g. `2 / 16 cores`. See formatAgentMemory for the fallbacks.
export function formatAgentCPU(agent: Agent, t: Translator): string {
  const host = agent.host_cpus > 0 ? formatHostCores(agent.host_cpus, t) : null;
  if (host === null) {
    return formatCPUQuota(agent.limit_cpu_quota, t);
  }
  return agent.limit_cpu_quota > 0 ? `${formatCPUQuota(agent.limit_cpu_quota, t)} / ${host}` : host;
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

// AgentCompatibility describes how an agent relates to the server it reports to:
// - incompatible: the server rejects the agent until it is updated
// - outdated: the agent works, but features added since its protocol are unavailable
// - release_differs: a different Crow release, but nothing is missing
// - current: same release as the server
export type AgentCompatibility = 'incompatible' | 'outdated' | 'release_differs' | 'current';

export function getAgentCompatibility(
  agent: Pick<Agent, 'version' | 'protocol_version'>,
  serverVersion: string | undefined | null,
  serverProtocol: AgentProtocol | null,
): AgentCompatibility {
  // protocol_version is 0 until the agent registers with a server that records it.
  if (agent.protocol_version > 0 && serverProtocol !== null) {
    if (agent.protocol_version < serverProtocol.min_agent) return 'incompatible';
    if (agent.protocol_version < serverProtocol.version) return 'outdated';
  }
  return isAgentVersionMismatch(agent.version, serverVersion) ? 'release_differs' : 'current';
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
