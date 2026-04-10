// Package clitest provides test helpers for [cli] runners and muxes.
//
// It plays the same role as [net/http/httptest]: construct a [*cli.Call]
// with [NewCall], run a [cli.Runner] directly, and inspect captured
// output on a [Recorder]. No process, no os.Args, no signal handling —
// just the runner, its input, and its output.
package clitest
