#!/usr/bin/env bash
# validate-readme-examples.sh — runs executable bash code blocks from
# go/cmd/eth-deposit-gen/README.md against a freshly built binary.
#
# Usage:
#   scripts/validate-readme-examples.sh
#
# The script builds the binary, extracts ```bash ... ``` blocks from the
# README, and runs each in a temporary directory. Blocks that begin with
# "export KEYSTORE_PASS=my-keystore-passphrase" are substituted with the
# real hoodi-golden-test fixture passphrase and keystore so they actually work.
# Blocks containing placeholder text like "<your-validator-pubkey>" or
# "0x<pk" are skipped (they are illustrative only).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GO_DIR="$REPO_ROOT/go"
BINARY="$GO_DIR/bin/eth-deposit-gen"
README="$GO_DIR/cmd/eth-deposit-gen/README.md"
TESTDATA="$GO_DIR/testdata/hoodi"
GOLDEN_PASS="hoodi-golden-test-passphrase"
GOLDEN_PUBKEY="0x$(cat "$TESTDATA/pubkeys.txt")"
GOLDEN_KEYSTORE_DIR="$TESTDATA/keystores"

# Build the binary.
echo "==> Building eth-deposit-gen"
(cd "$GO_DIR" && CGO_ENABLED=1 make build)

# Extract ```bash blocks from the README.
TMPDIR_BASE=$(mktemp -d)
trap 'rm -rf "$TMPDIR_BASE"' EXIT

python3 - "$README" "$TMPDIR_BASE" <<'PYEOF'
import sys, re, os, pathlib

readme = pathlib.Path(sys.argv[1]).read_text()
outdir = sys.argv[2]

blocks = re.findall(r'```bash\n(.*?)```', readme, re.DOTALL)
for i, block in enumerate(blocks):
    path = os.path.join(outdir, f"block_{i:03d}.sh")
    with open(path, 'w') as f:
        f.write(block)
PYEOF

PASS=0
SKIP=0
FAIL=0

for block_file in "$TMPDIR_BASE"/block_*.sh; do
    block=$(cat "$block_file")

    # Skip illustrative blocks containing placeholder pubkeys.
    if echo "$block" | grep -qE '0x<|<your-|<pk[0-9]|0x\.\.\.|0x93247|0xa1b2c'; then
        echo "SKIP (illustrative placeholder): $(echo "$block" | head -1)"
        SKIP=$((SKIP + 1))
        continue
    fi

    # Skip blocks that only set env vars (no command to run).
    stripped=$(echo "$block" | sed '/^#/d; /^$/d; /^export /d; /^read /d; /^unset /d')
    if [ -z "$stripped" ]; then
        echo "SKIP (no runnable commands): $(echo "$block" | head -1)"
        SKIP=$((SKIP + 1))
        continue
    fi

    # Skip blocks showing expected error output (text/console blocks are already
    # filtered, but guard against any that slipped through).
    if echo "$block" | grep -qE '^(mainnet selected|keystore load|cgo:|wrote \./out)'; then
        echo "SKIP (illustrative output): $(echo "$block" | head -1)"
        SKIP=$((SKIP + 1))
        continue
    fi

    # Substitute fixture values for runnable blocks.
    patched=$(echo "$block" \
        | sed "s|export KEYSTORE_PASS=my-keystore-passphrase|export KEYSTORE_PASS=$GOLDEN_PASS|g" \
        | sed "s|0x8420760d0de00ed65f290ab2122e65933e168539ad261b5e444a5094c649272527a1509dd105a801922c359e46e33fb9|$GOLDEN_PUBKEY|g" \
        | sed "s|\./keystores/|$GOLDEN_KEYSTORE_DIR/|g" \
        | sed "s|\./out|$TMPDIR_BASE/out|g" \
        | sed "s|eth-deposit-gen |$BINARY |g")

    mkdir -p "$TMPDIR_BASE/out"

    echo "---"
    echo "RUN: $(echo "$block" | head -2)"
    set +e
    bash -c "$patched" 2>&1
    rc=$?
    set -e

    if [ $rc -eq 0 ]; then
        echo "PASS (exit $rc)"
        PASS=$((PASS + 1))
    else
        echo "FAIL (exit $rc)"
        FAIL=$((FAIL + 1))
    fi
done

echo ""
echo "==> Results: $PASS passed, $SKIP skipped, $FAIL failed"
[ $FAIL -eq 0 ]
