package network_test

import (
	"errors"
	"testing"

	"github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen/internal/network"
)

// TestConstants verifies the compile-time byte values.
func TestConstants(t *testing.T) {
	t.Run("DomainDeposit", func(t *testing.T) {
		want := [4]byte{0x03, 0x00, 0x00, 0x00}
		if network.DomainDeposit != want {
			t.Errorf("DomainDeposit = %v, want %v", network.DomainDeposit, want)
		}
	})

	t.Run("ZeroGenesisValidatorsRoot", func(t *testing.T) {
		var want [32]byte // all zeros
		if network.ZeroGenesisValidatorsRoot != want {
			t.Errorf("ZeroGenesisValidatorsRoot = %v, want all zeros", network.ZeroGenesisValidatorsRoot)
		}
	})
}

// TestLookupHoodi verifies Hoodi fork bytes byte-for-byte.
func TestLookup(t *testing.T) {
	t.Run("Hoodi_fork_version", func(t *testing.T) {
		params, err := network.Lookup(network.Hoodi)
		if err != nil {
			t.Fatalf("Lookup(Hoodi) error = %v, want nil", err)
		}
		want := [4]byte{0x10, 0x00, 0x09, 0x10}
		if params.GenesisForkVersion != want {
			t.Errorf("GenesisForkVersion = %v, want %v", params.GenesisForkVersion, want)
		}
		if params.Name != network.Hoodi {
			t.Errorf("Name = %q, want %q", params.Name, network.Hoodi)
		}
	})

	t.Run("Mainnet_returns_ErrMainnetNotEnabled", func(t *testing.T) {
		_, err := network.Lookup(network.Mainnet)
		if err == nil {
			t.Fatal("Lookup(Mainnet) error = nil, want ErrMainnetNotEnabled")
		}
		if !errors.Is(err, network.ErrMainnetNotEnabled) {
			t.Errorf("Lookup(Mainnet) error = %v, want errors.Is(err, ErrMainnetNotEnabled)", err)
		}
	})

	t.Run("Unknown_network_returns_descriptive_error", func(t *testing.T) {
		unknown := network.Network("sepolia")
		_, err := network.Lookup(unknown)
		if err == nil {
			t.Fatal("Lookup(unknown) error = nil, want error")
		}
		// Error must not be ErrMainnetNotEnabled
		if errors.Is(err, network.ErrMainnetNotEnabled) {
			t.Errorf("Lookup(unknown) error should not be ErrMainnetNotEnabled")
		}
	})
}

// TestParseFlag verifies case-sensitive exact matching.
func TestParseFlag(t *testing.T) {
	validCases := []struct {
		input string
		want  network.Network
	}{
		{"mainnet", network.Mainnet},
		{"hoodi", network.Hoodi},
	}

	for _, tc := range validCases {
		tc := tc
		t.Run("valid_"+tc.input, func(t *testing.T) {
			got, err := network.ParseFlag(tc.input)
			if err != nil {
				t.Fatalf("ParseFlag(%q) error = %v, want nil", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseFlag(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}

	invalidCases := []string{
		"",
		"HOODI",
		"mainnet ",
		"sepolia",
		"Mainnet",
		" mainnet",
		"MAINNET",
	}

	for _, input := range invalidCases {
		input := input
		t.Run("invalid_"+input, func(t *testing.T) {
			_, err := network.ParseFlag(input)
			if err == nil {
				t.Errorf("ParseFlag(%q) error = nil, want error", input)
			}
		})
	}
}
