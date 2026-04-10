package cli

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// A Mux is a command multiplexer. It matches argv tokens against
// registered command names and dispatches to the corresponding [Runner].
type Mux struct {
	// Name is the mux identifier used in help output and command paths.
	Name string

	// NegateFlags enables --no- prefix negation for mux-level boolean
	// flags. When true, --no-flagname sets a flag to false. If a flag
	// is declared with a "no-" prefix, the bare form (--flagname)
	// also sets it to false. See [Command.NegateFlags].
	NegateFlags bool

	root node
	flags       flagSpecs
	options     optionSpecs
}

type ancestorHelp struct {
	flags   []HelpFlag
	options []HelpOption
}

// node is an internal trie node for command routing.
type node struct {
	segment         string
	parent          *node
	runner          Runner
	usageText       string
	descriptionText string
	children        map[string]*node
}

type nodeChild struct {
	name        string
	path        string
	usage       string
	description string
	node        *node
}

func (n *node) getOrCreate(name string) *node {
	if n.children == nil {
		n.children = map[string]*node{}
	}
	child, ok := n.children[name]
	if !ok {
		child = &node{segment: name, parent: n}
		n.children[name] = child
	}
	return child
}

func (n *node) childInfos(prefix string) []nodeChild {
	names := slices.Sorted(maps.Keys(n.children))
	children := make([]nodeChild, 0, len(names))
	for _, name := range names {
		child := n.children[name]
		path := name
		if prefix != "" {
			path = prefix + " " + name
		}
		children = append(children, nodeChild{
			name:        name,
			path:        path,
			usage:       child.usage(),
			description: child.description(),
			node:        child,
		})
	}
	return children
}

func (n *node) usageCommands(prefix string) []HelpCommand {
	children := n.childInfos(prefix)
	cmds := make([]HelpCommand, 0, len(children))
	for _, child := range children {
		cmds = append(cmds, HelpCommand{
			Name:        child.path,
			Usage:       child.usage,
			Description: child.description,
		})
	}
	return cmds
}

func (n *node) path() string {
	if n == nil {
		return ""
	}
	var segments []string
	for cur := n; cur != nil; cur = cur.parent {
		if cur.segment != "" {
			segments = append(segments, cur.segment)
		}
	}
	slices.Reverse(segments)
	return strings.Join(segments, " ")
}

func (n *node) usage() string       { return n.usageText }
func (n *node) description() string { return n.descriptionText }

func validateRunner(runner Runner) {
	if runner == nil {
		panic("cli: nil command runner")
	}
	if command, ok := runner.(*Command); ok && command != nil {
		if command.Run == nil {
			panic("cli: nil command handler")
		}
	}
}

func (n *node) setCommand(runner Runner, usage, description string) {
	validateRunner(runner)
	n.runner = runner
	n.usageText = usage
	n.descriptionText = description
}

func (n *node) commandRunner() Runner { return n.runner }
func (n *node) hasRunner() bool       { return n.runner != nil }

// NewMux returns a new [Mux] with the given program name.
// It panics if name is empty.
func NewMux(name string) *Mux {
	if name == "" {
		panic("cli: empty mux name")
	}
	return &Mux{Name: name}
}

// Flag declares a mux-level boolean flag that is parsed before subcommand
// routing. Parsed values accumulate in [Call.GlobalFlags].
//
// short is an optional one-character short form (e.g. "v" for -v).
// An empty string means the flag has no short form.
// It panics on duplicate or reserved names.
func (m *Mux) Flag(name, short string, value bool, usage string) {
	if m.options.hasName(name) {
		panic("cli: duplicate mux input " + `"` + name + `"`)
	}
	if short != "" && m.options.hasShort(short) {
		panic("cli: duplicate mux short input " + `"` + short + `"`)
	}
	m.flags.add(name, short, value, usage)
}

// Option declares a mux-level named value option that is parsed before
// subcommand routing. Parsed values accumulate in [Call.GlobalOptions].
//
// short is an optional one-character short form (e.g. "c" for -c).
// An empty string means the option has no short form.
// It panics on duplicate or reserved names.
func (m *Mux) Option(name, short, value, usage string) {
	if m.flags.hasName(name) {
		panic("cli: duplicate mux input " + `"` + name + `"`)
	}
	if short != "" && m.flags.hasShort(short) {
		panic("cli: duplicate mux short input " + `"` + short + `"`)
	}
	m.options.add(name, short, value, usage)
}

func (m *Mux) muxInputs() (*flagSpecs, *optionSpecs) {
	fs := &m.flags
	os := &m.options
	if len(fs.specs) == 0 {
		fs = nil
	}
	if len(os.specs) == 0 {
		os = nil
	}
	return fs, os
}

// Handle registers runner for the given command pattern with a short usage
// summary shown in help output.
//
// Pattern segments are split on whitespace. Multi-segment patterns create
// nested command paths (e.g. "repo init"). If runner is a [*Mux], it is
// mounted as a sub-mux at pattern. It panics on conflicting registrations
// or a nil runner.
func (m *Mux) Handle(pattern string, usage string, runner Runner) {
	var description string
	if cmd, ok := runner.(*Command); ok && cmd != nil {
		description = cmd.Description
	}
	if sub, ok := runner.(*Mux); ok {
		m.mount(pattern, usage, sub)
		return
	}
	n := &m.root
	for _, seg := range strings.Fields(pattern) {
		n = n.getOrCreate(seg)
	}
	if n.hasRunner() {
		panic("cli: command conflict at " + `"` + pattern + `"`)
	}
	n.setCommand(runner, usage, description)
}

// HandleFunc registers fn as the handler for pattern.
// It is a shorthand for Handle(pattern, usage, [RunnerFunc](fn)).
func (m *Mux) HandleFunc(pattern string, usage string, fn func(*Output, *Call) error) {
	m.Handle(pattern, usage, RunnerFunc(fn))
}

func (m *Mux) mount(prefix string, usage string, sub *Mux) {
	if sub == nil {
		panic("cli: nil mount mux")
	}
	n := &m.root
	for _, seg := range strings.Fields(prefix) {
		n = n.getOrCreate(seg)
	}
	if n.hasRunner() {
		panic("cli: mount conflict at " + `"` + prefix + `"`)
	}
	n.setCommand(sub, usage, "")
}

// RunCLI routes call.Argv through the command trie and dispatches to the
// matched handler. It panics if call is nil.
func (m *Mux) RunCLI(out *Output, call *Call) error {
	if call == nil {
		panic("cli: nil call")
	}
	return m.runWithPath(out, call, m.Name, "", "", nil, DefaultHelpFunc)
}

func (m *Mux) runWithPath(out *Output, call *Call, fullPath string, usage string, description string, ancestors *ancestorHelp, helpRenderer HelpFunc) error {
	if ancestors == nil {
		ancestors = &ancestorHelp{}
	}
	helpRenderer = resolveHelpFunc(helpRenderer)
	muxFlags, muxOptions := m.muxInputs()
	accFlags, accOptions := accumulateHelp(ancestors, muxFlags, muxOptions, m.NegateFlags)

	parsed, err := parseInput(muxFlags, muxOptions, slices.Clone(call.Argv), m.NegateFlags)
	if err != nil {
		if errors.Is(err, errFlagHelp) {
			d := &dispatch{
				out:           out,
				call:          call,
				mux:           m,
				path:          fullPath,
				usage:         usage,
				description:   description,
				globalFlags:   accFlags,
				globalOptions: accOptions,
				helpFunc:      helpRenderer,
			}
			return d.renderHelp(&m.root, nil, nil, nil, true)
		}
		return err
	}

	// Apply mux-level defaults eagerly — no middleware window at this level.
	muxFlags.applyDefaults(parsed.flags)
	muxOptions.applyDefaults(parsed.options)

	newCall := buildRoutingCall(call, parsed)

	d := &dispatch{
		out:           out,
		call:          newCall,
		mux:           m,
		path:          fullPath,
		usage:         usage,
		description:   description,
		globalFlags:   accFlags,
		globalOptions: accOptions,
		helpFunc:      helpRenderer,
	}

	return d.route(&m.root, &tokenCursor{tokens: parsed.args})
}

// accumulateHelp merges ancestor help entries with the current mux's
// flag and option entries.
func accumulateHelp(ancestors *ancestorHelp, fs *flagSpecs, os *optionSpecs, negateFlags bool) ([]HelpFlag, []HelpOption) {
	flags := append(append([]HelpFlag(nil), ancestors.flags...), fs.helpEntriesNegatable(negateFlags)...)
	options := append(append([]HelpOption(nil), ancestors.options...), os.helpEntries()...)
	return flags, options
}

// buildRoutingCall creates a new Call by merging the parent call's
// global flags/options with newly parsed mux-level input.
func buildRoutingCall(call *Call, parsed *parsedInput) *Call {
	globalFlags := make(FlagSet)
	maps.Insert(globalFlags, maps.All(call.GlobalFlags))
	maps.Insert(globalFlags, maps.All(parsed.flags))
	globalOptions := make(OptionSet)
	maps.Insert(globalOptions, maps.All(call.GlobalOptions))
	maps.Insert(globalOptions, maps.All(parsed.options))

	return &Call{
		ctx:           call.Context(),
		Pattern:       call.Pattern,
		Argv:          slices.Clone(parsed.args),
		Stdin:         call.Stdin,
		Env:           call.Env,
		GlobalFlags:   globalFlags,
		GlobalOptions: globalOptions,
		Flags:         maps.Clone(call.Flags),
		Options:       maps.Clone(call.Options),
		Args:          maps.Clone(call.Args),
		Rest:          slices.Clone(call.Rest),
	}
}

// dispatch bundles the state threaded through command routing.
type dispatch struct {
	out           *Output
	call          *Call
	mux           *Mux
	path          string
	usage         string
	description   string
	globalFlags   []HelpFlag
	globalOptions []HelpOption
	helpFunc      HelpFunc
}

func (d *dispatch) route(n *node, cur *tokenCursor) error {
	if !cur.done() {
		if child, ok := n.children[cur.peek()]; ok {
			token := cur.next()
			d.path = joinedPath(d.path, token)
			d.usage = ""
			d.description = ""
			return d.route(child, cur)
		}
	}

	if !n.hasRunner() {
		if !cur.done() && len(n.children) > 0 {
			fmt.Fprintf(d.out.Stderr, "unknown command %q\n\n", cur.peek())
		}
		return d.renderHelp(n, nil, nil, nil, false)
	}

	h := n.commandRunner()

	if sub, ok := h.(*Mux); ok {
		mountCall := &Call{
			ctx:           d.call.Context(),
			Pattern:       d.call.Pattern,
			Argv:          slices.Clone(cur.rest()),
			Stdin:         d.call.Stdin,
			Env:           d.call.Env,
			GlobalFlags:   maps.Clone(d.call.GlobalFlags),
			GlobalOptions: maps.Clone(d.call.GlobalOptions),
			Flags:         maps.Clone(d.call.Flags),
			Options:       maps.Clone(d.call.Options),
			Args:          maps.Clone(d.call.Args),
			Rest:          slices.Clone(d.call.Rest),
		}
		return sub.runWithPath(d.out, mountCall, d.path, n.usage(), n.description(), &ancestorHelp{
			flags:   d.globalFlags,
			options: d.globalOptions,
		}, d.helpFunc)
	}

	fs, os, as := commandInputs(h)
	captureRest := commandCaptureRest(h)
	negateFlags := commandNegateFlags(h)
	return d.runCommand(n, h, fs, os, as, captureRest, negateFlags, cur.rest())
}

func (d *dispatch) runCommand(n *node, h Runner, fs *flagSpecs, os *optionSpecs, as *argSpecs, captureRest bool, negateFlags bool, rest []string) error {
	parsed, err := parseInput(fs, os, rest, negateFlags)
	if err != nil {
		if errors.Is(err, errFlagHelp) {
			return d.renderHelp(n, fs, os, as, true, negateFlags)
		}
		return err
	}

	argState := ArgSet{}
	var restState []string
	if as != nil {
		argState, restState, err = as.parse(parsed.args, captureRest)
		if err != nil {
			return err
		}
	} else if captureRest {
		restState = slices.Clone(parsed.args)
	}

	runCall := &Call{
		ctx:           d.call.Context(),
		Pattern:       d.path,
		Argv:          slices.Clone(rest),
		Stdin:         d.call.Stdin,
		Env:           d.call.Env,
		GlobalFlags:   maps.Clone(d.call.GlobalFlags),
		GlobalOptions: maps.Clone(d.call.GlobalOptions),
		Flags:         parsed.flags,
		Options:       parsed.options,
		Args:          argState,
		Rest:          restState,
		flagDefaults:   fs.defaultMap(),
		optionDefaults: os.defaultMap(),
	}
	return h.RunCLI(d.out, runCall)
}

func (d *dispatch) renderHelp(n *node, fs *flagSpecs, os *optionSpecs, as *argSpecs, explicit bool, negateFlags ...bool) error {
	fullPath := n.path()
	if d.path != "" {
		fullPath = d.path
	}
	usageText := n.usage()
	desc := n.description()
	if n == &d.mux.root && (d.usage != "" || d.description != "") {
		usageText, desc = d.usage, d.description
	}

	name := n.segment
	if name == "" {
		name = lastPathSegment(fullPath)
	}
	help := Help{
		Name:          name,
		FullPath:      fullPath,
		Usage:         usageText,
		Description:   desc,
		GlobalFlags:   d.globalFlags,
		GlobalOptions: d.globalOptions,
		Commands:      n.usageCommands(""),
		Flags:         fs.helpEntriesNegatable(len(negateFlags) > 0 && negateFlags[0]),
		Options:       os.helpEntries(),
	}
	if as != nil {
		help.Arguments = as.helpArguments()
	}
	if err := d.helpFunc(d.out.Stderr, &help); err != nil {
		return err
	}
	if explicit {
		return nil
	}
	return ErrHelp
}

func joinedPath(base string, suffix string) string {
	if suffix == "" {
		return base
	}
	if base == "" {
		return suffix
	}
	return strings.TrimSpace(base + " " + suffix)
}

func lastPathSegment(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Fields(path)
	return parts[len(parts)-1]
}

func commandInputs(runner Runner) (*flagSpecs, *optionSpecs, *argSpecs) {
	if command, ok := runner.(*Command); ok && command != nil {
		return command.inputs()
	}
	return nil, nil, nil
}

func commandCaptureRest(runner Runner) bool {
	if command, ok := runner.(*Command); ok {
		return command != nil && command.CaptureRest
	}
	return false
}

func commandNegateFlags(runner Runner) bool {
	if command, ok := runner.(*Command); ok {
		return command != nil && command.NegateFlags
	}
	return false
}

func resolveHelpFunc(help HelpFunc) HelpFunc {
	if help != nil {
		return help
	}
	return DefaultHelpFunc
}

