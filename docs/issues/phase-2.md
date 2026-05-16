# Phase 2 — M2: Mainnet Enablement Issues

Sprint-ready issues for Phase 2 of `eth-deposit-gen`. Phase 2 unlocks mainnet
behind its own golden-file gate, validates an actual Hoodi deposit on-chain,
and hardens the network-confusion UX. Issues 11–14 map 1:1 to project-plan.md
tasks 2.1–2.4.

Story-point legend: **1 = 0.5d · 2 = 1d · 3 = 1.5d · 5 = 2d**.

---

## Issue #11 — Add mainnet constants in `internal/network`

**Story points:** 1
**Labels:** `core`, `phase-2`

### Description
Promote `Mainnet` from the Phase-1 "not yet enabled" stub into a real
`Params` entry with the spec-correct genesis fork version. Per ADR-004, the
values are compile-time only.

### Acceptance criteria
- [ ] `internal/network.Lookup(Mainnet)` returns
      `Params{Name: Mainnet, GenesisForkVersion: [4]byte{0x00,0x00,0x00,0x00}}`
      with `nil` error (replacing the Phase-1 `ErrMainnetNotEnabled`).
- [ ] The sentinel `ErrMainnetNotEnabled` is removed or — if other packages
      depend on it — re-purposed and documented as deprecated.
- [ ] A dedicated unit test asserts the mainnet fork bytes byte-for-byte:
      `[4]byte{0x00,0x00,0x00,0x00}`.
- [ ] `ParseFlag("mainnet")` returns `Mainnet, nil`.
- [ ] Existing Hoodi tests still pass unchanged.

### Dependencies
#2.

---

## Issue #12 — Mainnet golden-file fixture and integration test (M2 exit gate)

**Story points:** 2
**Labels:** `test`, `phase-2`, `critical-path`

### Description
Mirror the Hoodi golden-file test for mainnet. This is the M2 correctness
gate: no Phase 3 work begins until this is green.

### Acceptance criteria
- [ ] `testdata/mainnet/` is committed with `keystore.json`,
      `passphrase.txt`, `pubkeys.txt`, and `deposit_data-expected.json`
      produced by `staking-deposit-cli --chain mainnet` for the same pubkey
      set declared in `pubkeys.txt`.
- [ ] A test mirroring the Hoodi golden test (`test/e2e/mainnet_test.go`
      or equivalent) runs the full pipeline against these fixtures and
      asserts field-by-field equivalence with `deposit_data-expected.json`.
- [ ] The test verifies the emitted `fork_version` is `"00000000"` and
      `network_name` is `"mainnet"`.
- [ ] A test asserts that running with `--network mainnet` causes the
      Phase-1 confirmation banner to include the literal string `mainnet`
      on stderr before any signing happens.
- [ ] `make refresh-golden` is extended to regenerate both `hoodi/` and
      `mainnet/` fixtures; `docs/validation/mainnet-golden.md` is created
      and records the upstream CLI version, the exact command, and the
      regeneration date.
- [ ] CI runs both Hoodi and mainnet golden tests in the default
      `go test ./...` invocation.

### Dependencies
#11, #10.

---

## Issue #13 — On-chain Hoodi deposit validation

**Story points:** 2
**Labels:** `test`, `phase-2`, `infra`

### Description
End-to-end validation that the deposit contract accepts our output. Use
Hoodi (not mainnet) to keep the blast radius contained. This is a manual
acceptance step, not an automated test; the evidence is recorded under
`docs/validation/`.

### Acceptance criteria
- [ ] An operator runs `eth-deposit-gen --network hoodi ...` against a
      throwaway BLS key they control and submits the resulting JSON to the
      Hoodi deposit contract (via the Launchpad UI or a direct contract
      call).
- [ ] The deposit transaction is accepted by the deposit contract on first
      submission (no resubmission needed).
- [ ] `docs/validation/hoodi-e2e.md` is committed and records: validator
      pubkey, the deposit tx hash, the block number, the date of
      submission, the exact CLI invocation used, the operator's identity,
      and a link to a block explorer confirming the deposit log was
      emitted.
- [ ] Within ~16 hours, the validator appears as `pending_initialized` (or
      better) on `beaconcha.in/hoodi`; a link to the validator page is
      appended to `docs/validation/hoodi-e2e.md`.
- [ ] If submission fails, the failure is root-caused **before** Phase 2 is
      considered complete; the cause and the fix are documented in
      `docs/validation/hoodi-e2e.md`.

### Dependencies
#11.

---

## Issue #14 — Network safety banner hardening for mainnet

**Story points:** 1
**Labels:** `cli`, `phase-2`

### Description
The Phase-1 banner is informational. For mainnet, the PRD risk register
demands an explicit operator acknowledgement to prevent accidental
production submissions during testing. Implement a single hardening
mechanism — pick one (an explicit ack flag, recommended) and ship it.

### Acceptance criteria
- [ ] When `--network mainnet` is selected, the app **requires** the
      additional flag `--i-understand-this-is-mainnet`. Absence of the flag
      returns exit code 2 with a clear error: `mainnet selected; pass
      --i-understand-this-is-mainnet to acknowledge` — no signing occurs.
- [ ] For `--network hoodi`, the new flag is accepted but has no effect
      (no spurious warning).
- [ ] The banner printed before signing on mainnet contains the literal
      string `MAINNET` (uppercased) in addition to the Phase-1 fields.
- [ ] Unit tests cover: mainnet without ack flag → exit code 2 and no
      signer ever called; mainnet with ack flag → signing proceeds and
      banner contains `MAINNET`; hoodi with ack flag → signing proceeds
      normally.
- [ ] Help text documents the flag with a one-line warning about its
      irreversible consequence.
- [ ] The behaviour is mentioned in `docs/validation/mainnet-golden.md`
      (banner test reference).

### Dependencies
#11, #8.
