# cli

[![Go Reference](https://pkg.go.dev/badge/github.com/mzattahri/cli.svg)](https://pkg.go.dev/github.com/mzattahri/cli)

`cli` routes command-line arguments to handlers. It handles argv transport —
parsing, routing, and delivery — the same way `net/http` handles HTTP transport.
Application logic, validation, and policy live in your handlers and middleware,
not in the framework.

- **`Mux`** matches command names to `Runner` values and dispatches.
- **`Command`** adds typed input declarations (flags, options, positional
  arguments) to a `Runner`.
- **`Call`** carries parsed input for a single invocation.
- **`Output`** carries stdout and stderr writers.
- **`Program`** ties a root runner to the process environment and handles
  signal, I/O, and exit-code normalization.

## Example

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mzattahri/cli"
)

func main() {
	cmd := &cli.Command{
		Description: "Greet someone by name.",
		Run: func(out *cli.Output, call *cli.Call) error {
			name := call.Args["name"]
			if call.Flags["verbose"] {
				fmt.Fprintln(out.Stdout, "verbose mode")
			}
			_, err := fmt.Fprintf(out.Stdout, "hello %s\n", name)
			return err
		},
	}
	cmd.Flag("verbose", "v", false, "Enable verbose output")
	cmd.Arg("name", "Name to greet")

	mux := cli.NewMux("app")
	mux.Handle("greet", "Print a greeting", cmd)

	if err := (&cli.Program{Runner: mux}).Invoke(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(err.Code)
	}
}
```

For simple programs, the package-level functions operate on a default program:

```go
func main() {
	cli.HandleFunc("greet", "Print a greeting", func(out *cli.Output, call *cli.Call) error {
		_, err := fmt.Fprintf(out.Stdout, "hello %s\n", call.Argv[0])
		return err
	})
	if err := cli.Invoke(context.Background(), os.Args); err != nil {
		os.Exit(err.Code)
	}
}
```

## Input model

The package distinguishes three kinds of command-line input:

| Term       | CLI shape              | Go type  | Example            |
| ---------- | ---------------------- | -------- | ------------------ |
| **Flag**   | Presence-based boolean | `bool`   | `--verbose`        |
| **Option** | Named with value       | `string` | `--host localhost` |
| **Arg**    | Positional, required   | `string` | `<image>`          |

Flags and options must appear before positional arguments. The parser stops
consuming flags at the first non-flag token or after `--`.

Options may be repeated. `OptionSet` stores all values and `.Get()` returns the
last — the same convention as `http.Header`.

## Mux-level flags and options

Flags and options declared on a `Mux` are parsed before subcommand routing. Each
mux in a hierarchy can declare its own, and parsed values accumulate in
`Call.GlobalFlags` and `Call.GlobalOptions` as routing descends:

```go
root := cli.NewMux("app")
root.Flag("verbose", "v", false, "Enable verbose output")

repo := cli.NewMux("repo")
repo.Option("repository", "r", ".", "Repo path")
repo.Handle("init", "Initialize", initCmd)

root.Handle("repo", "Repository commands", repo)

// app --verbose repo --repository /tmp init
// → call.GlobalFlags["verbose"] == true
// → call.GlobalOptions.Get("repository") == "/tmp"
```

## CaptureRest

Setting `Command.CaptureRest` preserves unmatched trailing positional arguments
in `Call.Rest`. This is useful for passthrough commands like ssh or exec:

```go
cmd := &cli.Command{
	CaptureRest: true,
	Run: func(out *cli.Output, call *cli.Call) error {
		fmt.Fprintf(out.Stdout, "image=%s command=%v", call.Args["image"], call.Rest)
		return nil
	},
}
cmd.Arg("image", "Container image")

// app run alpine sh -c "echo hi"
// → call.Args["image"] == "alpine"
// → call.Rest == ["sh", "-c", "echo hi"]
```

## Flag negation

Setting `NegateFlags` on a `Mux` or `Command` enables `--no-` prefix negation
for boolean flags. Negation is bidirectional:

```go
cmd := &cli.Command{
	NegateFlags: true,
	Run: func(out *cli.Output, call *cli.Call) error {
		fmt.Fprintf(out.Stdout, "dns=%t cache=%t",
			call.Flags["accept-dns"], call.Flags["no-cache"])
		return nil
	},
}
cmd.Flag("accept-dns", "", true, "Accept DNS configuration")
cmd.Flag("no-cache", "", true, "Disable cache")

// app --no-accept-dns --cache
// → call.Flags["accept-dns"] == false   (--no- negated "accept-dns")
// → call.Flags["no-cache"] == false     (bare form negated "no-cache")
```

## Defaults

Command-level defaults are not applied during parsing. `Call.ApplyDefaults()`
fills them in — called automatically by `Command.RunCLI` before the handler
runs. Between parsing and `ApplyDefaults`, `Has` reports only values provided on
the command line. Middleware running in this window can inspect what was typed
and fill in values from other sources — environment variables, config files,
vaults — before defaults take effect.

`EnvMapRunner` uses this window to resolve environment variables for flags and
options not provided on the command line.

## Middleware

A `Runner` that wraps another `Runner` is the middleware pattern. Because `Mux`
and `Command` both implement `Runner`, wrapping works at any level of the
command tree:

```go
func withAuth(next cli.Runner) cli.Runner {
	return cli.RunnerFunc(func(out *cli.Output, call *cli.Call) error {
		token, ok := call.Env("AUTH_TOKEN")
		if !ok {
			return fmt.Errorf("AUTH_TOKEN not set")
		}
		ctx := context.WithValue(call.Context(), authKey{}, token)
		return next.RunCLI(out, call.WithContext(ctx))
	})
}

mux.Handle("deploy", "Deploy the app", withAuth(deployCmd))
```

## Error handling

`Program.Invoke` normalizes all results to `*ExitError`. A nil return means
success. `ErrHelp` is returned when help was displayed instead of executing a
command:

```go
err := program.Invoke(ctx, os.Args)
if err == nil {
	return
}
if errors.Is(err, cli.ErrHelp) {
	os.Exit(cli.ExitHelp)
}
fmt.Fprintln(os.Stderr, err)
os.Exit(err.Code)
```

## Core types

A `Runner` handles a command invocation. It has a single method,
`RunCLI(*Output, *Call) error`, making it the CLI equivalent of `http.Handler`.
`RunnerFunc` adapts a plain function to the interface, just as
`http.HandlerFunc` does for HTTP.

A `Mux` routes command paths to runners. It works like `http.ServeMux`: register
runners with `Handle`, and the mux matches argv tokens against registered
command names. Muxes implement `Runner`, so they nest — mount a sub-mux the same
way you mount a sub-handler.

A `Command` pairs a runner with input declarations — flags, options, and
positional arguments declared via `Flag`, `Option`, and `Arg`. The mux uses
these declarations to parse command-level input before calling the handler.

A `Call` carries the parsed input for a single invocation: flags, options, args,
argv tail, stdin, context, and environment lookup. It is the CLI equivalent of
`http.Request`.

`Output` carries `Stdout` and `Stderr` writers for a single invocation.

A `Program` ties a root runner to the process environment — I/O streams, signal
handling, and exit-code normalization. It is the entry point for `main`, the
same way `http.Server` is for a network service.

## Testing

The `clitest` sub-package provides in-memory helpers — no process, no os.Args,
no signal handling:

```go
recorder := clitest.NewRecorder()
call := clitest.NewCall("greet gopher", nil)
err := mux.RunCLI(recorder.Output(), call)
// recorder.Stdout.String() == "hello gopher"
```

## Shell completion

`CompletionRunner` returns a `Runner` that outputs tab completions. Register it
on your mux under any name you like:

```go
mux.Handle("complete", "Output completions", cli.CompletionRunner(mux))
```

The runner expects the current command line as positional arguments after `--`
and prints one `value\tdescription` pair per line.

### Bash

```bash
_myapp() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local words=("${COMP_WORDS[@]:1}")
  COMPREPLY=()
  while IFS=$'\t' read -r val _; do
    COMPREPLY+=("$val")
  done < <(myapp complete -- "${words[@]}")
}
complete -F _myapp myapp
```

### Zsh

```zsh
_myapp() {
  local -a completions
  while IFS=$'\t' read -r val desc; do
    completions+=("${val}:${desc}")
  done < <(myapp complete -- "${words[@]:1}")
  _describe 'command' completions
}
compdef _myapp myapp
```

### Fish

```fish
complete -c myapp -f -a '(myapp complete -- (commandline -cop))'
```
