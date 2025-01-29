import type { PipelineStatus } from '~/lib/api/types';

export const pipelineStatusColors: Record<PipelineStatus, 'green' | 'gray' | 'red' | 'blue' | 'yellow' | 'orange'> = {
  blocked: 'gray',
  declined: 'red',
  error: 'red',
  failure: 'red',
  killed: 'gray',
  pending: 'yellow',
  skipped: 'gray',
  running: 'orange',
  started: 'orange',
  success: 'green',
};
