package tx

import "errors"

var (
	// ErrNilContext, ErrNilFeeField, ErrInvalidAmount are declared in builder.go
	// and stub_builder.go respectively; they are consolidated here in a later issue.

	ErrZeroPubkey          = errors.New("pubkey is all zeros")
	ErrZeroSignature       = errors.New("signature is all zeros")
	ErrZeroDepositRoot     = errors.New("deposit_data_root is all zeros")
	ErrInvalidWCPrefix     = errors.New("withdrawal credentials prefix must be 0x00, 0x01, or 0x02")
	ErrInvalidWCFormat     = errors.New("withdrawal credentials format invalid for prefix")
	ErrUnconfiguredChainID = errors.New("network chain ID is zero")
)
