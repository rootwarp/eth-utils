package tx

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
)

// ErrNilContext is returned when BuildUnsigned is called with a nil context.
var ErrNilContext = errors.New("context must not be nil")

// ErrInvalidAmount is returned when the deposit entry amount is not exactly 32 ETH (32_000_000_000 Gwei).
// Only the 32 ETH first-deposit case is supported in Phase 2.
var ErrInvalidAmount = errors.New("deposit amount must be exactly 32_000_000_000 Gwei (32 ETH)")

// value32ETH is 32 ETH expressed in wei (32 * 10^18 = 0x1bc16d674ec800000).
var value32ETH = new(big.Int).Mul(big.NewInt(32), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

// Builder is the concrete implementation of TxBuilder.
type Builder struct{}

// compile-time assertion that *Builder satisfies TxBuilder.
var _ TxBuilder = (*Builder)(nil)

// NewBuilder creates a new Builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// BuildUnsigned constructs an unsigned EIP-1559 deposit transaction with
// real ABI-encoded calldata for the deposit() function.
// It validates the entry amount, ABI-encodes the deposit parameters, and
// sets the value to exactly 32 ETH regardless of the entry amount.
func (b *Builder) BuildUnsigned(ctx context.Context, entry deposit.Entry, cfg BuildConfig) (*UnsignedTx, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if entry.Amount != 32_000_000_000 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidAmount, entry.Amount)
	}
	if cfg.MaxFeePerGas == nil {
		return nil, fmt.Errorf("BuildConfig.MaxFeePerGas: %w", ErrNilFeeField)
	}
	if cfg.MaxPriorityFeePerGas == nil {
		return nil, fmt.Errorf("BuildConfig.MaxPriorityFeePerGas: %w", ErrNilFeeField)
	}

	nonce := uint64(0)
	if cfg.Nonce != nil {
		nonce = *cfg.Nonce
	}

	calldata := PackDeposit(entry.Pubkey, entry.WithdrawalCredentials, entry.Signature, entry.DepositDataRoot)

	return &UnsignedTx{
		ChainID:              cfg.NetworkParams.ChainID,
		To:                   cfg.NetworkParams.DepositContractAddressHex(),
		Value:                "0x" + fmt.Sprintf("%x", value32ETH),
		Data:                 "0x" + hex.EncodeToString(calldata),
		Gas:                  cfg.GasLimit,
		MaxFeePerGas:         "0x" + fmt.Sprintf("%x", cfg.MaxFeePerGas),
		MaxPriorityFeePerGas: "0x" + fmt.Sprintf("%x", cfg.MaxPriorityFeePerGas),
		Nonce:                nonce,
		Type:                 "0x2",
	}, nil
}
