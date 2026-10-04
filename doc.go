// Package az parses command lines with a kingpin-style builder API built on
// generics. A flag or argument is typed by the converter that parses it, so
// values come back as T with no type assertions, code generation, or
// reflection.
//
// az is unstable: the API may change in any release before v1.
// It requires Go 1.27 or newer, for generic methods.
//
//	app := az.NewApp("greet", "Print a greeting.")
//	name := app.Flag("name", "Who to greet.", az.String).Default("world").Ref()
//	times := app.Flag("times", "How many times.", az.Int).Default(1).Ref()
//	app.Alias("n", "name")
//	app.MustParse(os.Args[1:])
//
//	for range times.Get() {
//		fmt.Printf("Hello, %s!\n", name.Get())
//	}
//
// # Converters
//
// A [Conv] converts between strings and T, and suggests completions. It
// knows nothing about the command line. Built-in converters are [Bool],
// [String], [Int], [Int64], [Float64], [Duration], and [IP]. Combinators
// build new ones: [Enum], [Separator] (--tags a,b), [FixedList]
// (--point 1 2 3), [Greedy] (--ports 80 443), and [Until]
// (-exec grep foo ;).
//
// A converter decides how many value tokens its flag consumes by which
// interfaces it implements: [ZeroArgConv] for none, [MultiValueConv] for
// several, and one otherwise. Arity is declared, never discovered by
// converting, so parsing, completion, and help agree on it.
//
// # Flags, arguments, and commands
//
// Flag, FlagRepeat, FlagCounter, OnFlag, Arg, ArgRepeat, and Cmd are
// methods on both [App] and [Command], so commands nest to any depth.
// Flags of enclosing scopes stay in effect after a command name.
//
// OnFlag calls a function for every occurrence in command-line order, for
// programs like find where the order of flags matters.
//
// # Flag syntax
//
// A [Matcher] decides what a flag looks like. Apps use GNU syntax by
// default; [FindMatcher], [WindowsSlashMatcher], and [GCCMatcher] give
// -name, /Name:value, and -Ipath styles. Matching is driven by registered
// keys: a token is a flag only if it spells one, so "-1" and "/usr/bin" stay
// positional. An unregistered "--flag" is treated as a positional argument
// as well.
//
// # Help and completion
//
// Every app has a help flag. [App.ParseArgs] returns [ErrHelp] for it, and
// [App.MustParse] prints [App.WriteHelp] output and exits. [App.Complete]
// returns completion candidates for a partial command line without running
// any converters or callbacks.
package az
