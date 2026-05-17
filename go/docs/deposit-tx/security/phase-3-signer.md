# Phase 3 Signer Security Review

## Architecture

The Phase 3 signing subsystem consists of three components:

- **`Signer` interface** (`internal/signer/signer.go`): Defines `Sign(ctx, UnsignedTx)`, `Name()`, `RequiresUserInteraction()`, and `Close()`. All signers implement this interface. The security contract is stated on the interface: implementations MUST NOT log, persist, or expose private key material.

- **`LocalSigner`** (`internal/signer/local.go`): Signs EIP-1559 transactions using a raw secp256k1 private key held in memory. The key is sourced from an environment variable (never from CLI flags or argv). Key bytes are zeroized on `Close`.

- **`LedgerSigner`** (`internal/signer/ledger.go`): Signs via a Ledger hardware wallet using the go-ethereum `usbwallet` transport. The private key never leaves the device. Requires CGO (runtime-gated via `ledger_cgo.go` / `ledger_nocgo.go`).

The CLI (`cmd/eth-deposit-tx/sign.go`) wires both signers via `--signer local|ledger`. The env var name is validated as a POSIX identifier before use; the actual key value is never a CLI argument.

---

## Security Properties Verified by Code

### 1. Private keys never via flag or argv

**Code**: `sign.go:LoadSignConfig` reads `c.String("private-key-env")`, which is an environment variable *name*, not a value. The actual key bytes come from `os.Getenv(envVar)` inside `NewLocalSignerFromEnv`.

**POSIX name validation** (`sign.go:posixEnvVarName`, regex `^[A-Z_][A-Z0-9_]*$`): rejects values that look like hex keys accidentally passed as the flag value. A raw 32-byte hex key (`0x...`) starts with `0` or `x`, both of which fail the regex, producing exit code 2 with a diagnostic message.

**Audit grep**:
```sh
grep -n 'private.key\|PrivateKey\|hexKey\|rawKey\|keyBytes' go/cmd/eth-deposit-tx/sign.go
# Expected: zero matches for raw key handling in cmd layer
```

### 2. LocalSigner zeroizes key bytes on Close

**Code**: `local.go:Close` calls `s.closed.Swap(true)` to prevent double-close, then writes `0` to every byte of `s.key`.

**Test**: `TestLocalSigner_Close_ZeroizesKey` (internal test, `local_internal_test.go`) reads `s.key` directly after Close and verifies all bytes are zero.

**Audit grep**:
```sh
grep -n 'key\[' go/internal/signer/local.go
# Should show the zeroization loop: `for i := range s.key { s.key[i] = 0 }`
```

### 3. No key material in error messages or logs

**Code**: All error paths in `local.go` and `ledger.go` use pre-defined sentinel errors (`ErrInvalidKey`, `ErrSignerClosed`, etc.) or format strings that reference variable *names* (e.g., `"environment variable %q is not set"`), never variable values.

**Test**: `TestNewLocalSignerFromHex_ErrorDoesNotIncludeKey` verifies the error string for an invalid-length key does not contain the input bytes.

**Audit greps**:
```sh
# Verify no raw key bytes in error format strings:
grep -n 'Errorf.*key\|Errorf.*priv\|Errorf.*0x' go/internal/signer/local.go
grep -n 'Errorf.*key\|Errorf.*priv\|Errorf.*0x' go/internal/signer/ledger.go

# Verify slog calls do not log key material:
grep -rn 'slog\.' go/internal/signer/
# Expected: zero matches (no slog calls in signer package)
```

### 4. Ledger device owns its key; signature verified via sender recovery

**Code**: `ledger.go:Sign` calls `wallet.SignTx` and immediately recovers the sender address via `types.Sender(ethSigner, signedTx)`. The `From` field in `SignedTx` is this recovered address, not a trusted input.

**Note**: `LedgerSigner` does not perform a `From == derivedAddress` cross-check (unlike `LocalSigner`, which checks `from != expectedAddr`). The Ledger device enforces key ownership at the hardware level; adding a software cross-check would require storing the derived address across a round-trip and is deferred to Phase 4.

**Audit grep**:
```sh
grep -n 'Sender\|from\|From' go/internal/signer/ledger.go
# Should show types.Sender call and From field assignment
```

### 5. Sentinel-based error wrapping enables typed exit codes

**Code**: All signer errors wrap one of the sentinels in `errors.go`: `ErrInvalidKey`, `ErrInvalidChainID`, `ErrUserRejected`, `ErrNoDevice`, `ErrAppNotOpen`, `ErrChainIDMismatch`, `ErrSignerClosed`, `ErrLedgerNotSupported`.

`exit.go:ExitCodeFor` inspects these via `errors.Is` to map them to typed exit codes (2 = user error, 3 = signer/crypto error, 4 = user abort).

**Test**: `TestSentinelErrors` (exhaustive) verifies every sentinel is non-nil and has a non-empty message.

### 6. Signed output file permissions

**Code**: `sign.go:signAction` writes the signed tx JSON with permissions `0o600` (owner read/write only). Signed transactions contain the sender address, transaction hash, and signature components.

---

## Known Limitations / Non-Issues

### LocalSigner is for development only

`LocalSigner` is explicitly not recommended for real-fund mainnet use. The key comment on the type reads:

> SECURITY: For development and CI only. Real-fund usage MUST use Ledger (Phase 3.3+).

The CLI help text and the sign command `Description` repeat this warning. A future enforcement mechanism (e.g., refusing `--signer local` on non-testnet chain IDs) is a Phase 4 consideration.

### Ledger signing heuristics are pattern-based, not exact

`isUserRejectedErr`, `isChainIDMismatchErr`, and `isAppNotOpenErr` use string-matching heuristics on error messages returned by the go-ethereum `usbwallet` transport. These patterns cover known APDU error codes (6985, 6e00, 6e01, 6d00, 6a80, 6a81) and textual hints, but:

- The exact strings returned by go-ethereum may change across library versions.
- Hardware firmware updates may produce different error messages.
- The ordering matters: `isChainIDMismatchErr` is checked before `isUserRejectedErr` because some chain-ID errors contain the word "rejected".

**TODO**: Refine heuristics after the first real hardware test (see `phase-3-ledger-runbook.md`). Document the actual error strings observed.

### Goroutine leak on ctx-cancelled Ledger sign

When context cancellation interrupts `LedgerSigner.Sign`, the goroutine performing `wallet.SignTx` continues running until the user presses a button on the Ledger device or the device times out. This is an accepted trade-off: APDU exchanges on HID cannot be interrupted mid-flight. The goroutine will eventually return and send its result to the buffered `ch` channel, at which point it exits.

**Audit**: The goroutine does not hold any locks or shared state beyond the `ch` channel, which is buffered (size 1). No goroutine leak can grow unboundedly from a single invocation.

### No CGO_ENABLED=0 build for the whole repo

The repo transitively requires CGO via `herumi/bls-eth-go-binary`. `ledger_nocgo.go` (`//go:build !cgo`) provides an `ErrLedgerNotSupported` stub so code that conditionally uses `--signer ledger` can compile in all environments, but the overall module does not support `CGO_ENABLED=0 go build ./...`.

---

## Audit Checklist

Run these from the repo root to re-verify key security properties:

```sh
# 1. No raw key values in error paths
grep -rn 'hexKey\|rawKey\|keyBytes\|privateKey' go/internal/signer/

# 2. Only env var name (not value) appears in CLI layer
grep -n 'private.key' go/cmd/eth-deposit-tx/sign.go

# 3. Key zeroization on Close
grep -n 'for i := range s.key' go/internal/signer/local.go

# 4. Output file permissions are 0o600
grep -n '0o600\|0600' go/cmd/eth-deposit-tx/sign.go

# 5. POSIX env var name validation is in place
grep -n 'posixEnvVarName\|A-Z_' go/cmd/eth-deposit-tx/sign.go

# 6. No slog/log calls that could leak key material in signer package
grep -rn 'slog\.\|log\.' go/internal/signer/

# 7. All sentinel errors are wrapped (not returned raw)
grep -rn 'return ErrInvalidKey\|return ErrUserRejected\|return ErrNoDevice' go/internal/signer/
# Expected: zero matches — all returns use fmt.Errorf("...: %w", ErrXxx)
```
