// Package cli parses command lines and dispatches to runners.
//
// A [Mux] matches command names to [Runner] values. A [Command] parses
// flags, options, and positional arguments before invoking its runner.
// A [Call] holds the parsed input for a single invocation. A [Program]
// binds a root runner to the process environment.
//
// Flags are boolean values set by presence. Options carry string values
// and may be repeated. Positional arguments are required and ordered.
// Flags and options must appear before positional arguments; parsing
// stops at the first non-flag token or after "--".
//
// Flags and options declared on a [Mux] are parsed before subcommand
// routing. All parsed values accumulate in [Call.Flags] and
// [Call.Options]. Defaults from each routing level are applied
// automatically during dispatch. [FlagSet.Explicit] and
// [OptionSet.Explicit] distinguish command-line input from defaults.
//
// # Testing
//
// The clitest sub-package provides in-memory helpers for testing
// runners without a process, os.Args, or signal handling:
//
//	recorder := clitest.NewRecorder()
//	call := clitest.NewCall("greet gopher", nil)
//	err := mux.RunCLI(recorder.Output(), call)
//	// recorder.Stdout.String() == "hello gopher\n"
package cli
