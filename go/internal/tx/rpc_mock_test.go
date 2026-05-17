package tx

import (
	"context"
	"math/big"
)

// mockRPC is a test double for EthRPC using the function-field pattern.
// Set each Fn field to control per-call behavior.
type mockRPC struct {
	SuggestGasTipCapFn func(ctx context.Context) (*big.Int, error)
	BlockBaseFeeFn     func(ctx context.Context) (*big.Int, error)
	PendingNonceAtFn   func(ctx context.Context, account [20]byte) (uint64, error)
	EstimateGasFn      func(ctx context.Context, msg CallMsg) (uint64, error)
	ChainIDFn          func(ctx context.Context) (*big.Int, error)
	CloseFn            func()
}

func (m *mockRPC) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return m.SuggestGasTipCapFn(ctx)
}

func (m *mockRPC) BlockBaseFee(ctx context.Context) (*big.Int, error) {
	return m.BlockBaseFeeFn(ctx)
}

func (m *mockRPC) PendingNonceAt(ctx context.Context, account [20]byte) (uint64, error) {
	return m.PendingNonceAtFn(ctx, account)
}

func (m *mockRPC) EstimateGas(ctx context.Context, msg CallMsg) (uint64, error) {
	return m.EstimateGasFn(ctx, msg)
}

func (m *mockRPC) ChainID(ctx context.Context) (*big.Int, error) {
	return m.ChainIDFn(ctx)
}

func (m *mockRPC) Close() {
	if m.CloseFn != nil {
		m.CloseFn()
	}
}

// compile-time assertion
var _ EthRPC = (*mockRPC)(nil)
