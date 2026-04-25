package argvtest

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"mz.attahri.com/code/argv"
)

// NewCall returns a [*argv.Call] built from a shell-style argument
// string. Whitespace separates tokens; double and single quotes
// preserve spaces within a token. Inside double quotes, \" and \\
// are honored as escapes. Single quotes are literal.
//
// The call uses [context.TODO]. Set Stdin on the returned Call for
// stdin-dependent tests. NewCall panics on an unclosed quote.
//
// Use [NewCallArgs] when tokens are already split — for example, when
// forwarding a slice through a table-driven test.
func NewCall(args string) *argv.Call {
	return argv.NewCall(context.TODO(), tokenize(args))
}

// NewCallArgs returns a [*argv.Call] from a pre-tokenized slice. Use
// it when the shell-string tokenization in [NewCall] is unnecessary
// (tokens already split, or values contain characters the tokenizer
// would interpret).
//
// The call uses [context.TODO]. Set Stdin on the returned Call for
// stdin-dependent tests.
func NewCallArgs(args []string) *argv.Call {
	return argv.NewCall(context.TODO(), args)
}

// tokenize splits a shell-style argument string into argv tokens.
func tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	inToken := false
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
				continue
			}
			if quote == '"' && c == '\\' && i+1 < len(s) {
				next := s[i+1]
				if next == '"' || next == '\\' {
					cur.WriteByte(next)
					i++
					continue
				}
			}
			cur.WriteByte(c)
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			inToken = true
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' {
			if inToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inToken = false
			}
			continue
		}
		inToken = true
		cur.WriteByte(c)
	}
	if quote != 0 {
		panic("argvtest: unclosed quote in " + strconv.Quote(s))
	}
	if inToken {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// NewLookupFunc returns an [argv.LookupFunc] backed by env, suitable
// for test injection via [argv.EnvMiddleware]. A nil env produces a
// lookup that always reports a miss.
func NewLookupFunc(env map[string]string) argv.LookupFunc {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

// A Recorder captures stdout and stderr, analogous to
// [net/http/httptest.ResponseRecorder].
type Recorder struct {
	stdout, stderr bytes.Buffer
}

// NewRecorder returns a [Recorder] with empty buffers.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Output returns a [*argv.Output] backed by the recorder's buffers.
func (r *Recorder) Output() *argv.Output {
	return &argv.Output{Stdout: &r.stdout, Stderr: &r.stderr}
}

// Stdout returns the captured stdout contents.
func (r *Recorder) Stdout() string { return r.stdout.String() }

// Stderr returns the captured stderr contents.
func (r *Recorder) Stderr() string { return r.stderr.String() }

// Len returns the total number of bytes written to both buffers.
func (r *Recorder) Len() int {
	return r.stdout.Len() + r.stderr.Len()
}

// Reset clears both buffers so a single Recorder can be reused across
// table-driven subtests.
func (r *Recorder) Reset() {
	r.stdout.Reset()
	r.stderr.Reset()
}
