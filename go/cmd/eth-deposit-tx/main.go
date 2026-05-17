// Package main is the entry point for eth-deposit-tx.
// It sets up the urfave/cli/v2 application and wires the build and sign subcommands.
//
// This is the initial scaffold created in Issue 1.1. It will be expanded in later issues
// to use the internal/cli package and full dependency injection pattern (matching eth-deposit-gen).
package main

import (
	"fmt"
	"os"

	ucli "github.com/urfave/cli/v2"
)

// version, commit, and date are set at build time via -ldflags.
// Default values are used for local/dev builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := &ucli.App{
		Name:  "eth-deposit-tx",
		Usage: "Create and sign Ethereum deposit transactions from deposit data JSON",
		UsageText: `eth-deposit-tx build [options]
   eth-deposit-tx sign [options]`,
		Version: fmt.Sprintf("%s (commit=%s, built=%s)", version, commit, date),
		Description: `eth-deposit-tx converts Launchpad-compatible deposit_data JSON into raw Ethereum transactions
for the Beacon Chain deposit contract.

It supports a secure two-phase workflow:
  build  - Construct an unsigned transaction (supports offline/air-gapped mode)
  sign   - Sign the transaction, with Ledger hardware as the primary method

The tool produces standard hex-encoded RLP output ready for eth_sendRawTransaction.`,
		Commands: []*ucli.Command{
			buildCommand(),
			signCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func buildCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "build",
		Usage: "Construct an unsigned deposit transaction from deposit data",
		Description: `Reads a deposit_data JSON file (produced by eth-deposit-gen or the official Launchpad)
and produces an unsigned Ethereum transaction for the deposit contract.

Supports both hybrid mode (with optional --rpc-url) and fully offline/air-gapped mode.`,
		UsageText: `eth-deposit-tx build --input-file FILE --network NET [options]`,
		Flags: []ucli.Flag{
			&ucli.StringFlag{
				Name:     "input-file",
				Aliases:  []string{"i"},
				Usage:    "Path to deposit_data-*.json file (or '-' for stdin)",
				Required: true,
				EnvVars:  []string{"ETH_DEPOSIT_TX_INPUT_FILE"},
			},
			&ucli.StringFlag{
				Name:    "network",
				Aliases: []string{"n"},
				Usage:   "Target network (mainnet, hoodi, sepolia, holesky)",
				Value:   "hoodi",
				EnvVars: []string{"ETH_DEPOSIT_TX_NETWORK"},
			},
			&ucli.StringFlag{
				Name:    "output",
				Usage:   "Output file for the unsigned transaction (default: stdout)",
				EnvVars: []string{"ETH_DEPOSIT_TX_OUTPUT"},
			},
			&ucli.IntFlag{
				Name:    "index",
				Usage:   "Index of the deposit entry to use when the JSON contains multiple validators (default: 0)",
				Value:   0,
				EnvVars: []string{"ETH_DEPOSIT_TX_INDEX"},
			},
			&ucli.StringFlag{
				Name:    "rpc-url",
				Usage:   "JSON-RPC endpoint URL for gas/nonce estimation (optional; when omitted, all gas and nonce flags must be supplied explicitly)",
				EnvVars: []string{"ETH_DEPOSIT_TX_RPC_URL"},
			},
			&ucli.StringFlag{
				Name:    "gas-limit",
				Usage:   fmt.Sprintf("Gas limit for the deposit transaction (default: %d)", defaultGasLimit),
				EnvVars: []string{"ETH_DEPOSIT_TX_GAS_LIMIT"},
			},
			&ucli.StringFlag{
				Name:    "max-fee-per-gas",
				Usage:   "EIP-1559 maximum fee per gas in wei (decimal integer, e.g. 20000000000 for 20 Gwei)",
				EnvVars: []string{"ETH_DEPOSIT_TX_MAX_FEE_PER_GAS"},
			},
			&ucli.StringFlag{
				Name:    "max-priority-fee-per-gas",
				Usage:   "EIP-1559 maximum priority fee per gas in wei (decimal integer, e.g. 1000000000 for 1 Gwei)",
				EnvVars: []string{"ETH_DEPOSIT_TX_MAX_PRIORITY_FEE_PER_GAS"},
			},
			&ucli.StringFlag{
				Name:    "nonce",
				Usage:   "Override the sender account nonce (non-negative integer; omit to fetch from RPC or set later)",
				EnvVars: []string{"ETH_DEPOSIT_TX_NONCE"},
			},
		},
		Action: func(c *ucli.Context) error {
			cfg, err := LoadBuildConfig(c)
			if err != nil {
				return err
			}
			fmt.Fprintf(c.App.Writer,
				"build: would build for network=%s chain_id=%d contract=%s (Phase 2 will perform real ABI encoding)\n",
				cfg.Network,
				cfg.NetworkParams.ChainID,
				cfg.NetworkParams.DepositContractAddressHex(),
			)
			return nil
		},
	}
}

func signCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "sign",
		Usage: "Sign a previously built unsigned deposit transaction",
		Description: `Signs an unsigned transaction produced by "eth-deposit-tx build".

Primary signing method is Ledger hardware wallet. A local private-key fallback
is available via the ETH_DEPOSIT_TX_PRIVATE_KEY environment variable (with strong warnings).`,
		UsageText: `eth-deposit-tx sign --input FILE [--ledger | --private-key]`,
		Flags: []ucli.Flag{
			&ucli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "Path to the unsigned transaction (from build) or '-' for stdin",
				Required: true,
			},
			&ucli.BoolFlag{
				Name:  "ledger",
				Usage: "Sign using a connected Ledger device (default primary method)",
			},
			&ucli.StringFlag{
				Name:  "output",
				Usage: "Output file for the signed transaction (default: stdout)",
			},
		},
		Action: func(c *ucli.Context) error {
			fmt.Fprintf(c.App.Writer, "sign command placeholder (Issue 1.1 scaffold)\n")
			fmt.Fprintf(c.App.Writer, "Full Ledger + local signer implementation coming in Phase 3.\n")
			return nil
		},
	}
}
