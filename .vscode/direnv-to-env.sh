#!/usr/bin/env bash
# Snapshot the worktree's direnv state into .env so the VS Code debugger can
# consume it via launch.json's envFile. Run from the direnv-export preLaunchTask
# before each debug session.
#
# Exit policy: optional-tooling cases (direnv/jq absent, no .envrc) exit 0 so a
# machine that simply doesn't use direnv never blocks the debugger. A *blocked*
# .envrc is different — it's a real misconfiguration (most often: a fresh git
# worktree whose .envrc was never `direnv allow`-ed; allow is keyed on the
# .envrc's absolute path + content hash, so allowing the main checkout does NOT
# carry over to a worktree). That case exits non-zero so VS Code surfaces it
# instead of silently launching with an empty environment.

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

# direnv export json prints JSON of vars .envrc would apply (null = unset) on
# stdout, and status/error lines on stderr. Capture both in a single run — the
# .envrc fetches secrets from openbao, so we must not invoke it twice.
errfile=$(mktemp) || exit 0
json=$(direnv export json 2>"$errfile")
err=$(cat "$errfile" 2>/dev/null)
rm -f "$errfile"

# blocked_warning prints the actionable "run direnv allow" guidance.
blocked_warning() {
  {
    echo ""
    echo "direnv-export: this worktree's .envrc is BLOCKED — real config was not loaded."
    echo "The debugger would start with an empty environment (missing CROW_HOST, tokens,"
    echo "etc). Run this once inside the worktree, then relaunch:"
    echo ""
    echo "    direnv allow $(pwd)"
    echo ""
    echo "Each git worktree needs its own 'direnv allow' — the allow is keyed on the"
    echo ".envrc path + content hash, so allowing the main checkout does not carry over"
    echo "here, and it must be re-run whenever .envrc changes."
    echo ""
  } >&2
}

# A blocked or errored .envrc does NOT make 'direnv export json' fail: from a
# non-hooked shell (like the VS Code task) it silently emits ONLY direnv's own
# DIRENV_* bookkeeping keys and skips the .envrc body. So we can't rely on an
# empty result or an error string — we check that a var the .envrc is supposed
# to set (CROW_HOST) actually made it through.
if [ -z "$json" ] || ! printf '%s' "$json" | jq -e . >/dev/null 2>&1; then
  if printf '%s' "$err" | grep -qi 'blocked'; then
    blocked_warning
    exit 1
  fi
  echo "direnv-export: 'direnv export json' returned nothing usable; .env left untouched" >&2
  exit 0
fi
if ! printf '%s' "$json" | jq -e 'has("CROW_HOST")' >/dev/null 2>&1; then
  blocked_warning
  exit 1
fi

# Write every real var, but drop direnv's own DIRENV_* bookkeeping — those are
# internal state, not app config, and would pollute .env.
tmp=$(mktemp) || exit 0
printf '%s' "$json" \
  | jq -r 'to_entries | map(select(.value != null and (.key | startswith("DIRENV_") | not))) | .[] | "\(.key)=\(.value)"' \
  > "$tmp" 2>/dev/null
mv "$tmp" .env 2>/dev/null || { echo "direnv-export: failed to write .env"; exit 0; }
echo "direnv-export: wrote $(wc -l <.env | tr -d ' ') env var(s) from .envrc to .env"
exit 0
