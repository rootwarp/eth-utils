// Package tx — STUB (Phase 1 only; replaced by real ABI-encoding builder in Phase 2).
package tx

import (
	"math/big"
	"strings"
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/network"
)

func makeHoleskyEntry() deposit.Entry {
	var e deposit.Entry
	for i := range e.Pubkey {
		e.Pubkey[i] = 0xaa
	}
	for i := range e.WithdrawalCredentials {
		e.WithdrawalCredentials[i] = 0xbb
	}
	for i := range e.Signature {
		e.Signature[i] = 0xcc
	}
	for i := range e.DepositMessageRoot {
		e.DepositMessageRoot[i] = 0xdd
	}
	for i := range e.DepositDataRoot {
		e.DepositDataRoot[i] = 0xee
	}
	e.ForkVersion = [4]byte{0x01, 0x01, 0x70, 0x00}
	e.Amount = 32_000_000_000 // Gwei
	e.NetworkName = network.Holesky
	e.DepositCLIVersion = "2.7.0"
	return e
}

func holeskyParams(t *testing.T) network.Params {
	t.Helper()
	p, err := network.Lookup(network.Holesky)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBuildUnsigned_Success(t *testing.T) {
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := StubConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(20_000_000_000),
		MaxPriorityFeePerGas: big.NewInt(1_000_000_000),
		Nonce:                0,
	}

	tx, err := BuildUnsigned(entry, cfg)
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
	// 32 ETH = 32_000_000_000 Gwei * 1e9 = 0x1bc16d674ec800000
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
	if tx.MaxFeePerGas == "" {
		t.Error("MaxFeePerGas must not be empty")
	}
	if tx.MaxPriorityFeePerGas == "" {
		t.Error("MaxPriorityFeePerGas must not be empty")
	}
}

func TestBuildUnsigned_ChainIDMatchesNetwork(t *testing.T) {
	for _, n := range []network.Network{network.Mainnet, network.Holesky, network.Sepolia, network.Hoodi} {
		params, _ := network.Lookup(n)
		var e deposit.Entry
		for i := range e.Pubkey {
			e.Pubkey[i] = 0x01
		}
		for i := range e.Signature {
			e.Signature[i] = 0x01
		}
		for i := range e.DepositDataRoot {
			e.DepositDataRoot[i] = 0x01
		}
		e.Amount = 32_000_000_000
		e.NetworkName = n

		cfg := StubConfig{
			NetworkParams:        params,
			GasLimit:             250_000,
			MaxFeePerGas:         big.NewInt(1),
			MaxPriorityFeePerGas: big.NewInt(1),
		}
		tx, err := BuildUnsigned(e, cfg)
		if err != nil {
			t.Fatalf("network %s: BuildUnsigned error: %v", n, err)
		}
		if tx.ChainID != params.ChainID {
			t.Errorf("network %s: ChainID got %d want %d", n, tx.ChainID, params.ChainID)
		}
	}
}

func TestBuildUnsigned_ToMatchesDepositContract(t *testing.T) {
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := StubConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
	}
	tx, err := BuildUnsigned(entry, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ToLower(tx.To) != strings.ToLower(params.DepositContractAddressHex()) {
		t.Errorf("To mismatch: got %q want %q", tx.To, params.DepositContractAddressHex())
	}
}

func TestBuildUnsigned_Value32ETH(t *testing.T) {
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := StubConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
	}
	tx, err := BuildUnsigned(entry, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// 32 ETH in wei: 32 * 10^18 = 0x1bc16d674ec800000
	if tx.Value != "0x1bc16d674ec800000" {
		t.Errorf("Value: got %q, want 0x1bc16d674ec800000", tx.Value)
	}
}

func TestBuildUnsigned_CalldataStartsWithSelector(t *testing.T) {
	entry := makeHoleskyEntry()
	params := holeskyParams(t)
	cfg := StubConfig{
		NetworkParams:        params,
		GasLimit:             250_000,
		MaxFeePerGas:         big.NewInt(1),
		MaxPriorityFeePerGas: big.NewInt(1),
	}
	tx, err := BuildUnsigned(entry, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tx.Data, "0x22895118") {
		t.Errorf("Data must start with deposit() selector 0x22895118, got: %q", tx.Data)
	}
}
