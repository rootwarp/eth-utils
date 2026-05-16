# Issue Summary — `eth-deposit-gen`

All sprint-ready issues across Phases 1–4, with story-point totals and the
critical path called out. Story-point legend: **1 = 0.5d · 2 = 1d · 3 = 1.5d
· 5 = 2d**. Issues marked with **★** are on the critical path: the shortest
sequence that gates v1.0 release.

## All issues

| # | Phase | Title | Points | Labels | Depends On |
|---|-------|-------|--------|--------|------------|
| 1 | 1 | Bootstrap Go module and workspace for `eth-deposit-gen` | 1 | infra | — |
| 2 | 1 | Implement `internal/network` package (Hoodi-only) | 2 | core | 1 |
| 3 ★ | 1 | Implement hand-rolled `internal/ssz` hash_tree_root | 5 | core, crypto | 1 |
| 4 ★ | 1 | Implement `internal/bls` herumi wrapper | 3 | core, crypto | 1 |
| 5 | 1 | Implement `internal/keystore` EIP-2335 loader | 3 | core, crypto | 1 |
| 6 ★ | 1 | Implement `internal/deposit` orchestrator | 5 | core, crypto | 2, 3, 4 |
| 7 | 1 | Implement `internal/output` JSON writer | 2 | core, cli | 6 |
| 8 | 1 | Implement `internal/cli` flag parsing | 2 | cli | 2 |
| 9 | 1 | Wire `cmd/eth-deposit-gen` main and exit codes | 1 | cli | 5, 6, 7, 8 |
| 10 ★ | 1 | Hoodi golden-file integration test (M1 exit gate) | 5 | test | 9 |
| 11 | 2 | Add mainnet constants in `internal/network` | 1 | core | 2 |
| 12 ★ | 2 | Mainnet golden-file fixture and integration test (M2 exit gate) | 2 | test | 11, 10 |
| 13 | 2 | On-chain Hoodi deposit validation | 2 | test, infra | 11 |
| 14 | 2 | Network safety banner hardening for mainnet | 1 | cli | 11, 8 |
| 25 ★ | 3 | `--keystore-dir`: directory-based keystore loading | 3 | core, cli | 5, 8, 9 |
| 15 | 3 | Parallel signing worker pool | 3 | core, crypto | 6, 25 |
| 16 | 3 | `--dry-run` mode | 1 | cli | 7, 25 |
| 17 | 3 | Structured logging with `log/slog` | 2 | cli, infra | 9, 25 |
| 18 | 3 | `--verify-with-deposit-cli` cross-check | 2 | cli, test | 9, 25 |
| 19 | 3 | Progress indicator for >5 entries | 1 | cli | 6, 17 |
| 20 | 4 | GoReleaser configuration | 2 | infra | Phase 3 |
| 21 | 4 | Release CI pipeline | 2 | infra | 20 |
| 22 | 4 | README and usage documentation | 2 | docs | 15, 16, 17, 18, 19 |
| 23 | 4 | Internal audit pass on signing-critical code | 2 | crypto, security | 20, 21, 22 |
| 24 | 4 | Tag v1.0.0 and verify release artifacts | 1 | infra | 23 |

★ = on the critical path.

## Story-point totals

| Phase | Issues | Total points | Approx. dev-days |
|-------|--------|--------------|------------------|
| Phase 1 — M1 Core (Hoodi) | #1–#10 | **29** | ~14.5d (matches project-plan 8–11d once parallelizable streams overlap) |
| Phase 2 — M2 Mainnet | #11–#14 | **6** | ~3d |
| Phase 3 — M3 P1 polish | #25, #15–#19 | **12** | ~6d |
| Phase 4 — M4 Release | #20–#24 | **9** | ~4.5d |
| **Total** | **25 issues** | **56 points** | **~28 dev-days serial** |

The serial-day total is intentionally higher than the project-plan's 16–22
dev-day estimate because that estimate assumes the Phase-1 parallel work
streams (Streams A/B/C/D in project-plan.md) overlap. Two developers
splitting #3/#4 from #5/#7 inside Phase 1 compresses the wall-clock close to
the plan's ~3-week lower bound.

## Critical path (★)

```
#1  bootstrap
  └─▶ #2  network        ──┐
      #3  ssz             ──┤
      #4  bls             ──┤
                            └─▶ #6  deposit ──▶ #7 output ──▶ #8 cli ──▶ #9 main
                                                                              │
                                                                              ▼
                                                                       #10 Hoodi golden ★ (M1 gate)
                                                                              │
                                                                              ▼
                                                            #11 mainnet const ──▶ #12 mainnet golden ★ (M2 gate)
                                                                              │
                                                                              ▼
                                                                       Phase 3 polish
                                                                              │
                                                                              ▼
                                                                       Phase 4 release
```

The two golden-file checks — **#10 (Hoodi)** and **#12 (mainnet)** — are
the only correctness gates between this tool and the "lose 32 ETH" failure
mode. Everything else can slip; those two cannot. Issues **#3 (SSZ)** and
**#6 (orchestrator)** carry the most correctness risk on the critical path
and should receive disproportionate review attention.

## Parallel work streams (Phase 1)

After #1 and #2 land, the following streams can run concurrently:

- **Stream A — Crypto core:** #3 (ssz) → contributes to #6.
- **Stream B — Crypto wrapper:** #4 (bls) → contributes to #6.
- **Stream C — I/O edges:** #5 (keystore) and #7 (output) — independent.
- **Stream D — CLI surface:** #8 (cli) once #2 stabilizes.

All four converge at #6 → #9 → #10 (M1 gate).

Within Phase 3, #15 (parallel), #17 (logging), and #18 (verify cross-check)
are mutually independent and can be parallelized.

## Label index

| Label | Issues |
|-------|--------|
| `infra` | 1, 13, 20, 21, 24 |
| `core` | 2, 3, 4, 5, 6, 7, 11, 15, 25 |
| `crypto` | 3, 4, 5, 6, 15, 23 |
| `cli` | 7, 8, 9, 14, 16, 17, 18, 19, 22, 25 |
| `test` | 10, 12, 13, 18 |
| `docs` | 22 |
| `security` | 23 |
| `phase-1` | 1–10 |
| `phase-2` | 11–14 |
| `phase-3` | 15–19, 25 |
| `phase-4` | 20–24 |
| `critical-path` | 3, 4, 6, 10, 12, 25 |
