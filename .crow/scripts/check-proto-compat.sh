#!/bin/sh
# Fail when crow.proto changes in a way older agents cannot read, unless the
# change also raises MinAgentVersion so the server rejects those agents with a
# clear error instead of misreading their messages.
set -eu

BASE_REF="${1:?usage: check-proto-compat.sh <base-ref>}"
PROTO=pipeline/rpc/proto/crow.proto
VERSION_FILE=pipeline/rpc/proto/version.go
BASE_DIR="$(mktemp -d)"
trap 'rm -rf "$BASE_DIR"' EXIT

mkdir -p "$BASE_DIR/$(dirname "$PROTO")"
git show "$BASE_REF:$PROTO" >"$BASE_DIR/$PROTO"

CONFIG='{"version":"v2","modules":[{"path":"pipeline/rpc/proto"}],"breaking":{"use":["WIRE"]}}'
if buf breaking . --config "$CONFIG" --against "$BASE_DIR" --against-config "$CONFIG"; then
  echo "crow.proto stays wire compatible with $BASE_REF"
  exit 0
fi

min_agent() {
  sed -n 's/^const MinAgentVersion int32 = \([0-9]*\)$/\1/p'
}
BASE_MIN="$(git show "$BASE_REF:$VERSION_FILE" | min_agent)"
if [ -z "$BASE_MIN" ]; then
  # before negotiation, agents required an exact match with Version
  BASE_MIN="$(git show "$BASE_REF:$VERSION_FILE" | sed -n 's/^const Version int32 = \([0-9]*\)$/\1/p')"
fi
HEAD_MIN="$(min_agent <"$VERSION_FILE")"

if [ "$HEAD_MIN" -gt "$BASE_MIN" ]; then
  echo "crow.proto breaks older agents and MinAgentVersion was raised from $BASE_MIN to $HEAD_MIN"
  exit 0
fi

echo "crow.proto breaks agents that speak older protocol versions."
echo "Keep the change additive (new fields or RPCs, reserve removed field numbers),"
echo "or raise MinAgentVersion in $VERSION_FILE (currently $HEAD_MIN) to drop support for them."
exit 1
