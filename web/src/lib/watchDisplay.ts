import type { WatchOverviewItem } from '~/lib/api/types';

// The failing workflows of a watch, each with the run it last failed in, such
// as "build (#10), test (#12)". A pipeline-level failure has an empty name and
// is shown with the configError label.
export function failedNames(item: WatchOverviewItem, configError: string): string {
  return item.failing_workflows.map((w) => `${w.name || configError} (#${w.pipeline_number})`).join(', ');
}
