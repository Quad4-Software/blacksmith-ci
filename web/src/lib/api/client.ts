import type { useAuthStore } from '~/store/auth';

export interface ApiError {
  status: number;
  message: string;
  config?: {
    method?: string;
    url?: string;
  };
}

// Thrown when a pipeline was not created because all steps were filtered out by `when` conditions
export class PipelineFilteredError extends Error {
  constructor() {
    super('Pipeline was not created because all steps were filtered out by when conditions');
    this.name = 'PipelineFilteredError';
  }
}

type QueryParams = Record<string, string | number | boolean>;

export function encodeQueryString(_params: unknown = {}): string {
  const __params = _params as QueryParams;
  const params: QueryParams = {};

  Object.keys(__params).forEach((key) => {
    const val = __params[key];
    if (val !== undefined) {
      params[key] = val;
    }
  });

  return Object.keys(params)
    .sort()
    .map((key) => {
      const val = params[key];
      return `${encodeURIComponent(key)}=${encodeURIComponent(val)}`;
    })
    .join('&');
}

export default class ApiClient {
  server: string;

  token: string | null;

  csrf: string | null;

  onerror: ((err: ApiError) => void) | undefined;

  // Auth store reference for session management
  private authStore?: ReturnType<typeof useAuthStore>;

  constructor(server: string, token: string | null, csrf: string | null) {
    this.server = server;
    this.token = token;
    this.csrf = csrf;
  }

  /**
   * Sets the auth store reference for session management.
   * Must be called after Pinia is initialized.
   */
  setAuthStore(store: ReturnType<typeof useAuthStore>) {
    this.authStore = store;
  }

  private async _request(method: string, path: string, data?: unknown): Promise<unknown> {
    // Check if session is expired or we're redirecting to login
    // This prevents additional requests during session invalidation
    if (this.authStore?.isSessionExpired || this.authStore?.isRedirecting) {
      throw new Error('Session expired');
    }

    // Create AbortController for this request
    const abortController = new AbortController();
    this.authStore?.registerAbortController(abortController);

    try {
      const res = await fetch(`${this.server}${path}`, {
        method,
        signal: abortController.signal,
        headers: {
          ...(method !== 'GET' && this.csrf !== null ? { 'X-CSRF-TOKEN': this.csrf } : {}),
          ...(this.token !== null ? { Authorization: `Bearer ${this.token}` } : {}),
          ...(data !== undefined ? { 'Content-Type': 'application/json' } : {}),
        },
        body: data !== undefined ? JSON.stringify(data) : undefined,
      });

      if (!res.ok) {
        const resText = await res.text();
        let message: string;
        if (resText) {
          // Try to parse RFC 9457 Problem Details JSON (from huma error responses)
          try {
            const problem = JSON.parse(resText) as { detail?: string; title?: string };
            if (typeof problem.detail === 'string' && problem.detail !== '') {
              message = problem.detail;
            } else if (typeof problem.title === 'string' && problem.title !== '') {
              message = problem.title;
            } else {
              message = resText;
            }
          } catch {
            // Not JSON — use the plain text response
            message = res.statusText ? `${res.statusText}: ${resText}` : resText;
          }
        } else {
          message = res.statusText || `HTTP ${res.status}`;
        }
        const error: ApiError = {
          status: res.status,
          message,
          config: {
            method,
            url: path,
          },
        };

        // Trigger session invalidation on 401 only for authenticated users
        // Anonymous users getting 401 should not be redirected - they're already shown the Login button
        // Stale forge token errors should NOT invalidate the Crow session —
        // the session cookie is still valid, only the forge's OAuth token is broken.
        // These are handled by the error handler in App.vue which shows a notification instead.
        const isStaleForgeToken =
          message.includes('TOKEN_REFRESH_FAILED') ||
          message.includes('authorization has expired') ||
          message.includes('could not be refreshed');
        if (res.status === 401 && this.authStore?.isAuthenticated && !isStaleForgeToken) {
          this.authStore.invalidateSession();
        }

        if (this.onerror) {
          this.onerror(error);
        }
        throw new Error(message);
      }

      // Check for Pipeline-Filtered response (204 No Content with header)
      // This indicates all steps were filtered out by `when` conditions
      if (res.status === 204 && res.headers.get('Pipeline-Filtered') === 'true') {
        throw new PipelineFilteredError();
      }

      const contentType = res.headers.get('Content-Type');
      if (contentType !== null && contentType.startsWith('application/json')) {
        return await res.json();
      }

      return await res.text();
    } catch (error) {
      // Re-throw abort errors without additional processing
      if (error instanceof Error && error.name === 'AbortError') {
        throw error;
      }
      throw error;
    } finally {
      this.authStore?.removeAbortController(abortController);
    }
  }

  async _get(path: string) {
    return this._request('GET', path);
  }

  async _post(path: string, data?: unknown) {
    return this._request('POST', path, data);
  }

  async _patch(path: string, data?: unknown) {
    return this._request('PATCH', path, data);
  }

  async _put(path: string, data?: unknown) {
    return this._request('PUT', path, data);
  }

  async _delete(path: string, data?: unknown) {
    return this._request('DELETE', path, data);
  }

  _subscribe<T>(path: string, callback: (data: T) => void, opts = { reconnect: true }) {
    // Check if session is expired before creating the EventSource
    // This prevents creating connections that would immediately fail
    if (this.authStore?.isSessionExpired) {
      throw new Error('Session expired');
    }

    const query = encodeQueryString({
      access_token: this.token ?? undefined,
    });
    let _path = this.server ? this.server + path : path;
    _path = this.token !== null ? `${_path}?${query}` : _path;

    const events = new EventSource(_path);

    // Register IMMEDIATELY after creation to avoid race conditions during invalidation
    this.authStore?.registerEventSource(events);

    // Double-check if session expired while we were creating the EventSource
    if (this.authStore?.isSessionExpired) {
      events.close();
      throw new Error('Session expired');
    }

    events.onmessage = (event) => {
      const data = JSON.parse(event.data as string) as T;
      // eslint-disable-next-line promise/prefer-await-to-callbacks
      callback(data);
    };

    events.onerror = (err) => {
      // Check for connection closed (could be 401 or network error)
      // EventSource doesn't expose HTTP status codes directly
      if (events.readyState === EventSource.CLOSED) {
        this.authStore?.unregisterEventSource(events);
      }

      // If session is expired, close the connection to prevent auto-reconnect loops
      // The native EventSource API will keep trying to reconnect with the same
      // invalid token unless we explicitly close it
      if (this.authStore?.isSessionExpired) {
        events.close();
        return;
      }

      if (!opts.reconnect) {
        // TODO: check if such events really have a data property
        if ((err as Event & { data: string }).data === 'eof') {
          events.close();
        }
      }
    };

    // Wrap close to unregister from auth store
    const originalClose = events.close.bind(events);
    events.close = () => {
      this.authStore?.unregisterEventSource(events);
      originalClose();
    };

    return events;
  }

  setErrorHandler(onerror: (err: ApiError) => void) {
    this.onerror = onerror;
  }
}
