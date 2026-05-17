# eth-deposit-tx

Build and sign Ethereum Beacon Chain deposit transactions from Launchpad-compatible `deposit_data` JSON.

**Phase 3 status:** both `build` and `sign` commands work end-to-end. The `sign` command supports local private-key signing (for development/CI) and Ledger hardware wallet signing (recommended for real funds).

> Note: `--rpc-url` is accepted by the CLI for forward compatibility but is not yet wired to a live RPC client. The `build` command currently operates in static-config mode only — provide gas/fee/nonce flags explicitly or rely on defaults. RPC-based gas/nonce estimation will be plumbed in Phase 4.

## Overview

`eth-deposit-tx` converts the `deposit_data` JSON produced by `eth-deposit-gen` (or the official Ethereum Launchpad) into raw Ethereum transactions ready for the Beacon Chain deposit contract. It is designed around a secure two-phase workflow:

1. **build** — construct an unsigned transaction (can run fully offline / air-gapped)
2. **sign** — sign the transaction, primarily via Ledger hardware wallet

The two phases are intentionally separate so the unsigned transaction can be produced on an online machine and then transferred to a signing device that never touches the internet.

## Install

Requires CGO (transitively via BLS library).

```bash
# from the go/ module root
go install ./cmd/eth-deposit-tx

# or build locally
go build -o eth-deposit-tx ./cmd/eth-deposit-tx
```

## Quick Start (Phase 3)

**Step 1 — build the unsigned transaction:**

```bash
eth-deposit-tx build \
  --network holesky \
  --input-file deposit_data.json \
  --output unsigned.json
```

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

Flag values take precedence over environment variables.

## Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | Unexpected / internal error |
| 2 | User / configuration error (bad input, unknown network, missing file, invalid `--signer`) |
| 3 | Signer / crypto error (bad private key, no Ledger device, Ethereum app not open, chain ID mismatch) |
| 4 | User abort (SIGINT / Ctrl-C, or transaction rejected on Ledger device) |

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

## Status and roadmap

- **Phase 1 (done):** CLI scaffold, config resolution, stub `build` command producing unsigned tx JSON.
- **Phase 2 (done):** Real ABI encoding for `deposit(bytes,bytes,bytes,bytes32)`. Output is fully ABI-accurate. Golden artifact and round-trip decode tests committed.
- **Phase 3 (done):** `sign` command — Ledger hardware wallet (primary) and `ETH_DEPOSIT_TX_PRIVATE_KEY` env-var fallback (with strong warnings). Both signers fully implemented and tested.
- **Phase 4:** Optional `broadcast` command to submit the signed transaction via JSON-RPC; also wires `--rpc-url` for live gas/nonce estimation.

## For contributors

- [Product requirements](../../docs/deposit-tx/prd.md)
- [Architecture](../../docs/deposit-tx/architecture.md)
- [Project plan](../../docs/deposit-tx/project-plan.md)
- [Phase 1 issues](../../docs/deposit-tx/issues/phase-1-foundation.md)
- [Phase 2 issues](../../docs/deposit-tx/issues/phase-2-tx-builder.md)
- [Phase 3 issues](../../docs/deposit-tx/issues/phase-3-signer.md)
- [Phase 2 validation artifact](../../docs/deposit-tx/validation/phase-2-unsigned-tx.md)
