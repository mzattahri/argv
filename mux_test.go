package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func runMux(ctx context.Context, mux *Mux, stdout io.Writer, stderr io.Writer, args []string) error {
	call := NewCall(ctx, "app", args)
	return mux.RunCLI(&Output{Stdout: stdout, Stderr: stderr}, call)
}

func TestBasicDispatch(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("greet", "Say hello", RunnerFunc(func(out *Output, call *Call) error {
		_, err := fmt.Fprintf(out.Stdout, "hello %s", call.Argv[0])
		return err
	}))
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"greet", "Go"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello Go" {
		t.Fatalf("got %q", got)
	}
}

func TestHandleFunc(t *testing.T) {
	mux := NewMux("app")
	mux.HandleFunc("greet", "Say hello", func(out *Output, call *Call) error {
		_, err := fmt.Fprintf(out.Stdout, "hello %s", call.Argv[0])
		return err
	})
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"greet", "Go"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello Go" {
		t.Fatalf("got %q", got)
	}
}

func TestCommandFlagsAndOptions(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			repository := call.Options.Get("repository")
			verbose := call.Flags["verbose"]
			_, err := fmt.Fprintf(out.Stdout, "%s|%t", repository, verbose)
			return err
		},
	}
	cmd.Option("repository", "r", "", "repo path")
	cmd.Flag("verbose", "v", false, "verbose")
	mux.Handle("track", "", cmd)
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"track", "--repository", "/tmp/repo", "--verbose"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "/tmp/repo|true" {
		t.Fatalf("got %q", got)
	}
}

func TestShortFlagsAndOptions(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "%t|%t|%s", call.Flags["verbose"], call.Flags["force"], call.Options.Get("repository"))
			return err
		},
	}
	cmd.Flag("verbose", "v", false, "verbose")
	cmd.Flag("force", "f", false, "force")
	cmd.Option("repository", "r", "", "repo path")
	mux.Handle("track", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"track", "-vf", "-r", "/tmp/repo"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "true|true|/tmp/repo" {
		t.Fatalf("got %q", got)
	}
}

func TestCommandRunnerField(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("version", "", &Command{
		Run: func(out *Output, call *Call) error {
			_, err := io.WriteString(out.Stdout, "v1.0.0")
			return err
		},
	})
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "v1.0.0" {
		t.Fatalf("got %q", got)
	}
}

func TestHandlePointerCommandUsesDescription(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Description: "Print the current version.",
		Run: func(*Output, *Call) error {
			return nil
		},
	}
	mux.Handle("version", "Show version", cmd)

	var gotHelp *Help
	program := &Program{
		Runner:   mux,
		Stdout:   io.Discard,
		Stderr:   io.Discard,
		HelpFunc: func(_ io.Writer, help *Help) error { gotHelp = help; return nil },
	}

	err := program.Invoke(context.Background(), []string{"app", "version", "--help"})
	if err != nil {
		t.Fatal(err)
	}
	if gotHelp == nil {
		t.Fatal("expected help to be rendered")
	}
	if gotHelp.Description != "Print the current version." {
		t.Fatalf("got description %q", gotHelp.Description)
	}
}

func TestPositionalArgsAreStrings(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "%s|%s", call.Args["repo"], call.Args["path"])
			return err
		},
	}
	cmd.Arg("repo", "Repository name")
	cmd.Arg("path", "File path")
	mux.Handle("open", "", cmd)
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"open", "terminal", "README.md"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "terminal|README.md" {
		t.Fatalf("got %q", got)
	}
}

func TestCaptureRestPreservesTrailingArgs(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		CaptureRest: true,
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "%v", call.Rest)
			return err
		},
	}
	mux.Handle("match", "", cmd)
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"match", "a*", "b*", "c*"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[a* b* c*]" {
		t.Fatalf("got %q", got)
	}
}

func TestProgramGlobalFlagsAndOptions(t *testing.T) {
	mux := NewMux("app")
	mux.Option("host", "", "", "daemon socket")
	mux.Flag("verbose", "", false, "verbose")
	mux.Handle("run", "", RunnerFunc(func(out *Output, call *Call) error {
		host := call.GlobalOptions.Get("host")
		verbose := call.GlobalFlags["verbose"]
		_, err := fmt.Fprintf(out.Stdout, "%s|%t", host, verbose)
		return err
	}))
	var out bytes.Buffer
	program := &Program{
		Runner: mux,
		Stdout: &out,
		Stderr: io.Discard,
	}
	if err := program.Invoke(context.Background(), []string{"app", "--host", "unix:///tmp/docker.sock", "--verbose", "run"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "unix:///tmp/docker.sock|true" {
		t.Fatalf("got %q", got)
	}
}

func TestMountedMuxHelpIncludesProgramGlobals(t *testing.T) {
	root := NewMux("app")
	root.Flag("verbose", "v", false, "verbose")
	root.Option("config", "c", "", "config file")
	sub := NewMux("repo")
	var gotHelp *Help
	sub.Handle("init", "Initialize repo", RunnerFunc(func(*Output, *Call) error {
		return nil
	}))
	root.Handle("repo", "Repository commands", sub)

	program := &Program{
		Runner:   root,
		Stdout:   io.Discard,
		Stderr:   io.Discard,
		HelpFunc: func(_ io.Writer, help *Help) error { gotHelp = help; return nil },
	}

	if err := program.Invoke(context.Background(), []string{"app", "repo", "init", "--help"}); err != nil {
		t.Fatal(err)
	}
	if gotHelp == nil {
		t.Fatal("expected help to be rendered")
	}
	if len(gotHelp.GlobalFlags) != 1 || gotHelp.GlobalFlags[0].Name != "verbose" {
		t.Fatalf("got global flags %#v", gotHelp.GlobalFlags)
	}
	if len(gotHelp.GlobalOptions) != 1 || gotHelp.GlobalOptions[0].Name != "config" {
		t.Fatalf("got global options %#v", gotHelp.GlobalOptions)
	}
}

func TestNestedCommands(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("repo init", "", RunnerFunc(func(out *Output, call *Call) error {
		_, err := io.WriteString(out.Stdout, call.Argv[0])
		return err
	}))
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"repo", "init", "demo"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "demo" {
		t.Fatalf("got %q", got)
	}
}

func TestMount(t *testing.T) {
	sub := NewMux("repo")
	sub.Handle("init", "Initialize", RunnerFunc(func(out *Output, call *Call) error {
		_, err := io.WriteString(out.Stdout, "repo-init")
		return err
	}))
	mux := NewMux("app")
	mux.Handle("repo", "Manage repositories", sub)
	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"repo", "init"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "repo-init" {
		t.Fatalf("got %q", got)
	}
}

func TestUnknownCommandShowsHelp(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("greet", "Say hello", RunnerFunc(func(out *Output, call *Call) error { return nil }))
	var errout bytes.Buffer
	err := runMux(context.Background(), mux, io.Discard, &errout, []string{"nope"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("got err=%v", err)
	}
	if !strings.Contains(errout.String(), `unknown command "nope"`) {
		t.Fatalf("missing unknown command message:\n%s", errout.String())
	}
	if !strings.Contains(errout.String(), "greet") {
		t.Fatalf("help missing command:\n%s", errout.String())
	}
}

func TestNoSubcommandDoesNotSayUnknown(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("greet", "Say hello", RunnerFunc(func(out *Output, call *Call) error { return nil }))
	var errout bytes.Buffer
	err := runMux(context.Background(), mux, io.Discard, &errout, nil)
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("got err=%v", err)
	}
	if strings.Contains(errout.String(), "unknown command") {
		t.Fatalf("should not say unknown command when no args given:\n%s", errout.String())
	}
}

func TestProgramHelpFunc(t *testing.T) {
	mux := NewMux("app")
	mux.Handle("greet", "Say hello", RunnerFunc(func(out *Output, call *Call) error { return nil }))

	var errout bytes.Buffer
	program := &Program{
		Runner: mux,
		Stdout: io.Discard,
		Stderr: &errout,
		HelpFunc: func(w io.Writer, help *Help) error {
			_, _ = io.WriteString(w, "custom help")
			return nil
		},
	}
	err := program.Invoke(context.Background(), []string{"app", "--help"})
	if err != nil {
		t.Fatalf("got err=%v", err)
	}
	if got := errout.String(); got != "custom help" {
		t.Fatalf("got %q", got)
	}
}

func TestHelpIncludesOptionsAndArgs(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error { return nil },
	}
	cmd.Option("repository", "", "", "repo path")
	cmd.Arg("path", "Path to open")
	mux.Handle("open", "Open files", cmd)
	var errout bytes.Buffer
	if err := runMux(context.Background(), mux, io.Discard, &errout, []string{"open", "--help"}); err != nil {
		t.Fatalf("got err=%v", err)
	}
	help := errout.String()
	for _, want := range []string{"--repository", "<path>"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestCommandRawPreservesCommandLocalArgv(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "%v", call.Argv)
			return err
		},
	}
	cmd.Option("repository", "", "", "repo path")
	mux.Handle("open", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"open", "--repository", "/tmp/repo", "README.md"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[--repository /tmp/repo README.md]" {
		t.Fatalf("got %q", got)
	}
}

func TestCustomHelpGetsRootName(t *testing.T) {
	mux := NewMux("app")

	var errout bytes.Buffer
	program := &Program{
		Runner: mux,
		Stdout: io.Discard,
		Stderr: &errout,
		HelpFunc: func(w io.Writer, help *Help) error {
			_, _ = fmt.Fprintf(w, "%s|%s", help.Name, help.FullPath)
			return nil
		},
	}
	if err := program.Invoke(context.Background(), []string{"app", "--help"}); err != nil {
		t.Fatalf("got err=%v", err)
	}
	if got := errout.String(); got != "app|app" {
		t.Fatalf("got %q", got)
	}
}

func TestHelpDoesNotShadowOptionValue(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			value := call.Options.Get("template")
			_, err := io.WriteString(out.Stdout, value)
			return err
		},
	}
	cmd.Option("template", "", "", "template name")
	mux.Handle("render", "", cmd)

	var out bytes.Buffer
	var errout bytes.Buffer
	if err := runMux(context.Background(), mux, &out, &errout, []string{"render", "--template", "--help"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "--help" {
		t.Fatalf("got %q", got)
	}
	if got := errout.String(); got != "" {
		t.Fatalf("got unexpected stderr %q", got)
	}
}

func TestMuxFlagsAreScopedToLevel(t *testing.T) {
	root := NewMux("app")
	root.Option("host", "", "", "daemon socket")
	sub := NewMux("repo")
	sub.Handle("init", "", RunnerFunc(func(out *Output, call *Call) error {
		_, err := io.WriteString(out.Stdout, "run")
		return err
	}))
	root.Handle("repo", "", sub)

	// Root-level option placed at the root position works.
	var out bytes.Buffer
	program := &Program{
		Runner: root,
		Stdout: &out,
		Stderr: io.Discard,
	}
	if err := program.Invoke(context.Background(), []string{"app", "--host", "unix:///tmp/docker.sock", "repo", "init"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "run" {
		t.Fatalf("got %q", got)
	}
}

func TestProgramInvokeInstallsSignalContextByDefault(t *testing.T) {
	orig := notifyContext
	t.Cleanup(func() { notifyContext = orig })
	parentCtx := context.Background()
	derivedCtx := context.WithValue(parentCtx, struct{}{}, "derived")
	called := false
	notifyContext = func(parent context.Context, sig ...os.Signal) (context.Context, context.CancelFunc) {
		called = true
		return derivedCtx, func() {}
	}
	mux := NewMux("app")
	mux.Handle("run", "", RunnerFunc(func(out *Output, call *Call) error {
		if call.Context() != derivedCtx {
			t.Fatal("expected derived context")
		}
		return nil
	}))
	if err := (&Program{Runner: mux}).Invoke(parentCtx, []string{"app", "run"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected notifyContext call")
	}
}

func TestProgramMuxRootHandlerWithGlobalOptions(t *testing.T) {
	mux := NewMux("app")
	mux.Option("host", "", "", "daemon socket")
	mux.HandleFunc("", "Run the root command", func(out *Output, call *Call) error {
		host := call.GlobalOptions.Get("host")
		_, err := fmt.Fprintf(out.Stdout, "%s", host)
		return err
	})

	var out bytes.Buffer
	var errout bytes.Buffer
	program := &Program{
		Runner: mux,
		Stdout: &out,
		Stderr: &errout,
		Usage:  "Run the root command",
	}
	if err := program.Invoke(context.Background(), []string{"app", "--host", "unix:///tmp/docker.sock"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "unix:///tmp/docker.sock" {
		t.Fatalf("got %q", got)
	}

	out.Reset()
	errout.Reset()
	if err := program.Invoke(context.Background(), []string{"app", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errout.String(), "--host") {
		t.Fatalf("help missing global option:\n%s", errout.String())
	}
}

func TestMuxRejectsFlagOptionNameCollision(t *testing.T) {
	mux := NewMux("app")
	mux.Flag("name", "", false, "flag")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	mux.Option("name", "", "", "option")
}

func TestProgramRejectsFlagOptionNameCollision(t *testing.T) {
	program := &Program{Runner: NewMux("app")}
	program.Flag("name", "", false, "flag")

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	program.Option("name", "", "", "option")
}

func TestMuxFlagAndOption(t *testing.T) {
	mux := NewMux("app")
	mux.Flag("verbose", "v", false, "verbose")
	mux.Option("host", "", "", "daemon socket")
	mux.Handle("run", "", RunnerFunc(func(out *Output, call *Call) error {
		_, err := fmt.Fprintf(out.Stdout, "%s|%t", call.GlobalOptions.Get("host"), call.GlobalFlags["verbose"])
		return err
	}))
	var out bytes.Buffer
	call := NewCall(context.Background(), "app", []string{"--host", "localhost", "--verbose", "run"})
	if err := mux.RunCLI(&Output{Stdout: &out, Stderr: io.Discard}, call); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "localhost|true" {
		t.Fatalf("got %q", got)
	}
}

func TestMountedMuxScopedFlags(t *testing.T) {
	root := NewMux("app")
	root.Flag("verbose", "v", false, "verbose")
	sub := NewMux("repo")
	sub.Flag("dry-run", "n", false, "dry run")
	sub.Handle("init", "", RunnerFunc(func(out *Output, call *Call) error {
		_, err := fmt.Fprintf(out.Stdout, "verbose=%t dry-run=%t",
			call.GlobalFlags["verbose"], call.GlobalFlags["dry-run"])
		return err
	}))
	root.Handle("repo", "Repository commands", sub)

	var out bytes.Buffer
	program := &Program{Runner: root, Stdout: &out, Stderr: io.Discard}
	if err := program.Invoke(context.Background(), []string{"app", "--verbose", "repo", "--dry-run", "init"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "verbose=true dry-run=true" {
		t.Fatalf("got %q", got)
	}
}

func TestMountedMuxHelpShowsAllAncestorFlags(t *testing.T) {
	root := NewMux("app")
	root.Flag("verbose", "v", false, "verbose")
	sub := NewMux("repo")
	sub.Option("repository", "r", ".", "repo path")
	var gotHelp *Help
	sub.Handle("init", "Initialize", RunnerFunc(func(*Output, *Call) error { return nil }))
	root.Handle("repo", "Repository commands", sub)

	program := &Program{
		Runner:   root,
		Stdout:   io.Discard,
		Stderr:   io.Discard,
		HelpFunc: func(_ io.Writer, help *Help) error { gotHelp = help; return nil },
	}
	if err := program.Invoke(context.Background(), []string{"app", "repo", "init", "--help"}); err != nil {
		t.Fatal(err)
	}
	if gotHelp == nil {
		t.Fatal("expected help to be rendered")
	}
	// Should include both root mux flag and repo mux option.
	if len(gotHelp.GlobalFlags) != 1 || gotHelp.GlobalFlags[0].Name != "verbose" {
		t.Fatalf("got global flags %#v", gotHelp.GlobalFlags)
	}
	if len(gotHelp.GlobalOptions) != 1 || gotHelp.GlobalOptions[0].Name != "repository" {
		t.Fatalf("got global options %#v", gotHelp.GlobalOptions)
	}
}

func TestNegateFlagsCommand(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		NegateFlags: true,
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "verbose=%t force=%t",
				call.Flags["verbose"], call.Flags["force"])
			return err
		},
	}
	cmd.Flag("verbose", "v", false, "verbose")
	cmd.Flag("force", "f", false, "force")
	mux.Handle("run", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run", "--verbose", "--no-force"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "verbose=true force=false" {
		t.Fatalf("got %q", got)
	}
}

func TestNegateFlagsTrueDefault(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		NegateFlags: true,
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "accept-dns=%t", call.Flags["accept-dns"])
			return err
		},
	}
	cmd.Flag("accept-dns", "", true, "accept DNS")
	mux.Handle("up", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"up", "--no-accept-dns"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "accept-dns=false" {
		t.Fatalf("got %q", got)
	}
}

func TestNegateFlagsBidirectional(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		NegateFlags: true,
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "no-cache=%t", call.Flags["no-cache"])
			return err
		},
	}
	cmd.Flag("no-cache", "", true, "disable cache")
	mux.Handle("build", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"build", "--cache"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "no-cache=false" {
		t.Fatalf("got %q", got)
	}
}

func TestNegateFlagsUnknownErrors(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		NegateFlags: true,
		Run:         func(*Output, *Call) error { return nil },
	}
	cmd.Flag("verbose", "", false, "verbose")
	mux.Handle("run", "", cmd)

	err := runMux(context.Background(), mux, io.Discard, io.Discard, []string{"run", "--no-unknown"})
	if err == nil {
		t.Fatal("expected error for --no-unknown")
	}
}

func TestNegateFlagsDisabledByDefault(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(*Output, *Call) error { return nil },
	}
	cmd.Flag("verbose", "", false, "verbose")
	mux.Handle("run", "", cmd)

	err := runMux(context.Background(), mux, io.Discard, io.Discard, []string{"run", "--no-verbose"})
	if err == nil {
		t.Fatal("expected error for --no-verbose when NegateFlags is false")
	}
}

func TestNegateFlagsMux(t *testing.T) {
	mux := NewMux("app")
	mux.NegateFlags = true
	mux.Flag("verbose", "v", false, "verbose")
	mux.Handle("run", "", RunnerFunc(func(out *Output, call *Call) error {
		_, err := fmt.Fprintf(out.Stdout, "verbose=%t", call.GlobalFlags["verbose"])
		return err
	}))

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"--no-verbose", "run"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "verbose=false" {
		t.Fatalf("got %q", got)
	}
}

func TestNegateFlagsHelpShowsBothForms(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		NegateFlags: true,
		Run:         func(*Output, *Call) error { return nil },
	}
	cmd.Flag("accept-dns", "", true, "accept DNS")
	cmd.Flag("no-cache", "", true, "disable cache")
	mux.Handle("up", "Connect", cmd)

	var errout bytes.Buffer
	if err := runMux(context.Background(), mux, io.Discard, &errout, []string{"up", "--help"}); err != nil {
		t.Fatalf("got err=%v", err)
	}
	help := errout.String()
	if !strings.Contains(help, "--no-accept-dns") {
		t.Fatalf("help missing --no-accept-dns:\n%s", help)
	}
	if !strings.Contains(help, "--cache") {
		t.Fatalf("help missing --cache (negation of --no-cache):\n%s", help)
	}
}

func TestRepeatedOptionAccumulates(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "last=%s all=%v",
				call.Options.Get("tag"), call.Options.Values("tag"))
			return err
		},
	}
	cmd.Option("tag", "t", "", "tags")
	mux.Handle("run", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run", "--tag", "a", "--tag", "b", "-t", "c"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "last=c all=[a b c]" {
		t.Fatalf("got %q", got)
	}
}

func TestRepeatedOptionReplacesDefault(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "get=%s values=%v",
				call.Options.Get("host"), call.Options.Values("host"))
			return err
		},
	}
	cmd.Option("host", "", "localhost", "target host")
	mux.Handle("run", "", cmd)

	t.Run("default", func(t *testing.T) {
		var out bytes.Buffer
		if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run"}); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != "get=localhost values=[localhost]" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("override", func(t *testing.T) {
		var out bytes.Buffer
		if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run", "--host", "example.com"}); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != "get=example.com values=[example.com]" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestApplyDefaultsIdempotent(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			// ApplyDefaults already called by Command.RunCLI.
			// Calling again should be a no-op.
			call.ApplyDefaults()
			_, err := fmt.Fprintf(out.Stdout, "host=%s verbose=%t",
				call.Options.Get("host"), call.Flags.Get("verbose"))
			return err
		},
	}
	cmd.Option("host", "", "localhost", "target host")
	cmd.Flag("verbose", "", false, "verbose")
	mux.Handle("run", "", cmd)

	var out bytes.Buffer
	if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "host=localhost verbose=false" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyDefaultsSparseBeforeComplete(t *testing.T) {
	mux := NewMux("app")
	cmd := &Command{
		Run: func(out *Output, call *Call) error {
			_, err := fmt.Fprintf(out.Stdout, "host=%s verbose=%t",
				call.Options.Get("host"), call.Flags.Get("verbose"))
			return err
		},
	}
	cmd.Option("host", "", "localhost", "target host")
	cmd.Flag("verbose", "", false, "verbose")
	mux.Handle("run", "", cmd)

	t.Run("defaults applied", func(t *testing.T) {
		var out bytes.Buffer
		if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run"}); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != "host=localhost verbose=false" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("cli values override defaults", func(t *testing.T) {
		var out bytes.Buffer
		if err := runMux(context.Background(), mux, &out, io.Discard, []string{"run", "--host", "example.com", "--verbose"}); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != "host=example.com verbose=true" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestEnvMap(t *testing.T) {
	env := map[string]string{
		"APP_HOST": "env-host",
		"VERBOSE":  "1",
	}
	middleware := EnvMapRunner(map[string]string{
		"host":    "APP_HOST",
		"verbose": "VERBOSE",
	}, NewLookupEnv(env))

	t.Run("fills missing options", func(t *testing.T) {
		call := NewCall(context.Background(), "app run", nil)
		call.flagDefaults = map[string]bool{"verbose": false}
		call.optionDefaults = map[string]string{"host": "localhost"}

		inner := RunnerFunc(func(out *Output, call *Call) error {
			call.ApplyDefaults()
			_, err := fmt.Fprintf(out.Stdout, "host=%s verbose=%t",
				call.Options.Get("host"), call.Flags.Get("verbose"))
			return err
		})

		var out bytes.Buffer
		err := middleware(inner).RunCLI(&Output{Stdout: &out, Stderr: io.Discard}, call)
		if err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != "host=env-host verbose=true" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("cli overrides env", func(t *testing.T) {
		call := NewCall(context.Background(), "app run", nil)
		call.Options.Set("host", "cli-host")
		call.flagDefaults = map[string]bool{"verbose": false}
		call.optionDefaults = map[string]string{"host": "localhost"}

		inner := RunnerFunc(func(out *Output, call *Call) error {
			call.ApplyDefaults()
			_, err := fmt.Fprintf(out.Stdout, "host=%s verbose=%t",
				call.Options.Get("host"), call.Flags.Get("verbose"))
			return err
		})

		var out bytes.Buffer
		err := middleware(inner).RunCLI(&Output{Stdout: &out, Stderr: io.Discard}, call)
		if err != nil {
			t.Fatal(err)
		}
		if got := out.String(); got != "host=cli-host verbose=true" {
			t.Fatalf("got %q", got)
		}
	})
}
