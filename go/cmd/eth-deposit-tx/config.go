package main

import (
	"fmt"
	"math/big"
	"strconv"

	ucli "github.com/urfave/cli/v2"

	"github.com/rootwarp/eth-utils/go/internal/network"
)

// defaultGasLimit is the default gas limit for a deposit() call.
// The deposit() function costs ~200,000 gas; 250,000 provides comfortable headroom.
const defaultGasLimit uint64 = 250_000

// Config holds the validated, parsed inputs for eth-deposit-tx build.
type Config struct {
	// Network is the selected Ethereum consensus network.
	Network network.Network

	// NetworkParams is the resolved per-network constants (chain ID, deposit contract, etc.).
	NetworkParams network.Params

	// InputFile is the path to the deposit_data JSON file, or "-" for stdin.
	InputFile string

	// OutputFile is the output path for the unsigned transaction. Empty means stdout.
	OutputFile string

	// Index is the zero-based index into the deposit_data JSON array.
	Index int

	// RPCURL is an optional JSON-RPC endpoint for gas/nonce estimation.
	// Empty means the caller must supply all gas/nonce flags explicitly.
	RPCURL string

	// GasLimit is the EIP-1559 gas limit for the deposit transaction.
	GasLimit uint64

	// MaxFeePerGas is the EIP-1559 maximum total fee in wei. Nil if not set.
	MaxFeePerGas *big.Int

	// MaxPriorityFeePerGas is the EIP-1559 miner tip in wei. Nil if not set.
	MaxPriorityFeePerGas *big.Int

	// Nonce is an optional explicit nonce override. Nil means fetch from RPC or require manual flag.
	Nonce *uint64
}

// LoadBuildConfig resolves flag > env > defaults into a typed Config.
// It validates the result before returning. Unknown network or invalid numeric
// inputs produce an error with exit code 2 via ucli.Exit so callers can return
// the error directly to urfave.
//
// TODO(1.5): replace the literal exit code 2 with the exit-code constants.
func LoadBuildConfig(c *ucli.Context) (*Config, error) {
	// 1. Network — parse and look up constants.
	net, err := network.ParseFlag(c.String("network"))
	if err != nil {
		return nil, ucli.Exit(fmt.Sprintf("--network: %v", err), 2)
	}
	params, err := network.Lookup(net)
	if err != nil {
		return nil, ucli.Exit(fmt.Sprintf("--network: %v", err), 2)
	}

	// 2. Gas limit — string flag so env-var override works alongside flag.
	gasLimit := defaultGasLimit
	if s := c.String("gas-limit"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, ucli.Exit(fmt.Sprintf("--gas-limit: invalid value %q: must be a positive integer", s), 2)
		}
		gasLimit = v
	}

	// 3. Max fee per gas — optional, nil when absent.
	var maxFee *big.Int
	if s := c.String("max-fee-per-gas"); s != "" {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, ucli.Exit(fmt.Sprintf("--max-fee-per-gas: invalid value %q: must be a decimal integer in wei", s), 2)
		}
		maxFee = v
	}

	// 4. Max priority fee per gas — optional, nil when absent.
	var maxPrioFee *big.Int
	if s := c.String("max-priority-fee-per-gas"); s != "" {
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, ucli.Exit(fmt.Sprintf("--max-priority-fee-per-gas: invalid value %q: must be a decimal integer in wei", s), 2)
		}
		maxPrioFee = v
	}

	// 5. Nonce — optional, nil when absent.
	var nonce *uint64
	if s := c.String("nonce"); s != "" {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, ucli.Exit(fmt.Sprintf("--nonce: invalid value %q: must be a non-negative integer", s), 2)
		}
		nonce = &v
	}

	cfg := &Config{
		Network:              net,
		NetworkParams:        params,
		InputFile:            c.String("input-file"),
		OutputFile:           c.String("output"),
		Index:                c.Int("index"),
		RPCURL:               c.String("rpc-url"),
		GasLimit:             gasLimit,
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: maxPrioFee,
		Nonce:                nonce,
	}
	return cfg, nil
}
