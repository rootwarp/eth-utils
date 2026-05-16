package cli_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ucli "github.com/urfave/cli/v2"

	icli "github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen/internal/cli"
	"github.com/rootwarp/eth-utils/go/cmd/eth-deposit-gen/internal/network"
)

// runApp is a helper that invokes the app with the given args and captures stderr.
// It returns the Config received by the run callback (if called), stderr output, and any error.
// ExitErrHandler is overridden to prevent os.Exit from being called during tests.
func runApp(t *testing.T, args []string) (cfg icli.Config, stderr string, runCalled bool, err error) {
	t.Helper()

	var errBuf bytes.Buffer
	var capturedCfg icli.Config
	called := false

	app := icli.NewApp(func(ctx context.Context, c icli.Config) error {
		capturedCfg = c
		called = true
		return nil
	})
	app.Writer = io.Discard // suppress urfave/cli help text on required-flag errors
	app.ErrWriter = &errBuf
	// Suppress os.Exit during tests: ExitErrHandler is called by urfave/cli
	// when an ExitCoder error is returned from Action. We override it so that
	// the error propagates back to the caller instead of calling os.Exit.
	app.ExitErrHandler = func(_ *ucli.Context, _ error) {}

	fullArgs := append([]string{"eth-deposit-gen"}, args...)
	err = app.Run(fullArgs)
	return capturedCfg, errBuf.String(), called, err
}

// validPubkey is a 96-hex-char (48-byte) pubkey used in tests.
// 96 hex chars = 48 bytes, representing a BLS12-381 G1 compressed point.
const validPubkey = "93247f2209abcacfe7b55561da7ae6c4f1df5d7f36a2f4f11e0f5f9d0aa2e7e8b9d0a1c2e3f4a5b6c7d8e9f0a1b2c3d4"
const validPubkey2 = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8"

// TestMissingRequiredFlags verifies that omitting each required flag returns an error.
func TestMissingRequiredFlags(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "missing_validator_key_path",
			args:    []string{"--pubkeys", "0x" + validPubkey, "--network", "hoodi", "--output-dir", dir},
			wantErr: true,
		},
		{
			name:    "missing_pubkeys",
			args:    []string{"--validator-key-path", "/fake/path", "--network", "hoodi", "--output-dir", dir},
			wantErr: true,
		},
		{
			name:    "missing_network",
			args:    []string{"--validator-key-path", "/fake/path", "--pubkeys", "0x" + validPubkey, "--output-dir", dir},
			wantErr: true,
		},
		{
			name:    "missing_output_dir",
			args:    []string{"--validator-key-path", "/fake/path", "--pubkeys", "0x" + validPubkey, "--network", "hoodi"},
			wantErr: true,
		},
		{
			name:    "all_required_flags_present",
			args:    []string{"--validator-key-path", "/fake/path", "--pubkeys", "0x" + validPubkey, "--network", "hoodi", "--output-dir", dir},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := runApp(t, tc.args)
			if tc.wantErr && err == nil {
				t.Errorf("runApp(%v) error = nil, want error", tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("runApp(%v) error = %v, want nil", tc.args, err)
			}
		})
	}
}

// TestInvalidNetwork verifies that an unknown --network value returns an error before run is called.
func TestInvalidNetwork(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		network string
		wantErr bool
	}{
		{"hoodi", false},
		{"mainnet", false},
		{"sepolia", true},
		{"HOODI", true},
		{"Mainnet", true},
		{"", true},
	}

	for _, tc := range tests {
		t.Run("network_"+tc.network, func(t *testing.T) {
			args := []string{
				"--validator-key-path", "/fake/path",
				"--pubkeys", "0x" + validPubkey,
				"--network", tc.network,
				"--output-dir", dir,
			}
			// Empty network will be a missing flag scenario; add it anyway
			if tc.network == "" {
				args = []string{
					"--validator-key-path", "/fake/path",
					"--pubkeys", "0x" + validPubkey,
					"--output-dir", dir,
				}
			}
			_, _, called, err := runApp(t, args)
			if tc.wantErr {
				if err == nil {
					t.Errorf("runApp network=%q error = nil, want error", tc.network)
				}
				if called {
					t.Errorf("runApp network=%q: run was called, want it not called on error", tc.network)
				}
			} else {
				if err != nil {
					t.Errorf("runApp network=%q error = %v, want nil", tc.network, err)
				}
			}
		})
	}
}

// TestPubkeyHexLength verifies that pubkeys with wrong hex length return an error.
func TestPubkeyHexLength(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		pubkeys string
		wantErr bool
	}{
		{
			name:    "correct_length_no_prefix",
			pubkeys: validPubkey,
			wantErr: false,
		},
		{
			name:    "correct_length_with_prefix",
			pubkeys: "0x" + validPubkey,
			wantErr: false,
		},
		{
			name:    "too_short",
			pubkeys: "0x" + validPubkey[:94],
			wantErr: true,
		},
		{
			name:    "too_long",
			pubkeys: "0x" + validPubkey + "ab",
			wantErr: true,
		},
		{
			name:    "empty",
			pubkeys: "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{
				"--validator-key-path", "/fake/path",
				"--pubkeys", tc.pubkeys,
				"--network", "hoodi",
				"--output-dir", dir,
			}
			_, _, _, err := runApp(t, args)
			if tc.wantErr && err == nil {
				t.Errorf("pubkeys=%q: error = nil, want error", tc.pubkeys)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("pubkeys=%q: error = %v, want nil", tc.pubkeys, err)
			}
		})
	}
}

// TestPubkeyInvalidHexChars verifies that non-hex characters in pubkeys return an error.
func TestPubkeyInvalidHexChars(t *testing.T) {
	dir := t.TempDir()

	// Replace some chars with non-hex
	invalidHex := strings.Repeat("g", 96) // 'g' is not a hex char
	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", invalidHex,
		"--network", "hoodi",
		"--output-dir", dir,
	}
	_, _, _, err := runApp(t, args)
	if err == nil {
		t.Error("runApp with invalid hex chars: error = nil, want error")
	}
}

// TestPubkeyMixedPrefix verifies that mixing 0x-prefixed and unprefixed pubkeys returns an error.
func TestPubkeyMixedPrefix(t *testing.T) {
	dir := t.TempDir()

	// First has 0x, second does not
	mixed := "0x" + validPubkey + "," + validPubkey2
	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", mixed,
		"--network", "hoodi",
		"--output-dir", dir,
	}
	_, _, _, err := runApp(t, args)
	if err == nil {
		t.Errorf("runApp with mixed prefix pubkeys: error = nil, want error")
	}
}

// TestNonexistentOutputDir verifies that a non-existent output dir returns an error.
func TestNonexistentOutputDir(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")
	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", "0x" + validPubkey,
		"--network", "hoodi",
		"--output-dir", nonExistent,
	}
	_, _, _, err := runApp(t, args)
	if err == nil {
		t.Errorf("runApp with nonexistent output dir: error = nil, want error")
	}
}

// TestReadOnlyOutputDir verifies that a non-writable output dir returns an error.
func TestReadOnlyOutputDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only dir test skipped: running as root")
	}

	// Create a subdir and make it read-only
	parent := t.TempDir()
	roDir := filepath.Join(parent, "readonly")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	// Register cleanup to restore perms so t.TempDir() cleanup can remove it
	t.Cleanup(func() {
		os.Chmod(roDir, 0o755) //nolint:errcheck
	})
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", "0x" + validPubkey,
		"--network", "hoodi",
		"--output-dir", roDir,
	}
	_, _, _, err := runApp(t, args)
	if err == nil {
		t.Errorf("runApp with read-only output dir: error = nil, want error")
	}
}

// TestSinglePubkeyHappyPath verifies that a single valid pubkey passes through correctly.
func TestSinglePubkeyHappyPath(t *testing.T) {
	dir := t.TempDir()
	args := []string{
		"--validator-key-path", "/some/keystore.json",
		"--pubkeys", "0x" + validPubkey,
		"--network", "hoodi",
		"--output-dir", dir,
	}
	cfg, stderr, called, err := runApp(t, args)
	if err != nil {
		t.Fatalf("runApp: %v", err)
	}
	if !called {
		t.Fatal("run callback was not called")
	}

	// Verify Config fields
	if cfg.KeystorePath != "/some/keystore.json" {
		t.Errorf("KeystorePath = %q, want %q", cfg.KeystorePath, "/some/keystore.json")
	}
	if cfg.Network != network.Hoodi {
		t.Errorf("Network = %q, want %q", cfg.Network, network.Hoodi)
	}
	if cfg.OutputDir != dir {
		t.Errorf("OutputDir = %q, want %q", cfg.OutputDir, dir)
	}
	if len(cfg.Pubkeys) != 1 {
		t.Fatalf("len(Pubkeys) = %d, want 1", len(cfg.Pubkeys))
	}

	// Verify banner on stderr
	if !strings.Contains(stderr, "eth-deposit-gen:") {
		t.Errorf("stderr = %q, want banner containing %q", stderr, "eth-deposit-gen:")
	}
	if !strings.Contains(stderr, "network=hoodi") {
		t.Errorf("stderr = %q, want banner containing %q", stderr, "network=hoodi")
	}
	if !strings.Contains(stderr, "count=1") {
		t.Errorf("stderr = %q, want banner containing %q", stderr, "count=1")
	}
}

// TestMultiPubkeyHappyPath verifies that multiple valid pubkeys pass through correctly.
func TestMultiPubkeyHappyPath(t *testing.T) {
	dir := t.TempDir()
	pubkeys := "0x" + validPubkey + ",0x" + validPubkey2
	args := []string{
		"--validator-key-path", "/some/keystore.json",
		"--pubkeys", pubkeys,
		"--network", "hoodi",
		"--output-dir", dir,
	}
	cfg, stderr, called, err := runApp(t, args)
	if err != nil {
		t.Fatalf("runApp: %v", err)
	}
	if !called {
		t.Fatal("run callback was not called")
	}
	if len(cfg.Pubkeys) != 2 {
		t.Fatalf("len(Pubkeys) = %d, want 2", len(cfg.Pubkeys))
	}

	// Banner must contain first and last pubkey
	if !strings.Contains(stderr, "count=2") {
		t.Errorf("stderr = %q, want banner containing %q", stderr, "count=2")
	}
	// first and last should appear in banner
	if !strings.Contains(stderr, "first_pubkey=") {
		t.Errorf("stderr = %q, want banner containing %q", stderr, "first_pubkey=")
	}
	if !strings.Contains(stderr, "last_pubkey=") {
		t.Errorf("stderr = %q, want banner containing %q", stderr, "last_pubkey=")
	}
}

// TestPassphraseEnvOptional verifies that --passphrase-env is optional and propagated.
func TestPassphraseEnvOptional(t *testing.T) {
	dir := t.TempDir()

	t.Run("without_passphrase_env", func(t *testing.T) {
		args := []string{
			"--validator-key-path", "/some/keystore.json",
			"--pubkeys", "0x" + validPubkey,
			"--network", "hoodi",
			"--output-dir", dir,
		}
		cfg, _, called, err := runApp(t, args)
		if err != nil {
			t.Fatalf("runApp: %v", err)
		}
		if !called {
			t.Fatal("run callback was not called")
		}
		if cfg.PassphraseEnv != "" {
			t.Errorf("PassphraseEnv = %q, want empty string", cfg.PassphraseEnv)
		}
	})

	t.Run("with_passphrase_env", func(t *testing.T) {
		args := []string{
			"--validator-key-path", "/some/keystore.json",
			"--pubkeys", "0x" + validPubkey,
			"--network", "hoodi",
			"--output-dir", dir,
			"--passphrase-env", "MY_PASSPHRASE",
		}
		cfg, _, called, err := runApp(t, args)
		if err != nil {
			t.Fatalf("runApp: %v", err)
		}
		if !called {
			t.Fatal("run callback was not called")
		}
		if cfg.PassphraseEnv != "MY_PASSPHRASE" {
			t.Errorf("PassphraseEnv = %q, want %q", cfg.PassphraseEnv, "MY_PASSPHRASE")
		}
	})
}

// TestBannerFormat verifies the confirmation banner format more precisely.
func TestBannerFormat(t *testing.T) {
	dir := t.TempDir()
	args := []string{
		"--validator-key-path", "/some/keystore.json",
		"--pubkeys", "0x" + validPubkey + ",0x" + validPubkey2,
		"--network", "hoodi",
		"--output-dir", dir,
	}
	_, stderr, _, err := runApp(t, args)
	if err != nil {
		t.Fatalf("runApp: %v", err)
	}

	// Banner must have format: eth-deposit-gen: network=<net> first_pubkey=<hex> last_pubkey=<hex> count=<n>
	if !strings.HasPrefix(stderr, "eth-deposit-gen:") {
		t.Errorf("stderr banner does not start with 'eth-deposit-gen:': %q", stderr)
	}
}

// TestUnprefixedPubkeys verifies that all-unprefixed pubkeys are also accepted.
func TestUnprefixedPubkeys(t *testing.T) {
	dir := t.TempDir()
	args := []string{
		"--validator-key-path", "/some/keystore.json",
		"--pubkeys", validPubkey + "," + validPubkey2,
		"--network", "hoodi",
		"--output-dir", dir,
	}
	cfg, _, called, err := runApp(t, args)
	if err != nil {
		t.Fatalf("runApp with unprefixed pubkeys: %v", err)
	}
	if !called {
		t.Fatal("run callback was not called")
	}
	if len(cfg.Pubkeys) != 2 {
		t.Errorf("len(Pubkeys) = %d, want 2", len(cfg.Pubkeys))
	}
}

// TestNetworkParsedBeforeOtherWork verifies that invalid network is rejected before run is called.
func TestNetworkParsedBeforeOtherWork(t *testing.T) {
	dir := t.TempDir()
	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", "0x" + validPubkey,
		"--network", "invalidnet",
		"--output-dir", dir,
	}
	_, _, called, err := runApp(t, args)
	if err == nil {
		t.Error("runApp with invalid network: error = nil, want error")
	}
	if called {
		t.Error("run was called with invalid network, want it not called")
	}
}

// TestOutputDirIsFile verifies that passing a file path as --output-dir returns an error.
func TestOutputDirIsFile(t *testing.T) {
	// Create a file (not a directory)
	f, err := os.CreateTemp(t.TempDir(), "not-a-dir-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	f.Close()
	filePath := f.Name()

	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", "0x" + validPubkey,
		"--network", "hoodi",
		"--output-dir", filePath,
	}
	_, _, _, err = runApp(t, args)
	if err == nil {
		t.Errorf("runApp with file as output-dir: error = nil, want error")
	}
}

// TestErrorIsExitCoder verifies that validation errors returned by the app are
// ucli.ExitCoder values with exit code 1, matching the urfave/cli convention.
func TestErrorIsExitCoder(t *testing.T) {
	dir := t.TempDir()
	args := []string{
		"--validator-key-path", "/fake/path",
		"--pubkeys", "not-valid-hex!!!",
		"--network", "hoodi",
		"--output-dir", dir,
	}
	_, _, _, err := runApp(t, args)
	if err == nil {
		t.Fatal("runApp with invalid pubkeys: error = nil, want error")
	}

	exitErr, ok := err.(ucli.ExitCoder)
	if !ok {
		t.Fatalf("error type %T is not ucli.ExitCoder", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1", exitErr.ExitCode())
	}
}
