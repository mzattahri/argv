package clitest_test

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mzattahri/cli"
	"github.com/mzattahri/cli/clitest"
)

func ExampleNewCall() {
	mux := cli.NewMux("app")
	mux.Handle("echo", "Echo arguments", cli.RunnerFunc(func(out *cli.Output, call *cli.Call) error {
		_, err := fmt.Fprint(out.Stdout, strings.Join(call.Argv, ","))
		return err
	}))

	recorder := clitest.NewRecorder()
	call := clitest.NewCall("echo a b", nil)
	_ = mux.RunCLI(recorder.Output(), call)

	fmt.Printf("stdout=%q stderr=%q", recorder.Stdout.String(), recorder.Stderr.String())
	// Output: stdout="a,b" stderr=""
}

func ExampleNewCall_stdin() {
	mux := cli.NewMux("app")
	mux.Handle("cat", "Copy stdin to stdout", cli.RunnerFunc(func(out *cli.Output, call *cli.Call) error {
		_, err := io.Copy(out.Stdout, call.Stdin)
		return err
	}))

	recorder := clitest.NewRecorder()
	call := clitest.NewCall("cat", []byte("piped input"))
	_ = mux.RunCLI(recorder.Output(), call)

	fmt.Printf("stdout=%q stderr=%q", recorder.Stdout.String(), recorder.Stderr.String())
	// Output: stdout="piped input" stderr=""
}

func ExampleNewCall_directInputs() {
	type authKey struct{}

	call := clitest.NewCall("whoami", nil)
	call = call.WithContext(context.WithValue(context.Background(), authKey{}, "alice"))
	call.GlobalOptions = cli.OptionSet{"host": {"unix:///tmp/docker.sock"}}
	call.Flags = map[string]bool{"verbose": true}
	call.Args = map[string]string{
		"name":  "alice",
		"roles": "admin operator",
	}

	user := call.Context().Value(authKey{})
	host := call.GlobalOptions.Get("host")
	verbose := call.Flags["verbose"]

	fmt.Printf("user=%v host=%s verbose=%t name=%s roles=%s", user, host, verbose, call.Args["name"], call.Args["roles"])
	// Output: user=alice host=unix:///tmp/docker.sock verbose=true name=alice roles=admin operator
}
