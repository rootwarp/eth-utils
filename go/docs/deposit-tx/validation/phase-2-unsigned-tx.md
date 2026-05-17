# Phase 2 — Unsigned Transaction Validation Artifact

**SYNTHETIC TEST VECTOR — NOT A REAL VALIDATOR DEPOSIT**

This document records the Phase 2 golden artifact for independent verification.
All key material is fabricated and deterministic; it does not represent a real
Beacon Chain deposit.

## Fixture location

`go/testdata/phase2/holesky/deposit_data_single.json`

## Input parameters

| Field | Value |
|-------|-------|
| Network | Holesky (chainID 17000) |
| Deposit contract | `0x4242424242424242424242424242424242424242` |
| pubkey | 48 bytes of `0xaa` |
| withdrawal_credentials | `0x01` + 11 zero bytes + 20 bytes of `0x11` |
| signature | 96 bytes of `0xcc` |
| deposit_data_root | 32 bytes of `0xee` |
| amount | 32000000000 Gwei (32 ETH) |

## Expected unsigned transaction

```json
{
  "chainId": 17000,
  "to": "0x4242424242424242424242424242424242424242",
  "value": "0x1bc16d674ec800000",
  "data": "0x22895118000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000120eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0000000000000000000000000000000000000000000000000000000000000030aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002001000000000000000000000011111111111111111111111111111111111111110000000000000000000000000000000000000000000000000000000000000060cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
  "gas": 250000,
  "maxFeePerGas": "0x4a817c800",
  "maxPriorityFeePerGas": "0x3b9aca00",
  "nonce": 0,
  "type": "0x2"
}
```

## Calldata breakdown

The `data` field is 420 bytes: `selector(4) + head(128) + tail(288)`.

**Selector** (bytes 0–3): `0x22895118`

Verify: `cast 4byte 0x22895118` → `deposit(bytes,bytes,bytes,bytes32)`

**Head** (bytes 4–131, four 32-byte slots):

| Slot | Offset from data start | Value | Meaning |
|------|------------------------|-------|---------|
| 0 | 4 | `0x80` = 128 | offset to pubkey tail |
| 1 | 36 | `0xe0` = 224 | offset to withdrawal_credentials tail |
| 2 | 68 | `0x120` = 288 | offset to signature tail |
| 3 | 100 | `0xee...ee` | deposit_data_root (static bytes32) |

**Tail** (bytes 132–419):

| Segment | Bytes | Content |
|---------|-------|---------|
| pubkey length | 132–163 | uint256(48) |
| pubkey data | 164–211 | `0xaa` × 48 |
| pubkey padding | 212–227 | zero × 16 |
| wc length | 228–259 | uint256(32) |
| wc data | 260–291 | `0x01` + 11×`0x00` + 8×`0x11`... |
| sig length | 292–323 | uint256(96) |
| sig data | 324–419 | `0xcc` × 96 |

## Independent verification

Using [cast](https://book.getfoundry.sh/reference/cast/cast-calldata-decode):

```bash
cast calldata-decode \
  'deposit(bytes,bytes,bytes,bytes32)' \
  0x22895118000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000120eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee0000000000000000000000000000000000000000000000000000000000000030aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002001000000000000000000000011111111111111111111111111111111111111110000000000000000000000000000000000000000000000000000000000000060cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
```

Expected output:
- `bytes`: `0xaaa...aaa` (48 bytes, pubkey)
- `bytes`: `0x01000...111` (32 bytes, withdrawal_credentials)
- `bytes`: `0xccc...ccc` (96 bytes, signature)
- `bytes32`: `0xeee...eee` (deposit_data_root)
