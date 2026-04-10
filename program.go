package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"slices"
	"sync"
)

var notifyContext = signal.NotifyContext

// A Program describes the process-level invocation environment for a root
// [Runner].
type Program struct {
	// Name is shown in usage output. When empty, [Program.Invoke]
	// uses args[0].
	Name string

	// Runner is the root runner. When nil on [DefaultProgram], a
	// default [Mux] is created lazily.
	Runner Runner

	// Stdout is the standard output writer. When nil, [Program.Invoke]
	// uses [os.Stdout].
	Stdout io.Writer

	// Stderr is the standard error writer. When nil, [Program.Invoke]
	// uses [os.Stderr].
	Stderr io.Writer

	// Stdin is the standard input reader. When nil, [Program.Invoke]
	// uses [os.Stdin].
	Stdin io.Reader

	// Env resolves environment variables. When nil,
	// [Program.Invoke] uses [os.LookupEnv].
	Env LookupFunc

	// Usage is the short summary shown in top-level help output.
	Usage string

	// Description is longer free-form help text.
	Description string

	// HelpFunc overrides the default help renderer.
	HelpFunc HelpFunc

	// IgnoreSignals disables the default SIGINT context wrapping.
	IgnoreSignals bool
}

// Flag declares a boolean flag on the underlying [*Mux] runner.
// It panics if the program's Runner is not a [*Mux].
// See [Mux.Flag] for details.
func (p *Program) Flag(name, short string, value bool, usage string) {
	p.ensureMux().Flag(name, short, value, usage)
}

// Option declares a named value option on the underlying [*Mux] runner.
// It panics if the program's Runner is not a [*Mux].
// See [Mux.Option] for details.
func (p *Program) Option(name, short, value, usage string) {
	p.ensureMux().Option(name, short, value, usage)
}

func (p *Program) ensureMux() *Mux {
	if p == DefaultProgram {
		return defaultProgramMux()
	}
	mux, ok := p.Runner.(*Mux)
	if !ok || mux == nil {
		panic("cli: Program.Flag/Option requires a *Mux runner")
	}
	return mux
}

// DefaultProgram is the default [Program] used by the package-level
// [Handle], [HandleFunc], and [Invoke] functions.
var DefaultProgram = &Program{
	Stdout:    os.Stdout,
	Stderr:    os.Stderr,
	Stdin:     os.Stdin,
	Env: os.LookupEnv,
}

var defaultProgramMuxMu sync.Mutex

func (p *Program) output() *Output {
	if p == nil {
		return &Output{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	stdout := p.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := p.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Output{Stdout: stdout, Stderr: stderr}
}

// Invoke runs the program's root [Runner] and normalizes the result to an
// [*ExitError]. An explicit --help request returns nil after rendering
// help. It panics if ctx is nil.
func (p *Program) Invoke(ctx context.Context, args []string) *ExitError {
	if ctx == nil {
		panic("cli: nil context")
	}
	if p == nil {
		p = DefaultProgram
	}
	runner := p.Runner
	if runner == nil {
		if p == DefaultProgram {
			runner = defaultProgramMux()
		} else {
			panic("cli: nil program runner")
		}
	}
	stdin := p.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	lookupEnv := p.Env
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	programName := p.Name
	if len(args) > 0 {
		if programName == "" {
			programName = args[0]
		}
		args = args[1:]
	} else if programName == "" {
		if mux, ok := runner.(*Mux); ok && mux.Name != "" {
			programName = mux.Name
		} else {
			programName = "app"
		}
	}
	call := &Call{
		ctx:     ctx,
		Pattern: programName,
		Argv:    slices.Clone(args),
		Stdin:   stdin,
		Env:     lookupEnv,
	}

	out, call, stop := prepareProgramInvocation(p, call)
	if stop != nil {
		defer stop()
	}

	err := invokeRunnerWithProgram(p, runner, out, call, programName)

	// Flush stdout/stderr after the runner returns.
	if fErr := out.Flush(); fErr != nil && err == nil {
		err = fErr
	}

	if err == nil {
		return nil
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr
	}
	return &ExitError{Code: exitCode(err), Err: err}
}

// Flag declares a boolean flag on the [DefaultProgram] mux.
// The mux is created lazily on first use.
func Flag(name, short string, value bool, usage string) {
	defaultProgramMux().Flag(name, short, value, usage)
}

// Option declares a value option on the [DefaultProgram] mux.
// The mux is created lazily on first use.
func Option(name, short, value, usage string) {
	defaultProgramMux().Option(name, short, value, usage)
}

// Handle registers runner on the [DefaultProgram] mux.
// The mux is created lazily on first use.
func Handle(pattern string, usage string, runner Runner) {
	defaultProgramMux().Handle(pattern, usage, runner)
}

// HandleFunc registers fn on the [DefaultProgram] mux.
// The mux is created lazily on first use.
func HandleFunc(pattern string, usage string, fn func(*Output, *Call) error) {
	defaultProgramMux().HandleFunc(pattern, usage, fn)
}

// Invoke calls [DefaultProgram].Invoke(ctx, args).
func Invoke(ctx context.Context, args []string) *ExitError {
	return DefaultProgram.Invoke(ctx, args)
}

func defaultProgramMux() *Mux {
	defaultProgramMuxMu.Lock()
	defer defaultProgramMuxMu.Unlock()

	if mux, ok := DefaultProgram.Runner.(*Mux); ok && mux != nil {
		if mux.Name == "" {
			mux.Name = defaultProgramMuxName()
		}
		return mux
	}
	if DefaultProgram.Runner != nil {
		panic("cli: DefaultProgram.Runner is not a *Mux")
	}
	mux := NewMux(defaultProgramMuxName())
	DefaultProgram.Runner = mux
	return mux
}

func defaultProgramMuxName() string {
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "app"
}

func invokeRunnerWithProgram(program *Program, runner Runner, out *Output, call *Call, fullPath string) error {
	if mux, ok := runner.(*Mux); ok {
		name := fullPath
		if name == "" {
			name = mux.Name
		}
		return mux.runWithPath(out, call, name, program.Usage, program.Description, nil, program.HelpFunc)
	}

	// Non-mux runner: parse for --help handling only.
	parsed, err := parseInput(nil, nil, call.Argv, false)
	if err != nil {
		if errors.Is(err, errFlagHelp) {
			return resolveHelpFunc(program.HelpFunc)(out.Stderr, &Help{
				Name:        lastPathSegment(fullPath),
				FullPath:    fullPath,
				Usage:       program.Usage,
				Description: program.Description,
			})
		}
		return err
	}

	newCall := &Call{
		ctx:     call.Context(),
		Pattern: call.Pattern,
		Argv:    slices.Clone(parsed.args),
		Stdin:   call.Stdin,
		Env:     call.Env,
	}
	return runner.RunCLI(out, newCall)
}

func prepareProgramInvocation(program *Program, call *Call) (*Output, *Call, context.CancelFunc) {
	out := program.output()
	if program.IgnoreSignals {
		return out, call, nil
	}
	ctx, stop := notifyContext(call.Context(), os.Interrupt)
	return out, call.WithContext(ctx), stop
}
