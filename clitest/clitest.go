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
	call := cli.NewCall(context.Background(), "", strings.Fields(arg))
	if stdin != nil {
		call.Stdin = bytes.NewReader(stdin)
	}
	return call
}

// An OutputRecorder captures a single output stream.
type OutputRecorder struct {
	buf bytes.Buffer
}

// Write appends p to the recorded output.
func (o *OutputRecorder) Write(p []byte) (int, error) {
	return o.buf.Write(p)
}

// String returns the recorded output as a string.
func (o *OutputRecorder) String() string {
	return o.buf.String()
}

// Bytes returns the recorded output as a byte slice.
func (o *OutputRecorder) Bytes() []byte {
	return o.buf.Bytes()
}

// Len returns the number of recorded bytes.
func (o *OutputRecorder) Len() int {
	return o.buf.Len()
}

// Reset clears the recorded output.
func (o *OutputRecorder) Reset() {
	o.buf.Reset()
}

// A Recorder captures stdout and stderr for test assertions.
// It is the CLI equivalent of [net/http/httptest.ResponseRecorder].
type Recorder struct {
	Stdout *OutputRecorder
	Stderr *OutputRecorder
}

// NewRecorder returns a [Recorder] with empty in-memory output buffers.
func NewRecorder() *Recorder {
	return &Recorder{
		Stdout: &OutputRecorder{},
		Stderr: &OutputRecorder{},
	}
}

// Output returns a [*cli.Output] backed by the recorder's buffers.
func (r *Recorder) Output() *cli.Output {
	return &cli.Output{Stdout: r.Stdout, Stderr: r.Stderr}
}

// Reset clears both captured output streams.
func (r *Recorder) Reset() {
	r.Stdout.Reset()
	r.Stderr.Reset()
}
