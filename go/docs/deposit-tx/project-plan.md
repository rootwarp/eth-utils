# Project Plan: eth-deposit-tx

## Summary
Deliver a production-grade `eth-deposit-tx` CLI (sibling to the mature `eth-deposit-gen`) that ingests deposit JSON, constructs correct Ethereum deposit transactions using go-ethereum RLP + ABI, and supports a first-class two-phase air-gapped workflow ("build" → portable unsigned artifact → "sign"). Ledger hardware is the primary signer (CGO-isolated via build tags) with a private-key fallback that performs secure zeroization; optional RPC is available for gas/nonce/chainID but never required.

The plan exploits very high reuse of the existing `eth-deposit-gen` skeleton (cmd layout, cli.go patterns, ldflags versioning, exit codes, security notes, Makefile targets, golden-test style, internal/ packages for network/config, e2e/Hoodi patterns) so that new work is concentrated in `internal/tx` (TxBuilder + interchange) and `internal/signer` (pluggable Signer interface + Ledger + privkey impls) plus excellent Ledger UX, error messages, and documentation.

**Key structural change from previous draft (per user direction):**  
Polish, tests, documentation, error UX, and validation are **owned by each feature implementation phase** (vertical slices). There is no large separate "Polish" or "Tests" phase at the end. Each major capability (CLI foundation, Tx Builder + interchange, Signer + Ledger) is delivered production-ready (code + tests + docs + UX + validation) before the next phase begins. A small final integration/release phase handles cross-cutting concerns and ship readiness.

**Overall estimate:** 4 phases, single-stream team, ~1-2 developer weeks total (per research feasibility), with risk reduced early via Ledger spike and stable interchange design. Each phase produces shippable, tested, documented value.

## Prerequisites
- Approved architecture.md (Signer interface, TxBuilder, package boundaries `cmd/eth-deposit-tx` / `internal/tx` / `internal/signer`, reuse of `internal/cli` + `internal/network`, ADRs for build tags and interchange).
- Decision on final interchange format between build/sign phases (JSON envelope containing RLP, tx fields, chainID, metadata, version) — to be recorded as ADR early in Phase 2.
- Physical access to Ledger Nano S or X with Ethereum app installed + unlocked for real-hardware testing and UX validation (mandatory for P0 Ledger path).
- Go 1.21+ toolchain matching the monorepo, plus CGO build environment (libusb-1.0 dev headers on macOS/Linux) for Ledger development; cross-compile notes for releases.
- Confirmed deposit contract addresses and network constants (mainnet, Hoodi, Sepolia, Holesky) — extend/reuse existing `internal/network`.
- Agreement on P0 scope: build + sign (no auto-broadcast), private-key via env var only (security), legacy + 1559 tx support as needed, output to stdout/file/hex+JSON.
- Review of research findings (Ledger SDK patterns, CGO realities, gas estimation, prior art) and PRD (P0/P1/P2 split, user stories, success metrics, out-of-scope items, risks).
- Existing `eth-deposit-gen` testdata/golden files and Hoodi e2e harness available for reuse.

## Phase 1: CLI Foundation + Basic Build (with tests, docs & validation)
**Goal:** Bootstrap the `cmd/eth-deposit-tx` binary by directly reusing the proven `eth-deposit-gen` structure and patterns (main.go + cli.go, flag handling, version ldflags, help text, exit codes, security posture). Deliver a working "build" command that parses deposit JSON and emits a stable unsigned RLP + interchange artifact. Include unit/golden tests, basic documentation, Makefile integration, and early validation that the reuse model works and the air-gap workflow is already usable.

**Duration estimate:** small (2-3 days)

### Tasks
- [ ] Task 1.1 — Scaffold `cmd/eth-deposit-tx/` (copy/adapt tree), adapt `main.go` + `cli.go` from `eth-deposit-gen` (update app name, commands, copyright, version injection via `-ldflags "-X main.version=..."` + git commit/date). Preserve deps injection pattern for testability. Add subcommand skeleton for `build` and `sign`.
  - Dependencies: none
  - Complexity: low
- [ ] Task 1.2 — Implement minimal "build" subcommand surface (flags: `--input` (file/stdin), `--network`, `--output`, manual gas/chain overrides). Parse deposit JSON (reuse schema/validation logic or copy from deposit-gen where shared). Wire to a minimal TxBuilder stub.
  - Dependencies: Task 1.1
  - Complexity: low
- [ ] Task 1.3 — Create `internal/tx` package skeleton + stub `TxBuilder` that assembles a deposit transaction (go-ethereum `types`, `rlp`, basic ABI calldata for deposit contract) and outputs unsigned RLP hex + a first-draft interchange format (hex + minimal JSON envelope). Record the interchange format as an ADR.
  - Dependencies: Task 1.1
  - Complexity: medium
- [ ] Task 1.4 — Add unit/golden tests for the stub builder and RLP output (following existing golden-test style from eth-deposit-gen). Add basic CLI surface tests via the injectable run callback pattern.
  - Dependencies: Task 1.3
  - Complexity: low
- [ ] Task 1.5 — Wire basic Makefile integration (add to `go/Makefile` test/lint/vet/build targets; ensure `go build ./cmd/eth-deposit-tx` and `make` succeed). Add first draft of command help and security notes.
  - Dependencies: Task 1.1
  - Complexity: low
- [ ] Task 1.6 — Early validation: run `build` on real Hoodi deposit JSON (from eth-deposit-gen), produce RLP, manually decode/verify it is a valid unsigned deposit tx. Update README skeleton with the first air-gap example.
  - Dependencies: Tasks 1.2-1.3
  - Complexity: low

### Phase Exit Criteria
- `go run ./cmd/eth-deposit-tx --help` and `build --help` render clean, consistent UX matching eth-deposit-gen quality.
- `eth-deposit-tx build --input testdata/sample.json --network hoodi --output -` emits valid unsigned RLP hex (manually decodable and matching expected deposit contract calldata).
- Compiles, lints, vets cleanly; new code follows project style and security notes.
- Basic unit + golden tests pass; no signer/Ledger code present.
- Makefile targets updated; first air-gap workflow (build only) is usable and documented.
- Interchange format ADR is written and approved.

---

## Phase 2: Full Tx Builder + Interchange + Optional RPC (with polish, tests & validation)
**Goal:** Replace the stub with a correct, production `internal/tx` implementation (full ABI packing for the deposit() call, proper RLP for the transaction, support for all deposit data fields). Finalize and lock the stable interchange format. Add optional `ethclient` integration for gas/nonce/chainID (with full manual overrides). Deliver the feature production-ready: polished error messages, updated docs, comprehensive tests, and validation that both online and pure air-gapped paths work correctly.

**Duration estimate:** medium (3-4 days)

### Tasks
- [ ] Task 2.1 — Finalize interchange format (JSON with `chainId`, `nonce`, `gas`/`maxFeePerGas`, `to`, `data` (ABI-encoded), `value`, `rlp` hex, `metadata` for deposits, version). Lock the format and update the ADR. Ensure it is self-contained for offline sign.
  - Dependencies: Phase 1 exit (basic RLP + draft format exists)
  - Complexity: low-medium
- [ ] Task 2.2 — Implement full `internal/tx` builder: correct Beacon deposit contract ABI, `depositData` struct encoding, RLP transaction assembly (support legacy + 1559 as appropriate). Add table-driven unit tests with golden vectors (reuse Hoodi golden data from eth-deposit-gen).
  - Dependencies: Task 2.1
  - Complexity: high (exact calldata correctness is critical)
- [ ] Task 2.3 — Add optional RPC client (ethclient) behind `--rpc-url` / env. Auto-fetch nonce, estimate gas (with safe buffer), chainID. Graceful degradation when offline or flag omitted; all values overridable via flags. Polish error messages for RPC vs manual override cases.
  - Dependencies: Task 2.2
  - Complexity: medium
- [ ] Task 2.4 — Complete "build" command with full polish: support `--output-format` (hex, json, file), improved validation, clear progress/UX messages, final help text and examples. Update README with complete build examples (online + air-gapped).
  - Dependencies: Tasks 2.1-2.3
  - Complexity: medium
- [ ] Task 2.5 — Comprehensive testing + validation for the build path: golden RLP vectors, interchange roundtrips, RPC failure modes, manual override paths, error message quality. Early user-style validation on Hoodi data.
  - Dependencies: Tasks 2.2-2.4
  - Complexity: medium

### Phase Exit Criteria
- Builder produces byte-identical RLP for known-good deposit inputs (validated against golden files or sibling e2e data).
- Interchange artifact is stable, versioned, and round-trippable; used by both build and (future) sign.
- RPC path works when provided; pure offline path works with manual flags and clear guidance.
- Unit + table + golden tests cover the full builder; RPC paths are mockable; coverage target met for `internal/tx`.
- CLI surface for `build` is polished and documented; README examples are copy-paste ready.
- Both online and air-gapped `build` workflows have been manually validated end-to-end.

---

## Phase 3: Signer Module — Ledger Primary + Private-Key Fallback (with polish, tests & hardware validation)
**Goal:** Deliver the pluggable `internal/signer` package exactly as specified in architecture (clean `Signer` interface with `Sign` + `Close`). Perform the dedicated Ledger spike, then implement Ledger signer (primary path, CGO via `//go:build ledger`, using recommended Ledger Go SDK patterns, excellent blind-signing UX and error messages for common pitfalls) and private-key fallback (env-var only, immediate zeroization). Add factory selection logic and the "sign" command. Deliver the entire sign feature production-ready: rich UX, comprehensive tests (mocks + gated real hardware), updated docs, and real-hardware validation on a testnet.

**Duration estimate:** medium-large (4-6 days; includes dedicated Ledger spike)

### Tasks
- [ ] Task 3.1 — Ledger spike/prototype (front-loaded): minimal standalone program (or test) that connects to Ledger, selects Ethereum app, derives address, signs a sample tx hash from Phase 2. Catalog failure modes (app not open, locked, blind signing disabled, firmware, permissions, CGO build issues) and draft user-facing error strings + UX guidance. Update research/implementation notes. This is the highest-risk item — do it early.
  - Dependencies: Phase 2 (need real unsigned tx to sign)
  - Complexity: high
- [ ] Task 3.2 — Define `Signer` interface in `internal/signer/signer.go` (and supporting types) per architecture. Create factory/selector logic (try Ledger first, fallback to privkey if flag/env present, clear mutually-exclusive rules and actionable errors).
  - Dependencies: Task 3.1 start
  - Complexity: low
- [ ] Task 3.3 — Implement `ledger/signer.go` (build-tagged): connection, derivation path (default + override), blind signing flow with clear CLI instructions ("Review the transaction hash on your Ledger and approve"), rich error mapping, and UX polish from the spike.
  - Dependencies: Tasks 3.1-3.2
  - Complexity: high
- [ ] Task 3.4 — Implement `privkey/signer.go` (no CGO): read from secure env var, sign via go-ethereum, zeroize key material immediately after use (document the technique and add tests that assert wipe).
  - Dependencies: Task 3.2
  - Complexity: medium
- [ ] Task 3.5 — Implement "sign" subcommand: reads interchange (from build), selects signer via factory, emits signed RLP/hex/JSON, supports `--ledger` / `--private-key` (or env), `--output`, `--ledger-index`. Full error/UX polish.
  - Dependencies: Phase 2 interchange + Tasks 3.3-3.4
  - Complexity: medium
- [ ] Task 3.6 — Comprehensive testing + hardware validation for the sign path: full privkey unit tests + zeroization assertions; Ledger paths with mocks + gated real-hardware tests; end-to-end build → sign on Hoodi with real Ledger device; validation that signed tx is correct and would succeed on-chain.
  - Dependencies: Tasks 3.3-3.5
  - Complexity: high
- [ ] Task 3.7 — Update README with complete air-gapped + real Ledger examples, Ledger setup/troubleshooting section, and security model. Add Ledger-specific error catalog to docs and command help.
  - Dependencies: Tasks 3.3, 3.6
  - Complexity: low

### Phase Exit Criteria
- `eth-deposit-tx sign --help` clearly documents both paths and security model.
- Real Ledger (Eth app open) successfully performs blind signature of a deposit tx; CLI provides excellent guidance and device instructions.
- No Ledger present → clear fallback or error with next steps.
- Private-key path signs correctly and wipes material (code review + test assertion).
- `go build` (no tags) and `go build -tags ledger` both succeed; non-Ledger users unaffected.
- Privkey unit tests pass with high coverage; Ledger paths have mocks + at least one successful real-hardware execution recorded.
- Full end-to-end air-gapped workflow (build on one machine → transfer artifact → sign on Ledger machine) has been validated on Hoodi.
- Security notes, README, and error UX for the entire sign feature are production quality.

---

## Phase 4: End-to-End Integration, Release Prep & Handover
**Goal:** Perform final cross-feature integration validation (full build → sign on testnet + real hardware), address any remaining UX or edge-case gaps, ensure CI/release machinery treats the new tool as first-class, produce release artifacts, and hand over a polished, documented, tested tool ready for users. All polish, testing, and validation for the overall system happens here after the individual features are already production-ready.

**Duration estimate:** small-medium (2-4 days; gated by hardware availability)

### Tasks
- [ ] Task 4.1 — Final end-to-end validation on Hoodi (preferred): complete `eth-deposit-gen` → JSON → `eth-deposit-tx build` (offline) → file transfer → `sign` with real Ledger → verify signed tx (decode, optional small funded test deposit or dry-run submit). Record the full successful flow.
  - Dependencies: Phase 3 exit
  - Complexity: medium
- [ ] Task 4.2 — Bug-bash + final UX iteration from the full validation (any remaining Ledger flow, error message, or documentation gaps). Back-port fixes.
  - Dependencies: Task 4.1
  - Complexity: medium
- [ ] Task 4.3 — Full CI + release integration: ensure `make test`, `make lint`, `make vet`, `make build` (both tagged and untagged) include `eth-deposit-tx`; add any required cross-compile or release steps; verify reproducible builds.
  - Dependencies: Phase 1 (initial Makefile) + all prior
  - Complexity: low
- [ ] Task 4.4 — Final documentation & meta: close any open ADRs, update tracking docs, write short release notes, ensure all P0 success metrics (real hardware + testnet validation, excellent README with 4+ examples, security model) are demonstrable.
  - Dependencies: all prior
  - Complexity: low
- [ ] Task 4.5 — Optional P1/P2 items (e.g., simple `broadcast` helper, additional networks, shell completion) only if time and priority allow after P0 is solid.
  - Dependencies: Phase 3
  - Complexity: low

### Phase Exit Criteria
- Complete real-hardware + real-testnet deposit flow has been executed end-to-end and recorded (tx visible on explorer).
- All critical UX or correctness issues from validation resolved; no open P0s.
- Release binaries build and run correctly for both Ledger-enabled and plain users; `make` targets are complete.
- All artifacts (code, tests, docs, this plan, README) are up-to-date and consistent.
- Tool is ready for broader release or user validation.

---

## Dependency Graph
Critical path (single stream):

Phase 1 (CLI skeleton + basic build + tests + docs + interchange ADR)  
→ Phase 2 (full builder + stable interchange + RPC + tests + validation for build path)  
  → Phase 3 (Ledger spike + signer impl + sign command + tests + real-hardware validation for sign path)  
   → Phase 4 (full e2e validation + release integration + handover)

Parallel work (where safe):
- Ledger spike (Task 3.1) can begin as soon as a valid unsigned tx exists (late Phase 1 / early Phase 2).
- Documentation writing can run in parallel once commands stabilize.
- Makefile/CI updates can start early and be refined.

Hard blockers:
- Stable interchange (Phase 2) before any sign implementation.
- Real Ledger hardware access before completing Phase 3 and Phase 4 validation.
- Passing tests + full e2e validation before release.

## Risk Register

| Risk | Impact | Likelihood | Mitigation (built into plan) |
|------|--------|------------|------------------------------|
| Ledger CGO build, cross-compile, and device variability (OS, firmware, app version) | High | Medium | Dedicated spike (3.1) early in Phase 3; build tags isolate impact; rich error catalog + README troubleshooting; always-shippable no-tag binary; documented build env |
| Sub-par blind-signing UX or confusing Ledger errors causing user failure | High | Medium-High | Spike + explicit UX/polish tasks inside Phase 3; clear CLI + on-device guidance; catalog of common mistakes and fixes; real-hardware validation before phase exit |
| Interchange format instability between build and sign | Medium | Low | Design + ADR locked in Phase 2.1; versioned format; golden roundtrip tests in Phase 2; used by both phases |
| Gas estimation / fee-market edge cases or RPC failures breaking air-gap | Medium | Medium | Full manual overrides + clear errors in Phase 2; both paths unit-tested and validated before phase exit |
| Private-key material leakage or incomplete zeroization | High | Low | Env-var only, explicit zeroize impl + review + tests inside Phase 3; never shown in help/logs |
| JSON schema drift vs. eth-deposit-gen or spec | Medium | Low | Reuse parsing where possible; golden tests from real deposits; validate command in Phase 2 |
| Insufficient hardware test coverage in CI | Medium | High | Interface + mocks for most logic (Phases 2-3); gated real-Ledger target + manual checklist in Phases 3-4 |
| Scope creep (broadcast, extra networks, batching) | Low | Medium | Strict adherence to PRD P0/P1/P2 and out-of-scope list; only in Phase 4 if P0 is solid |

## Technical Spikes / Open Questions
- Ledger SDK choice, derivation path, exact blind-signing APDU flow, multi-Ledger handling, timeout behavior (Task 3.1 — Phase 3).
- Final interchange format details (metadata depth, EIP-1559 vs legacy, versioning) — locked in Phase 2.
- Private-key env var name and whether file or interactive prompt is ever allowed (P0 security decision — Phase 3).
- Build tag name (`ledger` vs `hw`) and release artifact strategy (single binary with runtime detect vs separate tagged builds) — Phase 3/4.
- Typical gas limit + buffer for deposit transactions; 1559 support depth — Phase 2.
- Windows Ledger CGO feasibility (document as secondary) — Phase 3.
- Exact release tooling and how tagged vs untagged binaries are published — Phase 4.

## Decision Log
- **Vertical slices (polish + tests + validation owned per feature phase)** — chosen per user direction to ensure each major capability (build path, sign path) is production-ready before moving on, rather than a big end-phase cleanup.
- **Heavy reuse of eth-deposit-gen skeleton** — chosen over green-field CLI framework to compress Phase 1 and maintain identical UX/quality bar across sibling tools (rationale: proven velocity and patterns).
- **Two-phase build/sign with portable interchange** — enforced for air-gap requirement and testability (rationale: matches PRD user stories and research).
- **Ledger primary + privkey fallback with build tags** — per architecture and research (CGO isolation, security model); privkey never the default happy path.
- **Hoodi + existing golden/e2e harness as primary validation** — maximizes reuse of sibling investment.
- **No broadcast in P0** — keeps air-gap pure and scope tight; optional later.
- **Env-var only for private key (P0)** — maximizes security; documented warnings everywhere.
- **Single-stream team** — plan sized accordingly; Ledger spike parallelized where safe.

All estimates assume the high reuse level described; any deviation in interchange design or Ledger SDK friction would be surfaced immediately in the Phase 3 spike.

## Phase Summary Table

| Phase | Focus | Est. Days | Key Deliverable | Exit Criteria |
|-------|-------|-----------|-----------------|---------------|
| 1 | CLI skeleton + basic build + tests + docs + interchange ADR | 2-3 | Working build emitting RLP + stable draft interchange + tests + docs | CLI + basic RLP + golden tests + ADR |
| 2 | Full Tx Builder + interchange + RPC + polish + tests + validation | 3-4 | Production `build` (online + air-gap) | Correct RLP + stable interchange + RPC/offline + full tests + validated |
| 3 | Ledger spike + Signer + sign command + polish + tests + hardware validation | 4-6 | Production `sign` (Ledger primary + fallback) | Real Ledger success + zeroization + gated tests + full air-gap validated |
| 4 | End-to-end integration + release prep + handover | 2-4 | Ship-ready tool | Full e2e on Hoodi + real Ledger + CI/release + final docs |

**Total single-stream estimate: ~11-17 days** (most likely 12-14 with experienced developer who knows the monorepo).

---

*Project Plan updated (2026-05-17) — vertical slices with polish/tests/validation owned per feature phase per user request.*