# Phase 3 Ledger Hardware Smoke Test Runbook

This is a manual procedure for a developer with Ledger hardware to validate
the first real Ledger-signed testnet deposit transaction.

## Prerequisites

1. **Ledger Nano S, Nano S+, or Nano X** with up-to-date firmware.
2. **Ethereum app** installed and open on the device (version ≥1.9.0 recommended).
3. **Holesky testnet ETH** in the account derived at `m/44'/60'/0'/0/0`.
   - The address is shown by the Ethereum app on-device.
   - Faucets: [holesky-faucet.pk910.de](https://holesky-faucet.pk910.de), [faucet.quicknode.com/ethereum/holesky](https://faucet.quicknode.com/ethereum/holesky).
   - You need at least 32.01 ETH (32 ETH deposit + gas).
4. **CGO build environment**: a C compiler (`gcc` or `clang`), `libusb` headers.
   - macOS: `brew install libusb`
   - Debian/Ubuntu: `apt-get install libusb-1.0-0-dev`
5. Go 1.22+.

---

## Step 1: Build with CGO enabled

```sh
cd go
CGO_ENABLED=1 go build -o eth-deposit-tx ./cmd/eth-deposit-tx
```

Verify:
```sh
./eth-deposit-tx --version
```

---

## Step 2: Generate an unsigned deposit transaction

Use the Phase 2 Holesky golden fixture or a real `deposit_data.json` from your validator client:

```sh
# Using the Phase 2 golden fixture (synthetic validator keys — do not broadcast to mainnet):
./eth-deposit-tx build \
  --network holesky \
  --input-file ../testdata/phase2/holesky/deposit_data_single.json \
  --output unsigned.json

# Or using a real deposit_data.json from your validator:
./eth-deposit-tx build \
  --network holesky \
  --input-file /path/to/deposit_data.json \
  --output unsigned.json
```

Inspect the output:
```sh
cat unsigned.json
```

Verify:
- `chainId` is `17000` (Holesky)
- `to` is `0x4242424242424242424242424242424242424242`
- `value` is `0x1bc16d674ec800000` (32 ETH in wei)

---

## Step 3: Connect Ledger and open Ethereum app

1. Connect the Ledger device via USB.
2. Unlock the device (enter your PIN).
3. Navigate to the Ethereum app and open it.
4. The device should show "Application is ready".

---

## Step 4: Sign with Ledger

```sh
./eth-deposit-tx sign \
  --signer ledger \
  --input unsigned.json \
  --output signed.json
```

The CLI will print:
```
Waiting for confirmation on Ledger device...
Please confirm the transaction on your Ledger device...
```

---

## Step 5: Verify on-device display

On the Ledger screen, you should see a transaction review sequence. Verify:

| Field | Expected Value |
|-------|---------------|
| Type | EIP-1559 (Type 2) |
| Network | Ethereum (or Holesky if firmware supports testnet labels) |
| To | `0x4242...4242` |
| Amount | `32 ETH` |
| Max Fee | matches `maxFeePerGas` from `unsigned.json` |

Press the right button to scroll through all fields. Press both buttons on the final screen to confirm.

---

## Step 6: Verify the signed output

```sh
cat signed.json
```

Check:

- `from` matches the address shown by the Ledger Ethereum app for `m/44'/60'/0'/0/0`.
- `v` is `"0"` or `"1"` (decimal y-parity for EIP-1559).
- `rawRLP` starts with `0x02` (EIP-2718 type-2 envelope).
- `r` and `s` are 0x-prefixed 32-byte hex values.
- `hash` is a 32-byte 0x-prefixed tx hash.

---

## Step 7 (Optional): Broadcast via cast

```sh
# Requires cast from foundry-rs/foundry
cast send --rpc-url https://rpc.holesky.ethpandaops.io \
  --raw $(jq -r '.rawRLP' signed.json)
```

Or via eth_sendRawTransaction directly:
```sh
curl -X POST https://rpc.holesky.ethpandaops.io \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_sendRawTransaction\",\"params\":[\"$(jq -r '.rawRLP' signed.json)\"],\"id\":1}"
```

---

## Troubleshooting

If signing fails, the error message from the Ledger transport may be a raw APDU code or a text string. Capture the exact error and update the heuristics in `internal/signer/ledger.go`:

| Symptom | Likely cause | Heuristic function to update |
|---------|-------------|------------------------------|
| Error contains an unknown APDU code | New firmware version | `isAppNotOpenErr`, `isUserRejectedErr`, or `isChainIDMismatchErr` |
| App detection fails | Ethereum app not in foreground | `isAppNotOpenErr` |
| Signing times out | Device screen timed out | Increase device auto-lock timeout |
| "Chain not supported" error | Ledger firmware doesn't know Holesky (17000) | `isChainIDMismatchErr`; may need to enable blind signing |

To enable blind signing (for unknown chain IDs): on the Ledger Ethereum app, go to Settings → Blind Signing → Enable.

---

## Recording actual error strings

When hardware tests produce errors, add the exact strings to this table so future heuristic refinements have a reference:

| Error string observed | Device action | Maps to | Heuristic to update |
|----------------------|---------------|---------|---------------------|
| *(fill in from hardware test)* | | | |

---

## Phase 3 sign-off criteria

The Phase 3 hardware validation is complete when:

- [ ] `eth-deposit-tx sign --signer ledger` runs without error on a Ledger Nano S/X.
- [ ] The on-device display shows the correct `to`, `value`, and chain ID.
- [ ] The `from` field in `signed.json` matches the Ledger's derived address.
- [ ] `rawRLP` decodes as a valid EIP-1559 transaction with correct signature.
- [ ] (Optional) The transaction is successfully broadcast and mined on Holesky.
