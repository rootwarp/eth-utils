# Mainnet Golden-File Fixture Provenance

**Regeneration date:** 2026-05-16

---

## Summary

This document records the provenance and regeneration procedure for the mainnet
golden-file test fixtures located in `go/testdata/mainnet/`.

---

## Fixture generation method: self-referential (Option A)

The mainnet fixtures were generated programmatically using the Go
`eth-deposit-gen` implementation itself — **not** from the external
`staking-deposit-cli` Python tool.

### Why self-referential?

`staking-deposit-cli` v1.2.2 (installed at time of generation) only accepts
BLS keys derived from a mnemonic phrase via the `existing-mnemonic` subcommand.
The golden test key used in `go/test/e2e/` is a raw 32-byte fixed secret (not
derived from any mnemonic), so the CLI cannot regenerate deposit data for the
same public key.

Cross-validation against `staking-deposit-cli` is a Phase 3 follow-up requiring
a test key pair derived from a known mnemonic.

### What is proven by these fixtures

Despite being self-referential, the fixtures verify the following invariants:

- The BLS domain is computed with `GenesisForkVersion = [0x00, 0x00, 0x00, 0x00]`
  (mainnet), producing a signature that differs from the hoodi fixture.
- `fork_version` in the output JSON equals `"00000000"`.
- `network_name` in the output JSON equals `"mainnet"`.
- The `pubkey` and `withdrawal_credentials` are identical to the hoodi fixtures
  (same BLS key, same withdrawal type), confirming that only the domain changed.
- The `deposit_data_root` differs from hoodi because it includes the signature,
  which is domain-dependent.

---

## Tool and version

| Field | Value |
|---|---|
| **Generation tool** | `eth-deposit-gen` Go implementation (self-referential) |
| **staking-deposit-cli version** | v1.2.2 installed but not used (see above) |
| **BLS secret** | Fixed 32-byte test-only secret (`goldenSecret` in `hoodi_test.go`) — NEVER use on mainnet |
| **Regeneration date** | 2026-05-16 |
| **Go module** | `github.com/rootwarp/eth-utils/go` |

---

## Exact command used

```bash
# From the go/ module root:
REFRESH_GOLDEN=1 CGO_ENABLED=1 go test -run TestRefreshMainnetGolden -v -timeout 120s ./test/e2e/
```

Or via the Makefile (regenerates both hoodi and mainnet in one step):

```bash
cd go/ && make refresh-golden
```

---

## Generated files

| File | Contents |
|---|---|
| `go/testdata/mainnet/keystore.json` | EIP-2335 v4 keystore (scrypt, N=2^18) — same BLS key as hoodi fixtures |
| `go/testdata/mainnet/passphrase.txt` | Plaintext passphrase protecting the keystore (test-only) |
| `go/testdata/mainnet/pubkeys.txt` | 96-hex-char BLS public key (unprefixed, lowercase) |
| `go/testdata/mainnet/deposit_data-expected.json` | Expected deposit data JSON for mainnet |

All four files are standalone copies (not symlinks) for portability across
different checkout locations.

---

## Key fields in deposit_data-expected.json

| Field | Value | Notes |
|---|---|---|
| `fork_version` | `"00000000"` | Mainnet `GENESIS_FORK_VERSION` |
| `network_name` | `"mainnet"` | |
| `pubkey` | (see fixtures) | Same as hoodi — same BLS key |
| `withdrawal_credentials` | `0x00…00` | Type 0x00 BLS withdrawal |
| `amount` | `32000000000` | 32 ETH in Gwei |
| `deposit_cli_version` | `"2.7.0"` | Matches `CLIVersion` constant in `main.go` |

---

## Note on the `--i-understand-this-is-mainnet` safety gate

In CI and automated tests, no additional confirmation flag is required because
the deposit pipeline is invoked **programmatically** (directly calling
`deposit.Generator.Generate` or `runWithDeps`) rather than through the CLI.

When using the CLI interactively for a real mainnet deposit, operators should
exercise additional caution: verify that `network_name == "mainnet"` and
`fork_version == "00000000"` in the output JSON before submitting to the
deposit contract. A future CLI safety gate (e.g. `--i-understand-this-is-mainnet`)
is tracked as a Phase 4 follow-up.

---

## Re-validation against staking-deposit-cli (future work)

To cross-validate with `staking-deposit-cli`:

1. Generate a fresh BLS key from a known mnemonic using `deposit existing-mnemonic --chain mainnet`.
2. Replace `goldenSecret` in `hoodi_test.go` / `mainnet_test.go` with the new key's raw secret.
3. Re-run `make refresh-golden` to regenerate all fixtures.
4. Compare the `deposit_data-expected.json` output against what `staking-deposit-cli` produces for the same key/network/amount.

---

## Reference

- Mainnet deposit contract address: `0x00000000219ab540356cBB839Cbe05303d7705Fa`
- Mainnet `GENESIS_FORK_VERSION`: `0x00000000`
- Consensus spec — deposit domain: [ethereum/consensus-specs §deposit](https://github.com/ethereum/consensus-specs/blob/dev/specs/phase0/beacon-chain.md#deposits)
- eth-deposit-gen PRD: [`docs/prd.md`](../prd.md)
- Hoodi e2e validation template: [`docs/validation/hoodi-e2e.md`](hoodi-e2e.md)
