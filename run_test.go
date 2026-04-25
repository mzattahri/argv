package argv

import (
	"bytes"
	"context"
	"io"
	"iter"
	"slices"
	"strings"
	"testing"
)

func TestInvokeDefaultsNilTTYAndStdin(t *testing.T) {
	mux := &Mux{}
	mux.Handle("noop", "", RunnerFunc(func(out *Output, call *Call) error {
		if out.Stdout == nil {
			t.Fatal("expected non-nil stdout from default Output")
		}
		if out.Stderr == nil {
			t.Fatal("expected non-nil stderr from default Output")
		}
		if call.Stdin == nil {
			t.Fatal("expected non-nil stdin from default")
		}
		return nil
	}))

	program := &Program{}
	if err := program.Invoke(context.Background(), mux, []string{"app", "noop"}); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeSkipsArgv0(t *testing.T) {
	mux := &Mux{}
	cmd := &Command{Run: func(out *Output, call *Call) error {
		_, err := out.Stdout.Write([]byte(call.Args.Get("msg")))
		return err
	}}
	cmd.Arg("msg", "message")
	mux.Handle("echo", "", cmd)

	var out bytes.Buffer
	program := &Program{Stdout: &out, Stderr: &bytes.Buffer{}}
	err := program.Invoke(context.Background(), mux, []string{"app", "echo", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestInvokeExplicitHelpReturnsSuccess(t *testing.T) {
	mux := &Mux{}
	mux.Handle("echo", "Echo output", RunnerFunc(func(out *Output, call *Call) error { return nil }))

	var stdout bytes.Buffer
	program := &Program{Stdout: &stdout, Stderr: io.Discard}
	err := program.Invoke(context.Background(), mux, []string{"app", "--help"})
	if err != nil {
		t.Fatalf("got err=%v, want nil", err)
	}
	if got := stdout.String(); got == "" {
		t.Fatal("expected help output")
	}
}

func TestInvokeWithPlainRunner(t *testing.T) {
	runner := RunnerFunc(func(out *Output, call *Call) error {
		_, err := io.WriteString(out.Stdout, "plain")
		return err
	})

	var stdout bytes.Buffer
	program := &Program{Stdout: &stdout, Stderr: &bytes.Buffer{}}
	if err := program.Invoke(context.Background(), runner, []string{"app"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "plain" {
		t.Fatalf("got %q, want %q", got, "plain")
	}
}

func TestInvokeEmptyArgsPanics(t *testing.T) {
	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("expected panic")
		}
		if s, ok := got.(string); !ok || !strings.Contains(s, "args") {
			t.Fatalf("got panic %v", got)
		}
	}()
	mux := &Mux{}
	mux.Handle("noop", "Do nothing", RunnerFunc(func(*Output, *Call) error { return nil }))
	program := &Program{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	_ = program.Invoke(context.Background(), mux, nil)
}

func TestWalkPlainRunner(t *testing.T) {
	runner := RunnerFunc(func(out *Output, call *Call) error { return nil })
	program := &Program{Usage: "A test app"}

	var paths []string
	for help := range program.Walk("app", runner) {
		paths = append(paths, help.FullPath)
		if help.Name != "app" {
			t.Fatalf("got name %q", help.Name)
		}
		if help.Usage != "A test app" {
			t.Fatalf("got usage %q", help.Usage)
		}
	}
	if len(paths) != 1 || paths[0] != "app" {
		t.Fatalf("got paths %v", paths)
	}
}

func TestWalkMux(t *testing.T) {
	mux := &Mux{}
	mux.Flag("verbose", "v", false, "verbose")

	deployCmd := &Command{
		Description: "Deploy the app",
		Run:         func(*Output, *Call) error { return nil },
	}
	deployCmd.Flag("force", "f", false, "force")
	deployCmd.Arg("target", "deploy target")
	mux.Handle("deploy", "Deploy", deployCmd)

	mux.Handle("version", "Print version", RunnerFunc(func(*Output, *Call) error { return nil }))

	program := &Program{Usage: "A CLI tool"}

	var paths []string
	helpByPath := map[string]*Help{}
	for help := range program.Walk("app", mux) {
		paths = append(paths, help.FullPath)
		helpByPath[help.FullPath] = help
	}

	wantPaths := []string{"app", "app deploy", "app version"}
	if !slices.Equal(paths, wantPaths) {
		t.Fatalf("got paths %v, want %v", paths, wantPaths)
	}

	// Root has usage and commands.
	root := helpByPath["app"]
	if root.Usage != "A CLI tool" {
		t.Fatalf("got root usage %q", root.Usage)
	}
	if len(root.Commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(root.Commands))
	}

	// Deploy has global flag, local flag, and argument.
	deploy := helpByPath["app deploy"]
	if deploy.Description != "Deploy the app" {
		t.Fatalf("got description %q", deploy.Description)
	}
	globalFlags := slices.Collect(deploy.GlobalFlags())
	localFlags := slices.Collect(deploy.LocalFlags())
	if len(globalFlags) != 1 || globalFlags[0].Name != "verbose" {
		t.Fatalf("got global flags %v", globalFlags)
	}
	if len(localFlags) != 1 || localFlags[0].Name != "force" {
		t.Fatalf("got local flags %v", localFlags)
	}
	if len(deploy.Arguments) != 1 || deploy.Arguments[0].Name != "<target>" {
		t.Fatalf("got arguments %v", deploy.Arguments)
	}
}

func TestWalkMountedMux(t *testing.T) {
	root := &Mux{}
	root.Flag("verbose", "v", false, "verbose")

	sub := &Mux{}
	sub.Option("path", "p", ".", "repo path")
	sub.Handle("init", "Initialize", RunnerFunc(func(*Output, *Call) error { return nil }))
	sub.Handle("clone", "Clone", RunnerFunc(func(*Output, *Call) error { return nil }))
	root.Handle("repo", "Repository operations", sub)

	program := &Program{}

	var paths []string
	helpByPath := map[string]*Help{}
	for help := range program.Walk("app", root) {
		paths = append(paths, help.FullPath)
		helpByPath[help.FullPath] = help
	}

	wantPaths := []string{"app", "app repo", "app repo clone", "app repo init"}
	if !slices.Equal(paths, wantPaths) {
		t.Fatalf("got paths %v, want %v", paths, wantPaths)
	}

	// Sub-mux commands inherit root's global flags.
	init := helpByPath["app repo init"]
	globalFlags := slices.Collect(init.GlobalFlags())
	globalOptions := slices.Collect(init.GlobalOptions())
	if len(globalFlags) != 1 || globalFlags[0].Name != "verbose" {
		t.Fatalf("got global flags %v", globalFlags)
	}
	if len(globalOptions) != 1 || globalOptions[0].Name != "path" {
		t.Fatalf("got global options %v", globalOptions)
	}
}

func TestWalkMultiSegmentPattern(t *testing.T) {
	mux := &Mux{}
	mux.Handle("repo init", "Initialize a repository", RunnerFunc(func(*Output, *Call) error { return nil }))
	mux.Handle("repo clone", "Clone a repository", RunnerFunc(func(*Output, *Call) error { return nil }))

	program := &Program{}

	var paths []string
	for help := range program.Walk("app", mux) {
		paths = append(paths, help.FullPath)
	}

	wantPaths := []string{"app", "app repo", "app repo clone", "app repo init"}
	if !slices.Equal(paths, wantPaths) {
		t.Fatalf("got paths %v, want %v", paths, wantPaths)
	}
}

// customWalker is a minimal third-party Walker that exposes its own
// static command tree to Program.Walk.
type customWalker struct {
	name string
}

func (c *customWalker) RunCLI(*Output, *Call) error { return nil }

func (c *customWalker) WalkCLI(path string, base *Help) iter.Seq2[*Help, Runner] {
	return func(yield func(*Help, Runner) bool) {
		if base == nil {
			base = &Help{}
		}
		if !yield(&Help{
			Name:        c.name,
			FullPath:    path,
			Usage:       base.Usage,
			Description: base.Description,
			Flags:       slices.Clone(base.Flags),
			Options:     slices.Clone(base.Options),
		}, c) {
			return
		}
		yield(&Help{
			Name:     "static-child",
			FullPath: path + " static-child",
			Usage:    "A synthetic child",
			Flags:    slices.Clone(base.Flags),
			Options:  slices.Clone(base.Options),
		}, c)
	}
}

func TestWalkCustomWalker(t *testing.T) {
	// Top-level external Walker: Program.Walk dispatches via the interface.
	program := &Program{Usage: "Custom CLI"}
	cw := &customWalker{name: "app"}

	var paths []string
	for help := range program.Walk("app", cw) {
		paths = append(paths, help.FullPath)
	}
	want := []string{"app", "app static-child"}
	if !slices.Equal(paths, want) {
		t.Fatalf("got %v, want %v", paths, want)
	}
}

func TestWalkCustomWalkerInMuxTree(t *testing.T) {
	// External Walker registered inside a Mux: walkChildren dispatches
	// via the Walker interface, and ancestor globals reach the child.
	mux := &Mux{}
	mux.Flag("verbose", "v", false, "verbose")
	mux.Handle("plug", "External subtree", &customWalker{name: "plug"})

	var paths []string
	helpByPath := map[string]*Help{}
	for help := range (&Program{}).Walk("app", mux) {
		paths = append(paths, help.FullPath)
		helpByPath[help.FullPath] = help
	}

	want := []string{"app", "app plug", "app plug static-child"}
	if !slices.Equal(paths, want) {
		t.Fatalf("got %v, want %v", paths, want)
	}

	// The external walker received the mux's verbose flag as a global.
	plug := helpByPath["app plug"]
	globals := slices.Collect(plug.GlobalFlags())
	if len(globals) != 1 || globals[0].Name != "verbose" {
		t.Fatalf("got globals %v", globals)
	}
}

func TestWalkEarlyTermination(t *testing.T) {
	mux := &Mux{}
	mux.Handle("a", "First", RunnerFunc(func(*Output, *Call) error { return nil }))
	mux.Handle("b", "Second", RunnerFunc(func(*Output, *Call) error { return nil }))
	mux.Handle("c", "Third", RunnerFunc(func(*Output, *Call) error { return nil }))

	program := &Program{}
	count := 0
	for range program.Walk("app", mux) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("got %d iterations, want 2", count)
	}
}
