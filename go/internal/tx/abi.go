package tx

import (
	"encoding/binary"
)

// depositSelectorBytes is the 4-byte ABI selector for deposit(bytes,bytes,bytes,bytes32).
// Derived from Keccak-256("deposit(bytes,bytes,bytes,bytes32)")[:4] = 0x22895118.
// Verified in abi_test.go via sha3.NewLegacyKeccak256.
var depositSelectorBytes = [4]byte{0x22, 0x89, 0x51, 0x18}

// PackDeposit ABI-encodes a call to the deposit contract's deposit() function.
//
// ABI layout (420 bytes total):
//
//	selector (4 bytes) || head (128 bytes) || tail (288 bytes)
//
// Head — 4 slots of 32 bytes each:
//
//	[0] offset_pubkey = 128  (4 head slots × 32 bytes)
//	[1] offset_wc     = 224  (128 + 32 length + 64 padded-pubkey)
//	[2] offset_sig    = 288  (224 + 32 length + 32 wc already 32-byte aligned)
//	[3] deposit_data_root (static bytes32, inline)
//
// Tail:
//
//	uint256(48) || pubkey(48) || pad(16)  — pubkey segment (96 bytes)
//	uint256(32) || wc(32)                 — withdrawal_credentials segment (64 bytes)
//	uint256(96) || sig(96)                — signature segment (128 bytes)
func PackDeposit(pubkey [48]byte, wc [32]byte, sig [96]byte, root [32]byte) []byte {
	buf := make([]byte, 420)
	pos := 0

	// Selector.
	copy(buf[pos:], depositSelectorBytes[:])
	pos += 4

	// Head slot 0: offset_pubkey = 128.
	putUint256(buf[pos:], 128)
	pos += 32

	// Head slot 1: offset_wc = 224.
	putUint256(buf[pos:], 224)
	pos += 32

	// Head slot 2: offset_sig = 288.
	putUint256(buf[pos:], 288)
	pos += 32

	// Head slot 3: deposit_data_root (static bytes32).
	copy(buf[pos:], root[:])
	pos += 32

	// Tail — pubkey segment: uint256(48) || pubkey(48) || pad(16).
	putUint256(buf[pos:], 48)
	pos += 32
	copy(buf[pos:], pubkey[:])
	pos += 48
	pos += 16 // zero padding to 64-byte boundary

	// Tail — withdrawal_credentials segment: uint256(32) || wc(32).
	putUint256(buf[pos:], 32)
	pos += 32
	copy(buf[pos:], wc[:])
	pos += 32

	// Tail — signature segment: uint256(96) || sig(96).
	putUint256(buf[pos:], 96)
	pos += 32
	copy(buf[pos:], sig[:])

	return buf
}

// putUint256 writes v as a big-endian 32-byte unsigned integer into b.
func putUint256(b []byte, v uint64) {
	// zero the top 24 bytes; make() zeroed the slice, but be explicit.
	for i := 0; i < 24; i++ {
		b[i] = 0
	}
	binary.BigEndian.PutUint64(b[24:32], v)
}
