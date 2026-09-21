import { mount } from '@vue/test-utils';
import { encode } from 'js-base64';
import { ref } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import type { PipelineConfig as PipelineConfigData } from '~/lib/api/types';
import PipelineConfig from './PipelineConfig.vue';

vi.mock('~/compositions/useCrowTitle', () => ({ useCrowTitle: vi.fn() }));

const route = ref<{ query: Record<string, string> }>({ query: {} });
vi.mock('vue-router', () => ({ useRoute: () => route.value }));

function config(name: string, data: string, generatedFrom?: string): PipelineConfigData {
  return { hash: name, name, data: encode(data), generated_from: generatedFrom };
}

const JSONNET_SOURCE = config('.crow/matrix.jsonnet', '[ { name: "build-" + os } for os in ["linux"] ]');
const LIBSONNET = config('.crow/lib.libsonnet', '{ step():: {} }');
const GENERATED = config(
  '.crow/matrix-0.yaml',
  'name: build-linux\nsteps:\n  build:\n    image: alpine\n',
  '.crow/matrix.jsonnet',
);
const PLAIN = config('.crow/release.yaml', 'name: release\n');

function mountConfig(configs: PipelineConfigData[], query: Record<string, string> = {}) {
  route.value = { query };
  return mount(PipelineConfig, {
    global: {
      provide: {
        repo: ref({ id: 1, full_name: 'owner/repo' }),
        pipeline: ref({ id: 1, number: 1 }),
        'pipeline-configs': ref(configs),
      },
      mocks: { $t: (key: string) => key },
      stubs: { SyntaxHighlight: true },
    },
  });
}

describe('pipeline config tab', () => {
  it('tags a generated document with the source it was evaluated from', () => {
    const wrapper = mountConfig([LIBSONNET, JSONNET_SOURCE, GENERATED, PLAIN]);

    const generatedPanel = wrapper
      .findAll('[data-slot="badge"]')
      .map((badge) => badge.text())
      .join(' ');
    expect(generatedPanel).toContain('repo.pipeline.config_generated');
    expect(generatedPanel).toContain('repo.pipeline.config_generated_from_short');
    expect(generatedPanel).toContain('repo.pipeline.config_import_only');
    expect(generatedPanel).toContain('repo.pipeline.config_source');

    wrapper.unmount();
  });

  it('shows no badges when the repo uses plain YAML only', () => {
    const wrapper = mountConfig([PLAIN, config('.crow/test.yaml', 'name: test\n')]);

    expect(wrapper.findAll('[data-slot="badge"]')).toHaveLength(0);

    wrapper.unmount();
  });

  it('expands generated documents and collapses the jsonnet they came from', () => {
    const wrapper = mountConfig([LIBSONNET, JSONNET_SOURCE, GENERATED, PLAIN]);

    // The generated YAML is what you came to this tab to read, so only it and the
    // hand-written config render their contents; the sources stay folded away.
    const visible = wrapper.text();
    expect(visible).toContain('.crow/matrix-0.yaml');
    expect(visible).toContain('.crow/matrix.jsonnet');

    const openPanels = wrapper.findAll('[data-state="open"]');
    expect(openPanels.length).toBeGreaterThan(0);
    const openTitles = openPanels.map((panel) => panel.text());
    expect(openTitles.some((title) => title.includes('matrix-0.yaml'))).toBe(true);
    expect(openTitles.some((title) => title.includes('matrix.jsonnet'))).toBe(false);
    expect(openTitles.some((title) => title.includes('lib.libsonnet'))).toBe(false);

    wrapper.unmount();
  });

  it('highlights the generated document when an error points at its jsonnet source', () => {
    // A single-document Jsonnet keeps its .jsonnet name through the step builder,
    // so pipeline errors report that name even though the line number belongs to
    // the YAML it evaluated to.
    const wrapper = mountConfig([JSONNET_SOURCE, GENERATED], { file: '.crow/matrix.jsonnet', line: '1' });

    const highlighted = wrapper.findAll('tr.bg-primary\\/10');
    expect(highlighted).toHaveLength(1);
    expect(highlighted[0].attributes('id')).toBe('.crow/matrix-0.yaml-L1');

    wrapper.unmount();
  });
});
