# Phase 2: Tx Builder

## Phase Overview
- **Goal:** Replace the Phase 1 stub with a correct, production `internal/tx` implementation (`TxBuilder` interface + `builder.go`) that produces ABI-encoded calldata for the Ethereum deposit contract’s `deposit()` function, handles all input validation (BLS pubkey, withdrawal credentials, signature, deposit_data_root), computes the correct 32 ETH value, and fills gas/EIP-1559 fields properly. At the end of this phase the `build` command emits a production-grade unsigned transaction ready for signing.
- **Issue count:** 5 issues, 12 total points
- **Estimated duration:** ~8–9 days (single-stream)
- **Entry criteria:** Phase 1 complete (types, config, stub build command, exit codes).
- **Exit criteria:**
  - `internal/tx/builder.go` implements `TxBuilder` and produces calldata that matches the deposit contract ABI exactly.
  - `build` command (now using the real builder) succeeds with real deposit-data JSON and produces correct `data` and `value`.
  - Comprehensive unit tests (table-driven, including error cases) pass.
  - Validation against known-good deposit data (cross-checked with eth-deposit-gen output or official test vectors).
  - Phase includes its own polish, tests, and a mini-validation run (unsigned tx inspected on a block explorer or via `cast`).

## Phase Summary

| Issue | Title | Points | Blocked by | Scope | Files |
|-------|-------|--------|------------|-------|-------|
| 2.1 | `internal/tx` package + `TxBuilder` interface + builder skeleton | 2 | Phase 1 types/config | 1 day | `internal/tx/builder.go`, `internal/tx/tx_builder.go` (interface), `internal/tx/abi.go` (maybe) |
| 2.2 | Real ABI encoding for `deposit()` + calldata construction | 3 | 2.1 | 2 days | `internal/tx/builder.go`, ABI binding or manual packing |
| 2.3 | Input validation (lengths, formats, BLS sanity) | 2 | 2.1 | 1–2 days | `internal/tx/validation.go` or methods on types |
| 2.4 | Gas, EIP-1559 fees, nonce, chain ID handling | 3 | 2.2 | 2 days | `internal/tx/builder.go` + config extensions |
| 2.5 | Unit tests + replace stub + Phase-2 validation | 2 | 2.2–2.4 | 2 days | `internal/tx/*_test.go`, update `cmd/eth-deposit-tx/cmd/build.go`, golden files |

## Phase Execution Plan

| Day | Issue | Notes |
|-----|-------|-------|
| 9–10 | 2.1 | Define interface, create package, skeleton that satisfies the build command |
| 11–12 | 2.2 | ABI pack the four deposit() parameters + value = 32 ETH |
| 13–14 | 2.3 | Strict validation; reject bad lengths early with clear exit codes |
| 15–16 | 2.4 | Dynamic gas + fee fetching (or sensible defaults + overrides); nonce handling |
| 17–18 | 2.5 | Full test suite + integrate real builder + produce a verified unsigned tx artifact |

---

## Issues

### Issue 2.1: `internal/tx` package + `TxBuilder` interface + builder skeleton
- **Points:** 2
- **Type:** architecture / foundation
- **Priority:** P0
- **Blocked by:** Phase 1 types/config
- **Blocks:** 2.2, 2.4
- **Scope:** 1 day

**Description:**  
Create the `internal/tx` package per the approved architecture. Define the `TxBuilder` interface (e.g., `BuildUnsigned(ctx, req DepositRequest, cfg NetworkConfig) (UnsignedTx, error)`). Implement a skeleton `builder.go` that the Phase 1 stub can be swapped for without changing the command layer.

**Implementation Notes:**
- New files: `internal/tx/builder.go`, `internal/tx/interface.go` (or `tx_builder.go`), possibly `internal/tx/abi.go`.
- The interface should be minimal and testable (easy to mock for signer tests later).
- Keep all Ethereum-specific logic (ABI, RLP, etc.) inside this package.

**Acceptance Criteria:**
- [ ] `internal/tx` compiles and the build command can import and call the (still stubby) implementation.
- [ ] Interface is defined in a way that Phase 3 signers can depend on `UnsignedTx` output.
- [ ] No CGO introduced.

**Testing Notes:**
- Interface test (compile-time assertion that concrete type satisfies it).

---

### Issue 2.2: Real ABI encoding for `deposit()` + calldata construction
- **Points:** 3
- **Type:** core logic
- **Priority:** P0
- **Blocked by:** 2.1
- **Blocks:** 2.5
- **Scope:** 2 days

**Description:**  
Implement the actual calldata for the deposit contract’s `deposit(bytes pubkey, bytes withdrawal_credentials, bytes signature, bytes32 deposit_data_root)` function. Use either `go-ethereum/accounts/abi` or a lightweight manual pack. Set `value` to exactly 32 ETH (32000000000000000000 wei). Produce the full `UnsignedTx` ready for signing.

**Implementation Notes:**
- File: `internal/tx/builder.go` (main logic), `internal/tx/abi.go` if you extract the ABI definition.
- Hard-code or embed the minimal ABI JSON for the deposit function only (no need for full contract binding).
- Ensure big-endian / 32-byte word alignment is correct.
- Cross-reference the Research document (tx-construction-and-ledger.md) for any known quirks.

**Acceptance Criteria:**
- [ ] For a known-good deposit-data JSON, the produced `data` field, when decoded, matches the expected four parameters.
- [ ] `value` is exactly 32 ETH.
- [ ] `to` is the correct deposit contract for the chosen network.

**Testing Notes:**
- At least one test that round-trips: take deposit data → build → decode calldata → assert fields.

---

### Issue 2.3: Input validation (lengths, formats, BLS sanity)
- **Points:** 2
- **Type:** correctness / security
- **Priority:** P0
- **Blocked by:** 2.1
- **Blocks:** 2.5
- **Scope:** 1–2 days

**Description:**  
Add rigorous validation before any ABI packing. Reject incorrect lengths (pubkey ≠ 48, withdrawal_credentials ≠ 32, signature ≠ 96, root ≠ 32). Optionally add light BLS pubkey format checks (even parity or basic point-on-curve if library cost is low — per research decision). Emit clear, non-sensitive error messages with the correct exit code.

**Implementation Notes:**
- Can live in `internal/tx/validation.go` or as methods on the domain types.
- Must be called early in `BuildUnsigned`.
- Do not log any secret material.

**Acceptance Criteria:**
- [ ] All four length checks are enforced with specific, actionable errors.
- [ ] Unit tests cover every error case.
- [ ] Validation errors use the Phase 1 exit code constants.

**Testing Notes:**
- Table-driven tests with every combination of bad lengths.

---

### Issue 2.4: Gas, EIP-1559 fees, nonce, chain ID handling
- **Points:** 3
- **Type:** core logic / integration
- **Priority:** P0
- **Blocked by:** 2.2
- **Blocks:** 2.5
- **Scope:** 2 days

**Description:**  
Extend the builder to populate realistic `gas`, `maxFeePerGas`, `maxPriorityFeePerGas`, `nonce`, and `chainId`. Support both static overrides (via config/flags) and dynamic fetching from an RPC (if provided). Handle the case where no RPC is available (use sensible defaults or fail with clear guidance).

**Implementation Notes:**
- Use `go-ethereum` RPC client when an RPC URL is configured.
- Respect config values from Phase 1 (`--gas-limit`, `--max-fee-per-gas`, etc.).
- Nonce can be “auto” (fetch from RPC) or “manual” for advanced users.
- This is the last piece before the builder is production-grade for unsigned tx.

**Acceptance Criteria:**
- [ ] With an RPC URL, the builder fetches current base fee + suggests a reasonable priority fee.
- [ ] All fields are present and valid for EIP-1559 transactions on the target chain.
- [ ] Unit tests include both static and dynamic fee paths (the latter using a test RPC or mock client).

**Testing Notes:**
- Mock the RPC client for deterministic tests.

---

### Issue 2.5: Unit tests + replace stub + Phase-2 validation
- **Points:** 2
- **Type:** test / integration / polish (vertical slice completion)
- **Priority:** P0
- **Blocked by:** 2.2, 2.3, 2.4
- **Blocks:** Phase 3
- **Scope:** 2 days

**Description:**  
This issue completes the Phase 2 vertical slice. Replace the stub builder with the real one inside the `build` command. Add comprehensive unit tests for the entire `internal/tx` package. Produce at least one golden “known-good unsigned tx” artifact (checked into the repo or docs) that can be inspected on a block explorer. Update README with Phase 2 status and an example of the new, correct output.

**Implementation Notes:**
- Delete or deprecate `internal/tx/stub_builder.go`.
- Update `cmd/eth-deposit-tx/cmd/build.go` to use the real `TxBuilder`.
- Add table-driven tests, error-path tests, and at least one end-to-end builder test that uses real deposit data.
- Run a manual validation: build an unsigned tx for Holesky, verify the calldata decodes correctly, optionally simulate with `cast` or Foundry.

**Acceptance Criteria:**
- [ ] `go test ./internal/tx/...` passes with high coverage.
- [ ] `eth-deposit-tx build ...` now produces a correct, ABI-accurate unsigned tx.
- [ ] Golden artifact committed (or documented) and verified.
- [ ] README updated with accurate Phase 2 usage and limitations (“signing not yet implemented”).
- [ ] All Phase 2 exit criteria satisfied; code ready for Phase 3 signer work.

**Testing Notes:**
- Include a test that compares the produced calldata against an independent reference (e.g., Python `eth2deposit` or Foundry cast output) if available in CI.

---

**Phase 2 Exit Criteria Checklist**
- [ ] All 5 issues closed.
- [ ] Real `TxBuilder` in place; stub removed.
- [ ] `build` command emits production-grade unsigned deposit transactions.
- [ ] Tests green; validation artifact produced.
- [ ] Ready for Phase 3 (signing not yet implemented).