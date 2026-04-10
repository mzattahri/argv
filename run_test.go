package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

func TestInvokeDefaultsNilTTYAndStdin(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("noop", "", RunnerFunc(func(out *Output, call *Call) error {
		if out.Stdout == nil {
			t.Fatal("expected non-nil stdout from default Output")
		}
		if out.Stderr == nil {
			t.Fatal("expected non-nil stderr from default Output")
		}
		if call.Stdin == nil {
			t.Fatal("expected non-nil stdin from default")
		}
		return nil
	}))

	program := *DefaultProgram
	program.Runner = mux
	if err := program.Invoke(context.Background(), []string{"app", "noop"}); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeSkipsArgv0(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("echo", "", RunnerFunc(func(out *Output, call *Call) error {
		value, _ := call.Env("TERMINAL_TEST_VALUE")
		_, err := out.Stdout.Write([]byte(call.Pattern + ":" + call.Argv[0] + ":" + value))
		return err
	}))

	t.Setenv("TERMINAL_TEST_VALUE", "ok")

	var out bytes.Buffer
	program := &Program{Stdout: &out, Stderr: &bytes.Buffer{}, Env: os.LookupEnv, Runner: mux}
	err := program.Invoke(context.Background(), []string{"app", "echo", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "app echo:hello:ok" {
		t.Fatalf("got %q, want %q", got, "app echo:hello:ok")
	}
}

func TestInvokeExplicitHelpReturnsSuccess(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("echo", "Echo output", RunnerFunc(func(out *Output, call *Call) error { return nil }))

	var errout bytes.Buffer
	program := &Program{Stdout: io.Discard, Stderr: &errout, Runner: mux}
	err := program.Invoke(context.Background(), []string{"app", "--help"})
	if err != nil {
		t.Fatalf("got err=%v, want nil", err)
	}
	if got := errout.String(); got == "" {
		t.Fatal("expected help output")
	}
}
