package tx

import (
	"fmt"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
)

// Validate runs all input-validity checks for BuildUnsigned. Returns the
// first error encountered, or nil if the entry and config are acceptable.
//
// Length note: deposit.Entry uses fixed-size byte arrays ([48]byte, [96]byte,
// etc.), so Go's type system enforces lengths at compile time. We satisfy the
// spirit of "length validation" via zero-detection and structural format checks.
//
// BLS pubkey point-on-curve check: skipped. internal/bls.ValidatePubkeyBytes
// can do this, but enabling it requires all test fixtures to carry real G1
// points, which is a significant lift for an "optional" check.
func Validate(entry deposit.Entry, cfg BuildConfig) error {
	// Config checks first so fee-nil tests short-circuit before entry checks.
	if cfg.MaxFeePerGas == nil {
		return fmt.Errorf("BuildConfig.MaxFeePerGas: %w", ErrNilFeeField)
	}
	if cfg.MaxPriorityFeePerGas == nil {
		return fmt.Errorf("BuildConfig.MaxPriorityFeePerGas: %w", ErrNilFeeField)
	}
	if cfg.NetworkParams.ChainID == 0 {
		return ErrUnconfiguredChainID
	}

	// Amount check.
	if entry.Amount != 32_000_000_000 {
		return fmt.Errorf("%w: got %d", ErrInvalidAmount, entry.Amount)
	}

	// Zero-value detection for fixed-size fields.
	if entry.Pubkey == ([48]byte{}) {
		return ErrZeroPubkey
	}
	if entry.Signature == ([96]byte{}) {
		return ErrZeroSignature
	}
	if entry.DepositDataRoot == ([32]byte{}) {
		return ErrZeroDepositRoot
	}

	// Withdrawal credentials structural check.
	wc := entry.WithdrawalCredentials
	switch wc[0] {
	case 0x00:
		// BLS withdrawal: no further format constraint.
	case 0x01, 0x02:
		// eth1-address and compounding formats: bytes 1–11 must be zero.
		for i := 1; i <= 11; i++ {
			if wc[i] != 0x00 {
				return fmt.Errorf("%w: prefix 0x%02x requires bytes 1–11 to be zero", ErrInvalidWCFormat, wc[0])
			}
		}
	default:
		return fmt.Errorf("%w: got 0x%02x", ErrInvalidWCPrefix, wc[0])
	}

	return nil
}
