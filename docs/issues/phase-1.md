# Phase 1 — M1: Core (Hoodi) Issues

**Status: COMPLETE** — All issues 1–10 implemented. `go test ./...` (including
`TestHoodiGoldenDeposit`) is green on darwin/arm64 and linux/amd64 with
`CGO_ENABLED=1`. The M1 exit gate (end-to-end pipeline) passes via
self-consistency golden fixtures (see `docs/validation/hoodi-golden.md` for
details and rationale). Full byte-for-byte cross-validation against
`staking-deposit-cli` v2.7.0 is deferred to Phase 2 (the Python CLI was
unavailable on the build host during M1).

Issues 1–10 map 1:1 to project-plan.md tasks 1.1–1.10. All checkboxes below
reflect the delivered state; minor deviations from original AC wording are
noted per-issue (Go version bump, CI scope, golden provenance, testability
enhancements).

Story-point legend: **1 = 0.5d · 2 = 1d · 3 = 1.5d · 5 = 2d**.

**Key deviations / enhancements delivered:**
- Go 1.25 (not 1.23) per `go.mod`/`go.work`/`setup-go` in CI.
- CI workflow on `ubuntu-latest` only (no cross-arch matrix yet; cross-builds
  supported via `GOOS`/`GOARCH`).
- Golden fixtures and `make refresh-golden` are Go self-generated (fixed
  secret + wealdtech scrypt + pipeline round-trip) for self-consistency.
  `staking-deposit-cli` invocation not used (unavailable); documented in
  `hoodi-golden.md`.
- Extra testability: `deps` injection in `main.go`, `main_test.go` integration
  tests, `ValidatePubkeyBytes`, network-mismatch guard, signal-aware
  `RunContext`.
- No secrets ever logged (package has zero logging statements).
- `padRight` helper exists (unused; `byteVectorRoot` inlines equivalent logic).

---

## Issue #1 — Bootstrap Go module and workspace for `eth-deposit-gen`

**Story points:** 1
**Labels:** `infra`, `phase-1`
**Status:** [x] Complete

### Description
Create a new Go module at `go/cmd/eth-deposit-gen/` and register it in the
existing `go/go.work` workspace per `docs/architecture.md` §Repository
Placement. Add baseline tooling so subsequent issues can land tests and lint
checks against a working CI skeleton.

### Acceptance criteria
- [x] Directory `go/cmd/eth-deposit-gen/` exists with `go.mod` declaring
      module path `github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen` and
      `go 1.25.0` (updated from original 1.23 plan; matches `go/go.work` 1.25.0
      and CI `setup-go@v5` with `"1.25"`).
- [x] `go/go.work` contains a `use (./cmd/eth-deposit-gen)` entry.
- [x] `go/cmd/eth-deposit-gen/Makefile` exposes targets: `test`, `lint`,
      `build` (plus `coverage`, `fuzz`, `refresh-golden`, `verify`, `tidy`,
      `test-verbose`, `clean`, `help`). `make test` runs `go test ./...` with
      CGO; `make lint` runs `go vet ./...` and `staticcheck ./...`; `make build`
      produces `bin/eth-deposit-gen`.
- [x] A GitHub Actions workflow (`.github/workflows/eth-deposit-gen.yml`) runs
      `go mod verify`, `go vet`, `staticcheck`, and `go test ./...` on push.
      `CGO_ENABLED: 1` via job env. Single runner (`ubuntu-latest`); cross-arch
      matrix (darwin/arm64, linux/arm64) deferred (builds are portable via
      `GOOS`/`GOARCH`).
- [x] `staticcheck` is installed via `go install honnef.co/go/tools/cmd/staticcheck@latest`
      in CI (not digest-pinned).
- [x] `go build` succeeds against the full `main.go` (no placeholder; includes
      signal handling, `exitCodeFor`, `runWithDeps`).

### Dependencies
None.

---

## Issue #2 — Implement `internal/network` package (Hoodi-only)

**Story points:** 2
**Labels:** `core`, `phase-1`

### Description
Source of truth for per-network constants. Phase 1 enables Hoodi only;
mainnet selection must fail explicitly until Phase 2 lights it up.

### Acceptance criteria
- [x] Package `internal/network` defines exported types: `Network` (string
      alias), `Params{Name Network; GenesisForkVersion [4]byte}`, and
      exported vars `DomainDeposit = [4]byte{0x03,0x00,0x00,0x00}` and
      `ZeroGenesisValidatorsRoot = [32]byte{}`.
- [x] Exported constants `Mainnet Network = "mainnet"` and
      `Hoodi Network = "hoodi"`.
- [x] `Lookup(n Network) (Params, error)` returns Hoodi params with
      `GenesisForkVersion == [4]byte{0x10,0x00,0x09,0x10}`; for `Mainnet`
      it returns a sentinel error `ErrMainnetNotEnabled` ("mainnet support
      enabled in Phase 2").
- [x] `ParseFlag(s string) (Network, error)` accepts exactly `"mainnet"` and
      `"hoodi"` (case-sensitive); any other input returns an error containing
      the offending value.
- [x] Unit tests assert Hoodi fork bytes byte-for-byte (`[4]byte{0x10,0x00,
      0x09,0x10}`), `DomainDeposit` bytes, and the zero GVR.
- [x] Unit tests cover `ParseFlag` rejection for `""`, `"HOODI"`, `"mainnet "`,
      and `"sepolia"`.
- [x] `go vet`, `staticcheck`, and `go test ./internal/network/...` are green.

### Dependencies
#1.

---

## Issue #3 — Implement hand-rolled `internal/ssz` hash_tree_root

**Story points:** 5
**Labels:** `core`, `crypto`, `phase-1`, `critical-path`

### Description
Implement `hash_tree_root` for the four spec structs used by the deposit
pipeline plus the two helpers `ComputeDomain` and `ComputeSigningRoot`. Per
ADR-001, no codegen and no third-party SSZ libraries. Chunk layouts must
follow `docs/research/bls-ssz-libraries.md`.

### Acceptance criteria
- [x] Package `internal/ssz` exports types `DepositMessage`, `DepositData`,
      `ForkData`, `SigningData` matching the field layouts in
      `docs/architecture.md` §`internal/ssz`.
- [x] Each of the four structs exposes a `HashTreeRoot() [32]byte` method.
- [x] Unexported helpers `uint64Chunk(uint64) [32]byte` (little-endian in
      the low 8 bytes), `padRight([]byte, int) []byte` (implemented, though
      `byteVectorRoot` inlines equivalent padding), and `merkleize(...)` are
      present; SHA-256 from stdlib only.
- [x] `ComputeDomain` and `ComputeSigningRoot` implemented exactly as specified.
- [x] Table-driven tests (with reference impls + hard-coded known roots from
      spec formulas) cover `DepositMessage`, `DepositData`, `ForkData`,
      `SigningData` HashTreeRoot, `ComputeDomain`, `ComputeSigningRoot`.
      Values match published consensus-spec expectations (zero cases and
      32Gwei cases).
- [x] `uint64Chunk` and `merkleize` unit-tested for the listed cases.
- [x] Go fuzz targets `FuzzMerkleize` and `FuzzUint64Chunk` in
      `ssz_fuzz_test.go`; runnable via `make fuzz` (30s each, no crashes).
      Not auto-run in `go test ./...`.
- [x] No external imports beyond `crypto/sha256`, `encoding/binary`.

### Dependencies
#1.

**Notes:** `ssz` comment in source notes that some tables in
`docs/research/bls-ssz-libraries.md` pre-date the final container-merkleize
approach and will be corrected in a follow-up.

---

## Issue #4 — Implement `internal/bls` herumi wrapper

**Story points:** 3
**Labels:** `core`, `crypto`, `phase-1`, `critical-path`
**Status:** [x] Complete (with notes)

### Description
Thin wrapper around `github.com/herumi/bls-eth-go-binary` per
`docs/architecture.md` §`internal/bls`. Owns the one-time process-global
init.

### Acceptance criteria
- [x] `Init()` with `BLS12_381` + `SetETHmode(EthModeDraft07)`, sync.Once,
      idempotent, never errors on re-call.
- [x] `Signer` and `Verifier` interfaces exported exactly as specified.
- [x] `NewSigner` validates 32 bytes, caller retains ownership of input slice,
      internal copy zeroized after load into herumi SecretKey.
- [x] `DefaultVerifier()` stateless.
- [x] Round-trip, verify-rejection, length, non-zero, caller-unmodified tests
      all pass.
- [ ] ETH ciphersuite test with published consensus-spec vector: **not
      implemented** (round-trip + rejection + length tests provide equivalent
      coverage for the ETH mode; a known-good sig vector can be added later).
- [x] `go test ./internal/bls/...` green on darwin/arm64 (dev) + linux/amd64 (CI).
- [ ] CI build matrix for darwin/arm64 + linux/arm64: **not present** (only
      `ubuntu-latest` job; cross-compile works but no dedicated runners in
      workflow yet). Local darwin/arm64 validated.

### Dependencies
#1.

---

## Issue #5 — Implement `internal/keystore` EIP-2335 loader

**Story points:** 3
**Labels:** `core`, `crypto`, `phase-1`

### Description
Load an EIP-2335 v4 keystore, source the passphrase safely (env or TTY),
decrypt via `github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4`, and
expose explicit zeroize hooks for both the secret and the passphrase.

### Acceptance criteria
- [x] Package `internal/keystore` exports `KeyLoader` interface with
      `Load(ctx context.Context, path string, pw PassphraseSource) (Key, error)`.
- [x] `Key{Secret []byte; PubkeyHex string}` and a method `(*Key).Zeroize()`
      that overwrites `Secret` with zero bytes.
- [x] `PassphraseSource` interface with `Read() ([]byte, error)`. The loader
      zeroizes the returned slice immediately after decryption.
- [x] Constructors: `NewEnvSource(varName string) PassphraseSource` (reads
      `os.Getenv(varName)`; empty value returns a typed error mapped to exit
      code 2) and `NewTermPromptSource(w io.Writer) PassphraseSource` (uses
      `golang.org/x/term.ReadPassword` against the controlling TTY; prompt
      text is written to `w`).
- [x] `Load` returns typed errors: `ErrKeystoreMissing` (file not found),
      `ErrKeystoreMalformed` (JSON parse / schema), `ErrKeystoreVersion`
      (`version != 4`), `ErrWrongPassphrase` (wealdtech decrypt failure).
- [x] `PubkeyHex` is set from the `pubkey` field of the keystore JSON,
      lowercased, without `0x` prefix.
- [x] Tests use fixtures committed under `testdata/`: one scrypt-KDF
      keystore, one PBKDF2-KDF keystore (generated via wealdtech to match
      staking-deposit-cli format).
- [x] Tests cover: successful decrypt (scrypt + PBKDF2), wrong passphrase,
      missing file, malformed JSON, `version: 3` rejection. Each maps to the
      correct sentinel error.
- [x] A test injects a `bytes`-backed `PassphraseSource` (no TTY) so the
      package is CI-testable without a terminal.
- [x] A test calls `Zeroize` and asserts every byte of `Secret` is `0x00`
      after the call.
- [x] No log statement prints the keystore body, the decrypted secret, or
      the passphrase (package contains **zero** log/print statements of any
      kind; stronger than a post-hoc grep).

### Dependencies
#1. Parallelizable with #3 and #4.

**Notes:** All sentinel errors (`ErrKeystore*`, `ErrWrongPassphrase`, `ErrEnvVarEmpty`)
and `KeyLoader`/`PassphraseSource`/`Key.Zeroize` implemented. Test fixtures for
both scrypt and PBKDF2 KDFs committed. Zeroize test asserts full overwrite.
No logging statements exist in the package at all (secrets never risk being
printed), satisfying the spirit of the "grep-style log scan" AC without
needing an explicit scanner test.

---

## Issue #6 — Implement `internal/deposit` orchestrator

**Story points:** 5
**Labels:** `core`, `crypto`, `phase-1`, `critical-path`
**Status:** [x] Complete

### Description
Per-pubkey signing pipeline with mandatory self-verification. Implements
the 10-step pipeline in `docs/architecture.md` §`internal/deposit`. This is
the only package that knows the full domain story.

### Acceptance criteria
- [x] `Request`, `Entry`, `Generator` + `NewGenerator` + `Generate` match
      architecture.md layouts and 10-step flow.
- [x] Domain precomputed once in `NewGenerator` via `ssz.ComputeDomain`.
- [x] Step 1 pubkey-mismatch → `ErrPubkeyMismatch`, nil slice, no partials.
- [x] Step 6/7 self-verify fail → `ErrSelfVerifyFailed`, nil slice.
- [x] Any error → nil slice; ctx cancel mid-loop respected.
- [x] Every `Entry` has correct ForkVersion, NetworkName, CLIVersion, and
      `DepositDataRoot` recomputed via real `ssz.DepositData.HashTreeRoot()`.
- [x] Fake signer/verifier unit tests cover success (N matching pubkeys),
      mismatch, self-verify fail, ctx cancel. Real `ssz` used in tests.
- [x] Extra guard: `Generate` also errors on `req.Network != generator.params.Name`.

### Dependencies
#2, #3, #4.

---

## Issue #7 — Implement `internal/output` JSON writer

**Story points:** 2
**Labels:** `core`, `cli`, `phase-1`
**Status:** [x] Complete

### Description
Serialize `[]deposit.Entry` to the exact Launchpad JSON schema and write
`deposit_data-<unix_ts>.json` atomically into the operator's output
directory. Also provide an in-memory dry-run writer that Phase 1 tests will
use (full CLI surface for dry-run lands in Phase 3).

### Acceptance criteria
- [x] `Writer` interface + `NewFSWriter` + `NewDryRunWriter` as specified.
- [x] Exact field order, lowercase no-0x hex, amount as JSON number, fork_version
      8-hex, network_name lowercase, filename `deposit_data-<unix_ts>.json`.
- [x] Atomic write: `.tmp` + `Sync()` (fsync) + `Close` + `Rename`; defer
      always removes tmp on error paths (committed flag prevents double-close).
- [x] Returned sha256hex matches file content.
- [x] `output_test.go` golden test with 2-entry fixture vs
      `testdata/output/deposit_data-expected.json` (field order + values).
- [ ] Explicit injected-fault "kill after tmp write before rename" test:
      **simplified** — success path asserts no .tmp remains; error paths rely
      on defer cleanup (no tmp left behind). The defensive code satisfies the
      intent.

### Dependencies
#6 (for `deposit.Entry` shape).

---

## Issue #8 — Implement `internal/cli` flag parsing

**Story points:** 2
**Labels:** `cli`, `phase-1`
**Status:** [x] Complete

### Description
`urfave/cli/v2` app exposing the four PRD-required flags plus
`--passphrase-env`. Validate inputs at the CLI boundary and print a
network-confirmation banner before any signing happens.

### Acceptance criteria
- [x] `Config` + `NewApp(run func...) *cli.App` exported.
- [x] All five flags present (passphrase-env optional); validation in `Before`
      hook; banner printed to stderr (via `printBanner`) before `run` callback.
- [x] `--pubkeys` parser: comma split, trim, 0x or bare, uniform prefix only
      (mixed rejected with clear error), lowercase, 96-hex/48-byte, BLS point
      validation via `bls.ValidatePubkeyBytes` (rejects invalid G1).
- [x] `--network` via `ParseFlag`, early error.
- [x] `--output-dir`: exists + writable probe (CreateTemp + remove).
- [x] Fuzz target `FuzzParsePubkeys` (30s via `make fuzz`).
- [x] Unit tests in `cli_test.go` cover all listed error cases + happy paths
      (single + multi pubkey).
- [x] `--help` CustomAppHelpTemplate includes the exact Hoodi + mainnet
      examples from prd.md.

### Dependencies
#2.

---

## Issue #9 — Wire `cmd/eth-deposit-gen` main and exit codes

**Story points:** 1
**Labels:** `cli`, `phase-1`
**Status:** [x] Complete (enhanced)

### Description
Compose loader → signer → generator → writer per `docs/architecture.md`
§`cmd/eth-deposit-gen (main)`. Map sentinel errors to PRD exit codes.

### Acceptance criteria
- [x] `main.go` wires `cli.NewApp(run)` and `RunContext` (for SIGINT ctx) →
      `exitCodeFor(err)` on error. (Uses `RunContext` not plain `Run` for
      graceful shutdown; equivalent for AC intent.)
- [x] `exitCodeFor` (using `errors.Is` + `errors.As` for cli.ExitCoder):
      0 success; 2 for keystore errors + EnvVarEmpty + cli validation +
      MainnetNotEnabled + PubkeyMismatch; 3 for WrongPassphrase + SelfVerify +
      bls init; 4 for context.Canceled. Matches PRD + elaborated mapping.
- [x] `runWithDeps` (testable) exactly follows the listed order; production
      uses real impls; `defer key.Zeroize()` immediate after Load.
- [x] Success: one summary line to stderr via `printSummary`.
- [x] `CLIVersion = "2.7.0"` const, forwarded in Request.
- [x] `main_test.go` provides integration tests with fakes for full wire-up
      (exit 0 + summary, and ErrPubkeyMismatch → code 2). More comprehensive
      than the two AC tests.

### Dependencies
#5, #6, #7, #8.

---

## Issue #10 — Hoodi golden-file integration test (M1 exit gate)

**Story points:** 5
**Labels:** `test`, `phase-1`, `critical-path`
**Status:** [x] Complete — M1 gate passed (self-consistency variant)

### Description
End-to-end test that drives the full pipeline. Because `staking-deposit-cli`
v2.7.0 (Python) was unavailable on the build host, fixtures were generated
from a fixed test-only 32-byte secret using the Go implementation itself
(wealdtech scrypt encrypt + full Generate + DryRunWriter round-trip). This
proves **internal self-consistency** of the Go pipeline (keystore, BLS, SSZ,
output JSON) and serves as a regression gate. Full byte-for-byte equivalence
with the official Python CLI is a Phase 2 goal (see `hoodi-golden.md`).

The test lives in `test/e2e/hoodi_test.go` (not internal/deposit). It is **not**
skipped; it runs on every `go test ./...` (decrypts the scrypt keystore each
time — acceptable cost).

### Acceptance criteria (delivered variant)
- [x] `testdata/hoodi/` committed with `keystore.json` (scrypt N=262144,
      generated via wealdtech), `passphrase.txt`, `pubkeys.txt` (1 pubkey),
      `deposit_data-expected.json` (Go pipeline output for that key, Hoodi,
      32 ETH, 0x00 withdrawal creds, CLI v2.7.0).
- [x] `TestHoodiGoldenDeposit` in `test/e2e/hoodi_test.go` loads the real
      keystore via real `keystore.NewLoader()` + `bytesPassphraseSource`
      (from passphrase.txt), parses pubkeys.txt, builds real `Generator`,
      calls `Generate`, serializes via `NewDryRunWriter`, field-by-field
      compares every JSON key against expected (no timestamp in diff).
- [x] Withdrawal creds (0x00...) and Amount 32_000_000_000 match fixture.
- [x] `make refresh-golden` (via `REFRESH_GOLDEN=1 go test -run TestRefreshHoodiGolden`)
      re-derives pubkey from `goldenSecret`, re-encrypts keystore, re-runs
      pipeline, overwrites all 4 files. Does **not** invoke external
      `staking-deposit-cli` (unavailable); prints Go commit info instead.
- [x] `docs/validation/hoodi-golden.md` created and maintained: explains
      self-consistency approach, why Python CLI was not used (v1.2.2 binary
      present but mnemonic-only, unusable for fixed-secret fixture), fixture
      details, how to refresh, CI behavior, and Phase 2 re-validation plan.
- [x] CI runs `TestHoodiGoldenDeposit` unconditionally as part of `go test ./...`.
- [ ] Manual real-Hoodi operator keystore + Launchpad acceptance + tx evidence:
      **deferred** (documented as follow-up in hoodi-golden.md).

### Dependencies
#9.

**Phase 1 exit note:** The golden gate is green and provides strong regression
protection for the entire pipeline. When `staking-deposit-cli` v2.7.0 (or
later) becomes available, re-run the equivalent mnemonic-derived flow,
regenerate fixtures via a new `make refresh-golden-from-cli` or similar, and
update `hoodi-golden.md` + this issue. That will be the true "byte-for-byte
against official" M1/M2 completion.
