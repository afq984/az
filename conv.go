package az

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Conv converts between strings and typed values.
// This is the core abstraction — it knows nothing about flags, tokens,
// or CLI conventions. Reusable for env vars, config files, etc.
type Conv[T any] interface {
	FromString(s string) (T, error)
	ToString(v T) string
	Complete(partial string) []string
}

// ZeroArgConv is implemented by converters whose flags don't consume
// a separate value token. The ImpliedValue is used when the flag
// appears without an explicit value (e.g., --verbose means true).
type ZeroArgConv[T any] interface {
	Conv[T]
	ImpliedValue() T
}

// MultiValueConv is implemented by converters whose flags consume several
// value tokens (e.g. --point 1 2 3). NArgs reports how many: a fixed
// count >= 2, or VariadicNArgs to consume until the next recognized flag
// or "--" (or until a terminator token, if TerminatedConv is also
// implemented).
//
// Arity is static metadata on the flag definition, not a runtime
// "wantMore" protocol on the converter: Conv values stay stateless
// shared singletons, and completion replay keeps working because token
// consumption is knowable without running conversions.
type MultiValueConv[T any] interface {
	Conv[T]
	NArgs() int
	FromStrings(parts []string) (T, error)
}

// VariadicNArgs as a MultiValueConv.NArgs result means "no fixed count".
const VariadicNArgs = -1

// TerminatedConv marks a variadic converter whose flag consumes raw
// tokens — even flag-shaped ones — until an exact terminator token,
// find -exec style.
type TerminatedConv interface {
	StopToken() string
}

// ---------------------------------------------------------------------------
// Built-in converters
// ---------------------------------------------------------------------------

type convFunc[T any] struct {
	from     func(string) (T, error)
	to       func(T) string
	complete func(string) []string
}

func (c convFunc[T]) FromString(s string) (T, error) { return c.from(s) }
func (c convFunc[T]) ToString(v T) string            { return c.to(v) }
func (c convFunc[T]) Complete(partial string) []string {
	if c.complete != nil {
		return c.complete(partial)
	}
	return nil
}

type boolConv struct{}

func (boolConv) FromString(s string) (bool, error) { return strconv.ParseBool(s) }
func (boolConv) ToString(v bool) string            { return strconv.FormatBool(v) }
func (boolConv) Complete(p string) []string        { return withPrefix([]string{"true", "false"}, p) }
func (boolConv) ImpliedValue() bool                { return true }

// Built-in converters. Bool is a ZeroArgConv: a Bool flag takes no value
// token (--verbose), but accepts an attached one (--verbose=false).
var (
	Bool     ZeroArgConv[bool]   = boolConv{}
	String   Conv[string]        = convFunc[string]{from: func(s string) (string, error) { return s, nil }, to: func(v string) string { return v }}
	Int      Conv[int]           = convFunc[int]{from: strconv.Atoi, to: strconv.Itoa}
	Int64    Conv[int64]         = convFunc[int64]{from: func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }, to: func(v int64) string { return strconv.FormatInt(v, 10) }}
	Float64  Conv[float64]       = convFunc[float64]{from: func(s string) (float64, error) { return strconv.ParseFloat(s, 64) }, to: func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }}
	Duration Conv[time.Duration] = convFunc[time.Duration]{from: time.ParseDuration, to: func(v time.Duration) string { return v.String() }}
	IP       Conv[net.IP]        = convFunc[net.IP]{
		from: func(s string) (net.IP, error) {
			ip := net.ParseIP(s)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP: %s", s)
			}
			return ip, nil
		},
		to: func(v net.IP) string { return v.String() },
	}
)

// ---------------------------------------------------------------------------
// Composable converters
// ---------------------------------------------------------------------------

// Enum returns a Conv that accepts only the given values. T is usually
// inferred: Enum("json", "text") is a Conv[string], and Enum(Red, Green)
// with typed constants is a Conv[Color].
func Enum[T ~string](allowed ...T) Conv[T] {
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	return convFunc[T]{
		from: func(s string) (T, error) {
			for _, a := range allowed {
				if s == string(a) {
					return a, nil
				}
			}
			return "", fmt.Errorf("must be one of %s, got %q", strings.Join(names, ", "), s)
		},
		to:       func(v T) string { return string(v) },
		complete: func(partial string) []string { return withPrefix(names, partial) },
	}
}

// withPrefix returns the candidates that start with partial.
func withPrefix(candidates []string, partial string) []string {
	var out []string
	for _, c := range candidates {
		if strings.HasPrefix(c, partial) {
			out = append(out, c)
		}
	}
	return out
}

// Separator builds a Conv[[]T] from a Conv[T], splitting on a delimiter.
func Separator[T any](delim string, element Conv[T]) Conv[[]T] {
	return separatorConv[T]{delim: delim, element: element}
}

type separatorConv[T any] struct {
	delim   string
	element Conv[T]
}

func (c separatorConv[T]) FromString(s string) ([]T, error) {
	parts := strings.Split(s, c.delim)
	result := make([]T, 0, len(parts))
	for _, p := range parts {
		v, err := c.element.FromString(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

func (c separatorConv[T]) ToString(v []T) string {
	strs := make([]string, len(v))
	for i, elem := range v {
		strs[i] = c.element.ToString(elem)
	}
	return strings.Join(strs, c.delim)
}

func (c separatorConv[T]) Complete(partial string) []string {
	lastDelim := strings.LastIndex(partial, c.delim)
	var prefix, last string
	if lastDelim >= 0 {
		prefix = partial[:lastDelim+len(c.delim)]
		last = partial[lastDelim+len(c.delim):]
	} else {
		last = partial
	}
	var out []string
	for _, comp := range c.element.Complete(last) {
		out = append(out, prefix+comp)
	}
	return out
}

// ---------------------------------------------------------------------------
// Multi-token converters
// ---------------------------------------------------------------------------

// FixedList builds a Conv[[]T] whose flag consumes exactly n value tokens:
//
//	point := app.Flag("point", "Coordinates", FixedList(3, Float64))
//	// --point 1 2 3, also --point=1,2,3
func FixedList[T any](n int, element Conv[T]) Conv[[]T] {
	if n < 2 {
		panic("az: FixedList: n must be >= 2")
	}
	return multiListConv[T]{n: n, element: element}
}

// Greedy builds a Conv[[]T] whose flag consumes value tokens until the
// next recognized flag, "--", or the end of arguments:
//
//	ports := app.Flag("ports", "Ports", Greedy(Int))
//	// --ports 80 443 8080 --verbose
//
// Greedy flags are ambiguous next to positional arguments and commands — a
// trailing positional or command name is consumed by the flag. Prefer FixedList, Separator, or
// FlagRepeat when the app has positionals.
func Greedy[T any](element Conv[T]) Conv[[]T] {
	return multiListConv[T]{n: VariadicNArgs, element: element}
}

// Until builds a Conv[[]T] whose flag consumes raw tokens — even
// flag-shaped ones — until an exact terminator token, find -exec style:
//
//	app.OnFlag("exec", "Command", Until(";", String), run)
//	// -exec grep -l foo ;
func Until[T any](stop string, element Conv[T]) Conv[[]T] {
	if stop == "" {
		panic("az: Until: stop token must be non-empty")
	}
	return multiListConv[T]{n: VariadicNArgs, stop: stop, element: element}
}

type multiListConv[T any] struct {
	n       int // fixed count >= 2, or VariadicNArgs
	stop    string
	element Conv[T]
}

func (c multiListConv[T]) NArgs() int        { return c.n }
func (c multiListConv[T]) StopToken() string { return c.stop }

func (c multiListConv[T]) FromStrings(parts []string) ([]T, error) {
	if c.n > 0 && len(parts) != c.n {
		return nil, fmt.Errorf("expected %d values, got %d", c.n, len(parts))
	}
	result := make([]T, 0, len(parts))
	for _, p := range parts {
		v, err := c.element.FromString(p)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

// FromString handles attached values (--point=1,2,3) and non-CLI sources
// (env vars, config files) with comma-separated syntax.
func (c multiListConv[T]) FromString(s string) ([]T, error) {
	return c.FromStrings(strings.Split(s, ","))
}

func (c multiListConv[T]) ToString(v []T) string {
	strs := make([]string, len(v))
	for i, elem := range v {
		strs[i] = c.element.ToString(elem)
	}
	return strings.Join(strs, ",")
}

func (c multiListConv[T]) Complete(partial string) []string {
	return c.element.Complete(partial)
}
