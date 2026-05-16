# Release Strategy — eth-deposit-gen v1.0

## CGO Strategy: zig cc cross-compilation (local) + native runners (CI)

`eth-deposit-gen` requires `CGO_ENABLED=1` because it depends on
`github.com/herumi/bls-eth-go-binary`, which wraps a prebuilt static C
archive (`libbls384_256.a`). The module ships these archives pre-built for
each target inside the Go module itself:

```
bls/lib/darwin/amd64/libbls384_256.a
bls/lib/darwin/arm64/libbls384_256.a
bls/lib/linux/amd64/libbls384_256.a
bls/lib/linux/arm64/libbls384_256.a
```

Because the `.a` is already compiled for each target, the only requirement is
a C linker for the target OS/arch — no C source compilation is needed.

### Local snapshot builds

`zig cc` (Zig 0.16) is used as the cross-linker for Linux targets when
building the snapshot on a macOS developer machine:

- `linux/amd64`: `CC="zig cc -target x86_64-linux-musl"`
- `linux/arm64`: `CC="zig cc -target aarch64-linux-musl"`

Zig ships pre-built musl-based toolchains as part of its standard
distribution (`brew install zig`). No pinning or special setup is needed.
The resulting Linux binaries are statically linked (musl libc).

Darwin targets are built natively by the host macOS compiler (`cc`).

### CI release builds (#21)

For the release workflow, the native-runner matrix is used instead:
- `macos-latest` runner builds `darwin/amd64` and `darwin/arm64`
- `ubuntu-latest` runner builds `linux/amd64` and `linux/arm64`

This uses the host C compiler on each runner, avoiding any zig dependency in
the CI environment. Static archives from `herumi/bls-eth-go-binary` are
fetched automatically via `go mod download`.

### Why not zig cc exclusively in CI?

The native-runner approach is simpler to debug in CI (standard compiler
errors, no zig version pinning, no musl-vs-glibc tradeoffs on the runner).
Zig `0.16` introduced some linker behavior changes; locking a zig version in
CI creates a maintenance burden. For a local snapshot on a developer machine,
zig is convenient and proven to work.

---

## Target Matrix

| OS      | Arch  | Strategy (local snapshot) | Strategy (CI release) |
|---------|-------|---------------------------|-----------------------|
| darwin  | amd64 | host `cc`                 | `macos-latest` runner |
| darwin  | arm64 | host `cc`                 | `macos-latest` runner |
| linux   | amd64 | `zig cc x86_64-linux-musl`| `ubuntu-latest` runner|
| linux   | arm64 | `zig cc aarch64-linux-musl`| `ubuntu-latest` runner|

Windows is excluded. `herumi/bls-eth-go-binary` does ship
`windows/amd64/libbls384_256.a`, but the operator use case is Linux/macOS
servers only and there is no Windows CI runner configured.

---

## Smoke Test Evidence

### Local snapshot build

`goreleaser release --snapshot --clean` ran successfully on 2026-05-16 on a
macOS arm64 host (Darwin 25.5.0, goreleaser v2.15.4, zig 0.16.0).

Output archives produced:

```
dist/eth-deposit-gen_darwin_amd64.tar.gz
dist/eth-deposit-gen_darwin_arm64.tar.gz
dist/eth-deposit-gen_linux_amd64.tar.gz
dist/eth-deposit-gen_linux_arm64.tar.gz
dist/checksums.txt
```

checksums.txt:
```
b0e2e3509f82f2d41d7905045a098c781e1395af6a9cf6865fd58ee3cea9cdec  eth-deposit-gen_darwin_amd64.tar.gz
b01cc6dbc24925e8ee96cf08f81319f12a62bee6e2ece69f043b042e8734ce6c  eth-deposit-gen_darwin_arm64.tar.gz
dee939a02cc9baab4a855e7b6edb731ebc1c2bb5d40a265fa72ab05512ef84dc  eth-deposit-gen_linux_amd64.tar.gz
a3fa46460a7ef82f5eab910bc83fe1cf5a946f27d1f6c4d48e00c470458bef53  eth-deposit-gen_linux_arm64.tar.gz
```

### darwin/arm64 smoke test (native, run on the build host)

```
$ dist/darwin_darwin_arm64_v8.0/eth-deposit-gen --version
eth-deposit-gen version v0.0.0-snapshot (commit=1eafab2985acf73822371d04d585a11611c1f627, built=2026-05-16T12:44:17Z)
```

```
$ dist/darwin_darwin_arm64_v8.0/eth-deposit-gen --help
NAME:
   eth-deposit-gen - Generate Launchpad-compatible deposit_data JSON for existing BLS validator keys

USAGE:
   eth-deposit-gen --keystore-dir DIR --pubkeys HEX[,...] --network NET --output-dir DIR [--passphrase-env VAR]

OPTIONS:
   --keystore-dir value            Directory containing EIP-2335 JSON keystore files, one per validator
   --pubkeys value                 Comma-separated BLS public keys in 96-hex-char form (0x-prefixed or bare)
   --network value                 Ethereum consensus network: "mainnet" or "hoodi"
   --output-dir value              Existing, writable directory for the output deposit_data-<ts>.json file
   --passphrase-env value          Name of the environment variable holding the keystore passphrase
   --i-understand-this-is-mainnet  Required when --network mainnet (default: false)
   --dry-run                       Print deposit JSON to stdout instead of writing to disk (default: false)
   --verbose                       Enable debug-level structured logging to stderr (default: false)
   --json-logs                     Emit logs as JSON objects (default: false)
   --parallel value                Number of concurrent signing workers (default: 1)
   --verify-with-deposit-cli       Cross-check output with staking-deposit-cli (default: false)
   --deposit-cli-path value        Path to staking-deposit-cli binary (default: "deposit")
   --help, -h                      show help
   --version, -v                   print the version
```

### linux/amd64 Docker smoke test

**DEFERRED**: Docker is not available in the local build environment. See the
VM handoff runbook below for the exact commands to run.

The linux/amd64 binary (`dist/linux-amd64_linux_amd64_v1/eth-deposit-gen`)
is a statically linked ELF (musl libc), verified via:

```
$ file dist/linux-amd64_linux_amd64_v1/eth-deposit-gen
ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, ...
```

---

## VM Handoff Runbook

These commands must be run by the engineer doing the final v1.0.0 release
verification (Issue #24). Record the output in this doc before tagging.

### linux/amd64 Docker smoke test

```bash
# From repo root after running goreleaser release --snapshot --clean
tar -xzf dist/eth-deposit-gen_linux_amd64.tar.gz -C /tmp/linux-amd64-smoke/

docker run --rm \
  -v /tmp/linux-amd64-smoke:/smoke \
  ubuntu:22.04 \
  /smoke/eth-deposit-gen --version

docker run --rm \
  -v /tmp/linux-amd64-smoke:/smoke \
  ubuntu:22.04 \
  /smoke/eth-deposit-gen --help
```

Expected: `--version` exits 0 and prints version string; `--help` exits 0 and
prints usage text.

### linux/arm64 VM smoke test

On a linux/arm64 VM or AWS Graviton instance:

```bash
tar -xzf eth-deposit-gen_linux_arm64.tar.gz
./eth-deposit-gen --version
./eth-deposit-gen --help
```

### darwin/amd64 smoke test

On an Intel Mac (or Rosetta 2):

```bash
tar -xzf eth-deposit-gen_darwin_amd64.tar.gz
./eth-deposit-gen --version
./eth-deposit-gen --help
```

### darwin/arm64 smoke test

On an Apple Silicon Mac:

```bash
tar -xzf eth-deposit-gen_darwin_arm64.tar.gz
./eth-deposit-gen --version
./eth-deposit-gen --help
```

### Functional smoke test (all platforms)

```bash
# Requires a keystore directory and matching pubkeys from testdata/hoodi/
./eth-deposit-gen \
  --network hoodi \
  --dry-run \
  --keystore-dir <path-to-keystores> \
  --pubkeys <pubkey-hex> \
  --output-dir /tmp
```

Verify the output JSON matches the expected deposit data field-for-field.
