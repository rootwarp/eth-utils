# Issue Estimates: eth-deposit-tx

## Estimation Approach
- Point scale: 1 (trivial, <0.5 day) / 2 (small, 0.5–1 day) / 3 (medium, 1–2 days) / 5 (large, 2+ days — must be justified or split)
- Target: Every issue completable in 1–2 days by a single code-writer (including coding, unit tests, review, and integration into the vertical slice).
- Points include all work: implementation, tests (unit + the phase’s integration/validation), docs updates, and polish.
- Execution model: Single-stream by default (one code-writer working issues in dependency order). No multi-stream plan requested.
- Vertical-slice principle (from approved plan): Each phase delivers a working, testable increment that includes its own polish, tests, and validation. Issues within a phase are ordered to enable early integration.

## Phase Files

| File | Phase | Issue Count | Total Points | Est. Duration (single-stream) |
|------|-------|-------------|--------------|-------------------------------|
| [phase-1-foundation.md](./phase-1-foundation.md) | Phase 1: Foundation (CLI skeleton + stub builder + config) | 6 | 11 | ~7–8 days |
| [phase-2-tx-builder.md](./phase-2-tx-builder.md) | Phase 2: Tx Builder (real construction + ABI + fees) | 5 | 12 | ~8–9 days |
| [phase-3-signer.md](./phase-3-signer.md) | Phase 3: Signer (local + Ledger + CGO) | 6 | 14 | ~10–11 days (highest risk) |
| [phase-4-integration.md](./phase-4-integration.md) | Phase 4: Integration (end-to-end, broadcast, docs, release, validation) | 6 | 13 | ~9–10 days |
| **Total** | | **23** | **50** | **~34–38 days** |

## Execution Plan (Single-Stream, Sequential)

| Day Range | Phase | Focus | Notes |
|-----------|-------|-------|-------|
| 1–8 | Phase 1 | CLI skeleton, types, config, stub “build”, exit codes, basic tests | Establishes command structure mirroring `eth-deposit-gen` |
| 9–17 | Phase 2 | Real `internal/tx/builder.go`, ABI, validation, gas/EIP-1559, replace stub | Delivers correct unsigned deposit tx |
| 18–28 | Phase 3 | Signer interface, local signer, Ledger CGO + transport, signing flow, cmd integration | Critical path; includes hardware testing |
| 29–38 | Phase 4 | Full pipeline wiring, optional broadcast, E2E validation on testnet, docs, Makefile/release, security checklist | Final slice + release readiness |

## Dependency Map (High-Level)

```
Phase 1 (Foundation)
  ├── 1.1 CLI scaffold (main, cobra, version, help)
  ├── 1.2 Core types (DepositRequest, NetworkConfig, etc.)
  ├── 1.3 Config loader (flags + env + defaults for networks/contracts)
  ├── 1.4 Stub build command (unsigned tx JSON output)
  ├── 1.5 Exit codes + error handling + logging
  └── 1.6 Phase-1 tests + README stub + help text

Phase 2 (Tx Builder) — depends on Phase 1 types/config
  ├── 2.1 internal/tx package + TxBuilder interface + builder.go skeleton
  ├── 2.2 Real ABI encoding for deposit() + calldata construction
  ├── 2.3 Input validation (BLS pubkey 48B, withdrawal creds, sig 96B, root 32B)
  ├── 2.4 Gas, EIP-1559 fees, nonce handling, chain ID
  └── 2.5 Unit tests + validation; integrate real builder into “build” command

Phase 3 (Signer) — depends on Phase 2 (needs real unsigned tx)
  ├── 3.1 Signer interface (internal/signer)
  ├── 3.2 Local private-key signer (for dev/test)
  ├── 3.3 Ledger signer skeleton (CGO, hid, build tags, discovery)
  ├── 3.4 Ledger signing flow (EIP-1559, device confirmation, signature application)
  ├── 3.5 cmd/eth-deposit-tx integration (sign subcommand or --signer flag)
  └── 3.6 Tests (mocks + build-tag guarded Ledger tests) + security hardening

Phase 4 (Integration) — depends on Phase 3 complete signer
  ├── 4.1 End-to-end pipeline (build → sign → signed tx output)
  ├── 4.2 Optional broadcast (“send”) with RPC + confirmation
  ├── 4.3 E2E validation on Holesky/Sepolia + sample artifacts
  ├── 4.4 Full UX polish, examples, consistent exit codes, error messages
  ├── 4.5 Documentation (README, security considerations, usage) to PRD bar
  └── 4.6 Makefile, CI, release process (mirror eth-deposit-gen), final security checklist
```

## Risk Flags
- **Ledger / CGO (Phase 3)**: Highest uncertainty. Research already completed (tx-construction-and-ledger.md). Issues 3.3–3.4 carry the risk of USBHID quirks, cross-compile pain, and device-specific behavior. Mitigations: build tags, mock-first development, early hardware smoke tests, clear “unsupported on this platform” graceful degradation.
- **ABI / deposit contract edge cases (Phase 2)**: BLS key formatting, withdrawal credential variants (0x00 vs 0x01), deposit_data_root mismatches. Mitigated by table-driven tests against known-good deposit data from eth-deposit-gen.
- **Security / secret handling (all phases)**: PRD requires explicit rules (no private key logging, redacted errors, etc.). Phase 4 has a dedicated security checklist issue.
- **Testnet access for validation (Phase 4)**: Requires Holesky/Sepolia RPC + funded test ETH. Issue 4.3 includes a “test account procurement” step.
- **Documentation bar (PRD)**: Phase 4 must meet the explicit documentation standard; do not defer.

## Recommendations
1. Treat Phase 3 (especially 3.3–3.4) as the critical path — start early smoke-testing on real Ledger Nano once the interface is defined (even before full builder is wired).
2. Keep the “build” command useful at the end of Phase 2 (unsigned tx JSON) so stakeholders can validate calldata before signing work lands.
3. In Phase 4, produce at least one reproducible signed deposit tx on a public testnet and attach it as a golden artifact for regression.
4. After Phase 4, consider a small follow-up for “send with gas-bump / retry” if the initial broadcast is minimal.
5. Review point totals against team velocity in the first planning meeting; the 50-point total is intentionally conservative to account for Ledger and security work.

## Internal Tracking (for estimator)
- Input ingestion: complete (PRD + Research + Architecture + vertical-slice Project Plan + eth-deposit-gen patterns + internal/ package style).
- All issues reference concrete files from the approved architecture (cmd/eth-deposit-tx/..., internal/tx/builder.go, internal/signer/ledger.go, internal/signer/local.go, etc.).
- No multi-stream file-ownership analysis performed (single-stream plan).
- Ready for user review + LGTM gate.

**Gate Question (see end of this document for full prompt):**
Are the estimates reasonable? Reply `LGTM` to proceed to wrap-up, or list issues to split, merge, or reprioritize.