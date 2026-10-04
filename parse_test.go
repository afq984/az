package az

import (
	"errors"
	"slices"
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
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func wantErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("got error %v, want one containing %q", err, substr)
	}
}

func wantPanic(t *testing.T, substr string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		if s, _ := r.(string); !strings.Contains(s, substr) {
			t.Fatalf("got panic %v, want one containing %q", r, substr)
		}
	}()
	f()
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

// The inherent ambiguity of greedy flags: a greedy flag
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
	// GNU default: flags after positionals still parse.
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
// Basics
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

func TestSubcommands(t *testing.T) {
	app := NewApp("git", "")
	verbose := app.Flag("verbose", "Verbose", Bool)
	remote := app.Cmd("remote", "Manage remotes")
	add := remote.Cmd("add", "Add a remote")
	name := add.Arg("name", "Name", String)
	fetch := add.Flag("fetch", "Fetch", Bool)

	cmd, err := app.ParseArgs([]string{"remote", "add", "--fetch", "origin", "--verbose"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, cmd, "remote add")
	eq(t, name.Ref().Get(), "origin")
	eq(t, fetch.Ref().Get(), true)
	eq(t, verbose.Ref().Get(), true) // outer flags stay in effect
}

func TestSubcommandFlagShadowsOuter(t *testing.T) {
	app := NewApp("prog", "")
	outer := app.Flag("n", "Outer", Int)
	inner := app.Cmd("run", "").Flag("n", "Inner", String)

	_, err := app.ParseArgs([]string{"-n", "1", "run", "-n", "x"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, outer.Ref().Get(), 1)
	eq(t, inner.Ref().Get(), "x")
}

func TestShadowedRequiredFlag(t *testing.T) {
	newApp := func() (*App, *FlagClause[int], *FlagClause[string]) {
		app := NewApp("prog", "")
		outer := app.Flag("n", "Outer", Int).Required()
		inner := app.Cmd("run", "").Flag("n", "Inner", String)
		return app, outer, inner
	}

	// After "run", -n is the inner flag; the outer one is out of effect.
	app, outer, inner := newApp()
	if _, err := app.ParseArgs([]string{"run", "-n", "x"}); err != nil {
		t.Fatal(err)
	}
	eq(t, outer.Ref().IsSet(), false)
	eq(t, inner.Ref().Get(), "x")

	// Without a command, the outer flag is still required.
	app, _, _ = newApp()
	_, err := app.ParseArgs(nil)
	wantErr(t, err, "required flag -n not provided")
}

func TestPartlyShadowedRequiredFlag(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("name", "Outer", String).Required()
	app.Alias("n", "name")
	app.Cmd("run", "").Flag("name", "Inner", String)

	// --name now means the inner flag, but -n still reaches the outer one.
	_, err := app.ParseArgs([]string{"run", "--name", "x"})
	wantErr(t, err, "required flag -n not provided")
}

func TestCommandFlagNotAvailableBeforeCommand(t *testing.T) {
	app := NewApp("prog", "")
	app.Cmd("run", "").Flag("fast", "", Bool)

	_, err := app.ParseArgs([]string{"--fast", "run"})
	wantErr(t, err, "unexpected argument: --fast")
}

// ---------------------------------------------------------------------------
// Required, defaults, repeats
// ---------------------------------------------------------------------------

func TestRequiredFlag(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("name", "Name", String).Default("x").Required()

	_, err := app.ParseArgs(nil)
	wantErr(t, err, "required flag --name not provided") // a default does not satisfy Required

	app = NewApp("prog", "")
	name := app.Flag("name", "Name", String).Required()
	app.Alias("n", "name")
	if _, err := app.ParseArgs([]string{"-n", "a"}); err != nil {
		t.Fatal(err)
	}
	eq(t, name.Ref().Get(), "a")
}

func TestRequiredArg(t *testing.T) {
	newApp := func() *App {
		app := NewApp("prog", "")
		app.Arg("src", "Source", String).Required()
		app.Arg("dst", "Destination", String)
		return app
	}

	_, err := newApp().ParseArgs(nil)
	wantErr(t, err, "required argument <src> not provided")

	if _, err := newApp().ParseArgs([]string{"a"}); err != nil {
		t.Fatal(err)
	}
}

func TestRequiredOnlyInSelectedCommand(t *testing.T) {
	newApp := func() *App {
		app := NewApp("prog", "")
		app.Cmd("deploy", "").Flag("target", "", String).Required()
		app.Cmd("status", "")
		return app
	}

	if _, err := newApp().ParseArgs([]string{"status"}); err != nil {
		t.Fatal(err)
	}
	cmd, err := newApp().ParseArgs([]string{"deploy"})
	wantErr(t, err, "required flag --target not provided")
	eq(t, cmd, "deploy")
}

func TestFlagRepeatReplacesDefault(t *testing.T) {
	app := NewApp("prog", "")
	tags := app.FlagRepeat("tag", "Tags", String).Default([]string{"default"})

	if _, err := app.ParseArgs([]string{"--tag", "a", "--tag=b"}); err != nil {
		t.Fatal(err)
	}
	sliceEq(t, tags.Ref().Get(), []string{"a", "b"})
}

func TestFlagRepeatKeepsDefaultWhenAbsent(t *testing.T) {
	app := NewApp("prog", "")
	tags := app.FlagRepeat("tag", "Tags", String).Default([]string{"default"})

	if _, err := app.ParseArgs(nil); err != nil {
		t.Fatal(err)
	}
	sliceEq(t, tags.Ref().Get(), []string{"default"})
	eq(t, tags.Ref().IsSet(), false)
}

func TestFlagRepeatMultiToken(t *testing.T) {
	app := NewApp("prog", "")
	points := app.FlagRepeat("point", "Points", FixedList(2, Int))

	if _, err := app.ParseArgs([]string{"--point", "1", "2", "--point", "3", "4"}); err != nil {
		t.Fatal(err)
	}
	got := points.Ref().Get()
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 points", got)
	}
	sliceEq(t, got[0], []int{1, 2})
	sliceEq(t, got[1], []int{3, 4})
}

func TestFlagCounterReplacesDefault(t *testing.T) {
	app := NewApp("prog", "")
	v := app.FlagCounter("verbose", "Verbosity").Default(5)
	app.Alias("v", "verbose")

	if _, err := app.ParseArgs([]string{"-vv", "--verbose=4"}); err != nil {
		t.Fatal(err)
	}
	eq(t, v.Ref().Get(), 6)
}

// ---------------------------------------------------------------------------
// Positional arguments
// ---------------------------------------------------------------------------

func TestArgRepeat(t *testing.T) {
	app := NewApp("cp", "")
	force := app.Flag("force", "Force", Bool)
	files := app.ArgRepeat("files", "Files", String)

	if _, err := app.ParseArgs([]string{"a", "--force", "b", "--", "--c"}); err != nil {
		t.Fatal(err)
	}
	sliceEq(t, files.Ref().Get(), []string{"a", "b", "--c"})
	eq(t, force.Ref().Get(), true)
}

func TestArgRepeatAfterArg(t *testing.T) {
	app := NewApp("exec", "").Interspersed(false)
	verbose := app.Flag("verbose", "Verbose", Bool)
	prog := app.Arg("prog", "Program", String).Required()
	argv := app.ArgRepeat("args", "Arguments", String)

	if _, err := app.ParseArgs([]string{"--verbose", "ls", "-l", "--verbose"}); err != nil {
		t.Fatal(err)
	}
	eq(t, verbose.Ref().Get(), true)
	eq(t, prog.Ref().Get(), "ls")
	sliceEq(t, argv.Ref().Get(), []string{"-l", "--verbose"})
}

func TestArgRepeatRequired(t *testing.T) {
	app := NewApp("rm", "")
	app.ArgRepeat("files", "Files", String).Required()

	_, err := app.ParseArgs(nil)
	wantErr(t, err, "required argument <files> not provided")
}

func TestUnexpectedArgument(t *testing.T) {
	app := NewApp("prog", "")
	app.Arg("file", "File", String)

	_, err := app.ParseArgs([]string{"a", "b"})
	wantErr(t, err, "unexpected argument: b")
}

func TestInvalidArgValue(t *testing.T) {
	app := NewApp("prog", "")
	app.Arg("count", "Count", Int)

	_, err := app.ParseArgs([]string{"x"})
	wantErr(t, err, "invalid value for <count>")
}

// ---------------------------------------------------------------------------
// Errors and callbacks
// ---------------------------------------------------------------------------

func TestInvalidValueNamesSpellingUsed(t *testing.T) {
	newApp := func() *App {
		app := NewApp("prog", "")
		app.Flag("count", "Count", Int)
		app.Alias("c", "count")
		return app
	}

	_, err := newApp().ParseArgs([]string{"--count", "x"})
	wantErr(t, err, "invalid value for --count")
	_, err = newApp().ParseArgs([]string{"-cx"})
	wantErr(t, err, "invalid value for -c")
}

func TestUnknownFlagInCluster(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("a", "", Bool)

	_, err := app.ParseArgs([]string{"-ax"})
	wantErr(t, err, "unknown flag -x in -ax")
}

// The rest of a cluster is matched by the short matcher only, so it can't
// spell a long flag or the end-of-flags marker.
func TestClusterCannotReachLongFlags(t *testing.T) {
	for _, tok := range []string{"-a-verbose", "-a-name=evil", "-a-"} {
		app := NewApp("prog", "")
		app.Flag("a", "", Bool)
		verbose := app.Flag("verbose", "", Bool)
		name := app.Flag("name", "", String)

		_, err := app.ParseArgs([]string{tok})
		wantErr(t, err, "unknown flag -- in "+tok)
		eq(t, verbose.Ref().IsSet(), false)
		eq(t, name.Ref().IsSet(), false)
	}
}

func TestShortAttachedValueKeepsBytes(t *testing.T) {
	app := NewApp("prog", "")
	out := app.Flag("o", "", String)

	if _, err := app.ParseArgs([]string{"-ocaf\xe9.txt"}); err != nil {
		t.Fatal(err)
	}
	eq(t, out.Ref().Get(), "caf\xe9.txt")
}

func TestOnFlagWithBoolFiresOnBareFlag(t *testing.T) {
	app := NewApp("prog", "")
	var got []bool
	app.OnFlag("dry-run", "", Bool, func(v bool) error {
		got = append(got, v)
		return nil
	})

	if _, err := app.ParseArgs([]string{"--dry-run", "--dry-run=false"}); err != nil {
		t.Fatal(err)
	}
	sliceEq(t, got, []bool{true, false})
}

var errStop = errors.New("stop")

func TestCallbackErrorPassesThrough(t *testing.T) {
	app := NewApp("prog", "")
	calls := 0
	app.OnFlagBool("stop", "", func() error {
		calls++
		return errStop
	})

	_, err := app.ParseArgs([]string{"--stop", "--stop"})
	if err != errStop {
		t.Fatalf("got %v, want errStop unwrapped", err)
	}
	eq(t, calls, 1) // parsing stops at the first error
}

func TestEnumTyped(t *testing.T) {
	type Color string
	const (
		Red   Color = "red"
		Green Color = "green"
	)
	newApp := func() (*App, *FlagClause[Color]) {
		app := NewApp("prog", "")
		return app, app.Flag("color", "Color", Enum(Red, Green)).Default(Red)
	}

	app, color := newApp()
	if _, err := app.ParseArgs([]string{"--color", "green"}); err != nil {
		t.Fatal(err)
	}
	eq(t, color.Ref().Get(), Green)

	app, _ = newApp()
	_, err := app.ParseArgs([]string{"--color", "blue"})
	wantErr(t, err, `must be one of red, green, got "blue"`)
}

// ---------------------------------------------------------------------------
// Help and version
// ---------------------------------------------------------------------------

func TestHelpFlag(t *testing.T) {
	app := NewApp("git", "")
	app.Flag("name", "", String).Required()
	app.Cmd("remote", "").Cmd("add", "")

	cmd, err := app.ParseArgs([]string{"remote", "add", "--help", "--bogus"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("got %v, want ErrHelp", err)
	}
	eq(t, cmd, "remote add")
}

func TestHelpShortAlias(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("all", "", Bool)
	app.Alias("a", "all")
	app.Alias("h", "help")

	_, err := app.ParseArgs([]string{"-ah"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("got %v, want ErrHelp", err)
	}
}

func TestVersionFlag(t *testing.T) {
	newApp := func() *App { return NewApp("prog", "").Version("1.2.3").Version("1.2.4") }

	_, err := newApp().ParseArgs([]string{"--version"})
	if !errors.Is(err, ErrVersion) {
		t.Fatalf("got %v, want ErrVersion", err)
	}
	if _, err := newApp().ParseArgs([]string{"--version=false"}); err != nil {
		t.Fatal(err)
	}
}

func TestMustParseOutcomes(t *testing.T) {
	newApp := func() *App {
		app := NewApp("prog", "Does things.").Version("v0.1.0")
		app.Cmd("run", "").Arg("n", "", Int)
		return app
	}
	for _, tc := range []struct {
		args           []string
		code           int
		stdout, stderr string
	}{
		{args: []string{"run", "1"}, code: -1},
		{args: []string{"--version"}, code: 0, stdout: "v0.1.0\n"},
		{args: []string{"--help"}, code: 0, stdout: "usage: prog [<flags>] <command> [<args> ...]\n"},
		{args: []string{"run", "--help"}, code: 0, stdout: "usage: prog run [<flags>] [<n>]\n"},
		{args: []string{"run", "x"}, code: 2, stderr: "prog: error: invalid value for <n>: strconv.Atoi: parsing \"x\": invalid syntax\n" +
			"Try 'prog run --help' for more information.\n"},
	} {
		var stdout, stderr strings.Builder
		_, code := newApp().parse(tc.args, &stdout, &stderr)
		eq(t, code, tc.code)
		if !strings.HasPrefix(stdout.String(), tc.stdout) || (tc.stdout == "") != (stdout.Len() == 0) {
			t.Errorf("%v: stdout = %q, want prefix %q", tc.args, stdout.String(), tc.stdout)
		}
		eq(t, stderr.String(), tc.stderr)
	}
}

// ---------------------------------------------------------------------------
// Registration mistakes panic
// ---------------------------------------------------------------------------

func TestRegistrationPanics(t *testing.T) {
	wantPanic(t, `flag "x" registered twice`, func() {
		app := NewApp("prog", "")
		app.Flag("x", "", Bool)
		app.FlagCounter("x", "")
	})
	wantPanic(t, `flag "help" registered twice`, func() {
		NewApp("prog", "").Flag("help", "", Bool)
	})
	wantPanic(t, `flag "version" registered twice`, func() {
		app := NewApp("prog", "")
		app.Flag("version", "", String)
		app.Version("1.0")
	})
	wantPanic(t, `flag "v" registered twice`, func() {
		app := NewApp("prog", "")
		app.Flag("v", "", Bool)
		app.Flag("verbose", "", Bool)
		app.Alias("v", "verbose")
	})
	wantPanic(t, `no flag "missing" in this scope`, func() {
		NewApp("prog", "").Alias("m", "missing")
	})
	wantPanic(t, `command "run" registered twice`, func() {
		app := NewApp("prog", "")
		app.Cmd("run", "")
		app.Cmd("run", "")
	})
	wantPanic(t, `argument "dst" registered after repeated argument "src"`, func() {
		app := NewApp("prog", "")
		app.ArgRepeat("src", "", String)
		app.Arg("dst", "", String)
	})
	wantPanic(t, "FixedList: n must be >= 2", func() { FixedList(1, Int) })
}
