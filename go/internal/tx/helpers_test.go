package tx

import (
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/network"
)

func holeskyParams(t *testing.T) network.Params {
	t.Helper()
	p, err := network.Lookup(network.Holesky)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
