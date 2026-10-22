import { provisioningPhaseStates, wholeMinutes } from '~/lib/provisioning';
import { describe, expect, it } from 'vitest';

describe('provisioningPhaseStates', () => {
  it('marks the newest phase as in progress', () => {
    expect(
      provisioningPhaseStates([
        { phase: 'Installing Hyper-V', at: 1 },
        { phase: 'Building the base image', at: 2 },
      ]),
    ).toEqual(['done', 'current']);
  });

  it('keeps a failed phase failed, even the newest', () => {
    expect(
      provisioningPhaseStates([
        { phase: 'Installing Hyper-V', at: 1 },
        { phase: 'Building the base image', failed: true, at: 2 },
      ]),
    ).toEqual(['done', 'failed']);
  });

  it('handles no phases', () => {
    expect(provisioningPhaseStates([])).toEqual([]);
  });
});

describe('wholeMinutes', () => {
  it('rounds to minutes and never shows zero', () => {
    expect(wholeMinutes(0)).toBe(1);
    expect(wholeMinutes(89)).toBe(1);
    expect(wholeMinutes(90)).toBe(2);
    expect(wholeMinutes(1800)).toBe(30);
  });
});
