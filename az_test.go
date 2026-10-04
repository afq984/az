package az

import (
	"strings"
	"testing"
)

func eq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func sliceEq[T comparable](t *testing.T, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Fixed arity: --point 1 2 3
// ---------------------------------------------------------------------------

func TestFixedAritySeparateTokens(t *testing.T) {
	app := NewApp("test", "")
	point := app.Flag("point", "Coordinates", FixedList(3, Float64))
	verbose := app.Flag("verbose", "Verbose", Bool)

	_, err := app.ParseArgs([]string{"--point", "1", "2.5", "3", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, point.Ref().Get(), []float64{1, 2.5, 3})
	eq(t, verbose.Ref().Get(), true)
}

func TestFixedArityAttached(t *testing.T) {
	app := NewApp("test", "")
	point := app.Flag("point", "Coordinates", FixedList(3, Float64))

	_, err := app.ParseArgs([]string{"--point=1,2,3"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, point.Ref().Get(), []float64{1, 2, 3})
}

func TestFixedArityShortAttached(t *testing.T) {
	app := NewApp("test", "")
	point := app.Flag("point", "Coordinates", FixedList(2, Int))
	app.Alias("p", "point")

	_, err := app.ParseArgs([]string{"-p1,2"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, point.Ref().Get(), []int{1, 2})
}

// Fixed arity consumes the declared number of tokens unconditionally, so
// flag-shaped values like negative numbers just work — no argparse-style
// negative-number heuristics needed.
func TestFixedArityNegativeNumbers(t *testing.T) {
	app := NewApp("test", "")
	delta := app.Flag("delta", "Deltas", FixedList(3, Int))

	_, err := app.ParseArgs([]string{"--delta", "-1", "5", "-3"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, delta.Ref().Get(), []int{-1, 5, -3})
}

func TestFixedArityTooFewValues(t *testing.T) {
	app := NewApp("test", "")
	app.Flag("point", "Coordinates", FixedList(3, Float64))

	_, err := app.ParseArgs([]string{"--point", "1", "2"})
	if err == nil || !strings.Contains(err.Error(), "requires 3 values, got 2") {
		t.Fatalf("want arity error, got %v", err)
	}
}

func TestFixedArityFollowedByPositional(t *testing.T) {
	// Unlike greedy, fixed arity composes fine with positionals.
	app := NewApp("test", "")
	point := app.Flag("point", "Coordinates", FixedList(2, Int))
	file := app.Arg("file", "File", String)

	_, err := app.ParseArgs([]string{"--point", "3", "4", "in.txt"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, point.Ref().Get(), []int{3, 4})
	eq(t, file.Ref().Get(), "in.txt")
}

// ---------------------------------------------------------------------------
// Greedy variadic: --ports 80 443 8080
// ---------------------------------------------------------------------------

func TestGreedyStopsAtFlag(t *testing.T) {
	app := NewApp("test", "")
	ports := app.Flag("ports", "Ports", Greedy(Int))
	verbose := app.Flag("verbose", "Verbose", Bool)

	_, err := app.ParseArgs([]string{"--ports", "80", "443", "8080", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, ports.Ref().Get(), []int{80, 443, 8080})
	eq(t, verbose.Ref().Get(), true)
}

func TestGreedyStopsAtEndOfArgs(t *testing.T) {
	app := NewApp("test", "")
	ports := app.Flag("ports", "Ports", Greedy(Int))

	_, err := app.ParseArgs([]string{"--ports", "80", "443"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, ports.Ref().Get(), []int{80, 443})
}

func TestGreedyStopsAtEndOfFlags(t *testing.T) {
	app := NewApp("test", "")
	ports := app.Flag("ports", "Ports", Greedy(Int))
	file := app.Arg("file", "File", String)

	_, err := app.ParseArgs([]string{"--ports", "80", "--", "in.txt"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, ports.Ref().Get(), []int{80})
	eq(t, file.Ref().Get(), "in.txt")
}

func TestGreedyEmpty(t *testing.T) {
	app := NewApp("test", "")
	app.Flag("ports", "Ports", Greedy(Int))
	app.Flag("verbose", "Verbose", Bool)

	_, err := app.ParseArgs([]string{"--ports", "--verbose"})
	if err == nil || !strings.Contains(err.Error(), "requires a value") {
		t.Fatalf("want missing-value error, got %v", err)
	}
}

// Registry-driven matching means "-1" is not a flag (no flag named "1"),
// so greedy flags accept negative numbers too — the same permissiveness
// that swallows --typos is what makes this work.
func TestGreedyNegativeNumbers(t *testing.T) {
	app := NewApp("test", "")
	nums := app.Flag("nums", "Numbers", Greedy(Int))
	verbose := app.Flag("verbose", "Verbose", Bool)

	_, err := app.ParseArgs([]string{"--nums", "-1", "-2", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, nums.Ref().Get(), []int{-1, -2})
	eq(t, verbose.Ref().Get(), true)
}

// The inherent ambiguity that kept greedy flags out of v8: a greedy flag
// followed by a positional swallows it. No mechanism can fix this — the
// grammar itself is ambiguous. But because arity is declared metadata,
// a registration-time lint (greedy flag + declared positionals) is
// possible; that's the introspection dividend of staying declarative.
func TestGreedySwallowsTrailingPositionalFootgun(t *testing.T) {
	app := NewApp("cp", "")
	files := app.Flag("files", "Files", Greedy(String))
	dst := app.Arg("dst", "Destination", String)

	_, err := app.ParseArgs([]string{"--files", "a.txt", "b.txt", "dest-dir"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, files.Ref().Get(), []string{"a.txt", "b.txt", "dest-dir"}) // dst swallowed!
	eq(t, dst.Ref().IsSet(), false)
}

// ---------------------------------------------------------------------------
// Terminator-delimited variadic: find -exec grep -l foo ;
// ---------------------------------------------------------------------------

func TestUntilTerminator(t *testing.T) {
	app := NewApp("find", "", FindMatcher{})
	path := app.Arg("path", "Start path", String)
	exec := app.Flag("exec", "Command", Until(";", String))
	// "l" is registered as our flag, but inside -exec ... ; the raw
	// tokens are consumed unconditionally, so grep's -l is not matched.
	ell := app.Flag("l", "Ell", Bool)
	print := app.Flag("print", "Print", Bool)

	_, err := app.ParseArgs([]string{"/usr", "-exec", "grep", "-l", "foo", "{}", ";", "-print"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, path.Ref().Get(), "/usr")
	sliceEq(t, exec.Ref().Get(), []string{"grep", "-l", "foo", "{}"})
	eq(t, ell.Ref().IsSet(), false)
	eq(t, print.Ref().Get(), true)
}

func TestUntilMissingTerminator(t *testing.T) {
	app := NewApp("find", "", FindMatcher{})
	app.Flag("exec", "Command", Until(";", String))

	_, err := app.ParseArgs([]string{"-exec", "grep", "foo"})
	if err == nil || !strings.Contains(err.Error(), "missing terminator") {
		t.Fatalf("want terminator error, got %v", err)
	}
}

// Ordered actions compose with multi-token consumption: each -exec fires
// the callback with its full token list, in occurrence order.
func TestUntilWithOnFlagOrdering(t *testing.T) {
	app := NewApp("find", "", FindMatcher{})
	var trace []string
	app.OnFlag("name", "Pattern", String, func(s string) error {
		trace = append(trace, "name="+s)
		return nil
	})
	app.OnFlag("exec", "Command", Until(";", String), func(argv []string) error {
		trace = append(trace, "exec="+strings.Join(argv, " "))
		return nil
	})
	app.OnFlagBool("print", "Print", func() error {
		trace = append(trace, "print")
		return nil
	})

	_, err := app.ParseArgs([]string{
		"-name", "*.go", "-exec", "wc", "-name", "{}", ";", "-print",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Note "-name" inside the exec region is raw, not our flag.
	sliceEq(t, trace, []string{"name=*.go", "exec=wc -name {}", "print"})
}

// ---------------------------------------------------------------------------
// Dispatch policy: Interspersed(false), POSIX-style
// ---------------------------------------------------------------------------

func TestInterspersedDefault(t *testing.T) {
	// GNU default (v8 behavior): flags after positionals still parse.
	app := NewApp("prog", "")
	verbose := app.Flag("verbose", "Verbose", Bool)
	file := app.Arg("file", "File", String)

	_, err := app.ParseArgs([]string{"a.txt", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, file.Ref().Get(), "a.txt")
	eq(t, verbose.Ref().Get(), true)
}

func TestNoInterspersedFlags(t *testing.T) {
	// POSIX: first positional ends flag parsing; later flag-shaped
	// tokens are literal positionals.
	app := NewApp("prog", "").Interspersed(false)
	verbose := app.Flag("verbose", "Verbose", Bool)
	file := app.Arg("file", "File", String)
	rest := app.Arg("rest", "Rest", String)

	_, err := app.ParseArgs([]string{"a.txt", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, file.Ref().Get(), "a.txt")
	eq(t, rest.Ref().Get(), "--verbose")
	eq(t, verbose.Ref().IsSet(), false)
}

func TestNoInterspersedFlagsBeforePositionalStillWork(t *testing.T) {
	app := NewApp("prog", "").Interspersed(false)
	verbose := app.Flag("verbose", "Verbose", Bool)
	file := app.Arg("file", "File", String)

	_, err := app.ParseArgs([]string{"--verbose", "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, verbose.Ref().Get(), true)
	eq(t, file.Ref().Get(), "a.txt")
}

// ---------------------------------------------------------------------------
// Completion through multi-token flags
// ---------------------------------------------------------------------------

func TestCompleteMidFixedArity(t *testing.T) {
	app := NewApp("test", "")
	app.Flag("pair", "Pair", FixedList(2, Enum("aa", "ab", "bb")))

	// One of two tokens consumed — still pending, complete the second.
	res := app.Complete([]string{"--pair", "aa"}, "a")
	sliceEq(t, res.Values, []string{"aa", "ab"})
}

func TestCompleteAfterFixedArity(t *testing.T) {
	app := NewApp("test", "")
	app.Flag("pair", "Pair", FixedList(2, Enum("aa", "bb")))
	app.Flag("verbose", "Verbose", Bool)

	// Arity satisfied — back to flag completion.
	res := app.Complete([]string{"--pair", "aa", "bb"}, "--v")
	sliceEq(t, res.Flags, []string{"verbose"})
}

// ---------------------------------------------------------------------------
// v8 baseline behavior unchanged
// ---------------------------------------------------------------------------

func TestBaselineStillWorks(t *testing.T) {
	app := NewApp("test", "")
	name := app.Flag("name", "Name", String).Default("world")
	verbosity := app.FlagCounter("verbose", "Verbosity")
	app.Alias("v", "verbose")
	all := app.Flag("all", "All", Bool)
	app.Alias("a", "all")
	tags := app.Flag("tags", "Tags", Separator(",", String))

	_, err := app.ParseArgs([]string{"--name", "Alice", "-avvv", "--tags", "x,y"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, name.Ref().Get(), "Alice")
	eq(t, all.Ref().Get(), true)
	eq(t, verbosity.Ref().Get(), 3)
	sliceEq(t, tags.Ref().Get(), []string{"x", "y"})
}
