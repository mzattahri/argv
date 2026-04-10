package cli

import (
	"context"
	"testing"
)

func TestCallStringUsesArgv(t *testing.T) {
	call := NewCall(context.Background(), "", []string{"app", "run", "-a", "-b", "--loud"})

	if got := call.String(); got != "app run -a -b --loud" {
		t.Fatalf("got %q", got)
	}
}

func TestCallStringQuotesArgvTokens(t *testing.T) {
	call := NewCall(context.Background(), "", []string{"app", "say", "hello world", ""})

	if got := call.String(); got != `app say "hello world" ""` {
		t.Fatalf("got %q", got)
	}
}

func TestNilCallString(t *testing.T) {
	var call *Call

	if got := call.String(); got != "" {
		t.Fatalf("got %q", got)
	}
}
