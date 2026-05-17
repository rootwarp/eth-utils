package tx

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"golang.org/x/crypto/sha3"
)

// depositSig is the canonical function signature for computing the selector.
const depositSig = "deposit(bytes,bytes,bytes,bytes32)"

// expectedCalldataLen is the exact byte length of ABI-encoded deposit() calldata.
// Layout: selector(4) + head(128) + tail(pubkey:32+64 + wc:32+32 + sig:32+96) = 4+128+288 = 420.
const expectedCalldataLen = 420

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func TestPackDeposit_SelectorMatchesKeccak256(t *testing.T) {
	got := PackDeposit([48]byte{}, [32]byte{}, [96]byte{}, [32]byte{})
	gotSelector := got[:4]
	wantSelector := keccak256([]byte(depositSig))[:4]
	if gotSelector[0] != wantSelector[0] || gotSelector[1] != wantSelector[1] ||
		gotSelector[2] != wantSelector[2] || gotSelector[3] != wantSelector[3] {
		t.Errorf("selector: got %x, want %x", gotSelector, wantSelector)
	}
}

func TestPackDeposit_Length(t *testing.T) {
	got := PackDeposit([48]byte{}, [32]byte{}, [96]byte{}, [32]byte{})
	if len(got) != expectedCalldataLen {
		t.Errorf("calldata length: got %d, want %d", len(got), expectedCalldataLen)
	}
}

func TestPackDeposit_LengthWithRandomBytes(t *testing.T) {
	var pubkey [48]byte
	var wc [32]byte
	var sig [96]byte
	var root [32]byte
	for i := range pubkey {
		pubkey[i] = byte(i)
	}
	for i := range wc {
		wc[i] = byte(i + 100)
	}
	for i := range sig {
		sig[i] = byte(i * 2)
	}
	for i := range root {
		root[i] = byte(i * 3)
	}
	got := PackDeposit(pubkey, wc, sig, root)
	if len(got) != expectedCalldataLen {
		t.Errorf("calldata length with random bytes: got %d, want %d", len(got), expectedCalldataLen)
	}
}

func TestPackDeposit_RoundTrip(t *testing.T) {
	var pubkey [48]byte
	var wc [32]byte
	var sig [96]byte
	var root [32]byte
	for i := range pubkey {
		pubkey[i] = 0xaa
	}
	for i := range wc {
		wc[i] = 0xbb
	}
	for i := range sig {
		sig[i] = 0xcc
	}
	for i := range root {
		root[i] = 0xee
	}

	got := PackDeposit(pubkey, wc, sig, root)

	// Verify selector.
	wantSel, _ := hex.DecodeString("22895118")
	if got[0] != wantSel[0] || got[1] != wantSel[1] || got[2] != wantSel[2] || got[3] != wantSel[3] {
		t.Errorf("selector mismatch: got %x, want 22895118", got[:4])
	}

	// Decode head: 4 slots of 32 bytes each.
	// Slot 0: offset_pubkey (uint256, big-endian 32 bytes)
	// Slot 1: offset_wc
	// Slot 2: offset_sig
	// Slot 3: deposit_data_root (static bytes32)
	head := got[4:132]

	offsetPubkey := readUint256AsUint64(head[0:32])
	offsetWC := readUint256AsUint64(head[32:64])
	offsetSig := readUint256AsUint64(head[64:96])
	gotRoot := head[96:128]

	if offsetPubkey != 128 {
		t.Errorf("offsetPubkey: got %d, want 128", offsetPubkey)
	}
	if offsetWC != 224 {
		t.Errorf("offsetWC: got %d, want 224", offsetWC)
	}
	if offsetSig != 288 {
		t.Errorf("offsetSig: got %d, want 288", offsetSig)
	}
	for i, b := range root {
		if gotRoot[i] != b {
			t.Errorf("deposit_data_root[%d]: got %02x, want %02x", i, gotRoot[i], b)
		}
	}

	// Decode tail starting at got[4] (tail offsets are relative to start of args, i.e. got[4:]).
	tail := got[4:]

	// Pubkey segment at offsetPubkey.
	pubkeyLen := readUint256AsUint64(tail[offsetPubkey : offsetPubkey+32])
	if pubkeyLen != 48 {
		t.Errorf("pubkey length: got %d, want 48", pubkeyLen)
	}
	gotPubkey := tail[offsetPubkey+32 : offsetPubkey+32+48]
	for i, b := range pubkey {
		if gotPubkey[i] != b {
			t.Errorf("pubkey[%d]: got %02x, want %02x", i, gotPubkey[i], b)
		}
	}

	// WC segment at offsetWC.
	wcLen := readUint256AsUint64(tail[offsetWC : offsetWC+32])
	if wcLen != 32 {
		t.Errorf("wc length: got %d, want 32", wcLen)
	}
	gotWC := tail[offsetWC+32 : offsetWC+32+32]
	for i, b := range wc {
		if gotWC[i] != b {
			t.Errorf("wc[%d]: got %02x, want %02x", i, gotWC[i], b)
		}
	}

	// Sig segment at offsetSig.
	sigLen := readUint256AsUint64(tail[offsetSig : offsetSig+32])
	if sigLen != 96 {
		t.Errorf("sig length: got %d, want 96", sigLen)
	}
	gotSig := tail[offsetSig+32 : offsetSig+32+96]
	for i, b := range sig {
		if gotSig[i] != b {
			t.Errorf("sig[%d]: got %02x, want %02x", i, gotSig[i], b)
		}
	}
}

// readUint256AsUint64 reads a big-endian 32-byte unsigned integer and returns the low 8 bytes as uint64.
func readUint256AsUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b[24:32])
}
