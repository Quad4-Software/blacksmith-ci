// A config for a pipeline.
export interface PipelineConfig {
  hash: string;
  name: string;
  data: string;
  // Set on configs the server generated rather than fetched from the forge:
  // names the .jsonnet source this YAML document was evaluated from.
  generated_from?: string;
}
