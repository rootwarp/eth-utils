// Package main is the entry point for eth-deposit-gen. It composes all
// internal packages into a working CLI and maps errors to exit codes per the PRD.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	ucli "github.com/urfave/cli/v2"

	"github.com/rootwarp/eth-utils/go/internal/bls"
	"github.com/rootwarp/eth-utils/go/internal/cli"
	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/keystore"
	"github.com/rootwarp/eth-utils/go/internal/network"
	"github.com/rootwarp/eth-utils/go/internal/output"
)

// CLIVersion mirrors the staking-deposit-cli release used to derive the golden
// test fixtures. Bump only after golden-file re-validation passes.
const CLIVersion = "2.7.0"

// errBLSInit is a sentinel used to detect bls.Init() failures in exitCodeFor.
// herumi errors have no exported sentinel, so we wrap them with this.
var errBLSInit = errors.New("bls init failed")

// defaultWithdrawalCreds returns the 32-byte withdrawal credentials for v1.
// Type 0x00 prefix = BLS withdrawal type. Per the architecture doc this is
// acceptable for v1; a future --withdrawal-address flag will plug in here.
//
// TODO(P1): replace with a real withdrawal address derived from --withdrawal-address flag.
func defaultWithdrawalCreds() [32]byte {
	var wc [32]byte
	wc[0] = 0x00 // BLS withdrawal type prefix; rest is zero
	return wc
}

// pickPassphraseSource returns the appropriate PassphraseSource based on cfg.
// If cfg.PassphraseEnv is non-empty, the source reads from that env var.
// Otherwise it falls back to a TTY prompt written to stderr.
func pickPassphraseSource(cfg cli.Config) keystore.PassphraseSource {
	if cfg.PassphraseEnv != "" {
		return keystore.NewEnvSource(cfg.PassphraseEnv)
	}
	return keystore.NewTermPromptSource(os.Stderr)
}

// deps holds the injectable dependencies for runWithDeps. In production these
// are filled with real implementations; in tests they can be replaced with fakes.
type deps struct {
	// initBLS initialises the herumi BLS library. In tests a no-op can be used.
	initBLS func() error

	// loader is used to load and decrypt the keystore.
	loader keystore.KeyLoader

	// newSigner constructs a BLS signer from a secret.
	newSigner func(secret []byte) (bls.Signer, error)

	// verifier is used for self-verification in the deposit generator.
	verifier bls.Verifier

	// writer is used to persist the deposit data JSON.
	writer output.Writer

	// summaryOut is where the success summary line is written.
	summaryOut io.Writer
}

// productionDeps returns the deps wired with all real implementations.
func productionDeps() deps {
	return deps{
		initBLS:    bls.Init,
		loader:     keystore.NewLoader(),
		newSigner:  bls.NewSigner,
		verifier:   bls.DefaultVerifier(),
		writer:     output.NewFSWriter(),
		summaryOut: os.Stderr,
	}
}

// runWithDeps is the testable core of run. It accepts a deps struct so tests
// can inject fakes without touching the real BLS or keystore implementations.
// It follows the exact wiring order prescribed by Issue #9 AC 3.
func runWithDeps(ctx context.Context, cfg cli.Config, d deps) error {
	// Step 1: initialise the BLS library (process-global, idempotent).
	if err := d.initBLS(); err != nil {
		return fmt.Errorf("%w: %v", errBLSInit, err)
	}

	// Step 2: resolve network parameters.
	params, err := network.Lookup(cfg.Network)
	if err != nil {
		return err
	}

	// Step 3: load and decrypt the keystore; zeroize immediately when done.
	pwSrc := pickPassphraseSource(cfg)
	key, err := d.loader.Load(ctx, cfg.KeystorePath, pwSrc)
	if err != nil {
		return err
	}
	defer key.Zeroize()

	// Step 4: construct the BLS signer from the decrypted secret.
	signer, err := d.newSigner(key.Secret)
	if err != nil {
		return err
	}

	// Step 5: build the deposit generator.
	gen := deposit.NewGenerator(signer, d.verifier, params)

	// Step 6: run the signing pipeline for all requested pubkeys.
	entries, err := gen.Generate(ctx, deposit.Request{
		Network:               cfg.Network,
		Pubkeys:               cfg.Pubkeys,
		WithdrawalCredentials: defaultWithdrawalCreds(),
		AmountGwei:            32_000_000_000,
		DepositCLIVersion:     CLIVersion,
	})
	if err != nil {
		return err
	}

	// Step 7: write the deposit data JSON atomically.
	path, sum, err := d.writer.Write(ctx, cfg.OutputDir, entries, time.Now())
	if err != nil {
		return err
	}

	// Success: print the summary line.
	printSummary(d.summaryOut, path, sum, len(entries), cfg.Network)
	return nil
}

// run is the urfave/cli action function. It delegates to runWithDeps with
// the production dependency set.
func run(ctx context.Context, cfg cli.Config) error {
	return runWithDeps(ctx, cfg, productionDeps())
}

// printSummary writes the success summary line to w.
// Format: wrote <path> (sha256=<hex>, n=<count>, network=<name>)\n
func printSummary(w io.Writer, path, sha256hex string, n int, net network.Network) {
	fmt.Fprintf(w, "wrote %s (sha256=%s, n=%d, network=%s)\n", path, sha256hex, n, net)
}

// exitCodeFor maps errors to exit codes per the PRD:
//
//	0 — success (nil)
//	2 — user / configuration errors (bad input, validation)
//	3 — signer / crypto errors (wrong passphrase, BLS failure)
//	4 — user abort (SIGINT / context.Canceled)
//	1 — fallback for any other error
func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}

	// Exit code 4: context cancellation (SIGINT).
	if errors.Is(err, context.Canceled) {
		return 4
	}

	// Exit code 2: user / configuration errors.
	if errors.Is(err, keystore.ErrKeystoreMissing) ||
		errors.Is(err, keystore.ErrKeystoreMalformed) ||
		errors.Is(err, keystore.ErrKeystoreVersion) ||
		errors.Is(err, keystore.ErrEnvVarEmpty) ||
		errors.Is(err, deposit.ErrPubkeyMismatch) {
		return 2
	}
	// CLI validation errors from urfave/cli (ExitCoder with code 2).
	var ec ucli.ExitCoder
	if errors.As(err, &ec) && ec.ExitCode() == 2 {
		return 2
	}

	// Exit code 3: crypto / signer errors.
	if errors.Is(err, keystore.ErrWrongPassphrase) ||
		errors.Is(err, deposit.ErrSelfVerifyFailed) ||
		errors.Is(err, errBLSInit) {
		return 3
	}

	// Fallback.
	return 1
}

func main() {
	// Set up a context that cancels on SIGINT so the pipeline can exit gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()

	app := cli.NewApp(run)
	if err := app.RunContext(ctx, os.Args); err != nil {
		os.Exit(exitCodeFor(err))
	}
}
