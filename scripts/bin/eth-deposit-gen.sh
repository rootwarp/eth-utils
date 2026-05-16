#!/usr/bin/env bash
# eth-deposit-gen.sh — wrapper script for eth-deposit-gen
#
# Builds the binary if needed, then runs it with the supplied flags.
# Passphrase is read from a TTY prompt by default; use --passphrase-env to
# supply a variable name instead (the variable holds the passphrase, not
# this script).
#
# Usage:
#   ./scripts/bin/eth-deposit-gen.sh [OPTIONS]
#
# Options mirror the binary flags; run with --help to see them.
#
# Quick start (Hoodi):
#   ./scripts/bin/eth-deposit-gen.sh \
#     --network hoodi \
#     --validator-key-path ./my-keystore.json \
#     --pubkeys 0xabc123... \
#     --output-dir ./out
#
# Quick start with passphrase env var:
#   export KS_PASSPHRASE="my secret"
#   ./scripts/bin/eth-deposit-gen.sh \
#     --network hoodi \
#     --validator-key-path ./my-keystore.json \
#     --pubkeys 0xabc123... \
#     --output-dir ./out \
#     --passphrase-env KS_PASSPHRASE

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODULE_DIR="${REPO_ROOT}/go/cmd/eth-deposit-gen"
BIN="${MODULE_DIR}/bin/eth-deposit-gen"

# ── Build if binary is missing or stale ───────────────────────────────────────

needs_build() {
  [[ ! -x "${BIN}" ]] && return 0
  # Rebuild if any Go source is newer than the binary.
  if find "${MODULE_DIR}" -name '*.go' -newer "${BIN}" | grep -q .; then
    return 0
  fi
  return 1
}

if needs_build; then
  echo "Building eth-deposit-gen..." >&2
  (cd "${MODULE_DIR}" && CGO_ENABLED=1 go build -o bin/eth-deposit-gen .) || {
    echo "ERROR: build failed." >&2
    exit 1
  }
fi

# ── Safety check for mainnet ──────────────────────────────────────────────────

for arg in "$@"; do
  if [[ "${arg}" == "mainnet" ]]; then
    echo "" >&2
    echo "  ╔══════════════════════════════════════════════════════════╗" >&2
    echo "  ║  WARNING: you selected --network mainnet                 ║" >&2
    echo "  ║  Incorrect deposit data permanently locks 32 ETH/key.   ║" >&2
    echo "  ║  Verify your keystore path and pubkeys before proceeding.║" >&2
    echo "  ╚══════════════════════════════════════════════════════════╝" >&2
    echo "" >&2
    printf "  Type YES to continue: " >&2
    read -r confirm
    if [[ "${confirm}" != "YES" ]]; then
      echo "Aborted." >&2
      exit 1
    fi
    echo "" >&2
    break
  fi
done

# ── Run ───────────────────────────────────────────────────────────────────────

exec "${BIN}" "$@"
