# Phase 4: Integration

## Phase Overview
- **Goal:** Deliver the complete, production-ready vertical slice: wire the full pipeline (`build` → `sign` with either signer → optional `send`/`broadcast`), implement end-to-end validation on a public testnet (Holesky or Sepolia), meet the PRD documentation bar, replicate the `eth-deposit-gen` release process (Makefile, CI, versioning), and perform the final security checklist. This phase owns its own polish, tests, and validation.
- **Issue count:** 6 issues, 13 total points
- **Estimated duration:** ~9–10 days (single-stream)
- **Entry criteria:** Phase 3 complete (both signers working, CLI integration done).
- **Exit criteria:**
  - A user can run a single command (or short sequence) that produces a signed, ready-to-broadcast deposit transaction for any supported network.
  - Optional `--broadcast` / `send` subcommand works on testnet with a funded test account (with confirmation prompt).
  - Full E2E test (build + local sign + optional broadcast) passes in CI or a documented manual run.
  - A real deposit has been successfully submitted to the Holesky or Sepolia deposit contract using the tool (or the signed tx has been verified and would succeed).
  - Documentation (README + security + examples) meets the PRD standard.
  - Makefile, cross-compile (with CGO for Ledger), and release process match or exceed `eth-deposit-gen`.
  - Final security checklist passed (no secret leakage, correct exit codes everywhere, redacted errors, etc.).
  - Version bumped and CHANGELOG entry present.

## Phase Summary

| Issue | Title | Points | Blocked by | Scope | Files |
|-------|-------|--------|------------|-------|-------|
| 4.1 | End-to-end pipeline wiring (build → sign → signed output) | 3 | Phase 3 | 2 days | `cmd/eth-deposit-tx/cmd/build.go` (or new `run.go`), `cmd/eth-deposit-tx/cmd/send.go` skeleton |
| 4.2 | Optional broadcast / “send” subcommand with RPC + confirmation | 2 | 4.1 | 1–2 days | `cmd/eth-deposit-tx/cmd/send.go`, RPC client usage |
| 4.3 | E2E validation on testnet + golden artifacts | 3 | 4.1, 4.2 | 2–3 days | Test scripts, `testdata/`, CI job, Holesky/Sepolia run |
| 4.4 | Full UX polish, examples, consistent errors, exit codes | 2 | 4.1 | 1–2 days | All `cmd/.../*.go`, help text, error messages |
| 4.5 | Documentation (README, security, usage) to PRD bar | 2 | 4.3 | 1–2 days | `README.md`, `docs/USAGE.md` or equivalent, `docs/SECURITY.md` |
| 4.6 | Makefile, CI, release process, final security checklist, version bump | 1 | 4.4, 4.5 | 1 day | `Makefile`, `.github/workflows/`, `CHANGELOG.md`, version const |

## Phase Execution Plan

| Day | Issue | Notes |
|-----|-------|-------|
| 32–33 | 4.1 | Wire the complete happy path (local signer first, then ledger) |
| 34–35 | 4.2 | Add `send` with double-confirmation (amount + “this will spend real ETH”) |
| 36–38 | 4.3 | Real testnet run; produce and archive the successful deposit tx / receipt |
| 39 | 4.4 | Polish pass across the entire CLI |
| 40–41 | 4.5 | Documentation sprint — must hit the PRD bar |
| 42 | 4.6 | Release engineering + final security sign-off + tag |

---

## Issues

### Issue 4.1: End-to-end pipeline wiring (build → sign → signed output)
- **Points:** 3
- **Type:** integration / vertical slice completion
- **Priority:** P0
- **Blocked by:** Phase 3 complete
- **Blocks:** 4.2, 4.3
- **Scope:** 2 days

**Description:**  
Connect the pieces so that the primary user flow works: `eth-deposit-tx build --network holesky --deposit-data d.json --signer local ...` (or the two-step `build` then `sign`) produces a fully signed transaction ready for broadcast. Support both local and Ledger signers. Output formats: signed JSON + raw RLP hex file (for easy `cast send` or direct broadcast).

**Implementation Notes:**
- May introduce a thin “run” or “deposit” high-level command, or keep the existing `build --sign` / `sign` split — follow the UX decision made in Phase 3.
- Ensure the pipeline is resilient to partial failures (e.g., build succeeds, sign fails → no partial files left in bad state).
- This issue completes the core happy-path vertical slice for the entire tool.

**Acceptance Criteria:**
- [ ] End-to-end with local signer produces a valid signed tx that can be broadcast.
- [ ] Same flow works with Ledger (manual test).
- [ ] Output artifacts are consistent and well-named.
- [ ] Error paths (signer failure, user rejection) clean up properly and use correct exit codes.

**Testing Notes:**
- Unit test the high-level flow with the local signer + mocks.

---

### Issue 4.2: Optional broadcast / “send” subcommand with RPC + confirmation
- **Points:** 2
- **Type:** feature / safety
- **Priority:** P1 (or P0 if PRD requires it)
- **Blocked by:** 4.1
- **Blocks:** 4.3
- **Scope:** 1–2 days

**Description:**  
Add a `send` (or `broadcast`) subcommand that takes a signed tx (or runs the full flow) and submits it via RPC. Include strong confirmation prompts because this spends real ETH (even on testnet the user should be intentional). Support gas bumping / replacement if desired, but keep it minimal per the plan.

**Implementation Notes:**
- New file: `cmd/eth-deposit-tx/cmd/send.go`.
- Use the same RPC client patterns from Phase 2.
- Prompt: “You are about to broadcast a deposit of 32 ETH to the Holesky deposit contract. Type the network name to confirm.”
- On success, print the tx hash and a link to the explorer.

**Acceptance Criteria:**
- [ ] `eth-deposit-tx send --signed signed.json --rpc-url <testnet>` broadcasts successfully.
- [ ] Confirmation prompt cannot be bypassed accidentally (no `--yes` in the first implementation, or it requires explicit opt-in).
- [ ] Failure to broadcast (nonce too low, insufficient funds, etc.) produces clear exit code + message.

**Testing Notes:**
- The broadcast path is harder to unit-test; use a mock RPC client and at least one manual testnet run (see 4.3).

---

### Issue 4.3: E2E validation on testnet + golden artifacts
- **Points:** 3
- **Type:** validation / test (highest value for confidence)
- **Priority:** P0
- **Blocked by:** 4.1, 4.2
- **Blocks:** 4.5 (docs need real examples)
- **Scope:** 2–3 days

**Description:**  
Execute a complete end-to-end deposit on Holesky or Sepolia using the tool (local signer for CI, Ledger for the “production” artifact). Record the unsigned tx, signed tx, broadcast tx hash, and receipt. Commit the artifacts under `testdata/deposit-e2e/` or similar. If a real deposit is made, note the validator pubkey (it will be used only for testing and can be discarded later).

**Implementation Notes:**
- Requires a funded testnet account with >32 ETH.
- Script or Makefile target to reproduce the run.
- If Ledger is used for the final artifact, document the exact firmware/app version.
- This is the ultimate validation that the research, builder, and signer all work together.

**Acceptance Criteria:**
- [ ] At least one successful deposit transaction has been submitted to a public testnet deposit contract using this tool.
- [ ] Artifacts (unsigned, signed, receipt) are committed and the tx can be independently verified on the explorer.
- [ ] A short “Validation Report” (Markdown) is added to `docs/validation/` describing the run, date, network, tool version, and any deviations.
- [ ] CI job (or documented manual step) can re-run the local-signer path against a testnet RPC.

**Testing Notes:**
- This is the capstone test for the entire project.

---

### Issue 4.4: Full UX polish, examples, consistent errors, exit codes
- **Points:** 2
- **Type:** polish / UX
- **Priority:** P0
- **Blocked by:** 4.1
- **Blocks:** 4.5, 4.6
- **Scope:** 1–2 days

**Description:**  
Perform a final pass across all commands: consistent flag naming, improved help text and examples, better progress / confirmation messages, uniform error formatting, and guaranteed use of the defined exit codes everywhere. Add a few realistic end-to-end examples in the help or a dedicated `examples/` directory.

**Implementation Notes:**
- Touch files across `cmd/eth-deposit-tx/cmd/`.
- Add or improve shell-completion support if the base tool has it.
- Ensure `eth-deposit-tx --help` and subcommand help are exemplary.

**Acceptance Criteria:**
- [ ] Running any command with invalid args produces the correct exit code and a helpful message (no stack traces).
- [ ] Examples in `--help` or docs can be copy-pasted (with placeholders).
- [ ] All PRD-mandated exit codes are used in their documented situations.

**Testing Notes:**
- Add a small “help output” test or golden file if the team values it.

---

### Issue 4.5: Documentation (README, security, usage) to PRD bar
- **Points:** 2
- **Type:** docs
- **Priority:** P0
- **Blocked by:** 4.3 (needs real examples and validation results)
- **Blocks:** 4.6
- **Scope:** 1–2 days

**Description:**  
Bring all user-facing documentation up to the standard defined in the PRD (completeness, security considerations, examples, warnings, platform notes for Ledger, build instructions with CGO, etc.). Create or update:
- `README.md` (quick start, install, basic usage, supported networks)
- `docs/USAGE.md` or equivalent (full command reference)
- `docs/SECURITY.md` (key handling, Ledger, what the tool does and does not do, threat model)
- Mention of exit codes and their meanings

**Implementation Notes:**
- Follow the exact documentation requirements from the PRD section “Documentation Bar”.
- Include the validation report link or summary.
- Add a “Contributing / Building from source” section that explains the Ledger CGO build.

**Acceptance Criteria:**
- [ ] Documentation review passes the PRD checklist (completeness, accuracy, security warnings, examples).
- [ ] A new user can follow the README from `go install` (or build) through a successful testnet deposit.
- [ ] Security document explicitly calls out that the tool never exports private keys when using Ledger and that local signing is for testing only.

**Testing Notes:**
- Documentation is reviewed by at least one other person (or the product owner).

---

### Issue 4.6: Makefile, CI, release process, final security checklist, version bump
- **Points:** 1
- **Type:** release / polish / security (vertical slice completion)
- **Priority:** P0
- **Blocked by:** 4.4, 4.5
- **Blocks:** none (final issue)
- **Scope:** 1 day

**Description:**  
Replicate the release engineering of `eth-deposit-gen`:
- `Makefile` with targets for build (CGO and non-CGO), test, lint, cross-compile (Linux/macOS, amd64/arm64), and release archives.
- GitHub Actions (or equivalent) that builds the Ledger-enabled binaries on the right runners.
- Version bump (from dev to v0.1.0 or the first tagged release).
- `CHANGELOG.md` entry.
- Final pass of the full PRD security checklist (applied across all code, not just Phase 4 work).

**Implementation Notes:**
- Copy/adapt the Makefile and CI from `cmd/eth-deposit-gen/` with appropriate renames.
- Add a `make release` or `goreleaser` config if that is the pattern.
- Tag the repo (or prepare the PR for tagging).

**Acceptance Criteria:**
- [ ] `make build` and `make release` (or equivalent) produce the expected binaries, including the Ledger-enabled variant on the correct OS.
- [ ] CI is green.
- [ ] Final security checklist (from PRD) has been walked through and all items are either implemented or explicitly accepted as out-of-scope for v1.
- [ ] Tool is ready to be tagged and announced.

**Testing Notes:**
- The release artifacts themselves are the ultimate test.

---

**Phase 4 Exit Criteria Checklist (Project Completion)**
- [ ] All 6 issues closed.
- [ ] Full pipeline works (build + sign + optional send).
- [ ] Real testnet deposit executed and artifacts archived.
- [ ] Documentation meets PRD bar.
- [ ] Release process matches eth-deposit-gen quality.
- [ ] Final security checklist passed.
- [ ] Version tagged / ready for release.

**Project Completion Criteria (from PRD + Plan)**
- The `eth-deposit-tx` tool is functionally complete, documented, and releasable.
- A user can construct, sign (with Ledger or local key), and optionally broadcast a valid Ethereum deposit transaction for mainnet or supported testnets.
- All P0 requirements from the PRD are satisfied.