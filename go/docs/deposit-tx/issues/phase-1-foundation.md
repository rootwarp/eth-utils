# Phase 1: Foundation

## Phase Overview
- **Goal:** Establish the `cmd/eth-deposit-tx` CLI skeleton (mirroring `eth-deposit-gen` structure), core domain types, configuration handling for networks and deposit contract addresses, a working “build” command that produces an unsigned transaction artifact (via a stub builder), consistent exit codes, logging, and basic help/version. This phase delivers the first vertical slice: a user can invoke the tool and obtain a structurally correct (but not yet ABI-accurate) unsigned deposit tx JSON.
- **Issue count:** 6 issues, 11 total points
- **Estimated duration:** ~7–8 days (single-stream)
- **Entry criteria:** Approved PRD, Research (tx-construction-and-ledger.md), Architecture, and vertical-slice Project Plan. Existing `go/cmd/eth-deposit-gen/` patterns available for reference.
- **Exit criteria:** 
  - `eth-deposit-tx --help` and `--version` work cleanly.
  - `eth-deposit-tx build --network holesky --deposit-data deposit_data.json --out unsigned.json` produces a JSON file with `to`, `value`, `data`, `chainId`, `gas`, `maxFeePerGas`, etc. (stub values acceptable).
  - Exit codes defined and used (per PRD).
  - Unit tests for CLI parsing, config loading, and the stub build path pass.
  - Initial README stub + command help text committed.
  - All Phase 1 issues closed; code builds on macOS and Linux (no CGO yet).

## Phase Summary

| Issue | Title | Points | Blocked by | Scope | Files (new or modified) |
|-------|-------|--------|------------|-------|-------------------------|
| 1.1 | CLI scaffold (`cmd/eth-deposit-tx`) | 2 | — | 1 day | `cmd/eth-deposit-tx/main.go`, `cmd/eth-deposit-tx/cmd/root.go`, `cmd/eth-deposit-tx/cmd/build.go` (stub) |
| 1.2 | Core domain types | 2 | — | 1 day | `internal/types/deposit.go`, `internal/types/network.go` (new package) |
| 1.3 | Configuration loader (networks, contracts, RPC endpoints) | 2 | 1.2 | 1–2 days | `internal/config/config.go`, `internal/config/defaults.go` |
| 1.4 | Stub “build” command + unsigned tx output | 3 | 1.1, 1.2, 1.3 | 2 days | `cmd/eth-deposit-tx/cmd/build.go`, `internal/tx/stub_builder.go` (temporary) |
| 1.5 | Exit codes + error handling + logging | 1 | 1.1 | 0.5–1 day | `internal/errors/exit.go`, updates to root + build commands |
| 1.6 | Phase-1 tests + help text + README stub | 1 | 1.1–1.5 | 1 day | `cmd/eth-deposit-tx/cmd/*_test.go`, `README.md` (initial) |

## Phase Execution Plan (Single-Stream)

| Day | Issue | Notes |
|-----|-------|-------|
| 1 | 1.1 CLI scaffold | Mirror eth-deposit-gen cobra layout; add root + build (stub) commands |
| 2 | 1.2 Core domain types | DepositRequest, NetworkConfig, UnsignedTx, etc. |
| 3–4 | 1.3 Config loader | Flags + env + sane defaults for mainnet/holesky/sepolia + deposit contract addresses |
| 5–6 | 1.4 Stub build | Produces unsigned JSON (to/value/data/chainId/gas/maxFeePerGas/maxPriorityFeePerGas/nonce) |
| 7 | 1.5 Exit codes + logging | Define constants per PRD; consistent error paths |
| 8 | 1.6 Polish + tests | Unit tests for parsing/config, README stub, help text review |

---

## Issues

### Issue 1.1: CLI scaffold (`cmd/eth-deposit-tx`)
- **Points:** 2
- **Type:** foundation / setup
- **Priority:** P0
- **Blocked by:** none
- **Blocks:** 1.4, 1.5
- **Scope:** 1 day

**Description:**  
Create the initial Go CLI application under `cmd/eth-deposit-tx/` using the same command structure and patterns as the existing `cmd/eth-deposit-gen/` tool (cobra or equivalent, main entrypoint, version embedding, root command with subcommands). Add a placeholder `build` subcommand that will later call the real builder.

**Implementation Notes:**
- Files likely affected / created: `cmd/eth-deposit-tx/main.go`, `cmd/eth-deposit-tx/cmd/root.go`, `cmd/eth-deposit-tx/cmd/build.go` (initial stub), `cmd/eth-deposit-tx/cmd/version.go` (or reuse root).
- Follow the exact layout and Makefile/release conventions from `eth-deposit-gen` (binary name `eth-deposit-tx`).
- Add `--version` / `-v` and `--help` that match the documentation bar in the PRD.
- New files to create: the entire `cmd/eth-deposit-tx/` tree skeleton.
- Do not introduce CGO yet (pure Go only in Phase 1).

**Acceptance Criteria:**
- [ ] `go run ./cmd/eth-deposit-tx --help` shows usage and subcommands (at minimum `build`).
- [ ] `go run ./cmd/eth-deposit-tx --version` prints a semantic version (can be dev placeholder).
- [ ] `go run ./cmd/eth-deposit-tx build --help` shows flags (even if not all wired yet).
- [ ] Binary builds cleanly with `go build ./cmd/eth-deposit-tx` on macOS and Linux.
- [ ] No CGO dependency introduced in this issue.

**Testing Notes:**
- Basic smoke test in CI that the binary starts and shows help (can be a simple `go run` check).

---

### Issue 1.2: Core domain types
- **Points:** 2
- **Type:** foundation
- **Priority:** P0
- **Blocked by:** none
- **Blocks:** 1.3, 1.4
- **Scope:** 1 day

**Description:**  
Define the core domain types that the entire tool will use: `DepositRequest` (pubkey, withdrawal_credentials, signature, deposit_data_root, and amount), `NetworkConfig` (chain ID, deposit contract address, RPC URL, explorer), `UnsignedTx` / `SignedTx` envelopes, and any BLS/SSZ helper types needed early.

**Implementation Notes:**
- New package: `internal/types/` (or `internal/deposit/` per architecture guidance — confirm with architect if `types` is preferred).
- Files: `internal/types/deposit.go`, `internal/types/network.go`, `internal/types/tx.go`.
- Keep types minimal but sufficient for the stub builder and later real builder.
- Include JSON tags for deposit-data input compatibility with eth-deposit-gen output format.
- Do not implement ABI encoding yet (that is Phase 2).

**Acceptance Criteria:**
- [ ] Types compile and have basic validation methods (e.g., `Validate()` on DepositRequest that checks lengths: pubkey 48B, withdrawal 32B, sig 96B, root 32B).
- [ ] Unit tests cover happy path and length/format errors.
- [ ] Types are importable from `cmd/eth-deposit-tx` and future `internal/tx` and `internal/signer` packages.

**Testing Notes:**
- Table-driven tests for validation.

---

### Issue 1.3: Configuration loader (networks, contracts, RPC endpoints)
- **Points:** 2
- **Type:** foundation
- **Priority:** P0
- **Blocked by:** 1.2
- **Blocks:** 1.4
- **Scope:** 1–2 days

**Description:**  
Build a configuration system that resolves network selection (`--network mainnet|holesky|sepolia`), deposit contract addresses (hard-coded known values + override), RPC endpoints, and gas defaults. Support flag > env > defaults precedence. Provide a clean struct for commands to consume.

**Implementation Notes:**
- New files: `internal/config/config.go`, `internal/config/defaults.go`, `internal/config/networks.go`.
- Hard-code the canonical deposit contract addresses:
  - Mainnet: `0x00000000219ab540356cBB839Cbe05303d7705Fa`
  - Holesky / Sepolia as per research doc.
- Allow `--rpc-url` override and `--gas-limit`, `--max-fee-per-gas`, etc.
- No secrets in config (private keys come later via signer).

**Acceptance Criteria:**
- [ ] `eth-deposit-tx build --network holesky` correctly selects the Holesky deposit contract and chain ID.
- [ ] Environment variable overrides work (e.g., `ETH_DEPOSIT_TX_RPC_URL`).
- [ ] Unknown network produces a clear error with exit code per PRD.
- [ ] Unit tests cover resolution order and validation.

**Testing Notes:**
- Mockable config for later command tests.

---

### Issue 1.4: Stub “build” command + unsigned tx output
- **Points:** 3
- **Type:** feature (vertical slice)
- **Priority:** P0
- **Blocked by:** 1.1, 1.2, 1.3
- **Blocks:** Phase 2 integration
- **Scope:** 2 days

**Description:**  
Implement the `build` subcommand that reads a deposit-data JSON file (compatible with eth-deposit-gen output), resolves config for the selected network, and writes an unsigned transaction JSON artifact. Use a temporary stub builder that produces plausible but not yet ABI-correct calldata. This gives the first end-to-end vertical slice of the tool.

**Implementation Notes:**
- Modify/create: `cmd/eth-deposit-tx/cmd/build.go` (real command), `internal/tx/stub_builder.go` (temporary file that will be replaced in Phase 2).
- Output format (JSON) should include at minimum: `to`, `from` (optional placeholder), `value`, `data` (hex), `chainId`, `gas`, `maxFeePerGas`, `maxPriorityFeePerGas`, `nonce`.
- Accept `--deposit-data FILE` (or `-d`), `--network`, `--out FILE` (or stdout).
- The stub should still call the domain types from 1.2 and config from 1.3.

**Acceptance Criteria:**
- [ ] `eth-deposit-tx build --network holesky --deposit-data sample.json --out unsigned.json` succeeds and produces a valid JSON file.
- [ ] The JSON contains all required Ethereum tx fields for later signing.
- [ ] Error cases (missing file, invalid JSON, unknown network) produce correct exit codes and helpful messages.
- [ ] Command is documented in `--help`.

**Testing Notes:**
- Unit tests for the command layer (using the stub) + golden-file test for the output JSON shape.

---

### Issue 1.5: Exit codes, error handling, logging
- **Points:** 1
- **Type:** foundation / polish
- **Priority:** P0
- **Blocked by:** 1.1
- **Blocks:** 1.6 (and later phases reuse the constants)
- **Scope:** 0.5–1 day

**Description:**  
Define the official exit code constants and error handling conventions required by the PRD. Introduce a small `internal/errors` package (or equivalent) and ensure all commands use them. Add basic structured logging (or the same style used in eth-deposit-gen).

**Implementation Notes:**
- New file: `internal/errors/exit.go` (or `internal/cli/exit.go`).
- Document the codes in the PRD section and in code comments.
- Ensure errors never leak private material (this will be enforced more strictly in Phase 3/4).

**Acceptance Criteria:**
- [ ] All error paths in Phase 1 commands use the defined exit codes.
- [ ] No raw `os.Exit(1)` or magic numbers left in command code.
- [ ] Logging does not print any sensitive fields (none exist yet, but the pattern is set).

**Testing Notes:**
- Small test that exercises error paths and verifies exit code.

---

### Issue 1.6: Phase-1 tests, help text, README stub
- **Points:** 1
- **Type:** test / docs / polish (vertical slice completion)
- **Priority:** P0
- **Blocked by:** 1.1–1.5
- **Blocks:** Phase 2 start (clean handoff)
- **Scope:** 1 day

**Description:**  
Complete the vertical slice for Phase 1: add unit tests for the new commands and config, improve help text and usage examples, and write the initial README section that describes the tool purpose and Phase 1 status. This satisfies the “each phase includes its polish, tests, and validation” requirement.

**Implementation Notes:**
- Add `_test.go` files for config, build command (stub), and types.
- Update `README.md` with “Getting Started (Phase 1)” and a placeholder for later phases.
- Run `go test ./...` and ensure clean output.

**Acceptance Criteria:**
- [ ] `go test ./...` passes with ≥80% coverage on new packages (or team threshold).
- [ ] README contains accurate installation + basic usage for the stub build flow.
- [ ] Help text is reviewed for clarity and matches PRD documentation bar.
- [ ] Phase 1 exit criteria checklist is satisfied.

**Testing Notes:**
- Include at least one table-driven test and one golden-file test for the unsigned JSON output shape.

---

**Phase 1 Exit Criteria Checklist (for developer sign-off)**
- [ ] All 6 issues closed.
- [ ] `eth-deposit-tx build` produces unsigned tx JSON on Holesky with a deposit-data file.
- [ ] Exit codes and error messages consistent.
- [ ] Tests green, README stub present.
- [ ] Ready for Phase 2 (real TxBuilder) to replace the stub.