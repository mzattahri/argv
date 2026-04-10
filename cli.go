// Package cli routes command-line arguments to handlers.
//
// A [Mux] maps command names to [Runner] values. A [*Command] adds
// typed input declarations (flags, options, and positional arguments)
// to a Runner. A [Program] ties a root runner to the process
// environment and handles signal, I/O, and exit-code normalization.
//
//	cmd := &cli.Command{
//		Run: func(out *cli.Output, call *cli.Call) error {
//			_, err := fmt.Fprintf(out.Stdout, "hello %s\n", call.Args["name"])
//			return err
//		},
//	}
//	cmd.Arg("name", "Person to greet")
//
//	mux := cli.NewMux("app")
//	mux.Handle("greet", "Print a greeting", cmd)
//
// # Input model
//
// The package distinguishes three kinds of command-line input:
//
//   - Flags are booleans toggled by presence (--verbose).
//   - Options carry a string value (--host localhost). Options may be
//     repeated; [OptionSet] stores all values and [OptionSet.Get]
//     returns the last.
//   - Positional arguments are required and ordered.
//
// Flags and options must appear before positional arguments; the parser
// stops at the first non-flag token or after "--".
//
// In POSIX terminology flags and options are both "options." This
// package separates them because the two forms have different
// signatures.
//
// # Mux-level flags
//
// Flags and options declared on a [Mux] are parsed before subcommand
// routing. Parsed values accumulate in [Call.GlobalFlags] and
// [Call.GlobalOptions] as routing descends through mounted sub-muxes:
//
//	root := cli.NewMux("app")
//	root.Flag("verbose", "v", false, "Enable verbose output")
//
//	repo := cli.NewMux("repo")
//	repo.Option("repository", "r", ".", "Repo path")
//	repo.Handle("init", "Initialize", initCmd)
//
//	root.Handle("repo", "Repository commands", repo)
//	// app --verbose repo --repository /tmp init
//
// # Submux mounting
//
// Passing a [*Mux] to [Mux.Handle] mounts it as a sub-mux. Each
// sub-mux is a self-contained [Runner] that can be built and tested
// independently:
//
//	root.Handle("repo", "Repository commands", repoMux())
//
// # CaptureRest
//
// Setting [Command.CaptureRest] preserves unmatched trailing positional
// arguments in [Call.Rest]. This is useful for passthrough commands like
// ssh or exec:
//
//	cmd := &cli.Command{CaptureRest: true, Run: handler}
//	cmd.Arg("target", "Host to connect to")
//	// app ssh host -- -L 8080:localhost:80
//	// call.Args["target"] == "host", call.Rest == ["-L", "8080:localhost:80"]
//
// # Flag negation
//
// Setting [Mux.NegateFlags] or [Command.NegateFlags] enables --no-
// prefix negation for boolean flags. Negation is bidirectional:
// --no-verbose negates a flag named "verbose", and --cache negates a
// flag named "no-cache".
//
// # Defaults
//
// Command-level defaults are not applied during parsing. Instead,
// [Call.ApplyDefaults] fills them in — called automatically by
// [Command.RunCLI] before the handler runs. Between parsing and
// ApplyDefaults, [OptionSet.Has] and [FlagSet.Has] report only values
// provided on the command line. Middleware running in this window can
// inspect what was typed and fill in values from other sources before
// defaults take effect.
//
// # Middleware
//
// A [Runner] that wraps another [Runner] is the middleware pattern.
// Because [Mux] and [Command] both implement [Runner], wrapping works
// at any level of the command tree:
//
//	func withAuth(next cli.Runner) cli.Runner {
//		return cli.RunnerFunc(func(out *cli.Output, call *cli.Call) error {
//			token, ok := call.Env("AUTH_TOKEN")
//			if !ok {
//				return fmt.Errorf("AUTH_TOKEN not set")
//			}
//			ctx := context.WithValue(call.Context(), authKey{}, token)
//			return next.RunCLI(out, call.WithContext(ctx))
//		})
//	}
package cli
