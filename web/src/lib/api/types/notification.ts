export type NotificationEvent = 'success' | 'failure' | 'status_change';

export interface SMTPStatus {
  configured: boolean;
  host?: string;
  port?: number;
  from?: string;
  auth_type?: string;
}

export interface SMTPTestResult {
  success: boolean;
  message: string;
}
