# eth-deposit-tx

Build, sign, and broadcast Ethereum Beacon Chain deposit transactions from Launchpad-compatible `deposit_data` JSON.

## Documentation

- [USAGE.md](docs/USAGE.md) — complete command reference (all flags, examples, workflows)
- [SECURITY.md](docs/SECURITY.md) — threat model, key handling, Ledger guidance
- [INSTALL.md](docs/INSTALL.md) — build and install instructions
- [EXAMPLES.md](docs/EXAMPLES.md) — copy-pasteable end-to-end recipes
- [Phase 3 security review](../../docs/deposit-tx/security/phase-3-signer.md)
- [Phase 4 E2E validation template](../../docs/deposit-tx/validation/phase-4-e2e-template.md)

## Overview

`eth-deposit-tx` converts the `deposit_data` JSON produced by `eth-deposit-gen` (or the official Ethereum Launchpad) into raw Ethereum transactions ready for the Beacon Chain deposit contract. It supports four subcommands:

1. **build** — construct an unsigned transaction (runs fully offline / air-gapped)
2. **sign** — sign a previously built unsigned transaction (Ledger or local key)
3. **run** — convenience command that runs build + sign in one step on the same machine
4. **send** — broadcast a signed transaction via JSON-RPC (requires explicit network-name confirmation)

The two-phase workflow (`build` then `sign`) is the canonical air-gapped path: produce the unsigned tx on an online machine, transfer it, and sign on a device that never touches the internet. Use `run` when both phases happen on the same machine and you want a single command.

## Install

Requires CGO (transitively via BLS library).

```bash
# from the go/ module root
go install ./cmd/eth-deposit-tx

# or build locally
go build -o eth-deposit-tx ./cmd/eth-deposit-tx
```

## Quick Start

### Convenience: build + sign in one step (`run`)

Use `run` when the online machine is also your signing machine (e.g., CI with a dev key):

```bash
# Development / CI (local key from env var):
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-dev-hex-private-key>
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json
# Produces: signed.json (full SignedTx JSON) + signed.raw (0x-prefixed RLP hex)
```

```bash
# Ledger hardware wallet (recommended for real funds):
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer ledger \
  --output signed.json
# Prompts for on-device confirmation. Produces signed.json + signed.raw.
```

Use `--keep-unsigned` to also write the unsigned tx (useful for auditing):

```bash
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json \
  --keep-unsigned
# Produces: unsigned.json, signed.json, signed.raw
```

### Air-gapped workflow: `build` then `sign` on separate machines

Use when the signing device must never touch the internet:

**Step 1 — build the unsigned transaction (on the online machine):**

```bash
eth-deposit-tx build \
  --network holesky \
  --input-file deposit_data.json \
  --output unsigned.json
```

Transfer `unsigned.json` to the air-gapped signing device.

**Step 2a — sign with Ledger (recommended):**

```bash
# Prerequisites: Ledger device connected via USB, Ethereum app open on device.
eth-deposit-tx sign \
  --signer ledger \
  --input unsigned.json \
  --output signed.json
```

**Step 2b — sign with local key (development / CI only):**

```bash
# WARNING: for development only. Never use real-fund keys this way.
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-dev-hex-private-key>
eth-deposit-tx sign \
  --signer local \
  --input unsigned.json \
  --output signed.json
```

**Step 3 — broadcast with `send`:**

```bash
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.example/rpc
# Prompts you to type the network name to confirm before broadcasting.
```

### Broadcast: `send`

> **WARNING: `send` broadcasts to the live network and spends real ETH. There is no undo.**

The `send` command requires you to type the network name (e.g., `holesky`) to confirm before submitting. Use `--yes` to bypass the prompt in automation.

```bash
# Interactive confirmation (default):
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<key>

# Output:
# > You are about to BROADCAST a 32.000000 ETH deposit transaction.
# >   Network:        holesky (chain ID 17000)
# >   From:           0xabcd...
# >   ...
# > Type the network name to confirm:
# holesky
# > Broadcasting...
# Tx hash: 0xdeadbeef...
# Explorer: https://holesky.etherscan.io/tx/0xdeadbeef...

# Non-interactive (CI/automation):
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --yes

# Wait for receipt and save it:
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --yes \
  --wait-for-receipt \
  --receipt-output receipt.json
```

**Chain ID safety check:** `send` fetches the chain ID from the RPC node and compares it to the signed tx's chain ID. If they differ, broadcast is refused (exit 5). This prevents accidentally broadcasting a Holesky-signed tx to mainnet.

## Flag reference

### `build` subcommand

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--input-file`, `-i` | `ETH_DEPOSIT_TX_INPUT_FILE` | *(required)* | Path to `deposit_data-*.json` file, or `-` for stdin |
| `--network`, `-n` | `ETH_DEPOSIT_TX_NETWORK` | `hoodi` | Target network: `mainnet`, `hoodi`, `sepolia`, `holesky` |
| `--output` | `ETH_DEPOSIT_TX_OUTPUT` | stdout | Output file for the unsigned transaction |
| `--index` | `ETH_DEPOSIT_TX_INDEX` | `0` | Zero-based index into the deposit JSON array (for multi-validator files) |
| `--rpc-url` | `ETH_DEPOSIT_TX_RPC_URL` | *(none)* | JSON-RPC endpoint for gas/nonce estimation; omit for fully offline mode |
| `--gas-limit` | `ETH_DEPOSIT_TX_GAS_LIMIT` | `250000` | Gas limit for the deposit transaction |
| `--max-fee-per-gas` | `ETH_DEPOSIT_TX_MAX_FEE_PER_GAS` | `20000000000` (20 Gwei) | EIP-1559 max fee per gas in wei |
| `--max-priority-fee-per-gas` | `ETH_DEPOSIT_TX_MAX_PRIORITY_FEE_PER_GAS` | `1000000000` (1 Gwei) | EIP-1559 priority fee per gas in wei |
| `--nonce` | `ETH_DEPOSIT_TX_NONCE` | `0` | Explicit sender nonce override |

### `sign` subcommand

| Flag | Default | Description |
|------|---------|-------------|
| `--signer` | *(required)* | Signing method: `local` or `ledger` |
| `--input`, `-i` | *(required)* | Path to unsigned tx JSON (from `build`), or `-` for stdin |
| `--output`, `-o` | stdout | Output file for the signed transaction |
| `--private-key-env` | `ETH_DEPOSIT_TX_PRIVATE_KEY` | Env var name holding the hex private key (local signer only) |

### `run` subcommand

Inherits all `build` flags (see above) plus:

| Flag | Default | Description |
|------|---------|-------------|
| `--signer` | *(required)* | Signing method: `local` or `ledger` |
| `--output` | `ETH_DEPOSIT_TX_OUTPUT` / stdout | Output file for the signed transaction |
| `--private-key-env` | `ETH_DEPOSIT_TX_PRIVATE_KEY` | Env var name holding the hex private key (local signer only) |
| `--keep-unsigned` | false | Also write the unsigned tx to disk before signing (requires `--output` to be a file) |
| `--raw-output` | *(auto-derived)* | Override the companion `.raw` filename; default is `<output-stem>.raw` next to `--output` |

### `send` subcommand

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--input`, `-i` | *(none)* | *(required)* | Path to signed tx JSON (from `sign` or `run`), or `-` for stdin |
| `--rpc-url` | `ETH_DEPOSIT_TX_RPC_URL` | *(required)* | JSON-RPC endpoint URL for broadcast |
| `--yes` | *(none)* | false | Skip the interactive network-name confirmation prompt |
| `--wait-for-receipt` | *(none)* | false | Poll until the transaction receipt is available |
| `--receipt-timeout` | *(none)* | `60s` | Maximum wait time for receipt when `--wait-for-receipt` is set |
| `--receipt-output` | *(none)* | *(none)* | Write the receipt JSON to this file (implies `--wait-for-receipt`) |

**Output artifacts when `--output signed.json` is given:**

| File | Permissions | Content |
|------|-------------|---------|
| `signed.json` | `0o600` | Full `SignedTx` JSON (unsigned tx + from, hash, r, s, v, rawRLP) |
| `signed.raw` | `0o600` | `0x`-prefixed RLP hex — pass directly to `eth_sendRawTransaction` |
| `unsigned.json` | `0o644` | Unsigned tx JSON (only when `--keep-unsigned`) |

When `--output` is omitted or `-`, only `SignedTx` JSON is written to stdout; no companion `.raw` file is produced.

Flag values take precedence over environment variables.

## Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | Unexpected / internal error |
| 2 | User / configuration error (bad input, unknown network, missing file, invalid `--signer`) |
| 3 | Signer / crypto error (bad private key, no Ledger device, Ethereum app not open, chain ID mismatch) |
| 4 | User abort (SIGINT / Ctrl-C, Ledger device rejection, or declined `send` confirmation) |
| 5 | Broadcast / RPC error (dial failure, `eth_sendRawTransaction` rejection, chain ID mismatch between signed tx and RPC node) |

## Security

### Local signer (`--signer local`)

- **For development and CI only. Never use with real-fund keys.**
- The private key is read from the environment variable named by `--private-key-env` (default: `ETH_DEPOSIT_TX_PRIVATE_KEY`). It must never be passed as a CLI argument — that would expose it in shell history and process listings.
- Key bytes are zeroized in memory when the signer is closed. They never appear in log output or error messages.
- If `ETH_DEPOSIT_TX_PRIVATE_KEY` is set in your shell, unset it after signing: `unset ETH_DEPOSIT_TX_PRIVATE_KEY`.

### Ledger signer (`--signer ledger`)

- The private key never leaves the device. This is the **recommended path** for all real-fund operations.
- Prerequisites before running:
  1. Connect your Ledger device via USB.
  2. Unlock the device.
  3. Open the **Ethereum** app on the device.
- The tool derives the sender address at `m/44'/60'/0'/0/0` (BIP-44 default Ethereum path). Verify the displayed address on your device screen.
- The user must confirm the transaction on the device. Rejection or Ctrl-C exits with code 4.

### General

- Mainnet deposit transactions are **irreversible**. Verify the `to` address and `value` fields in `unsigned.json` before signing.
- No private key material is ever logged, written to disk, or included in error messages by this tool.

## Phase 3 golden artifact

A deterministic local-signer test fixture lives in `testdata/phase3/holesky/`:

- `unsigned_tx.json` — Holesky deposit tx (copied from Phase 2 golden)
- `private_key.txt` — synthetic key `0x0101...0101` (NEVER use with real funds)
- `signed_tx_golden.json` — signed output produced by the CLI with the synthetic key

SHA256 of the golden: `67c1f5a9b2f595892b0aad06104211405c2ad67d2da16cba815c03e26f80ccf2`

`TestPhase3_HoleskyLocalSignerGolden` asserts byte-for-byte equality on every `go test` run. Regenerate with `UPDATE_PHASE3_GOLDEN=1 go test ./cmd/eth-deposit-tx/...`.

## Phase 3 hardware smoke test

Developers with a Ledger device can follow the manual procedure in [`docs/deposit-tx/validation/phase-3-ledger-runbook.md`](../../docs/deposit-tx/validation/phase-3-ledger-runbook.md).

The runbook covers: prerequisites, building with CGO, generating an unsigned tx, confirming on-device display (chainId 17000, to `0x4242...4242`, 32 ETH), verifying the signed output, and optional broadcast via `cast`.

## Security review

See [`docs/deposit-tx/security/phase-3-signer.md`](../../docs/deposit-tx/security/phase-3-signer.md) for:

- Security properties verified by code (no-key-in-flag, key zeroization, no-key-in-errors, sender recovery, sentinel exit codes, 0o600 output permissions)
- Known limitations (LocalSigner dev-only, heuristic patterns, goroutine trade-off)
- Audit grep commands to re-verify key properties

## E2E testing

### Mock E2E (CI-safe, no network access)

Runs the full `run` and `send` subcommands against a mock broadcaster. No RPC endpoint required. This is the path run in CI.

```bash
# from the go/ module root
make e2e-mock
# equivalent to:
CGO_ENABLED=1 go test -tags=e2e -count=1 ./cmd/eth-deposit-tx/...
```

Tests covered:
- `TestE2E_LocalSigner_FullPipeline_NoRPC` — `run` subcommand (build+sign) with Phase 3 fixture key; verifies `SignedTx` fields and `0x02` RLP prefix.
- `TestE2E_LocalSigner_BuildSignSendMock` — sign the Phase 3 unsigned tx, then send via injected mock broadcaster; verifies tx hash in output.
- `TestE2E_SendMock_ReceiptPolling` — full send + receipt polling path via mock; verifies receipt file contents.

### Real testnet E2E (manual, requires funded account)

Runs the full pipeline against a live testnet RPC endpoint. **This broadcasts a real transaction and spends testnet ETH.** Use a test account only.

```bash
# Set required env vars
export RPC_URL=https://holesky.infura.io/v3/<your-key>
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-test-key>

# Optional: provide a real deposit_data JSON (default uses the test fixture)
# export DEPOSIT_DATA_FILE=/path/to/deposit_data.json
# export NETWORK=holesky  # default

# Run from the go/ module root
make e2e-testnet
```

The script saves all artifacts (binary, unsigned tx, signed tx, raw RLP, receipt) to `testdata/deposit-e2e/<timestamp>/`.

After a successful run, fill in the validation report template:
[`docs/deposit-tx/validation/phase-4-e2e-template.md`](../../docs/deposit-tx/validation/phase-4-e2e-template.md)

For detailed steps, see [`scripts/e2e-testnet.sh`](../../scripts/e2e-testnet.sh).

## Status and roadmap

- **Phase 1 (done):** CLI scaffold, config resolution, stub `build` command producing unsigned tx JSON.
- **Phase 2 (done):** Real ABI encoding for `deposit(bytes,bytes,bytes,bytes32)`. Output is fully ABI-accurate. Golden artifact and round-trip decode tests committed.
- **Phase 3 (done):** `sign` command — Ledger hardware wallet (primary) and `ETH_DEPOSIT_TX_PRIVATE_KEY` env-var fallback (with strong warnings). Both signers fully implemented and tested. Security review, golden artifact, and hardware runbook committed. Pending: first successful hardware test on real device.
- **Phase 4.1 (done):** `run` command (build + sign in one step), RPC wiring for gas/nonce estimation.
- **Phase 4.2 (done):** `send` command — broadcast via JSON-RPC with double-confirmation prompt, chain ID safety check, receipt polling, and receipt file output.

## For contributors

- [Product requirements](../../docs/deposit-tx/prd.md)
- [Architecture](../../docs/deposit-tx/architecture.md)
- [Project plan](../../docs/deposit-tx/project-plan.md)
- [Phase 1 issues](../../docs/deposit-tx/issues/phase-1-foundation.md)
- [Phase 2 issues](../../docs/deposit-tx/issues/phase-2-tx-builder.md)
- [Phase 3 issues](../../docs/deposit-tx/issues/phase-3-signer.md)
- [Phase 2 validation artifact](../../docs/deposit-tx/validation/phase-2-unsigned-tx.md)
- [Phase 3 security review](../../docs/deposit-tx/security/phase-3-signer.md)
- [Phase 3 Ledger runbook](../../docs/deposit-tx/validation/phase-3-ledger-runbook.md)
