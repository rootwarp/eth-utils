package tx

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/network"
)

// TestBuilderSatisfiesTxBuilder is an explicit runtime assertion complementing
// the compile-time var _ check in builder.go.
func TestBuilderSatisfiesTxBuilder(t *testing.T) {
	var _ TxBuilder = NewBuilder()
}

func TestBuilder_BuildUnsigned_Success(t *testing.T) {
	ctx := context.Background()
	entry := makeHoleskyEntry()
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
	entry := makeHoleskyEntry()
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
	//nolint:staticcheck
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

func TestBuilder_BuildUnsigned_ChainIDMatchesNetwork(t *testing.T) {
	ctx := context.Background()
	for _, n := range []network.Network{network.Mainnet, network.Holesky, network.Sepolia, network.Hoodi} {
		params, _ := network.Lookup(n)
		var e deposit.Entry
		for i := range e.Pubkey {
			e.Pubkey[i] = 0x01
		}
		for i := range e.Signature {
			e.Signature[i] = 0x01
		}
		e.Amount = 32_000_000_000
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
