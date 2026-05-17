package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	ucli "github.com/urfave/cli/v2"

	internaltx "github.com/rootwarp/eth-utils/go/internal/tx"
)

func TestExitCodeFor(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"ErrInvalidInput direct", ErrInvalidInput, 2},
		{"ErrInvalidInput wrapped via WrapInputErr", WrapInputErr("--flag", errors.New("bad")), 2},
		{"ErrInvalidInput wrapped via fmt.Errorf %w", fmt.Errorf("wrap: %w", ErrInvalidInput), 2},
		{"context.Canceled", context.Canceled, 4},
		{"ErrUserAborted", ErrUserAborted, 4},
		{"ErrUserAborted wrapped", fmt.Errorf("outer: %w", ErrUserAborted), 4},
		{"ucli.Exit code 2", ucli.Exit("bad input", 2), 2},
		{"ucli.Exit code 1", ucli.Exit("other", 1), 1},
		{"unknown error", errors.New("some unexpected error"), 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExitCodeFor(tc.err)
			if got != tc.want {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestWrapInputErr(t *testing.T) {
	inner := errors.New("bad hex value")
	wrapped := WrapInputErr("--max-fee-per-gas", inner)

	if !errors.Is(wrapped, ErrInvalidInput) {
		t.Error("wrapped error should satisfy errors.Is(ErrInvalidInput)")
	}
	if !errors.Is(wrapped, inner) {
		t.Error("wrapped error should satisfy errors.Is(inner)")
	}
	if ExitCodeFor(wrapped) != 2 {
		t.Errorf("ExitCodeFor(WrapInputErr(...)) = %d, want 2", ExitCodeFor(wrapped))
	}
}

// TestExitCodeFor_BuildUnsignedErrorPath verifies that a BuildUnsigned error
// wrapped via WrapInputErr routes to exit code 2 via the ErrInvalidInput
// sentinel branch (not the ucli.ExitCoder branch).
func TestExitCodeFor_BuildUnsignedErrorPath(t *testing.T) {
	err := WrapInputErr("build", internaltx.ErrNilFeeField)
	if !errors.Is(err, ErrInvalidInput) {
		t.Error("WrapInputErr(build, ErrNilFeeField) must satisfy errors.Is(ErrInvalidInput)")
	}
	if got := ExitCodeFor(err); got != 2 {
		t.Errorf("ExitCodeFor(WrapInputErr(build, ErrNilFeeField)) = %d, want 2", got)
	}
}
