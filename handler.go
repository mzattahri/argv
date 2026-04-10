package cli

import "io"

// Output carries the output streams for a command invocation.
type Output struct {
	// Stdout is the standard output stream. If it implements [Flusher],
	// [Output.Flush] flushes it after the runner returns.
	Stdout io.Writer

	// Stderr is the standard error stream. If it implements [Flusher],
	// [Output.Flush] flushes it after the runner returns.
	Stderr io.Writer
}

// Flusher is implemented by writers that support flushing buffered output.
type Flusher interface {
	Flush() error
}

// Flush flushes Stdout and Stderr if either implements [Flusher].
func (o *Output) Flush() error {
	if o == nil {
		return nil
	}
	if err := flushWriter(o.Stdout); err != nil {
		return err
	}
	if err := flushWriter(o.Stderr); err != nil {
		return err
	}
	return nil
}

// A Runner handles a CLI command invocation.
//
// RunCLI should write output to out and return nil on success or a
// non-nil error on failure. Returning signals that the invocation is
// finished; the caller may reuse or discard out after RunCLI returns.
// A Runner should not retain references to out or call after returning.
type Runner interface {
	RunCLI(out *Output, call *Call) error
}

// RunnerFunc adapts a plain function to the [Runner] interface.
type RunnerFunc func(out *Output, call *Call) error

// RunCLI calls f(out, call).
func (f RunnerFunc) RunCLI(out *Output, call *Call) error { return f(out, call) }

// A Command combines a handler function with per-command input declarations.
//
// Flags, options, and positional arguments are declared with the [Command.Flag],
// [Command.Option], and [Command.Arg] methods. All declarations must be made
// before the command is registered with [Mux.Handle].
type Command struct {
	// Description is the longer help text shown by [HelpFunc].
	Description string

	// CaptureRest preserves unmatched trailing positional arguments
	// in [Call.Rest].
	CaptureRest bool

	// NegateFlags enables --no- prefix negation for boolean flags.
	// When true, --no-flagname sets a flag to false, and if a flag
	// is declared with a "no-" prefix, --flagname (without the prefix)
	// also sets it to false.
	NegateFlags bool

	// Run handles the command invocation.
	Run RunnerFunc

	// Completer optionally handles tab completion for this command.
	// When nil, the command only completes its own flags and options.
	Completer Completer

	flags   flagSpecs
	options optionSpecs
	args    argSpecs
}

// Flag declares a boolean flag toggled by presence.
//
// short is an optional one-character short form (e.g. "v" for -v).
// An empty string means the flag has no short form.
// It panics on duplicate or reserved names.
func (c *Command) Flag(name, short string, value bool, usage string) {
	if c.options.hasName(name) {
		panic("cli: duplicate command input " + `"` + name + `"`)
	}
	if short != "" && c.options.hasShort(short) {
		panic("cli: duplicate command short input " + `"` + short + `"`)
	}
	c.flags.add(name, short, value, usage)
}

// Option declares a named value option with a default.
//
// short is an optional one-character short form (e.g. "o" for -o).
// An empty string means the option has no short form.
// It panics on duplicate or reserved names.
func (c *Command) Option(name, short, value, usage string) {
	if c.flags.hasName(name) {
		panic("cli: duplicate command input " + `"` + name + `"`)
	}
	if short != "" && c.flags.hasShort(short) {
		panic("cli: duplicate command short input " + `"` + short + `"`)
	}
	c.options.add(name, short, value, usage)
}

// Arg declares a required positional argument.
// It panics if name is empty or duplicated.
func (c *Command) Arg(name, usage string) {
	c.args.add(name, usage)
}

// RunCLI calls [Call.ApplyDefaults] and then c.Run.
func (c *Command) RunCLI(out *Output, call *Call) error {
	call.ApplyDefaults()
	return c.Run(out, call)
}

func (c *Command) inputs() (*flagSpecs, *optionSpecs, *argSpecs) {
	fs := &c.flags
	os := &c.options
	as := &c.args
	if len(fs.specs) == 0 {
		fs = nil
	}
	if len(os.specs) == 0 {
		os = nil
	}
	if len(as.specs) == 0 {
		as = nil
	}
	return fs, os, as
}

// Chain composes middleware functions so they execute in the order given.
// Each middleware wraps a [Runner] in another Runner. The first middleware
// is the outermost wrapper:
//
//	stack := Chain(withLogging, withAuth)
//	mux.Handle("deploy", "Deploy", stack(deployCmd))
func Chain(mw ...func(Runner) Runner) func(Runner) Runner {
	return func(r Runner) Runner {
		for i := len(mw) - 1; i >= 0; i-- {
			r = mw[i](r)
		}
		return r
	}
}

func flushWriter(w io.Writer) error {
	if f, ok := w.(Flusher); ok {
		return f.Flush()
	}
	return nil
}
