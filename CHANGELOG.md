# Changelog

All notable changes to `eth-deposit-gen` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [1.0.0] - 2026-05-17

### Added

- **Networks:** Hoodi testnet support (fully enabled, golden-tested). Mainnet support enabled behind `--i-understand-this-is-mainnet` safety gate; the flag and an uppercase `MAINNET` banner are required before any mainnet signing occurs.
- **`--keystore-dir`:** Directory-based keystore loading; scans a directory of EIP-2335 v4 keystores and loads only the keystore matching each requested pubkey — no decryption of unneeded files.
- **`--parallel N`:** Bounded parallel signing worker pool (default 1); deterministic output order regardless of parallelism level; benchmarked at ≥ 200 entries/sec.
- **`--dry-run`:** Print deposit JSON to stdout without writing a file; sha256 on stderr matches stdout bytes.
- **`--verbose` / `--json-logs`:** Structured `log/slog` logging; text or JSON handler; signing-critical packages contain zero log statements.
- **`--verify-with-deposit-cli`:** Optional post-generation cross-check via `deposit verify --input-file` (requires `staking-deposit-cli >= 2.7.0`).
- **Progress indicator:** `signing: <i>/<n>` on TTY stderr for batches > 5; 10%-boundary `slog.Info` on non-TTY.
- **Exit codes:** 0 success, 2 configuration/user errors, 3 crypto/verification failures, 4 SIGINT.
- **Pre-built binaries:** darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 with `checksums.txt` and per-artifact SBOM (SPDX 2.3).

### Security

- Internal audit (`docs/validation/audit-v1.md`) signed off 2026-05-17: SSZ chunk tables, BLS boundary sizes (pubkey 48 bytes, signature 96 bytes, secret 32 bytes), 10-step deposit pipeline, zeroization on every error path, atomic output write (temp + fsync + rename).
- `GOFLAGS=-mod=readonly` enforced in all CI jobs (both `eth-deposit-gen.yml` and `release.yml`).
- Atomic file write: `.tmp` file + `f.Sync()` + `os.Rename` — no partial file is ever visible to the OS.
- BLS secret key zeroized immediately after signing via `key.Zeroize()`; passphrase bytes zeroized via `defer zeroizeBytes` with `runtime.KeepAlive` guard.

---

[1.0.0]: https://github.com/rootwarp/eth-utils/releases/tag/v1.0.0
