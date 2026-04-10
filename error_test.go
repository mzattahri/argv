package cli

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitOK},
		{name: "help", err: ErrHelp, want: ExitHelp},
		{name: "default", err: errors.New("boom"), want: ExitFailure},
		{name: "exit error", err: &ExitError{Code: 42, Err: errors.New("nope")}, want: 42},
		{name: "wrapped exit error", err: errors.Join(errors.New("outer"), &ExitError{Code: 7, Err: errors.New("inner")}), want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{name: "ok", got: ExitOK, want: 0},
		{name: "failure", got: ExitFailure, want: 1},
		{name: "help", got: ExitHelp, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %d, want %d", tt.got, tt.want)
			}
		})
	}
}
