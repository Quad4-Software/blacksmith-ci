#!/usr/bin/env bash
# Refresh .env from direnv, then poll the Crow server's /readyz until it
# responds 200 (or 300 s timeout). Always exits 0 so the agent launches even
# if the server isn't up — its gRPC client retries transient errors.

cd "$(dirname "$0")/.." || exit 0

"$(dirname "$0")/direnv-to-env.sh" || true

for _ in $(seq 1 300); do
  if curl -sf http://localhost:8000/readyz >/dev/null 2>&1; then
    echo "wait-for-server: crow server ready on :8000"
    exit 0
  fi
  sleep 1
done

echo "wait-for-server: crow server did not become ready on :8000 within 300s; starting agent anyway (it retries gRPC on transient errors)"
exit 0
