package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunExitsZero(t *testing.T) {
	var buf bytes.Buffer
	code := run(&buf)
	if code != 0 {
		t.Errorf("run() = %d, want 0", code)
	}
}

func TestRunPrintsVersion(t *testing.T) {
	var buf bytes.Buffer
	run(&buf)
	got := buf.String()
	if got == "" {
		t.Error("run() produced no output, want version string")
	}
	if !strings.Contains(got, "eth-deposit-gen") {
		t.Errorf("run() output %q does not contain %q", got, "eth-deposit-gen")
	}
}
