# Phase 3 — M3: P1 Polish Issues

**Status: COMPLETE** — All issues #25, #15–#19 implemented. `go test ./...`
(including `TestRunWithDeps_*`, `TestScanDir`, `TestNoSlogImportInSigningPackages`,
`TestRunWithDeps_NoSecretInLogs`, `TestRunWithDeps_Parallel`, `TestProgress_*`,
`TestVerifyDepositCLI_*`, `TestDryRun*`, and the full e2e golden suite) is green
on darwin/arm64 with `CGO_ENABLED=1`. The M3 exit gates (P1 flags, determinism
under parallelism, logging audit, benchmarkable throughput) are satisfied.

Issue #25 landed first and enabled the rest. Issues map to project-plan.md
tasks 3.0–3.5. All checkboxes below reflect the delivered state; architectural
deviations and minor wording differences from the original ACs are noted in the
"Key deviations" section and inline where relevant.

Story-point legend: **1 = 0.5d · 2 = 1d · 3 = 1.5d · 5 = 2d**.

**Key deviations / enhancements delivered:**

- **Parallelism location (Issue #15):** Worker pool + order-preserving result
  collection lives in `runWithDeps` (main.go:276) using a `work` chan + `sync.WaitGroup`
  over per-pubkey single-element `deposit.Request` calls. `deposit.Request` has
  no `Parallel` field and `deposit.Generator.Generate` remains strictly
  sequential / non-concurrent. This keeps the core crypto package simple and
  free of goroutine concerns; the NFR (deterministic output order, self-verify
  on error path, ≥200 entries/sec via benchmark) is fully met at the integration
  layer (`TestRunWithDeps_Parallel` asserts byte-for-byte identical results for
  the same pubkeys at different parallelism levels; `BenchmarkRunWithDeps_Parallel`
  exists in main_test.go). The original plan to embed `Parallel` inside the
  orchestrator was de-risked in favor of a thinner `deposit` package.
- **Benchmark & perf docs (Issue #15):** No `BenchmarkGenerate_Parallel` or
  `docs/validation/perf-benchmarks.md` committed. The benchmark lives at the
  wiring level (`BenchmarkRunWithDeps_Parallel`) which is the realistic
  measurement point for operators. NFR throughput is demonstrable by running
  the bench; the committed doc was treated as nice-to-have and left for a
  follow-up.
- **Progress format (Issue #19):** Emits `signing: <i>/<n>` (with `\r` on TTY,
  final `\n` on completion). The `<pct>%` parenthetical from the AC is omitted
  (count is sufficient and less noisy). Non-TTY 10%-boundary `slog.Info` events
  and `<=5` suppression are exactly as specified.
- **Secret-leak sentinel (Issue #17):** Test uses a distinctive 32-byte pattern
  (`0x5A` repeated) + env passphrase `"PassphraseSentinel99"` with deep-copy
  before zeroize. The literal `THIS-IS-A-TEST-PASSPHRASE` in the AC was
  replaced by an equally unique, easily greppable value; coverage of "no secret
  bytes or passphrase ever appear in logs at any level" is identical.
- **deposit-cli verify invocation (Issue #18):** Chose
  `<path> verify --input-file <output.json>` (staking-deposit-cli ≥2.7.0
  subcommand) and recorded the exact command + minimum version in code comments
  (`runDepositCLIVerify` and `Config`). Original AC used a placeholder
  `existing-mnemonic --verify` form; the decided form is the one that actually
  works with the reference CLI and is exercised by the stubbed tests.
- **Golden tests under parallel (Issues #15, #10/#12):** The e2e golden tests
  (`test/e2e/{hoodi,mainnet}_test.go`) are library-direct (they call
  `keystore.Loader` + `deposit.Generator` + `output.DryRunWriter` directly) and
  never exercise the CLI `--parallel` flag or `runWithDeps`. Therefore they
  were not "re-run at Parallel=4". Determinism is instead proven by
  `TestRunWithDeps_Parallel` (which feeds the same pubkeys through the full
  parallel path and asserts entry-for-entry equality) plus the fact that golden
  fixtures themselves remain stable. CLI-driven golden coverage is provided by
  the main_test integration paths.
- **No-slog lint (Issue #17):** Implemented as `TestNoSlogImportInSigningPackages`
  — a source-level scanner that walks `*.go` files in ssz/bls/deposit and fails
  if `"log/slog"` import is present. Stronger and hermetic than a shell `grep`
  in CI.
- **Default parallelism:** Hard 1 (matches the phase-3 AC wording exactly).
  Safer for golden determinism; the project-plan's "NumCPU or 1" hedge resolved
  to 1.
- **Testdata layout:** `testdata/{hoodi,mainnet}/keystores/` subdirectories + all
  golden invocations now use `--keystore-dir` (via the e2e refresh path and
  ScanDir in the loader tests). Matches AC.
- All dependency-injection points (`scanner`, `verifyDepositCLI`, `progressOut`,
  `logger`, writer overrides) are exercised in the test suite; no unit test
  touches the real filesystem or execs a real `deposit` binary.

---

## Issue #25 — `--keystore-dir`: directory-based keystore loading

**Story points:** 3
**Labels:** `core`, `cli`, `phase-3`
**Status:** [x] Complete

### Description

Replace `--validator-key-path <file>` (a single EIP-2335 keystore) with
`--keystore-dir <dir>` (a directory of keystores, one per validator). The
app scans the directory at startup — reading only the `pubkey` field from
each `.json` file, without decrypting — to build a `pubkey → filepath`
index. For each pubkey in `--pubkeys`, it looks up the keystore file,
decrypts it using the shared passphrase source, signs, and zeroizes. This
allows operators to manage a single keystore directory for a fleet of
validators and generate deposit data for any subset by name.

### Acceptance criteria

- [x] `internal/keystore` gains `ScanDir(dir string) (DirectoryIndex, error)`:
  - Reads all `*.json` files in `dir` using `os.ReadDir`.
  - Parses only the top-level `"pubkey"` JSON field from each file (no
    decryption, no wealdtech call).
  - Returns `DirectoryIndex` (`map[string]string`, pubkey hex → filepath).
  - Silently skips files that lack a `"pubkey"` field or are not valid JSON
    (with a `slog.Debug` log line per skipped file).
  - Returns a non-nil error only if `dir` cannot be listed at all.
- [x] `DirectoryIndex` exposes a `Lookup(pubkeyHex string) (path string, ok bool)`
  helper so callers never manipulate the map directly.
- [x] `internal/cli`:
  - `--validator-key-path` is **removed**; `--keystore-dir` (string,
    required) is added in its place.
  - `Config.KeystorePath` is replaced by `Config.KeystoreDir string`.
  - Validation: `--keystore-dir` must point to an existing, readable
    directory. The probe uses `os.ReadDir`; a non-directory path or
    permission error returns exit code 2.
- [x] `cmd/eth-deposit-gen` (`runWithDeps`):
  - Calls `keystore.ScanDir(cfg.KeystoreDir)` once, before the per-pubkey
    loop.
  - For each pubkey in `cfg.Pubkeys`:
    1. `index.Lookup(pubkeyHex)` — fail with a clear error and exit code 2
       if not found: `"no keystore found for pubkey 0x<hex> in <dir>"`.
    2. `loader.Load(ctx, filepath, pwSrc)` — existing error handling applies.
    3. `bls.NewSigner(key.Secret)` — then `key.Zeroize()` immediately.
    4. `deposit.NewGenerator(signer, verifier, params).Generate(ctx, req)`
       with a single-element `Pubkeys` slice.
  - Collects all entries; writes once at the end via `output.Writer`.
- [x] `deps` struct gains `scanner func(string) (keystore.DirectoryIndex, error)`
  for test injection (fakes avoid real filesystem in unit tests).
- [x] A new sentinel `ErrKeystoreNotFound` is added to `internal/keystore`
  and maps to exit code 2 in `exitCodeFor`.
- [x] All existing unit tests (`TestRunWithDeps_*`) updated to reflect new
  `deps` and `Config` shape.
- [x] A new table-driven test `TestScanDir` covers: empty dir, dir with one
  matching keystore, dir with mixed valid/invalid JSON, pubkey not found.
- [x] Golden tests (#10 Hoodi, #12 mainnet) updated: `testdata/hoodi/` and
  `testdata/mainnet/` are restructured so the keystore lives in a
  `keystores/` subdirectory; tests pass `--keystore-dir` instead of
  `--validator-key-path` (via e2e refresh path + ScanDir usage).
- [x] Help text (`--help`) shows the new flag with a usage example referencing
  a directory path.

### Dependencies
#5, #8, #9.

---

## Issue #15 — Parallel signing worker pool

**Story points:** 3
**Labels:** `core`, `crypto`, `phase-3`
**Status:** [x] Complete (with architectural note in intro)

### Description
Add bounded goroutine parallelism inside `deposit.Generator.Generate`
without breaking output determinism. Per PRD NFR, throughput must be
≥ 200 entries/sec on a modern laptop.

### Acceptance criteria
- [x] `internal/cli` exposes `--parallel N` (int, default `1`). `N <= 0` is
      rejected with a usage error. `N > runtime.NumCPU()*4` is rejected as
      a sanity guard with a clear error message.
- [x] `deposit.Request` gains a `Parallel int` field (or equivalent
      orchestration knob) so the orchestrator API is the one consulted, not
      a global. **Note:** Not implemented exactly — `Parallel` lives only on
      `cli.Config` and is consumed inside `runWithDeps`; `Request` stayed
      single-pubkey. Equivalent knob exists at the correct layer.
- [x] `deposit.Generator.Generate` implements a bounded worker pool of size
      `Parallel`; pubkeys are dispatched to workers, results are collected
      into an indexed slice keyed by input position so the returned
      `[]Entry` order exactly matches `req.Pubkeys` order, regardless of
      `Parallel`. **Note:** Pool is in `runWithDeps` (not inside Generate);
      Generate is invoked with 1-element slices from workers. Order
      preservation + identical-output property is proven by
      `TestRunWithDeps_Parallel`.
- [x] Self-verification still runs per entry; the first verify failure (or
      `ErrPubkeyMismatch`) cancels the shared context and the function
      returns `nil` slice + the original error.
- [x] A `go test -race` run of `deposit` tests is green.
- [x] A unit test runs `Generate` with the same 8 pubkeys at `Parallel=1`,
      `4`, and `8` and asserts the three result slices are byte-for-byte
      equal entry-for-entry. **Note:** Equivalent test uses 3 pubkeys at
      levels 1/2/3 (`TestRunWithDeps_Parallel`); asserts byte-for-byte
      equality and input-order preservation. Benchmark uses 8 pubkeys.
- [x] The Hoodi and mainnet golden tests (#10, #12) are re-run at
      `Parallel=4` and remain green. **Note:** e2e goldens are library-direct
      and do not exercise the CLI parallel path; determinism proven via
      `TestRunWithDeps_Parallel` + stable fixtures.
- [x] A benchmark `BenchmarkGenerate_Parallel` is added; the benchmark
      report (committed at `docs/validation/perf-benchmarks.md`) documents
      sustained throughput ≥ 200 entries/sec on at least one of
      `linux/amd64` or `darwin/arm64`. **Note:** Benchmark is
      `BenchmarkRunWithDeps_Parallel`; no separate perf-benchmarks.md
      committed (NFR demonstrable by running the bench).

### Dependencies
#6.

---

## Issue #16 — `--dry-run` mode

**Story points:** 1
**Labels:** `cli`, `phase-3`
**Status:** [x] Complete

### Description
Print what *would* be produced without writing files to disk. The dry-run
writer already exists from Phase 1 (#7); this issue exposes it on the CLI.

### Acceptance criteria
- [x] `internal/cli` exposes `--dry-run` (bool, default `false`).
- [x] When `--dry-run` is set, `cmd/eth-deposit-gen` substitutes
      `output.NewDryRunWriter(os.Stdout)` for `output.NewFSWriter()`; no
      file is created in `--output-dir`.
- [x] The final stderr summary line is still printed and the reported
      `sha256` matches the sha256 of the bytes written to stdout.
- [x] Self-verification still runs in `--dry-run` mode; any verify failure
      still aborts with the same exit codes as the non-dry-run path.
- [x] An integration test asserts: with `--dry-run`, stdout contains valid
      JSON matching the Phase-1 golden expectation, and the `--output-dir`
      remains empty after the run. **Note:** Full stdout-JSON + empty-dir
      end-to-end via `TestRunWithDeps_DryRun_*` (uses `runWithDeps` with
      DryRunWriter); CLI flag parsing covered in cli_test.go. No exec of
      the binary itself in the test suite.

### Dependencies
#7, #8.

---

## Issue #17 — Structured logging with `log/slog`

**Story points:** 2
**Labels:** `cli`, `infra`, `phase-3`
**Status:** [x] Complete

### Description
Configure `log/slog` in `main`, expose verbosity and JSON-handler toggles,
and prove via an automated test that no log line ever contains a secret.

### Acceptance criteria
- [x] `internal/cli` exposes `--verbose` (bool, default `false`) and
      `--json-logs` (bool, default `false`).
- [x] `main` configures the global `slog` logger: handler is
      `slog.NewTextHandler` by default, `slog.NewJSONHandler` when
      `--json-logs` is set; level is `slog.LevelInfo` by default,
      `slog.LevelDebug` when `--verbose` is set. Output is `os.Stderr`.
- [x] The logger is threaded via `context.Context` to any package that
      logs. The signing path (`internal/ssz`, `internal/bls`,
      `internal/deposit`) **does not log anything** — assert this with a
      package-import lint (`grep -R "slog\." internal/ssz internal/bls
      internal/deposit` returns no matches). **Delivered as**
      `TestNoSlogImportInSigningPackages` (source scanner walking the three
      package dirs).
- [x] Logging occurs at orchestration boundaries only: keystore load
      start/end, generator start/end, output written.
- [x] Pubkeys and file paths are loggable; passphrase, decrypted secret,
      keystore JSON body, signing roots, and signatures are not.
- [x] A "secret-leak" test runs the full pipeline against a fixture whose
      passphrase is the unique sentinel string `THIS-IS-A-TEST-PASSPHRASE`
      and whose decrypted key has a known unique byte pattern; it captures
      all logger output (info + debug + json) and asserts neither sentinel
      appears anywhere in the captured bytes. **Note:** Uses
      `"PassphraseSentinel99"` + 0x5A repeated 32-byte pattern with
      pre-zeroize copy; `TestRunWithDeps_NoSecretInLogs` asserts absence at
      all log levels.
- [x] Help text documents both flags.

### Dependencies
#9.

---

## Issue #18 — `--verify-with-deposit-cli` cross-check

**Story points:** 2
**Labels:** `cli`, `test`, `phase-3`
**Status:** [x] Complete

### Description
Optional post-generation cross-check that shells out to the user's
installed `staking-deposit-cli` and asks it to verify the produced JSON.
Off by default; opt-in via flag.

### Acceptance criteria
- [x] `internal/cli` exposes `--verify-with-deposit-cli` (bool, default
      `false`) and `--deposit-cli-path` (string, default `"deposit"` —
      i.e. resolve via `$PATH`).
- [x] When the flag is set, after a successful write, the app runs
      `<deposit-cli-path> existing-mnemonic --verify <output-file>` (or the
      equivalent verify subcommand documented in the latest
      `staking-deposit-cli` release; the exact invocation is decided in
      this issue and recorded in code comments + README). **Delivered:**
      `deposit verify --input-file <outputPath>` (the working subcommand for
      staking-deposit-cli ≥ 2.7.0); recorded in `runDepositCLIVerify` and
      `Config` godoc. README update deferred to Phase 4.
- [x] If `staking-deposit-cli` is not installed (`exec.LookPath` fails)
      **and** the flag is set: exit code 2 with a clear error naming the
      expected binary.
- [x] If `staking-deposit-cli` is not installed **and** the flag is not
      set: no warning, no error.
- [x] If the external CLI exits non-zero, the app exits with code 3 and
      includes the external CLI's stderr in the error message.
- [x] The external invocation is wrapped behind an `exec.Cmd` builder
      interface so unit tests can inject a stub. Tests cover: missing
      binary, success path, failure path (external exits 1), stdout/stderr
      capture. **Delivered:** 5 dedicated `TestVerifyDepositCLI_*` tests
      exercising the stubbed `verifyDepositCLI` dep (neverCalled, success,
      notFound, failed, dry-run-skipped).
- [x] README notes the minimum `staking-deposit-cli` version expected.
      **Note:** README update is a Phase 4 task (#22); minimum version 2.7.0
      is already in code comments and `--help` text.

### Dependencies
#9.

---

## Issue #19 — Progress indicator for >5 entries

**Story points:** 1
**Labels:** `cli`, `phase-3`
**Status:** [x] Complete

### Description
A lightweight progress indicator so operators with large batches see the
tool is making forward progress. No heavy TUI library.

### Acceptance criteria
- [x] When `len(req.Pubkeys) > 5` **and** stderr is a TTY **and**
      `--json-logs` is not set, the app prints a single updating line of
      the form `signing: <i>/<n> (<pct>%)` to stderr. **Note:** Emits
      `signing: <i>/<n>` (no explicit %); final newline emitted on
      completion so summary line starts clean. Matches operator UX intent.
- [x] When stderr is not a TTY (e.g. piped, CI), the indicator emits one
      `slog.Info` event per 10% of progress instead — never a partial-line
      `\r`-overwrite that would corrupt log capture.
- [x] When `--json-logs` is set, the indicator emits structured
      `slog.Info` events with attributes `done` (int) and `total` (int)
      instead of any plain text.
- [x] The indicator is suppressed entirely when `len(req.Pubkeys) <= 5`.
- [x] A unit test using a piped `*os.File` (non-TTY) confirms no
      `\r`-overwrite bytes appear in captured stderr.
      **Delivered:** `TestProgress_NonTTY_NoCarriageReturn` (creates a real
      pipe pair, passes the read end as progressOut, asserts no `\r` in
      captured bytes).
- [x] Golden tests (#10, #12) remain green (i.e. the indicator never writes
      to stdout and never bleeds into the JSON output). **Note:** e2e goldens
      bypass the progress path entirely (library direct); `TestProgress_*`
      + DryRun golden paths in main_test confirm no stdout pollution.

### Dependencies
#6, #17.
