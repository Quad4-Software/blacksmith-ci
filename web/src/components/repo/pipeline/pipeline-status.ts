import type { PipelineStatus } from '~/lib/api/types';

export const pipelineStatusColors: Record<
  PipelineStatus,
  'green' | 'gray' | 'red' | 'blue' | 'yellow' | 'orange' | 'amber'
> = {
  blocked: 'gray',
  declined: 'amber',
  error: 'red',
  failure: 'red',
  killed: 'red',
  pending: 'yellow',
  skipped: 'gray',
  running: 'orange',
  started: 'orange',
  success: 'green',
};
