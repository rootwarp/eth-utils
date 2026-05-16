// Package cli defines the urfave/cli/v2 application, flag schema, and input
// validation for eth-deposit-gen. It converts raw CLI flags into a typed Config
// and invokes the caller-supplied run function only after all validations pass.
package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	ucli "github.com/urfave/cli/v2"

	"github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen/internal/network"
)

// Config holds the validated, parsed inputs from the CLI flags.
type Config struct {
	// KeystorePath is the filesystem path to the EIP-2335 JSON keystore.
	KeystorePath string

	// Pubkeys is the decoded list of 48-byte BLS12-381 G1 compressed points.
	Pubkeys [][48]byte

	// Network identifies the Ethereum consensus network (mainnet or hoodi).
	Network network.Network

	// OutputDir is the validated, writable directory for deposit_data-<ts>.json.
	OutputDir string

	// PassphraseEnv is the name of the environment variable holding the keystore
	// passphrase. An empty string means the tool will fall back to a TTY prompt.
	PassphraseEnv string
}

// NewApp constructs and returns a configured *cli.App. The run callback receives
// a validated Config; it is only invoked when all flags are present and valid.
// Validation errors are returned as cli.Exit errors (exit code 1) so that urfave
// can print them to ErrWriter and exit cleanly.
func NewApp(run func(context.Context, Config) error) *ucli.App {
	app := ucli.NewApp()
	app.Name = "eth-deposit-gen"
	app.Usage = "Generate Launchpad-compatible deposit_data JSON for existing BLS validator keys"
	app.UsageText = `eth-deposit-gen --validator-key-path PATH --pubkeys HEX[,...] --network NET --output-dir DIR [--passphrase-env VAR]`
	app.Description = `Produces deposit_data-<ts>.json for one or more BLS validator public keys by
signing each deposit message with the BLS key loaded from an EIP-2335 keystore.
Output is byte-for-byte compatible with the official ethereum/staking-deposit-cli.`

	// UX examples from docs/prd.md §UX
	app.CustomAppHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.UsageText}}

DESCRIPTION:
   {{.Description}}

EXAMPLES:
   # Hoodi testnet, two pubkeys
   eth-deposit-gen \
     --network hoodi \
     --validator-key-path ./bls-keystore.json \
     --pubkeys 0x93247f2209abcafd...,0xa1b2c3d4e5f6... \
     --output-dir ./out

   # Mainnet, single pubkey
   eth-deposit-gen \
     --network mainnet \
     --validator-key-path ./bls-keystore.json \
     --pubkeys 0x93247f2209abcafd... \
     --output-dir ./out

OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
`

	app.Flags = []ucli.Flag{
		&ucli.StringFlag{
			Name:     "validator-key-path",
			Usage:    "Path to the EIP-2335 JSON keystore containing the BLS signing key",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "pubkeys",
			Usage:    "Comma-separated BLS public keys in 96-hex-char form (0x-prefixed or bare)",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "network",
			Usage:    `Ethereum consensus network: "mainnet" or "hoodi"`,
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "output-dir",
			Usage:    "Existing, writable directory for the output deposit_data-<ts>.json file",
			Required: true,
		},
		&ucli.StringFlag{
			Name:  "passphrase-env",
			Usage: "Name of the environment variable holding the keystore passphrase (omit for TTY prompt)",
		},
	}

	app.Action = func(c *ucli.Context) error {
		// Validation order: network first (per spec), then pubkeys, then output-dir.

		// 1. Parse and validate --network
		net, err := network.ParseFlag(c.String("network"))
		if err != nil {
			return ucli.Exit(fmt.Sprintf("--network: %v", err), 1)
		}

		// 2. Parse and validate --pubkeys
		pubkeys, err := parsePubkeys(c.String("pubkeys"))
		if err != nil {
			return ucli.Exit(fmt.Sprintf("--pubkeys: %v", err), 1)
		}

		// 3. Validate --output-dir
		outputDir := c.String("output-dir")
		if err := validateOutputDir(outputDir); err != nil {
			return ucli.Exit(fmt.Sprintf("--output-dir: %v", err), 1)
		}

		cfg := Config{
			KeystorePath:  c.String("validator-key-path"),
			Pubkeys:       pubkeys,
			Network:       net,
			OutputDir:     outputDir,
			PassphraseEnv: c.String("passphrase-env"),
		}

		// 4. Print confirmation banner to stderr before invoking run.
		printBanner(c.App.ErrWriter, cfg)

		return run(c.Context, cfg)
	}

	return app
}

// parsePubkeys splits a comma-separated pubkey string, validates each entry,
// and decodes them into [48]byte arrays. It is an unexported function so that
// the fuzz target in cli_fuzz_test.go can call it directly.
//
// Rules:
//   - Split on ',' and trim whitespace per entry.
//   - Accept both 0x-prefixed and unprefixed hex.
//   - Lowercase hex before decoding (hex.DecodeString is case-insensitive but
//     we normalise for consistency).
//   - Reject mixed prefix: all entries must be uniformly prefixed or unprefixed.
//   - Each hex string must decode to exactly 48 bytes (96 hex chars).
func parsePubkeys(s string) ([][48]byte, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("no pubkeys supplied")
	}

	parts := strings.Split(s, ",")
	entries := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			return nil, fmt.Errorf("empty pubkey entry in list")
		}
		entries = append(entries, trimmed)
	}

	// Determine prefix uniformity: inspect the first entry, then check all others match.
	firstHasPrefix := strings.HasPrefix(entries[0], "0x") || strings.HasPrefix(entries[0], "0X")
	for i, e := range entries {
		hasPrefix := strings.HasPrefix(e, "0x") || strings.HasPrefix(e, "0X")
		if hasPrefix != firstHasPrefix {
			return nil, fmt.Errorf("mixed 0x prefix: entry %d %q does not match prefix style of entry 0 %q — all pubkeys must be uniformly prefixed or unprefixed", i, e, entries[0])
		}
	}

	result := make([][48]byte, 0, len(entries))
	for _, e := range entries {
		h := strings.ToLower(e)
		h = strings.TrimPrefix(h, "0x")

		// Validate length: 48 bytes = 96 hex chars.
		if len(h) != 96 {
			return nil, fmt.Errorf("pubkey %q has wrong hex length %d, want 96 (48 bytes)", e, len(h))
		}

		b, err := hex.DecodeString(h)
		if err != nil {
			return nil, fmt.Errorf("pubkey %q is not valid hex: %w", e, err)
		}

		var arr [48]byte
		copy(arr[:], b)
		result = append(result, arr)
	}

	return result, nil
}

// validateOutputDir checks that dir exists and the process can write to it.
// It probes writability by creating and immediately removing a temporary file.
func validateOutputDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory %q does not exist", dir)
		}
		return fmt.Errorf("cannot stat directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// Probe writability: create a temp file then remove it immediately.
	f, err := os.CreateTemp(dir, ".eth-deposit-gen-probe-*")
	if err != nil {
		return fmt.Errorf("directory %q is not writable: %w", dir, err)
	}
	f.Close()             //nolint:errcheck
	os.Remove(f.Name())   //nolint:errcheck
	return nil
}

// printBanner writes the confirmation banner to w (which should be app.ErrWriter).
// Format: eth-deposit-gen: network=<net> first_pubkey=<hex> last_pubkey=<hex> count=<n>
// Pubkeys are rendered as 0x-prefixed lowercase hex.
func printBanner(w io.Writer, cfg Config) {
	first := cfg.Pubkeys[0]
	last := cfg.Pubkeys[len(cfg.Pubkeys)-1]
	fmt.Fprintf(w, "eth-deposit-gen: network=%s first_pubkey=0x%x last_pubkey=0x%x count=%d\n",
		cfg.Network,
		first[:],
		last[:],
	len(cfg.Pubkeys))
}
