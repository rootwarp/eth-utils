# Research: Ethereum Deposit Spec — Constants & Formulas

This document captures the exact consensus-spec constants and formulas the deposit data generator must implement. These are the values that, if wrong by a single byte, produce a deposit that the on-chain deposit contract or the Launchpad will reject — or worse, accept with funds permanently locked. Treat this file as the canonical reference when implementing the signing pipeline.

## Network Constants

### Genesis fork version

| Network  | `GENESIS_FORK_VERSION` (4 bytes) |
|----------|-----------------------------------|
| Mainnet  | `[0x00, 0x00, 0x00, 0x00]`        |
| Hoodi    | `[0x10, 0x00, 0x09, 0x10]`        |

These are the *deposit-domain* fork versions: deposits are always signed against the *genesis* fork version of the target network, regardless of any later forks (Altair, Bellatrix, Capella, Deneb, Electra, Fulu, etc.). The consensus spec's `compute_deposit_domain` deliberately ignores the current head fork.

### Domain type

| Constant         | Value (4 bytes)              |
|------------------|------------------------------|
| `DOMAIN_DEPOSIT` | `[0x03, 0x00, 0x00, 0x00]`   |

### Genesis validators root

For deposit signing, `genesis_validators_root` is **always `bytes32(0)`** (32 zero bytes), independent of the network. This is mandated by the consensus spec's deposit-handling rules: deposits must be verifiable before any beacon chain exists, so they cannot depend on a chain-derived genesis validators root.

This is the single most common source of bugs in homemade deposit tools — using the *actual* network `genesis_validators_root` here (which is non-zero for both mainnet and Hoodi) produces a signature that fails verification.

## Domain Computation

```
fork_data_root = hash_tree_root(ForkData{
    current_version:         fork_version,            // 4 bytes, e.g. 0x00000000 for mainnet
    genesis_validators_root: ZERO_HASH,               // 32 zero bytes for deposits
})

domain = domain_type ++ fork_data_root[:28]            // 4 + 28 = 32 bytes
```

Where `domain_type = DOMAIN_DEPOSIT = 0x03000000` and `++` is byte concatenation. `fork_data_root[:28]` is the first 28 bytes of the 32-byte SSZ root — this is *not* a hash, just a truncation.

## Signing Root

```
signing_root = hash_tree_root(SigningData{
    object_root: deposit_message_root,
    domain:      domain,
})
```

The BLS signature is produced over `signing_root` using the validator's BLS signing key.

## `deposit_message_root`

```
deposit_message_root = hash_tree_root(DepositMessage{
    pubkey:                 <48 bytes, BLS12-381 G1 compressed pubkey>,
    withdrawal_credentials: <32 bytes>,
    amount:                 <uint64 gwei>,
})
```

The amount is in **gwei**, not wei and not ETH. Default deposit amount is `32 ETH = 32_000_000_000 gwei`.

## `deposit_data_root`

```
deposit_data_root = hash_tree_root(DepositData{
    pubkey:                 <48 bytes>,
    withdrawal_credentials: <32 bytes>,
    amount:                 <uint64 gwei>,
    signature:              <96 bytes, BLS12-381 G2 compressed signature>,
})
```

This is the value the on-chain deposit contract recomputes and emits in the `DepositEvent` log; it is also the value the beacon chain uses to verify the deposit. It is *not* the same root as `deposit_message_root` — the latter is signed-over, the former binds the signature into the deposit identity.

## JSON Output Schema

The output must match the format consumed by the Ethereum Staking Launchpad and produced by `staking-deposit-cli`. Each entry is a JSON object with exactly these fields, and the file is a JSON array of one or more such objects:

```json
{
  "pubkey": "<96 hex chars>",
  "withdrawal_credentials": "<64 hex chars>",
  "amount": <uint64 gwei>,
  "signature": "<192 hex chars>",
  "deposit_message_root": "<64 hex chars>",
  "deposit_data_root": "<64 hex chars>",
  "fork_version": "<8 hex chars>",
  "network_name": "mainnet" | "hoodi",
  "deposit_cli_version": "2.7.0"
}
```

Critical encoding rules:

- All byte fields are **lowercase hex without `0x` prefix**. A leading `0x` will cause the Launchpad uploader to reject the file.
- `amount` is a JSON *number* (not a string), in gwei. JSON numbers are safely represented up to 2^53 − 1; 32 ETH (3.2 × 10^10 gwei) is well within range.
- `network_name` is the lowercase short name of the target network.
- `deposit_cli_version` mirrors the most recent `staking-deposit-cli` release this tool has been tested against. Update this string when re-validating against new releases.

The file containing the JSON array should be named `deposit_data-<unix_timestamp>.json` per the PRD.

## `withdrawal_credentials` Encoding

`withdrawal_credentials` is 32 bytes, with the first byte indicating the credential type:

- **0x00 — BLS withdrawal credentials**: `0x00` + `sha256(withdrawal_pubkey)[1:]` (used by old keys before the Capella unlock; should not be produced by new deposits).
- **0x01 — Execution withdrawal credentials**: `0x01` + 11 zero bytes + 20-byte execution-layer address (32 bytes total).
- **0x02 — Compounding withdrawal credentials** (Electra/EIP-7251): `0x02` + 11 zero bytes + 20-byte execution-layer address. Same byte layout as 0x01; differs only in the prefix and in the validator's effective-balance ceiling on the beacon chain.

For this tool, default to **0x01** (execution-address) credentials. The layout is:

```
[0x01] [0x00 × 11] [20-byte execution address] = 32 bytes
```

## Default Amount

`amount = 32 ETH = 32_000_000_000 gwei`.

Per the PRD this is uniform across the batch in v1; per-entry overrides are out of scope.

## Cross-checks

When implementing, validate against these invariants:

- `len(pubkey) == 48` bytes; the pubkey is a valid compressed G1 point (BLS library will return an error on decode if not).
- `len(withdrawal_credentials) == 32` bytes; first byte is `0x01` (for v1); next 11 bytes are zero; last 20 bytes form a valid 20-byte address.
- `len(signature) == 96` bytes; the BLS library accepts it as a compressed G2 point, AND verifies against `(pubkey, signing_root)`.
- `amount` is a positive `uint64` ≥ `MIN_DEPOSIT_AMOUNT` (1 ETH = 1_000_000_000 gwei). For default deposits this is always 32 ETH.
- `fork_version` in the JSON matches `GENESIS_FORK_VERSION` of the selected `--network`.
