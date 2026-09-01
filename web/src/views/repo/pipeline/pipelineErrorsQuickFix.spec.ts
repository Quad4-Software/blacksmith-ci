import { describe, expect, it } from 'vitest';

import { WebhookEvents } from '~/lib/api/types';
import type { PipelineError, SecretWithOrg } from '~/lib/api/types';

import {
  addQuickFixEventToSecret,
  getSecretEventQuickFix,
  resolveScopedSecretForQuickFix,
} from './pipelineErrorsQuickFix';

describe('pipelineErrorsQuickFix', () => {
  it('returns quick-fix data for compiler errors with a supported event', () => {
    const error: PipelineError = {
      type: 'compiler',
      message: 'Secret "token" is not allowed to be used with pipeline event "pull_request"',
      is_warning: false,
      data: {
        secret_name: 'token',
        event: WebhookEvents.PullRequest,
      },
    };

    expect(getSecretEventQuickFix(error)).toEqual({
      secretName: 'token',
      event: WebhookEvents.PullRequest,
    });
  });

  it('ignores compiler errors without structured quick-fix data', () => {
    const error: PipelineError = {
      type: 'compiler',
      message: 'plain compiler error',
      is_warning: false,
      data: {
        step: 'test',
      },
    };

    expect(getSecretEventQuickFix(error)).toBeNull();
  });

  it('resolves secrets using repo, then org, then global precedence', () => {
    const secrets: SecretWithOrg[] = [
      {
        id: '1',
        name: 'shared',
        value: '',
        repo_id: 0,
        org_id: 0,
        events: [],
        images: [],
        created_at: 0,
        updated_at: 0,
        org_name: '',
        scope_type: 'global',
      },
      {
        id: '2',
        name: 'shared',
        value: '',
        repo_id: 0,
        org_id: 42,
        events: [],
        images: [],
        created_at: 0,
        updated_at: 0,
        org_name: 'acme',
        scope_type: 'org',
      },
      {
        id: '3',
        name: 'shared',
        value: '',
        repo_id: 99,
        org_id: 0,
        events: [],
        images: [],
        created_at: 0,
        updated_at: 0,
        org_name: 'acme',
        repo_name: 'crow',
        scope_type: 'repo',
      },
    ];

    expect(resolveScopedSecretForQuickFix(secrets, 'shared', 99, 42)?.id).toBe('3');
    expect(resolveScopedSecretForQuickFix(secrets, 'shared', 100, 42)?.id).toBe('2');
    expect(resolveScopedSecretForQuickFix(secrets, 'shared', 100, 43)?.id).toBe('1');
  });

  it('adds the missing event without duplicating existing entries', () => {
    const secret: SecretWithOrg = {
      id: '3',
      name: 'shared',
      value: '',
      repo_id: 99,
      org_id: 0,
      events: [WebhookEvents.Push],
      images: [],
      created_at: 0,
      updated_at: 0,
      org_name: 'acme',
      repo_name: 'crow',
      scope_type: 'repo',
    };

    expect(addQuickFixEventToSecret(secret, WebhookEvents.PullRequest).events).toEqual([
      WebhookEvents.Push,
      WebhookEvents.PullRequest,
    ]);
    expect(addQuickFixEventToSecret(secret, WebhookEvents.Push).events).toEqual([WebhookEvents.Push]);
  });
});
