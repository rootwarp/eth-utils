package tx

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
)

// TestGolden_Phase2Holesky_DecodeAndVerify loads the phase2/holesky fixture,
// runs Builder.BuildUnsigned, then manually decodes the ABI calldata and
// verifies each segment matches the input entry.
func TestGolden_Phase2Holesky_DecodeAndVerify(t *testing.T) {
	fixturePath := findPhase2Fixture(t)
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("could not read fixture: %v", err)
	}

	var entries []struct {
		Pubkey                string `json:"pubkey"`
		WithdrawalCredentials string `json:"withdrawal_credentials"`
		Amount                uint64 `json:"amount"`
		Signature             string `json:"signature"`
		DepositDataRoot       string `json:"deposit_data_root"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("could not parse fixture: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("fixture has no entries")
	}
	e := entries[0]

	var entry deposit.Entry
	pubkeyBytes, err := hex.DecodeString(e.Pubkey)
	if err != nil || len(pubkeyBytes) != 48 {
		t.Fatalf("bad pubkey: %v", err)
	}
	copy(entry.Pubkey[:], pubkeyBytes)

	wcBytes, err := hex.DecodeString(e.WithdrawalCredentials)
	if err != nil || len(wcBytes) != 32 {
		t.Fatalf("bad withdrawal_credentials: %v", err)
	}
	copy(entry.WithdrawalCredentials[:], wcBytes)

	sigBytes, err := hex.DecodeString(e.Signature)
	if err != nil || len(sigBytes) != 96 {
		t.Fatalf("bad signature: %v", err)
	}
	copy(entry.Signature[:], sigBytes)

	rootBytes, err := hex.DecodeString(e.DepositDataRoot)
	if err != nil || len(rootBytes) != 32 {
		t.Fatalf("bad deposit_data_root: %v", err)
	}
	copy(entry.DepositDataRoot[:], rootBytes)

	entry.Amount = e.Amount

	params := holeskyParams(t)
	nonce := uint64(0)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(20_000_000_000),
		MaxPriorityFeePerGas: big.NewInt(1_000_000_000),
		Nonce:                &nonce,
	}

	b := NewBuilder()
	tx, err := b.BuildUnsigned(context.Background(), entry, cfg)
	if err != nil {
		t.Fatalf("BuildUnsigned failed: %v", err)
	}

	// Decode calldata.
	hexData := strings.TrimPrefix(tx.Data, "0x")
	calldata, err := hex.DecodeString(hexData)
	if err != nil {
		t.Fatalf("could not decode Data hex: %v", err)
	}
	if len(calldata) != 420 {
		t.Fatalf("calldata length: got %d, want 420", len(calldata))
	}

	// Selector (bytes 0–3).
	if calldata[0] != 0x22 || calldata[1] != 0x89 || calldata[2] != 0x51 || calldata[3] != 0x18 {
		t.Errorf("selector mismatch: got %x", calldata[:4])
	}

	// deposit_data_root in head slot 3 (offset 4+96 = 100, length 32).
	gotRoot := calldata[4+96 : 4+128]
	for i, b := range entry.DepositDataRoot {
		if gotRoot[i] != b {
			t.Errorf("DepositDataRoot[%d]: got %02x, want %02x", i, gotRoot[i], b)
		}
	}

	tail := calldata[4:] // 416 bytes: 4 head slots already consumed the selector
	// pubkey tail: offset from args start = 128 bytes (4 head slots × 32); length prefix at tail[128..160], data at tail[160..208]
	pubkeyData := tail[128+32 : 128+32+48]
	for i, b := range entry.Pubkey {
		if pubkeyData[i] != b {
			t.Errorf("Pubkey[%d]: got %02x, want %02x", i, pubkeyData[i], b)
		}
	}

	// withdrawal_credentials tail: offset = 224; length prefix at tail[224..256], data at tail[256..288]
	wcData := tail[224+32 : 224+32+32]
	for i, b := range entry.WithdrawalCredentials {
		if wcData[i] != b {
			t.Errorf("WithdrawalCredentials[%d]: got %02x, want %02x", i, wcData[i], b)
		}
	}

	// signature tail: offset = 288; length prefix at tail[288..320], data at tail[320..416]
	sigData := tail[288+32 : 288+32+96]
	for i, b := range entry.Signature {
		if sigData[i] != b {
			t.Errorf("Signature[%d]: got %02x, want %02x", i, sigData[i], b)
		}
	}
}

func findPhase2Fixture(t *testing.T) string {
	t.Helper()
	// Walk up from package dir to find repo root.
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
	return filepath.Join(dir, "testdata", "phase2", "holesky", "deposit_data_single.json")
}
