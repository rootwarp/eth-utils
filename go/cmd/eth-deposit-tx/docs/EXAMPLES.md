# eth-deposit-tx — End-to-End Examples

Copy-pasteable recipes for common scenarios. All examples use Holesky testnet unless otherwise noted.

---

## Sign a testnet deposit with a local key (synthetic key)

This recipe uses the synthetic test key from the Phase 3 golden fixture. It produces a valid signed transaction for Holesky but with an address that controls no real funds.

```bash
# Set the synthetic key (NEVER use with real funds)
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x0101010101010101010101010101010101010101010101010101010101010101

# Build the unsigned transaction (offline)
eth-deposit-tx build \
  --network holesky \
  --input-file go/cmd/eth-deposit-tx/testdata/deposit_data.json \
  --gas-limit 250000 \
  --max-fee-per-gas 20000000000 \
  --max-priority-fee-per-gas 1000000000 \
  --nonce 0 \
  --output unsigned.json

# Sign with local key
eth-deposit-tx sign \
  --signer local \
  --input unsigned.json \
  --output signed.json

# Inspect the signed transaction
cat signed.json
# Expect: { "unsigned": {...}, "from": "0x...", "hash": "0x...", "r": "0x...", "s": "0x...", "v": "0x...", "rawRLP": "0x02..." }

# Unset the key immediately
unset ETH_DEPOSIT_TX_PRIVATE_KEY
```

Expected output of `eth-deposit-tx sign`:
```
(no stdout output — signed.json written)
INFO wrote signed tx path=signed.json signer=local
```

---

## Air-gapped Ledger workflow

**Recommended for mainnet.** The private key never leaves the Ledger device. The online machine never sees the key.

### Machine A: online (no key)

```bash
# Step 1: Build unsigned transaction (fully offline, no network required)
eth-deposit-tx build \
  --network mainnet \
  --input-file deposit_data.json \
  --gas-limit 250000 \
  --max-fee-per-gas 20000000000 \
  --max-priority-fee-per-gas 1000000000 \
  --nonce 0 \
  --output unsigned.json

# Verify unsigned.json looks right
cat unsigned.json
# Expect: { "chainId": 1, "to": "0x00000000219ab540...", "value": "0x1bc16d674ec80000", ... }
```

Transfer `unsigned.json` to the air-gapped machine via USB drive, QR code, or other offline channel.

### Machine B: air-gapped (Ledger connected)

```bash
# Prerequisites:
#   1. Ledger device connected via USB
#   2. Ledger unlocked with PIN
#   3. Ethereum app open on device

# Step 2: Sign with Ledger
eth-deposit-tx sign \
  --signer ledger \
  --input unsigned.json \
  --output signed.json

# The tool prints the "Waiting for confirmation on Ledger device..." message.
# On the Ledger screen, verify:
#   - Chain ID: 1 (mainnet)
#   - To: 0x00000000219ab540356cBB839Cbe05303d7705Fa
#   - Value: 32 ETH
#   - From: your funding address
# Press confirm on device.
```

Transfer `signed.json` back to Machine A.

### Machine A: broadcast

```bash
# Step 3: Broadcast (interactive confirmation)
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://mainnet.infura.io/v3/<your-key>

# When prompted, type "mainnet" to confirm
# > Type the network name to confirm: mainnet
# > Broadcasting...
# Tx hash: 0x...
# Explorer: https://etherscan.io/tx/0x...
```

---

## Generate, sign, and broadcast in one shot (run + send)

Use on a single machine for testnet workflows.

```bash
# Load your test key
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-test-key>

# Build + sign in one step, output to file
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json
# Produces: signed.json (0o600) + signed.raw (0o600)

# Broadcast with receipt polling
eth-deposit-tx send \
  --input signed.json \
  --rpc-url https://holesky.infura.io/v3/<your-key> \
  --yes \
  --wait-for-receipt \
  --receipt-output receipt.json
# Polls until tx is mined (up to 60s by default)
# Writes receipt JSON to receipt.json

# Unset key
unset ETH_DEPOSIT_TX_PRIVATE_KEY
```

Expected output:
```
INFO wrote signed tx path=signed.json signer=local
INFO wrote raw RLP path=signed.raw
> Broadcasting...
Tx hash: 0x...
Explorer: https://holesky.etherscan.io/tx/0x...
Receipt: status=success block=1234567 gasUsed=83421
```

---

## Pipe run directly into send (one-liner)

No intermediate files. Useful in CI where you want the full pipeline in a single command.

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-test-key>

eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output - | \
eth-deposit-tx send \
  --input - \
  --rpc-url https://holesky.infura.io/v3/<your-key> \
  --yes

unset ETH_DEPOSIT_TX_PRIVATE_KEY
```

Note: `--output -` skips writing `signed.raw` to disk. If you need the raw RLP for other purposes, use a file output instead.

---

## Keep unsigned tx for audit trail

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-test-key>

eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --signer local \
  --output signed.json \
  --keep-unsigned
# Produces: unsigned.json (0o644), signed.json (0o600), signed.raw (0o600)

unset ETH_DEPOSIT_TX_PRIVATE_KEY
```

The `unsigned.json` is written before signing begins. If signing fails, `unsigned.json` is preserved as a valid artifact for retry.

---

## Multi-validator file: sign each deposit separately

```bash
export ETH_DEPOSIT_TX_PRIVATE_KEY=0x<your-test-key>

# Sign validator 0
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --index 0 \
  --signer local \
  --output signed-validator-0.json

# Sign validator 1
eth-deposit-tx run \
  --network holesky \
  --input-file deposit_data.json \
  --index 1 \
  --signer local \
  --output signed-validator-1.json

unset ETH_DEPOSIT_TX_PRIVATE_KEY
```

---

## Broadcast a pre-signed raw RLP hex

If you have the raw RLP hex (the content of `signed.raw`) and want to broadcast it directly without using this tool, you can use `cast` (from Foundry) or `curl`:

```bash
# Using cast:
cast send --rpc-url https://holesky.infura.io/v3/<key> --async "$(cat signed.raw)"

# Using curl (eth_sendRawTransaction):
curl -s -X POST https://holesky.infura.io/v3/<key> \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendRawTransaction\",\"params\":[\"$(cat signed.raw)\"],\"id\":1}"
```

This is provided for reference; `eth-deposit-tx send` is the recommended path because it performs the chain ID safety check and the double-confirmation prompt.
