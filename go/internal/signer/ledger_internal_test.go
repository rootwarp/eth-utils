package signer

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core/types"

	internaltx "github.com/rootwarp/eth-utils/go/internal/tx"
)

func internaltxUnsigned() internaltx.UnsignedTx {
	return internaltx.UnsignedTx{ChainID: 1, To: "0x1234", Value: "0x1", Gas: 21000, Type: "0x2"}
}

// mockHub implements ledgerHub for tests.
type mockHub struct {
	wallets []ledgerWallet
}

func (m *mockHub) Wallets() []ledgerWallet { return m.wallets }

// mockWallet implements ledgerWallet for tests.
// Each method is a replaceable function field so tests can control behavior.
type mockWallet struct {
	URLFn    func() accounts.URL
	OpenFn   func(passphrase string) error
	CloseFn  func() error
	StatusFn func() (string, error)
	DeriveFn func(path accounts.DerivationPath, pin bool) (accounts.Account, error)
	SignTxFn func(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error)
}

func (m *mockWallet) URL() accounts.URL { return m.URLFn() }
func (m *mockWallet) Open(p string) error {
	if m.OpenFn != nil {
		return m.OpenFn(p)
	}
	return nil
}
func (m *mockWallet) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}
func (m *mockWallet) Status() (string, error) {
	if m.StatusFn != nil {
		return m.StatusFn()
	}
	return "ok", nil
}
func (m *mockWallet) Derive(path accounts.DerivationPath, pin bool) (accounts.Account, error) {
	if m.DeriveFn != nil {
		return m.DeriveFn(path, pin)
	}
	return accounts.Account{}, nil
}
func (m *mockWallet) SignTx(acc accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	if m.SignTxFn != nil {
		return m.SignTxFn(acc, tx, chainID)
	}
	return nil, errors.New("not implemented")
}

// withMockHub replaces newLedgerHub for the duration of a test.
func withMockHub(t *testing.T, hub ledgerHub) {
	t.Helper()
	orig := newLedgerHub
	newLedgerHub = func() (ledgerHub, error) { return hub, nil }
	t.Cleanup(func() { newLedgerHub = orig })
}

// --- Tests ---

func TestLedgerSigner_NoDevice(t *testing.T) {
	withMockHub(t, &mockHub{wallets: nil})

	_, err := NewLedgerSigner()
	if !errors.Is(err, ErrNoDevice) {
		t.Fatalf("expected ErrNoDevice, got %v", err)
	}
}

func TestLedgerSigner_AppNotOpen_FromOpen(t *testing.T) {
	w := &mockWallet{
		OpenFn: func(_ string) error { return errors.New("ledger: 6e00 app not open") },
	}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	_, err := NewLedgerSigner()
	if !errors.Is(err, ErrAppNotOpen) {
		t.Fatalf("expected ErrAppNotOpen, got %v", err)
	}
}

func TestLedgerSigner_AppNotOpen_FromStatus(t *testing.T) {
	w := &mockWallet{
		OpenFn:   func(_ string) error { return nil },
		StatusFn: func() (string, error) { return "", errors.New("ethereum app not open on device") },
	}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	_, err := NewLedgerSigner()
	if !errors.Is(err, ErrAppNotOpen) {
		t.Fatalf("expected ErrAppNotOpen, got %v", err)
	}
}

func TestLedgerSigner_OpenFailure_Generic(t *testing.T) {
	w := &mockWallet{
		OpenFn: func(_ string) error { return errors.New("usb: device disconnected") },
	}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	_, err := NewLedgerSigner()
	if !errors.Is(err, ErrNoDevice) {
		t.Fatalf("expected ErrNoDevice (wrapped), got %v", err)
	}
}

func TestLedgerSigner_DiscoverySuccess(t *testing.T) {
	w := &mockWallet{}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	s, err := NewLedgerSigner()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name() != "ledger" {
		t.Errorf("Name() = %q, want %q", s.Name(), "ledger")
	}
	if !s.RequiresUserInteraction() {
		t.Error("RequiresUserInteraction() = false, want true")
	}
	_ = s.Close()
}

func TestLedgerSigner_Sign_NotImplemented(t *testing.T) {
	w := &mockWallet{}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	s, err := NewLedgerSigner()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, signErr := s.Sign(context.Background(), internaltxUnsigned())
	if signErr == nil {
		t.Fatal("Sign() returned nil error, expected not-implemented error")
	}
}

func TestLedgerSigner_Sign_AfterClose(t *testing.T) {
	w := &mockWallet{}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	s, err := NewLedgerSigner()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = s.Close()

	_, signErr := s.Sign(context.Background(), internaltxUnsigned())
	if !errors.Is(signErr, ErrSignerClosed) {
		t.Fatalf("expected ErrSignerClosed after Close, got %v", signErr)
	}
}

func TestLedgerSigner_HubInitError(t *testing.T) {
	orig := newLedgerHub
	newLedgerHub = func() (ledgerHub, error) { return nil, errors.New("hub failed") }
	t.Cleanup(func() { newLedgerHub = orig })

	_, err := NewLedgerSigner()
	if err == nil {
		t.Fatal("expected error from hub init, got nil")
	}
}

func TestLedgerSigner_DeriveFailure(t *testing.T) {
	w := &mockWallet{
		DeriveFn: func(_ accounts.DerivationPath, _ bool) (accounts.Account, error) {
			return accounts.Account{}, errors.New("derive: device busy")
		},
	}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	_, err := NewLedgerSigner()
	if err == nil {
		t.Fatal("expected error from Derive, got nil")
	}
}

func TestLedgerSigner_Close_Idempotent(t *testing.T) {
	closeCalls := 0
	w := &mockWallet{
		CloseFn: func() error { closeCalls++; return nil },
	}
	withMockHub(t, &mockHub{wallets: []ledgerWallet{w}})

	s, err := NewLedgerSigner()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Errorf("first Close() error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second Close() error: %v", err)
	}
	if closeCalls != 1 {
		t.Errorf("wallet.Close called %d times, want 1", closeCalls)
	}
}
