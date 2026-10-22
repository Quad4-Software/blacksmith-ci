import type { ProvisioningPhase } from '~/lib/api/types';

export type ProvisioningPhaseState = 'done' | 'current' | 'failed';

// provisioningPhaseStates marks every phase done except the newest, which is
// still in progress unless it failed.
export function provisioningPhaseStates(phases: ProvisioningPhase[]): ProvisioningPhaseState[] {
  return phases.map((phase, index) => {
    if (phase.failed) return 'failed';
    return index === phases.length - 1 ? 'current' : 'done';
  });
}

// wholeMinutes rounds seconds to minutes for display, never below one.
export function wholeMinutes(seconds: number): number {
  return Math.max(1, Math.round(seconds / 60));
}
