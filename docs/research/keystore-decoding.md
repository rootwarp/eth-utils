# Research: EIP-2335 Keystore Decoding in Go

The deposit generator loads the validator's BLS signing key from an EIP-2335 JSON keystore file passed via `--validator-key-path`. This document describes the keystore format, the recommended Go library, and how the decryption passphrase should be sourced.

## Library Recommendation

**`github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4`**

- Implements EIP-2335 keystore v4 — the same format produced by `staking-deposit-cli` and by Prysm / Lighthouse / Teku / Nimbus exporters.
- Supports both KDFs allowed by the spec: **`scrypt`** and **`PBKDF2`**.
- The `Decrypt(crypto, passphrase) ([]byte, error)` API returns the raw 32-byte BLS secret key on success and a clear error on bad passphrase / corrupted file.
- Well-maintained; widely used (notably by `wealdtech/ethdo`, a battle-tested CLI in the Ethereum validator-tooling ecosystem).
- Pure Go, no CGO — installs and cross-compiles cleanly.

There are a handful of alternatives (Prysm's internal keystore code, Lighthouse's Go translation, custom rolling) but the wealdtech package is the path of least resistance and the most directly aligned with the format `staking-deposit-cli` emits.

The 32 raw bytes returned by `Decrypt` are then loaded into the BLS library (`herumi/bls-eth-go-binary`) via `SecretKey.Deserialize(bytes)` to become a usable signing key.

## EIP-2335 Keystore JSON Structure

```json
{
  "crypto": {
    "kdf": {
      "function": "scrypt",
      "params": { "dklen": 32, "n": 262144, "r": 8, "p": 1, "salt": "<hex>" },
      "message": ""
    },
    "checksum": {
      "function": "SHA256",
      "params": {},
      "message": "<hex>"
    },
    "cipher": {
      "function": "aes-128-ctr",
      "params": { "iv": "<hex>" },
      "message": "<hex>"
    }
  },
  "pubkey": "<96 hex chars, no 0x>",
  "path": "m/12381/3600/0/0/0",
  "uuid": "<uuid>",
  "version": 4
}
```

Notes on the fields:

- `crypto.kdf.function` is either `scrypt` (default for `staking-deposit-cli`) or `pbkdf2`. The wealdtech library autoselects based on this string.
- `crypto.checksum.message` is `sha256(decryption_key[16:32] ++ cipher.message)`. The encryptor verifies this before attempting decryption — a wrong passphrase fails fast on the checksum, not on producing garbage key bytes.
- `crypto.cipher.message` is the AES-128-CTR ciphertext of the BLS secret key, keyed by `decryption_key[0:16]`.
- `pubkey` is the 48-byte BLS G1 compressed public key (96 lowercase hex chars, **no** `0x` prefix). This must match the pubkey derived from the decrypted secret key — the tool should assert this as a sanity check before signing.
- `path` is the EIP-2334 derivation path (`m/12381/3600/<index>/0/0` for validator signing keys). Informational only — the tool does not need to use it.
- `uuid` is a v4 UUID identifying the keystore (not the validator).
- `version` must be `4`.

## Passphrase Sourcing

The keystore is encrypted; we need a passphrase to decrypt. Per the PRD's security NFR, the passphrase must **never** be accepted as a direct CLI flag value — flag values land in shell history (`~/.bash_history`, `~/.zsh_history`) and in process listings (`ps auxe`), both of which are routinely scraped on shared hosts and snapshotted by ops tooling.

Support two safe sourcing modes:

### 1. `--passphrase-env <VAR_NAME>` flag

The user supplies the *name* of an environment variable, not the passphrase itself. The tool reads `os.Getenv(varName)`. Example:

```bash
read -s -p "Passphrase: " VALIDATOR_PW
export VALIDATOR_PW
eth-deposit-gen --validator-key-path ./keystore.json \
                --passphrase-env VALIDATOR_PW \
                --network hoodi \
                --pubkeys 0x... \
                --output-dir ./out
unset VALIDATOR_PW
```

This is the right mode for scripts and CI pipelines, where the secret is injected by a secrets manager (`aws-vault exec`, Vault agent, GitHub Actions secret) into the environment of the process — never the command line.

### 2. Secure stdin prompt (default fallback)

When `--passphrase-env` is not provided, prompt interactively via `golang.org/x/term`:

```go
import "golang.org/x/term"

fmt.Fprint(os.Stderr, "Keystore passphrase: ")
pw, err := term.ReadPassword(int(os.Stdin.Fd()))
fmt.Fprintln(os.Stderr) // newline after the (suppressed) input
```

`term.ReadPassword` disables terminal echo, so the passphrase is never displayed and never recorded in line-editing history. This is the right mode for interactive use on a desktop / laptop. The prompt is written to stderr (not stdout) so that redirecting stdout — e.g., piping the tool's structured logs to a file — does not swallow the prompt.

### Handling the passphrase in memory

- Decode the keystore and decrypt as the first thing the tool does after argument parsing; hold the decrypted 32-byte BLS key for as little time as possible.
- After signing the entire batch, zeroize both the passphrase byte slice and the decrypted key bytes (`for i := range buf { buf[i] = 0 }`). Go's garbage collector does not zero buffers; this must be explicit.
- Never log the passphrase, the decrypted key, or the encrypted keystore contents — not even at `--verbose`. Structured logging fields for the keystore should be limited to the file path and (optionally) the pubkey field from the JSON.

### What we explicitly do not support

- A `--passphrase <value>` flag of any kind. Even with redaction in our own logs, the kernel still captures the value in `/proc/<pid>/cmdline` and the user's shell history. The PRD's security NFR is explicit about this.
- Reading the passphrase from a file path passed on the CLI. Plausible but adds another file-handling code path; if a user wants file-based sourcing they can do `--passphrase-env VAR` with `VAR=$(cat passphrase.txt)`.
