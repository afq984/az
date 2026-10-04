package az

import (
	"strings"
	"testing"
)

func helpOf(t *testing.T, app *App, path string) string {
	t.Helper()
	var b strings.Builder
	if err := app.WriteHelp(&b, path); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func gitApp() *App {
	app := NewApp("git", "The stupid content tracker.").Version("2.0")
	app.Alias("h", "help")
	app.FlagCounter("verbose", "Be more verbose.")
	app.Alias("v", "verbose")
	app.Flag("git-dir", "Path to the repository.", String).Default(".git").PlaceHolder("PATH")
	app.Flag("debug-internal", "Internal.", Bool).Hidden()

	clone := app.Cmd("clone", "Clone a repository.")
	clone.Flag("depth", "Create a shallow clone.", Int)
	clone.Flag("branch", "Branch to check out.", String).Required()
	clone.Alias("b", "branch")
	clone.Arg("repository", "Repository to clone.", String).Required()
	clone.Arg("directory", "Target directory.", String)

	remote := app.Cmd("remote", "Manage remotes.")
	remote.Cmd("add", "Add a remote.")
	remote.Cmd("remove", "Remove a remote.")
	app.Cmd("fsck-internal", "").Hidden()

	app.FlagRepeat("config", "Set a config value.", String).PlaceHolder("KEY=VALUE")
	app.Flag("point", "Coordinates.", FixedList(2, Int)).PlaceHolder("N")
	app.Flag("ports", "Ports.", Greedy(Int))
	app.Flag("exec", "Run a command.", Until(";", String)).PlaceHolder("CMD")
	return app
}

func TestHelpApp(t *testing.T) {
	got := helpOf(t, gitApp(), "")
	want := `usage: git [<flags>] <command> [<args> ...]

The stupid content tracker.

Flags:
  -h, --help               Show help.
      --version            Show version.
  -v, --verbose            Be more verbose.
      --git-dir PATH       Path to the repository. (default: .git)
      --config KEY=VALUE   Set a config value.
      --point N N          Coordinates.
      --ports PORTS...     Ports.
      --exec CMD... ;      Run a command.

Commands:
  clone    Clone a repository.
  remote   Manage remotes.
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestHelpCommand(t *testing.T) {
	got := helpOf(t, gitApp(), "clone")
	want := `usage: git clone --branch BRANCH [<flags>] <repository> [<directory>]

Clone a repository.

Flags:
      --depth DEPTH        Create a shallow clone.
  -b, --branch BRANCH      Branch to check out.
  -h, --help               Show help.
      --version            Show version.
  -v, --verbose            Be more verbose.
      --git-dir PATH       Path to the repository. (default: .git)
      --config KEY=VALUE   Set a config value.
      --point N N          Coordinates.
      --ports PORTS...     Ports.
      --exec CMD... ;      Run a command.

Arguments:
  <repository>   Repository to clone.
  <directory>    Target directory.
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestHelpNestedCommand(t *testing.T) {
	app := gitApp()
	got := helpOf(t, app, "remote")
	if !strings.HasPrefix(got, "usage: git remote [<flags>] <command> [<args> ...]\n\nManage remotes.\n") {
		t.Errorf("got:\n%s", got)
	}
	got = helpOf(t, app, "remote add")
	if !strings.HasPrefix(got, "usage: git remote add [<flags>]\n\nAdd a remote.\n") {
		t.Errorf("got:\n%s", got)
	}
}

func TestHelpUnknownCommand(t *testing.T) {
	err := gitApp().WriteHelp(&strings.Builder{}, "remote rename")
	wantErr(t, err, `unknown command "remote rename"`)
}

func TestHelpWindowsSlash(t *testing.T) {
	app := NewApp("robocopy", "", WindowsSlashMatcher{})
	app.Flag("MIR", "Mirror a directory tree.", Bool)
	app.Flag("R", "Retries.", Int).Default(3)
	app.ArgRepeat("paths", "Source, destination, files.", String)

	got := helpOf(t, app, "")
	want := `usage: robocopy [<flags>] [<paths>...]

Flags:
  /help   Show help.
  /MIR    Mirror a directory tree.
  /R R    Retries. (default: 3)

Arguments:
  <paths>...   Source, destination, files.
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
