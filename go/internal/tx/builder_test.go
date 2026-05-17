package tx

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/network"
)

// TestBuilderSatisfiesTxBuilder is an explicit runtime assertion complementing
// the compile-time var _ check in builder.go.
func TestBuilderSatisfiesTxBuilder(t *testing.T) {
	var _ TxBuilder = NewBuilder()
}

func TestBuilder_BuildUnsigned_Success(t *testing.T) {
	ctx := context.Background()
	entry := makeValidEntry()
	params := holeskyParams(t)

	nonce := uint64(3)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(20_000_000_000),
		MaxPriorityFeePerGas: big.NewInt(1_000_000_000),
		Nonce:                &nonce,
	}

	b := NewBuilder()
	tx, err := b.BuildUnsigned(ctx, entry, cfg)
	if err != nil {
		t.Fatalf("BuildUnsigned returned error: %v", err)
	}
	if tx == nil {
		t.Fatal("BuildUnsigned returned nil tx")
	}
	if tx.ChainID != params.ChainID {
		t.Errorf("ChainID: got %d, want %d", tx.ChainID, params.ChainID)
	}
	wantTo := strings.ToLower(params.DepositContractAddressHex())
	if strings.ToLower(tx.To) != wantTo {
		t.Errorf("To: got %q, want %q", tx.To, wantTo)
	}
	if tx.Value != "0x1bc16d674ec800000" {
		t.Errorf("Value: got %q, want 0x1bc16d674ec800000", tx.Value)
	}
	if !strings.HasPrefix(tx.Data, "0x22895118") {
		t.Errorf("Data: must start with deposit() selector 0x22895118, got %q", tx.Data)
	}
	if tx.Gas != 250_000 {
		t.Errorf("Gas: got %d, want 250000", tx.Gas)
	}
	if tx.Type != "0x2" {
		t.Errorf("Type: got %q, want 0x2", tx.Type)
	}
	if tx.Nonce != nonce {
		t.Errorf("Nonce: got %d, want %d", tx.Nonce, nonce)
	}
}

func TestBuilder_BuildUnsigned_NilNonce_DefaultsToZero(t *testing.T) {
	ctx := context.Background()
	entry := makeValidEntry()
	params := holeskyParams(t)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
		Nonce:                nil,
	}

	b := NewBuilder()
	tx, err := b.BuildUnsigned(ctx, entry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Nonce != 0 {
		t.Errorf("Nonce: got %d, want 0", tx.Nonce)
	}
}

func TestBuilder_BuildUnsigned_NilContext(t *testing.T) {
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
	}

	b := NewBuilder()
	//lint:ignore SA1012 intentionally testing nil context rejection
	_, err := b.BuildUnsigned(nil, entry, cfg)
	if err == nil {
		t.Fatal("expected error for nil context, got nil")
	}
	if !errors.Is(err, ErrNilContext) {
		t.Errorf("expected ErrNilContext, got: %v", err)
	}
}

func TestBuilder_BuildUnsigned_NilMaxFeePerGas(t *testing.T) {
	ctx := context.Background()
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         nil,
		MaxPriorityFeePerGas: big.NewInt(1),
	}

	b := NewBuilder()
	_, err := b.BuildUnsigned(ctx, entry, cfg)
	if err == nil {
		t.Fatal("expected error for nil MaxFeePerGas, got nil")
	}
	if !errors.Is(err, ErrNilFeeField) {
		t.Errorf("expected ErrNilFeeField, got: %v", err)
	}
}

func TestBuilder_BuildUnsigned_NilMaxPriorityFeePerGas(t *testing.T) {
	ctx := context.Background()
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: nil,
	}

	b := NewBuilder()
	_, err := b.BuildUnsigned(ctx, entry, cfg)
	if err == nil {
		t.Fatal("expected error for nil MaxPriorityFeePerGas, got nil")
	}
	if !errors.Is(err, ErrNilFeeField) {
		t.Errorf("expected ErrNilFeeField, got: %v", err)
	}
}

func TestBuilder_BuildUnsigned_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
	}
	b := NewBuilder()
	_, err := b.BuildUnsigned(ctx, entry, cfg)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestBuilder_BuildUnsigned_WrongAmount(t *testing.T) {
	ctx := context.Background()
	entry := makeHoleskyEntry()
	entry.Amount = 1_000_000_000 // 1 ETH in Gwei, not 32
	params := holeskyParams(t)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
	}
	b := NewBuilder()
	_, err := b.BuildUnsigned(ctx, entry, cfg)
	if err == nil {
		t.Fatal("expected error for wrong amount, got nil")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount, got: %v", err)
	}
}

func TestBuilder_BuildUnsigned_DataLength(t *testing.T) {
	ctx := context.Background()
	entry := makeValidEntry()
	params := holeskyParams(t)
	nonce := uint64(0)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
		Nonce:                &nonce,
	}
	b := NewBuilder()
	tx, err := b.BuildUnsigned(ctx, entry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "0x" + 420 bytes * 2 hex chars = 2 + 840 = 842 chars
	if len(tx.Data) != 842 {
		t.Errorf("Data length: got %d, want 842", len(tx.Data))
	}
}

func TestBuilder_BuildUnsigned_RoundTrip(t *testing.T) {
	ctx := context.Background()
	entry := makeValidEntry()
	params := holeskyParams(t)
	nonce := uint64(0)
	cfg := BuildConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
		Nonce:                &nonce,
	}
	b := NewBuilder()
	tx, err := b.BuildUnsigned(ctx, entry, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Decode calldata from hex.
	hexData := strings.TrimPrefix(tx.Data, "0x")
	raw, err := hex.DecodeString(hexData)
	if err != nil {
		t.Fatalf("failed to decode Data hex: %v", err)
	}
	if len(raw) != 420 {
		t.Fatalf("raw calldata length: got %d, want 420", len(raw))
	}

	// Verify deposit_data_root in head slot 3 (bytes 4+96..4+128).
	gotRoot := raw[4+96 : 4+128]
	for i, b := range entry.DepositDataRoot {
		if gotRoot[i] != b {
			t.Errorf("DepositDataRoot[%d]: got %02x, want %02x", i, gotRoot[i], b)
		}
	}

	// Verify pubkey in tail (offset 128 from args start = raw[4+128..]).
	tail := raw[4:]
	pubkeyData := tail[128+32 : 128+32+48]
	for i, b := range entry.Pubkey {
		if pubkeyData[i] != b {
			t.Errorf("Pubkey[%d]: got %02x, want %02x", i, pubkeyData[i], b)
		}
	}

	// Verify withdrawal_credentials in tail (offset 224).
	wcData := tail[224+32 : 224+32+32]
	for i, b := range entry.WithdrawalCredentials {
		if wcData[i] != b {
			t.Errorf("WithdrawalCredentials[%d]: got %02x, want %02x", i, wcData[i], b)
		}
	}

	// Verify signature in tail (offset 288).
	sigData := tail[288+32 : 288+32+96]
	for i, b := range entry.Signature {
		if sigData[i] != b {
			t.Errorf("Signature[%d]: got %02x, want %02x", i, sigData[i], b)
		}
	}
}

func TestBuilder_BuildUnsigned_ChainIDMatchesNetwork(t *testing.T) {
	ctx := context.Background()
	for _, n := range []network.Network{network.Mainnet, network.Holesky, network.Sepolia, network.Hoodi} {
		params, _ := network.Lookup(n)
		e := makeValidEntry()
		e.NetworkName = n

		cfg := BuildConfig{
			NetworkParams:        params,
			GasLimit:             250_000,
			MaxFeePerGas:         big.NewInt(1),
			MaxPriorityFeePerGas: big.NewInt(1),
		}
		b := NewBuilder()
		tx, err := b.BuildUnsigned(ctx, e, cfg)
		if err != nil {
			t.Fatalf("network %s: BuildUnsigned error: %v", n, err)
		}
		if tx.ChainID != params.ChainID {
			t.Errorf("network %s: ChainID got %d want %d", n, tx.ChainID, params.ChainID)
		}
	}
}

// TestBuilder_BuildUnsigned_ValidationWiredIn asserts that Validate is called by
// BuildUnsigned: a malformed entry (all-zero pubkey) must return ErrZeroPubkey.
func TestBuilder_BuildUnsigned_ValidationWiredIn(t *testing.T) {
	ctx := context.Background()
	entry := makeValidEntry()
	entry.Pubkey = [48]byte{} // trigger ErrZeroPubkey
	cfg := makeValidConfig(t)

	b := NewBuilder()
	_, err := b.BuildUnsigned(ctx, entry, cfg)
	if err == nil {
		t.Fatal("expected validation error for all-zero pubkey, got nil")
	}
	if !errors.Is(err, ErrZeroPubkey) {
		t.Errorf("expected ErrZeroPubkey, got: %v", err)
	}
}
