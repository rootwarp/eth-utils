# Software Architecture: Ethereum Validator Deposit Data Generator (`eth-deposit-gen`)

## Overview

`eth-deposit-gen` is a single-binary Go CLI that produces Launchpad-compatible
`deposit_data-<ts>.json` files for a caller-supplied set of BLS validator
pubkeys, signing each deposit with a BLS key loaded from an EIP-2335 keystore.

The architecture is a **modular monolith inside a single Go module**:
a thin `cmd/` entry point performs flag parsing, then orchestrates a set of
narrow, independently-testable `internal/` packages — `keystore`, `bls`, `ssz`,
`network`, `deposit`, `output`. Each package exposes a small interface so that
the orchestrator (`deposit.Generator`) can be unit-tested with fakes and so
that the signing pipeline can be reasoned about end-to-end without reaching
into package internals.

The driving constraint is correctness: a single byte wrong in the signing
pipeline permanently locks 32 ETH per validator. The design therefore favors
**few dependencies, hand-rolled SSZ for the four spec structs, hard-coded
network constants, and self-verification at the boundary** over flexibility.

## Architecture Principles

- **Correctness-first, dependency-minimal** — Every additional dependency on
  the signing path is a supply-chain risk against funds. Hand-roll SSZ for the
  four fixed-size structs; lean only on audited libraries (`herumi/bls`,
  `wealdtech/keystorev4`) where rolling our own would be reckless.
- **Pure-core, impure-shell** — `ssz`, `deposit`, and `network` are pure
  (no I/O, no globals, no goroutines). I/O (`keystore` loading, `output`
  writing) and CGO/process-global state (`bls`) live at the edges behind
  interfaces.
- **Interface at the seam, struct inside** — Each package exposes the smallest
  interface its consumer actually needs (`Signer`, `KeyLoader`, `Writer`).
  Concrete types are unexported.
- **Hard-coded network constants** — Fork versions, domain type, network
  names, and the deposit `genesis_validators_root` (zero) are compile-time
  constants. No network fetches, no config files.
- **Verify-before-write** — Every entry is BLS-verified and its
  `deposit_data_root` re-computed before any bytes hit disk. One failure
  aborts the whole run with a non-zero exit code; no partial output.
- **No secrets on the CLI surface** — Only a keystore *path* and an
  *env-var name* (or interactive prompt) are ever accepted. Decoded key
  material is zeroized after use.

## System Context Diagram

```text
                  ┌──────────────────────────────────────────────┐
                  │                  eth-deposit-gen             │
   operator ───▶  │                                              │  ───▶  deposit_data-<ts>.json
   (TTY/CI)       │  flags ─▶ keystore ─▶ deposit.Generator ─▶ output│      (filesystem)
                  │                          ▲                   │
                  │                   ssz / bls / network        │
                  └──────────────────────────────────────────────┘
                                          │
                                          ▼
                                  EIP-2335 keystore
                                  (filesystem, read-only)
```

No network calls. No external services. Inputs: keystore file + flags.
Output: one JSON file. The deposit JSON is later consumed off-host by the
Launchpad / deposit contract — out of scope for this tool.

## Repository Placement

The existing repo already establishes a polyglot layout:

```
/Users/nil/git/rootwarp/eth-utils/
├── go/
│   ├── go.work          ← workspace, use ( ./cmd/myutil, ./pkg/mylib )
│   ├── cmd/             ← one Go module per CLI tool
│   ├── pkg/             ← shared, exported Go libraries
│   └── internal/        ← (workspace-level placeholder, unused for now)
├── rust/  python/  scripts/  ...
```

`eth-deposit-gen` is therefore added as **its own Go module** under
`go/cmd/eth-deposit-gen/`, and registered in `go/go.work`:

```
// go/go.work
go 1.23
use (
    ./cmd/eth-deposit-gen
)
```

This keeps the tool's dependency graph (notably CGO/`herumi`) isolated from
any future pure-Go libraries under `go/pkg/`.

## Module / Package Overview

| Package                              | Responsibility                                          | Owns                          | Depends on (internal)    | I/O? |
|--------------------------------------|---------------------------------------------------------|-------------------------------|--------------------------|------|
| `cmd/eth-deposit-gen` (`main`)       | Flag parsing, wiring, exit codes                        | CLI surface                   | all below                | yes  |
| `internal/cli`                       | `urfave/cli/v2` app + flag → typed `Config`             | flag schema, validation       | `network`                | env  |
| `internal/network`                   | Compile-time network constants (mainnet, hoodi)       | `Network` enum, fork bytes    | —                        | no   |
| `internal/keystore`                  | EIP-2335 load + decrypt + zeroize                       | `KeyLoader` iface, key bytes  | (wealdtech, x/term)      | yes  |
| `internal/bls`                       | herumi init, sign, verify, pubkey-from-secret           | `Signer`, `Verifier` ifaces   | (herumi)                 | no*  |
| `internal/ssz`                       | Hand-rolled `hash_tree_root` for the 4 spec structs     | chunking + merkleize          | —                        | no   |
| `internal/deposit`                   | Domain orchestration: build message, sign, self-verify  | `Generator`, `Entry`          | `ssz`, `bls`, `network`  | no   |
| `internal/output`                    | Serialize `[]Entry` → Launchpad JSON, write to disk     | `Writer` iface, file naming   | —                        | yes  |

`*` `bls` has CGO-global state (one-time `bls.Init` + `SetETHmode`) but no
filesystem or network I/O.

### Module dependency graph

```text
cmd/eth-deposit-gen
        │
        ▼
   internal/cli ─────────────────────────────────────▶ internal/network
        │
        ▼
   internal/keystore ──▶ wealdtech/keystorev4
        │   x/term (prompt)
        ▼
   internal/deposit ──▶ internal/ssz
        │           └─▶ internal/bls ──▶ herumi/bls-eth-go-binary
        │           └─▶ internal/network
        ▼
   internal/output  (stdlib only)
```

No cycles. `ssz`, `network`, `bls` have no internal dependencies and are leaf
packages. `deposit` is the only place that knows about the full pipeline.

---

## Package Details

### `internal/network`

**Responsibility:** Source of truth for per-network constants used in the
signing domain and the JSON output.

**Domain types:**

```go
type Network string

const (
    Mainnet Network = "mainnet"
    Hoodi Network = "hoodi"
)

type Params struct {
    Name              Network
    GenesisForkVersion [4]byte // e.g. {0x00,0x00,0x00,0x00}
}

// DomainDeposit is the 4-byte SSZ domain type for deposits (spec constant).
var DomainDeposit = [4]byte{0x03, 0x00, 0x00, 0x00}

// ZeroGenesisValidatorsRoot is the genesis_validators_root used for deposit
// signing — always 32 zero bytes per the consensus spec.
var ZeroGenesisValidatorsRoot = [32]byte{}

func Lookup(n Network) (Params, error)
func ParseFlag(s string) (Network, error) // accepts "mainnet"|"hoodi", case-sensitive
```

**Key decisions:**
- Constants are compiled in — no runtime config. Removes a class of
  supply-chain attack.
- `DOMAIN_DEPOSIT` and the zero genesis-validators-root live here, not in
  `deposit`, so the spec constants are colocated.

---

### `internal/keystore`

**Responsibility:** Load an EIP-2335 v4 keystore from disk, source the
passphrase safely, decrypt, return the raw 32-byte BLS secret, and provide a
zeroize hook.

**Interface:**

```go
type KeyLoader interface {
    // Load reads and decrypts the keystore at path, returning the raw 32-byte
    // BLS secret and the pubkey hex string declared in the keystore JSON.
    // The returned slice MUST be zeroized by the caller via Zeroize.
    Load(ctx context.Context, path string, pw PassphraseSource) (Key, error)
}

type Key struct {
    Secret     []byte // 32 bytes; zeroize after use
    PubkeyHex  string // 96 lowercase hex, no 0x — from keystore JSON
}

func (k *Key) Zeroize()

// PassphraseSource abstracts where the passphrase comes from so the loader
// can be tested without TTY or env.
type PassphraseSource interface {
    Read() ([]byte, error) // returns a slice the loader will zeroize
}

func NewEnvSource(varName string) PassphraseSource          // os.Getenv
func NewTermPromptSource(w io.Writer) PassphraseSource      // x/term, prompt on w (stderr)
```

**Failure modes:**
- File missing / not JSON / `version != 4` → typed error, exit code 2.
- Wrong passphrase → checksum mismatch from wealdtech → exit code 3.
- Empty / unset env var → exit code 2 (user error before signer ever runs).

**Key decisions:**
- Passphrase source is an interface so test code injects a `bytes` source
  and TTY isn't required in CI.
- `Zeroize` is explicit — Go's GC does not clear key material.

---

### `internal/bls`

**Responsibility:** Thin, Ethereum-flavored wrapper around
`herumi/bls-eth-go-binary`. Owns one-time init.

**Interface:**

```go
// Init must be called exactly once at process start. Idempotent guard inside.
func Init() error  // calls bls.Init(bls.BLS12_381) + bls.SetETHmode(EthModeDraft07)

type Signer interface {
    Sign(signingRoot [32]byte) (sig [96]byte, err error)
    PublicKey() (pub [48]byte, err error)
}

type Verifier interface {
    Verify(pub [48]byte, signingRoot [32]byte, sig [96]byte) (bool, error)
}

// NewSigner consumes a 32-byte secret. Caller still owns + zeroizes secret.
func NewSigner(secret []byte) (Signer, error)

// DefaultVerifier returns a stateless verifier backed by herumi.
func DefaultVerifier() Verifier
```

**Key decisions:**
- All `[N]byte` fixed-size arrays at the boundary — no ambiguity over length.
- `Signer` does not expose the secret; once constructed, the wrapper holds the
  herumi `SecretKey` and exposes only Sign / PublicKey.
- `Init` lives here because herumi's mode is process-global; setting it from
  anywhere else would invite surprises.

---

### `internal/ssz`

**Responsibility:** Hand-rolled `hash_tree_root` for the four structs the
deposit pipeline needs: `DepositMessage`, `DepositData`, `ForkData`,
`SigningData`. Plus `ComputeDomain` and `ComputeSigningRoot` helpers.

**Public surface:**

```go
type DepositMessage struct {
    Pubkey                [48]byte
    WithdrawalCredentials [32]byte
    Amount                uint64 // gwei
}
func (m DepositMessage) HashTreeRoot() [32]byte

type DepositData struct {
    Pubkey                [48]byte
    WithdrawalCredentials [32]byte
    Amount                uint64
    Signature             [96]byte
}
func (d DepositData) HashTreeRoot() [32]byte

type ForkData struct {
    CurrentVersion         [4]byte
    GenesisValidatorsRoot  [32]byte
}
func (f ForkData) HashTreeRoot() [32]byte

type SigningData struct {
    ObjectRoot [32]byte
    Domain     [32]byte
}
func (s SigningData) HashTreeRoot() [32]byte

// ComputeDomain = domainType[0:4] || ForkData{forkVersion, gvr}.HashTreeRoot()[0:28]
func ComputeDomain(domainType [4]byte, forkVersion [4]byte, gvr [32]byte) [32]byte

// ComputeSigningRoot = SigningData{objectRoot, domain}.HashTreeRoot()
func ComputeSigningRoot(objectRoot [32]byte, domain [32]byte) [32]byte
```

**Internal helpers (unexported):**

```go
func uint64Chunk(v uint64) [32]byte                 // LE in low 8 bytes
func padRight(b []byte, size int) []byte            // 0-pad
func merkleize(chunks [][32]byte, limit int) [32]byte // pad to next pow2, then pairwise SHA-256
```

**Key decisions:**
- No reflection, no codegen, no tags. Four `HashTreeRoot` methods, each
  ~10 lines, total package well under 200 LOC including tests.
- Chunk layouts exactly as documented in `docs/research/bls-ssz-libraries.md`
  — that document is the spec for this package.
- SHA-256 from `crypto/sha256` (stdlib).

---

### `internal/deposit`

**Responsibility:** Orchestrate the per-pubkey signing pipeline and enforce
self-verification. This is the only package that knows the full domain story.

**Interface:**

```go
type Request struct {
    Network               network.Network
    Pubkeys               [][48]byte         // parsed + validated upstream
    WithdrawalCredentials [32]byte           // uniform for v1 (0x01... layout)
    AmountGwei            uint64             // default 32e9
    DepositCLIVersion     string             // e.g. "2.7.0"
}

type Entry struct {
    Pubkey               [48]byte
    WithdrawalCredentials [32]byte
    Amount               uint64
    Signature            [96]byte
    DepositMessageRoot   [32]byte
    DepositDataRoot      [32]byte
    ForkVersion          [4]byte
    NetworkName          network.Network
    DepositCLIVersion    string
}

type Generator struct {
    Signer   bls.Signer    // already keyed
    Verifier bls.Verifier
    // unexported fields: precomputed domain, fork version, gvr=zero
}

func NewGenerator(s bls.Signer, v bls.Verifier, params network.Params) *Generator

// Generate processes every pubkey in req. Returns ([]Entry, nil) only if
// every entry passed self-verification. Returns the offending pubkey on any
// failure and no partial slice.
func (g *Generator) Generate(ctx context.Context, req Request) ([]Entry, error)
```

**Pipeline (per pubkey):**

```
1. Assert signer.PublicKey() == req.Pubkeys[i]               (else error: PubkeyMismatch)
2. msg     := ssz.DepositMessage{pubkey, wc, amount}
3. msgRoot := msg.HashTreeRoot()
4. domain  := ssz.ComputeDomain(DOMAIN_DEPOSIT, forkVersion, ZERO_GVR)   // precomputed once
5. signing := ssz.ComputeSigningRoot(msgRoot, domain)
6. sig     := signer.Sign(signing)
7. ok      := verifier.Verify(pubkey, signing, sig)          (else error: VerifyFailed)
8. data    := ssz.DepositData{pubkey, wc, amount, sig}
9. dataRoot:= data.HashTreeRoot()
10. emit Entry{ ... }
```

Step 1's check enforces the PRD's "loaded private key must correspond to the
supplied pubkey." A single keystore + many pubkeys (the common operator case)
implies all pubkeys in `req.Pubkeys` must equal the signer's pubkey; the tool
fails fast on the first mismatch.

**Failure modes:**
- Pubkey mismatch with signer → `ErrPubkeyMismatch`, exit code 2.
- BLS verify returns false after our own sign → `ErrSelfVerifyFailed`,
  exit code 3 (signer or SSZ bug — should never happen in practice).
- Context cancelled mid-batch → return partial-progress error, write nothing.

**Concurrency:** v1 is sequential. The interface accepts a `context.Context`
so a future `--parallel N` worker pool (PRD P1) can be added without changing
callers.

---

### `internal/output`

**Responsibility:** Serialize `[]deposit.Entry` to the exact Launchpad JSON
schema and write `deposit_data-<unix_ts>.json` atomically into the output
directory.

**Interface:**

```go
type Writer interface {
    Write(ctx context.Context, dir string, entries []deposit.Entry, now time.Time) (path string, sha256hex string, err error)
}

func NewFSWriter() Writer
func NewDryRunWriter(w io.Writer) Writer // P1: --dry-run prints to w instead
```

**Wire format:** lowercase hex without `0x` for byte fields; `amount` as JSON
number; `network_name` as the network's short string. Field order matches the
official CLI for byte-for-byte diff-ability of golden fixtures.

```go
type jsonEntry struct {
    Pubkey               string `json:"pubkey"`
    WithdrawalCredentials string `json:"withdrawal_credentials"`
    Amount               uint64 `json:"amount"`
    Signature            string `json:"signature"`
    DepositMessageRoot   string `json:"deposit_message_root"`
    DepositDataRoot      string `json:"deposit_data_root"`
    ForkVersion          string `json:"fork_version"`
    NetworkName          string `json:"network_name"`
    DepositCLIVersion    string `json:"deposit_cli_version"`
}
```

**Atomicity:** write to `dir/.deposit_data-<ts>.json.tmp`, fsync, rename to
final path. No half-written deposit files on operator disks.

---

### `internal/cli`

**Responsibility:** Define the `urfave/cli/v2` app, parse flags into a typed
`Config`, validate (pubkey hex, network value, dir existence), and hand off to
`cmd/eth-deposit-gen` for wiring.

```go
type Config struct {
    KeystorePath  string
    Pubkeys       [][48]byte
    Network       network.Network
    OutputDir     string
    PassphraseEnv string // empty = use TTY prompt
}

func NewApp(run func(context.Context, Config) error) *cli.App
```

Flag schema is exactly:
```
--validator-key-path  (string, required)
--pubkeys             (string, required, comma-sep, validated as 48-byte hex)
--network             (string, required, one of {mainnet, hoodi})
--output-dir          (string, required, must exist & be writable)
--passphrase-env      (string, optional; empty triggers stdin prompt)
```

---

### `cmd/eth-deposit-gen` (main)

**Responsibility:** Compose everything. ~50 lines.

```go
func main() {
    app := cli.NewApp(run)
    if err := app.Run(os.Args); err != nil {
        // exit code mapping per PRD: 2=validation, 3=signer, 4=user abort
        os.Exit(exitCodeFor(err))
    }
}

func run(ctx context.Context, cfg cli.Config) error {
    if err := bls.Init(); err != nil { return err }

    params, _ := network.Lookup(cfg.Network)

    pwSrc := pickPassphraseSource(cfg) // env or TTY
    key, err := keystore.NewLoader().Load(ctx, cfg.KeystorePath, pwSrc)
    if err != nil { return err }
    defer key.Zeroize()

    signer, err := bls.NewSigner(key.Secret)
    if err != nil { return err }

    gen := deposit.NewGenerator(signer, bls.DefaultVerifier(), params)
    entries, err := gen.Generate(ctx, deposit.Request{
        Network:               cfg.Network,
        Pubkeys:               cfg.Pubkeys,
        WithdrawalCredentials: defaultWithdrawalCreds(), // P1 may make configurable
        AmountGwei:            32_000_000_000,
        DepositCLIVersion:     CLIVersion,
    })
    if err != nil { return err }

    path, sum, err := output.NewFSWriter().Write(ctx, cfg.OutputDir, entries, time.Now())
    if err != nil { return err }
    fmt.Fprintf(os.Stderr, "wrote %s (sha256=%s, n=%d, network=%s)\n",
        path, sum, len(entries), cfg.Network)
    return nil
}
```

Note: the v1 design hard-codes `withdrawal_credentials` per the PRD's
"defaults applied uniformly" note. A future flag (e.g.
`--withdrawal-address`) plugs into `defaultWithdrawalCreds()` without
changing any internal package signature.

---

## End-to-End Data Flow

```text
flags  ──▶ cli.Config
                │
                ▼
  ┌──────────────────────────────────┐
  │ keystore.Load(path, pwSrc)       │  reads keystore JSON, prompts/env passphrase
  │  └─▶ wealdtech.Decrypt           │  AES-128-CTR + scrypt/PBKDF2
  │      └─▶ 32-byte BLS secret      │
  └───────────────┬──────────────────┘
                  │
                  ▼
          bls.NewSigner(secret) ─────▶ secret zeroized via defer
                  │
                  ▼
  ┌──────────────────────────────────────────────────────┐
  │ deposit.Generator.Generate(req)  — for each pubkey:   │
  │                                                       │
  │   pubkey ── signer.PublicKey() equality check         │
  │                                                       │
  │   ssz.DepositMessage{pub,wc,amount}.HashTreeRoot()    │
  │      └─▶ msgRoot                                       │
  │                                                       │
  │   ssz.ComputeDomain(0x03000000,                       │
  │                     params.GenesisForkVersion,        │
  │                     ZERO_GVR)                         │
  │      └─▶ domain                                       │
  │                                                       │
  │   ssz.ComputeSigningRoot(msgRoot, domain)             │
  │      └─▶ signingRoot                                  │
  │                                                       │
  │   signer.Sign(signingRoot) ──▶ sig                    │
  │   verifier.Verify(pub, signingRoot, sig) MUST be true │
  │                                                       │
  │   ssz.DepositData{pub,wc,amount,sig}.HashTreeRoot()   │
  │      └─▶ dataRoot                                     │
  │                                                       │
  │   emit Entry{…}                                       │
  └───────────────┬──────────────────────────────────────┘
                  │
                  ▼
   output.Write(dir, entries, now)
        └─▶ deposit_data-<unix_ts>.json (atomic rename)
        └─▶ stderr: path, sha256, count, network
```

Failure at any step aborts the run with no file written and an exit code that
maps cleanly to the PRD's spec (2 / 3 / 4).

---

## Cross-Cutting Concerns

### Logging & Observability
- `log/slog` configured in `main`; passed via `context` to downstream
  packages that want to log (none currently need to — the signing path is
  silent). Final summary banner is printed to **stderr**.
- No log line ever contains: passphrase, decrypted key bytes, keystore
  contents, or signing-root. Pubkeys and file paths are loggable.

### Error Handling
- Sentinel errors per package (`ErrPubkeyMismatch`, `ErrSelfVerifyFailed`,
  `ErrBadKeystore`, …). `cmd/eth-deposit-gen.exitCodeFor` does an
  `errors.Is` cascade to map to exit codes 2/3/4.
- Errors wrap with `%w` and always include the offending pubkey (or
  filename) when applicable.

### Configuration
- Network constants: compile-time, in `internal/network`.
- `CLIVersion`: `var CLIVersion = "2.7.0"` in `cmd/eth-deposit-gen` —
  mirrors latest tested `staking-deposit-cli` release; bumped only after
  golden-file re-validation passes.
- All other inputs come from CLI flags. No config files.

### Security
- Passphrase via env-var-name or TTY only — never as a flag value.
- `key.Zeroize()` deferred immediately after `keystore.Load`.
- Filesystem writes via temp + rename.
- `go.sum` checked in; CI uses `GOFLAGS=-mod=readonly`.

---

## Testing Strategy

### Unit Tests (per package, fakes at interfaces)

- `internal/network`: `Lookup` table tests; `ParseFlag` rejects unknown
  values; mainnet/hoodi bytes asserted byte-for-byte.
- `internal/ssz`: **golden-vector tests** with known
  `DepositMessage` / `DepositData` inputs whose `hash_tree_root` values are
  published in the consensus-spec test vectors. Each `HashTreeRoot` method
  has its own table-driven test. `merkleize` tested independently with
  unit chunks.
- `internal/bls`: round-trip test — generate a key, sign a known root,
  verify; assert ETH ciphersuite by signing a spec-published vector and
  checking the signature byte-for-byte.
- `internal/keystore`: tests use a fixture EIP-2335 keystore generated by
  `staking-deposit-cli` (committed under `testdata/`). Cover scrypt and
  PBKDF2 KDFs; wrong-passphrase path; missing-file path; non-v4-version
  path. `PassphraseSource` is faked with a `[]byte`-returning stub.
- `internal/deposit`: fake `Signer` + fake `Verifier` test the orchestrator
  decisions (pubkey mismatch aborts, verify-fail aborts, success emits
  Entry with correct roots). Uses real `ssz` package.
- `internal/output`: golden-file test asserting the written JSON matches a
  fixture byte-for-byte; atomic-rename test using `t.TempDir`.

### Integration / Golden-File Tests (the real correctness gate)

A single test package (`internal/deposit/golden_test.go` or
`test/e2e/`) drives the full pipeline against fixtures produced by the
official `staking-deposit-cli`:

```
testdata/
├── mainnet/
│   ├── keystore.json              ← from staking-deposit-cli
│   ├── passphrase.txt             ← plaintext, test-only
│   ├── pubkeys.txt
│   └── deposit_data-expected.json ← from staking-deposit-cli
└── hoodi/
    └── (same layout)
```

The test:
1. Loads the keystore via real `keystore.Loader` with a `bytes`-source
   passphrase.
2. Runs `deposit.Generator.Generate` with the same inputs the official CLI
   was given.
3. Re-serializes via `output.NewDryRunWriter` to an in-memory buffer.
4. Compares to `deposit_data-expected.json` **field by field**, ignoring
   the timestamped filename. Any byte-level divergence is a hard failure.

This is the gate the PRD's correctness requirement leans on. CI fails the
build if golden files diverge — and refreshing them requires an explicit,
reviewed regeneration step (`make refresh-golden`) that documents which
upstream CLI version produced them.

### Fuzz Tests

- `ssz.merkleize` and `uint64Chunk` are pure and cheap to fuzz; included
  for confidence in chunk packing.
- Pubkey-hex parsing in `internal/cli` is fuzzed for crash safety.

### CI

- `go test ./...` on linux/amd64 + linux/arm64 + macos/arm64.
- `CGO_ENABLED=1` matrix (herumi requirement).
- Lint: `go vet`, `staticcheck`.
- `go mod verify` to check checksums.

---

## Infrastructure & Deployment

- **Single static-ish binary** (CGO-dynamic against libc, otherwise
  self-contained). Built via GoReleaser for darwin/linux × amd64/arm64.
- **`go install github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen@latest`**
  path documented in README.
- No services, no daemons, no servers. The tool runs to completion in
  seconds and exits.

### Service Extraction Path
Not applicable in the microservice sense, but the package structure
intentionally makes future reuse trivial:
- `internal/ssz`, `internal/bls`, `internal/keystore`, `internal/network`
  can be promoted to `go/pkg/...` (exported) the day a sibling tool (e.g.
  `eth-exit-gen` for voluntary exits) needs them. Each is already free of
  CLI / I/O concerns, and the interfaces between them are stable.

---

## Technology Choices

| Concern              | Choice                                                   | Rationale                                                                |
|----------------------|----------------------------------------------------------|--------------------------------------------------------------------------|
| Language             | Go ≥ 1.22 (workspace pins 1.23)                          | Single-binary, fits existing repo's `go/` tree.                          |
| CLI framework        | `github.com/urfave/cli/v2`                               | PRD-mandated; minimal, well-known.                                       |
| BLS                  | `github.com/herumi/bls-eth-go-binary` (CGO)              | ETH ciphersuite preselected; years of mainnet use; small wrapper.        |
| SSZ                  | hand-rolled `hash_tree_root` (~150 LOC)                  | Only 4 fixed-size structs needed; eliminates a codegen dependency.       |
| Keystore             | `github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4` | Pure-Go EIP-2335 v4; aligned with `staking-deposit-cli` output.          |
| Passphrase prompt    | `golang.org/x/term`                                      | Echo-suppressed TTY input; standard.                                     |
| Hashing              | `crypto/sha256` (stdlib)                                 | SSZ merkleization only needs SHA-256.                                    |
| Logging              | `log/slog` (stdlib)                                      | No third-party logging dep.                                              |
| Release packaging    | GoReleaser                                               | Per PRD; multi-arch binaries.                                            |

---

## ADRs

### ADR-001: Hand-roll SSZ instead of importing `fastssz`
- **Status:** Accepted
- **Context:** Only four fixed-size structs (`DepositMessage`, `DepositData`,
  `ForkData`, `SigningData`) need `hash_tree_root`. `fastssz` brings codegen
  and a non-trivial runtime.
- **Decision:** Implement `hash_tree_root` directly in `internal/ssz`,
  validated against consensus-spec test vectors and `staking-deposit-cli`
  golden files.
- **Alternatives:** `prysmaticlabs/fastssz` (heavier, codegen step);
  Prysm-vendored SSZ (drags Prysm module graph).
- **Consequences:** ~150 LOC we own and must maintain; far smaller supply-
  chain surface; if scope grows (exits, BTEC), revisit.

### ADR-002: `herumi/bls-eth-go-binary` over `blst`
- **Status:** Accepted
- **Context:** Both are CGO; both implement the ETH BLS ciphersuite. `blst`
  is faster; `herumi` is the historical default.
- **Decision:** Use `herumi/bls-eth-go-binary`. Performance is not the
  bottleneck (≥ 200 sigs/sec required, herumi exceeds easily).
- **Alternatives:** `blst` (faster but lower-level API); Prysm wrapper
  (heavy module graph).
- **Consequences:** CGO build matrix in CI; swap to `blst` later is local
  to `internal/bls`.

### ADR-003: Modular monolith inside a single Go module
- **Status:** Accepted
- **Context:** Tool is small; package boundaries still matter for testability
  and future reuse by sibling utilities under `go/cmd/...`.
- **Decision:** One Go module at `go/cmd/eth-deposit-gen` with `internal/`
  subpackages behind interfaces. No premature extraction to `go/pkg/`.
- **Consequences:** Easy to reason about; trivial to promote packages later
  by moving them under `go/pkg/` and exporting.

### ADR-004: Compile-time network constants, no config file
- **Status:** Accepted
- **Context:** Wrong fork version = unrecoverable lost funds. Fetching from
  the network or reading from a file is a supply-chain risk.
- **Decision:** `internal/network` hard-codes mainnet and hoodi bytes.
  Adding a network requires a code change + golden-file refresh.
- **Consequences:** Adding Sepolia/Hoodi (PRD P2) is a one-PR change but
  blocked on golden fixtures from `staking-deposit-cli`.

### ADR-005: Verify-before-write, atomic file writes
- **Status:** Accepted
- **Context:** Partial or unverified output may be submitted by operators
  and lock funds.
- **Decision:** `deposit.Generator` re-verifies every signature; `output`
  writes to a temp file and renames. Any failure aborts the run with no
  output file.
- **Consequences:** No partial-progress mode; users re-run from scratch on
  failure. Acceptable given seconds-scale runtime.

---

## Open Questions

- Should the `WithdrawalCredentials` for v1 be hard-coded (e.g., a specific
  operator address) or accepted as an additional flag? The PRD lists
  per-entry overrides as out-of-scope but is silent on a single global
  `--withdrawal-address`. Flagging as a likely first follow-up.
- `CLIVersion` string: pin to a specific `staking-deposit-cli` release (e.g.
  `"2.7.0"`) or expose as a build-time `-ldflags -X` variable so release
  engineering can bump without code changes?
- Should we ship a `--verify-only` mode now (re-checks an existing deposit
  JSON without re-signing)? The orchestrator design already supports it for
  free — the question is whether to expose it in v1's CLI.

## Risks

| Risk                                            | Mitigation                                                                 |
|------------------------------------------------|----------------------------------------------------------------------------|
| Hand-rolled SSZ diverges from spec               | Golden-file tests vs `staking-deposit-cli` fixtures in CI; spec test vecs. |
| CGO build complications across release matrix   | GoReleaser config + CI matrix exercises every target before tagging.       |
| Operator passes mainnet keystore + hoodi flag  | Per-entry pubkey-match check + confirmation banner printed before signing. |
| Passphrase leakage                              | No flag accepts it; logs scrub paths only; key zeroized after use.         |
| Stale `deposit_cli_version` rejected by Launchpad| Re-run golden-file tests against each upstream CLI release; bump string.   |
