package signer

import (
	"context"

	"github.com/rootwarp/eth-utils/go/internal/tx"
)

// Signer abstracts the act of producing an EIP-1559 signature for an
// UnsignedTx. Concrete implementations include LocalSigner (raw private key
// from env var) and LedgerSigner (hardware wallet, CGO-gated, Phase 3.3+).
//
// SECURITY CONTRACT: implementations MUST NOT log, persist, or otherwise
// expose private key material. Errors returned to callers must not include
// raw key bytes, partial signatures, or any sensitive material.
type Signer interface {
	// Sign produces a SignedTx for the given unsigned transaction.
	// ctx is honored for cancellation (especially important for Ledger,
	// where signing can block on user confirmation).
	Sign(ctx context.Context, unsigned tx.UnsignedTx) (*SignedTx, error)

	// Name returns a short human-readable identifier for the signer
	// ("local", "ledger") — used in logs and error messages, never sensitive.
	Name() string

	// RequiresUserInteraction reports whether Sign blocks on a user action
	// (e.g., pressing buttons on a Ledger). The CLI uses this to print
	// "please confirm on device" messages.
	RequiresUserInteraction() bool

	// Close releases any resources held by the signer (HID handle for
	// Ledger, zeroized key buffer for local). Idempotent.
	Close() error
}
