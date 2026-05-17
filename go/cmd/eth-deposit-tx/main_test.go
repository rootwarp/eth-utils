package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	ucli "github.com/urfave/cli/v2"
)

func TestMain_BuildsCleanly(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "/dev/null", ".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/eth-deposit-tx failed: %v\n%s", err, output)
	}
}

func TestApp_Help(t *testing.T) {
	app := newTestApp()
	var buf bytes.Buffer
	app.Writer = &buf
	app.ErrWriter = &buf

	_ = app.Run([]string{"eth-deposit-tx", "--help"})

	s := buf.String()
	if !strings.Contains(s, "eth-deposit-tx") {
		t.Errorf("help output missing app name")
	}
	if !strings.Contains(s, "build") || !strings.Contains(s, "sign") {
		t.Errorf("help output missing expected subcommands build/sign")
	}
}

func TestApp_Version(t *testing.T) {
	app := newTestApp()
	var buf bytes.Buffer
	app.Writer = &buf
	app.ErrWriter = &buf

	_ = app.Run([]string{"eth-deposit-tx", "--version"})

	s := buf.String()
	if !strings.Contains(s, "dev") && !strings.Contains(s, "eth-deposit-tx") {
		t.Errorf("version output unexpected: %s", s)
	}
}

func TestBuildSubcommand_Help(t *testing.T) {
	app := newTestApp()
	var buf bytes.Buffer
	app.Writer = &buf
	app.ErrWriter = &buf

	_ = app.Run([]string{"eth-deposit-tx", "build", "--help"})

	s := buf.String()
	if !strings.Contains(s, "input-file") {
		t.Errorf("build --help missing expected flag, got: %s", s)
	}
}

func TestSignSubcommand_Help(t *testing.T) {
	app := newTestApp()
	var buf bytes.Buffer
	app.Writer = &buf
	app.ErrWriter = &buf

	_ = app.Run([]string{"eth-deposit-tx", "sign", "--help"})

	s := buf.String()
	if !strings.Contains(s, "ledger") {
		t.Errorf("sign --help missing expected ledger flag, got: %s", s)
	}
}

// newTestApp returns a minimal app instance for testing (avoids side effects of the real main).
func newTestApp() *ucli.App {
	return &ucli.App{
		Name:     "eth-deposit-tx",
		Usage:    "Create and sign Ethereum deposit transactions from deposit data JSON",
		Version:  "dev",
		Commands: []*ucli.Command{buildCommand(), signCommand()},
	}
}
