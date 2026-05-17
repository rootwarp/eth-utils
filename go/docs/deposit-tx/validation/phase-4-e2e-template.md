# Phase 4 E2E Validation Report

> **Instructions:** Copy this template, fill in every section, and commit the completed
> report as `phase-4-e2e-<date>-<network>.md` alongside this file.
> Leave blank fields as `N/A` if not applicable; do not delete them.

---

## Run metadata

| Field | Value |
|-------|-------|
| Date (UTC) | `YYYY-MM-DD HH:MM` |
| Network | `holesky` / `sepolia` / other |
| Chain ID | e.g. `17000` |
| Operator | GitHub handle or name |

---

## Tool version

| Field | Value |
|-------|-------|
| Git SHA | `git rev-parse HEAD` output |
| Binary SHA256 | `shasum -a 256 <binary>` output |
| Go version | `go version` output |
| Built with CGO | yes / no |

---

## Signer

| Field | Value |
|-------|-------|
| Signer type | `local` / `ledger` |
| Ledger firmware version | e.g. `v2.1.0` (N/A if local) |
| Ledger Ethereum app version | e.g. `v1.10.4` (N/A if local) |
| Key derivation path | e.g. `m/44'/60'/0'/0/0` (N/A if local) |
| From address | `0x...` |

> **Note:** For local signer, confirm the private key was NOT a mainnet key
> and was loaded from an environment variable (never from a CLI flag).

---

## Deposit data

| Field | Value |
|-------|-------|
| Deposit data source | path or description |
| Validator pubkey (BLS) | `0xaaa...` — mark as TEST KEY, can be discarded |
| Withdrawal credentials | `0x01...` or `0x00...` |
| Amount | `32 ETH` |
| Network in deposit data | must match Network above |

---

## Transaction

| Field | Value |
|-------|-------|
| Unsigned tx file | `testdata/deposit-e2e/<timestamp>/unsigned.json` |
| Signed tx file | `testdata/deposit-e2e/<timestamp>/signed.json` |
| RLP hex file | `testdata/deposit-e2e/<timestamp>/signed.raw` |
| RawRLP prefix | must be `0x02` (EIP-1559) |
| Deposit contract (to) | `0x4242424242424242424242424242424242424242` |
| Nonce | integer |
| Gas limit | integer |
| Max fee per gas (Gwei) | e.g. `20` |
| Max priority fee (Gwei) | e.g. `1` |

---

## Broadcast result

| Field | Value |
|-------|-------|
| Tx hash | `0x...` |
| Explorer link | `https://holesky.etherscan.io/tx/0x...` |
| Broadcast timestamp (UTC) | `YYYY-MM-DD HH:MM:SS` |

---

## Receipt

| Field | Value |
|-------|-------|
| Status | `success` / `REVERTED` |
| Block number | integer |
| Block hash | `0x...` |
| Gas used | integer |
| Receipt file | `testdata/deposit-e2e/<timestamp>/receipt.json` |

---

## Procedure followed

Describe which procedure was followed and any commands run (copy from terminal output or refer to the script used):

```
# Example:
cd go
export RPC_URL=https://holesky.infura.io/v3/<redacted>
export ETH_DEPOSIT_TX_PRIVATE_KEY=<redacted — describe key source, not value>
make e2e-testnet
```

---

## Deviations from documented procedure

List any steps that differed from `scripts/e2e-testnet.sh` or the Phase 3 runbook.
If none, write **None**.

---

## Verification checklist

- [ ] Tool version confirmed (binary SHA256 matches expected)
- [ ] Chain ID in signed tx matches RPC chain ID (no cross-network confusion)
- [ ] `to` address in unsigned tx is the correct deposit contract for the network
- [ ] `value` in unsigned tx is `0x1bc16d674ec800000` (32 ETH in wei)
- [ ] RawRLP starts with `0x02` (EIP-1559 type prefix)
- [ ] Transaction confirmed on explorer (Status = 1 / success)
- [ ] Receipt file saved and committed under `testdata/deposit-e2e/`
- [ ] Private key was NOT used on any mainnet account

---

## Notes / observations

_Any other notes about the run, warnings encountered, or follow-up items._
