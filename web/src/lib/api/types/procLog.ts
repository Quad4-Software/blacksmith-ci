export interface ProcLogEntry {
  id: number;
  source_type: 'agent' | 'autoscaler';
  source_id: number;
  time: number; // unix nanos, producer-side
  level: 'debug' | 'info' | 'warn' | 'error' | 'trace' | 'fatal';
  line: string;
}
