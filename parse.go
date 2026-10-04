package az

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"
	"unicode/utf8"
)

var (
	// ErrHelp is returned by ParseArgs when the help flag is given.
	ErrHelp = errors.New("help requested")
	// ErrVersion is returned by ParseArgs when the version flag is given.
	ErrVersion = errors.New("version requested")
)

// convErr marks a conversion failure, as opposed to an error returned by an
// OnFlag callback, which is passed through unchanged.
type convErr struct{ err error }

func (e convErr) Error() string { return e.err.Error() }

// ParseArgs parses command-line arguments (without the program name) into
// the registered flags and arguments, and returns the selected command
// path, space-separated: "" for none, "remote add" for a nested command.
// The path is returned even on error, so callers can show help for the
// command the user was invoking.
//
// Parsing stops at the help or version flag with ErrHelp or ErrVersion.
// Values are stored as they are parsed, so an App is meant to be parsed
// once.
func (a *App) ParseArgs(args []string) (string, error) {
	w := a.newWalker(false)
	for _, tok := range args {
		if err := w.step(tok); err != nil {
			return w.cmdPath(), err
		}
	}
	return w.cmdPath(), w.finish()
}

// MustParse parses args like ParseArgs and handles everything except
// success: help and version are written to stdout followed by exit status
// 0, and errors to stderr followed by exit status 2.
func (a *App) MustParse(args []string) string {
	cmd, code := a.parse(args, os.Stdout, os.Stderr)
	if code >= 0 {
		os.Exit(code)
	}
	return cmd
}

// parse implements MustParse. It returns the exit status, or -1 to continue.
func (a *App) parse(args []string, stdout, stderr io.Writer) (string, int) {
	cmd, err := a.ParseArgs(args)
	switch {
	case err == nil:
		return cmd, -1
	case errors.Is(err, ErrHelp):
		a.WriteHelp(stdout, cmd)
		return cmd, 0
	case errors.Is(err, ErrVersion):
		fmt.Fprintln(stdout, a.version)
		return cmd, 0
	}
	invocation := strings.Join(append(append([]string{a.name}, strings.Fields(cmd)...), a.displayKey("help")), " ")
	fmt.Fprintf(stderr, "%s: error: %v\nTry '%s' for more information.\n", a.name, err, invocation)
	return cmd, 2
}

// walker consumes command-line tokens one at a time. ParseArgs runs it
// live; Complete and WriteHelp run it dry, which advances the same state
// without running converters, setters, or callbacks. That works because
// how many tokens a flag consumes is declared metadata (nargs, stopToken),
// never the result of a conversion.
type walker struct {
	app *App
	dry bool

	path            []string              // selected command names
	scopes          []*scope              // the app, then each selected command
	registry        map[string]*flagEntry // keys in effect; inner scopes shadow outer ones
	posIdx          int                   // next positional argument of the innermost scope
	forcePositional bool                  // after "--", or after a positional with Interspersed(false)

	pending       *flagEntry // flag waiting for value tokens
	pendingKey    string     // the key it was matched by, for messages
	pendingTokens []string   // value tokens collected for a multi-token flag
}

func (a *App) newWalker(dry bool) *walker {
	return &walker{app: a, dry: dry, scopes: []*scope{&a.scope}, registry: a.registry}
}

func (w *walker) cur() *scope     { return w.scopes[len(w.scopes)-1] }
func (w *walker) cmdPath() string { return strings.Join(w.path, " ") }

func (w *walker) step(tok string) error {
	if w.pending != nil {
		consumed, err := w.feed(tok)
		if consumed || err != nil {
			return err
		}
		// A greedy flag ended at tok, which is processed as a flag below.
	}
	if !w.forcePositional {
		if m, matcher := w.match(tok); m != nil {
			if m.Key == "" {
				w.forcePositional = true
				return nil
			}
			return w.handleMatch(tok, m, matcher)
		}
		if cmd, ok := w.cur().commands[tok]; ok {
			w.enter(tok, cmd)
			return nil
		}
	}
	return w.positional(tok)
}

// feed offers tok to the pending flag and reports whether it was consumed.
func (w *walker) feed(tok string) (bool, error) {
	e := w.pending
	switch {
	case e.nargs == 1:
		key := w.pendingKey
		w.clearPending()
		return true, w.apply(key, func() error { return e.setValue(tok) })

	case e.nargs > 1:
		// Fixed arity consumes unconditionally, so "--delta -1 5 -3" works:
		// the next 3 tokens are values by declaration.
		w.pendingTokens = append(w.pendingTokens, tok)
		if len(w.pendingTokens) < e.nargs {
			return true, nil
		}
		return true, w.flush()

	case e.stopToken != "":
		// Consume raw tokens, even ones that look like our flags, up to the
		// terminator (find -exec grep -l foo ;).
		if tok == e.stopToken {
			return true, w.flush()
		}
		w.pendingTokens = append(w.pendingTokens, tok)
		return true, nil

	default:
		// Greedy: consume until the next recognized flag.
		if m, _ := w.match(tok); m == nil {
			w.pendingTokens = append(w.pendingTokens, tok)
			return true, nil
		}
		return false, w.flush()
	}
}

// flush hands the collected tokens to the pending multi-token flag.
func (w *walker) flush() error {
	e, key, parts := w.pending, w.pendingKey, w.pendingTokens
	w.clearPending()
	if len(parts) == 0 && e.stopToken == "" && !w.dry {
		return fmt.Errorf("flag %s requires a value", w.app.displayKey(key))
	}
	return w.apply(key, func() error { return e.setValues(parts) })
}

func (w *walker) clearPending() {
	w.pending, w.pendingKey, w.pendingTokens = nil, "", nil
}

// apply runs set for the flag matched as key, unless walking dry.
func (w *walker) apply(key string, set func() error) error {
	if w.dry {
		return nil
	}
	err := set()
	if ce, ok := err.(convErr); ok {
		return fmt.Errorf("invalid value for %s: %w", w.app.displayKey(key), ce.err)
	}
	return err
}

func (w *walker) lookup(key string) (bool, int) {
	e, ok := w.registry[key]
	if !ok {
		return false, 0
	}
	return true, e.nargs
}

// match returns the first matcher's interpretation of tok as a flag.
func (w *walker) match(tok string) (*Match, Matcher) {
	for _, m := range w.app.matchers {
		if match := m.TryMatch(tok, w.lookup); match != nil {
			return match, m
		}
	}
	return nil, nil
}

// handleMatch applies m, which matcher produced from tok.
func (w *walker) handleMatch(tok string, m *Match, matcher Matcher) error {
	e, ok := w.registry[m.Key]
	if !ok {
		return fmt.Errorf("unknown flag: %s", w.app.displayKey(m.Key))
	}
	if !w.dry {
		e.seen = true
	}
	var err error
	switch {
	case m.Attached != nil:
		// Attached values go through FromString for any arity:
		// --verbose=false, /V:true, --point=1,2,3.
		err = w.apply(m.Key, func() error { return e.setValue(*m.Attached) })
	case e.nargs == 0:
		err = w.apply(m.Key, e.setTrue)
	default:
		w.pending, w.pendingKey, w.pendingTokens = e, m.Key, nil
	}
	if err != nil || m.Remainder == "" {
		return err
	}
	// Short cluster: -abc was handled as -a; the same matcher continues
	// with -bc, so "-a-verbose" cannot reach a long flag.
	next := matcher.TryMatch("-"+m.Remainder, w.lookup)
	if next == nil {
		_, size := utf8.DecodeRuneInString(m.Remainder)
		return fmt.Errorf("unknown flag -%s in %s", m.Remainder[:size], tok)
	}
	return w.handleMatch(tok, next, matcher)
}

func (w *walker) enter(name string, cmd *Command) {
	w.path = append(w.path, name)
	w.scopes = append(w.scopes, &cmd.scope)
	merged := maps.Clone(w.registry)
	maps.Copy(merged, cmd.registry)
	w.registry = merged
	w.posIdx = 0
}

func (w *walker) positional(tok string) error {
	args := w.cur().args
	if w.posIdx >= len(args) {
		return fmt.Errorf("unexpected argument: %s", tok)
	}
	arg := args[w.posIdx]
	if !arg.repeat {
		w.posIdx++
	}
	if !w.app.interspersed {
		w.forcePositional = true
	}
	if w.dry {
		return nil
	}
	arg.seen = true
	if err := arg.setValue(tok); err != nil {
		return fmt.Errorf("invalid value for <%s>: %w", arg.name, err)
	}
	return nil
}

// finish checks the state at the end of the arguments.
func (w *walker) finish() error {
	if e := w.pending; e != nil {
		key := w.app.displayKey(w.pendingKey)
		switch {
		case e.nargs == 1:
			return fmt.Errorf("flag %s requires a value", key)
		case e.nargs > 1:
			return fmt.Errorf("flag %s requires %d values, got %d", key, e.nargs, len(w.pendingTokens))
		case e.stopToken != "":
			return fmt.Errorf("flag %s: missing terminator %q", key, e.stopToken)
		}
		if err := w.flush(); err != nil {
			return err
		}
	}
	for _, s := range w.scopes {
		for _, e := range s.flags {
			// A flag shadowed by an inner command's flag is out of effect,
			// as in help, so its Required no longer applies.
			if keys := w.keys(e); e.required && !e.seen && len(keys) > 0 {
				return fmt.Errorf("required flag %s not provided", w.app.displayKey(keys[0]))
			}
		}
		for _, a := range s.args {
			if a.required && !a.seen {
				return fmt.Errorf("required argument <%s> not provided", a.name)
			}
		}
	}
	return nil
}

// flags returns the flags in effect, innermost scope first, each scope in
// registration order. Fully shadowed flags are left out.
func (w *walker) flags() []*flagEntry {
	var out []*flagEntry
	for i := len(w.scopes) - 1; i >= 0; i-- {
		for _, e := range w.scopes[i].flags {
			if len(w.keys(e)) > 0 {
				out = append(out, e)
			}
		}
	}
	return out
}

// keys returns the keys of e that are not shadowed by an inner scope.
func (w *walker) keys(e *flagEntry) []string {
	var out []string
	for _, k := range e.keys {
		if w.registry[k] == e {
			out = append(out, k)
		}
	}
	return out
}

// displayKey spells key the way users type it, using the first matcher
// that can match it.
func (a *App) displayKey(key string) string {
	for _, m := range a.matchers {
		if f, ok := m.(KeyFormatter); ok {
			if s, ok := f.FormatKey(key); ok {
				return s
			}
		}
	}
	return key
}
