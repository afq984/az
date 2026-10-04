package az

import "testing"

func TestWindowsSlash(t *testing.T) {
	app := NewApp("robocopy", "", WindowsSlashMatcher{})
	mir := app.Flag("MIR", "Mirror", Bool)
	retries := app.Flag("R", "Retries", Int)
	log := app.Flag("LOG", "Log file", String)
	paths := app.ArgRepeat("paths", "Paths", String)

	_, err := app.ParseArgs([]string{"/usr/src", "/MIR", "/R:5", "/LOG", "out.txt", "/dst"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, mir.Ref().Get(), true)
	eq(t, retries.Ref().Get(), 5)
	eq(t, log.Ref().Get(), "out.txt")
	sliceEq(t, paths.Ref().Get(), []string{"/usr/src", "/dst"}) // unregistered: positional

	app = NewApp("robocopy", "", WindowsSlashMatcher{})
	app.Flag("R", "Retries", Int)
	_, err = app.ParseArgs([]string{"/R:x"})
	wantErr(t, err, "invalid value for /R")
}

func TestGCCLongestPrefix(t *testing.T) {
	app := NewApp("cc", "", GCCMatcher{})
	includes := app.FlagRepeat("I", "Include dir", String)
	system := app.FlagRepeat("isystem", "System include dir", String)
	defines := app.FlagRepeat("D", "Define", String)
	opt := app.Flag("O", "Optimization", Int)
	files := app.ArgRepeat("files", "Sources", String)

	_, err := app.ParseArgs([]string{"-I/a", "-I", "/b", "-isystem/c", "-DFOO=1", "-O2", "main.c"})
	if err != nil {
		t.Fatal(err)
	}
	sliceEq(t, includes.Ref().Get(), []string{"/a", "/b"})
	sliceEq(t, system.Ref().Get(), []string{"/c"})
	sliceEq(t, defines.Ref().Get(), []string{"FOO=1"})
	eq(t, opt.Ref().Get(), 2)
	sliceEq(t, files.Ref().Get(), []string{"main.c"})
}

// A zero-arg key doesn't take a trailing value: -config is not -c "onfig".
func TestGCCZeroArgKeyNeedsExactMatch(t *testing.T) {
	app := NewApp("cc", "", GCCMatcher{})
	compile := app.Flag("c", "Compile only", Bool)
	files := app.ArgRepeat("files", "Sources", String)

	_, err := app.ParseArgs([]string{"-c", "-config"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, compile.Ref().Get(), true)
	sliceEq(t, files.Ref().Get(), []string{"-config"})
}

func TestGNUShortAttachedAndCluster(t *testing.T) {
	app := NewApp("tar", "")
	extract := app.Flag("x", "Extract", Bool)
	verbose := app.Flag("v", "Verbose", Bool)
	file := app.Flag("f", "Archive", String)

	_, err := app.ParseArgs([]string{"-xvfa.tar"})
	if err != nil {
		t.Fatal(err)
	}
	eq(t, extract.Ref().Get(), true)
	eq(t, verbose.Ref().Get(), true)
	eq(t, file.Ref().Get(), "a.tar")
}

func TestDisplayKeyPrefersEarlierMatcher(t *testing.T) {
	// GNULongMatcher can match a one-letter key too ("--v"), so with it
	// listed first, it wins the spelling.
	app := NewApp("prog", "", GNULongMatcher{}, GNUShortMatcher{})
	eq(t, app.displayKey("v"), "--v")
	eq(t, NewApp("prog", "").displayKey("v"), "-v")
	eq(t, NewApp("prog", "", EndOfFlagsMatcher{Token: "--"}).displayKey("v"), "v")
}
