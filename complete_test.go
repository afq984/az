package az

import "testing"

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
	sliceEq(t, res.Flags, []string{"--verbose"})
}

// Completion replays tokens dry: no converters, setters, or callbacks run,
// so completing "find -exec rm {} ;" never executes anything.
func TestCompleteRunsNoSideEffects(t *testing.T) {
	app := NewApp("find", "", FindMatcher{})
	calls := 0
	app.OnFlag("exec", "Command", Until(";", String), func([]string) error {
		calls++
		return nil
	})
	name := app.Flag("name", "Pattern", String)
	count := app.FlagCounter("v", "Verbosity")

	app.Complete([]string{"-name", "*.go", "-v", "-exec", "rm", "{}", ";"}, "")
	eq(t, calls, 0)
	eq(t, name.Ref().IsSet(), false)
	eq(t, count.Ref().Get(), 0)
}

func TestCompleteThroughShortCluster(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("all", "", Bool)
	app.Alias("a", "all")
	app.Flag("format", "", Enum("json", "jsonl", "text"))
	app.Alias("f", "format")

	res := app.Complete([]string{"-af"}, "js")
	sliceEq(t, res.Values, []string{"json", "jsonl"})
}

func TestCompleteFlagsInRegistrationOrder(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("verbose", "", Bool)
	app.Alias("v", "verbose")
	app.Flag("version-check", "", Bool)
	app.Flag("hidden", "", Bool).Hidden()

	res := app.Complete(nil, "-")
	sliceEq(t, res.Flags, []string{"--help", "--verbose", "-v", "--version-check"})
	res = app.Complete(nil, "--ver")
	sliceEq(t, res.Flags, []string{"--verbose", "--version-check"})
}

func TestCompleteCommandsAndArgs(t *testing.T) {
	app := NewApp("prog", "")
	app.Cmd("build", "")
	app.Cmd("bench", "").Hidden()
	run := app.Cmd("run", "")
	run.Flag("fast", "", Bool)
	run.Arg("mode", "", Enum("debug", "release"))

	res := app.Complete(nil, "b")
	sliceEq(t, res.Commands, []string{"build"})

	res = app.Complete([]string{"run"}, "")
	sliceEq(t, res.Flags, []string{"--fast", "--help"}) // innermost scope first
	sliceEq(t, res.Values, []string{"debug", "release"})
	sliceEq(t, res.Commands, nil)
}

// Live parsing ends a greedy flag at the next flag once it has a value, so
// completion offers flags there too.
func TestCompleteAfterGreedyValue(t *testing.T) {
	app := NewApp("prog", "")
	app.Flag("ports", "", Greedy(Enum("80", "443")))
	app.Flag("verbose", "", Bool)

	res := app.Complete([]string{"--ports"}, "")
	sliceEq(t, res.Flags, nil) // a value is required first
	sliceEq(t, res.Values, []string{"80", "443"})

	res = app.Complete([]string{"--ports", "80"}, "--v")
	sliceEq(t, res.Flags, []string{"--verbose"})

	res = app.Complete([]string{"--ports", "80"}, "4")
	sliceEq(t, res.Values, []string{"443"})
}

func TestCompleteAfterEndOfFlags(t *testing.T) {
	app := NewApp("prog", "")
	app.ArgRepeat("files", "", Enum("a.txt", "b.txt"))

	res := app.Complete([]string{"--"}, "")
	sliceEq(t, res.Flags, nil)
	sliceEq(t, res.Values, []string{"a.txt", "b.txt"})
}
