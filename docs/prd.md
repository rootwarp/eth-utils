# PRD: Ethereum Validator Deposit Data Generator (CLI)

## Overview
A Go command-line tool that generates Ethereum validator `deposit_data` JSON files for a user-supplied set of BLS public keys. Output is byte-for-byte compatible with the official [`ethereum/staking-deposit-cli`](https://github.com/ethereum/staking-deposit-cli) format consumed by the Ethereum Staking Launchpad. Deposit signatures are produced from a BLS signing key loaded from a JSON keystore file at runtime.

## Problem Statement
Validators and node operators frequently need to produce or re-produce `deposit_data.json` for *existing* validator BLS keys — e.g., to top-up deposits, re-submit lost deposit files, on-board pre-generated keys to a custodial/staking platform, or migrate keys between operators. The official `staking-deposit-cli` is oriented around generating *new* mnemonics/keys, and signing flows that target keys already held in cold storage are awkward or unsupported. Operators today resort to ad-hoc scripts that risk producing malformed deposits — a mistake that can permanently lock 32 ETH per validator.

This tool fills that gap: take known pubkeys + a BLS signing key from a keystore, and emit a Launchpad-compatible deposit JSON that operators can trust.

## Goals & Success Metrics
**Primary goal:** Allow an operator to generate Launchpad-compatible deposit data for arbitrary BLS pubkeys they control, using a BLS keystore file, on Ethereum mainnet or Hoodi.

**Success metrics:**
- 100% of generated deposit JSON files validate successfully against `staking-deposit-cli`'s `--existing_mnemonic`/verification logic and against the Launchpad upload validator.
- 100% of submitted deposits (test transactions on Hoodi, real on mainnet) are accepted by the deposit contract on first submission.
- Time from "I have N pubkeys" to "valid deposit JSON" is < 2 minutes for typical batches.
- Zero reported incidents of lost/locked ETH attributable to malformed output.

## Target Users
- **Solo validators / home stakers** with one or a small number of validators who hold their BLS signing keys in an encrypted keystore.
- **Professional node operators / staking providers** managing larger key sets, often holding BLS keys in encrypted vaults and needing to (re)generate deposit files programmatically.
- **Internal stakeholders:** SRE / key-management teams who script onboarding pipelines.

### User Needs
- Trust that the output exactly matches what the Launchpad and deposit contract expect.
- A scriptable, non-interactive flow suitable for automation pipelines.
- Batch processing for many keys.
- Clear separation between mainnet and testnet to prevent accidental mainnet submissions during testing.

## User Stories
- As a solo staker, I want to point the CLI at my BLS keystore and a pubkey, and produce a Hoodi deposit JSON so I can practice the deposit flow before going to mainnet.
- As a node operator, I want to pass a comma-separated list of pubkeys plus a keystore path and receive a single `deposit_data-<ts>.json` containing all entries.
- As a security engineer, I want the tool to refuse to run when network and withdrawal-credential prefixes are inconsistent, so I cannot accidentally generate a mainnet deposit while testing.
- As an auditor, I want every produced entry to be re-verifiable offline against the BLS pubkey and deposit message, so I can independently confirm correctness before broadcasting.

## Functional Requirements

### Must Have (P0)
1. **CLI invocation** built on `github.com/urfave/cli/v2` with exactly the following flags:
   - `--validator-key-path <path>` (required, string): path to a JSON keystore file containing the BLS signing key used to produce signatures.
   - `--pubkeys <list>` (required, string): comma-separated list of BLS pubkeys (hex, 0x-prefixed) to generate deposit data for, e.g. `0xabc...,0xdef...`.
   - `--network {mainnet,hoodi}` (required): selects the correct `fork_version`, `genesis_validators_root`, and `network_name` in the output.
   - `--output-dir <path>` (required): directory to write the resulting deposit JSON file into.
2. **Pubkey parsing & validation**: split `--pubkeys` on `,`, trim whitespace, and validate each entry is a 48-byte BLS12-381 G1 point in compressed hex form.
3. **Keystore loading**: read the JSON keystore at `--validator-key-path`, parse it, and extract the BLS signing key. Fail with a clear error if the file is missing, malformed, or the key cannot be loaded.
4. **Deposit message construction** per the consensus spec: build `DepositMessage`, compute `deposit_message_root`, build `DepositData` domain (`DOMAIN_DEPOSIT` + fork version + zero genesis validators root for deposits), and sign.
5. **Signing**: produce a BLS signature over the signing root using the loaded key; verify the resulting signature against the supplied pubkey before writing output (refuse if mismatch). The loaded private key must correspond to the supplied pubkey — reject any entry where it does not.
6. **Output file**: write `deposit_data-<unix_timestamp>.json` to `--output-dir`, containing a JSON array of entries with exactly the fields the Launchpad expects:
   `pubkey`, `withdrawal_credentials`, `amount`, `signature`, `deposit_message_root`, `deposit_data_root`, `fork_version`, `network_name`, `deposit_cli_version`.
7. **Self-verification step**: after generating each entry, re-verify the BLS signature and the `deposit_data_root` before serialization. Abort the whole run if any entry fails.

### Should Have (P1)
8. **Batch / parallel signing** to accelerate large pubkey lists (goroutine pool).
9. **Dry-run mode** (`--dry-run`) that prints what would be produced without writing files.
10. **Structured logging** with `--verbose` / `--json-logs` flags (e.g., `log/slog`).
11. **Cross-check against `staking-deposit-cli`**: optional `--verify-with-deposit-cli` post-step that shells out to the official CLI's verify routine if installed.

### Nice to Have (P2)
12. Support for additional testnets (Sepolia, Hoodi/future devnets) via a pluggable network config.
13. Integration with cloud KMS / HSM signers (AWS KMS, GCP KMS, Vault) as additional signing backends.
14. QR-code output for air-gapped transfer of the deposit JSON.

## Non-Functional Requirements

### Security
- Private key material must never be written to disk, logs, or process listings. The decoded key is held only as long as needed for signing and zeroized in memory afterward (e.g., via explicit byte slice clearing).
- Signatures are always verified against the supplied pubkey before being written to the output file.
- `fork_version` and `genesis_validators_root` for each network are hard-coded constants compiled in (not fetched from the network) to prevent supply-chain substitution.
- Dependencies pinned via `go.mod` / `go.sum` with module checksum verification (`GOFLAGS=-mod=readonly`, vendored or verified against the public checksum database). BLS implementation comes from an audited Go library.
- The private key must never be accepted as a CLI argument or any flag whose value lands in shell history; only a keystore *path* is accepted.

### Compatibility
- Output JSON must validate against the schema used by `ethereum/staking-deposit-cli` v2.x and the Launchpad uploader for both mainnet and Hoodi.
- `deposit_cli_version` field set to a value the Launchpad accepts (mirroring the latest official release the tool has been tested against).
- Built with Go ≥ 1.22; produces statically linked binaries for macOS (amd64/arm64) and Linux (amd64/arm64). Windows best-effort.

### Usability (UX)
- Single self-contained binary (`go install` or downloadable release artifact).
- Clear error messages that name the offending pubkey and the failing field.
- Progress indicator when processing > 5 entries.
- Help text (`--help` via urfave/cli) shows concrete examples.

### Reliability & Performance
- Signing throughput: ≥ 200 entries/sec on a modern laptop.
- Deterministic output for the same inputs (identical bytes, modulo the timestamp filename).

### Observability
- Exit codes: `0` success, `2` validation error, `3` signer error, `4` user abort.
- Optional summary report printed at end: N entries written, network, output path, sha256 of output file.

## Technical Considerations
- **CLI framework:** `github.com/urfave/cli/v2`.
- **BLS library:** `github.com/herumi/bls-eth-go-binary` (Eth2-flavored BLS, widely used by Prysm), or alternatively `github.com/prysmaticlabs/prysm/v5/crypto/bls` which wraps it. Both are battle-tested in production beacon clients.
- **SSZ / hashing:** `github.com/prysmaticlabs/fastssz` (or the Prysm-vendored equivalent) for `HashTreeRoot` of `DepositMessage` and `DepositData`. Must match consensus-specs exactly.
- **Keystore parsing:** EIP-2335 JSON keystore decoder — e.g., `github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4` or equivalent. The keystore decryption passphrase, if required by the file's `crypto` section, will be sourced from a secure prompt or env var as an implementation detail (the *path* is the only public CLI surface).
- **Network constants** (compile-time):
  - Mainnet `GENESIS_FORK_VERSION = 0x00000000`, deposit `genesis_validators_root = 0x00..00` (per spec, deposit domain uses zero root).
  - Hoodi `GENESIS_FORK_VERSION = 0x10000910`.
- **Domain computation:** `compute_domain(DOMAIN_DEPOSIT=0x03000000, fork_version, genesis_validators_root=ZERO)` per consensus spec.
- **Packaging:** distributed as a Go binary — installable via `go install github.com/rootwarp/eth-utils/cmd/eth-deposit-gen@latest` and as prebuilt release binaries (GitHub Releases via GoReleaser).
- **Testing:** Go test suite (`go test ./...`) including golden-file tests cross-checked against fixtures produced by the official `staking-deposit-cli`; table-driven tests for SSZ root computation; CI matrix on linux/macOS.

## UX / Design Notes
Example invocations:

```bash
# Hoodi, two pubkeys
eth-deposit-gen \
  --network hoodi \
  --validator-key-path ./bls-keystore.json \
  --pubkeys 0x93247f2209abcafd...,0xa1b2c3d4e5f6... \
  --output-dir ./out

# Mainnet, single pubkey
eth-deposit-gen \
  --network mainnet \
  --validator-key-path ./bls-keystore.json \
  --pubkeys 0x93247f2209abcafd... \
  --output-dir ./out
```

The keystore file at `--validator-key-path` is a standard EIP-2335 JSON keystore, e.g.:
```json
{
  "crypto": { "...": "..." },
  "pubkey": "93247f2209abcafd...",
  "path": "m/12381/3600/0/0/0",
  "uuid": "...",
  "version": 4
}
```

## Out of Scope
- Generating new BLS keys or mnemonics (use `staking-deposit-cli` for that).
- Submitting the deposit transaction on-chain (operator submits via Launchpad or own tooling).
- Hardware-wallet signing backends (e.g., Ledger) — not supported in v1.
- Validator client setup, keystore distribution, slashing-protection import/export.
- Voluntary exits, BLS-to-execution-credential change messages (`signed_bls_to_execution_change`) — possible follow-up tool, not in this PRD.
- Custodial workflows beyond a local JSON keystore (KMS/HSM listed as P2 only).
- Per-entry `withdrawal_credentials` / `amount` overrides via additional CLI inputs — defaults are applied uniformly across the supplied pubkeys; richer batch-input formats are deferred.
- GUI; this is CLI-only for v1.
- Networks other than mainnet and Hoodi in v1.

## Risks & Mitigations
| Risk | Impact | Mitigation |
|---|---|---|
| Malformed deposit JSON locks ETH | Critical — funds permanently lost | Mandatory self-verification of every signature + `deposit_data_root` before writing; golden-file tests against official CLI; explicit `--verify-with-deposit-cli` option. |
| Wrong network constants used | Funds sent to wrong fork / unaccepted | Hard-code constants; print selected network + first/last pubkey in a confirmation banner before signing. |
| Keystore file leaks through process listing or logs | Key compromise | Only the *path* is accepted on the CLI; never log file contents or decoded key bytes; redact in all log output. |
| Loaded private key does not match a supplied pubkey | Invalid deposit / wasted run | Per-entry pubkey-vs-privkey check before signing; abort with clear error naming the offending pubkey. |
| BLS library bug / SSZ mismatch | Incorrect signatures across all entries | Use audited libs (herumi BLS, fastssz) pinned via `go.sum`; CI cross-checks against `staking-deposit-cli` fixtures every release. |
| Supply-chain attack on dependency | Malicious signer or output | `go.sum` checksum verification; minimal dependency surface; reproducible builds via GoReleaser. |

## Open Questions
- Should the tool also emit a `deposit_data-<ts>.txt` summary alongside the JSON, mirroring some operator tooling conventions?
- How should the keystore passphrase be sourced when the EIP-2335 file is encrypted — env var, secure stdin prompt, or both? (Implementation detail, not a public CLI flag.)
- Should `--pubkeys` also accept an `@file` indirection (read newline- or comma-separated pubkeys from a file) to avoid shell command-line length limits, or is that a follow-up?
- Is there appetite for a `--verify-only` mode that takes an existing deposit JSON and re-checks every signature without re-signing?

## Milestones & Phases
- **M1 — Core (Hoodi):** urfave/cli skeleton with the four required flags (`--validator-key-path`, `--pubkeys`, `--network`, `--output-dir`); keystore loading; pubkey parsing/validation; deposit-message construction, signing, and self-verification on Hoodi; golden-file tests green.
- **M2 — Mainnet enablement:** mainnet network constants wired up; end-to-end test with a real deposit on Hoodi.
- **M3 — P1 polish:** parallel signing, dry-run mode, structured logging, optional cross-check with `staking-deposit-cli`.
- **M4 — Release v1.0:** GoReleaser-built release binaries, `go install` path documented, README with examples, audit pass on signing-critical code paths.
