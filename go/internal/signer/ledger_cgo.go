//go:build cgo

package signer

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/core/types"
)

func init() {
	newLedgerHub = func() (ledgerHub, error) {
		hub, err := usbwallet.NewLedgerHub()
		if err != nil {
			return nil, err
		}
		return &realHub{hub: hub}, nil
	}
}

// realHub wraps *usbwallet.Hub and satisfies ledgerHub.
type realHub struct {
	hub *usbwallet.Hub
}

func (h *realHub) Wallets() []ledgerWallet {
	ws := h.hub.Wallets()
	out := make([]ledgerWallet, len(ws))
	for i, w := range ws {
		out[i] = &realWallet{w: w}
	}
	return out
}

// realWallet wraps accounts.Wallet and satisfies ledgerWallet.
type realWallet struct {
	w accounts.Wallet
}

func (r *realWallet) URL() accounts.URL { return r.w.URL() }

func (r *realWallet) Open(passphrase string) error { return r.w.Open(passphrase) }

func (r *realWallet) Close() error { return r.w.Close() }

func (r *realWallet) Status() (string, error) { return r.w.Status() }

func (r *realWallet) Derive(path accounts.DerivationPath, pin bool) (accounts.Account, error) {
	return r.w.Derive(path, pin)
}

func (r *realWallet) SignTx(account accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	return r.w.SignTx(account, tx, chainID)
}
