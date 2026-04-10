package cli

import (
	"context"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
)

type flagSpec struct {
	Name    string
	Short   string
	Usage   string
	Default bool
}

type optionSpec struct {
	Name    string
	Short   string
	Usage   string
	Default string
}

type argSpec struct {
	Name  string
	Usage string
}

// A Call carries the parsed input for a single command invocation.
type Call struct {
	ctx context.Context

	// Pattern is the matched command path (e.g. "app repo init").
	Pattern string

	// Argv is the remaining argument tail after command routing.
	Argv []string

	// Stdin is the standard input stream.
	Stdin io.Reader

	// Env resolves environment variables.
	Env LookupFunc

	// GlobalFlags holds mux-level boolean flags accumulated during routing.
	GlobalFlags FlagSet

	// GlobalOptions holds mux-level option values accumulated during routing.
	GlobalOptions OptionSet

	// Flags holds command-level boolean flags.
	Flags FlagSet

	// Options holds command-level option values.
	Options OptionSet

	// Args holds bound positional arguments.
	Args ArgSet

	// Rest holds unmatched trailing positional arguments when
	// [Command.CaptureRest] is set.
	Rest []string

	optionDefaults  map[string]string
	flagDefaults    map[string]bool
	defaultsApplied bool
}

// A LookupFunc resolves a name to a value. It follows the signature
// of [os.LookupEnv].
type LookupFunc func(string) (string, bool)

// NewLookupEnv returns a [LookupFunc] backed by env.
// When env is nil the returned function always reports a miss.
func NewLookupEnv(env map[string]string) LookupFunc {
	if env == nil {
		return func(string) (string, bool) { return "", false }

	}
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

// NewCall returns a new [Call] with the given context, pattern, and argv.
// All map fields are initialized to non-nil empty maps.
//
// It panics if ctx is nil.
func NewCall(ctx context.Context, pattern string, argv []string) *Call {
	if ctx == nil {
		panic("cli: nil context")
	}
	return &Call{
		ctx:           ctx,
		Pattern:       pattern,
		Argv:          slices.Clone(argv),
		Env:           NewLookupEnv(nil),
		GlobalFlags:   FlagSet{},
		GlobalOptions: OptionSet{},
		Flags:         FlagSet{},
		Options:       OptionSet{},
		Args:          ArgSet{},
	}
}

// WithContext returns a shallow copy of c with ctx replacing the original
// context. Exported maps and slices are deep-copied.
//
// It panics if ctx is nil.
func (c *Call) WithContext(ctx context.Context) *Call {
	if ctx == nil {
		panic("cli: nil context")
	}
	c2 := *c
	c2.ctx = ctx
	c2.Argv = slices.Clone(c.Argv)
	c2.GlobalFlags = maps.Clone(c.GlobalFlags)
	c2.GlobalOptions = maps.Clone(c.GlobalOptions)
	c2.Flags = maps.Clone(c.Flags)
	c2.Options = maps.Clone(c.Options)
	c2.Args = maps.Clone(c.Args)
	c2.Rest = slices.Clone(c.Rest)
	return &c2
}

// ApplyDefaults fills in default values for any command-level flags and
// options that were not provided on the command line. It is idempotent —
// the first call applies defaults and subsequent calls are no-ops.
//
// Middleware that needs to distinguish "not provided" from "default"
// should inspect the call before ApplyDefaults is called.
// [Command.RunCLI] calls ApplyDefaults automatically before invoking
// the handler, so handlers always see a complete call.
func (c *Call) ApplyDefaults() {
	if c == nil || c.defaultsApplied {
		return
	}
	c.defaultsApplied = true
	for name, val := range c.flagDefaults {
		if !c.Flags.Has(name) {
			c.Flags[name] = val
		}
	}
	for name, val := range c.optionDefaults {
		if !c.Options.Has(name) {
			c.Options.Set(name, val)
		}
	}
}

// Context returns the call's context, defaulting to [context.Background]
// if the call or its context is nil.
func (c *Call) Context() context.Context {
	if c == nil || c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// String returns Argv joined as a shell-like string, quoting tokens that
// contain special characters.
func (c *Call) String() string {
	if c == nil || len(c.Argv) == 0 {
		return ""
	}
	tokens := make([]string, 0, len(c.Argv))
	for _, token := range c.Argv {
		tokens = append(tokens, quoteToken(token))
	}
	return strings.Join(tokens, " ")
}

func quoteToken(token string) string {
	if token == "" {
		return `""`
	}
	if strings.ContainsAny(token, " \t\n\r\"'\\`$&|;()<>[]{}*?!#~") {
		return strconv.Quote(token)
	}
	return token
}
