const PORT_REGEX = /^\d{1,5}$/;

// cliTokenUrl builds the URL of the CLI's local token listener. The port comes
// from the `?port=` query parameter, which anyone can craft, so only a plain
// decimal port in 1-65535 is accepted and the host is pinned to localhost.
// Anything else (e.g. `1@evil.com`) yields null so the token is never sent.
export function cliTokenUrl(port: unknown): URL | null {
  if (typeof port !== 'string' || !PORT_REGEX.test(port)) {
    return null;
  }
  const portNumber = Number(port);
  if (portNumber < 1 || portNumber > 65535) {
    return null;
  }

  const url = new URL('http://localhost');
  url.port = String(portNumber);
  url.pathname = '/token';
  if (url.hostname !== 'localhost') {
    return null;
  }
  return url;
}

// sendCliToken posts the token to the CLI listener and reports whether the CLI
// acknowledged it. A listener that is down or answers garbage counts as failure.
export async function sendCliToken(url: URL, token: string): Promise<boolean> {
  try {
    const resp = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    });
    const data = (await resp.json()) as { ok?: unknown };
    return data.ok === 'true';
  } catch {
    return false;
  }
}
