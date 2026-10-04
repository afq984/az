package az

import "strings"

// Match is what a Matcher returns when it recognizes a flag.
type Match struct {
	Key       string  // registered key, e.g. "verbose", "I", "Output"
	Attached  *string // value from --key=val, /Key:val, -Ival
	Remainder string  // unprocessed short cluster chars (e.g. "bc" from -abc)
}

// Matcher tries to interpret a raw arg as a flag.
// Returns nil if the arg is not a flag (positional / command).
type Matcher interface {
	TryMatch(arg string, lookup func(key string) (exists bool, nargs int)) *Match
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

// GNUShortMatcher handles -f, -fvalue, and -abc bundling.
type GNUShortMatcher struct{}

func (GNUShortMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "-") || len(arg) <= 1 || strings.HasPrefix(arg, "--") {
		return nil
	}
	runes := []rune(arg[1:])
	first := string(runes[0])
	rest := string(runes[1:])

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
// Registry-driven: "/usr/bin" is not a flag unless "usr" is registered.
type WindowsSlashMatcher struct{}

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
type GCCMatcher struct{}

func (GCCMatcher) TryMatch(arg string, lookup func(string) (bool, int)) *Match {
	if !strings.HasPrefix(arg, "-") || len(arg) <= 1 || strings.HasPrefix(arg, "--") {
		return nil
	}
	rest := arg[1:]
	for end := len(rest); end > 0; end-- {
		candidate := rest[:end]
		if ok, _ := lookup(candidate); ok {
			tail := rest[end:]
			if tail == "" {
				return &Match{Key: candidate}
			}
			return &Match{Key: candidate, Attached: &tail}
		}
	}
	return nil
}
