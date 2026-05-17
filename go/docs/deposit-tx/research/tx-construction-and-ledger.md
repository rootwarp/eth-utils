# Research: Transaction Construction, RLP, ABI and Ledger Integration for eth-deposit-tx

## Recommendation

**Use `github.com/ethereum/go-ethereum` (pinned to a recent v1.14.x release) for all transaction construction, RLP encoding, ABI packing, optional RPC calls, and Ledger hardware signing.**

This is the canonical, lowest-risk choice that guarantees correct EIP-1559 (preferred) unsigned deposit transactions for the `deposit(bytes pubkey, bytes withdrawal_credentials, bytes signature, bytes32 deposit_data_root) payable` call. The only notable trade-off is that the primary Ledger path requires CGO (via the transitive `karalabe/usb` dependency for HID). This is unavoidable for reliable Ledger Nano S/X support and is acceptable under the PRD because Ledger is the explicit primary signing path. All other constraints (hybrid offline/RPC, air-gapped capability, private-key zeroization, minimal additional surface, and following existing urfave/cli + internal/network patterns) are cleanly satisfied with a thin, well-understood import surface from this single module.

No lighter/safer alternative exists for production-grade EIP-1559 + Ledger signing without reinventing critical cryptography and protocol logic.

## Context

- **Goal:** Implement the new `eth-deposit-tx` CLI (primarily `build` to produce an unsigned transaction from deposit data JSON + optional overrides, and `sign` for Ledger-primary / private-key fallback) per the approved PRD.
- **Constraints (directly from PRD — single source of truth):**
  - EIP-1559 preferred; legacy fallback supported.
  - Primary signing: Ledger Nano S / Nano X (HID).
  - Private-key fallback must use strong zeroization (leverage existing patterns from the eth-utils Go codebase).
  - Optional `--rpc-url` for nonce, gas fees, base fee, chain ID (graceful pure-offline fallback with manual overrides required).
  - Air-gapped workflows fully supported.
  - Follow existing CLI structure (urfave/cli/v2), network handling (`go/internal/network/network.go`), and security expectations.
  - No go-ethereum in the current codebase — this will be a new (but canonical) dependency.
  - "No heavy / CGO-heavy where possible" — CGO is tolerated only for the Ledger path.
  - Output of `build` must be consumable by `sign` (RLP/hex or structured form).
- **Evaluated:** go-ethereum `core/types` + `accounts/usbwallet` + `accounts/abi` + `ethclient`, alternative Ledger libraries (ledger-go etc.), manual RLP/ABI implementations, prior-art tools (Foundry cast, ethereal, Safe offline flows).

## Comparison: Ledger Signing Library Options

| Criteria                    | go-ethereum/accounts/usbwallet (Recommended)          | github.com/ledger/ledger-go (or ledger-go) | Low-level custom APDU (using ledger libs) | Pure-Go HID attempts |
|-----------------------------|-------------------------------------------------------|--------------------------------------------|-------------------------------------------|----------------------|
| License                     | LGPL-3.0 (via go-ethereum)                            | MIT / Apache                               | MIT                                       | Varies              |
| CGO / HID requirement       | Yes (karalabe/usb)                                    | Yes                                        | Yes                                       | Partial / incomplete |
| Native EIP-1559 + types     | Full (DynamicFeeTx, correct hashing)                  | None (you implement everything)            | None                                      | None                |
| Maturity & production usage | Very high (geth itself + many staking/tools CLIs)     | Medium (general Ledger APDU)               | Low (you own the protocol)                | Low                 |
| Maintenance (May 2026)      | Active (tied to go-ethereum)                          | Moderate                                   | None                                      | Varies              |
| Code you must write         | Thin wrapper + UX + error mapping                     | Full tx hashing, EIP-1559, Ledger ETH app protocol, blind-sign handling | Everything + crypto/rlp                   | Everything          |
| Blind signing for contracts | Supported (user enables in Eth app settings)          | You must implement                         | You must implement                        | N/A                 |
| Dep tree impact             | Adds go-ethereum (large but canonical & already trusted in ecosystem) | Smaller additional dep                    | You add rlp + crypto + types              | Small               |
| Risk of protocol / replay bugs | Very low                                            | Medium-high                                | High                                      | High                |

**Clear winner: `github.com/ethereum/go-ethereum/accounts/usbwallet`.** It gives correct, audited tx construction + signing for exactly the `types.Transaction` objects we build in the `build` phase.

## Detailed Analysis

### 1. Transaction Construction & RLP in Go (`github.com/ethereum/go-ethereum/core/types`)

**Best current practice (2026):** Construct `&types.DynamicFeeTx`, wrap with `types.NewTx(...)`, then `rlp.EncodeToBytes(tx)` for the unsigned form. This is the exact wire format a signer (Ledger or private key) expects. Legacy transactions use `&types.LegacyTx` the same way.

**Recommended minimal import surface + version pin:**

```go
// go.mod
require github.com/ethereum/go-ethereum v1.14.8  // or latest v1.14 patch; verify at implementation time
```

```go
import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)
```

**Code sketch — Build unsigned deposit transaction (EIP-1559 preferred):**

```go
const depositContractABI = `[
  {
    "inputs": [
      {"name": "pubkey", "type": "bytes"},
      {"name": "withdrawal_credentials", "type": "bytes"},
      {"name": "signature", "type": "bytes"},
      {"name": "deposit_data_root", "type": "bytes32"}
    ],
    "name": "deposit",
    "outputs": [],
    "stateMutability": "payable",
    "type": "function"
  }
]`

func BuildUnsignedDepositTx(
    network NetworkConfig,
    params TxParams,          // nonce, gasLimit, gasTipCap, gasFeeCap, etc. (from RPC or manual)
    deposit DepositData,      // from JSON (pubkey, withdrawal_credentials, signature, deposit_data_root, amount in Gwei)
) (*types.Transaction, error) {

    // JSON amount is in Gwei → wei
    value := new(big.Int).Mul(deposit.AmountGwei, big.NewInt(1_000_000_000))

    parsedABI, err := abi.JSON(strings.NewReader(depositContractABI))
    if err != nil { return nil, err }

    callData, err := parsedABI.Pack(
        "deposit",
        deposit.Pubkey,
        deposit.WithdrawalCredentials,
        deposit.Signature,
        deposit.DepositDataRoot,
    )
    if err != nil { return nil, err }

    tx := types.NewTx(&types.DynamicFeeTx{
        ChainID:   network.ChainID,
        Nonce:     params.Nonce,
        GasTipCap: params.GasTipCap,
        GasFeeCap: params.GasFeeCap,
        Gas:       params.GasLimit,           // sensible default or RPC estimate (~150k-200k gas)
        To:        &network.DepositContract,
        Value:     value,
        Data:      callData,
    })
    return tx, nil
}

// The form that `sign` will consume
func UnsignedTxRLP(tx *types.Transaction) (string, error) {
    raw, err := rlp.EncodeToBytes(tx)
    if err != nil { return "", err }
    return "0x" + hex.EncodeToString(raw), nil
}
```

**Legacy fallback:** Construct `&types.LegacyTx{...}` identically.

**Key points for architect:**
- `types.NewTx` + DynamicFeeTx with no `V/R/S` fields = unsigned.
- Use the official `abi` package for packing — it handles the dynamic `bytes` + `bytes32` correctly (including length prefixes).
- Always include `ChainID` in the tx (EIP-155 replay protection).
- Output can be the raw RLP hex + a small JSON envelope with human-readable fields for review (`to`, `value`, `data`, `nonce`, `maxFeePerGas`, etc.).

### 2. Ledger Hardware Wallet Integration (Primary Signing Path)

**Recommended library:** `github.com/ethereum/go-ethereum/accounts/usbwallet`

**Typical setup & signing flow (high-level for `sign --ledger`):**
1. `hub := usbwallet.NewLedgerHub()` (or equivalent detector).
2. Enumerate wallets (user must have Ledger plugged in, unlocked, and the **Ethereum** app open).
3. `w.Open("")`; derive or select account (standard path or `--hd-path` override).
4. `signed, err := w.SignTx(account, unsignedTx, chainID)` — this computes the correct hash and sends the request over HID.
5. On the device the user sees a blind-signing prompt (large byte arrays for pubkey/signature). **Blind signing must be enabled** in the Ethereum app settings, otherwise the device rejects the data as "too long".
6. On approval, return the fully signed `types.Transaction` (or its RLP) for output / optional broadcast.

**Maturity, license, maintenance, size:** Part of the main go-ethereum tree — extremely mature, actively maintained, used by geth and many production tools. License is LGPL-3.0 (acceptable for CLI). Dep tree is the same go-ethereum module we already pull for tx construction.

**CGO & platform gotchas (critical — flag for architect):**
- `CGO_ENABLED=1` is **required** on macOS, Linux, and Windows (karalabe/usb uses platform HID libraries via cgo).
- macOS: First execution often triggers a Security & Privacy prompt.
- Linux: udev rules for Ledger are usually needed.
- Ledger Ethereum app: v1.9+ / v1.10+ recommended for EIP-1559 + contract data; blind signing toggle **must** be ON for deposit transactions.
- Device locked / wrong app / no blind signing → clear, actionable errors are essential UX.

**Alternatives considered:** Lower-level `github.com/ledger/ledger-go` or raw APDU requires re-implementing EIP-1559 hashing, signature encoding, and the entire Ledger ETH app protocol — high risk of subtle bugs and far more code. Not recommended for P0.

**Code sketch (high-level wrapper):**
```go
// pseudocode for sign command
hub := usbwallet.NewLedgerHub()
wallets := hub.Wallets()
if len(wallets) == 0 { return errors.New("no Ledger detected — is it unlocked and the Ethereum app open?") }
w := wallets[0]
w.Open("")
// derive account...
signed, err := w.SignTx(account, unsignedTx, chainID)
```

### 3. Network / Deposit Contract Constants

**Canonical values (verified from authoritative sources + alignment with existing eth-deposit-gen):**

| Network   | Chain ID | Deposit Contract                              | Notes |
|-----------|----------|-----------------------------------------------|-------|
| mainnet   | 1        | 0x00000000219ab540356cBB839Cbe05303d7705Fa      | Production |
| hoodi     | 560048   | 0x00000000219ab540356cBB839Cbe05303d7705Fa      | Used by eth-deposit-gen (confirm exact value + fork_version from source) |
| sepolia   | 11155111 | 0x00000000219ab540356cBB839Cbe05303d7705Fa      | Easy to add |
| holesky   | 17000    | 0x00000000219ab540356cBB839Cbe05303d7705Fa      | Easy to add |

**Mapping strategy (recommended):** The input JSON contains `network_name` (and `fork_version` for BLS/deposit-data validation). Use `network_name` as the key into a central map that returns `ChainID` + `DepositContract`. `fork_version` is **not** used for the execution-layer tx — it stays with the existing validation logic.

**Implementation hint:** Extend `internal/network` (already present) with a small `DepositNetwork` struct + map. This avoids duplication with eth-deposit-gen and makes adding networks trivial.

**Authoritative sources:** `ethereum/staking-deposit-cli/settings.py`, ethereum/consensus-specs deposit contract deployments, official Hoodi/Launchpad documentation.

**Uncertain / needs architect confirmation:** Exact deposit contract address and fork_version for hoodi — pull the literal values from the current eth-deposit-gen implementation to prevent drift.

### 4. Hybrid RPC vs Pure Offline Feasibility

- When `--rpc-url` is supplied: use `ethclient.Dial` → `ChainID()`, `PendingNonceAt()`, `SuggestGasTipCap()`, latest header for base fee, optional `EstimateGas` for the exact call data.
- All RPC usage is **read-only** — never sends secrets.
- Fallback (no URL or any failure): require explicit CLI flags (`--nonce`, `--gas-limit`, `--max-fee-per-gas`, `--max-priority-fee-per-gas`, `--chain-id`). Emit a clear, actionable error: "RPC unavailable or failed — supply --nonce ... for air-gapped mode".
- Fully supports the PRD's air-gapped requirement (`build` on one machine with manual params, `sign` on another or the same).

Security implication: negligible (reads only). Implementation cost: low (thin wrapper around ethclient + good flag/override logic).

### 5. Prior Art & Competitive Landscape (Two-Phase "Build then Sign" CLIs)

- **Foundry `cast`**: `cast mktx` / `cast send --unsigned` and related flows are the most popular current example of exactly this pattern.
- **ethereal** (Go CLI, ecosystem contributors): Direct precedent in the Go/Ethereum world for offline tx building + Ledger/private-key signing.
- **Safe Transaction Builder + offline signers**: Same two-phase model (construct unsigned → collect signatures out-of-band).
- **Python staking-deposit-cli**: Produces the input `deposit_data.json` — our tool is the natural execution-layer companion.
- Common pattern across the ecosystem: `build` produces a portable artifact (RLP hex or JSON envelope); `sign` consumes it + a signer (Ledger, key, or multisig).

This validates the PRD's `build` / `sign` separation as the correct, battle-tested design.

### 6. Dependency & Security Constraints

- **Only new top-level dependency:** `github.com/ethereum/go-ethereum` (we use a deliberately thin surface: `core/types`, `rlp`, `common`, `accounts/abi`, `ethclient`, `accounts/usbwallet`, `crypto`).
- **CGO policy:** Accept for Ledger only. Add prominent documentation, build instructions (`CGO_ENABLED=1 go build`), and runtime error messages.
- **Private-key fallback security:** Use `crypto.ToECDSA` + `types.SignTx`. After use, zero the key material with the project's existing zeroization helpers (already present for BLS keys in the eth-deposit-gen style code). Do not rely on GC alone.
- **Known footguns (call out in code/docs):**
  - Wrong `ChainID` in the tx (replay protection failure).
  - Incorrect ABI packing (tx will revert on-chain).
  - Ledger blind signing not enabled (device error).
  - Using non-canonical RLP (use the official `types` + `rlp`).

No other heavy or risky dependencies are required.

### 7. Feasibility Assessment

**Verdict: P0 scope is realistic and can be delivered with high quality in modest effort (core tx/RLP/ABI logic is small and standard; the majority of work is robust Ledger UX, error handling, cross-platform testing, CLI surface, and tests).**

**High-risk areas — prototype/spike early (Phase 1):**
- Complete Ledger happy path + all common failure modes (device detection, app not open, blind signing off, macOS USB permission, Nano S vs X differences).
- Gas estimation + sensible defaults for the deposit call.
- Exact interchange format between `build` output and `sign` input (rich JSON vs minimal RLP hex — affects reviewability and air-gapped workflow).

**Recommended package boundaries (hints for architect):**
- `cmd/eth-deposit-tx/main.go` + `build.go`, `sign.go` (mirror the exact style and structure of `eth-deposit-gen`).
- `internal/tx/builder.go` — pure, easily testable construction/RLP/ABI logic.
- `internal/signer/ledger.go` + `privatekey.go` (with zeroization).
- `internal/network/` — extend the existing package with deposit constants + optional `RPCClient` helper.
- Deposit data types: reuse/adapt the struct from eth-deposit-gen if feasible (avoids duplication).

**Effort estimate (rough):** 1–2 focused developer-weeks for a solid P0 implementation including tests and docs (heavier on Ledger testing).

## Open Questions for the Software Architect

- Exact output format of `build` (minimal `0x...` RLP hex, or a review-friendly JSON envelope with decoded fields + the RLP?).
- Default gas limit and fee strategy when no RPC is available.
- Support for custom HD derivation paths on Ledger (`--hd-path`)?
- Build strategy for the CGO requirement (documented `CGO_ENABLED=1`, build tags, or a separate "with-ledger" binary?).
- Final supported network list for v1 (P0 = mainnet + hoodi; should sepolia/holesky be included?).
- Any additional audit metadata the unsigned artifact should carry (e.g. hash of original deposit JSON).

## Risks & Gotchas

- Real-device Ledger testing on macOS (primary dev OS), Linux, and Windows is mandatory.
- "Blind signing" UX friction will be the #1 user support issue — document it everywhere (README, command help, error messages).
- go-ethereum transitive dependency size is noticeable but acceptable for a specialized CLI (similar to other tools in the ecosystem).
- Any future breaking change in go-ethereum's usbwallet API is unlikely but would require a small adapter layer.

## Sources

[1] ethereum/go-ethereum (core/types, accounts/usbwallet/ledger.go, accounts/abi, ethclient) — https://github.com/ethereum/go-ethereum — Primary reference for tx construction, RLP, ABI, and Ledger integration (accessed May 2026).

[2] ethereum/staking-deposit-cli (settings.py and network definitions) — https://github.com/ethereum/staking-deposit-cli — Authoritative deposit contract addresses and chain IDs.

[3] Ledger Ethereum app documentation and community reports on blind signing / contract data (2024–2026).

[4] Foundry cast documentation — prior art for unsigned tx / two-phase signing flows.

[5] Existing eth-utils Go codebase (`go/cmd/eth-deposit-gen/main.go`, `go/internal/network/network.go`, go.mod, security/zeroization patterns) — CLI idioms, deposit JSON schema, network handling, and security expectations.

[6] ethereum/consensus-specs (deposit contract and fork versions) — https://github.com/ethereum/consensus-specs

---

*Research complete. All 7 investigation areas + feasibility covered with actionable recommendations, code sketches, exact import paths, version guidance, CGO flags, and open questions for the architect.*