package az

import (
	"fmt"
	"strconv"
	"strings"
)

// scope is the registration surface shared by App and Command. Generic
// methods are promoted through embedding, so one method set serves every
// level of the command tree.
type scope struct {
	registry map[string]*flagEntry // every key, aliases included
	flags    []*flagEntry          // registration order
	args     []*argEntry
	commands map[string]*Command
	cmds     []*Command // registration order
}

func newScope() scope {
	return scope{
		registry: make(map[string]*flagEntry),
		commands: make(map[string]*Command),
	}
}

// App is the top-level CLI application.
type App struct {
	scope
	name         string
	help         string
	version      string
	matchers     []Matcher
	interspersed bool
	versionFlag  bool // the "version" flag is registered
}

// Command is a subcommand with its own flags, arguments, and subcommands.
// Flags of enclosing scopes stay in effect after the command name; a flag
// registered on the command shadows an outer flag with the same key, and a
// fully shadowed outer flag is no longer checked by Required.
type Command struct {
	scope
	name   string
	help   string
	hidden bool
}

// NewApp creates a CLI application. Without matchers, GNU-style flags are
// used: --long, --long=value, -s, -svalue, -abc, and "--" to end flags.
//
// The app registers a "help" flag, for which ParseArgs returns ErrHelp.
// Add a short form with app.Alias("h", "help").
func NewApp(name, help string, matchers ...Matcher) *App {
	if len(matchers) == 0 {
		matchers = []Matcher{
			EndOfFlagsMatcher{Token: "--"},
			GNUShortMatcher{},
			GNULongMatcher{},
		}
	}
	a := &App{scope: newScope(), name: name, help: help, matchers: matchers, interspersed: true}
	a.addFlag(newFlagEntry("help", "Show help.", Bool, requestFlag(ErrHelp)))
	return a
}

// Version sets the version string and registers a "version" flag, for which
// ParseArgs returns ErrVersion. It panics if the app already has a
// different "version" flag.
func (a *App) Version(v string) *App {
	if !a.versionFlag {
		a.addFlag(newFlagEntry("version", "Show version.", Bool, requestFlag(ErrVersion)))
		a.versionFlag = true
	}
	a.version = v
	return a
}

// Interspersed controls whether flags may follow positional arguments.
// Default true (GNU style). Interspersed(false) gives POSIX behavior:
// the first positional argument ends flag parsing.
func (a *App) Interspersed(v bool) *App {
	a.interspersed = v
	return a
}

// requestFlag returns a flag setter that reports err when the flag is true.
func requestFlag(err error) func(bool) error {
	return func(v bool) error {
		if v {
			return err
		}
		return nil
	}
}

// Hidden omits the command from help and completion.
func (c *Command) Hidden() *Command {
	c.hidden = true
	return c
}

// =========================================================================
// Registration: generic methods on scope, shared by App and Command.
// =========================================================================

// Flag registers a flag of type T, inferred from conv.
//
// If conv implements ZeroArgConv, the flag takes no value token (--verbose).
// If it implements MultiValueConv, the flag consumes the declared number of
// value tokens (--point 1 2 3). Otherwise it consumes one.
func (s *scope) Flag[T any](name, help string, conv Conv[T]) *FlagClause[T] {
	ref := &Ref[T]{}
	e := newFlagEntry(name, help, conv, func(v T) error {
		ref.value, ref.isSet = v, true
		return nil
	})
	s.addFlag(e)
	return &FlagClause[T]{entry: e, ref: ref, format: conv.ToString}
}

// FlagRepeat registers a flag that appends each occurrence to a []T.
func (s *scope) FlagRepeat[T any](name, help string, conv Conv[T]) *FlagClause[[]T] {
	ref := &Ref[[]T]{}
	e := newFlagEntry(name, help, conv, func(v T) error {
		if !ref.isSet {
			ref.value = nil // the first occurrence replaces the default
		}
		ref.value, ref.isSet = append(ref.value, v), true
		return nil
	})
	s.addFlag(e)
	return &FlagClause[[]T]{entry: e, ref: ref, format: joinFormat(conv)}
}

// FlagCounter registers a flag that counts occurrences: -vvv → 3.
// An explicit value adds to the count: --verbose=2.
func (s *scope) FlagCounter(name, help string) *FlagClause[int] {
	ref := &Ref[int]{}
	add := func(n int) error {
		if !ref.isSet {
			ref.value = 0 // the first occurrence replaces the default
		}
		ref.value, ref.isSet = ref.value+n, true
		return nil
	}
	e := &flagEntry{
		name:    name,
		help:    help,
		setTrue: func() error { return add(1) },
		setValue: func(raw string) error {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return convErr{err}
			}
			return add(n)
		},
	}
	s.addFlag(e)
	return &FlagClause[int]{entry: e, ref: ref, format: strconv.Itoa}
}

// OnFlag registers a flag that calls fn on every occurrence, in command-line
// order. Errors from fn stop parsing and are returned by ParseArgs as is.
func (s *scope) OnFlag[T any](name, help string, conv Conv[T], fn func(T) error) {
	s.addFlag(newFlagEntry(name, help, conv, fn))
}

// OnFlagBool registers a zero-arg flag that calls fn on every occurrence.
// An explicit false (--flag=false) does not call fn.
func (s *scope) OnFlagBool(name, help string, fn func() error) {
	s.OnFlag(name, help, Bool, func(v bool) error {
		if v {
			return fn()
		}
		return nil
	})
}

// Alias registers an additional key for a flag registered in the same
// scope, typically a short form: app.Alias("v", "verbose").
func (s *scope) Alias(alias, target string) {
	e, ok := s.registry[target]
	if !ok {
		panic(fmt.Sprintf("az: Alias(%q, %q): no flag %q in this scope", alias, target, target))
	}
	s.addKey(alias, e)
	e.keys = append(e.keys, alias)
}

// Arg registers a positional argument of type T, inferred from conv.
func (s *scope) Arg[T any](name, help string, conv Conv[T]) *ArgClause[T] {
	ref := &Ref[T]{}
	e := &argEntry{
		name: name,
		help: help,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return err
			}
			ref.value, ref.isSet = v, true
			return nil
		},
		complete: conv.Complete,
	}
	s.addArg(e)
	return &ArgClause[T]{entry: e, ref: ref, format: conv.ToString}
}

// ArgRepeat registers a positional argument that collects all remaining
// positional arguments into a []T. It must be the scope's last argument.
func (s *scope) ArgRepeat[T any](name, help string, conv Conv[T]) *ArgClause[[]T] {
	ref := &Ref[[]T]{}
	e := &argEntry{
		name:   name,
		help:   help,
		repeat: true,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return err
			}
			if !ref.isSet {
				ref.value = nil // the first value replaces the default
			}
			ref.value, ref.isSet = append(ref.value, v), true
			return nil
		},
		complete: conv.Complete,
	}
	s.addArg(e)
	return &ArgClause[[]T]{entry: e, ref: ref, format: joinFormat(conv)}
}

// Cmd registers a subcommand. Works at any depth, since Command embeds
// scope too.
func (s *scope) Cmd(name, help string) *Command {
	if name == "" {
		panic("az: empty command name")
	}
	if _, dup := s.commands[name]; dup {
		panic(fmt.Sprintf("az: command %q registered twice", name))
	}
	cmd := &Command{scope: newScope(), name: name, help: help}
	s.commands[name] = cmd
	s.cmds = append(s.cmds, cmd)
	return cmd
}

func (s *scope) addFlag(e *flagEntry) {
	s.addKey(e.name, e)
	e.keys = []string{e.name}
	s.flags = append(s.flags, e)
}

func (s *scope) addKey(key string, e *flagEntry) {
	if key == "" {
		panic("az: empty flag name")
	}
	if _, dup := s.registry[key]; dup {
		panic(fmt.Sprintf("az: flag %q registered twice", key))
	}
	s.registry[key] = e
}

func (s *scope) addArg(e *argEntry) {
	if n := len(s.args); n > 0 && s.args[n-1].repeat {
		panic(fmt.Sprintf("az: argument %q registered after repeated argument %q", e.name, s.args[n-1].name))
	}
	s.args = append(s.args, e)
}

// newFlagEntry builds a flag that converts with conv and hands each value
// to set. Arity comes from the interfaces conv implements.
func newFlagEntry[T any](name, help string, conv Conv[T], set func(T) error) *flagEntry {
	e := &flagEntry{
		name:  name,
		help:  help,
		nargs: 1,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return convErr{err}
			}
			return set(v)
		},
		complete: conv.Complete,
	}
	switch c := conv.(type) {
	case ZeroArgConv[T]:
		e.nargs = 0
		implied := c.ImpliedValue()
		e.setTrue = func() error { return set(implied) }
	case MultiValueConv[T]:
		e.nargs = c.NArgs()
		if e.nargs < 2 && e.nargs != VariadicNArgs {
			panic(fmt.Sprintf("az: flag %q: NArgs() = %d, want >= 2 or VariadicNArgs", name, e.nargs))
		}
		e.setValues = func(parts []string) error {
			v, err := c.FromStrings(parts)
			if err != nil {
				return convErr{err}
			}
			return set(v)
		}
		if tc, ok := conv.(TerminatedConv); ok {
			e.stopToken = tc.StopToken()
		}
	}
	return e
}

// joinFormat formats a []T for help output.
func joinFormat[T any](conv Conv[T]) func([]T) string {
	return func(vs []T) string {
		strs := make([]string, len(vs))
		for i, v := range vs {
			strs[i] = conv.ToString(v)
		}
		return strings.Join(strs, ",")
	}
}
