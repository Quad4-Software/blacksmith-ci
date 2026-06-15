#!/usr/bin/env bash
# Ensures a valid OpenBao token exists for the given environment.
# Extracts BAO_ADDR from the environment's .envrc and checks token validity.
# Only triggers OIDC login if the token is missing or expired.
#
# Usage: ensure-bao-token

set -euo pipefail

envrc=".envrc"
if [ ! -f "$envrc" ]; then
  echo "Error: $envrc not found" >&2
  exit 1
fi

export BAO_ADDR=$(grep '^export BAO_ADDR=' "$envrc" | head -1 | sed 's/^export BAO_ADDR=//')
if [ -z "${BAO_ADDR:-}" ]; then
  exit 0  # no BAO_ADDR in .envrc, nothing to do
fi

if bao token lookup &>/dev/null; then
  exit 0  # token is valid
fi

echo "OpenBao token for $BAO_ADDR is missing or expired, logging in..."
bao login -method=oidc
