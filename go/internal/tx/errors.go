package tx

import "errors"

var (
	// ErrNilFeeField is kept for callers that test error wrapping; the stub builder
	// that previously returned it has been removed. New code should use ErrMissingFeeStatic.
	ErrNilFeeField = errors.New("fee field must not be nil")

	ErrZeroPubkey = errors.New("pubkey is all zeros")
	ErrZeroSignature       = errors.New("signature is all zeros")
	ErrZeroDepositRoot     = errors.New("deposit_data_root is all zeros")
	ErrInvalidWCPrefix     = errors.New("withdrawal credentials prefix must be 0x00, 0x01, or 0x02")
	ErrInvalidWCFormat     = errors.New("withdrawal credentials format invalid for prefix")
	ErrUnconfiguredChainID = errors.New("network chain ID is zero")

	// Static-mode sentinel errors (returned when RPC == nil and a required field is missing).
	ErrMissingFeeStatic         = errors.New("MaxFeePerGas required when no RPC is provided")
	ErrMissingPriorityFeeStatic = errors.New("MaxPriorityFeePerGas required when no RPC is provided")
	ErrMissingNonceStatic       = errors.New("Nonce required when no RPC is provided")
	ErrMissingGasLimitStatic    = errors.New("GasLimit required when no RPC is provided")

	// RPC-mode sentinel errors.
	ErrMissingFromForNonce = errors.New("From address required to fetch nonce via RPC")
	ErrChainIDMismatch     = errors.New("RPC chain ID does not match configured network")
)
