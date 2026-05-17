package tx

import (
	"context"
	"errors"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
)

// ErrNilContext is returned when BuildUnsigned is called with a nil context.
var ErrNilContext = errors.New("context must not be nil")

// Builder is the concrete implementation of TxBuilder. Future issues will add
// fields for an injected RPC client (Issue 2.4) and ABI encoder (Issue 2.2).
type Builder struct{}

// compile-time assertion that *Builder satisfies TxBuilder.
var _ TxBuilder = (*Builder)(nil)

// NewBuilder creates a new Builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// BuildUnsigned constructs an unsigned EIP-1559 deposit transaction.
// It delegates to the package-level BuildUnsigned stub until Issue 2.2
// replaces the stub with real ABI encoding.
func (b *Builder) BuildUnsigned(ctx context.Context, entry deposit.Entry, cfg BuildConfig) (*UnsignedTx, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	nonce := uint64(0)
	if cfg.Nonce != nil {
		nonce = *cfg.Nonce
	}

	stubCfg := StubConfig{
		NetworkParams:        cfg.NetworkParams,
		RPCURL:               cfg.RPCURL,
		GasLimit:             cfg.GasLimit,
		MaxFeePerGas:         cfg.MaxFeePerGas,
		MaxPriorityFeePerGas: cfg.MaxPriorityFeePerGas,
		Nonce:                nonce,
	}
	return BuildUnsigned(entry, stubCfg)
}
