# Hoodi End-to-End Deposit Validation

**Status:** PENDING
<!-- Operator: change to COMPLETE once the deposit is confirmed on-chain. -->

---

## Purpose

This document records evidence that `eth-deposit-gen` output was accepted by the **Hoodi testnet deposit contract on first submission**. Passing this validation is a required gate for Phase 2 (mainnet enablement): it proves that the generated `deposit_data` JSON is byte-for-byte compatible with the on-chain contract and the Launchpad uploader, under real network conditions, before any mainnet ETH is at risk.

The operator performs a single 32 hoodi-ETH deposit using a **throwaway BLS key** (not a key intended for mainnet) and records the outcome in the [Evidence](#evidence) section below.

---

## Prerequisites

Before starting, confirm you have:

- [ ] Access to the `eth-deposit-gen` tool (binary or wrapper script in this repo)
- [ ] A BLS keystore JSON file (`EIP-2335` format) for a **throwaway** key — do **not** use a key you intend to run on mainnet
- [ ] The BLS public key (`0x`-prefixed, 96 hex chars) corresponding to that keystore
- [ ] An execution-layer address to use as the withdrawal credential (`0x01`-prefixed, your own address on Hoodi)
- [ ] Enough hoodi-ETH to cover 32 ETH + gas (available from the [Hoodi faucet](https://hoodi-faucet.pk910.de/))
- [ ] A wallet (e.g. MetaMask) connected to the Hoodi network (Chain ID: 560048)

---

## Step-by-Step Instructions

### 1. Generate the deposit data

Run the wrapper script from the repository root. Replace the placeholder values with your actual keystore path, pubkey, and output directory:

```bash
./scripts/bin/eth-deposit-gen.sh \
  --network hoodi \
  --validator-key-path ./throwaway-keystore.json \
  --pubkeys 0x<YOUR_THROWAWAY_PUBKEY> \
  --output-dir ./out
```

The script will:
- Build the `eth-deposit-gen` binary if needed
- Prompt you for the keystore passphrase (entered securely via TTY — it does not appear in shell history)
- Write `out/deposit_data-<timestamp>.json`

Verify the output file exists and is valid JSON before proceeding:

```bash
cat out/deposit_data-*.json | python3 -m json.tool
```

Check that:
- `network_name` is `"hoodi"`
- `fork_version` is `"10000910"`
- `pubkey` matches your throwaway key (without `0x` prefix, lowercase)
- `amount` is `32000000000` (32 ETH in gwei)

### 2. Submit via the Hoodi Launchpad

1. Go to the **Hoodi Staking Launchpad**: <https://holesky.launchpad.ethereum.org/en/> and select **Hoodi** network, or use the direct Hoodi Launchpad URL if available.

   > **Note:** As of 2026 the canonical Hoodi Launchpad URL may be `https://launchpad.hoodi.ethpandaops.io/`. Confirm the current URL at <https://hoodi.ethpandaops.io/>.

2. Follow the Launchpad wizard until the **Upload deposit data** step.
3. Upload the `deposit_data-<timestamp>.json` file generated in Step 1.
4. The Launchpad will validate the JSON client-side (schema check, signature format). If it rejects the file, record the error in the [Failure](#failure) section before investigating.
5. Connect your Hoodi wallet and submit the deposit transaction. Confirm in your wallet.
6. Wait for the transaction to be included in a block (typically < 60 seconds on Hoodi).

### 3. Confirm on-chain acceptance

After the transaction is mined:

1. **Check the transaction receipt** on the Hoodi block explorer (e.g. `https://hoodi.etherscan.io/tx/<TX_HASH>`).
   - Status must be **Success**.
   - The transaction logs must include a `DepositEvent` emitted by the deposit contract (`0x4242424242424242424242424242424242424242`). This confirms the contract accepted the deposit data and the `deposit_data_root` matched.

2. **Check the validator page** on beaconcha.in for Hoodi (e.g. `https://hoodi.beaconcha.in/validator/<YOUR_PUBKEY>`).
   - This page becomes available approximately **16 hours** after the deposit is included, once the beacon chain processes the deposit queue.
   - The validator status must reach at least **`pending_initialized`** (or better: `pending_queued`, `active_ongoing`).

---

## Evidence

> **Operator instructions:** Fill in every field below. Do not leave placeholders. Change the **Status** at the top of the document to `COMPLETE` once the beaconcha.in status confirms acceptance.

| Field | Value |
|---|---|
| **Validator pubkey** | `0x` |
| **Keystore file used** | _(filename only — do not include the path or passphrase)_ |
| **CLI invocation** | _(full command as run, with real flag values — omit the passphrase)_ |
| **Deposit tx hash** | `0x` |
| **Block number** | |
| **Date of submission (ISO 8601)** | `YYYY-MM-DD` |
| **Operator name / handle** | |
| **Block explorer link** | |
| **beaconcha.in validator link** | |
| **Validator status on beaconcha.in** | _(e.g. `pending_initialized`)_ |

### Output file sha256

```
# Record the sha256 of the deposit_data-*.json file used for submission:
# sha256sum out/deposit_data-<timestamp>.json
```

Paste the output here:

```
<sha256>  deposit_data-<timestamp>.json
```

---

## Failure

> If submission fails at any step — Launchpad upload rejection, transaction revert, deposit contract log missing, or `DepositEvent` with a mismatched `deposit_data_root` — do **not** mark the status `COMPLETE`. Instead, root-cause the failure and record it here before Phase 2 is considered ready.

| Field | Value |
|---|---|
| **Failure step** | _(e.g. "Launchpad upload", "tx reverted", "no DepositEvent in logs")_ |
| **Error message / evidence** | |
| **Root cause** | |
| **Fix applied** | _(code change, config change, operator error, etc.)_ |
| **Resolved** | YES / NO |
| **Re-test required** | YES / NO |

If a code fix was required, open a GitHub issue and link it here before re-running validation from Step 1.

---

## Reference

- Hoodi deposit contract address: `0x4242424242424242424242424242424242424242`
- Hoodi `GENESIS_FORK_VERSION`: `0x10000910`
- Hoodi Chain ID: `560048`
- Hoodi Launchpad: <https://launchpad.hoodi.ethpandaops.io/>
- Hoodi block explorer: <https://hoodi.etherscan.io/>
- Hoodi beaconcha.in: <https://hoodi.beaconcha.in/>
- Hoodi faucet: <https://hoodi-faucet.pk910.de/>
- Consensus spec — deposit domain: [ethereum/consensus-specs §deposit](https://github.com/ethereum/consensus-specs/blob/dev/specs/phase0/beacon-chain.md#deposits)
- eth-deposit-gen PRD: [`docs/prd.md`](../prd.md)
- Deposit signing constants: [`docs/research/deposit-spec.md`](../research/deposit-spec.md)
