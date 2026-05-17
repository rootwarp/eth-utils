# eth-deposit-tx

Build and sign Ethereum Beacon Chain deposit transactions from Launchpad-compatible `deposit_data` JSON.

**Phase 2 status:** the `build` command now produces a fully ABI-accurate unsigned EIP-1559 transaction with correct 420-byte `deposit(bytes,bytes,bytes,bytes32)` calldata. Signing arrives in Phase 3.

> Note: `--rpc-url` is accepted by the CLI for forward compatibility but is not yet wired to a live RPC client. The `build` command currently operates in static-config mode only — provide gas/fee/nonce flags explicitly or rely on defaults. RPC-based gas/nonce estimation will be plumbed in Phase 4.

## Overview

`eth-deposit-tx` converts the `deposit_data` JSON produced by `eth-deposit-gen` (or the official Ethereum Launchpad) into raw Ethereum transactions ready for the Beacon Chain deposit contract. It is designed around a secure two-phase workflow:

1. **build** — construct an unsigned transaction (can run fully offline / air-gapped)
2. **sign** — sign the transaction, primarily via Ledger hardware wallet *(Phase 3)*

The two phases are intentionally separate so the unsigned transaction can be produced on an online machine and then transferred to a signing device that never touches the internet.

## Install

Pure Go — no CGO required.

```bash
# from the go/ module root
go install ./cmd/eth-deposit-tx

# or build locally
go build -o eth-deposit-tx ./cmd/eth-deposit-tx
```

## Quick Start (Phase 2)

Use the included test fixture to exercise the build command:

```bash
go run ./cmd/eth-deposit-tx build \
  --network holesky \
  --input-file ./cmd/eth-deposit-tx/testdata/deposit-fixture.json
```

Expected output (ABI-accurate, 420-byte calldata):

```json
{
  "chainId": 17000,
  "to": "0x4242424242424242424242424242424242424242",
  "value": "0x1bc16d674ec800000",
  "data": "0x22895118<420-byte ABI-encoded deposit() calldata>",
  "gas": 250000,
  "maxFeePerGas": "0x4a817c800",
  "maxPriorityFeePerGas": "0x3b9aca00",
  "nonce": 0,
  "type": "0x2"
}
```

Write the unsigned transaction to a file:

```bash
eth-deposit-tx build \
  --network holesky \
  --input-file deposit_data.json \
  --output unsigned.json
```

Read deposit data from stdin:

```bash
cat deposit_data.json | eth-deposit-tx build --network holesky --input-file -
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

Flag values take precedence over environment variables.

## Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | Unexpected / internal error |
| 2 | User / configuration error (bad input, unknown network, missing file, invalid value) |
| 3 | Signer / crypto error *(reserved for Phase 3)* |
| 4 | User abort (SIGINT / Ctrl-C) |

## Status and roadmap

- **Phase 1 (done):** CLI scaffold, config resolution, stub `build` command producing unsigned tx JSON.
- **Phase 2 (done):** Real ABI encoding for `deposit(bytes,bytes,bytes,bytes32)`. Output is fully ABI-accurate. Golden artifact and round-trip decode tests committed. `sign` is not yet implemented.
- **Phase 3 (next):** `sign` command — Ledger hardware wallet (primary) and `ETH_DEPOSIT_TX_PRIVATE_KEY` env-var fallback (with strong warnings).
- **Phase 4:** Optional `broadcast` command to submit the signed transaction via JSON-RPC; also wires `--rpc-url` for live gas/nonce estimation.

## Security notes

- No signing occurs in Phase 2. The unsigned transaction JSON contains no key material.
- Phase 3 will handle private keys exclusively via the `ETH_DEPOSIT_TX_PRIVATE_KEY` environment variable — never as a CLI flag — to avoid exposure in shell history and process listings.
- Ledger hardware wallet signing is the preferred path; the env-var key is a last-resort fallback.
- Mainnet deposit transactions are **irreversible**. Verify the `to` address and `value` fields before signing.

## For contributors

- [Product requirements](../../docs/deposit-tx/prd.md)
- [Architecture](../../docs/deposit-tx/architecture.md)
- [Project plan](../../docs/deposit-tx/project-plan.md)
- [Phase 1 issues](../../docs/deposit-tx/issues/phase-1-foundation.md)
- [Phase 2 issues](../../docs/deposit-tx/issues/phase-2-tx-builder.md)
- [Phase 2 validation artifact](../../docs/deposit-tx/validation/phase-2-unsigned-tx.md)
