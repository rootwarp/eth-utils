package tx

import (
	"context"
	"math/big"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/network"
)

// TxBuilder constructs unsigned EIP-1559 deposit transactions.
type TxBuilder interface {
	BuildUnsigned(ctx context.Context, entry deposit.Entry, cfg BuildConfig) (*UnsignedTx, error)
}

// BuildConfig carries the parameters needed to build an unsigned transaction.
// Nonce is a pointer so callers can distinguish "not set" from "set to 0";
// nil means the implementation should resolve a nonce (e.g., via RPC in Issue 2.4).
type BuildConfig struct {
	NetworkParams        network.Params
	RPCURL               string
	GasLimit             uint64
	MaxFeePerGas         *big.Int
	MaxPriorityFeePerGas *big.Int
	Nonce                *uint64
}
