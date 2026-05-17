package network_test

import (
	"strings"
	"testing"

	"github.com/rootwarp/eth-utils/go/internal/network"
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

// TestLookupMainnet verifies mainnet fork bytes byte-for-byte.
func TestLookupMainnet(t *testing.T) {
	params, err := network.Lookup(network.Mainnet)
	if err != nil {
		t.Fatalf("Lookup(Mainnet) unexpected error: %v", err)
	}
	want := [4]byte{0x00, 0x00, 0x00, 0x00}
	if params.GenesisForkVersion != want {
		t.Errorf("GenesisForkVersion = %v, want %v", params.GenesisForkVersion, want)
	}
	if params.Name != network.Mainnet {
		t.Errorf("Name = %q, want %q", params.Name, network.Mainnet)
	}
}

// TestLookup verifies Lookup for all supported networks, new fields, and error cases.
func TestLookup(t *testing.T) {
	tests := []struct {
		net                    network.Network
		wantForkVersion        [4]byte
		wantChainID            uint64
		wantDepositContractHex string
	}{
		{
			net:                    network.Mainnet,
			wantForkVersion:        [4]byte{0x00, 0x00, 0x00, 0x00},
			wantChainID:            1,
			wantDepositContractHex: "0x00000000219ab540356cbb839cbe05303d7705fa",
		},
		{
			net:                    network.Hoodi,
			wantForkVersion:        [4]byte{0x10, 0x00, 0x09, 0x10},
			wantChainID:            560048,
			wantDepositContractHex: "0x00000000219ab540356cbb839cbe05303d7705fa",
		},
		{
			net:                    network.Sepolia,
			wantForkVersion:        [4]byte{0x90, 0x00, 0x00, 0x69},
			wantChainID:            11155111,
			wantDepositContractHex: "0x7f02c3e3c98b133055b8b348b2ac625669ed295d",
		},
		{
			net:                    network.Holesky,
			wantForkVersion:        [4]byte{0x01, 0x01, 0x70, 0x00},
			wantChainID:            17000,
			wantDepositContractHex: "0x4242424242424242424242424242424242424242",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.net), func(t *testing.T) {
			params, err := network.Lookup(tc.net)
			if err != nil {
				t.Fatalf("Lookup(%q) error = %v, want nil", tc.net, err)
			}
			if params.Name != tc.net {
				t.Errorf("Name = %q, want %q", params.Name, tc.net)
			}
			if params.GenesisForkVersion != tc.wantForkVersion {
				t.Errorf("GenesisForkVersion = %v, want %v", params.GenesisForkVersion, tc.wantForkVersion)
			}
			if params.ChainID != tc.wantChainID {
				t.Errorf("ChainID = %d, want %d", params.ChainID, tc.wantChainID)
			}
			// DepositContractAddressHex returns lowercase 0x-prefixed.
			gotHex := strings.ToLower(params.DepositContractAddressHex())
			if gotHex != tc.wantDepositContractHex {
				t.Errorf("DepositContractAddressHex() = %q, want %q", gotHex, tc.wantDepositContractHex)
			}
		})
	}

	t.Run("Unknown_network_returns_descriptive_error", func(t *testing.T) {
		unknown := network.Network("goerli")
		_, err := network.Lookup(unknown)
		if err == nil {
			t.Fatal("Lookup(unknown) error = nil, want error")
		}
		if !strings.Contains(err.Error(), "goerli") {
			t.Errorf("error %q does not mention the unknown network name", err.Error())
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
		{"sepolia", network.Sepolia},
		{"holesky", network.Holesky},
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
		"goerli",
		"Mainnet",
		" mainnet",
		"MAINNET",
		"SEPOLIA",
		"Holesky",
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

// TestDepositContractAddressHex verifies the hex formatting helper.
func TestDepositContractAddressHex(t *testing.T) {
	params, err := network.Lookup(network.Holesky)
	if err != nil {
		t.Fatalf("Lookup(Holesky) error = %v", err)
	}
	got := params.DepositContractAddressHex()
	if !strings.HasPrefix(got, "0x") {
		t.Errorf("DepositContractAddressHex() = %q: missing 0x prefix", got)
	}
	// hex body is 40 chars (20 bytes)
	if len(got) != 42 {
		t.Errorf("DepositContractAddressHex() len = %d, want 42", len(got))
	}
}
