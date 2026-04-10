package cli

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
)

// A HelpFunc renders help output to w for a resolved command path.
type HelpFunc func(w io.Writer, help *Help) error

// A HelpFlag describes a boolean flag in rendered help.
type HelpFlag struct {
	Name    string
	Short   string
	Usage   string
	Default bool

	// Negatable indicates that [DefaultHelpFunc] should render both
	// --flag and --no-flag forms. It is set automatically when the
	// declaring [Mux] or [Command] has NegateFlags enabled.
	Negatable bool
}

// A HelpOption describes a value option in rendered help.
type HelpOption struct {
	Name    string
	Short   string
	Usage   string
	Default string
}

// A HelpArg describes a positional argument in rendered help.
type HelpArg struct {
	Name  string
	Usage string
}

// A HelpCommand describes an immediate child command in rendered help.
type HelpCommand struct {
	Name        string
	Usage       string
	Description string
}

// Help holds the data passed to a [HelpFunc] when rendering help output.
type Help struct {
	// Name is the final segment of the command path.
	Name string
	// FullPath is the complete command path (e.g. "app repo init").
	FullPath string
	// Usage is a short one-line summary.
	Usage string
	// Description is longer free-form help text.
	Description string

	// GlobalFlags lists program-level boolean flags in scope.
	GlobalFlags []HelpFlag
	// GlobalOptions lists program-level value options in scope.
	GlobalOptions []HelpOption

	// Commands lists the immediate child commands.
	Commands []HelpCommand
	// Arguments lists positional arguments accepted by this command.
	Arguments []HelpArg
	// Flags lists command-level boolean flags.
	Flags []HelpFlag
	// Options lists command-level value options.
	Options []HelpOption
}

// DefaultHelpFunc is the built-in [HelpFunc] used when no override is set.
// It renders a tabular summary to w.
func DefaultHelpFunc(w io.Writer, help *Help) error {
	if help == nil {
		panic("cli: nil help")
	}
	slices.SortFunc(help.Commands, func(a, b HelpCommand) int {
		return cmp.Compare(a.Name, b.Name)
	})
	if help.Usage != "" {
		if _, err := fmt.Fprintf(w, "%s - %s\n", help.FullPath, help.Usage); err != nil {
			return err
		}
	}
	if help.Description != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", help.Description); err != nil {
			return err
		}
	}

	if _, err := io.WriteString(w, "\nUsage:\n"); err != nil {
		return err
	}

	line := "  " + help.FullPath
	if len(help.Commands) > 0 {
		line += " [command]"
	}
	if len(help.GlobalFlags) > 0 || len(help.GlobalOptions) > 0 || len(help.Flags) > 0 || len(help.Options) > 0 {
		line += " [options]"
	}
	if len(help.Arguments) > 0 {
		line += " [arguments]"
	}
	line += "\n"
	if _, err := io.WriteString(w, line); err != nil {
		return err
	}

	if err := renderFlagSection(w, "Global Flags", help.GlobalFlags); err != nil {
		return err
	}
	if err := renderOptionSection(w, "Global Options", help.GlobalOptions); err != nil {
		return err
	}
	if err := renderFlagSection(w, "Flags", help.Flags); err != nil {
		return err
	}
	if err := renderOptionSection(w, "Options", help.Options); err != nil {
		return err
	}

	if len(help.Arguments) > 0 {
		if _, err := io.WriteString(w, "\nArguments:\n"); err != nil {
			return err
		}
		rows := make([]helpRow, 0, len(help.Arguments))
		for _, argument := range help.Arguments {
			rows = append(rows, helpRow{Name: argument.Name, Usage: argument.Usage})
		}
		if err := renderHelpTable(w, rows); err != nil {
			return err
		}
	}

	if len(help.Commands) > 0 {
		if _, err := io.WriteString(w, "\nCommands:\n"); err != nil {
			return err
		}
		rows := make([]helpRow, 0, len(help.Commands))
		for _, cmd := range help.Commands {
			rows = append(rows, helpRow{Name: cmd.Name, Usage: cmd.Usage})
		}
		if err := renderHelpTable(w, rows); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Use %q for more information.\n", help.FullPath+" [command] --help"); err != nil {
			return err
		}
	}
	return nil
}

func renderFlagSection(w io.Writer, title string, entries []HelpFlag) error {
	if len(entries) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\n%s:\n", title); err != nil {
		return err
	}
	rows := make([]helpRow, 0, len(entries))
	for _, e := range entries {
		usage := e.Usage
		usage += fmt.Sprintf(" (default: %t)", e.Default)
		rows = append(rows, helpRow{Name: formatInputName(e.Name, e.Short, e.Negatable), Usage: usage})
	}
	return renderHelpTable(w, rows)
}

func renderOptionSection(w io.Writer, title string, entries []HelpOption) error {
	if len(entries) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "\n%s:\n", title); err != nil {
		return err
	}
	rows := make([]helpRow, 0, len(entries))
	for _, e := range entries {
		usage := e.Usage
		if e.Default != "" {
			usage += fmt.Sprintf(" (default: %s)", e.Default)
		}
		rows = append(rows, helpRow{Name: formatInputName(e.Name, e.Short, false), Usage: usage})
	}
	return renderHelpTable(w, rows)
}

type helpRow struct {
	Name  string
	Usage string
}

func renderHelpTable(w io.Writer, rows []helpRow) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		lines := strings.Split(row.Usage, "\n")
		if len(lines) == 0 {
			lines = []string{""}
		}
		if _, err := fmt.Fprintf(tw, "  %s\t%s\n", row.Name, lines[0]); err != nil {
			return err
		}
		for _, line := range lines[1:] {
			if _, err := fmt.Fprintf(tw, "  \t%s\n", line); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
}

func formatInputName(name, short string, negatable bool) string {
	var b strings.Builder
	if short != "" {
		b.WriteString("-")
		b.WriteString(short)
		b.WriteString(", ")
	}
	b.WriteString("--")
	b.WriteString(name)
	if negatable {
		b.WriteString(", --")
		if strings.HasPrefix(name, "no-") {
			b.WriteString(name[3:])
		} else {
			b.WriteString("no-")
			b.WriteString(name)
		}
	}
	return b.String()
}
