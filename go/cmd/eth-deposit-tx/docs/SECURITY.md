# eth-deposit-tx — Security Guide

## Threat model

`eth-deposit-tx` handles private keys that control ETH balances. Its job is narrow: read an unsigned deposit transaction from disk (or stdin), sign it with a secp256k1 private key, and write a signed transaction. The following properties are explicitly in scope:

**In scope (enforced by this tool):**
- Private key material never appears in CLI flags, log output, error messages, or stdout.
- Key bytes are zeroized in memory immediately after signing.
- Output files containing key-derived material (`signed.json`, `signed.raw`) are written with `0o600` permissions (owner read/write only).
- Exit codes are deterministic sentinels; the tool never silently swallows errors.
- Chain ID is verified against the RPC node before broadcast to prevent cross-network replay.
- The `send` command requires explicit double-confirmation (type the network name) before broadcasting.

**Not in scope (caller's responsibility):**
- Protecting the private key at rest or in the environment variable.
- Securing the machine that runs `sign` or `run`.
- Verifying that the deposit data itself is correct (correct pubkey, withdrawal credentials, etc.) — use `eth-deposit-gen` and cross-check with `staking-deposit-cli`.
- Rate limiting or MEV protection for the broadcast transaction.

---

## Key handling

### ETH_DEPOSIT_TX_PRIVATE_KEY

The private key is read exclusively from the environment variable named by `--private-key-env` (default: `ETH_DEPOSIT_TX_PRIVATE_KEY`). It is never accepted as a CLI argument.

**Why this matters:** CLI arguments are visible in:
- Shell history (`~/.bash_history`, `~/.zsh_history`)
- Process listings (`ps auxe`, `/proc/<pid>/cmdline`)
- CI logs that echo `$@`

Load the key into the environment securely:

```bash
# Interactive (TTY): read without echo
read -rs -p "Private key (0x...): " ETH_DEPOSIT_TX_PRIVATE_KEY
export ETH_DEPOSIT_TX_PRIVATE_KEY

# ... sign or run ...

# Unset immediately after use
unset ETH_DEPOSIT_TX_PRIVATE_KEY
```

In CI, inject the key from a secrets manager (e.g., GitHub Actions secrets, Vault) rather than hard-coding it in the pipeline definition.

### Key zeroization

The key bytes are explicitly overwritten with zeros in memory when `LocalSigner.Close()` is called. The `defer s.Close()` in `signAction` ensures this happens even if signing fails.

### The key must never leave the environment variable

If you see `--private-key-env` being passed an `0x`-prefixed hex string, you are doing it wrong. That flag takes a variable *name* (e.g., `ETH_DEPOSIT_TX_PRIVATE_KEY`), not the key value. The tool validates this with a POSIX env-var name regex and exits with code 2 if it looks like a key.

---

## Ledger hardware wallet — primary signer

**The Ledger signer is the recommended path for all operations involving real funds.**

The private key never leaves the device. The signing operation happens on the device hardware; only the signature (r, s, v) is returned to the host.

### Prerequisites

1. Connect the Ledger device via USB.
2. Unlock the device with your PIN.
3. Open the **Ethereum** app on the device.

The tool derives the sender address at BIP-44 path `m/44'/60'/0'/0/0`. The address is displayed both in the tool output and on the Ledger screen — verify they match before confirming.

### What to verify on the Ledger screen before pressing confirm

Before pressing the confirm button, verify the following fields on the device screen:

| Field | Expected |
|-------|----------|
| **Chain ID** | The network's chain ID (e.g., 17000 for Holesky, 1 for mainnet) |
| **To address** | The deposit contract address for the network (e.g., `0x4242...4242` for Holesky) |
| **Value** | 32.000000 ETH (32 ETH per validator) |
| **Sender** | Your funding address |

If any of these values are wrong, press **Reject** and investigate before retrying.

### Device rejection

Pressing **Reject** on the device, or a timeout, exits the tool with code 4. No transaction is written. This is the correct behavior — rejection is always safe.

---

## LocalSigner — development and testing only

**The LocalSigner is FOR DEVELOPMENT AND TESTING ONLY. Never use it with real-fund keys.**

It exists so that CI pipelines can sign transactions without hardware. Using a private key from an environment variable on a general-purpose machine exposes the key to:
- Other processes with access to `/proc/<pid>/environ`
- Memory dumps if the machine is compromised

The synthetic test key in `testdata/phase3/holesky/private_key.txt` is `0x0101...0101` (64 `01` hex digits). It is **obviously synthetic** and must **never** be used with real funds. The address it derives controls no real ETH on any network we care about.

---

## Air-gapped workflow for mainnet

For mainnet deposits, use the air-gapped workflow:

1. **Build** the unsigned transaction on an online machine (no key required).
2. **Transfer** `unsigned.json` to an air-gapped machine (USB drive, QR code, etc.).
3. **Sign** on the air-gapped machine using a Ledger device.
4. **Transfer** `signed.json` back to the online machine.
5. **Broadcast** from the online machine via `send`.

The build step requires no private key. The air-gapped machine never needs network access. The Ledger keeps the private key off any networked machine entirely.

---

## Exit codes as a security tool

The tool never silently accepts an error. Every failure path returns a non-zero exit code:

| Code | Meaning | Security significance |
|------|---------|----------------------|
| 0 | Success | |
| 1 | Unexpected internal error | Never treat a non-zero exit as success |
| 2 | User/config error | Input validation failed before any key access |
| 3 | Signer/crypto error | Key invalid, Ledger not found, or Ledger app error |
| 4 | User abort | Ctrl-C or device rejection — no tx written |
| 5 | Broadcast/RPC error | Chain ID mismatch or node rejection |

In scripts, always check `$?` or use `set -e`. A script that ignores exit codes can silently proceed without a signed or broadcast transaction.

---

## Output file permissions

Files containing key-derived material are written with restrictive permissions:

| File | Permissions | Reason |
|------|-------------|--------|
| `signed.json` | `0o600` | Contains `from` address, tx hash, r, s, v, rawRLP |
| `signed.raw` | `0o600` | RLP-encoded signed tx; sufficient to broadcast |

These files should be treated as sensitive. After broadcast, delete them or move to secure storage.

---

## Deeper references

- [Phase 3 signer security review](../../../docs/deposit-tx/security/phase-3-signer.md) — security properties verified in code, known limitations, audit grep commands.
