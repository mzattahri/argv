package argv_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"mz.attahri.com/code/argv"
	"mz.attahri.com/code/argv/argvtest"
)

type authContextKey struct{}

func Example() {
	cmd := &argv.Command{
		Run: func(out *argv.Output, call *argv.Call) error {
			name := call.Args.Get("firstname") + call.Options.Get("separator") + call.Args.Get("lastname")
			if call.Flags.Get("uppercase") {
				name = strings.ToUpper(name)
			}
			_, err := fmt.Fprintln(out.Stdout, name)
			return err
		},
	}
	cmd.Flag("uppercase", "u", false, "Uppercase the full name")
	cmd.Option("separator", "s", " ", "Separator between names")
	cmd.Arg("firstname", "First name")
	cmd.Arg("lastname", "Last name")

	program := &argv.Program{
		Usage:       "Print a full name",
		Description: "Join two names with a separator, optionally uppercased.",
	}
	_ = program.Invoke(context.Background(), cmd, []string{"name", "--uppercase", "--separator", "-", "John", "Doe"})
	// Output: JOHN-DOE
}

func ExampleCommand() {
	cmd := &argv.Command{
		CaptureRest: true,
		Run: func(out *argv.Output, call *argv.Call) error {
			detach := call.Flags.Get("detach")
			_, err := fmt.Fprintf(out.Stdout, "image=%s detach=%t command=%v", call.Args.Get("image"), detach, call.Rest)
			return err
		},
	}
	cmd.Flag("detach", "", false, "Run in background")
	cmd.Arg("image", "Image reference")

	mux := &argv.Mux{}
	mux.Handle("run", "Run a container", cmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	program := &argv.Program{Stdout: &stdout, Stderr: &stderr}
	_ = program.Invoke(context.Background(), mux, []string{"app", "run", "--detach", "alpine", "sh", "-c", "echo hi"})
	fmt.Print(stdout.String())
	// Output: image=alpine detach=true command=[sh -c echo hi]
}

func ExampleCommand_negateFlags() {
	cmd := &argv.Command{
		NegateFlags: true,
		Run: func(out *argv.Output, call *argv.Call) error {
			_, err := fmt.Fprintf(out.Stdout, "dns=%t cache=%t",
				call.Flags.Get("accept-dns"), call.Flags.Get("no-cache"))
			return err
		},
	}
	cmd.Flag("accept-dns", "", true, "Accept DNS")
	cmd.Flag("no-cache", "", true, "Disable cache")

	mux := &argv.Mux{}
	mux.Handle("up", "Connect", cmd)

	var stdout bytes.Buffer
	program := &argv.Program{Stdout: &stdout, Stderr: &bytes.Buffer{}}
	_ = program.Invoke(context.Background(), mux, []string{"app", "up", "--no-accept-dns", "--cache"})
	fmt.Print(stdout.String())
	// Output: dns=false cache=false
}

func ExampleProgram_Invoke() {
	cmd := &argv.Command{
		Run: func(out *argv.Output, call *argv.Call) error {
			_, err := fmt.Fprintf(out.Stdout, "hello %s", call.Args.Get("name"))
			return err
		},
	}
	cmd.Arg("name", "Person to greet")

	mux := &argv.Mux{}
	mux.Handle("greet", "Print a greeting", cmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	program := &argv.Program{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	_ = program.Invoke(context.Background(), mux, []string{"app", "greet", "gopher"})
	fmt.Print(stdout.String())
	// Output: hello gopher
}

func ExampleProgram_Invoke_errorHandling() {
	mux := &argv.Mux{}
	mux.Handle("fail", "Always fails", argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		return argv.Errorf(7, "something went wrong")
	}))

	program := &argv.Program{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	err := program.Invoke(context.Background(), mux, []string{"app", "fail"})
	var exitErr *argv.ExitError
	if errors.As(err, &exitErr) {
		fmt.Printf("code=%d err=%s", exitErr.Code, exitErr.Err)
	}
	// Output: code=7 err=something went wrong
}

func ExampleProgram_Invoke_helpDetection() {
	mux := &argv.Mux{}
	mux.Handle("run", "Run something", argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		return nil
	}))

	program := &argv.Program{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	err := program.Invoke(context.Background(), mux, []string{"app", "nope"})
	if err != nil && errors.Is(err, argv.ErrHelp) {
		fmt.Print("help was shown")
	}
	// Output: help was shown
}

func ExampleNewMiddleware() {
	// A Runner wrapping another Runner is the middleware pattern.
	// Use argv.NewMiddleware so Helper/Walker/Completer on inner
	// survive the wrap.
	withLog := argv.NewMiddleware(func(next argv.Runner, out *argv.Output, call *argv.Call) error {
		fmt.Fprintln(out.Stderr, "running")
		return next.RunCLI(out, call)
	})

	inner := argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		_, err := fmt.Fprint(out.Stdout, "done")
		return err
	})

	mux := &argv.Mux{}
	mux.Handle("deploy", "Deploy the app", withLog(inner))

	recorder := argvtest.NewRecorder()
	call := argvtest.NewCall("deploy")
	_ = mux.RunCLI(recorder.Output(), call)
	fmt.Printf("stdout=%s stderr=%s", recorder.Stdout(), recorder.Stderr())
	// Output: stdout=done stderr=running
}

func ExampleNewMiddleware_nested() {
	withLog := argv.NewMiddleware(func(next argv.Runner, out *argv.Output, call *argv.Call) error {
		fmt.Fprintf(out.Stderr, "log ")
		return next.RunCLI(out, call)
	})
	withAuth := argv.NewMiddleware(func(next argv.Runner, out *argv.Output, call *argv.Call) error {
		fmt.Fprintf(out.Stderr, "auth ")
		return next.RunCLI(out, call)
	})

	handler := argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		_, err := fmt.Fprint(out.Stdout, "done")
		return err
	})

	mux := &argv.Mux{}
	mux.Handle("deploy", "Deploy the app", withLog(withAuth(handler)))

	recorder := argvtest.NewRecorder()
	call := argvtest.NewCall("deploy")
	_ = mux.RunCLI(recorder.Output(), call)
	fmt.Printf("stdout=%s stderr=%s", recorder.Stdout(), recorder.Stderr())
	// Output: stdout=done stderr=log auth
}

func ExampleCall_WithContext() {
	call := argvtest.NewCall("whoami")
	ctx := context.WithValue(context.Background(), authContextKey{}, "alice")
	derived := call.WithContext(ctx)

	fmt.Print(derived.Context().Value(authContextKey{}))
	// Output: alice
}

func ExampleMux_Flag_submux() {
	root := &argv.Mux{}
	root.Flag("verbose", "v", false, "Enable verbose output")

	sub := &argv.Mux{}
	sub.Option("path", "p", ".", "Repository path")
	sub.Handle("init", "Initialize a repository", argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		_, err := fmt.Fprintf(out.Stdout, "verbose=%t path=%s",
			call.Flags.Get("verbose"), call.Options.Get("path"))
		return err
	}))

	root.Handle("repo", "Manage repositories", sub)

	var stdout bytes.Buffer
	program := &argv.Program{Stdout: &stdout, Stderr: &bytes.Buffer{}}
	_ = program.Invoke(context.Background(), root, []string{"app", "--verbose", "repo", "--path", "/tmp", "init"})
	fmt.Print(stdout.String())
	// Output: verbose=true path=/tmp
}

func ExampleCompletionCommand() {
	mux := &argv.Mux{}
	mux.Handle("greet", "Print a greeting", argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		_, err := fmt.Fprint(out.Stdout, "hello")
		return err
	}))
	mux.Handle("complete", "Output completions", argv.CompletionCommand(mux))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	program := &argv.Program{Stdout: &stdout, Stderr: &stderr}
	_ = program.Invoke(context.Background(), mux, []string{"app", "complete", "--", "gr"})
	fmt.Print(stdout.String())
	// Output:
	// greet	Print a greeting
}

// deployCmd embeds [*argv.Command] for Run/Help and implements
// [argv.Completer] to provide dynamic --host value suggestions.
// Non-value positions materialize the embedded Command's [argv.Help]
// and delegate to its CompleteCLI, which emits flag, option, and
// argument candidates.
type deployCmd struct {
	*argv.Command
	hosts []string
}

func (d *deployCmd) CompleteCLI(w *argv.TokenWriter, completed []string, partial string) error {
	if len(completed) > 0 && completed[len(completed)-1] == "--host" {
		for _, h := range d.hosts {
			if strings.HasPrefix(h, partial) {
				w.WriteToken(h, "")
			}
		}
		return nil
	}
	var help argv.Help
	d.HelpCLI(&help)
	return help.CompleteCLI(w, completed, partial)
}

func ExampleCompleter() {
	inner := &argv.Command{
		Run: func(out *argv.Output, call *argv.Call) error {
			_, err := fmt.Fprintf(out.Stdout, "host=%s", call.Options.Get("host"))
			return err
		},
	}
	inner.Option("host", "H", "", "Target host")

	cmd := &deployCmd{
		Command: inner,
		hosts:   []string{"prod.example.com", "staging.example.com", "dev.example.com"},
	}

	mux := &argv.Mux{}
	mux.Handle("deploy", "Deploy the app", cmd)
	mux.Handle("complete", "Output completions", argv.CompletionCommand(mux))

	var stdout bytes.Buffer
	program := &argv.Program{Stdout: &stdout, Stderr: &bytes.Buffer{}}

	// "--host <TAB>" with partial "sta" completes to staging.
	_ = program.Invoke(context.Background(), mux, []string{"app", "complete", "--", "deploy", "--host", "sta"})
	fmt.Print(stdout.String())
	// Output:
	// staging.example.com
}

func ExampleEnvMiddleware() {
	// EnvMiddleware resolves environment variables for flags and
	// options not set on the command line. CLI values take precedence.
	envMW := argv.EnvMiddleware(
		map[string]string{
			"host":  "API_HOST",
			"token": "API_TOKEN",
		},
		argvtest.NewLookupFunc(map[string]string{
			"API_HOST":  "env.example.com",
			"API_TOKEN": "secret",
		}),
	)

	handler := &argv.Command{
		Run: func(out *argv.Output, call *argv.Call) error {
			_, err := fmt.Fprintf(out.Stdout, "host=%s token=%s",
				call.Options.Get("host"), call.Options.Get("token"))
			return err
		},
	}
	handler.Option("host", "", "", "API host")
	handler.Option("token", "", "", "API token")

	recorder := argvtest.NewRecorder()
	call := argvtest.NewCall("")
	_ = envMW(handler).RunCLI(recorder.Output(), call)
	fmt.Println(recorder.Stdout())

	// CLI-provided values take precedence over env.
	recorder.Reset()
	call = argvtest.NewCall("")
	call.Options.Set("host", "argv.example.com")
	_ = envMW(handler).RunCLI(recorder.Output(), call)
	fmt.Print(recorder.Stdout())
	// Output:
	// host=env.example.com token=secret
	// host=argv.example.com token=secret
}

func ExampleCall_WithContext_timeout() {
	cmd := &argv.Command{
		Run: func(out *argv.Output, call *argv.Call) error {
			ctx := call.Context()
			// Simulate work that respects the context deadline.
			select {
			case <-ctx.Done():
				_, err := fmt.Fprint(out.Stdout, "timed out")
				return err
			default:
				_, err := fmt.Fprint(out.Stdout, "done")
				return err
			}
		},
	}

	mux := &argv.Mux{}
	mux.Handle("fetch", "Fetch data", cmd)

	// Middleware that enforces a timeout via context.
	withTimeout := func(next argv.Runner) argv.Runner {
		return argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
			ctx, cancel := context.WithTimeout(call.Context(), 0) // immediate timeout
			defer cancel()
			return next.RunCLI(out, call.WithContext(ctx))
		})
	}

	mux.Handle("slow", "Fetch with timeout", withTimeout(cmd))

	var stdout bytes.Buffer
	program := &argv.Program{Stdout: &stdout, Stderr: &bytes.Buffer{}}

	_ = program.Invoke(context.Background(), mux, []string{"app", "fetch"})
	fmt.Println(stdout.String())

	stdout.Reset()
	_ = program.Invoke(context.Background(), mux, []string{"app", "slow"})
	fmt.Print(stdout.String())
	// Output:
	// done
	// timed out
}

func ExampleRecorder() {
	cmd := &argv.Command{
		Run: func(out *argv.Output, call *argv.Call) error {
			_, err := fmt.Fprintf(out.Stdout, "hello %s", call.Args.Get("name"))
			return err
		},
	}
	cmd.Arg("name", "Person to greet")

	mux := &argv.Mux{}
	mux.Handle("greet", "Print a greeting", cmd)

	recorder := argvtest.NewRecorder()
	call := argvtest.NewCall("greet gopher")
	_ = mux.RunCLI(recorder.Output(), call)
	fmt.Print(recorder.Stdout())
	// Output: hello gopher
}

func ExampleProgram_Walk() {
	mux := &argv.Mux{}
	mux.Flag("verbose", "v", false, "Enable verbose output")
	mux.Handle("deploy", "Deploy the app", argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		return nil
	}))

	sub := &argv.Mux{}
	sub.Handle("init", "Initialize a repository", argv.RunnerFunc(func(out *argv.Output, call *argv.Call) error {
		return nil
	}))
	mux.Handle("repo", "Manage repositories", sub)

	program := &argv.Program{}
	for help := range program.Walk("app", mux) {
		if help.Usage != "" {
			fmt.Printf("%s — %s\n", help.FullPath, help.Usage)
		} else {
			fmt.Println(help.FullPath)
		}
	}
	// Output:
	// app
	// app deploy — Deploy the app
	// app repo — Manage repositories
	// app repo init — Initialize a repository
}

func ExampleMux_Match() {
	mux := &argv.Mux{}
	deploy := argv.RunnerFunc(func(*argv.Output, *argv.Call) error { return nil })
	mux.Handle("deploy", "Deploy the app", deploy)

	runner, path := mux.Match([]string{"deploy", "production"})
	fmt.Printf("matched=%t path=%q\n", runner != nil, path)

	runner, path = mux.Match([]string{"unknown"})
	fmt.Printf("matched=%t path=%q\n", runner != nil, path)
	// Output:
	// matched=true path="deploy"
	// matched=false path=""
}

// greetingHelper is a custom Runner that implements [argv.Helper] so
// that [argv.Mux] extracts its Description at registration time and
// [argv.Program.Walk] enumerates it with full help metadata.
type greetingHelper struct{}

func (greetingHelper) RunCLI(out *argv.Output, call *argv.Call) error {
	_, err := fmt.Fprintln(out.Stdout, "hi")
	return err
}

func (greetingHelper) HelpCLI(h *argv.Help) {
	h.Description = "Print a fixed greeting"
	h.Flags = append(h.Flags, argv.HelpFlag{Name: "loud", Short: "l", Usage: "Shout the greeting"})
}

func ExampleHelper() {
	mux := &argv.Mux{}
	mux.Handle("greet", "Say hi", greetingHelper{})

	for help := range (&argv.Program{}).Walk("app", mux) {
		if help.FullPath != "app greet" {
			continue
		}
		fmt.Printf("desc=%q flags=%d\n", help.Description, len(help.Flags))
	}
	// Output: desc="Print a fixed greeting" flags=1
}

// staticWalker is a third-party dispatcher that yields a synthetic
// subtree. Implementing [argv.Walker] lets it participate in
// [argv.Program.Walk] as a first-class node.
type staticWalker struct{ name string }

func (s staticWalker) RunCLI(*argv.Output, *argv.Call) error { return nil }

func (s staticWalker) WalkCLI(path string, base *argv.Help) iter.Seq2[*argv.Help, argv.Runner] {
	return func(yield func(*argv.Help, argv.Runner) bool) {
		if !yield(&argv.Help{Name: s.name, FullPath: path, Usage: "Synthetic root"}, s) {
			return
		}
		child := path + " child"
		yield(&argv.Help{Name: "child", FullPath: child, Usage: "Synthetic child"}, s)
	}
}

func ExampleWalker() {
	mux := &argv.Mux{}
	mux.Handle("plug", "External subtree", staticWalker{name: "plug"})

	for help := range (&argv.Program{}).Walk("app", mux) {
		if help.Usage == "" {
			fmt.Println(help.FullPath)
		} else {
			fmt.Printf("%s — %s\n", help.FullPath, help.Usage)
		}
	}
	// Output:
	// app
	// app plug — Synthetic root
	// app plug child — Synthetic child
}
