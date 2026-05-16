// Package network is the source of truth for per-network compile-time constants
// used in the deposit signing pipeline.
package network

import (
	"fmt"
)

// Network identifies an Ethereum consensus network.
type Network string

const (
	// Mainnet is the Ethereum mainnet.
	Mainnet Network = "mainnet"

	// Hoodi is the Hoodi testnet.
	Hoodi Network = "hoodi"
)

// Params holds the per-network constants required by the deposit signing pipeline.
type Params struct {
	Name               Network
	GenesisForkVersion [4]byte
}

// DomainDeposit is the 4-byte SSZ domain type for deposits (consensus spec constant).
var DomainDeposit = [4]byte{0x03, 0x00, 0x00, 0x00}

// ZeroGenesisValidatorsRoot is the genesis_validators_root used for deposit
// signing — always 32 zero bytes per the consensus spec.
var ZeroGenesisValidatorsRoot = [32]byte{}

// Lookup returns the Params for the given network.
// It returns a descriptive error for any unknown network.
func Lookup(n Network) (Params, error) {
	switch n {
	case Mainnet:
		return Params{
			Name:               Mainnet,
			GenesisForkVersion: [4]byte{0x00, 0x00, 0x00, 0x00},
		}, nil
	case Hoodi:
		return Params{
			Name:               Hoodi,
			GenesisForkVersion: [4]byte{0x10, 0x00, 0x09, 0x10},
		}, nil
	default:
		return Params{}, fmt.Errorf("unknown network %q: must be %q or %q", n, Mainnet, Hoodi)
	}
}

// ParseFlag parses a network flag string and returns the corresponding Network.
// It accepts exactly "mainnet" and "hoodi" (case-sensitive). Any other input
// returns an error containing the offending value.
func ParseFlag(s string) (Network, error) {
	switch s {
	case string(Mainnet):
		return Mainnet, nil
	case string(Hoodi):
		return Hoodi, nil
	default:
		return "", fmt.Errorf("unsupported network %q: must be %q or %q", s, Mainnet, Hoodi)
	}
}
