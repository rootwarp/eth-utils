# PRD: eth-deposit-tx

## Overview

`eth-deposit-tx` is a lightweight Golang CLI tool that converts Ethereum 2.0 validator deposit data (JSON produced by `eth-deposit-gen` or the official Launchpad) into raw, ready-to-send deposit transactions for the Beacon Chain deposit contract.

It follows a secure two-phase workflow:

- `build`: Construct an unsigned raw transaction (hex RLP), supporting both hybrid mode (optional `--rpc-url` to fetch nonce, gas, chain ID) and fully manual/offline/air-gapped mode.
- `sign`: Sign the transaction, with Ledger hardware wallet as the primary method and a local private-key fallback (environment variable only, with strong security warnings and immediate zeroization).

The tool produces standard hex-encoded RLP output usable directly with `eth_sendRawTransaction`. It is designed for both solo stakers (simple Ledger flow) and professional operators (scriptable, batch-friendly, air-gapped).

It lives alongside `eth-deposit-gen` in the `go/` module of the eth-utils monorepo and adheres to the same high standards for CLI structure (urfave/cli/v2), security, error handling, documentation, and code quality.

## Problem Statement

Users generating deposit data today have no first-class, consistent CLI tool to turn that JSON into a valid deposit transaction. They must manually:

- Derive the correct deposit contract address and chain ID from the network name
- Construct the ABI calldata for `deposit(pubkey, withdrawal_credentials, signature, deposit_data_root)` with the correct `value` (derived from the `amount` field, typically 32 ETH)
- Obtain or supply nonce, gas limit, gas price/fee, chain ID
- Sign the transaction securely (ideally with a hardware wallet)
- Produce a hex RLP string for broadcast

This process is error-prone, especially in air-gapped environments or when handling multiple validators. Existing workflows rely on ad-hoc scripts, web interfaces, or risky private-key handling on online machines. Solo stakers and operators need a tool that reuses the trusted deposit JSON, prioritizes Ledger, supports fully offline use, provides excellent guidance and error messages, and matches the quality of `eth-deposit-gen`.

## Goals & Success Metrics

**Primary goal:** Deliver a secure, reliable, and user-friendly CLI tool that allows anyone with a deposit_data JSON to safely produce a valid raw deposit transaction using their preferred signing method (Ledger first), while fully supporting air-gapped and scripted workflows.

**Key success metrics / definition of "done" for v1:**

- P0 functionality works correctly for mainnet and hoodi (and other networks via the JSON `network_name` field + internal mapping table).
- Both solo stakers (1-8 validators, interactive + Ledger) and professional operators (50+ validators, scripted/air-gapped) can use it effectively, as validated by realistic examples and user testing.
- 100% of generated transactions are valid and would succeed on-chain when broadcast (verified via testnet dry-runs or known vectors).
- Excellent, actionable error messages and help text; exit codes consistently use 1/2/3/4 as in the existing tool.
- High-quality README with at least 4 complete, copy-pasteable examples covering solo Ledger, air-gapped operator batch, piping `build | sign`, and fully manual mode.
- Security: private-key fallback only via `ETH_PRIVATE_KEY` env var, never as a flag value; key is zeroized immediately after use; prominent warnings printed.
- No new heavy or CGO-heavy dependencies (pure-Go preference; Ledger support kept minimal and simple per constraints).
- Code follows all existing conventions (typed Config, injectable dependencies for testability, security patterns, ldflags versioning, tests).
- "Done" = PRD approved → research/architecture complete → implementation + tests + docs complete → user has exercised the full flow with a real Ledger on a testnet (hoodi) and confirmed output is correct → LGTM to merge.

## Target Users

- **Solo / home stakers**: Running a small number of validators (typically 1–8). Often using the official Launchpad for the first time. Want a simple, guided CLI experience with Ledger Nano S/X, clear warnings, and minimal flags. Value copy-paste examples and helpful error messages.
- **Professional validator operators & staking pools**: Managing dozens to hundreds of validators. Frequently work in air-gapped or highly secured environments, use scripts/CI, and handle batch deposit JSON files. Need deterministic behavior, `--index` selection, full stdin/stdout support for piping, and reliable multi-entry handling with warnings.

The design prioritizes both segments equally: simple defaults for individuals + power features (index, overrides, pipes, manual flags) for operators.

## User Stories / Use Cases

1. As a solo staker, I want to run `eth-deposit-tx build --input-file deposit_data.json --rpc-url https://eth-hoodi.g.alchemy.com/v2/... | eth-deposit-tx sign --ledger -` so that I get a signed hex tx I can safely broadcast, using my Ledger without ever exposing a private key.

2. As a professional operator in an air-gapped setup, I want to generate 50 unsigned txs on an online machine (`eth-deposit-tx build --input-file big_batch.json --index 0 --output-file unsigned-0.hex` ... for each), transfer the files, then sign them one-by-one or scripted with `cat unsigned-*.hex | eth-deposit-tx sign --ledger --ledger-index 0 - > signed-*.hex` so that the signing machine never touches the internet.

3. As a solo staker using only the fallback for testing, I want the tool to refuse private-key usage unless `ETH_PRIVATE_KEY` is set, print a loud security warning, zeroize the key, and still produce a valid signed tx.

4. As an operator scripting the flow, I want `build` to warn on stderr when a JSON contains multiple entries and default to index 0, but allow `--index 7` to pick exactly the right validator, and support `-` for stdin so I can pipe from `jq` or other tools.

5. As any user, I want `--help` and subcommand help to be comprehensive, with examples, flag descriptions, and security notes, matching or exceeding the quality of `eth-deposit-gen`.

6. As a user targeting a rare network, I want `--network-override hoodi` (or custom) to force the correct chain ID and deposit contract even if the JSON `network_name` is unexpected.

## Functional Requirements

### Must Have (P0)

- **CLI structure (urfave/cli/v2)**:
  - `eth-deposit-tx` (root) with `--version`, `--help`
  - `eth-deposit-tx build [flags]`
  - `eth-deposit-tx sign [flags]`
  - Follows exact patterns from `go/cmd/eth-deposit-gen/main.go` and `go/internal/cli/cli.go`: `NewApp`, subcommands, typed `Config` struct populated from flags, validation, `run` callback returning error that maps to clear exit codes (1=usage, 2=input, 3=rpc/network, 4=signing, etc.).

- **`build` subcommand (P0 core)**:
  - Flags (all optional where sensible, required when offline):
    - `--input-file FILE | -` (required; supports stdin via `-`)
    - `--index N` (default 0; warns on stderr if array length > 1 and using default)
    - `--rpc-url URL` (optional; when present, auto-fetches nonce, gas price/fee data, chain ID if not overridden)
    - `--network-override NAME` (e.g. `mainnet`, `hoodi`; overrides JSON `network_name`)
    - Manual offline flags (required when no `--rpc-url`): `--chain-id`, `--nonce`, `--gas-limit`, `--gas-price` (or EIP-1559 equivalents `--max-fee-per-gas`, `--max-priority-fee-per-gas`)
    - `--output-file FILE | -` (default stdout)
  - Parses the deposit JSON array using (or mirroring) the existing `deposit.Entry` type / logic from `go/internal/output/output.go`.
  - Resolves target chain ID and deposit contract address from `network_name` (or override) using an internal mapping table (mainnet: chain 1, `0x00000000219ab540356cBB839Cbe05303d7705Fa`; hoodi and common testnets to be populated in research).
  - Constructs the transaction:
    - `to` = deposit contract address
    - `value` = amount from JSON × 1_000_000_000 (wei)  [standard 32 ETH deposit]
    - `data` = ABI-encoded call to `deposit(bytes,bytes,bytes,bytes32)` with `pubkey`, `withdrawal_credentials`, `signature`, `deposit_data_root` from the entry
    - Nonce, gas, gas price/fee, chain ID as provided or fetched
  - Outputs **hex-encoded RLP of the unsigned transaction** (standard form usable by the signer) to stdout or file. (JSON metadata output is P1.)
  - Supports full pipe: `cat data.json | eth-deposit-tx build - --index 2`

- **`sign` subcommand (P0 core)**:
  - Flags:
    - `--input FILE | -` (required; accepts either the hex RLP from `build` or the original deposit JSON for re-derivation)
    - `--ledger` (selects Ledger method; auto-detects first connected device)
    - `--ledger-index N` (for users with multiple Ledgers)
    - `--output FILE | -` (default stdout)
  - Primary path: Ledger hardware wallet (unlock + open Ethereum app required; clear user guidance on errors).
  - Fallback path: only when `ETH_PRIVATE_KEY` environment variable is set (never accepted as a CLI flag value, for security parity with passphrase handling in eth-deposit-gen). Prints prominent `SECURITY WARNING: Using a private key on a machine with network access is dangerous. Prefer --ledger. Key will be zeroized after use.` to stderr, uses the key to sign, then immediately zeroizes the secret in memory.
  - Supports direct piping for convenient online+Ledger flows: `...build... | eth-deposit-tx sign --ledger -`
  - Outputs **hex-encoded RLP of the fully signed transaction** (directly usable with `eth_sendRawTransaction`).

- **Network / deposit contract resolution**:
  - Internal mapping (extend or reuse `go/internal/network` or introduce `go/internal/tx/network.go`): `network_name` → `{chainID, depositContractAddress}`.
  - At minimum for v1: mainnet and hoodi (plus any other networks already supported by eth-deposit-gen).
  - `--network-override` takes precedence.

- **Security & secrets**:
  - Private key never appears in flags, logs, or help.
  - Immediate zeroization after signing use.
  - Consistent with all existing patterns in the monorepo.

- **Documentation & UX**:
  - `README.md` in `go/cmd/eth-deposit-tx/` with installation, all P0 examples (minimum 4 realistic flows), security notes, troubleshooting (Ledger not found, wrong network, etc.).
  - Subcommand `--help` text is high-quality, includes examples and security warnings.
  - Actionable error messages (e.g., "multiple entries found, using index 0 (use --index N to select another)", "Ledger device not found. Please connect your Ledger, unlock it, and open the Ethereum app, then retry.").

- **Versioning & build**: Same ldflags mechanism as eth-deposit-gen (`-ldflags "-X main.version=..."`).

- **Testing**: Unit tests for tx construction, calldata encoding, network mapping, RLP roundtrips; mockable signer/RPC for sign and build; integration-style tests where safe (testnets).

### Should Have (P1)

- Private-key fallback fully implemented with zeroization and warning (part of core P0 per user answers).
- Full hybrid RPC mode with sensible gas estimation when `--rpc-url` is provided.
- Optional JSON output for `build` (`--output-format json`) that includes the unsigned hex plus human-readable tx fields (to, value, data, etc.).
- Support for both legacy and EIP-1559 transaction types (default to EIP-1559 when base fee is available from RPC or explicit flags).
- Comprehensive error mapping and user-friendly messages covering all common failure modes.
- Makefile target / build support consistent with the rest of the repo.
- Basic validation that the resolved chain ID matches the JSON's implied network (or override).

### Nice to Have (P2)

- `--all` flag on `build` to process every entry in the JSON array and output multiple hex lines or a JSON array of results.
- Verbose / debug logging flag.
- Dry-run mode that prints the would-be tx details without producing RLP.
- Reuse or integration with existing `internal/keystore` or BLS packages if relevant for future features.
- Support for additional networks (sepolia, holesky, etc.) as they are added to eth-deposit-gen.
- Shell completion.

## Non-Functional Requirements

- **Security**: Follow all existing monorepo security practices (no secrets in flags/CLI, zeroization, warnings). Private key fallback is a deliberate convenience with strong guardrails; Ledger is always promoted as the safe path.
- **Reliability & correctness**: Every produced RLP must be a valid transaction that the Ethereum client will accept for a deposit. Must handle both single and multi-validator JSON files gracefully.
- **Performance**: Negligible latency; 100+ entries processed in <1s (excluding RPC).
- **Dependencies**: Zero new heavy dependencies. Ledger support must be kept as lightweight as possible (preference for pure-Go solutions; CGO only if unavoidable for HID and clearly documented).
- **Compatibility**: Go 1.25+, same `go.mod` baseline as the rest of the `go/` module. Runs on macOS, Linux (and Windows where Ledger HID works).
- **Accessibility / UX consistency**: Matches the tone, flag naming, error style, and help quality of `eth-deposit-gen`.
- **Offline capability**: `build` and `sign` must function completely without network when all manual flags are supplied.

## Technical Considerations

- **Package layout** (proposed; finalized in architecture stage):
  - `go/cmd/eth-deposit-tx/main.go` — thin entrypoint (mirrors eth-deposit-gen)
  - `go/internal/cli` — reuse/extend for common CLI helpers if valuable
  - `go/internal/deposit` or `go/internal/tx/deposit.go` — deposit-specific tx builder, calldata encoding (`go-ethereum/accounts/abi`), amount-to-wei conversion, RLP handling
  - `go/internal/signer` or `go/internal/tx/signer.go` — interface for signing (Ledger + private key impls)
  - Extend `go/internal/network` (or new `go/internal/tx/network.go`) with the network → chainID + deposit address map + lookup helpers
  - `go/internal/output` — possibly reuse for structured output if JSON mode is added

- **Transaction construction details**:
  - Use `github.com/ethereum/go-ethereum/core/types` and `accounts/abi` (already available or easy to import).
  - Calldata: `abi.Pack("deposit", pubkey[:], withdrawalCredentials[:], signature[:], depositDataRoot)`
  - Value: `big.NewInt(0).Mul(big.NewInt(int64(amountFromJSON)), big.NewInt(1_000_000_000))` (converting the gwei-style amount in the JSON to wei).
  - Unsigned RLP: build a `types.Transaction` (or legacy/EIP1559 variant), then use the unsigned serialization path for `build` output.
  - Signing: `types.SignTx` for private key; Ledger equivalent via the chosen Ledger library (returns signed tx).
  - Final output: `tx.MarshalBinary()` or `EncodeRLP` → hex.

- **RPC client**: Use `go-ethereum/ethclient` only when `--rpc-url` is supplied (injected for testability).

- **Ledger integration**: Research the lightest pure-Go or low-CGO library compatible with the monorepo constraints (Nano S, Nano X, Ledger Live / HID). Provide clear setup instructions in README (install app, enable blind signing if needed for deposit contract, etc.).

- **Testing strategy**: Table-driven tests for network lookup, amount conversion, calldata; mock RPC and signer interfaces; golden files or known-good RLP vectors for mainnet/hoodi deposits.

- **Error handling**: Centralized mapping from internal errors to user messages + exit codes, consistent with existing tool.

- **Version**: Injected via ldflags in Makefile / build process, same pattern as eth-deposit-gen.

## UX / Design Notes

- **Command examples** (must appear in README and help):

  ```bash
  # Solo staker - hybrid + Ledger (recommended online flow)
  eth-deposit-tx build \
    --input-file deposit_data-0x123.json \
    --rpc-url https://rpc.hoodi.ethpandaops.io \
    | eth-deposit-tx sign --ledger -

  # Air-gapped operator - fully manual, multiple validators
  eth-deposit-tx build \
    --input-file batch.json \
    --index 7 \
    --chain-id 560048 \
    --nonce 123 \
    --gas-limit 150000 \
    --gas-price 2000000000 \
    --output-file unsigned-7.hex

  # Then on signing machine:
  eth-deposit-tx sign --ledger --ledger-index 0 --input unsigned-7.hex --output signed-7.hex

  # Pipe with warning for multi-entry
  cat deposit_data.json | eth-deposit-tx build - --index 2 | eth-deposit-tx sign --ledger -
  ```

- **Error / warning examples** (stderr):
  - "WARNING: JSON contains 8 entries; using index 0 (use --index N to select another)"
  - "SECURITY WARNING: Using a private key on a machine with network access is dangerous. Prefer --ledger. Key will be zeroized after use."
  - "Error: no Ledger device detected. Connect your device, unlock it, open the Ethereum app, and try again. (exit code 4)"

- **Help text**: Each subcommand `--help` includes "EXAMPLES" section with the above flows and security notes.

- **Input flexibility**: `--input-file -` and `--output-file -` for full Unix pipe compatibility.

- **Defaults**: Sensible and safe (index 0 with warning, EIP-1559 when possible, Ledger preferred).

## Out of Scope

- Broadcasting / submitting the signed transaction (`eth_sendRawTransaction`, `cast send`, etc.). The user is responsible for the final broadcast step.
- Batching multiple deposits into a single transaction via Multicall or similar.
- Advanced gas features: dynamic priority fee estimation UI, gas bumping, or oracle beyond what `--rpc-url` basic fetch provides.
- Any graphical, web, or TUI interface.
- Automatic recovery from Ledger errors or disconnections (user retries).
- Accepting private key material anywhere except the `ETH_PRIVATE_KEY` environment variable.
- Heavy new dependencies or CGO (per explicit constraint).
- Support for every possible Ethereum testnet beyond those already in eth-deposit-gen + hoodi/mainnet for v1.

## Open Questions

- Exact chain IDs and deposit contract addresses for hoodi and any additional testnets (mainnet is known; research will confirm the canonical values used by the deposit contract on each network).
- Preferred transaction type for v1 (EIP-1559 vs legacy) and exact flag names for manual gas when offline (`--gas-price` vs separate maxFee/maxPriority).
- Choice of Ledger Go library (lightest option that satisfies "0 new CGO deps where possible").
- Whether to introduce a new `go/internal/tx/` package or keep logic closer to `deposit/` and `network/`.
- Level of structured output for `build` (minimal hex vs optional rich JSON) — P1.
- Should `sign` also accept a "tx object" JSON from `build --output-format json` in addition to raw hex RLP?

## Milestones & Phases (if applicable)

1. **PRD complete + LGTM** (current stage)
2. **Research** (Ledger options, exact network constants, tx RLP unsigned/signed details, dep choices, security review)
3. **Architecture & design** (package layout, interfaces for signer/RPC, error taxonomy, test strategy)
4. **Implementation** (skeleton CLI, build, network map, tx builder, Ledger signer, private-key signer, tests)
5. **Polish & docs** (README with all examples, help text, error messages, Makefile integration)
6. **Review & handoff** (user verification on real hardware + testnet, final LGTM)

## Risks & Mitigations

- **Incorrect deposit calldata or value leading to failed (and expensive) on-chain deposits**: High impact. Mitigation: reuse battle-tested ABI patterns, exhaustive table tests with known-good deposit data from Launchpad/mainnet, golden RLP vectors, clear "verify the tx before broadcasting" guidance in docs, encourage testnet first use.
- **Ledger integration fragility across OSes / firmware**: Medium. Mitigation: research + choose mature library early, document exact setup steps (Ethereum app version, blind signing), provide helpful error messages, test on macOS + Linux during dev.
- **Private-key fallback being misused despite warnings**: High. Mitigation: env-only (no flag), loud repeated warning, zeroization, docs that strongly recommend Ledger and never storing the env var in shell history or scripts long-term.
- **Scope creep (requests for broadcast, multicall, etc.)**: Medium. Mitigation: explicit "Out of Scope" section in PRD and README; defer to future v2/v3 issues.
- **Dependency or CGO bloat**: Low-Medium. Mitigation: hard constraint stated in PRD and enforced in architecture review; prefer existing go-ethereum imports.
- **Users on wrong network / wrong deposit contract**: Medium. Mitigation: always surface the resolved network/chainID/contract in verbose output or errors; validate against RPC when possible; `--network-override` escape hatch with warning.
- **Air-gapped UX friction (file transfer, multiple indexes)**: Low. Mitigation: excellent `--index`, stdin/stdout, and multi-file examples in README; warning for multi-entry is clear.

---

**End of PRD.**