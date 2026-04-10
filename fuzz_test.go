package cli

import (
	"testing"
)

func FuzzParseInput(f *testing.F) {
	// Seed with representative inputs.
	f.Add("--verbose")
	f.Add("-v")
	f.Add("-vf")
	f.Add("--host localhost")
	f.Add("--host=localhost")
	f.Add("--verbose=true")
	f.Add("--verbose=false")
	f.Add("--")
	f.Add("-- foo bar")
	f.Add("-h")
	f.Add("--help")
	f.Add("")
	f.Add("-")
	f.Add("---")
	f.Add("--=")
	f.Add("--verbose --host localhost positional")
	f.Add("-v -r /tmp/repo")
	f.Add("-vr /tmp/repo")
	f.Add("--host=")
	f.Add("--host= --verbose")
	f.Add("positional1 positional2")
	f.Add("-- --verbose")
	f.Add("-x")
	f.Add("--unknown")
	f.Add("--verbose=notabool")

	flags := &flagSpecs{}
	flags.add("verbose", "v", false, "verbose output")
	flags.add("force", "f", false, "force operation")

	options := &optionSpecs{}
	options.add("host", "r", "default", "target host")
	options.add("output", "o", "", "output path")

	f.Fuzz(func(t *testing.T, input string) {
		args := splitTestArgs(input)
		// Must not panic.
		parseInput(flags, options, args, false)
	})
}

func FuzzParseInputNilSets(f *testing.F) {
	f.Add("--verbose")
	f.Add("-v")
	f.Add("")
	f.Add("-")
	f.Add("--")
	f.Add("foo bar baz")
	f.Add("--help")

	f.Fuzz(func(t *testing.T, input string) {
		args := splitTestArgs(input)
		// Must not panic with nil flag/option sets.
		parseInput(nil, nil, args, false)
	})
}

func FuzzArgSetParse(f *testing.F) {
	f.Add("hello")
	f.Add("hello world")
	f.Add("")
	f.Add("-- hello")
	f.Add("a b c d e")
	f.Add("--")

	as := &argSpecs{}
	as.add("name", "the name")
	as.add("target", "the target")

	f.Fuzz(func(t *testing.T, input string) {
		args := splitTestArgs(input)
		// Must not panic.
		as.parse(args, false)
		as.parse(args, true)
	})
}

// splitTestArgs splits a string into args on whitespace, matching how
// shells tokenize. An empty string produces a nil slice.
func splitTestArgs(s string) []string {
	if s == "" {
		return nil
	}
	var args []string
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if start >= 0 {
				args = append(args, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		args = append(args, s[start:])
	}
	return args
}
