package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	ucli "github.com/urfave/cli/v2"

	"github.com/rootwarp/eth-utils/go/internal/bls"
	"github.com/rootwarp/eth-utils/go/internal/cli"
	"github.com/rootwarp/eth-utils/go/internal/deposit"
	"github.com/rootwarp/eth-utils/go/internal/keystore"
	"github.com/rootwarp/eth-utils/go/internal/network"
	"github.com/rootwarp/eth-utils/go/internal/output"
)

// ---------------------------------------------------------------------------
// Fakes for runWithDeps tests
// ---------------------------------------------------------------------------

// fakeLoader is a KeyLoader that returns a fixed key or error.
type fakeLoader struct {
	key keystore.Key
	err error
}

func (f *fakeLoader) Load(_ context.Context, _ string, _ keystore.PassphraseSource) (keystore.Key, error) {
	return f.key, f.err
}

// fakeSigner implements bls.Signer for tests.
type fakeSigner struct {
	pubkey [48]byte
	sig    [96]byte
	err    error
}

func (f *fakeSigner) Sign(_ [32]byte) ([96]byte, error) {
	return f.sig, f.err
}

func (f *fakeSigner) PublicKey() ([48]byte, error) {
	return f.pubkey, f.err
}

// fakeVerifier implements bls.Verifier for tests.
type fakeVerifier struct {
	ok  bool
	err error
}

func (f *fakeVerifier) Verify(_ [48]byte, _ [32]byte, _ [96]byte) (bool, error) {
	return f.ok, f.err
}

// fakeWriter implements output.Writer for tests.
type fakeWriter struct {
	path      string
	sha256hex string
	err       error
}

func (f *fakeWriter) Write(_ context.Context, _ string, _ []deposit.Entry, _ time.Time) (string, string, error) {
	return f.path, f.sha256hex, f.err
}

// ---------------------------------------------------------------------------
// fakeScanner is a scanner func that returns a fixed DirectoryIndex or error.
type fakeScanner struct {
	index keystore.DirectoryIndex
	err   error
}

func (f *fakeScanner) scan(_ string) (keystore.DirectoryIndex, error) {
	return f.index, f.err
}

// makeTestDeps returns a valid deps set that can be customised per test case.
// The fake pubkey and signer pubkey must match for the deposit pipeline to succeed.
// ---------------------------------------------------------------------------

func makeTestDeps(summaryBuf *bytes.Buffer, writerOverride output.Writer) deps {
	// Use a known 48-byte pubkey for the fake signer.
	var pk [48]byte
	pk[0] = 0xAB
	pkHex := fmt.Sprintf("%x", pk[:])

	fakeSign := &fakeSigner{pubkey: pk}

	var w output.Writer
	if writerOverride != nil {
		w = writerOverride
	} else {
		w = &fakeWriter{path: "/out/deposit_data-1.json", sha256hex: "cafebabe"}
	}

	// Build a DirectoryIndex that maps the fake pubkey to a fake path.
	idx := keystore.DirectoryIndex{
		pkHex: "/fake/keystore.json",
	}
	fs := &fakeScanner{index: idx}

	return deps{
		initBLS: func() error { return nil },
		scanner: fs.scan,
		loader: &fakeLoader{key: keystore.Key{
			Secret:    make([]byte, 32), // 32 zero bytes (valid length)
			PubkeyHex: pkHex,
		}},
		newSigner: func(_ []byte) (bls.Signer, error) {
			return fakeSign, nil
		},
		verifier:   &fakeVerifier{ok: true},
		writer:     w,
		summaryOut: summaryBuf,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// makeCfg returns a minimal valid Config for testing runWithDeps.
func makeCfg() cli.Config {
	var pk [48]byte
	pk[0] = 0xAB
	return cli.Config{
		KeystoreDir: "/fake/keystores",
		Pubkeys:     [][48]byte{pk},
		Network:     network.Hoodi,
		OutputDir:   "/tmp",
	}
}

// ---------------------------------------------------------------------------
// TestRunWithDeps — integration tests for the run wiring using fakes
// ---------------------------------------------------------------------------

func TestRunWithDeps_Success_ExitCode0(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)
	cfg := makeCfg()

	err := runWithDeps(context.Background(), cfg, d)
	if err != nil {
		t.Fatalf("runWithDeps() returned unexpected error: %v", err)
	}
	code := exitCodeFor(err)
	if code != 0 {
		t.Errorf("exitCodeFor(nil) = %d, want 0", code)
	}
}

func TestRunWithDeps_Success_PrintsSummary(t *testing.T) {
	var summaryBuf bytes.Buffer
	w := &fakeWriter{path: "/out/deposit_data-99.json", sha256hex: "deadbeef99"}
	d := makeTestDeps(&summaryBuf, w)
	cfg := makeCfg()

	if err := runWithDeps(context.Background(), cfg, d); err != nil {
		t.Fatalf("runWithDeps() unexpected error: %v", err)
	}

	got := summaryBuf.String()
	if !strings.Contains(got, "wrote /out/deposit_data-99.json") {
		t.Errorf("summary line missing path; got %q", got)
	}
	if !strings.Contains(got, "sha256=deadbeef99") {
		t.Errorf("summary line missing sha256; got %q", got)
	}
	if !strings.Contains(got, "network=hoodi") {
		t.Errorf("summary line missing network; got %q", got)
	}
}

func TestRunWithDeps_BLSInitError_ExitCode3(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)
	d.initBLS = func() error { return errors.New("herumi init failure") }

	err := runWithDeps(context.Background(), makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want bls init error")
	}
	if !errors.Is(err, errBLSInit) {
		t.Errorf("error = %v, want wrapped errBLSInit", err)
	}
	if code := exitCodeFor(err); code != 3 {
		t.Errorf("exitCodeFor(bls init error) = %d, want 3", code)
	}
}

func TestRunWithDeps_KeystoreLoadError_ExitCode2(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)
	d.loader = &fakeLoader{err: fmt.Errorf("%w: /fake/ks.json", keystore.ErrKeystoreMissing)}

	err := runWithDeps(context.Background(), makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want ErrKeystoreMissing")
	}
	if !errors.Is(err, keystore.ErrKeystoreMissing) {
		t.Errorf("error = %v, want ErrKeystoreMissing", err)
	}
	if code := exitCodeFor(err); code != 2 {
		t.Errorf("exitCodeFor(ErrKeystoreMissing) = %d, want 2", code)
	}
}

func TestRunWithDeps_WrongPassphrase_ExitCode3(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)
	d.loader = &fakeLoader{err: fmt.Errorf("%w: bad checksum", keystore.ErrWrongPassphrase)}

	err := runWithDeps(context.Background(), makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want ErrWrongPassphrase")
	}
	if code := exitCodeFor(err); code != 3 {
		t.Errorf("exitCodeFor(ErrWrongPassphrase) = %d, want 3", code)
	}
}

func TestRunWithDeps_PubkeyMismatch_ExitCode2(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)

	// The cfg asks for pubkey 0xBB, but the signer (returned for any key) has
	// pubkey 0xAB → deposit.Generator detects the mismatch → ErrPubkeyMismatch.
	// We must include 0xBB in the scanner index so the scan lookup succeeds;
	// the pubkey mismatch is detected later in the deposit pipeline.
	var wrongPk [48]byte
	wrongPk[0] = 0xBB
	wrongPkHex := fmt.Sprintf("%x", wrongPk[:])

	d.scanner = func(_ string) (keystore.DirectoryIndex, error) {
		return keystore.DirectoryIndex{
			wrongPkHex: "/fake/wrong-keystore.json",
		}, nil
	}

	cfg := makeCfg()
	cfg.Pubkeys = [][48]byte{wrongPk}

	err := runWithDeps(context.Background(), cfg, d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want ErrPubkeyMismatch")
	}
	if !errors.Is(err, deposit.ErrPubkeyMismatch) {
		t.Errorf("error = %v, want ErrPubkeyMismatch", err)
	}
	if code := exitCodeFor(err); code != 2 {
		t.Errorf("exitCodeFor(ErrPubkeyMismatch) = %d, want 2", code)
	}
}

func TestRunWithDeps_WriterError_ExitCode1(t *testing.T) {
	var summaryBuf bytes.Buffer
	w := &fakeWriter{err: errors.New("disk full")}
	d := makeTestDeps(&summaryBuf, w)

	err := runWithDeps(context.Background(), makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want writer error")
	}
	if code := exitCodeFor(err); code != 1 {
		t.Errorf("exitCodeFor(disk full) = %d, want 1", code)
	}
}

func TestRunWithDeps_ContextCanceled_ExitCode4(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)

	// Loader blocks and cancels the ctx mid-load.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	d.loader = &fakeLoader{err: context.Canceled}

	err := runWithDeps(ctx, makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if code := exitCodeFor(err); code != 4 {
		t.Errorf("exitCodeFor(context.Canceled) = %d, want 4", code)
	}
}

// ---------------------------------------------------------------------------
// TestExitCodeFor — table-driven tests for exitCodeFor
// ---------------------------------------------------------------------------

func TestExitCodeFor_Success(t *testing.T) {
	if got := exitCodeFor(nil); got != 0 {
		t.Errorf("exitCodeFor(nil) = %d, want 0", got)
	}
}

func TestExitCodeFor_ErrorCodes(t *testing.T) {
	// A synthetic urfave ExitCoder with code 2.
	exitCoder2 := ucli.Exit("validation error", 2)

	// A synthetic urfave ExitCoder with non-2 code (should not match code-2 path).
	exitCoder1 := ucli.Exit("some urfave error", 1)

	// bls init error — has no exported sentinel; use the wrapper we define in main.
	blsInitErr := fmt.Errorf("%w: herumi Init: something went wrong", errBLSInit)

	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		// --- exit code 0 ---
		{"nil", nil, 0},

		// --- exit code 2 ---
		{"ErrKeystoreMissing", keystore.ErrKeystoreMissing, 2},
		{"ErrKeystoreMissing wrapped", fmt.Errorf("wrap: %w", keystore.ErrKeystoreMissing), 2},
		{"ErrKeystoreMalformed", keystore.ErrKeystoreMalformed, 2},
		{"ErrKeystoreVersion", keystore.ErrKeystoreVersion, 2},
		{"ErrEnvVarEmpty", keystore.ErrEnvVarEmpty, 2},
		{"ErrEnvVarEmpty wrapped", fmt.Errorf("passphrase source: %w", keystore.ErrEnvVarEmpty), 2},
		{"ErrKeystoreNotFound", keystore.ErrKeystoreNotFound, 2},
		{"ErrKeystoreNotFound wrapped", fmt.Errorf("no keystore found for pubkey 0xaabb in /dir: %w", keystore.ErrKeystoreNotFound), 2},
		{"ErrPubkeyMismatch", deposit.ErrPubkeyMismatch, 2},
		{"ErrPubkeyMismatch wrapped", fmt.Errorf("wrap: %w", deposit.ErrPubkeyMismatch), 2},
		{"ExitCoder code 2", exitCoder2, 2},

		// --- exit code 3 ---
		{"ErrWrongPassphrase", keystore.ErrWrongPassphrase, 3},
		{"ErrSelfVerifyFailed", deposit.ErrSelfVerifyFailed, 3},
		{"ErrSelfVerifyFailed wrapped", fmt.Errorf("wrap: %w", deposit.ErrSelfVerifyFailed), 3},
		{"errBLSInit", blsInitErr, 3},

		// --- exit code 4 ---
		{"context.Canceled", context.Canceled, 4},
		{"context.Canceled wrapped", fmt.Errorf("wrap: %w", context.Canceled), 4},

		// --- exit code 1 (fallback) ---
		{"unknown error", errors.New("something else"), 1},
		{"ExitCoder code 1", exitCoder1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := exitCodeFor(tc.err)
			if got != tc.wantCode {
				t.Errorf("exitCodeFor(%v) = %d, want %d", tc.err, got, tc.wantCode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestPrintSummary — verifies the exact summary line format
// ---------------------------------------------------------------------------

func TestPrintSummary_Format(t *testing.T) {
	var buf bytes.Buffer
	path := "/output/deposit_data-1700000000.json"
	sha256hex := "abc123def456"
	n := 3
	net := network.Network("hoodi")

	printSummary(&buf, path, sha256hex, n, net)

	got := buf.String()
	want := fmt.Sprintf("wrote %s (sha256=%s, n=%d, network=%s)\n", path, sha256hex, n, net)
	if got != want {
		t.Errorf("printSummary output:\ngot  %q\nwant %q", got, want)
	}
}

func TestPrintSummary_ContainsRequiredParts(t *testing.T) {
	var buf bytes.Buffer
	printSummary(&buf, "/some/path.json", "deadbeef", 5, network.Hoodi)

	got := buf.String()

	parts := []string{
		"wrote /some/path.json",
		"sha256=deadbeef",
		"n=5",
		"network=hoodi",
	}
	for _, part := range parts {
		if !strings.Contains(got, part) {
			t.Errorf("printSummary output %q does not contain %q", got, part)
		}
	}
}

// ---------------------------------------------------------------------------
// TestCLIVersion — constant must be set to 2.7.0
// ---------------------------------------------------------------------------

func TestCLIVersion(t *testing.T) {
	if CLIVersion != "2.7.0" {
		t.Errorf("CLIVersion = %q, want %q", CLIVersion, "2.7.0")
	}
}

// ---------------------------------------------------------------------------
// TestDefaultWithdrawalCreds — first byte is BLS type prefix, rest is zero
// ---------------------------------------------------------------------------

func TestDefaultWithdrawalCreds(t *testing.T) {
	wc := defaultWithdrawalCreds()
	if wc[0] != 0x00 {
		t.Errorf("defaultWithdrawalCreds()[0] = 0x%02x, want 0x00 (BLS withdrawal type)", wc[0])
	}
	// All remaining bytes must be zero.
	for i := 1; i < len(wc); i++ {
		if wc[i] != 0 {
			t.Errorf("defaultWithdrawalCreds()[%d] = 0x%02x, want 0x00", i, wc[i])
		}
	}
}

// ---------------------------------------------------------------------------
// TestPickPassphraseSource — selects env or term source based on cfg
// ---------------------------------------------------------------------------

func TestPickPassphraseSource_EnvSource(t *testing.T) {
	cfg := cli.Config{PassphraseEnv: "MY_PASSPHRASE_VAR"}
	src := pickPassphraseSource(cfg)
	if src == nil {
		t.Fatal("pickPassphraseSource returned nil")
	}
	// EnvSource.Read() returns ErrEnvVarEmpty when the var is unset.
	// We verify the type by calling Read() and checking for the keystore error.
	t.Setenv("MY_PASSPHRASE_VAR", "")
	_, err := src.Read()
	if !errors.Is(err, keystore.ErrEnvVarEmpty) {
		t.Errorf("env source with unset var: error = %v, want ErrEnvVarEmpty", err)
	}
}

func TestPickPassphraseSource_EnvSourceWithValue(t *testing.T) {
	cfg := cli.Config{PassphraseEnv: "MY_PASSPHRASE_VAR"}
	t.Setenv("MY_PASSPHRASE_VAR", "secret123")
	src := pickPassphraseSource(cfg)
	pw, err := src.Read()
	if err != nil {
		t.Fatalf("env source with set var: Read() = %v, want nil", err)
	}
	if string(pw) != "secret123" {
		t.Errorf("env source returned %q, want %q", string(pw), "secret123")
	}
}

func TestPickPassphraseSource_TermSource(t *testing.T) {
	// Empty PassphraseEnv → should return a term prompt source (non-nil).
	cfg := cli.Config{PassphraseEnv: ""}
	src := pickPassphraseSource(cfg)
	if src == nil {
		t.Fatal("pickPassphraseSource returned nil for empty PassphraseEnv")
	}
	// We can't call Read() on a term source in tests (no TTY), but we can
	// verify it's non-nil and satisfies the interface.
	_ = src
}

// ---------------------------------------------------------------------------
// TestRunWithDeps — scanner-specific tests
// ---------------------------------------------------------------------------

func TestRunWithDeps_ScannerError_ExitCode1(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)
	d.scanner = func(_ string) (keystore.DirectoryIndex, error) {
		return nil, errors.New("cannot read directory: permission denied")
	}

	err := runWithDeps(context.Background(), makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want scanner error")
	}
	// Scanner errors from unreadable dirs are not user errors; they map to code 1.
	if code := exitCodeFor(err); code != 1 {
		t.Errorf("exitCodeFor(scanner error) = %d, want 1", code)
	}
}

func TestRunWithDeps_PubkeyNotInIndex_ExitCode2(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)

	// Override scanner to return an empty index — pubkey won't be found.
	d.scanner = func(_ string) (keystore.DirectoryIndex, error) {
		return keystore.DirectoryIndex{}, nil
	}

	err := runWithDeps(context.Background(), makeCfg(), d)
	if err == nil {
		t.Fatal("runWithDeps() returned nil error, want ErrKeystoreNotFound")
	}
	if !errors.Is(err, keystore.ErrKeystoreNotFound) {
		t.Errorf("error = %v, want wrapped ErrKeystoreNotFound", err)
	}
	if code := exitCodeFor(err); code != 2 {
		t.Errorf("exitCodeFor(ErrKeystoreNotFound) = %d, want 2", code)
	}
}

func TestRunWithDeps_ErrorMessageContainsPubkeyAndDir(t *testing.T) {
	var summaryBuf bytes.Buffer
	d := makeTestDeps(&summaryBuf, nil)

	// Empty index so the lookup fails.
	d.scanner = func(_ string) (keystore.DirectoryIndex, error) {
		return keystore.DirectoryIndex{}, nil
	}

	cfg := makeCfg()
	err := runWithDeps(context.Background(), cfg, d)
	if err == nil {
		t.Fatal("expected error")
	}

	msg := err.Error()
	// Error must mention the pubkey (0x-prefixed) and the keystore dir.
	if !strings.Contains(msg, "0x") {
		t.Errorf("error message %q does not mention pubkey (0x prefix)", msg)
	}
	if !strings.Contains(msg, cfg.KeystoreDir) {
		t.Errorf("error message %q does not mention keystore dir %q", msg, cfg.KeystoreDir)
	}
}

// ---------------------------------------------------------------------------
// TestPrintSummary_DryRunEmptyPath verifies that an empty path (returned by
// DryRunWriter) is rendered as "<stdout>" in the summary line.
func TestPrintSummary_DryRunEmptyPath(t *testing.T) {
	var buf bytes.Buffer
	printSummary(&buf, "", "deadbeef", 1, network.Hoodi)
	got := buf.String()
	if !strings.Contains(got, "wrote <stdout>") {
		t.Errorf("printSummary with empty path: %q does not contain %q", got, "wrote <stdout>")
	}
}

// TestPickWriter — verifies that pickWriter selects the correct writer type
// ---------------------------------------------------------------------------

func TestPickWriter_FSWriterWhenDryRunFalse(t *testing.T) {
	cfg := cli.Config{DryRun: false}
	w := pickWriter(cfg, io.Discard)
	if w == nil {
		t.Fatal("pickWriter returned nil")
	}
	// FSWriter produces a non-empty path; DryRunWriter produces an empty path.
	// We can't distinguish the concrete type without a type assertion, but we
	// can verify behavior: FSWriter.Write returns an error when dir is not
	// writable (we pass "/nonexistent"), while DryRunWriter writes to the
	// provided io.Writer and returns ("", sha256, nil).
	// Use a temp dir so FSWriter doesn't error on dir access.
	dir := t.TempDir()
	path, _, err := w.Write(context.Background(), dir, []deposit.Entry{}, time.Now())
	if err != nil {
		t.Fatalf("FSWriter.Write: %v", err)
	}
	if path == "" {
		t.Errorf("FSWriter returned empty path; want non-empty (a real file path)")
	}
}

func TestPickWriter_DryRunWriterWhenDryRunTrue(t *testing.T) {
	var stdoutBuf bytes.Buffer
	cfg := cli.Config{DryRun: true}
	w := pickWriter(cfg, &stdoutBuf)
	if w == nil {
		t.Fatal("pickWriter returned nil")
	}
	// DryRunWriter.Write always returns ("", sha256, nil) — path is empty.
	path, _, err := w.Write(context.Background(), "", []deposit.Entry{}, time.Now())
	if err != nil {
		t.Fatalf("DryRunWriter.Write: %v", err)
	}
	if path != "" {
		t.Errorf("DryRunWriter returned non-empty path %q; want empty", path)
	}
}

// ---------------------------------------------------------------------------
// TestRunWithDeps_DryRun — dry-run mode integration tests
// ---------------------------------------------------------------------------

// TestRunWithDeps_DryRun_StdoutContainsJSON verifies that with DryRun=true,
// stdout receives valid JSON and the summary sha256 matches the sha256 of
// the bytes written to stdout. (AC#3, AC#5)
func TestRunWithDeps_DryRun_StdoutContainsJSON(t *testing.T) {
	var stdoutBuf bytes.Buffer
	var summaryBuf bytes.Buffer

	// Build deps with a DryRunWriter pointing to stdoutBuf.
	d := makeTestDeps(&summaryBuf, output.NewDryRunWriter(&stdoutBuf))
	cfg := makeCfg()
	cfg.DryRun = true

	if err := runWithDeps(context.Background(), cfg, d); err != nil {
		t.Fatalf("runWithDeps(dry-run): %v", err)
	}

	// AC#5a: stdout must contain valid JSON.
	got := stdoutBuf.Bytes()
	var parsed []any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, got)
	}

	// AC#3: sha256 in summary must match sha256 of bytes written to stdout.
	h := sha256.Sum256(got)
	wantSHA := hex.EncodeToString(h[:])
	summary := summaryBuf.String()
	if !strings.Contains(summary, "sha256="+wantSHA) {
		t.Errorf("summary sha256 does not match stdout sha256\nsummary: %q\nwant sha256=%s", summary, wantSHA)
	}
}

// TestRunWithDeps_DryRun_OutputDirEmpty verifies that no files are created
// in output-dir when DryRun=true. (AC#5b)
func TestRunWithDeps_DryRun_OutputDirEmpty(t *testing.T) {
	var stdoutBuf bytes.Buffer
	var summaryBuf bytes.Buffer
	outDir := t.TempDir()

	d := makeTestDeps(&summaryBuf, output.NewDryRunWriter(&stdoutBuf))
	cfg := makeCfg()
	cfg.DryRun = true
	cfg.OutputDir = outDir

	if err := runWithDeps(context.Background(), cfg, d); err != nil {
		t.Fatalf("runWithDeps(dry-run): %v", err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", outDir, err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("output-dir not empty after dry-run; found files: %v", names)
	}
}

// TestRunWithDeps_DryRun_SummaryStillPrinted verifies that the summary line
// is still written to stderr in dry-run mode. (AC#3)
func TestRunWithDeps_DryRun_SummaryStillPrinted(t *testing.T) {
	var stdoutBuf bytes.Buffer
	var summaryBuf bytes.Buffer

	d := makeTestDeps(&summaryBuf, output.NewDryRunWriter(&stdoutBuf))
	cfg := makeCfg()
	cfg.DryRun = true

	if err := runWithDeps(context.Background(), cfg, d); err != nil {
		t.Fatalf("runWithDeps(dry-run): %v", err)
	}

	summary := summaryBuf.String()
	if summary == "" {
		t.Error("summary line was empty; expected it to be printed even in dry-run mode")
	}
	// sha256 and network must still appear.
	if !strings.Contains(summary, "sha256=") {
		t.Errorf("summary %q does not contain sha256=", summary)
	}
	if !strings.Contains(summary, "network=") {
		t.Errorf("summary %q does not contain network=", summary)
	}
}

// TestRunWithDeps_DryRun_VerifyFailureAbortsWithSameExitCode verifies that
// self-verification still runs in dry-run mode and aborts with exit code 3. (AC#4)
func TestRunWithDeps_DryRun_VerifyFailureAbortsWithSameExitCode(t *testing.T) {
	var stdoutBuf bytes.Buffer
	var summaryBuf bytes.Buffer

	d := makeTestDeps(&summaryBuf, output.NewDryRunWriter(&stdoutBuf))
	// Force the verifier to fail so self-verification aborts the pipeline.
	d.verifier = &fakeVerifier{ok: false, err: nil}
	cfg := makeCfg()
	cfg.DryRun = true

	err := runWithDeps(context.Background(), cfg, d)
	if err == nil {
		t.Fatal("runWithDeps(dry-run, bad verifier): expected error, got nil")
	}
	if !errors.Is(err, deposit.ErrSelfVerifyFailed) {
		t.Errorf("error = %v, want ErrSelfVerifyFailed", err)
	}
	if code := exitCodeFor(err); code != 3 {
		t.Errorf("exitCodeFor(ErrSelfVerifyFailed) = %d, want 3", code)
	}
}
