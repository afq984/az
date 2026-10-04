package az

import (
	"strings"
	"unicode/utf8"
)

// Match is what a Matcher returns when it recognizes a flag.
type Match struct {
	Key       string  // registered key, e.g. "verbose", "I", "Output"
	Attached  *string // value from --key=val, /Key:val, -Ival
	Remainder string  // unprocessed short cluster chars (e.g. "bc" from -abc); the same Matcher is asked to match "-" + Remainder next
}

// Matcher tries to interpret a raw arg as a flag.
// Returns nil if the arg is not a flag (positional / command).
//
// Matching is registry-driven: lookup reports whether a key is registered
// and how many value tokens it takes, so "-1" stays positional unless a
// flag "1" exists.
type Matcher interface {
	TryMatch(arg string, lookup func(key string) (exists bool, nargs int)) *Match
}

// KeyFormatter is implemented by Matchers that can spell a key the way
// users type it, for help, completion, and error messages. FormatKey
// reports false if the matcher cannot match key at all. The first matcher
// in the app's list that can spell a key wins.
type KeyFormatter interface {
	FormatKey(key string) (string, bool)
}

// EndOfFlagsMatcher recognizes a sentinel (typically "--") to stop flag processing.
type EndOfFlagsMatcher struct {
	Token string
}

func (m EndOfFlagsMatcher) TryMatch(arg string, _ func(string) (bool, int)) *Match {
	if arg == m.Token {
		return &Match{} // empty Key signals end-of-flags
	}
	return nil
}

// GNULongMatcher handles --flag and --flag=value.
type GNULongMatcher struct{}

func (GNULongMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "--") || len(arg) <= 2 {
		return nil
	}
	name := arg[2:]
	if eq := strings.IndexByte(name, '='); eq >= 0 {
		val := name[eq+1:]
		name = name[:eq]
		if ok, _ := lookup(name); ok {
			return &Match{Key: name, Attached: &val}
		}
		return nil
	}
	if ok, _ := lookup(name); ok {
		return &Match{Key: name}
	}
	return nil
}

func (GNULongMatcher) FormatKey(key string) (string, bool) { return "--" + key, true }

// GNUShortMatcher handles -f, -fvalue, and -abc bundling.
type GNUShortMatcher struct{}

func (GNUShortMatcher) FormatKey(key string) (string, bool) {
	return "-" + key, utf8.RuneCountInString(key) == 1
}

func (GNUShortMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "-") || len(arg) <= 1 || strings.HasPrefix(arg, "--") {
		return nil
	}
	// Split off the first rune by byte offset: values may be arbitrary bytes.
	_, size := utf8.DecodeRuneInString(arg[1:])
	first, rest := arg[1:1+size], arg[1+size:]

	ok, nargs := lookup(first)
	if !ok {
		return nil
	}
	if rest == "" {
		return &Match{Key: first}
	}
	if nargs != 0 { // one value, fixed multi, or variadic: rest is an attached value
		return &Match{Key: first, Attached: &rest}
	}
	return &Match{Key: first, Remainder: rest}
}

// WindowsSlashMatcher handles /Flag and /Flag:Value.
// Registry-driven: "/usr/bin" is not a flag unless "usr/bin" is registered.
type WindowsSlashMatcher struct{}

func (WindowsSlashMatcher) FormatKey(key string) (string, bool) { return "/" + key, true }

func (WindowsSlashMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "/") || len(arg) <= 1 {
		return nil
	}
	rest := arg[1:]
	if colon := strings.IndexByte(rest, ':'); colon >= 0 {
		key := rest[:colon]
		val := rest[colon+1:]
		if ok, _ := lookup(key); ok {
			return &Match{Key: key, Attached: &val}
		}
		return nil
	}
	if ok, _ := lookup(rest); ok {
		return &Match{Key: rest}
	}
	return nil
}

// FindMatcher handles -name style flags (single-dash, full word, no bundling).
type FindMatcher struct{}

func (FindMatcher) FormatKey(key string) (string, bool) { return "-" + key, true }

func (FindMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "-") || len(arg) <= 1 || strings.HasPrefix(arg, "--") {
		return nil
	}
	name := arg[1:]
	if ok, _ := lookup(name); ok {
		return &Match{Key: name}
	}
	return nil
}

// GCCMatcher handles longest-prefix matching: -I/path, -isystem, -DFOO.
// A trailing value attaches only to keys that take one, so -config is not
// -c with "onfig".
type GCCMatcher struct{}

func (GCCMatcher) FormatKey(key string) (string, bool) { return "-" + key, true }

func (GCCMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "-") || len(arg) <= 1 || strings.HasPrefix(arg, "--") {
		return nil
	}
	rest := arg[1:]
	for end := len(rest); end > 0; end-- {
		candidate := rest[:end]
		ok, nargs := lookup(candidate)
		tail := rest[end:]
		switch {
		case !ok:
		case tail == "":
			return &Match{Key: candidate}
		case nargs != 0:
			return &Match{Key: candidate, Attached: &tail}
		}
	}
	return nil
}
