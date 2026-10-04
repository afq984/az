package az_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/afq984/az"
)

func Example() {
	app := az.NewApp("greet", "Print a greeting.")
	name := app.Flag("name", "Who to greet.", az.String).Default("world").Ref()
	times := app.Flag("times", "How many times.", az.Int).Default(1).Ref()
	app.Alias("n", "name")

	if _, err := app.ParseArgs([]string{"-n", "gopher", "--times=2"}); err != nil {
		fmt.Println(err)
		return
	}
	for range times.Get() {
		fmt.Printf("Hello, %s!\n", name.Get())
	}
	// Output:
	// Hello, gopher!
	// Hello, gopher!
}

func Example_commands() {
	app := az.NewApp("todo", "Track things to do.")
	verbose := app.Flag("verbose", "Verbose output.", az.Bool).Ref()

	add := app.Cmd("add", "Add an item.")
	priority := add.Flag("priority", "Priority.", az.Enum("low", "high")).Default("low").Ref()
	words := add.ArgRepeat("words", "Item text.", az.String).Required().Ref()

	done := app.Cmd("done", "Mark an item done.")
	id := done.Arg("id", "Item number.", az.Int).Required().Ref()

	for _, args := range [][]string{
		{"add", "--priority", "high", "buy", "milk", "--verbose"},
		{"done", "3"},
	} {
		cmd, err := app.ParseArgs(args)
		if err != nil {
			fmt.Println(err)
			return
		}
		switch cmd {
		case "add":
			fmt.Printf("add %q (%s, verbose=%t)\n", strings.Join(words.Get(), " "), priority.Get(), verbose.Get())
		case "done":
			fmt.Printf("done #%d\n", id.Get())
		}
	}
	// Output:
	// add "buy milk" (high, verbose=true)
	// done #3
}

// Order-dependent flags, find style: one dash, full words, and an -exec
// that consumes raw tokens up to ";".
func Example_find() {
	app := az.NewApp("find", "Search for files.", az.FindMatcher{})
	app.Arg("path", "Where to start.", az.String).Default(".")
	app.OnFlag("name", "Match a name.", az.String, func(pattern string) error {
		fmt.Println("filter: name", pattern)
		return nil
	})
	app.OnFlag("exec", "Run a command.", az.Until(";", az.String), func(argv []string) error {
		fmt.Println("action: exec", argv)
		return nil
	})

	_, err := app.ParseArgs([]string{"src", "-name", "*.go", "-exec", "gofmt", "-l", "{}", ";"})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// filter: name *.go
	// action: exec [gofmt -l {}]
}

func ExampleApp_WriteHelp() {
	app := az.NewApp("serve", "Serve files over HTTP.").Version("v0.1.0")
	app.Alias("h", "help")
	app.Flag("addr", "Listen address.", az.String).Default(":8080")
	app.Flag("timeout", "Request timeout.", az.Duration).PlaceHolder("DUR")
	app.FlagCounter("verbose", "More logging; repeatable.")
	app.Alias("v", "verbose")
	app.Arg("root", "Directory to serve.", az.String).Default(".")

	app.WriteHelp(os.Stdout, "")
	// Output:
	// usage: serve [<flags>] [<root>]
	//
	// Serve files over HTTP.
	//
	// Flags:
	//   -h, --help          Show help.
	//       --version       Show version.
	//       --addr ADDR     Listen address. (default: :8080)
	//       --timeout DUR   Request timeout.
	//   -v, --verbose       More logging; repeatable.
	//
	// Arguments:
	//   <root>   Directory to serve. (default: .)
}

func ExampleApp_Complete() {
	app := az.NewApp("deploy", "")
	app.Flag("env", "Target environment.", az.Enum("dev", "staging", "prod"))
	app.Flag("dry-run", "Show what would happen.", az.Bool)

	fmt.Println(app.Complete([]string{"--env"}, "s").Values)
	fmt.Println(app.Complete(nil, "--d").Flags)
	// Output:
	// [staging]
	// [--dry-run]
}
