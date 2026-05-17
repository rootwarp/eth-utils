// Package tx — STUB (Phase 1 only; replaced by real ABI-encoding builder in Phase 2).
//
// This file exists to provide a structurally correct unsigned transaction for
// the Phase 1 vertical slice. It does NOT perform real ABI encoding. The
// calldata contains the real deposit() selector followed by a deterministic
// placeholder derived from the entry fields (pubkey + withdrawal_credentials +
// signature — no ABI padding). Phase 2 will replace this file entirely.
package tx

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/network"
)

// ErrNilFeeField is returned when MaxFeePerGas or MaxPriorityFeePerGas is nil.
var ErrNilFeeField = errors.New("fee field must not be nil")

// depositSelector is the 4-byte Keccak-256 selector for deposit(bytes,bytes,bytes,bytes32).
const depositSelector = "22895118"

// StubConfig carries the parameters the stub builder needs to fill all
// non-entry fields of the unsigned transaction.
type StubConfig struct {
	// NetworkParams provides the chain ID and deposit contract address.
	NetworkParams network.Params

	// RPCURL is reserved for Phase 2 gas/nonce estimation; unused by the stub.
	RPCURL string

	// GasLimit is the EIP-1559 gas limit.
	GasLimit uint64

	// MaxFeePerGas is the EIP-1559 maximum total fee per gas in wei.
	MaxFeePerGas *big.Int

	// MaxPriorityFeePerGas is the EIP-1559 miner tip per gas in wei.
	MaxPriorityFeePerGas *big.Int

	// Nonce is the sender account nonce.
	Nonce uint64
}

// BuildUnsigned produces a structurally correct but ABI-inaccurate unsigned
// deposit transaction. The calldata begins with the real deposit() selector
// (0x22895118) followed by a deterministic placeholder: the entry's pubkey,
// withdrawal_credentials, and signature concatenated without ABI padding.
//
// Phase 2 will replace this function with one that performs real ABI encoding.
func BuildUnsigned(entry deposit.Entry, cfg StubConfig) (*UnsignedTx, error) {
	if cfg.MaxFeePerGas == nil {
		return nil, fmt.Errorf("StubConfig.MaxFeePerGas: %w", ErrNilFeeField)
	}
	if cfg.MaxPriorityFeePerGas == nil {
		return nil, fmt.Errorf("StubConfig.MaxPriorityFeePerGas: %w", ErrNilFeeField)
	}

	// Derive value in wei from the entry amount (stored in Gwei).
	// 1 Gwei = 1e9 wei.
	amountWei := new(big.Int).Mul(
		new(big.Int).SetUint64(entry.Amount),
		big.NewInt(1_000_000_000),
	)

	// Stub calldata: selector + pubkey(48) + withdrawal_credentials(32) + signature(96) = 180 bytes.
	// This is structural-only; Phase 2 will apply real ABI encoding.
	placeholder := make([]byte, 0, 48+32+96)
	placeholder = append(placeholder, entry.Pubkey[:]...)
	placeholder = append(placeholder, entry.WithdrawalCredentials[:]...)
	placeholder = append(placeholder, entry.Signature[:]...)
	data := fmt.Sprintf("0x%s%s", depositSelector, hex.EncodeToString(placeholder))

	return &UnsignedTx{
		ChainID:              cfg.NetworkParams.ChainID,
		To:                   cfg.NetworkParams.DepositContractAddressHex(),
		Value:                "0x" + fmt.Sprintf("%x", amountWei),
		Data:                 data,
		Gas:                  cfg.GasLimit,
		MaxFeePerGas:         "0x" + fmt.Sprintf("%x", cfg.MaxFeePerGas),
		MaxPriorityFeePerGas: "0x" + fmt.Sprintf("%x", cfg.MaxPriorityFeePerGas),
		Nonce:                cfg.Nonce,
		Type:                 "0x2",
	}, nil
}
