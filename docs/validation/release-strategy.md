# Release Strategy — eth-deposit-gen v1.0

## CGO Strategy: native-runner matrix (CI) + zig cc (local snapshot)

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

Because the `.a` is already compiled for each target, the C compiler only
needs to link — no C source compilation is required.

### CI release builds (#21)

The release workflow uses a native-runner matrix:
- `macos-latest` runner builds `darwin/amd64` and `darwin/arm64`
- `ubuntu-latest` runner builds `linux/amd64` and `linux/arm64`

Each runner uses the host `cc`. Static archives from `herumi/bls-eth-go-binary`
are fetched automatically via `go mod download`. This avoids any zig version
pinning and any musl-vs-glibc tradeoffs in CI.

The `.goreleaser.yaml` linux build configs read the `CC_FOR_LINUX_AMD64` and
`CC_FOR_LINUX_ARM64` env vars (defaulting to `cc` when unset), so CI with no
special vars automatically uses the native host compiler.

### Local snapshot builds

`zig cc` (Zig 0.16) is used as the cross-linker for Linux targets when
building a snapshot on a macOS developer machine. Run `make snapshot` from the
repo root, which exports:

- `CC_FOR_LINUX_AMD64="zig cc -target x86_64-linux-musl"`
- `CC_FOR_LINUX_ARM64="zig cc -target aarch64-linux-musl"`

Zig ships pre-built musl-based toolchains (`brew install zig`). The resulting
Linux binaries are statically linked against musl libc.

Darwin targets are always built natively by the host macOS compiler.

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

`make snapshot` (`goreleaser release --snapshot --clean` with zig cc env vars)
ran successfully on 2026-05-16 on a macOS arm64 host
(Darwin 25.5.0, goreleaser v2.15.4, zig 0.16.0).

Output archives produced:

```
dist/eth-deposit-gen_darwin_amd64.tar.gz
dist/eth-deposit-gen_darwin_arm64.tar.gz
dist/eth-deposit-gen_linux_amd64.tar.gz
dist/eth-deposit-gen_linux_arm64.tar.gz
dist/checksums.txt
```

Each archive contains: binary, README.md, LICENSE.

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

Run on 2026-05-16. Image: `ubuntu:22.04`. Docker `29.3.1` on macOS arm64 host.
Binary extracted from `dist/eth-deposit-gen_linux_amd64.tar.gz` (statically
linked musl ELF — no shared library dependencies in the container).

```
$ docker run --rm -v /tmp/eth-smoke-linux-amd64:/smoke ubuntu:22.04 \
    /smoke/eth-deposit-gen --version
eth-deposit-gen version v0.0.0-snapshot (commit=8028d7fc4da026c21e1881db1811406578066e34, built=2026-05-16T12:50:56Z)
```

Exit code: 0

```
$ docker run --rm -v /tmp/eth-smoke-linux-amd64:/smoke ubuntu:22.04 \
    /smoke/eth-deposit-gen --help
NAME:
   eth-deposit-gen - Generate Launchpad-compatible deposit_data JSON for existing BLS validator keys

USAGE:
   eth-deposit-gen --keystore-dir DIR --pubkeys HEX[,...] --network NET --output-dir DIR [--passphrase-env VAR]

DESCRIPTION:
   Produces deposit_data-<ts>.json for one or more BLS validator public keys by
signing each deposit message with the BLS key loaded from an EIP-2335 keystore.
Output is byte-for-byte compatible with the official ethereum/staking-deposit-cli.

OPTIONS:
   --keystore-dir value            Directory containing EIP-2335 JSON keystore files, one per validator (e.g. ./keystores/)
   --pubkeys value                 Comma-separated BLS public keys in 96-hex-char form (0x-prefixed or bare)
   --network value                 Ethereum consensus network: "mainnet" or "hoodi"
   --output-dir value              Existing, writable directory for the output deposit_data-<ts>.json file
   --passphrase-env value          Name of the environment variable holding the keystore passphrase (omit for TTY prompt)
   --i-understand-this-is-mainnet  Required when --network mainnet (default: false)
   --dry-run                       Print deposit JSON to stdout instead of writing to disk (default: false)
   --verbose                       Enable debug-level structured logging to stderr (default: false)
   --json-logs                     Emit logs as JSON objects (default: false)
   --parallel value                Number of concurrent signing workers (1–16); values ≤0 or >16 are rejected (default: 1)
   --verify-with-deposit-cli       Cross-check output with staking-deposit-cli (default: false)
   --deposit-cli-path value        Path to staking-deposit-cli binary (default: "deposit")
   --help, -h                      show help
   --version, -v                   print the version
```

Exit code: 0

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
