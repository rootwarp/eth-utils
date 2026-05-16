# Phase 3 — M3: P1 Polish Issues

Sprint-ready issues for Phase 3 of `eth-deposit-gen`. Phase 3 layers
operator-quality-of-life P1 features (parallelism, dry-run, logging,
optional cross-check, progress indicator) on top of the already-correct
Phase-2 core. Issues 15–19 map 1:1 to project-plan.md tasks 3.1–3.5.

Story-point legend: **1 = 0.5d · 2 = 1d · 3 = 1.5d · 5 = 2d**.

---

## Issue #15 — Parallel signing worker pool

**Story points:** 3
**Labels:** `core`, `crypto`, `phase-3`

### Description
Add bounded goroutine parallelism inside `deposit.Generator.Generate`
without breaking output determinism. Per PRD NFR, throughput must be
≥ 200 entries/sec on a modern laptop.

### Acceptance criteria
- [ ] `internal/cli` exposes `--parallel N` (int, default `1`). `N <= 0` is
      rejected with a usage error. `N > runtime.NumCPU()*4` is rejected as
      a sanity guard with a clear error message.
- [ ] `deposit.Request` gains a `Parallel int` field (or equivalent
      orchestration knob) so the orchestrator API is the one consulted, not
      a global.
- [ ] `deposit.Generator.Generate` implements a bounded worker pool of size
      `Parallel`; pubkeys are dispatched to workers, results are collected
      into an indexed slice keyed by input position so the returned
      `[]Entry` order exactly matches `req.Pubkeys` order, regardless of
      `Parallel`.
- [ ] Self-verification still runs per entry; the first verify failure (or
      `ErrPubkeyMismatch`) cancels the shared context and the function
      returns `nil` slice + the original error.
- [ ] A `go test -race` run of `deposit` tests is green.
- [ ] A unit test runs `Generate` with the same 8 pubkeys at `Parallel=1`,
      `4`, and `8` and asserts the three result slices are byte-for-byte
      equal entry-for-entry.
- [ ] The Hoodi and mainnet golden tests (#10, #12) are re-run at
      `Parallel=4` and remain green.
- [ ] A benchmark `BenchmarkGenerate_Parallel` is added; the benchmark
      report (committed at `docs/validation/perf-benchmarks.md`) documents
      sustained throughput ≥ 200 entries/sec on at least one of
      `linux/amd64` or `darwin/arm64`.

### Dependencies
#6.

---

## Issue #16 — `--dry-run` mode

**Story points:** 1
**Labels:** `cli`, `phase-3`

### Description
Print what *would* be produced without writing files to disk. The dry-run
writer already exists from Phase 1 (#7); this issue exposes it on the CLI.

### Acceptance criteria
- [ ] `internal/cli` exposes `--dry-run` (bool, default `false`).
- [ ] When `--dry-run` is set, `cmd/eth-deposit-gen` substitutes
      `output.NewDryRunWriter(os.Stdout)` for `output.NewFSWriter()`; no
      file is created in `--output-dir`.
- [ ] The final stderr summary line is still printed and the reported
      `sha256` matches the sha256 of the bytes written to stdout.
- [ ] Self-verification still runs in `--dry-run` mode; any verify failure
      still aborts with the same exit codes as the non-dry-run path.
- [ ] An integration test asserts: with `--dry-run`, stdout contains valid
      JSON matching the Phase-1 golden expectation, and the `--output-dir`
      remains empty after the run.

### Dependencies
#7, #8.

---

## Issue #17 — Structured logging with `log/slog`

**Story points:** 2
**Labels:** `cli`, `infra`, `phase-3`

### Description
Configure `log/slog` in `main`, expose verbosity and JSON-handler toggles,
and prove via an automated test that no log line ever contains a secret.

### Acceptance criteria
- [ ] `internal/cli` exposes `--verbose` (bool, default `false`) and
      `--json-logs` (bool, default `false`).
- [ ] `main` configures the global `slog` logger: handler is
      `slog.NewTextHandler` by default, `slog.NewJSONHandler` when
      `--json-logs` is set; level is `slog.LevelInfo` by default,
      `slog.LevelDebug` when `--verbose` is set. Output is `os.Stderr`.
- [ ] The logger is threaded via `context.Context` to any package that
      logs. The signing path (`internal/ssz`, `internal/bls`,
      `internal/deposit`) **does not log anything** — assert this with a
      package-import lint (`grep -R "slog\." internal/ssz internal/bls
      internal/deposit` returns no matches).
- [ ] Logging occurs at orchestration boundaries only: keystore load
      start/end, generator start/end, output written.
- [ ] Pubkeys and file paths are loggable; passphrase, decrypted secret,
      keystore JSON body, signing roots, and signatures are not.
- [ ] A "secret-leak" test runs the full pipeline against a fixture whose
      passphrase is the unique sentinel string `THIS-IS-A-TEST-PASSPHRASE`
      and whose decrypted key has a known unique byte pattern; it captures
      all logger output (info + debug + json) and asserts neither sentinel
      appears anywhere in the captured bytes.
- [ ] Help text documents both flags.

### Dependencies
#9.

---

## Issue #18 — `--verify-with-deposit-cli` cross-check

**Story points:** 2
**Labels:** `cli`, `test`, `phase-3`

### Description
Optional post-generation cross-check that shells out to the user's
installed `staking-deposit-cli` and asks it to verify the produced JSON.
Off by default; opt-in via flag.

### Acceptance criteria
- [ ] `internal/cli` exposes `--verify-with-deposit-cli` (bool, default
      `false`) and `--deposit-cli-path` (string, default `"deposit"` —
      i.e. resolve via `$PATH`).
- [ ] When the flag is set, after a successful write, the app runs
      `<deposit-cli-path> existing-mnemonic --verify <output-file>` (or the
      equivalent verify subcommand documented in the latest
      `staking-deposit-cli` release; the exact invocation is decided in
      this issue and recorded in code comments + README).
- [ ] If `staking-deposit-cli` is not installed (`exec.LookPath` fails)
      **and** the flag is set: exit code 2 with a clear error naming the
      expected binary.
- [ ] If `staking-deposit-cli` is not installed **and** the flag is not
      set: no warning, no error.
- [ ] If the external CLI exits non-zero, the app exits with code 3 and
      includes the external CLI's stderr in the error message.
- [ ] The external invocation is wrapped behind an `exec.Cmd` builder
      interface so unit tests can inject a stub. Tests cover: missing
      binary, success path, failure path (external exits 1), stdout/stderr
      capture.
- [ ] README notes the minimum `staking-deposit-cli` version expected.

### Dependencies
#9.

---

## Issue #19 — Progress indicator for >5 entries

**Story points:** 1
**Labels:** `cli`, `phase-3`

### Description
A lightweight progress indicator so operators with large batches see the
tool is making forward progress. No heavy TUI library.

### Acceptance criteria
- [ ] When `len(req.Pubkeys) > 5` **and** stderr is a TTY **and**
      `--json-logs` is not set, the app prints a single updating line of
      the form `signing: <i>/<n> (<pct>%)` to stderr.
- [ ] When stderr is not a TTY (e.g. piped, CI), the indicator emits one
      `slog.Info` event per 10% of progress instead — never a partial-line
      `\r`-overwrite that would corrupt log capture.
- [ ] When `--json-logs` is set, the indicator emits structured
      `slog.Info` events with attributes `done` (int) and `total` (int)
      instead of any plain text.
- [ ] The indicator is suppressed entirely when `len(req.Pubkeys) <= 5`.
- [ ] A unit test using a piped `*os.File` (non-TTY) confirms no
      `\r`-overwrite bytes appear in captured stderr.
- [ ] Golden tests (#10, #12) remain green (i.e. the indicator never writes
      to stdout and never bleeds into the JSON output).

### Dependencies
#6, #17.
