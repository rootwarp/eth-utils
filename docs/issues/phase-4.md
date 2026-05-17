# Phase 4 — M4: Release v1.0 Issues

Sprint-ready issues for Phase 4 of `eth-deposit-gen`. Phase 4 ships v1.0:
multi-arch release binaries, release CI, documentation, an internal audit
pass on signing-critical code, and the actual v1.0.0 tag. Issues 20–24 map
1:1 to project-plan.md tasks 4.1–4.5.

Story-point legend: **1 = 0.5d · 2 = 1d · 3 = 1.5d · 5 = 2d**.

---

## Issue #20 — GoReleaser configuration

**Story points:** 2
**Labels:** `infra`, `phase-4`

### Description
Stand up `.goreleaser.yaml` for darwin/linux × amd64/arm64. CGO complicates
cross-compilation; pick (and document) a strategy — either `zig cc` or
per-target GitHub Actions runners.

### Acceptance criteria
- [ ] `.goreleaser.yaml` is committed at the module root and builds:
      `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`.
- [ ] Windows is either built best-effort and clearly marked
      `experimental` in the release notes, or excluded entirely with a one-
      line rationale in `.goreleaser.yaml`.
- [ ] CGO strategy is documented in a top-level comment in
      `.goreleaser.yaml` and in `docs/validation/release-strategy.md`:
      either (a) `zig cc` cross-compilation with pinned zig version, or
      (b) matrix of native runners. The decision and rationale are
      recorded.
- [ ] `goreleaser release --snapshot --clean` succeeds locally and produces
      one archive per target with the binary, README, and `LICENSE`.
- [ ] Each produced binary runs `--help` and `--version` successfully on a
      VM/container of the matching OS+arch (recorded in
      `docs/validation/release-strategy.md`).
- [ ] `go install github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen@<sha>`
      against the current commit succeeds on a fresh `linux/amd64`
      environment with CGO toolchain present.
- [ ] Archive checksum files (`checksums.txt`) are produced and listed in
      the goreleaser manifest.

### Dependencies
Phase 3 complete.

---

## Issue #21 — Release CI pipeline

**Story points:** 2
**Labels:** `infra`, `phase-4`

### Description
GitHub Actions workflow that runs on `v*` tag push: lints, runs the CGO
test matrix, executes GoReleaser, and uploads checksums + SBOM artifacts.

### Acceptance criteria
- [ ] `.github/workflows/release.yml` triggers on `push` of tags matching
      `v*`.
- [ ] Pre-release jobs run: `go mod verify`, `go vet ./...`,
      `staticcheck ./...`, `go test ./...` on `linux/amd64`, `linux/arm64`
      (via QEMU or native), and `darwin/arm64`. All with `CGO_ENABLED=1`.
- [ ] `goreleaser release` runs only after all pre-release jobs are green.
- [ ] A `GITHUB_TOKEN` with `contents: write` is the only secret required;
      no long-lived PAT.
- [ ] The release upload includes: target binaries (per archive),
      `checksums.txt`, and an SBOM (`syft` or equivalent) per artifact.
- [ ] A dry-run validation: pushing a `v0.0.0-rc1` tag in a feature branch
      successfully produces a draft release with all expected assets
      (evidence recorded in `docs/validation/release-strategy.md`).
- [ ] The workflow fails (does not publish) if `go mod verify` fails or any
      test fails — including the Hoodi and mainnet golden tests.

### Dependencies
#20.

---

## Issue #22 — README and usage documentation

**Story points:** 2
**Labels:** `docs`, `phase-4`

### Description
Operator-facing README with concrete examples, security guidance, exit-code
reference, and a troubleshooting section. Link contributors to architecture
and research docs.

### Acceptance criteria
- [ ] `go/cmd/eth-deposit-gen/README.md` is committed and contains, in
      order: tagline, install instructions (`go install …@latest` and
      release-binary download), quickstart for Hoodi, quickstart for
      mainnet (with the `--i-understand-this-is-mainnet` flag from #14),
      full flag reference auto-listed or hand-maintained, security notes,
      exit-code table, and a troubleshooting section covering wrong
      passphrase, missing CGO toolchain, and `--output-dir not writable`.
- [ ] Security notes explicitly state: passphrase is never accepted as a
      flag; sourced via `--passphrase-env <NAME>` or TTY prompt; the
      decrypted key is zeroized after signing.
- [ ] Exit-code table matches the mapping in #9 (0/2/3/4) and includes the
      sentinel error each code corresponds to.
- [ ] At least one example shows `--dry-run` (#16), one shows
      `--parallel` (#15), one shows `--verify-with-deposit-cli` (#18).
- [ ] Contributor section links to `docs/architecture.md`,
      `docs/prd.md`, `docs/project-plan.md`, and the three research notes
      under `docs/research/`.
- [ ] All example commands in the README are executed verbatim on a fresh
      environment as part of the issue's acceptance work; any deviation is
      corrected before merge.

### Dependencies
Phase 3 flags finalized (#15, #16, #17, #18, #19).

---

## Issue #23 — Internal audit pass on signing-critical code

**Story points:** 2
**Labels:** `crypto`, `security`, `phase-4`

### Description
Re-read `internal/ssz`, `internal/bls`, and `internal/deposit` end-to-end
with a "what could write a wrong byte?" lens. Re-confirm golden fixtures
against the latest upstream `staking-deposit-cli`. Sign off via a checklist
under `docs/validation/`.

### Acceptance criteria
- [ ] `docs/validation/audit-v1.md` is committed and contains a dated
      checklist covering at minimum:
  - [ ] every `HashTreeRoot` in `internal/ssz` cross-checked once more
        against `docs/research/bls-ssz-libraries.md` chunk tables;
  - [ ] every `[N]byte` size at the `internal/bls` boundary verified
        (`48`, `96`, `32`);
  - [ ] every step of the 10-step deposit pipeline (#6) re-checked against
        `docs/architecture.md`;
  - [ ] confirmation that `defer key.Zeroize()` is reached on every error
        path in `cmd/eth-deposit-gen.run`;
  - [ ] confirmation that the `internal/deposit` pubkey-equality check
        precedes any call to `Sign`;
  - [ ] confirmation that `output` writes only via temp + `fsync` + rename.
- [ ] Golden fixtures (`testdata/hoodi/` and `testdata/mainnet/`) are
      regenerated against the latest `staking-deposit-cli` release; if the
      output diverges, the cause is investigated **before** bumping the
      pinned `CLIVersion` constant.
- [ ] `CLIVersion` in `cmd/eth-deposit-gen` is bumped (if needed) to match
      the `staking-deposit-cli` version used to regenerate fixtures; the
      bump is recorded in `audit-v1.md`.
- [ ] CI is verified to set `GOFLAGS=-mod=readonly` and run `go mod verify`
      on every job; if absent, this issue adds them.
- [ ] `go.sum` is reviewed; any hash change since Phase 1 is justified in
      the audit doc.
- [ ] The audit doc is signed off by at least one engineer other than the
      primary implementer (PR review approval suffices as the signature).

### Dependencies
#20, #21, #22.

---

## Issue #24 — Tag v1.0.0 and verify release artifacts

**Story points:** 1
**Labels:** `infra`, `phase-4`

### Description
Cut the v1.0.0 release once the audit (#23) is signed off. Verify the
produced binaries run on fresh macOS and Linux VMs.

### Acceptance criteria
- [ ] Git tag `v1.0.0` is pushed and the release CI pipeline (#21)
      completes successfully, producing a GitHub release with darwin/linux
      × amd64/arm64 binaries and `checksums.txt`.
- [ ] The release notes are populated and link to: `docs/architecture.md`,
      the audit doc (`docs/validation/audit-v1.md`), and a short changelog.
- [ ] On a fresh `macOS arm64` VM (or laptop), the downloaded
      `eth-deposit-gen` binary runs `--help` successfully without
      additional install steps.
- [ ] On a fresh `linux/amd64` VM, the downloaded binary runs `--help`
      successfully.
- [ ] On both VMs, `eth-deposit-gen --network hoodi --dry-run …` against
      `testdata/hoodi/` produces output matching `deposit_data-expected.json`
      field-for-field.
- [ ] `go install github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen@v1.0.0`
      succeeds on a clean machine with CGO toolchain available.
- [ ] No open P0 issues remain against the v1.0.0 tag.

### Dependencies
#23.
