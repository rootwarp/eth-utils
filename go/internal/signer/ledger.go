// Package signer — Ledger hardware wallet signer.
//
// Build-tag isolation: this file has no build tag and compiles everywhere.
// ledger_cgo.go (//go:build cgo) provides the real usbwallet transport.
// ledger_nocgo.go (//go:build !cgo) provides a stub returning ErrLedgerNotSupported.
//
// Note: the repo already requires CGO transitively via herumi/bls-eth-go-binary,
// so CGO_ENABLED=0 go build ./... does not succeed for the module as a whole.
// The non-CGO stub is retained for hygiene and to let callers that guard Ledger
// usage behind a flag compile without the HID transport.
//
// Coverage: ledger.go (orchestration) + ledger_internal_test.go (mock) achieve
// ≥80% for the package without exercising ledger_cgo.go (requires CGO) or
// ledger_nocgo.go (excluded when CGO is on).

package signer

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts"

	internaltx "github.com/rootwarp/eth-utils/go/internal/tx"
)

const ledgerSignerName = "ledger"

// LedgerSigner signs transactions via a Ledger hardware wallet.
// The private key never leaves the device.
//
// Construct with NewLedgerSigner. Close must be called to release the HID handle.
type LedgerSigner struct {
	wallet  ledgerWallet
	account accounts.Account
	closed  atomic.Bool
}

// NewLedgerSigner discovers the first connected Ledger, opens the Ethereum app,
// and derives the account at m/44'/60'/0'/0/0 (accounts.DefaultBaseDerivationPath).
//
// Returns ErrLedgerNotSupported if the binary was built without CGO.
// Returns ErrNoDevice if no Ledger is detected.
// Returns ErrAppNotOpen if a Ledger is found but the Ethereum app is not open.
func NewLedgerSigner() (*LedgerSigner, error) {
	hub, err := newLedgerHub()
	if err != nil {
		return nil, fmt.Errorf("ledger hub init: %w", err)
	}

	wallets := hub.Wallets()
	if len(wallets) == 0 {
		return nil, ErrNoDevice
	}

	w := wallets[0]

	if err := w.Open(""); err != nil {
		if isAppNotOpenErr(err) {
			return nil, ErrAppNotOpen
		}
		return nil, fmt.Errorf("ledger init failed: %w", ErrNoDevice)
	}

	// Check Status — Open can succeed even when the Ethereum app isn't active.
	// TODO(3.4): refine these heuristics once real hardware error messages are known.
	_, statusErr := w.Status()
	if statusErr != nil {
		if isAppNotOpenErr(statusErr) {
			_ = w.Close()
			return nil, ErrAppNotOpen
		}
		_ = w.Close()
		return nil, fmt.Errorf("ledger status check failed: %w", ErrNoDevice)
	}

	acc, err := w.Derive(accounts.DefaultBaseDerivationPath, true)
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("ledger derive failed: %w", err)
	}

	return &LedgerSigner{wallet: w, account: acc}, nil
}

// isAppNotOpenErr returns true when err suggests the Ethereum app is not open.
// Matches known APDU error codes (6e00, 6e01, 6d00) and textual hints.
// TODO(3.4): replace with exact string from real hardware test.
func isAppNotOpenErr(err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "6e00") || strings.Contains(msg, "6e01") || strings.Contains(msg, "6d00") {
		return true
	}
	return strings.Contains(msg, "app") && (strings.Contains(msg, "not") || strings.Contains(msg, "open"))
}

// Sign is a placeholder — full EIP-1559 signing is implemented in Issue 3.4.
func (s *LedgerSigner) Sign(_ context.Context, _ internaltx.UnsignedTx) (*SignedTx, error) {
	if s.closed.Load() {
		return nil, ErrSignerClosed
	}
	return nil, fmt.Errorf("ledger signing not yet implemented; wired in Issue 3.4")
}

func (s *LedgerSigner) Name() string                  { return ledgerSignerName }
func (s *LedgerSigner) RequiresUserInteraction() bool { return true }

// Close releases the HID handle. Idempotent.
func (s *LedgerSigner) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	return s.wallet.Close()
}

// Compile-time assertion.
var _ Signer = (*LedgerSigner)(nil)
