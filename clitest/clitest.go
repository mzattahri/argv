package clitest

import (
	"bytes"
	"context"
	"strings"

	"github.com/mzattahri/cli"
)

// NewCall returns a [*cli.Call] from a space-separated argument string and
// optional raw stdin bytes. The call uses [context.Background].
func NewCall(arg string, stdin []byte) *cli.Call {
	call := cli.NewCall(context.Background(), strings.Fields(arg))
	if stdin != nil {
		call.Stdin = bytes.NewReader(stdin)
	}
	return call
}

// A Recorder captures stdout and stderr, analogous to
// [net/http/httptest.ResponseRecorder].
type Recorder struct {
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

// NewRecorder returns a [Recorder] with empty buffers.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Output returns a [*cli.Output] backed by the recorder's buffers.
func (r *Recorder) Output() *cli.Output {
	return &cli.Output{Stdout: &r.Stdout, Stderr: &r.Stderr}
}

// Len returns the total number of bytes written to both buffers.
func (r *Recorder) Len() int {
	return r.Stdout.Len() + r.Stderr.Len()
}

// Reset clears both buffers.
func (r *Recorder) Reset() {
	r.Stdout.Reset()
	r.Stderr.Reset()
}
