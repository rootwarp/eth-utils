# eth-deposit-tx — Complete Command Reference

`eth-deposit-tx` converts `deposit_data` JSON produced by `eth-deposit-gen` (or the official Ethereum Launchpad) into raw Ethereum transactions for the Beacon Chain deposit contract.

## Subcommands

- [`build`](#build) — construct unsigned transaction (fully offline)
- [`sign`](#sign) — sign a previously built unsigned transaction
- [`run`](#run) — convenience: build + sign in one step
- [`send`](#send) — broadcast a signed transaction via JSON-RPC

---

## build

Constructs an unsigned deposit transaction from `deposit_data` JSON. Runs entirely offline — no network access required unless you supply `--rpc-url` for gas/nonce estimation.

### Synopsis

```
eth-deposit-tx build --input-file FILE --network NET [options]
```

### Flags

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--input-file`, `-i` | `ETH_DEPOSIT_TX_INPUT_FILE` | *(required)* | Path to `deposit_data-*.json`, or `-` for stdin |
| `--network`, `-n` | `ETH_DEPOSIT_TX_NETWORK` | `hoodi` | Target network: `mainnet`, `hoodi`, `sepolia`, `holesky` |
| `--output` | `ETH_DEPOSIT_TX_OUTPUT` | stdout | Output file for the unsigned transaction |
| `--index` | `ETH_DEPOSIT_TX_INDEX` | `0` | Zero-based index into the deposit JSON array |
| `--rpc-url` | `ETH_DEPOSIT_TX_RPC_URL` | *(none)* | JSON-RPC endpoint for gas/nonce estimation; omit for fully offline mode |
| `--gas-limit` | `ETH_DEPOSIT_TX_GAS_LIMIT` | `250000` | Gas limit for the deposit transaction |
| `--max-fee-per-gas` | `ETH_DEPOSIT_TX_MAX_FEE_PER_GAS` | `20000000000` (20 Gwei) | EIP-1559 max fee per gas in wei |
| `--max-priority-fee-per-gas` | `ETH_DEPOSIT_TX_MAX_PRIORITY_FEE_PER_GAS` | `1000000000` (1 Gwei) | EIP-1559 priority fee per gas in wei |
| `--nonce` | `ETH_DEPOSIT_TX_NONCE` | `0` | Explicit sender nonce override |

Flag values take precedence over environment variables.

### Examples

Build offline with explicit gas settings:

```bash
eth-deposit-tx build \
  --network holesky \
  --input-file deposit_data.json \
  --output unsigned.json \
  --gas-limit 250000 \
  --max-fee-per-gas 20000000000 \
  --max-priority-fee-per-gas 1000000000 \
  --nonce 0
```

Build with RPC estimation (gas, nonce fetched from node):

```bash
eth-deposit-tx build \
  --network holesky \
  --input-file deposit_data.json \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --output unsigned.json
```

Read deposit data from stdin, write unsigned tx to stdout:

```bash
cat deposit_data.json | eth-deposit-tx build --network holesky --input-file -
```

Build second validator in a multi-validator file:

```bash
eth-deposit-tx build \
  --network holesky \
  --input-file deposit_data.json \
  --index 1 \
  --output unsigned-validator-1.json
```

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | Unexpected internal error |
| 2 | User/config error (bad input, unknown network, missing file) |
| 5 | RPC error (when `--rpc-url` is set and estimation fails) |

### Common errors

- `--input-file: open ...: no such file or directory` — file path is wrong; verify with `ls`.
- `unknown network "..."` — valid values are `mainnet`, `hoodi`, `sepolia`, `holesky`.
- `index 3 out of range (deposit data has 2 entries)` — `--index` exceeds the array length.

---

## sign

Signs an unsigned transaction produced by `build`. Supports Ledger hardware wallet (recommended) or local private key (development only).

### Synopsis

```
eth-deposit-tx sign --signer local|ledger --input FILE [--output FILE] [--private-key-env VAR]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--signer` | *(required)* | Signing method: `local` or `ledger` |
| `--input`, `-i` | *(required)* | Path to unsigned tx JSON (from `build`), or `-` for stdin |
| `--output`, `-o` | stdout | Output file for the signed transaction; `-` writes to stdout |
| `--private-key-env` | `ETH_DEPOSIT_TX_PRIVATE_KEY` | Env var name holding the hex private key (local signer only) |

### Examples

Sign with Ledger (recommended for real funds):

```bash
eth-deposit-tx sign \
  --signer ledger \
  --input unsigned.json \
  --output signed.json
```

Sign with local key for development:

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<hex-private-key>
eth-deposit-tx sign \
  --signer local \
  --input unsigned.json \
  --output signed.json
```

Sign and pipe output directly to `send`:

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<hex-private-key>
eth-deposit-tx sign --signer local --input unsigned.json --output - | \
  eth-deposit-tx send --input - --rpc-url https://holesky.infura.io/v3/<key> --yes
```

Use a custom env var name for the private key:

```bash
export MY_SIGNER_KEY=0x<hex-private-key>
eth-deposit-tx sign \
  --signer local \
  --input unsigned.json \
  --output signed.json \
  --private-key-env MY_SIGNER_KEY
```

### Output artifacts

When `--output signed.json` is given, the file is written with `0o600` permissions (owner read/write only). Output to stdout (default or `--output -`) produces no file.

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 2 | User/config error (bad `--signer`, missing `--input`, invalid JSON) |
| 3 | Signer/crypto error (bad private key, no Ledger device, Ethereum app not open) |
| 4 | User abort (Ctrl-C or rejection on Ledger device) |

### Common errors

- `--signer: unsupported value "foo"` — must be exactly `local` or `ledger`.
- `ETH_DEPOSIT_TX_PRIVATE_KEY: env var not set or empty` — set the env var before running.
- `ledger: no device found` — connect Ledger via USB and open the Ethereum app.
- `--private-key-env: "0x..." is not a valid POSIX env var name` — pass the variable *name*, not the key value.

---

## run

Convenience command that runs `build` and `sign` in-process without writing an intermediate unsigned transaction to disk. Use when both phases happen on the same machine.

For air-gapped workflows (build on an online machine, sign on an offline device), use `build` and `sign` separately.

### Synopsis

```
eth-deposit-tx run --input-file FILE --network NET --signer local|ledger [options]
```

### Flags

Inherits all `build` flags (see [build flags](#flags) above) plus:

| Flag | Default | Description |
|------|---------|-------------|
| `--signer` | *(required)* | Signing method: `local` or `ledger` |
| `--output` | stdout | Output file for the signed transaction; `-` writes to stdout |
| `--private-key-env` | `ETH_DEPOSIT_TX_PRIVATE_KEY` | Env var name holding the hex private key (local signer only) |
| `--keep-unsigned` | false | Also write the unsigned tx to disk before signing (requires `--output` to be a file path) |
| `--raw-output` | *(auto-derived)* | Override the companion `.raw` filename |

### Output artifacts

| File | Permissions | Content |
|------|-------------|---------|
| `signed.json` | `0o600` | Full `SignedTx` JSON (unsigned tx + from, hash, r, s, v, rawRLP) |
| `signed.raw` | `0o600` | `0x`-prefixed RLP hex — pass to `eth_sendRawTransaction` |
| `unsigned.json` | `0o644` | Unsigned tx (only when `--keep-unsigned`) |

When `--output` is omitted or `-`, only `SignedTx` JSON goes to stdout; no `.raw` companion is produced.

### Examples

Same-machine build + sign, output to file:

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<hex-private-key>
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json
# Produces: signed.json (0o600) + signed.raw (0o600)
```

Ledger hardware wallet:

```bash
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer ledger \
  --output signed.json
```

Keep unsigned tx for audit trail:

```bash
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json \
  --keep-unsigned
# Produces: unsigned.json, signed.json, signed.raw
```

Pipe directly to `send`:

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<hex-private-key>
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output - | \
eth-deposit-tx send \
  --input - \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --yes
```

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 2 | User/config error (missing file, bad `--network`, missing `--signer`) |
| 3 | Signer/crypto error (bad key, no Ledger device, Ethereum app not open) |
| 4 | User abort (Ctrl-C or rejection on Ledger device) |
| 1 | Unexpected internal error |

---

## send

Broadcasts a signed transaction (produced by `sign` or `run`) to the Ethereum network via `eth_sendRawTransaction`.

**WARNING: This command spends real ETH and is irreversible.**

### Synopsis

```
eth-deposit-tx send --input FILE --rpc-url URL [--yes] [--wait-for-receipt] [--receipt-output FILE]
```

### Flags

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--input`, `-i` | *(none)* | *(required)* | Path to signed tx JSON (from `sign` or `run`), or `-` for stdin |
| `--rpc-url` | `ETH_DEPOSIT_TX_RPC_URL` | *(required)* | JSON-RPC endpoint URL for broadcast |
| `--yes` | *(none)* | false | Skip the interactive network-name confirmation prompt |
| `--wait-for-receipt` | *(none)* | false | Poll until the transaction receipt is available |
| `--receipt-timeout` | *(none)* | `60s` | Maximum wait time for receipt |
| `--receipt-output` | *(none)* | *(none)* | Write the receipt JSON to this file (implies `--wait-for-receipt`) |

### Chain ID safety check

`send` fetches the chain ID from the RPC node and compares it to the signed tx's chain ID. If they differ, broadcast is refused with exit code 5. This prevents accidentally broadcasting a Holesky-signed tx to mainnet.

### Examples

Interactive broadcast (type `holesky` when prompted):

```bash
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<key>
```

Non-interactive (CI/automation) with receipt:

```bash
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --yes \
  --wait-for-receipt \
  --receipt-output receipt.json
```

Read signed tx from stdin (piped from `run --output -`):

```bash
eth-deposit-tx send \
  --input - \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --yes
```

### Sample interactive output

```
> You are about to BROADCAST a 32.000000 ETH deposit transaction.
>   Network:        holesky (chain ID 17000)
>   From:           0xabcd...
>   To (deposit):   0x4242424242424242424242424242424242424242
>   Value:          32.000000 ETH
>   Nonce:          7
>   MaxFeePerGas:   20.000000 Gwei
>   Tx hash:        0xdeadbeef...
>
> Type the network name to confirm: holesky
> Broadcasting...
Tx hash: 0xdeadbeef...
Explorer: https://holesky.etherscan.io/tx/0xdeadbeef...
```

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 2 | User/config error (missing flags, invalid JSON) |
| 4 | User abort (Ctrl-C or declined confirmation) |
| 5 | Broadcast/RPC error (dial failure, chain ID mismatch, node rejection) |

### Common errors

- `--rpc-url: dial ...: connection refused` — RPC endpoint is unreachable.
- `chain ID mismatch: signed tx has chain ID 17000 but RPC reports 1` — you built for holesky but pointed at mainnet.
- Exit 4 with no error when you type the wrong network name — re-run and type the name exactly.

---

## Workflows

### Air-gapped workflow (recommended for mainnet)

Build on an online machine, sign on an offline device, broadcast from an online machine.

**Step 1 — Build (online machine):**

```bash
eth-deposit-tx build \
  --network mainnet \
  --input-file deposit_data.json \
  --gas-limit 250000 \
  --max-fee-per-gas 20000000000 \
  --max-priority-fee-per-gas 1000000000 \
  --nonce 0 \
  --output unsigned.json
```

Transfer `unsigned.json` to the air-gapped signing machine (USB drive, QR code, etc.).

**Step 2 — Sign (air-gapped machine, Ledger):**

```bash
eth-deposit-tx sign \
  --signer ledger \
  --input unsigned.json \
  --output signed.json
# Confirm transaction on Ledger device screen
```

Transfer `signed.json` (or `signed.raw`) back to the online machine.

**Step 3 — Broadcast (online machine):**

```bash
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://mainnet.infura.io/v3/<key>
# Type "mainnet" when prompted
```

### Convenience workflow (same machine, testnet)

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-dev-key>
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json

eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<key> \
  --yes
```

---

## Environment variables

All flags that have an `ETH_DEPOSIT_TX_*` env var counterpart accept values from the environment as a fallback. Flag values always take precedence.

| Variable | Corresponding flag | Notes |
|----------|--------------------|-------|
| `ETH_DEPOSIT_TX_INPUT_FILE` | `--input-file` | build, run |
| `ETH_DEPOSIT_TX_NETWORK` | `--network` | build, run |
| `ETH_DEPOSIT_TX_OUTPUT` | `--output` | build, run |
| `ETH_DEPOSIT_TX_INDEX` | `--index` | build, run |
| `ETH_DEPOSIT_TX_RPC_URL` | `--rpc-url` | build, run, send |
| `ETH_DEPOSIT_TX_GAS_LIMIT` | `--gas-limit` | build, run |
| `ETH_DEPOSIT_TX_MAX_FEE_PER_GAS` | `--max-fee-per-gas` | build, run |
| `ETH_DEPOSIT_TX_MAX_PRIORITY_FEE_PER_GAS` | `--max-priority-fee-per-gas` | build, run |
| `ETH_DEPOSIT_TX_NONCE` | `--nonce` | build, run |
| `ETH_DEPOSIT_TX_PRIVATE_KEY` | *(referenced by `--private-key-env`)* | sign, run; key value, not variable name |

---

## Network configuration

| Network | Chain ID | Deposit contract | Explorer |
|---------|----------|-----------------|----------|
| `mainnet` | 1 | `0x00000000219ab540356cBB839Cbe05303d7705Fa` | https://etherscan.io |
| `hoodi` | 560048 | `0x00000000219ab540356cBB839Cbe05303d7705Fa` | https://hoodi.etherscan.io |
| `sepolia` | 11155111 | `0x7f02C3E3c98b133055B8B348B2Ac625669Ed295D` | https://sepolia.etherscan.io |
| `holesky` | 17000 | `0x4242424242424242424242424242424242424242` | https://holesky.etherscan.io |

The deposit contract address and chain ID are hard-coded per network and verified on every signing operation.
