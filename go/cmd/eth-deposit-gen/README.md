# eth-deposit-gen

Generate Launchpad-compatible `deposit_data` JSON for existing BLS validator keys.

## Install

### From source (requires CGO toolchain)

`eth-deposit-gen` links against `herumi/bls-eth-go-binary` prebuilt static archives. CGO must be enabled.

```console
CGO_ENABLED=1 go install github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen@latest
```

On macOS, `CGO_ENABLED=1` is the default. On Linux you need `gcc` or `clang` in `PATH`.

If you see linker errors like `undefined reference to ...`, see [Troubleshooting: Missing CGO toolchain](#missing-cgo-toolchain-undefined-reference-errors).

### Pre-built binary

Download a binary for your platform from the [GitHub releases page](https://github.com/rootwarp/eth-utils/releases/latest).

Binaries are available for:

- `darwin/amd64`, `darwin/arm64`
- `linux/amd64`, `linux/arm64`

Verify the download against `checksums.txt` (shipped alongside each release archive).

## Quickstart

### Hoodi testnet

```bash
export KEYSTORE_PASS=my-keystore-passphrase

eth-deposit-gen \
  --network hoodi \
  --keystore-dir ./keystores/ \
  --pubkeys 0x8420760d0de00ed65f290ab2122e65933e168539ad261b5e444a5094c649272527a1509dd105a801922c359e46e33fb9 \
  --output-dir ./out \
  --passphrase-env KEYSTORE_PASS
```

Example output (stderr + stdout):

```text
eth-deposit-gen: network=hoodi first_pubkey=0x8420... last_pubkey=0x8420... count=1
wrote ./out/deposit_data-1716000000.json (sha256=..., n=1, network=hoodi)
```

To preview without writing a file:

```bash
export KEYSTORE_PASS=my-keystore-passphrase

eth-deposit-gen \
  --network hoodi \
  --keystore-dir ./keystores/ \
  --pubkeys 0x8420760d0de00ed65f290ab2122e65933e168539ad261b5e444a5094c649272527a1509dd105a801922c359e46e33fb9 \
  --output-dir ./out \
  --passphrase-env KEYSTORE_PASS \
  --dry-run
```

The JSON is printed to stdout; no file is created.

### Mainnet

Mainnet deposits are **irreversible**. Pass `--i-understand-this-is-mainnet` to acknowledge.

Attempting `--network mainnet` without the flag exits with code 2:

```text
mainnet selected; pass --i-understand-this-is-mainnet to acknowledge
```

Correct invocation:

```bash
export KEYSTORE_PASS=my-keystore-passphrase

eth-deposit-gen \
  --network mainnet \
  --i-understand-this-is-mainnet \
  --keystore-dir ./keystores/ \
  --pubkeys 0x<your-validator-pubkey> \
  --output-dir ./out \
  --passphrase-env KEYSTORE_PASS
```

## Flag reference

```text
NAME:
   eth-deposit-gen - Generate Launchpad-compatible deposit_data JSON for existing BLS validator keys

USAGE:
   eth-deposit-gen --keystore-dir DIR --pubkeys HEX[,...] --network NET --output-dir DIR [--passphrase-env VAR]

OPTIONS:
   --keystore-dir value            Directory containing EIP-2335 JSON keystore files, one per validator (e.g. ./keystores/)
   --pubkeys value                 Comma-separated BLS public keys in 96-hex-char form (0x-prefixed or bare)
   --network value                 Ethereum consensus network: "mainnet" or "hoodi"
   --output-dir value              Existing, writable directory for the output deposit_data-<ts>.json file
   --passphrase-env value          Name of the environment variable holding the keystore passphrase (omit for TTY prompt)
   --i-understand-this-is-mainnet  Required when --network mainnet: acknowledges this produces REAL mainnet deposit data with irreversible financial consequences (default: false)
   --dry-run                       Print the deposit JSON to stdout instead of writing a file to --output-dir; no file is created. The sha256 on stderr matches the bytes written to stdout. (default: false)
   --verbose                       Enable debug-level structured logging to stderr (default: false)
   --json-logs                     Emit logs as JSON objects instead of human-readable text (default: false)
   --parallel value                Number of concurrent signing workers (1–runtime.NumCPU()×4); values ≤0 or above the maximum are rejected (default: 1)
   --verify-with-deposit-cli       After writing the deposit JSON, run the installed staking-deposit-cli to cross-check the output file (requires staking-deposit-cli >= 2.7.0; see --deposit-cli-path). Skipped in --dry-run mode. Off by default. (default: false)
   --deposit-cli-path value        Name or absolute path of the staking-deposit-cli binary used for --verify-with-deposit-cli (minimum supported version: 2.7.0). Defaults to "deposit" (looked up in PATH). (default: "deposit")
   --help, -h                      show help
   --version, -v                   print the version
```

Run `eth-deposit-gen --help` to see these flags in your terminal.

### Version

```bash
eth-deposit-gen --version
```

Example output (release binary):

```text
eth-deposit-gen version 1.0.0 (commit=abc1234, built=2026-01-15T12:00:00Z)
```

### Parallel signing

For large batches of validators, use `--parallel` to sign concurrently:

```bash
export KEYSTORE_PASS=my-keystore-passphrase

eth-deposit-gen \
  --network hoodi \
  --keystore-dir ./keystores/ \
  --pubkeys 0x<pk1>,0x<pk2>,0x<pk3> \
  --output-dir ./out \
  --passphrase-env KEYSTORE_PASS \
  --parallel 4
```

`--parallel` accepts values from 1 to `runtime.NumCPU()×4`. Values outside that range exit with code 2.

### Cross-checking with staking-deposit-cli

If you have `staking-deposit-cli` >= 2.7.0 installed, pass `--verify-with-deposit-cli` to run a post-generation verification step. This shells out to the `deposit verify --input-file` subcommand:

```bash
export KEYSTORE_PASS=my-keystore-passphrase

eth-deposit-gen \
  --network hoodi \
  --keystore-dir ./keystores/ \
  --pubkeys 0x<your-validator-pubkey> \
  --output-dir ./out \
  --passphrase-env KEYSTORE_PASS \
  --verify-with-deposit-cli
```

If the `deposit` binary (or the binary named by `--deposit-cli-path`) is not found in PATH, the command exits with code 2. If verification fails, it exits with code 3.

## Security notes

**Passphrase is never accepted as a CLI flag.**

Accepting a passphrase as a flag value would expose it in:
- shell history (`~/.bash_history`, `~/.zsh_history`)
- process listings (`ps auxe`, `/proc/<pid>/cmdline`)

Two safe sourcing modes are supported:

1. **`--passphrase-env <NAME>`** — reads the passphrase from the named environment variable. Recommended for scripts and CI pipelines where the secret is injected by a secrets manager.

   ```console
   read -rs -p "Passphrase: " KEYSTORE_PASS
   export KEYSTORE_PASS
   eth-deposit-gen ... --passphrase-env KEYSTORE_PASS
   unset KEYSTORE_PASS
   ```

2. **Interactive TTY prompt** (default, when `--passphrase-env` is omitted) — prompts on stderr with echo disabled. Suitable for interactive desktop use.

**The decrypted BLS secret key is zeroized immediately after the deposit signature is produced.** It is held in memory only for the duration of the signing operation.

## Exit codes

| Code | Name / sentinel | Trigger condition |
|------|-----------------|-------------------|
| 0 | — (success) | All deposit entries written successfully |
| 1 | (fallback) | Unclassified error (e.g. output file write failed) |
| 2 | `ErrKeystoreMissing`, `ErrKeystoreMalformed`, `ErrKeystoreVersion`, `ErrEnvVarEmpty`, `ErrKeystoreNotFound`, `ErrPubkeyMismatch`, `errMainnetAckRequired`, `ErrDepositCLINotFound`, urfave/cli validation | Invalid flags, missing/malformed keystore, pubkey not found in keystore dir, `--network mainnet` without `--i-understand-this-is-mainnet`, `--verify-with-deposit-cli` binary not found in PATH |
| 3 | `ErrWrongPassphrase`, `ErrSelfVerifyFailed`, `errBLSInit`, `ErrDepositCLIFailed` | Wrong keystore passphrase, internal BLS self-verification failure, BLS library init failure, staking-deposit-cli cross-check returned non-zero |
| 4 | `context.Canceled` | SIGINT / Ctrl-C (user abort) |

## Troubleshooting

### Wrong passphrase

```text
wrong passphrase: <underlying crypto error>
```

The passphrase supplied via `--passphrase-env` or the TTY prompt does not decrypt the keystore. Exits with code 3.

Check which environment variable you passed and verify its value:

```bash
echo "passphrase length: ${#KEYSTORE_PASS}"
```

See `docs/research/keystore-decoding.md` for details on how the keystore checksum detects a bad passphrase before any signing work occurs.

### Missing CGO toolchain (`undefined reference` errors)

```text
cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in $PATH
```

or during `go install`:

```text
./cgo_helpers.go:7:8: could not import C (cannot find cgo)
```

`eth-deposit-gen` depends on `herumi/bls-eth-go-binary`, which requires CGO to link prebuilt BLS static archives. Install a C toolchain:

- **macOS**: `xcode-select --install`
- **Ubuntu/Debian**: `sudo apt-get install build-essential`
- **Alpine**: `apk add build-base`

Then rebuild or re-install with `CGO_ENABLED=1`.

### `--output-dir` not writable

```text
--output-dir: directory "/path/to/out" is not writable: open /path/to/out/.eth-deposit-gen-probe-...: permission denied
```

The tool probes writability by creating and immediately removing a temp file in `--output-dir`. Exits with code 2.

Fix the directory permissions or point `--output-dir` at a writable path:

```bash
mkdir -p ./out
chmod u+w ./out
```

## For contributors

- [Architecture overview](../../../docs/architecture.md)
- [Product requirements](../../../docs/prd.md)
- [Project plan](../../../docs/project-plan.md)
- Research notes:
  - [BLS and SSZ libraries](../../../docs/research/bls-ssz-libraries.md)
  - [Deposit spec](../../../docs/research/deposit-spec.md)
  - [Keystore decoding](../../../docs/research/keystore-decoding.md)
