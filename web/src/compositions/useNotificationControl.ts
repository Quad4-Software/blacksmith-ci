/**
 * Shared notification control state.
 *
 * Provides suppress404Errors() for temporarily suppressing 404 error
 * notifications (e.g., after deleting a resource and navigating away).
 */

let suppress404Until = 0;

export function suppress404Errors(durationMs = 2000) {
  suppress404Until = Date.now() + durationMs;
}

export function is404Suppressed(): boolean {
  return Date.now() < suppress404Until;
}
