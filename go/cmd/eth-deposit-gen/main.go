// Package main is the entry point for eth-deposit-gen. It composes all
// internal packages into a working CLI and maps errors to exit codes per the PRD.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
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

// errMainnetAckRequired is returned by runWithDeps when cfg.Network is mainnet
// but cfg.MainnetAck is false. The CLI gate in app.Action catches this first for
// CLI callers; this sentinel protects non-CLI callers (integration tests, future
// programmatic APIs) and maps to exit code 2.
var errMainnetAckRequired = errors.New("mainnet requires explicit acknowledgement (set Config.MainnetAck = true)")

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

// pickWriter returns the appropriate output.Writer based on cfg.
// When cfg.DryRun is true, returns a DryRunWriter that writes JSON to w
// (typically os.Stdout); otherwise returns an FSWriter that writes to disk.
func pickWriter(cfg cli.Config, w io.Writer) output.Writer {
	if cfg.DryRun {
		return output.NewDryRunWriter(w)
	}
	return output.NewFSWriter()
}

// deps holds the injectable dependencies for runWithDeps. In production these
// are filled with real implementations; in tests they can be replaced with fakes.
type deps struct {
	// initBLS initialises the herumi BLS library. In tests a no-op can be used.
	initBLS func() error

	// scanner scans a keystore directory and returns a pubkey→path index.
	// It is called once before the per-pubkey loop; no decryption occurs here.
	scanner func(string) (keystore.DirectoryIndex, error)

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

	// logger receives structured debug messages. Set to a discarding logger to
	// suppress all output; set to a text/JSON handler to enable debug logging.
	logger *slog.Logger
}

// buildLogger constructs a *slog.Logger based on the verbose and jsonLogs flags.
// Output is always written to w (os.Stderr in production, a buffer in tests).
// When verbose is true, the handler level is set to Debug; otherwise Info.
// When jsonLogs is true, slog.NewJSONHandler is used; otherwise slog.NewTextHandler.
func buildLogger(verbose, jsonLogs bool, w io.Writer) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if jsonLogs {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

// productionDeps returns the deps wired with all real implementations.
// The logger field is intentionally set to a discarding logger here; run()
// overrides it with the cfg-configured logger before calling runWithDeps.
func productionDeps() deps {
	return deps{
		initBLS:    bls.Init,
		scanner:    keystore.ScanDir,
		loader:     keystore.NewLoader(),
		newSigner:  bls.NewSigner,
		verifier:   bls.DefaultVerifier(),
		writer:     output.NewFSWriter(),
		summaryOut: os.Stderr,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// runWithDeps is the testable core of run. It accepts a deps struct so tests
// can inject fakes without touching the real BLS or keystore implementations.
// It follows the exact wiring order prescribed by Issue #25.
func runWithDeps(ctx context.Context, cfg cli.Config, d deps) error {
	log := d.logger

	// Step 1: initialise the BLS library (process-global, idempotent).
	log.Debug("bls: initialising library")
	if err := d.initBLS(); err != nil {
		log.Debug("bls: init failed", "error", err)
		return fmt.Errorf("%w: %v", errBLSInit, err)
	}
	log.Debug("bls: library ready")

	// Step 2: resolve network parameters.
	log.Debug("network: looking up params", "network", cfg.Network)
	params, err := network.Lookup(cfg.Network)
	if err != nil {
		return err
	}
	log.Debug("network: params resolved",
		"network", params.Name,
		"genesis_fork_version", fmt.Sprintf("0x%x", params.GenesisForkVersion))

	// Defense-in-depth: re-verify the mainnet acknowledgement inside the pipeline
	// so that non-CLI callers (integration tests, future programmatic APIs) cannot
	// skip the safety gate by constructing a Config directly. The CLI app.Action
	// fires first for CLI callers and returns before reaching this point.
	if cfg.Network == network.Mainnet && !cfg.MainnetAck {
		log.Debug("mainnet: ack not set, aborting")
		return errMainnetAckRequired
	}
	if cfg.Network == network.Mainnet {
		log.Debug("mainnet: explicit ack verified")
	}

	// Step 3: scan the keystore directory — no decryption yet.
	log.Debug("keystore: scanning directory", "dir", cfg.KeystoreDir)
	index, err := d.scanner(cfg.KeystoreDir)
	if err != nil {
		log.Debug("keystore: scan failed", "error", err)
		return err
	}
	log.Debug("keystore: directory scanned", "count", len(index))

	pwSrc := pickPassphraseSource(cfg)
	passphraseSource := "tty"
	if cfg.PassphraseEnv != "" {
		passphraseSource = "env:" + cfg.PassphraseEnv
	}

	// Step 4: process pubkeys concurrently using a bounded worker pool.
	// The pool size defaults to 1 when cfg.Parallel == 0 (Config built outside CLI).
	parallel := cfg.Parallel
	if parallel < 1 {
		parallel = 1
	}

	// workerResult carries the output (or error) from one pubkey processing unit.
	type workerResult struct {
		idx   int
		entry deposit.Entry
		err   error
	}

	// Create a cancellable child context so workers can signal each other on error.
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	// work is pre-filled with pubkey indices; workers drain it.
	work := make(chan int, len(cfg.Pubkeys))
	for i := range cfg.Pubkeys {
		work <- i
	}
	close(work)

	results := make(chan workerResult, len(cfg.Pubkeys))

	var wg sync.WaitGroup
	for w := 0; w < parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				pk := cfg.Pubkeys[i]
				pkHex := fmt.Sprintf("%x", pk[:])
				log.Debug("deposit: processing pubkey", "pubkey", pkHex)

				keystorePath, ok := index.Lookup(pkHex)
				if !ok {
					results <- workerResult{idx: i, err: fmt.Errorf(
						"no keystore found for pubkey 0x%s in %s: %w",
						pkHex, cfg.KeystoreDir, keystore.ErrKeystoreNotFound)}
					workerCancel()
					continue
				}
				log.Debug("keystore: loading", "pubkey", pkHex, "path", keystorePath, "passphrase_source", passphraseSource)

				key, err := d.loader.Load(workerCtx, keystorePath, pwSrc)
				if err != nil {
					log.Debug("keystore: load failed", "pubkey", pkHex, "error", err)
					results <- workerResult{idx: i, err: err}
					workerCancel()
					continue
				}
				log.Debug("keystore: loaded", "pubkey", key.PubkeyHex, "secret_len", len(key.Secret))

				signer, err := d.newSigner(key.Secret)
				key.Zeroize() // zeroize immediately after signer is constructed, even on error path
				if err != nil {
					log.Debug("signer: construction failed", "pubkey", pkHex, "error", err)
					results <- workerResult{idx: i, err: err}
					workerCancel()
					continue
				}
				log.Debug("signer: ready", "pubkey", pkHex)

				gen := deposit.NewGenerator(signer, d.verifier, params)
				log.Debug("deposit: generating entry", "pubkey", pkHex, "network", cfg.Network)
				e, err := gen.Generate(workerCtx, deposit.Request{
					Network:               cfg.Network,
					Pubkeys:               [][48]byte{pk},
					WithdrawalCredentials: defaultWithdrawalCreds(),
					AmountGwei:            32_000_000_000,
					DepositCLIVersion:     CLIVersion,
				})
				if err != nil {
					log.Debug("deposit: generation failed", "pubkey", pkHex, "error", err)
					results <- workerResult{idx: i, err: err}
					workerCancel()
					continue
				}
				results <- workerResult{idx: i, entry: e[0]}
			}
		}()
	}

	// Close results channel once all workers have finished.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in an indexed slice to preserve input order.
	entries := make([]deposit.Entry, len(cfg.Pubkeys))
	var firstErr error
	for r := range results {
		if r.err != nil {
			// Prefer the first non-Canceled error so that the returned error
			// reflects the root cause rather than the cascading cancellation.
			if firstErr == nil || (errors.Is(firstErr, context.Canceled) && !errors.Is(r.err, context.Canceled)) {
				firstErr = r.err
			}
			workerCancel()
			continue
		}
		entries[r.idx] = r.entry
	}
	if firstErr != nil {
		return firstErr
	}

	log.Debug("deposit: generation complete", "entry_count", len(entries))

	// Step 5: write the deposit data JSON atomically.
	log.Debug("output: writing deposit data", "output_dir", cfg.OutputDir, "entry_count", len(entries))
	path, sum, err := d.writer.Write(ctx, cfg.OutputDir, entries, time.Now())
	if err != nil {
		log.Debug("output: write failed", "error", err)
		return err
	}
	log.Debug("output: written", "path", path, "sha256", sum)

	// Success: print the summary line.
	printSummary(d.summaryOut, path, sum, len(entries), cfg.Network)
	return nil
}

// run is the urfave/cli action function. It delegates to runWithDeps with
// the production dependency set. The writer and logger are configured here
// from cfg so that productionDeps() remains free of flag knowledge.
func run(ctx context.Context, cfg cli.Config) error {
	d := productionDeps()
	d.writer = pickWriter(cfg, os.Stdout)
	d.logger = buildLogger(cfg.Verbose, cfg.JSONLogs, os.Stderr)
	return runWithDeps(ctx, cfg, d)
}

// printSummary writes the success summary line to w.
// Format: wrote <path> (sha256=<hex>, n=<count>, network=<name>)\n
// When path is empty (DryRunWriter returns ""), the placeholder "<stdout>" is
// used so the summary remains human-readable.
func printSummary(w io.Writer, path, sha256hex string, n int, net network.Network) {
	display := path
	if display == "" {
		display = "<stdout>"
	}
	fmt.Fprintf(w, "wrote %s (sha256=%s, n=%d, network=%s)\n", display, sha256hex, n, net)
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
		errors.Is(err, keystore.ErrKeystoreNotFound) ||
		errors.Is(err, deposit.ErrPubkeyMismatch) ||
		errors.Is(err, errMainnetAckRequired) {
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
