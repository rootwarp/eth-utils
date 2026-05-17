package tx

import (
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/network"
)

// makeHoleskyEntry returns a deposit.Entry with synthetic bytes for Holesky.
// Uses 0xbb withdrawal_credentials (BLS prefix 0x00 is not valid with the new
// Validate; this helper is kept for historical test coverage of the ABI layer
// only — builder_test.go uses makeValidEntry() from validation_test.go).
func makeHoleskyEntry() deposit.Entry {
	var e deposit.Entry
	for i := range e.Pubkey {
		e.Pubkey[i] = 0xaa
	}
	for i := range e.WithdrawalCredentials {
		e.WithdrawalCredentials[i] = 0xbb
	}
	for i := range e.Signature {
		e.Signature[i] = 0xcc
	}
	for i := range e.DepositMessageRoot {
		e.DepositMessageRoot[i] = 0xdd
	}
	for i := range e.DepositDataRoot {
		e.DepositDataRoot[i] = 0xee
	}
	e.ForkVersion = [4]byte{0x01, 0x01, 0x70, 0x00}
	e.Amount = 32_000_000_000
	e.NetworkName = network.Holesky
	e.DepositCLIVersion = "2.7.0"
	return e
}

func holeskyParams(t *testing.T) network.Params {
	t.Helper()
	p, err := network.Lookup(network.Holesky)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
