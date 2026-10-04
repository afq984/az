// Package az explores whether v8's closed dispatch core can absorb two
// "advanced" demands declaratively, without reopening a v7-style handler
// layer:
//
//  1. Multi-token flags: --point 1 2 3 (fixed arity), --ports 80 443
//     (variadic, stops at the next recognized flag), and find-style
//     -exec grep -l foo ; (variadic, stops at a terminator token).
//     Arity stays metadata on the flag (MultiValueConv.NArgs), not a
//     runtime wantMore protocol on the converter: Convs remain stateless
//     shared values, and completion replay keeps working because token
//     consumption is knowable without running conversions. The buffering
//     state ("wantMore") lives in the dispatch loop as pendingTokens,
//     bounded by the declared arity.
//
//  2. Dispatch policy: Interspersed(false) makes the first positional end
//     flag parsing (POSIX getopt / pflag SetInterspersed behavior).
//     "--" already works via EndOfFlagsMatcher, including mid-greedy.
//
// Registration uses v9's generic-methods surface (go.dev/issue/77273):
// Flag/Arg/Cmd are methods on a scope embedded in both App and Command,
// so one method set serves every level of the command tree. Generic
// methods shipped in go1.27rc2 with no GOEXPERIMENT flag, so this
// package lives in its own module pinned to that toolchain (like v9).
//
// Dispatch internals are verbatim v8 apart from the two changes above.
package az

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// =========================================================================
// App and Command
// =========================================================================

// scope is the shared registration surface for App and Command.
// Generic methods are promoted through embedding, so CmdFlag/CmdArg/
// CmdAlias/SubCmd from v8 disappear entirely.
type scope struct {
	registry map[string]*flagEntry
	args     []*argEntry
	commands map[string]*Command
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
}

// Command is a subcommand with its own flags, arguments, and subcommands.
type Command struct {
	scope
	name   string
	help   string
	hidden bool
}

// NewApp creates a new CLI application. If no matchers are provided,
// GNU-style flags (--flag, -f, --) are used by default.
func NewApp(name, help string, matchers ...Matcher) *App {
	if len(matchers) == 0 {
		matchers = []Matcher{
			EndOfFlagsMatcher{Token: "--"},
			GNULongMatcher{},
			GNUShortMatcher{},
		}
	}
	return &App{scope: newScope(), name: name, help: help, matchers: matchers, interspersed: true}
}

func (a *App) Version(v string) *App { a.version = v; return a }

// Interspersed controls whether flags may follow positional arguments.
// Default true (GNU style). Interspersed(false) gives POSIX behavior:
// the first positional argument ends flag parsing.
func (a *App) Interspersed(v bool) *App { a.interspersed = v; return a }

// =========================================================================
// Registration: generic methods on scope, shared by App and Command.
// =========================================================================

// Flag registers a flag of type T. T is inferred from the Conv argument.
func (s *scope) Flag[T any](name, help string, conv Conv[T]) *FlagClause[T] {
	ref := &Ref[T]{}
	nargs := 1
	stopToken := ""
	var setTrue func()
	var setValues func([]string) error
	if za, ok := conv.(ZeroArgConv[T]); ok {
		nargs = 0
		implied := za.ImpliedValue()
		setTrue = func() { ref.value = implied; ref.isSet = true }
	} else if mv, ok := conv.(MultiValueConv[T]); ok {
		nargs = mv.NArgs()
		setValues = func(parts []string) error {
			v, err := mv.FromStrings(parts)
			if err != nil {
				return err
			}
			ref.value = v
			ref.isSet = true
			return nil
		}
		if tc, ok := conv.(TerminatedConv); ok {
			stopToken = tc.StopToken()
		}
	}
	entry := &flagEntry{
		name:      name,
		help:      help,
		nargs:     nargs,
		stopToken: stopToken,
		setTrue:   setTrue,
		setValues: setValues,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return err
			}
			ref.value = v
			ref.isSet = true
			return nil
		},
		complete: func(partial string) []string { return conv.Complete(partial) },
	}
	s.registry[name] = entry
	return &FlagClause[T]{entry: entry, ref: ref}
}

// FlagRepeat registers a flag that appends each occurrence to a []T.
func (s *scope) FlagRepeat[T any](name, help string, conv Conv[T]) *FlagClause[[]T] {
	ref := &Ref[[]T]{}
	entry := &flagEntry{
		name:  name,
		help:  help,
		nargs: 1,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return err
			}
			ref.value = append(ref.value, v)
			ref.isSet = true
			return nil
		},
		complete: func(partial string) []string { return conv.Complete(partial) },
	}
	s.registry[name] = entry
	return &FlagClause[[]T]{entry: entry, ref: ref}
}

// FlagCounter registers a flag that counts occurrences: -vvv → 3.
func (s *scope) FlagCounter(name, help string) *FlagClause[int] {
	ref := &Ref[int]{}
	entry := &flagEntry{
		name:  name,
		help:  help,
		nargs: 0,
		addCount: func() {
			ref.value++
			ref.isSet = true
		},
		setValue: func(raw string) error {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return err
			}
			ref.value += n
			ref.isSet = true
			return nil
		},
	}
	s.registry[name] = entry
	return &FlagClause[int]{entry: entry, ref: ref}
}

// OnFlag registers a flag that fires a callback on every occurrence.
func (s *scope) OnFlag[T any](name, help string, conv Conv[T], fn func(T) error) {
	nargs := 1
	stopToken := ""
	var setValues func([]string) error
	if _, ok := conv.(ZeroArgConv[T]); ok {
		nargs = 0
	} else if mv, ok := conv.(MultiValueConv[T]); ok {
		nargs = mv.NArgs()
		setValues = func(parts []string) error {
			v, err := mv.FromStrings(parts)
			if err != nil {
				return err
			}
			return fn(v)
		}
		if tc, ok := conv.(TerminatedConv); ok {
			stopToken = tc.StopToken()
		}
	}
	entry := &flagEntry{
		name:      name,
		help:      help,
		nargs:     nargs,
		stopToken: stopToken,
		setValues: setValues,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return err
			}
			return fn(v)
		},
		complete: func(partial string) []string { return conv.Complete(partial) },
	}
	s.registry[name] = entry
}

// OnFlagBool registers a zero-arg flag that fires a callback on each occurrence.
func (s *scope) OnFlagBool(name, help string, fn func() error) {
	entry := &flagEntry{
		name:  name,
		help:  help,
		nargs: 0,
		setTrue: func() {
			fn()
		},
		setValue: func(raw string) error {
			v, err := strconv.ParseBool(raw)
			if err != nil {
				return err
			}
			if v {
				return fn()
			}
			return nil
		},
	}
	s.registry[name] = entry
}

// Alias registers an additional key for an existing flag.
func (s *scope) Alias(alias, target string) {
	if entry, ok := s.registry[target]; ok {
		s.registry[alias] = entry
	}
}

// Arg registers a positional argument of type T.
func (s *scope) Arg[T any](name, help string, conv Conv[T]) *ArgClause[T] {
	ref := &Ref[T]{}
	entry := &argEntry{
		name: name,
		help: help,
		setValue: func(raw string) error {
			v, err := conv.FromString(raw)
			if err != nil {
				return err
			}
			ref.value = v
			ref.isSet = true
			return nil
		},
		complete: func(partial string) []string { return conv.Complete(partial) },
	}
	s.args = append(s.args, entry)
	return &ArgClause[T]{entry: entry, ref: ref}
}

// Cmd registers a subcommand. Works at any depth — Command embeds scope too.
func (s *scope) Cmd(name, help string) *Command {
	cmd := &Command{scope: newScope(), name: name, help: help}
	s.commands[name] = cmd
	return cmd
}

// =========================================================================
// Parsing
// =========================================================================

func (a *App) ParseArgs(osArgs []string) (string, error) {
	ds := &dispatchState{
		matchers:     a.matchers,
		interspersed: a.interspersed,
		registry:     a.registry,
		commands:     a.commands,
		args:         a.args,
	}
	return runDispatch(osArgs, ds)
}

func (a *App) MustParse(args []string) string {
	cmd, err := a.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return cmd
}

type dispatchState struct {
	matchers        []Matcher
	interspersed    bool
	registry        map[string]*flagEntry
	commands        map[string]*Command
	args            []*argEntry
	posIdx          int
	forcePositional bool
	pendingEntry    *flagEntry
	pendingName     string
	pendingTokens   []string // collected values for a multi-token pending flag
}

func (ds *dispatchState) registryLookup(key string) (bool, int) {
	entry, ok := ds.registry[key]
	if !ok {
		return false, 0
	}
	return true, entry.nargs
}

func (ds *dispatchState) clearPending() {
	ds.pendingEntry = nil
	ds.pendingName = ""
	ds.pendingTokens = nil
}

func (ds *dispatchState) flushPending() error {
	err := ds.pendingEntry.setValues(ds.pendingTokens)
	if err != nil {
		err = fmt.Errorf("invalid value for %s: %w", ds.pendingName, err)
	}
	ds.clearPending()
	return err
}

func runDispatch(osArgs []string, ds *dispatchState) (string, error) {
	for i := 0; i < len(osArgs); i++ {
		raw := osArgs[i]

		// 1. Pending flag wants value tokens. How many is declared
		// metadata (nargs / stopToken), so consumption needs no
		// converter execution and replays identically for completion.
		if ds.pendingEntry != nil {
			e := ds.pendingEntry
			switch {
			case e.nargs == 1:
				if err := e.setValue(raw); err != nil {
					return "", fmt.Errorf("invalid value for %s: %w", ds.pendingName, err)
				}
				ds.clearPending()
				continue

			case e.nargs > 1:
				// Fixed arity consumes unconditionally — "--delta -1 5 -3"
				// works because the next 3 tokens are values by declaration.
				ds.pendingTokens = append(ds.pendingTokens, raw)
				if len(ds.pendingTokens) == e.nargs {
					if err := ds.flushPending(); err != nil {
						return "", err
					}
				}
				continue

			case e.stopToken != "":
				// Terminator-delimited: consume raw tokens, even ones that
				// look like our flags (find -exec grep -l foo ;).
				if raw != e.stopToken {
					ds.pendingTokens = append(ds.pendingTokens, raw)
					continue
				}
				if err := ds.flushPending(); err != nil {
					return "", err
				}
				continue

			default:
				// Greedy variadic: stop at the next recognized flag.
				if ds.tryMatch(raw) == nil {
					ds.pendingTokens = append(ds.pendingTokens, raw)
					continue
				}
				if len(ds.pendingTokens) == 0 {
					return "", fmt.Errorf("flag %s requires a value", ds.pendingName)
				}
				if err := ds.flushPending(); err != nil {
					return "", err
				}
				// Fall through: raw is processed as a flag below.
			}
		}

		// 2. Try matchers (unless force-positional).
		if !ds.forcePositional {
			match := ds.tryMatch(raw)
			if match != nil {
				if match.Key == "" {
					ds.forcePositional = true
					continue
				}
				if err := ds.handleMatch(match); err != nil {
					return "", err
				}
				continue
			}
		}

		// 3. Not a flag — try command, then positional.
		if !ds.forcePositional {
			if cmd, ok := ds.commands[raw]; ok {
				childDS := &dispatchState{
					matchers:     ds.matchers,
					interspersed: ds.interspersed,
					registry:     mergeRegistries(ds.registry, cmd.registry),
					commands:     cmd.commands,
					args:         cmd.args,
				}
				subcmd, err := runDispatch(osArgs[i+1:], childDS)
				if err != nil {
					return "", err
				}
				if subcmd != "" {
					return raw + " " + subcmd, nil
				}
				return raw, nil
			}
		}

		// 4. Positional arg.
		if ds.posIdx >= len(ds.args) {
			return "", fmt.Errorf("unexpected argument: %s", raw)
		}
		arg := ds.args[ds.posIdx]
		if err := arg.setValue(raw); err != nil {
			return "", fmt.Errorf("invalid value for %q: %w", arg.name, err)
		}
		ds.posIdx++
		if !ds.interspersed {
			ds.forcePositional = true
		}
	}

	if ds.pendingEntry != nil {
		e := ds.pendingEntry
		switch {
		case e.nargs > 1:
			return "", fmt.Errorf("flag %s requires %d values, got %d", ds.pendingName, e.nargs, len(ds.pendingTokens))
		case e.nargs == 1:
			return "", fmt.Errorf("flag %s requires a value", ds.pendingName)
		case e.stopToken != "":
			return "", fmt.Errorf("flag %s: missing terminator %q", ds.pendingName, e.stopToken)
		default:
			if len(ds.pendingTokens) == 0 {
				return "", fmt.Errorf("flag %s requires a value", ds.pendingName)
			}
			if err := ds.flushPending(); err != nil {
				return "", err
			}
		}
	}
	return "", nil
}

func (ds *dispatchState) tryMatch(raw string) *Match {
	for _, m := range ds.matchers {
		if match := m.TryMatch(raw, ds.registryLookup); match != nil {
			return match
		}
	}
	return nil
}

func (ds *dispatchState) handleMatch(match *Match) error {
	entry, ok := ds.registry[match.Key]
	if !ok {
		return fmt.Errorf("unknown flag: %s", match.Key)
	}

	if entry.nargs == 0 {
		if match.Attached != nil {
			// Explicit value: --verbose=false, /V:true
			if err := entry.setValue(*match.Attached); err != nil {
				return fmt.Errorf("invalid value for %s: %w", match.Key, err)
			}
		} else if entry.addCount != nil {
			entry.addCount()
		} else if entry.setTrue != nil {
			entry.setTrue()
		}
	} else {
		if match.Attached != nil {
			// Attached values go through FromString for any arity:
			// --point=1,2,3 uses the converter's single-string syntax.
			if err := entry.setValue(*match.Attached); err != nil {
				return fmt.Errorf("invalid value for %s: %w", match.Key, err)
			}
		} else {
			ds.pendingEntry = entry
			ds.pendingName = match.Key
			ds.pendingTokens = nil
		}
	}

	// Handle short cluster remainder: -abc → -a processed, now process -bc.
	if match.Remainder != "" {
		pseudoArg := "-" + match.Remainder
		reMatch := ds.tryMatch(pseudoArg)
		if reMatch == nil {
			return fmt.Errorf("unknown flag in cluster: -%s", match.Remainder)
		}
		return ds.handleMatch(reMatch)
	}

	return nil
}

func mergeRegistries(parent, child map[string]*flagEntry) map[string]*flagEntry {
	merged := make(map[string]*flagEntry, len(parent)+len(child))
	for k, v := range parent {
		merged[k] = v
	}
	for k, v := range child {
		merged[k] = v
	}
	return merged
}

// =========================================================================
// Completion
// =========================================================================

type CompleteResult struct {
	Flags    []string
	Values   []string
	Commands []string
}

func (a *App) Complete(tokens []string, partial string) CompleteResult {
	ds := &dispatchState{
		matchers:     a.matchers,
		interspersed: a.interspersed,
		registry:     a.registry,
		commands:     a.commands,
		args:         a.args,
	}

	// Replay completed tokens to reach the current state.
	for _, tok := range tokens {
		if ds.pendingEntry != nil {
			e := ds.pendingEntry
			consumed := true
			switch {
			case e.nargs == 1:
				e.setValue(tok)
				ds.clearPending()
			case e.nargs > 1:
				ds.pendingTokens = append(ds.pendingTokens, tok)
				if len(ds.pendingTokens) == e.nargs {
					ds.flushPending()
				}
			case e.stopToken != "":
				if tok != e.stopToken {
					ds.pendingTokens = append(ds.pendingTokens, tok)
				} else {
					ds.flushPending()
				}
			default:
				if ds.tryMatch(tok) == nil {
					ds.pendingTokens = append(ds.pendingTokens, tok)
				} else {
					ds.flushPending()
					consumed = false // tok is a flag; process it below
				}
			}
			if consumed {
				continue
			}
		}
		if !ds.forcePositional {
			match := ds.tryMatch(tok)
			if match != nil {
				if match.Key == "" {
					ds.forcePositional = true
					continue
				}
				if entry, ok := ds.registry[match.Key]; ok {
					if match.Attached != nil {
						entry.setValue(*match.Attached)
					} else if entry.nargs != 0 {
						ds.pendingEntry = entry
						ds.pendingName = match.Key
						ds.pendingTokens = nil
					}
				}
				continue
			}
		}
		if !ds.forcePositional {
			if cmd, ok := ds.commands[tok]; ok {
				ds.registry = mergeRegistries(ds.registry, cmd.registry)
				ds.commands = cmd.commands
				ds.args = cmd.args
				ds.posIdx = 0
				continue
			}
		}
		if ds.posIdx < len(ds.args) {
			ds.args[ds.posIdx].setValue(tok)
			ds.posIdx++
			if !ds.interspersed {
				ds.forcePositional = true
			}
		}
	}

	// Generate completions based on current state.
	var result CompleteResult

	if ds.pendingEntry != nil && ds.pendingEntry.complete != nil {
		result.Values = ds.pendingEntry.complete(partial)
		return result
	}

	for name, entry := range ds.registry {
		if entry.hidden {
			continue
		}
		if strings.HasPrefix(name, partial) || strings.HasPrefix("--"+name, partial) || strings.HasPrefix("/"+name, partial) {
			result.Flags = append(result.Flags, name)
		}
	}
	for name, cmd := range ds.commands {
		if cmd.hidden {
			continue
		}
		if strings.HasPrefix(name, partial) {
			result.Commands = append(result.Commands, name)
		}
	}
	if ds.posIdx < len(ds.args) && ds.args[ds.posIdx].complete != nil {
		result.Values = ds.args[ds.posIdx].complete(partial)
	}
	return result
}
