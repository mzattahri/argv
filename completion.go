package argv

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// A TokenWriter provides methods for writing tab-separated completion entries.
type TokenWriter struct {
	io.Writer
}

// WriteToken writes a completion candidate with an optional description.
// It returns the number of bytes written and any write error.
func (w *TokenWriter) WriteToken(value, desc string) (int, error) {
	if desc != "" {
		return fmt.Fprintf(w.Writer, "%s\t%s\n", value, desc)
	}
	return fmt.Fprintln(w.Writer, value)
}

// A Completer writes tab completions for a partial command line to w.
// It is the escape hatch for custom completion: a [Runner] that wants
// dynamic option-value suggestions or context-dependent candidates
// implements CompleteCLI. A Runner that does not implement Completer
// still gets tab completion — candidates are derived from its Help
// metadata by [CompletionCommand].
//
// completed holds the tokens before the cursor that have already been
// fully typed. partial is the token currently being typed (it may be
// empty).
//
// The idiomatic implementation embeds [*Command] for routing-level
// behavior and overrides CompleteCLI to emit dynamic values for
// specific option positions, falling back to the embedded Command's
// Help for structural candidates:
//
//	type deployCmd struct {
//		*argv.Command
//		hosts []string
//	}
//
//	func (d *deployCmd) CompleteCLI(w *argv.TokenWriter, completed []string, partial string) error {
//		// Dynamic values at --host <TAB>.
//		if len(completed) > 0 && completed[len(completed)-1] == "--host" {
//			for _, h := range d.hosts {
//				if strings.HasPrefix(h, partial) {
//					w.WriteToken(h, "")
//				}
//			}
//			return nil
//		}
//		// Delegate everything else to the embedded Command's Help —
//		// [*Help] also implements Completer and handles flag tokens,
//		// option names, and argument hints.
//		var help argv.Help
//		d.HelpCLI(&help)
//		return help.CompleteCLI(w, completed, partial)
//	}
//
// The pattern suits leaf commands. For subtree-shaped runners,
// implement Completer on leaves; [CompletionCommand] descends via
// [Walker] and dispatches to the matching node.
//
// See [ExampleCompleter] for a full runnable example.
type Completer interface {
	CompleteCLI(w *TokenWriter, completed []string, partial string) error
}

// CompleterFunc adapts a plain function to the [Completer] interface.
type CompleterFunc func(w *TokenWriter, completed []string, partial string) error

// CompleteCLI calls f(w, completed, partial).
func (f CompleterFunc) CompleteCLI(w *TokenWriter, completed []string, partial string) error {
	return f(w, completed, partial)
}

// CompletionCommand returns a [*Command] that outputs tab completions
// for root. The concrete return type lets callers inspect or augment
// the command (set Description, add middleware, etc.) before mounting
// it.
//
// Shell integration scripts invoke this command on each TAB press,
// passing the current tokens (shell tokens) as positional arguments
// after "--":
//
//	myapp complete -- repo init --f
//
// Completion derives from root's capabilities:
//
//   - If root implements [Completer], it handles completion directly.
//   - Otherwise, if root implements [Walker], the command walks the
//     subtree to the deepest node matching the typed positional
//     tokens. That node's [Completer] (if any) runs; otherwise
//     candidates come from the node's [*Help.CompleteCLI].
//   - Otherwise, if root implements [Helper], the command materializes
//     a [*Help] from it and delegates to [*Help.CompleteCLI]. No
//     subcommand traversal.
//
// It panics if root is nil.
func CompletionCommand(root Runner) *Command {
	if root == nil {
		panic("argv: nil runner")
	}
	return &Command{
		Description: "Emit tab-completion candidates for the current shell tokens.",
		CaptureRest: true,
		Run: func(out *Output, call *Call) error {
			args := call.Rest
			var completed []string
			partial := ""
			if len(args) > 0 {
				completed = args[:len(args)-1]
				partial = args[len(args)-1]
			}
			tw := &TokenWriter{Writer: out.Stdout}
			return walkComplete(root, tw, completed, partial)
		},
	}
}

// walkComplete dispatches completion for root by detecting the
// capabilities it implements.
func walkComplete(root Runner, w *TokenWriter, completed []string, partial string) error {
	if c, ok := root.(Completer); ok {
		return c.CompleteCLI(w, completed, partial)
	}
	if walker, ok := root.(Walker); ok {
		return walkerComplete(walker, w, completed, partial)
	}
	if h, ok := root.(Helper); ok {
		var help Help
		h.HelpCLI(&help)
		return help.CompleteCLI(w, completed, partial)
	}
	return nil
}

// walkerComplete descends walker's subtree by the positional tokens
// in completed, respecting flag-skip semantics at each level. At the
// deepest match it delegates to a node-level [Completer] if present
// or emits candidates from that node's [Help].
func walkerComplete(walker Walker, w *TokenWriter, completed []string, partial string) error {
	if slices.Contains(completed, "--") {
		return nil
	}

	type entry struct {
		help   *Help
		runner Runner
	}
	byPath := map[string]*entry{}
	var rootPath string
	empty := true
	for help, runner := range walker.WalkCLI("", nil) {
		if empty {
			rootPath = help.FullPath
			empty = false
		}
		byPath[help.FullPath] = &entry{help: help, runner: runner}
	}
	if empty {
		return nil
	}

	current := byPath[rootPath]
	currentPath := rootPath
	lastSubcommandEnd := 0 // completed index immediately after the last matched subcommand token
	i := 0
	for i < len(completed) {
		tok := completed[i]
		if tok == "--" {
			break
		}
		if strings.HasPrefix(tok, "-") {
			if isValueOption(tok, current.help.Options) {
				i++ // swallow value
				if i >= len(completed) {
					break
				}
			}
			i++
			continue
		}
		candidatePath := joinedPath(currentPath, tok)
		child, ok := byPath[candidatePath]
		if !ok {
			break
		}
		current = child
		currentPath = candidatePath
		i++
		lastSubcommandEnd = i
	}

	if c, ok := current.runner.(Completer); ok {
		return c.CompleteCLI(w, completed, partial)
	}
	return current.help.CompleteCLI(w, completed[lastSubcommandEnd:], partial)
}

// writeFlagEntries emits flag and option tokens derived from Help.
// Flag entries marked Negatable emit their --no- variant.
func writeFlagEntries(w *TokenWriter, flags []HelpFlag, options []HelpOption, partial string) error {
	for _, f := range flags {
		if err := writeEntry(w, "--"+f.Name, f.Usage, partial); err != nil {
			return err
		}
		if f.Short != "" {
			if err := writeEntry(w, "-"+f.Short, f.Usage, partial); err != nil {
				return err
			}
		}
		if f.Negatable {
			var negName string
			if strings.HasPrefix(f.Name, "no-") {
				negName = f.Name[3:]
			} else {
				negName = "no-" + f.Name
			}
			if err := writeEntry(w, "--"+negName, f.Usage, partial); err != nil {
				return err
			}
		}
	}
	for _, o := range options {
		if err := writeEntry(w, "--"+o.Name, o.Usage, partial); err != nil {
			return err
		}
		if o.Short != "" {
			if err := writeEntry(w, "-"+o.Short, o.Usage, partial); err != nil {
				return err
			}
		}
	}
	if err := writeEntry(w, "--help", "Show help", partial); err != nil {
		return err
	}
	return writeEntry(w, "-h", "Show help", partial)
}

// writeSubcommands emits subcommand name candidates from Help.Commands.
func writeSubcommands(w *TokenWriter, commands []HelpCommand, partial string) error {
	for _, c := range commands {
		if err := writeEntry(w, c.Name, c.Usage, partial); err != nil {
			return err
		}
	}
	return nil
}

// isValueOption reports whether word matches the long or short form
// of any value-taking option in the given Help entries.
func isValueOption(word string, options []HelpOption) bool {
	for _, o := range options {
		if word == "--"+o.Name || (o.Short != "" && word == "-"+o.Short) {
			return true
		}
	}
	return false
}

// isPartialOptionValue reports whether partial is a --option= prefix
// awaiting a value.
func isPartialOptionValue(partial string, flags []HelpFlag, options []HelpOption) bool {
	_, _, ok := splitOptionValuePartial(partial, flags, options)
	return ok
}

// splitOptionValuePartial reports whether partial is a "--name=value"
// prefix awaiting completion of the value portion. When it returns
// true, name is the option name and value is the current partial
// value. It returns false for boolean flags and for names that are
// not registered as value-taking options.
func splitOptionValuePartial(partial string, flags []HelpFlag, options []HelpOption) (name, value string, ok bool) {
	if !strings.HasPrefix(partial, "--") {
		return "", "", false
	}
	n, v, hasEquals := strings.Cut(partial[2:], "=")
	if !hasEquals {
		return "", "", false
	}
	for _, f := range flags {
		if f.Name == n {
			return "", "", false
		}
	}
	for _, o := range options {
		if o.Name == n {
			return n, v, true
		}
	}
	return "", "", false
}

// writeArgHint emits the next expected positional argument name as a
// completion hint, if any remain.
func writeArgHint(w *TokenWriter, args []HelpArg, completed []string, options []HelpOption) error {
	if len(args) == 0 {
		return nil
	}
	// Count positional tokens already consumed (skip flags and option values).
	pos := 0
	skipNext := false
	for _, tok := range completed {
		if skipNext {
			skipNext = false
			continue
		}
		if tok == "--" {
			break
		}
		if strings.HasPrefix(tok, "-") {
			if isValueOption(tok, options) {
				skipNext = true
			}
			continue
		}
		pos++
	}
	if pos < len(args) {
		a := args[pos]
		_, err := w.WriteToken(a.Name, a.Usage)
		return err
	}
	return nil
}

func writeEntry(w *TokenWriter, value, desc, partial string) error {
	if strings.HasPrefix(value, partial) {
		_, err := w.WriteToken(value, desc)
		return err
	}
	return nil
}
