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

The release workflow uses a two-tier strategy:

**Test jobs — native runner matrix (3 jobs run in parallel):**
- `linux/amd64` — `ubuntu-latest` runner, host gcc, `CGO_ENABLED=1`
- `linux/arm64` — `ubuntu-24.04-arm` runner (native ARM, free for public repos
  since GitHub's 2025 rollout), host gcc, `CGO_ENABLED=1`
- `darwin/arm64` — `macos-14` runner (Apple M-series), Xcode clang, `CGO_ENABLED=1`

**linux/arm64 runner choice rationale:** `ubuntu-24.04-arm` is a GitHub-hosted
native ARM64 runner available on public repositories at no charge (announced
Feb 2025). It avoids QEMU overhead and gives a real ARM kernel for tests.
Fallback (if plan/billing changes): replace `ubuntu-24.04-arm` with
`ubuntu-latest` and add `docker/setup-qemu-action@v3` + run tests inside an
`arm64` QEMU container. The fallback is documented as commented-out YAML in
`.github/workflows/release.yml`.

**Release build job — goreleaser-cross container on ubuntu-latest:**
The goreleaser job runs in the `ghcr.io/goreleaser/goreleaser-cross:v1.25.9`
Docker container (Go 1.25, goreleaser v2, full cross-toolchain). This single
container image bundles osxcross (darwin) and GNU cross-compilers (linux/arm64),
enabling all 4 target archives from one `ubuntu-latest` runner:
- `darwin/amd64` → `o64-clang` (osxcross)
- `darwin/arm64` → `oa64-clang` (osxcross)
- `linux/amd64`  → `x86_64-linux-gnu-gcc`
- `linux/arm64`  → `aarch64-linux-gnu-gcc`

The `.goreleaser.yaml` build configs read `CC_FOR_DARWIN_AMD64`,
`CC_FOR_DARWIN_ARM64`, `CC_FOR_LINUX_AMD64`, and `CC_FOR_LINUX_ARM64` env vars,
defaulting to `cc` when unset. CI sets all four; local `make snapshot` sets
only the linux vars (using zig cc) and leaves darwin to the host Xcode clang.

The native-runner matrix in the strategy doc refers to **test jobs only**. The
actual release build (goreleaser) runs on a single runner with cross-compile
toolchains, not per-OS native runners. This distinction is intentional: tests
validate behaviour on real OS/arch, while goreleaser-cross builds all archives
from one reproducible container.

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
| darwin  | amd64 | host `cc`                 | `goreleaser-cross` container |
| darwin  | arm64 | host `cc`                 | `goreleaser-cross` container |
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

---

## Release CI dry-run

This runbook describes how to exercise the release workflow end-to-end without
publishing a real release. Perform this before tagging v1.0.0 (Issue #24).

### Prerequisites

- You have push access to the repository (or a fork where Actions runs).
- The branch under test has `.github/workflows/release.yml` present.
- `GITHUB_TOKEN` is the only secret required — it is injected automatically by
  GitHub Actions; no additional secrets need to be configured.

### Steps

1. **Ensure the branch is pushed to the remote:**
   ```bash
   git push origin feat/issue-21-release-ci
   ```

2. **Create and push the test tag from that branch:**
   ```bash
   git tag v0.0.0-rc1
   git push origin v0.0.0-rc1
   ```
   The `release.yml` workflow triggers on `push` of tags matching `v*`.
   This tag matches and will start the pipeline.

3. **Monitor the workflow run:**
   ```bash
   gh run list --workflow=release.yml --limit=5
   gh run watch   # select the run for v0.0.0-rc1
   ```

4. **Verify the pre-release on GitHub:**
   - Navigate to: `https://github.com/rootwarp/eth-utils/releases`
   - A pre-release tagged `v0.0.0-rc1` should appear with all expected assets:
     - `eth-deposit-gen_darwin_amd64.tar.gz`
     - `eth-deposit-gen_darwin_arm64.tar.gz`
     - `eth-deposit-gen_linux_amd64.tar.gz`
     - `eth-deposit-gen_linux_arm64.tar.gz`
     - `checksums.txt`
     - `eth-deposit-gen_darwin_amd64.sbom.spdx.json`
     - `eth-deposit-gen_darwin_arm64.sbom.spdx.json`
     - `eth-deposit-gen_linux_amd64.sbom.spdx.json`
     - `eth-deposit-gen_linux_arm64.sbom.spdx.json`
   - `.goreleaser.yaml` sets `release.prerelease: auto`, so `v0.0.0-rc1`
     (pre-release semver) will be marked as a GitHub pre-release, not a draft.
     If you want an explicit draft for the dry-run, temporarily set
     `release.draft: true` in `.goreleaser.yaml` before pushing the tag.

5. **Download and smoke-test one binary:**
   ```bash
   gh release download v0.0.0-rc1 --pattern 'eth-deposit-gen_linux_amd64.tar.gz'
   tar -xzf eth-deposit-gen_linux_amd64.tar.gz
   ./eth-deposit-gen --version
   ./eth-deposit-gen --help
   ```

6. **Verify the SBOM:**
   ```bash
   gh release download v0.0.0-rc1 --pattern 'eth-deposit-gen_linux_amd64.sbom.spdx.json'
   python3 -c "import json; d=json.load(open('eth-deposit-gen_linux_amd64.sbom.spdx.json')); print(d['spdxVersion'])"
   ```
   Expected: `SPDX-2.3` (or similar version string).

7. **Verify checksums:**
   ```bash
   gh release download v0.0.0-rc1 --pattern 'checksums.txt'
   sha256sum -c checksums.txt 2>/dev/null || shasum -a 256 -c checksums.txt
   ```

8. **Clean up the test tag and release after verification:**
   ```bash
   gh release delete v0.0.0-rc1 --yes
   git push origin :v0.0.0-rc1   # delete the remote tag
   git tag -d v0.0.0-rc1         # delete the local tag
   ```

### What constitutes a passing dry-run

- All three pre-release test jobs (linux/amd64, linux/arm64, darwin/arm64)
  complete green.
- The goreleaser job completes and uploads all 4 archives + checksums.txt.
- The SBOM steps attach at least one `.spdx.json` per platform.
- No long-lived PAT or external secret is referenced in any workflow log.
- Record the run URL and asset list in this doc under "Dry-run evidence" before
  proceeding to Issue #24.
