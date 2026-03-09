export type NotificationEvent = 'success' | 'failure' | 'status_change';

export interface NotificationConfig {
  id: number;
  repo_id: number;
  is_enabled: boolean;
  events: NotificationEvent[];
  created_at: number;
  updated_at: number;
}

export interface SMTPStatus {
  configured: boolean;
  host?: string;
  port?: number;
  from?: string;
}

export interface SMTPTestResult {
  success: boolean;
  message: string;
}
