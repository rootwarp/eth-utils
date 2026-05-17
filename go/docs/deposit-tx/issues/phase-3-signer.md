# Phase 3: Signer

## Phase Overview
- **Goal:** Implement the `Signer` abstraction and two concrete implementations (local private-key for development/testing, and Ledger Nano via CGO for production hardware signing). Wire the signer into the CLI so that after Phase 2’s `build` step, a user can produce a fully signed deposit transaction. This phase owns its own tests (including build-tag-guarded Ledger tests), security hardening, and a hardware smoke-test validation.
- **Issue count:** 6 issues, 14 total points (highest point total — reflects CGO/Ledger complexity per Research)
- **Estimated duration:** ~10–11 days (single-stream); this is the critical path
- **Entry criteria:** Phase 2 complete (real `TxBuilder` producing correct unsigned tx).
- **Exit criteria:**
  - `Signer` interface defined in `internal/signer`.
  - Local signer works (`--signer local --private-key ...` or equivalent, with strong warnings).
  - Ledger signer works on macOS and Linux (requires Ledger Nano with Ethereum app open).
  - `eth-deposit-tx sign` (or `build --sign` flow) produces a signed, RLP-encoded transaction or signed tx JSON.
  - No private key material is ever logged or written to disk by the tool.
  - Unit + integration tests pass (Ledger path guarded by build tag or env).
  - Phase includes its own polish, security review items, and a documented “first successful Ledger-signed deposit tx on testnet” artifact (optional but recommended).

## Phase Summary

| Issue | Title | Points | Blocked by | Scope | Files |
|-------|-------|--------|------------|-------|-------|
| 3.1 | `Signer` interface + package skeleton | 1 | Phase 2 (needs UnsignedTx) | 0.5 day | `internal/signer/signer.go`, `internal/signer/types.go` |
| 3.2 | Local private-key signer implementation | 2 | 3.1 | 1 day | `internal/signer/local.go` |
| 3.3 | Ledger signer skeleton (CGO, hid, build tags, discovery) | 3 | 3.1 | 2–3 days | `internal/signer/ledger.go`, build tags, `cgo` directives |
| 3.4 | Ledger signing flow (EIP-1559, device confirmation, signature application) | 3 | 3.3 | 2–3 days | `internal/signer/ledger.go` (main logic) |
| 3.5 | CLI integration (`sign` command or `--signer` flag on build) | 2 | 3.2, 3.4 | 1–2 days | `cmd/eth-deposit-tx/cmd/sign.go` (new) or updates to build |
| 3.6 | Tests, security hardening, Phase-3 validation | 3 | 3.2–3.5 | 2 days | `*_test.go` files, mock transport, security checklist items |

## Phase Execution Plan (Critical Path Highlighted)

| Day | Issue | Notes |
|-----|-------|-------|
| 19 | 3.1 | Interface only — unblocks parallel thinking for local vs ledger |
| 20–21 | 3.2 | Local signer (easy win, used for all later tests) |
| 22–24 | 3.3 | Ledger skeleton + CGO setup (research findings applied) — highest risk |
| 25–27 | 3.4 | Full Ledger signing flow + device UX (confirmation on screen) |
| 28–29 | 3.5 | Wire into CLI; user can now `build` then `sign` or do it in one shot |
| 30–31 | 3.6 | Tests + security pass + first real Ledger signature on testnet |

---

## Issues

### Issue 3.1: `Signer` interface + package skeleton
- **Points:** 1
- **Type:** architecture
- **Priority:** P0
- **Blocked by:** Phase 2 (UnsignedTx type stable)
- **Blocks:** 3.2, 3.3
- **Scope:** 0.5 day

**Description:**  
Define the `Signer` interface per the architecture (`type Signer interface { Sign(ctx context.Context, unsigned UnsignedTx) (SignedTx, error) }` or equivalent). Create the `internal/signer/` package skeleton with clear separation between transport (for Ledger) and signing logic.

**Implementation Notes:**
- File: `internal/signer/signer.go` (interface + common errors), `internal/signer/types.go` if needed.
- The interface must be mockable for unit tests of the CLI and later integration.
- Document the security contract: “Signer implementations must never log or persist private material.”

**Acceptance Criteria:**
- [ ] Interface compiles and is documented.
- [ ] Both future local and ledger implementations can satisfy it.
- [ ] No CGO in this issue.

**Testing Notes:**
- Compile-time interface satisfaction tests.

---

### Issue 3.2: Local private-key signer implementation
- **Points:** 2
- **Type:** feature
- **Priority:** P0
- **Blocked by:** 3.1
- **Blocks:** 3.5, 3.6 (used heavily for tests)
- **Scope:** 1 day

**Description:**  
Implement a `LocalSigner` that takes a raw private key (hex or keystore) and signs EIP-1559 transactions using `go-ethereum/crypto` and `types.SignTx`. This is the primary signer for development, CI, and automated tests.

**Implementation Notes:**
- New file: `internal/signer/local.go`.
- Accept private key via a secure input path (flag that reads from stdin or a file descriptor, never from command-line history in examples).
- Strong warnings in help text: “For development only. Never use with real funds.”
- Must produce a valid `SignedTx` that can be RLP-encoded and broadcast.

**Acceptance Criteria:**
- [ ] Local signer successfully signs a Phase 2 unsigned tx and produces a valid signature (r, s, v).
- [ ] Unit tests cover happy path and bad-key cases.
- [ ] No private key is ever printed in logs or error messages.

**Testing Notes:**
- Use `crypto.GenerateKey()` in tests; never hard-code real keys.

---

### Issue 3.3: Ledger signer skeleton (CGO, hid, build tags, discovery)
- **Points:** 3
- **Type:** integration / hardware (high risk)
- **Priority:** P0
- **Blocked by:** 3.1
- **Blocks:** 3.4
- **Scope:** 2–3 days

**Description:**  
Implement the Ledger signer skeleton following the Research findings (tx-construction-and-ledger.md). Introduce CGO with the appropriate Ledger HID library (e.g., `github.com/ledgerhq/ledger-go` or `hid`), use build tags (`// +build ledger` or `cgo`), implement device discovery, open the Ethereum app, and handle the “app not open / wrong app” error cases gracefully.

**Implementation Notes:**
- Main file: `internal/signer/ledger.go` (with CGO blocks).
- Create a separate file or build-tag variant for non-CGO platforms that returns a clear “Ledger not supported on this platform” error.
- Implement `LedgerSigner` struct that satisfies `Signer`.
- Discovery should list connected devices and select the first (or allow `--ledger-device` index/path).
- Reference the Research doc for exact library choice, known pitfalls, and cross-compile strategy.

**Acceptance Criteria:**
- [ ] Code compiles on macOS and Linux with `CGO_ENABLED=1` and the `ledger` build tag.
- [ ] On platforms without CGO or without the tag, the binary still builds and gives a helpful error when Ledger is requested.
- [ ] Device discovery works (at least enumerates a connected Ledger).
- [ ] Clear error if Ethereum app is not open.

**Testing Notes:**
- The real Ledger path is guarded; provide a mock transport for unit tests of higher layers.

---

### Issue 3.4: Ledger signing flow (EIP-1559, device confirmation, signature application)
- **Points:** 3
- **Type:** core logic / hardware (highest complexity)
- **Priority:** P0
- **Blocked by:** 3.3
- **Blocks:** 3.5, 3.6
- **Scope:** 2–3 days

**Description:**  
Implement the full signing round-trip on Ledger: serialize the EIP-1559 transaction in the exact format the Ledger Ethereum app expects, send the APDU / sign request, wait for user confirmation on the device screen (“Sign deposit tx? 32 ETH to deposit contract on Holesky?”), receive the signature (r, s, v), and apply it to produce a `SignedTx`.

**Implementation Notes:**
- All in `internal/signer/ledger.go`.
- Must handle user rejection on device (return a specific “user rejected” error with correct exit code).
- Must never expose the private key (it never leaves the device).
- Follow the exact APDU / BIP-44 derivation path and chain ID handling recommended in the Research doc.
- Add progress / “please confirm on device” user feedback.

**Acceptance Criteria:**
- [ ] A real Ledger Nano (with Ethereum app open) can sign a testnet deposit tx produced by the Phase 2 builder.
- [ ] Rejection on the device produces a clean, non-crashing error with the correct exit code.
- [ ] Signature is valid and the resulting signed tx passes `types.Transaction` verification.

**Testing Notes:**
- This issue requires at least one manual hardware test. Record the first successful signature as a golden artifact (unsigned + signed pair) for regression.

---

### Issue 3.5: CLI integration (`sign` command or `--signer` flag)
- **Points:** 2
- **Type:** integration / UX
- **Priority:** P0
- **Blocked by:** 3.2, 3.4
- **Blocks:** 3.6
- **Scope:** 1–2 days

**Description:**  
Expose the signers to the user. Options (choose one or both per architecture decision):
- A new `sign` subcommand that takes an unsigned tx JSON (from `build --out`) and a `--signer local|ledger` flag.
- Or extend the `build` command with a `--sign` / `--signer` mode that does build + sign in one shot.
- Support for supplying the private key securely for the local signer (stdin, file, or env with warning).
- For Ledger: clear instructions and device confirmation prompt.

**Implementation Notes:**
- New or modified file: `cmd/eth-deposit-tx/cmd/sign.go` (preferred for separation) or heavy updates to `build.go`.
- Update root command and help text.
- Ensure the output is a signed transaction (JSON with `raw` RLP hex, or separate `.signed.json` + `.signed.raw` files).

**Acceptance Criteria:**
- [ ] User can run `eth-deposit-tx sign --signer local --private-key <...> --in unsigned.json --out signed.json`.
- [ ] Same flow works with `--signer ledger` (device required for test).
- [ ] Help text clearly documents the security implications of each signer.
- [ ] Exit codes for “user rejected on device”, “no device found”, “invalid key”, etc. are correct.

**Testing Notes:**
- Local signer path must be fully unit-testable (no hardware).

---

### Issue 3.6: Tests, security hardening, Phase-3 validation
- **Points:** 3
- **Type:** test / security / polish (vertical slice completion)
- **Priority:** P0
- **Blocked by:** 3.2–3.5
- **Blocks:** Phase 4
- **Scope:** 2 days

**Description:**  
Complete the Phase 3 vertical slice. Add unit tests for the local signer and mock-based tests for the Ledger path. Perform a security hardening pass (no secret leakage in errors/logs, buffer clearing, clear warnings). Produce a documented “first successful Ledger-signed testnet deposit tx” artifact (or at minimum a local-signed one if hardware is unavailable in CI). Update README with Phase 3 status and security section.

**Implementation Notes:**
- Create `internal/signer/local_test.go`, `internal/signer/ledger_test.go` (build-tag guarded), and a mock transport.
- Run the full security checklist items that apply to Phase 3 (from PRD).
- Commit a pair of unsigned + signed tx files (for local signer at minimum) as regression artifacts.

**Acceptance Criteria:**
- [ ] `go test ./internal/signer/...` passes (Ledger tests skipped or mocked on non-hardware CI).
- [ ] A real or simulated end-to-end sign flow succeeds and produces a verifiable signature.
- [ ] No private key material appears in any log, error message, or test output.
- [ ] README security section for signing is accurate.
- [ ] All Phase 3 exit criteria satisfied; code ready for full pipeline integration in Phase 4.

**Testing Notes:**
- Include a test that exercises the “user rejected on device” path via the mock.

---

**Phase 3 Exit Criteria Checklist**
- [ ] All 6 issues closed.
- [ ] Both local and Ledger signers work end-to-end.
- [ ] Security hardening complete for this phase.
- [ ] First real (or mock) signed deposit tx artifact produced and verified.
- [ ] Ready for Phase 4 (full pipeline + broadcast + release).