# Research: BLS & SSZ Libraries for Go

This document surveys Go libraries for the cryptographic primitives required by the Ethereum validator deposit data generator described in `docs/prd.md`: BLS12-381 signatures (Ethereum's Proof-of-Possession scheme) and SSZ `hash_tree_root` computation for `DepositMessage` / `DepositData`.

## BLS12-381 Libraries for Go (Ethereum PoP scheme)

All Ethereum consensus-layer signatures use the BLS signature scheme identified by the ciphersuite string:

```
BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_
```

This is "BLS signatures on BLS12-381, signatures in G2, hash-to-curve with SHA-256 (SSWU + Random Oracle), Proof-of-Possession variant." Any library used must implement this exact ciphersuite — alternatives such as the `_NUL_` (basic) or `_AUG_` (message-augmentation) variants produce signatures that the Ethereum deposit contract and beacon chain will reject.

### Candidates

#### 1. `github.com/herumi/bls-eth-go-binary`

- CGO bindings around the well-known C++ `herumi/bls` library.
- Ships with the Ethereum-flavored ciphersuite already pre-selected (`bls.Init(bls.BLS12_381)` + `bls.SetETHmode(bls.EthModeDraft07)`).
- Used historically by Prysm and many third-party Go staking tools (Rocket Pool node, ethdo, etc.).
- Mature, deployed at scale on mainnet for years.
- Trade-off: CGO requirement makes cross-compilation slightly more involved (must use `CC=...` toolchains for cross-builds; `CGO_ENABLED=1`).

#### 2. `github.com/supranational/blst` (Go bindings)

- Also CGO, wrapping the highly-optimized `blst` C/assembly implementation by Supranational.
- Substantially faster than herumi on most hardware (notably so for batch verification and aggregation).
- Adopted by Prysm v4+ as the default backend and used by other newer clients.
- Implements the same `BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_` scheme.
- Trade-off: lower-level API surface; the consumer must handle proof-of-possession and ciphersuite domain-separation tags explicitly. CGO + assembly may complicate some build environments.

#### 3. `github.com/prysmaticlabs/prysm/v5/crypto/bls`

- A thin Go wrapper that selects between `herumi` and `blst` at compile time via build tags.
- Provides a uniform Ethereum-specific API (`bls.SecretKey`, `bls.PublicKey`, `bls.Signature`).
- Pulling in this package transitively imports a large portion of the Prysm monorepo and its replace-directives, which is heavy for a small CLI.

### Recommendation

Use **`github.com/herumi/bls-eth-go-binary`**.

Rationale:
- The simplest path to a correct Ethereum-flavored BLS signing & verification surface in Go: one `Init` + `SetETHmode` call and the API is "Ethereum-shaped."
- Multi-year track record in mainnet staking tooling — the bar this CLI must clear is interoperability with the deposit contract and the Launchpad, both of which have been exercised against herumi for the lifetime of Ethereum's PoS.
- Avoids dragging in Prysm's heavy module graph.
- The performance gap vs. blst does not matter for this tool: even ~200 deposits/sec on herumi is far beyond the PRD throughput requirement.

Note the CGO requirement (`CGO_ENABLED=1`). Release builds must publish per-OS / per-arch binaries (the PRD already calls this out under "Compatibility").

If at some later point batch verification of thousands of signatures becomes a bottleneck, swapping to `blst` is a straightforward 1–2 day migration because the call sites are tiny (one signer, one verifier).

## SSZ Hashing for `DepositMessage` and `DepositData`

The deposit generator only needs `hash_tree_root` for two extremely small fixed-size structs. The full SSZ surface (variable-length lists, unions, containers-of-containers, merkle proofs) is not required.

### Struct shapes

```
DepositMessage:
    pubkey:                 Bytes48
    withdrawal_credentials: Bytes32
    amount:                 uint64

DepositData:
    pubkey:                 Bytes48
    withdrawal_credentials: Bytes32
    amount:                 uint64
    signature:              Bytes96
```

Plus two helpers also needed for domain computation:

```
ForkData:
    current_version:        Bytes4
    genesis_validators_root: Bytes32

SigningData:
    object_root:            Bytes32
    domain:                 Bytes32
```

All fields are fixed-size — there are no variable-length collections, so SSZ `hash_tree_root` reduces to "pack fields into 32-byte chunks, pad to a power-of-two number of chunks with zero chunks, merkleize via SHA-256."

### Candidate libraries

#### `github.com/ferranbt/fastssz` (a.k.a. `github.com/prysmaticlabs/fastssz`)

- Code-generation–based: define structs with tags, run `sszgen`, get a generated `HashTreeRoot` method.
- Used by many ETH2 projects in production; correctness against the consensus spec test vectors is well established.
- Trade-off: introduces a build-time codegen step and a non-trivial dependency for what is, in this tool's case, four tiny structs.

#### Hand-rolled `hash_tree_root`

- For fixed-size structs the algorithm is straightforward and auditable in well under 100 lines:
  1. Encode each field as a sequence of 32-byte chunks (left-padded big-endian for `uint64`; raw bytes split into 32-byte chunks for `BytesN`).
  2. Concatenate field chunks in declaration order.
  3. Right-pad with zero chunks to the next power of two.
  4. Merkleize: repeatedly hash adjacent pairs with SHA-256 until one 32-byte root remains.

### Exact chunk layout

`DepositMessage` (3 fields → 4 chunks before padding → already a power of two):

| Chunk | Source                                          |
|------:|--------------------------------------------------|
|     0 | `pubkey[0:32]`                                   |
|     1 | `pubkey[32:48]` right-padded with 16 zero bytes  |
|     2 | `withdrawal_credentials` (32 bytes)              |
|     3 | `uint64_le(amount)` right-padded with 24 zeros   |

Tree (depth 2):
```
        root
       /    \
      h01    h23
     /  \   /  \
    c0  c1 c2  c3
```

`DepositData` (4 fields, pubkey = 2 chunks, withdrawal_credentials = 1, amount = 1, signature = 2 → 6 chunks, padded to 8):

| Chunk | Source                                                   |
|------:|----------------------------------------------------------|
|     0 | `pubkey[0:32]`                                           |
|     1 | `pubkey[32:48]` right-padded with 16 zero bytes          |
|     2 | `withdrawal_credentials` (32 bytes)                      |
|     3 | `uint64_le(amount)` right-padded with 24 zeros           |
|     4 | `signature[0:32]`                                        |
|     5 | `signature[32:64]`                                       |
|     6 | `signature[64:96]` — exactly 32 bytes                    |
|     7 | zero chunk (padding to next power of two)                |

Tree (depth 3): standard balanced binary merkle tree of 8 chunks → 4 → 2 → 1.

Note: the SSZ convention for `uint64` is **little-endian**, not big-endian. The 8-byte LE encoding is placed in the *low* 8 bytes of the chunk and the remaining 24 bytes are zero.

`ForkData` (2 fields → 2 chunks, already a power of two):

| Chunk | Source                                                  |
|------:|---------------------------------------------------------|
|     0 | `current_version` (4 bytes) right-padded with 28 zeros  |
|     1 | `genesis_validators_root` (32 bytes)                    |

`SigningData` (2 fields → 2 chunks):

| Chunk | Source                       |
|------:|------------------------------|
|     0 | `object_root` (32 bytes)     |
|     1 | `domain` (32 bytes)          |

### Recommendation

**Hand-roll `hash_tree_root` for these two specific structs** (plus the two helpers `ForkData` and `SigningData`).

Rationale:
- The four structs above are the *entire* SSZ surface this CLI needs. Pulling in a code-generator and its runtime to handle four trivially-fixed-size containers is disproportionate.
- The hand-rolled implementation is small enough to be reviewed in one sitting and unit-tested directly against consensus-spec test vectors (`hash_tree_root` values for known `DepositMessage` / `DepositData` inputs are widely published).
- It eliminates a class of supply-chain risk on a path that, if it produces a wrong root, irrecoverably locks 32 ETH per affected validator. Fewer dependencies → smaller blast radius.
- Golden-file tests cross-checked against `staking-deposit-cli` output (already required by the PRD) will catch any divergence from the official implementation.

If we later expand the tool to handle voluntary exits, `bls_to_execution_change`, or attestations, we should reconsider — at that point a maintained SSZ library starts paying for itself.
