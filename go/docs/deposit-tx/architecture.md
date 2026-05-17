# Software Architecture: eth-deposit-tx

## Overview

The `eth-deposit-tx` CLI is a focused, production-grade tool in the eth-utils monorepo for constructing and signing Ethereum deposit transactions targeting the Beacon Chain deposit contract. It implements a strict two-phase workflow (`build` then `sign`) to enable secure air-gapped operations, with Ledger hardware wallets as the primary signing method and a private-key-from-environment-variable fallback used only when necessary.

The architecture deliberately follows the exact, proven patterns established by the sibling `eth-deposit-gen` tool:
- `NewApp` + typed `Config` + injectable run callback
- Centralized error-to-exit-code mapping
- Dependency injection for testability

It maximally reuses and lightly extends existing packages (`internal/cli`, `internal/network`, `internal/output`, `internal/deposit`) while introducing only two small, single-responsibility new packages (`internal/tx` and `internal/signer`).

All design decisions are locked to the approved PRD (P0/P1/P2 requirements, CLI UX, security rules, exit codes, documentation bar) and the research findings (go-ethereum v1.14.x thin surface, `usbwallet` for Ledger, CGO isolated to HID only, `DynamicFeeTx` + `abi.Pack` + `rlp`, network mapping in `internal/network`, RLP-based interchange format, high-risk areas).

The result is a modular, decoupled, highly testable CLI that can be maintained and evolved independently while remaining consistent with the rest of the monorepo.

## Architecture Principles

- **Small modules, single responsibility** — Each package has one clear purpose describable in a single sentence.
- **Loose coupling, high cohesion** — Modules interact only through well-defined interfaces and explicit dependency injection. No direct imports of another module’s internal implementation details.
- **Interface-first** — Key contracts (`Signer`, builder functions, config types) are defined before any implementation.
- **Data ownership** — `internal/tx` is the sole owner of unsigned/signed transaction artifacts and the interchange format between `build` and `sign`.
- **Microservice-aware (but not mandatory)** — Package boundaries are clean enough that `internal/signer` or `internal/tx` could be extracted to a standalone library with minimal effort.
- **Reuse over reinvention** — Extend `internal/network` and reuse the `internal/cli` skeleton exactly; do not duplicate.
- **Security by design** — Private key material is never accepted via flags or arguments, is read only from environment variables, is zeroized as soon as possible, and Ledger is the strongly recommended path.
- **Testability as a first-class concern** — Every module can be unit-tested in complete isolation using mocks and build tags.

## System Context Diagram

```text
┌────────────────────┐
│      Operator      │  (human; may be on an air-gapped machine for the sign phase)
│  (Ledger attached) │
└──────────┬─────────┘
           │ invokes CLI
           ▼
┌──────────────────────────────────────────────────────────────┐
│  eth-deposit-tx (cmd/eth-deposit-tx)                         │
│  • build  (produces unsigned RLP artifact)                   │
│  • sign   (consumes artifact, produces signed RLP hex)       │
└──────────┬───────────────────────────────┬───────────────────┘
           │ optional (gas/nonce)          │ HID (Ledger)      │ env var only
           ▼                               ▼                   ▼
     ┌──────────┐                    ┌──────────────┐    ┌─────────────────┐
     │ Any RPC  │                    │ Ledger       │    │ Environment     │
     │ Provider │                    │ (Ethereum    │    │ Variable        │
     │          │                    │ app open)    │    │ (never a flag)  │
     └────┬─────┘                    └──────┬───────┘    └────────┬────────┘
          │                                 │ signs on device     │ read once
          │                                 ▼                     ▼
          │                            Device screen         zeroized after use
          │                                 (blind signing risk)
          ▼
     Ethereum Network
     (deposit contract address per network via internal/network)
```

External systems: Ledger device (via usbwallet + HID), optional JSON-RPC provider, the deposit contract (address resolved from `internal/network`).

## Module Overview

| Module / Package          | Responsibility                                                                 | Owns Data                                      | Depends On                              | Communication                  |
|---------------------------|--------------------------------------------------------------------------------|------------------------------------------------|-----------------------------------------|--------------------------------|
| cmd/eth-deposit-tx        | Thin CLI entrypoint, subcommand registration (`build`, `sign`), typed Config loading, dependency injection, orchestration, and error-to-exit-code mapping | —                                              | internal/cli, internal/tx, internal/signer, internal/output | Synchronous calls             |
| internal/cli              | Reusable `NewApp` factory, typed Config structs, flag parsing, help, versioning via ldflags (reused and lightly extended) | App runner + Config types                      | —                                       | —                              |
| internal/network          | Network metadata (chain ID, RPC URLs, deposit contract addresses). Extended only — never duplicated | NetworkConfig, deposit contract addresses      | —                                       | —                              |
| internal/tx               | Deposit transaction construction, ABI packing of calldata, DynamicFeeTx assembly, optional RPC gas/nonce estimation, RLP (un)signed encoding/decoding, interchange artifact handling | UnsignedTx, SignedTx, DepositCallData, RLP interchange format | internal/network, (optional) rpc.Client | Synchronous (builder API)     |
| internal/signer           | Pluggable signing abstraction: `Signer` interface + Ledger implementation (usbwallet + CGO/HID) + PrivateKey implementation (env-var only + zeroization) | —                                              | go-ethereum/usbwallet (Ledger path only) | Synchronous (`Sign` method)   |
| internal/output           | Formatted output (hex RLP, optional JSON envelope, error rendering). Reused/adapted | Output helpers                                 | —                                       | —                              |
| internal/deposit          | Deposit data structures (`DepositDataEntry` etc.). Reused for `--index` handling | DepositDataEntry                               | —                                       | —                              |

## Module Dependency Graph

```text
cmd/eth-deposit-tx
   │
   ├─► internal/cli          (exact NewApp + typed Config + run callback pattern)
   │
   ├─► internal/tx
   │      │
   │      ├─► internal/network   (GetConfig, DepositContractAddress)
   │      └─► (optional injected) rpc.Client for gas estimation / nonce
   │
   └─► internal/signer
          │
          ├─► LedgerSigner     (CGO/HID via usbwallet — build-tag isolated)
          └─► PrivateKeySigner (env var only, explicit zeroization)

internal/output   (used by cmd for all printing)
internal/deposit  (used by internal/tx when --index is supplied)

No cycles exist. All external dependencies are injected via interfaces or constructors.
```

## Module Details

### Module: cmd/eth-deposit-tx

**Responsibility:** Thin wiring and orchestration layer that follows the exact `eth-deposit-gen` CLI skeleton (`NewApp` + typed `Config` + injectable run callback) to register the `build` and `sign` subcommands, load configuration, inject the correct `Signer` and `TxBuilder`, execute the workflow, and map every error to a consistent exit code.

**Domain Entities:**
- Per-subcommand configuration structs (flags for network, index, RPC URL, gas parameters, input artifact, output mode, etc.)
- Action functions (`buildAction`, `signAction`)

**Data Store:** None (pure in-memory CLI)

**Public API (interface to other modules):**
- `main()` creates the app via the reused `internal/cli` package and dispatches to action callbacks that receive a fully validated, dependency-injected configuration.

**Events Published/Consumed:** None

**Internal Structure (proposed concrete layout):**
```
cmd/eth-deposit-tx/
├── main.go     # entrypoint, app creation, version ldflags
├── build.go    # buildAction(cfg) error — thin delegation to internal/tx
├── sign.go     # signAction(cfg) error — thin delegation to internal/signer + internal/tx
└── (optional root.go for shared flag definitions)
```

**Key Design Decisions:**
- Fidelity to the existing `eth-deposit-gen` pattern is non-negotiable for consistency, reduced review surface, and reuse of already-tested CLI infrastructure.
- All real business logic lives in `internal/*` packages; the cmd layer only wires and maps errors.

**Failure Modes:**
- Usage/flag errors → exit code 2 + helpful usage text
- All other failures (Ledger not found, invalid artifact, RPC error, signing rejection) → exit code 1 with excellent, actionable messages
- Panics are recovered and mapped to non-zero exit

### Module: internal/tx

**Responsibility:** Owns the complete lifecycle of deposit transaction construction and (un)signed RLP serialization, including ABI packing of the deposit contract call data, `DynamicFeeTx` assembly, optional gas/nonce estimation via an injected RPC client, and the portable interchange format used between the `build` and `sign` phases.

**Domain Entities:**
- `BuildParams` (network, index or deposit data, gas overrides, etc.)
- `UnsignedTx` / `SignedTx` wrappers (or direct use of `types.Transaction`)
- `DepositCallData` (pubkey, withdrawalCredentials, amount, signature, depositDataRoot)
- Interchange artifact (see below)

**Data Store:** None

**Public API (key interfaces defined early):**
```go
type TxBuilder struct {
    // holds network config + optional rpc client
}

func NewTxBuilder(net *network.Config, rpc *rpc.Client) *TxBuilder

func (b *TxBuilder) BuildUnsigned(ctx context.Context, p BuildParams) ([]byte /* unsigned RLP hex or envelope */, error)

func (b *TxBuilder) DecodeUnsigned(input []byte) (*types.Transaction, error) // used by sign command
```

**Events Published/Consumed:** None

**Internal Structure:**
```
internal/tx/
├── builder.go      # NewTxBuilder, BuildUnsigned, gas/nonce logic
├── types.go        # BuildParams, envelope structs, helpers
├── abi.go          # deposit contract ABI + PackDepositCallData
├── rlp.go          # thin RLP encode/decode + hex utilities
└── network_ext.go  # minimal additions to internal/network (deposit contract addresses)
```

**Key Design Decisions:**
- Research-locked recommendation: `DynamicFeeTx` + `abi.Pack` + `rlp` using only a thin surface of go-ethereum v1.14.x.
- RPC client is strictly optional and injected — enables fully offline/air-gapped `build` when manual gas flags are supplied.
- `--index` handling reuses `internal/deposit` structures to select the correct `DepositDataEntry` and pack calldata.

**Data Ownership & Interchange Format (between build and sign):**
- Primary artifact: hex-encoded RLP of the unsigned `DynamicFeeTx`.
- Optional versioned JSON envelope (produced when `--json` is used or when input looks like JSON):
  ```json
  {
    "version": 1,
    "network": "holesky",
    "chainId": "17000",
    "depositIndex": 0,
    "unsignedRLP": "0xf8a4...",
    "humanReadable": {
      "to": "0x00000000219ab540356cBB839Cbe05303d7705Fa",
      "value": "32 ETH",
      "dataDecoded": { ... }
    }
  }
  ```
- The `sign` subcommand accepts either plain hex RLP or the envelope (intelligently detects and parses). This format is owned exclusively by `internal/tx`.

**Failure Modes:**
- RPC unavailable when estimation is required and no manual overrides supplied → clear error telling the user exactly which flags to add.
- ABI packing or validation failures → precise, field-level error messages.
- Malformed RLP on input to `sign` → “invalid unsigned artifact” with guidance.

### Module: internal/signer

**Responsibility:** Provides a single `Signer` interface and two implementations so that the rest of the system can request a signature without knowing whether a Ledger device or an environment variable private key is being used.

**Domain Entities:** Transient only (signing material exists only for the duration of a `Sign` call).

**Data Store:** None

**Public API (key interface defined early):**
```go
type Signer interface {
    Sign(ctx context.Context, unsignedRLP []byte, chainID *big.Int) (signedRLP []byte, err error)
    Close() error
    // Name() string, RequiresUserInteraction() bool for UX helpers
}

func NewLedgerSigner() (Signer, error)                    // opens HID device
func NewPrivateKeySignerFromEnv(envVarName string) (Signer, error) // reads env only
```

**Events Published/Consumed:** None

**Internal Structure:**
```
internal/signer/
├── signer.go       # interface definition + factory selection logic + common errors
├── ledger.go       # Ledger implementation (usbwallet) — gated behind CGO/HID build tag
├── privatekey.go   # Env-var implementation with explicit zeroization
└── errors.go
```

**Key Design Decisions:**
- The interface is the contract; both implementations satisfy it, enabling perfect mocking in tests.
- Ledger is the primary/recommended path; the private-key path exists only as a fallback and is deliberately harder to use (env var only, never a CLI flag).
- CGO is isolated to the Ledger implementation via build tags so the rest of the codebase (and the `build` subcommand) can compile without CGO.
- Before calling `Sign` on a Ledger, the CLI prints human-readable decoded fields so the operator can verify them on the device screen (direct mitigation of blind-signing risk identified in research).

**Failure Modes:**
- Ledger not found, wrong app open, or user rejects on device → descriptive error, exit 1, zero key exposure.
- Required environment variable missing or invalid → clear guidance message; never leaks any key material.
- Chain-ID mismatch or other signing failures → wrapped, actionable error.

### Reuse Modules (`internal/cli`, `internal/network`, `internal/output`, `internal/deposit`)

These packages are extended only where strictly necessary (e.g., adding deposit contract addresses to `internal/network`) and are otherwise used as-is. They retain their existing data ownership and are not modified in ways that would break `eth-deposit-gen`.

## Cross-Cutting Concerns

### Error Handling & Exit Code Mapping
- Exact reuse of the error-to-exit-code pattern from `eth-deposit-gen/internal/cli`.
- Errors either implement `interface { ExitCode() int }` or are wrapped with exit metadata.
- Standard mapping:
  - 0 = success
  - 1 = operational error (Ledger, RPC, invalid input, signing failure, etc.)
  - 2 = usage / flag validation error
- Every error message follows the monorepo bar: states the problem, probable cause, and exact next action the user should take. No raw stack traces in normal output.

### Logging / Observability
- Minimal and deliberate (CLI tool). Progress and errors go to stderr via `fmt`.
- Examples: “Waiting for confirmation on Ledger device…”, “Using RPC for gas estimation…”.
- No structured logging or external telemetry. A `--verbose` flag (if added later) only increases stderr detail.

### Configuration (CLI flags + env)
- Typed `Config` structs populated by the reused `internal/cli` framework.
- All non-secret values via flags.
- Private key material **only** via a documented environment variable (never via flag or argument).
- Network selection via `--network` (resolved through `internal/network`) or explicit overrides.
- RPC URL is optional for `build`; when omitted, manual gas/nonce/fee flags are required or documented defaults with warnings are used.
- Output control: `--output -` for stdout, `--json` for envelope format.

### Security
- Ledger is the strongly recommended and default path; the private-key path is a deliberate second-class citizen.
- Private key bytes are read from the environment variable, used for the shortest possible time, and explicitly zeroized (range loop or equivalent) on all exit paths.
- No secrets ever appear in `os.Args`, process listings, logs, or output.
- Air-gapped workflow is first-class: the unsigned RLP artifact contains no secrets and can be safely transferred.
- Blind signing mitigation (research high-risk item): the CLI always prints the exact pubkey, withdrawal credentials, amount, and other decoded fields before prompting the user to confirm on the Ledger.

### Testability
- `Signer` and the RPC client are interfaces → unit tests for `build` and `sign` actions inject mocks.
- CLI actions are exercised via the injectable run callback from `internal/cli` (no real `os.Args` or subprocesses needed).
- Ledger code is behind build tags (`//go:build cgo && ledger` or equivalent) → all unit tests run on every platform using the private-key signer or mocks.
- Table-driven tests cover RLP round-trips, ABI packing, envelope parsing, gas estimation fallbacks, and zeroization.
- Integration tests (tagged) can exercise real Ledger hardware when available.

## Data Flow Diagrams

**Build phase (produces portable unsigned artifact):**
```
Operator ──eth-deposit-tx build --network holesky --index 0 [--rpc-url ...] [--json]──▶ cmd/eth-deposit-tx
cmd ──NewTxBuilder(network, optional rpc)──▶ internal/tx
tx (optional) ──RPC calls for nonce/gas──▶ RPC
tx ──load DepositDataEntry via --index──▶ internal/deposit
tx ──abi.Pack(Deposit, pubkey, wc, amount, sig, root)──▶ calldata
tx ──assemble DynamicFeeTx (to=depositContract, value=32 ETH, data, gas, fees)──▶ 
tx ──rlp.EncodeToBytes → hex (or JSON envelope)──▶ 
cmd ──internal/output.Write──▶ stdout or file
```

**Sign phase (consumes artifact, produces final signed RLP):**
```
Operator (air-gapped or Ledger machine) ──eth-deposit-tx sign --input unsigned.hex|json [--ledger]──▶ cmd
cmd ──parse input (hex or envelope) → DecodeUnsigned via internal/tx──▶ unsigned tx
cmd ──NewLedgerSigner() or NewPrivateKeySignerFromEnv()──▶ internal/signer
signer.Sign(ctx, unsignedRLP, chainID)──▶ signedRLP (Ledger device confirmation or privkey)
cmd ──output hex RLP of signed tx (ready for broadcast)──▶ stdout
```

The RLP-based interchange artifact is the only contract between the two phases and carries zero secrets.

## Infrastructure & Deployment

**Deployment Model:** Monorepo under `go/`. Single Go binary.

- Versioning via ldflags (identical to existing tools).
- Ledger support requires `CGO_ENABLED=1` + appropriate build tags; non-Ledger builds are CGO-free.
- Makefile targets will encapsulate the different build modes.
- Binaries are released via the normal eth-utils release process.

**Scaling Strategy:** N/A (stateless, on-demand CLI).

**Service Extraction Path:**
- `internal/signer` — **Ready now** (clean interface, CGO isolated, minimal dependencies).
- `internal/tx` — **Ready now** (pure logic, optional RPC injection).
- `cmd/eth-deposit-tx` — Remains the CLI driver.
- Shared packages (`internal/network`, `internal/cli`) stay together.

## Technology Choices

| Concern                  | Choice                                              | Rationale |
|--------------------------|-----------------------------------------------------|-----------|
| Language                 | Go 1.21+                                            | Monorepo standard, static binaries, strong crypto & CLI ecosystem |
| CLI Framework            | urfave/cli/v2 (exact match to eth-deposit-gen)      | Typed Config, subcommands, testability via NewApp pattern |
| Ethereum libraries       | go-ethereum v1.14.x (thin surface: core/types, accounts/abi, rlp, common, crypto, usbwallet) | Research-locked recommendation; DynamicFeeTx + official Ledger support |
| Ledger integration       | usbwallet + HID (CGO only on ledger path)           | Official, maintained path; key never leaves device |
| Transaction type         | DynamicFeeTx (EIP-1559)                             | Modern standard per research |
| Network & contract data  | Extend internal/network in place                    | Single source of truth across all eth-utils tools |
| Interchange format       | Hex RLP (primary) + versioned JSON envelope (optional) | Portable, safe for air-gap, human + machine readable |
| Output                   | Reuse/adapt internal/output                         | Consistency with sibling tool |
| Dependency injection     | Constructor functions + interfaces (Signer, optional RPC) | Enables mocking and swappability |
| Build constraints        | Build tags on signer/ledger.go                      | Allows clean no-CGO builds for air-gapped or simple environments |

## ADRs

### ADR-001: Two-phase `build` / `sign` workflow with RLP interchange
- **Status:** Accepted
- **Context:** PRD requirement for air-gapped Ledger usage.
- **Decision:** Separate subcommands; `build` emits a portable, secret-free RLP artifact (hex or envelope); `sign` consumes it.
- **Alternatives:** Single command (rejected — breaks air-gap and forces key exposure on an online machine).
- **Consequences:** Slightly more user steps on the happy path, but first-class secure workflow; excellent documentation required.

### ADR-002: Ledger primary + private key strictly from environment variable (no CLI flag)
- **Status:** Accepted (PRD security rules + research)
- **Context:** Prevent accidental or malicious leakage of private keys via shell history, `ps`, logs, etc.
- **Decision:** Ledger is default and recommended; private key path exists only via a documented env var and is zeroized immediately.
- **Alternatives:** `--private-key` flag with warnings (rejected for security posture).
- **Consequences:** Users must `export` the variable for the fallback path; error messages guide them; attack surface is dramatically reduced.

### ADR-003: Exact reuse of eth-deposit-gen CLI wiring and internal packages
- **Status:** Accepted
- **Context:** Desire for consistency, reduced maintenance, and proven patterns.
- **Decision:** Use `internal/cli.NewApp` + typed Config exactly; extend `internal/network` in place; adapt output/deposit.
- **Alternatives:** Fork or reimplement (rejected — high maintenance cost and risk of drift).
- **Consequences:** Improvements to shared packages benefit both tools; architecture review can focus on the genuinely new `tx` and `signer` modules.

## Open Questions
(Non-blocking after PRD + research approval; for the implementation team)

- Exact environment variable name for the private-key fallback (recommend `ETH_DEPOSIT_TX_PRIVATE_KEY` or similar — confirm convention).
- Precise semantics of `--index` when loading deposit data (JSON array from file/stdin vs individual fields).
- Exact set of manual gas/fee/nonce override flags required when `--rpc-url` is omitted.
- Makefile target naming and build-tag convention for Ledger vs non-Ledger binaries.
- Whether future P2 features (e.g., `--broadcast`) should be considered in the initial package boundaries.

## Risks & Mitigations

- **Ledger CGO / HID platform support** (research high-risk) — Only the Ledger path requires CGO. **Mitigation:** build tags isolate the code; clear Makefile + README instructions; CI validates the no-CGO path; pre-built guidance for common platforms.
- **Blind signing UX on Ledger** (research high-risk) — User may not verify the exact deposit data on the device. **Mitigation:** CLI always prints full decoded fields (pubkey, withdrawal credentials, amount, etc.) with an explicit “Verify these exact values on your Ledger” message before calling `Sign`.
- **Gas estimation / RPC unreliability** — Estimation can fail or be inaccurate. **Mitigation:** RPC is fully optional; comprehensive manual override flags exist; error messages tell the user precisely how to obtain values from a block explorer.
- **Nonce races in repeated deposits** — RPC nonce can race. **Mitigation:** Explicit `--nonce` override flag + documentation warning.
- **Incomplete zeroization in Go** — GC may retain copies. **Mitigation:** explicit clearing of all key material, short lifetimes, strong preference for Ledger path, documented limitation.
- **Interchange format version skew** — Future `build` may produce a newer envelope. **Mitigation:** version field + backward-compatible parser in `sign`; wire format documented in README.
- **go-ethereum version pinning** — Breaking changes in types/abi. **Mitigation:** explicit go.mod pin, thin imports, RLP round-trip tests.

**Internal Quality Checklist (verified before writing and during final review):**
- [x] No circular dependencies
- [x] Every module has a single, one-sentence responsibility
- [x] No shared databases or mutable cross-module state
- [x] All inter-module communication goes through defined interfaces or explicit constructors
- [x] Every module is testable in isolation with mocks
- [x] Cross-cutting concerns (errors, security, output, logging) are standardized
- [x] Failure modes defined for each module and every dependency
- [x] Service extraction paths are clear (`internal/signer` and `internal/tx` are ready now)
- [x] Data flows are fully traceable (build → RLP artifact → sign)
- [x] Module count is justified (maximal reuse of existing packages; only two new focused packages)
- [x] 100% alignment with eth-deposit-gen patterns, PRD requirements, and all locked research recommendations

This architecture is complete, ready for project planning, and directly usable by the downstream specialists.

---