import { describe, expect, it } from 'vitest';

import { watchKeyFor, watchLabel } from './watchKey';

const base = { event: 'push', branch: '', sender: '', deploy_to: '' };

describe('watchKeyFor', () => {
  it.each([
    ['push uses branch', { ...base, event: 'push', branch: 'main' }, { event: 'push', target: 'main' }],
    [
      'manual uses branch',
      { ...base, event: 'manual', branch: 'release/2.x' },
      { event: 'manual', target: 'release/2.x' },
    ],
    [
      'cron uses cron name',
      { ...base, event: 'cron', sender: 'nightly', branch: 'main' },
      { event: 'cron', target: 'nightly' },
    ],
    [
      'deployment uses target',
      { ...base, event: 'deployment', deploy_to: 'production', branch: 'main' },
      { event: 'deployment', target: 'production' },
    ],
    ['tag has empty target', { ...base, event: 'tag', branch: 'v1' }, { event: 'tag', target: '' }],
    ['release has empty target', { ...base, event: 'release', branch: 'v1' }, { event: 'release', target: '' }],
    ['pull_request not watchable', { ...base, event: 'pull_request', branch: 'main' }, null],
    ['pull_request_closed not watchable', { ...base, event: 'pull_request_closed' }, null],
    ['pull_request_merged not watchable', { ...base, event: 'pull_request_merged' }, null],
    ['pull_request_edited not watchable', { ...base, event: 'pull_request_edited' }, null],
  ])('%s', (_name, pipeline, want) => {
    expect(watchKeyFor(pipeline as never)).toEqual(want);
  });
});

describe('watchLabel', () => {
  it.each([
    ['push', 'main', 'push → main'],
    ['manual', 'main', 'manual → main'],
    ['cron', 'nightly', 'cron · nightly'],
    ['deployment', 'production', 'deploy → production'],
    ['tag', '', 'tag'],
    ['release', '', 'release'],
  ])('%s %s', (event, target, want) => {
    expect(watchLabel(event, target)).toBe(want);
  });
});
