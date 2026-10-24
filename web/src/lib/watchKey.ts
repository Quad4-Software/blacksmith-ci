import type { Pipeline } from '~/lib/api/types';

// Mirrors model.WatchKeyFor on the server: pull request events are not watchable.
export function watchKeyFor(
  p: Pick<Pipeline, 'event' | 'branch' | 'sender' | 'deploy_to'>,
): { event: string; target: string } | null {
  switch (p.event as string) {
    case 'push':
    case 'manual':
      return { event: p.event, target: p.branch };
    case 'cron':
      return { event: p.event, target: p.sender };
    case 'deployment':
      return { event: p.event, target: p.deploy_to };
    case 'tag':
    case 'release':
      return { event: p.event, target: '' };
    default:
      return null;
  }
}

export function watchLabel(event: string, target: string): string {
  switch (event) {
    case 'cron':
      return `cron · ${target}`;
    case 'deployment':
      return `deploy → ${target}`;
    case 'tag':
    case 'release':
      return event;
    default:
      return target ? `${event} → ${target}` : event;
  }
}
