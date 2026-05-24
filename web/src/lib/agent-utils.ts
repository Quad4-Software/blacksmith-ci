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
