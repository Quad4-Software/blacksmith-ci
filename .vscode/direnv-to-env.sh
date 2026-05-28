#!/usr/bin/env bash
# Snapshot the worktree's direnv state into .env so the VS Code debugger can
# consume it via launch.json's envFile. Run from the direnv-export preLaunchTask
# before each debug session. Always exits 0 so a missing tool / unallowed .envrc
# never blocks the debugger.

cd "$(dirname "$0")/.." || exit 0

if ! command -v direnv >/dev/null 2>&1; then
  echo "direnv-export: direnv not in PATH; .env left untouched"
  exit 0
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "direnv-export: jq not in PATH; .env left untouched"
  exit 0
fi
if [ ! -f .envrc ]; then
  echo "direnv-export: no .envrc in $(pwd); .env left untouched"
  exit 0
fi

# direnv export json prints JSON of vars .envrc would apply; null = unset.
# Status lines go to stderr; route them away from stdout.
json=$(direnv export json 2>/dev/null)
if [ -z "$json" ]; then
  echo "direnv-export: 'direnv export json' returned nothing; try 'direnv allow' in this worktree"
  exit 0
fi
if ! printf '%s' "$json" | jq -e . >/dev/null 2>&1; then
  echo "direnv-export: 'direnv export json' returned invalid JSON; skipping"
  exit 0
fi

tmp=$(mktemp) || exit 0
printf '%s' "$json" \
  | jq -r 'to_entries | map(select(.value != null)) | .[] | "\(.key)=\(.value)"' \
  > "$tmp" 2>/dev/null
mv "$tmp" .env 2>/dev/null || { echo "direnv-export: failed to write .env"; exit 0; }
echo "direnv-export: wrote $(wc -l <.env | tr -d ' ') env var(s) from .envrc to .env"
exit 0
