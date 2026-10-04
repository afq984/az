# az

[![Go Reference](https://pkg.go.dev/badge/github.com/afq984/az.svg)](https://pkg.go.dev/github.com/afq984/az)

A command-line parser for Go with a kingpin-style builder API, built on
generics. A flag or argument is typed by the converter that parses it, so
values come back as `T`: no type assertions, no code generation, no
reflection.

> [!WARNING]
> **Unstable.** az is pre-v1 and the API is still being designed. Expect
> breaking changes in any release. See [ROADMAP.md](ROADMAP.md) for open
> questions.

Requires **Go 1.27** or newer: the API uses generic methods
([go.dev/issue/77273](https://go.dev/issue/77273)).

```
go get github.com/afq984/az
```

## Example

```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/afq984/az"
)

func main() {
	app := az.NewApp("fetch", "Download URLs.").Version("v0.1.0")
	app.Alias("h", "help")
	timeout := app.Flag("timeout", "Per-request timeout.", az.Duration).Default(30 * time.Second).Ref()
	retries := app.Flag("retries", "Attempts per URL.", az.Int).Default(3).Ref()
	format := app.Flag("format", "Output format.", az.Enum("json", "text")).Default("text").Ref()
	verbose := app.FlagCounter("verbose", "More logging; repeatable.").Ref()
	app.Alias("v", "verbose")
	urls := app.ArgRepeat("urls", "URLs to fetch.", az.String).Required().Ref()

	app.MustParse(os.Args[1:])

	fmt.Println(timeout.Get(), retries.Get(), format.Get(), verbose.Get(), urls.Get())
}
```

```
$ fetch --help
usage: fetch [<flags>] <urls>...

Download URLs.

Flags:
  -h, --help              Show help.
      --version           Show version.
      --timeout TIMEOUT   Per-request timeout. (default: 30s)
      --retries RETRIES   Attempts per URL. (default: 3)
      --format FORMAT     Output format. (default: text)
  -v, --verbose           More logging; repeatable.

Arguments:
  <urls>...   URLs to fetch.

$ fetch -vv --timeout=5s example.com
5s 3 text 2 [example.com]

$ fetch --retries many example.com
fetch: error: invalid value for --retries: strconv.Atoi: parsing "many": invalid syntax
Try 'fetch --help' for more information.
```

## Features

- **Typed values.** `app.Flag(name, help, conv)` infers `T` from the
  converter: `Default` takes a `T`, and `Ref().Get()` returns a `T`.
- **Composable converters.** Built-ins are `Bool`, `String`, `Int`, `Int64`,
  `Float64`, `Duration`, and `IP`. Combinators build more: `Enum`,
  `Separator` (`--tags a,b`), `FixedList` (`--point 1 2 3`), `Greedy`
  (`--ports 80 443`), and `Until` (`-exec grep foo ;`). A converter is just
  a `Conv[T]` interface, so it can be reused outside the command line.
- **Commands at any depth.** `Cmd` returns a `*Command` with the same
  registration methods as the app. Outer flags stay in effect after the
  command name.
- **Pluggable flag syntax.** GNU by default. `FindMatcher` handles `-name`,
  `WindowsSlashMatcher` handles `/Name:value`, and `GCCMatcher` handles
  `-Ipath`. Matching only recognizes registered keys, so `-1` and
  `/usr/bin` stay positional.
- **Order-dependent flags.** `OnFlag` calls a function for each occurrence,
  in order, for find-style programs.
- **Help, version, and completion.** Help is generated from registrations.
  `Complete` reports candidates for a partial command line without running
  any converter or callback.

## License

[MIT](LICENSE)
