import { WebhookEvents } from '~/lib/api/types';
import type { CompilerErrorData, PipelineError, SecretWithOrg } from '~/lib/api/types';

const quickFixableSecretEvents = new Set<WebhookEvents>([
  WebhookEvents.Push,
  WebhookEvents.Tag,
  WebhookEvents.Release,
  WebhookEvents.PullRequest,
  WebhookEvents.Deploy,
  WebhookEvents.Cron,
  WebhookEvents.Manual,
]);

export interface SecretEventQuickFix {
  secretName: string;
  event: WebhookEvents;
}

export function getSecretEventQuickFix(error: PipelineError): SecretEventQuickFix | null {
  if (error.type !== 'compiler') {
    return null;
  }

  const data = error.data as CompilerErrorData | undefined;
  const secretName = data?.secret_name;
  const event = data?.event;
  if (secretName == null || secretName === '' || event == null || event === '') {
    return null;
  }
  if (!quickFixableSecretEvents.has(event as WebhookEvents)) {
    return null;
  }

  return {
    secretName,
    event: event as WebhookEvents,
  };
}

export function resolveScopedSecretForQuickFix(
  secrets: SecretWithOrg[],
  secretName: string,
  repoId: number,
  orgId: number,
): SecretWithOrg | null {
  return (
    secrets.find((secret) => secret.scope_type === 'repo' && secret.repo_id === repoId && secret.name === secretName) ??
    secrets.find((secret) => secret.scope_type === 'org' && secret.org_id === orgId && secret.name === secretName) ??
    secrets.find((secret) => secret.scope_type === 'global' && secret.name === secretName) ??
    null
  );
}

export function addQuickFixEventToSecret(secret: SecretWithOrg, event: WebhookEvents): SecretWithOrg {
  const currentEvents = secret.events ?? [];
  if (currentEvents.includes(event)) {
    return secret;
  }

  return {
    ...secret,
    events: [...currentEvents, event],
  };
}
