package az

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

// WriteHelp writes help for the command at path, as returned by ParseArgs:
// "" for the app itself, "remote add" for a nested command.
func (a *App) WriteHelp(out io.Writer, path string) error {
	w := a.newWalker(true)
	desc := a.help
	for _, name := range strings.Fields(path) {
		cmd, ok := w.cur().commands[name]
		if !ok {
			return fmt.Errorf("unknown command %q", path)
		}
		w.enter(name, cmd)
		desc = cmd.help
	}
	s := w.cur()

	var flags []*flagEntry
	padShort := false // some flag pairs a short key with a long one: -v, --verbose
	for _, e := range w.flags() {
		if e.hidden {
			continue
		}
		flags = append(flags, e)
		keys := w.keys(e)
		padShort = padShort || (slices.ContainsFunc(keys, isShort) && slices.ContainsFunc(keys, isLong))
	}
	var cmds []*Command
	for _, c := range s.cmds {
		if !c.hidden {
			cmds = append(cmds, c)
		}
	}

	var b strings.Builder
	usage := append([]string{"usage:", a.name}, w.path...)
	for _, e := range flags {
		if e.required {
			usage = append(usage, a.displayKey(w.keys(e)[0])+valueUsage(e))
		}
	}
	if len(flags) > 0 {
		usage = append(usage, "[<flags>]")
	}
	for _, arg := range s.args {
		usage = append(usage, argUsage(arg))
	}
	if len(cmds) > 0 {
		usage = append(usage, "<command> [<args> ...]")
	}
	fmt.Fprintln(&b, strings.Join(usage, " "))
	if desc != "" {
		fmt.Fprintf(&b, "\n%s\n", desc)
	}

	if len(flags) > 0 {
		var rows [][2]string
		for _, e := range flags {
			rows = append(rows, [2]string{
				a.flagSpelling(w.keys(e), padShort) + valueUsage(e),
				withDefault(e.help, e.defaultText),
			})
		}
		writeSection(&b, "Flags:", rows)
	}
	if len(s.args) > 0 {
		var rows [][2]string
		for _, arg := range s.args {
			name := "<" + arg.name + ">"
			if arg.repeat {
				name += "..."
			}
			rows = append(rows, [2]string{name, withDefault(arg.help, arg.defaultText)})
		}
		writeSection(&b, "Arguments:", rows)
	}
	if len(cmds) > 0 {
		var rows [][2]string
		for _, c := range cmds {
			rows = append(rows, [2]string{c.name, c.help})
		}
		writeSection(&b, "Commands:", rows)
	}

	_, err := io.WriteString(out, b.String())
	return err
}

func isShort(key string) bool { return utf8.RuneCountInString(key) == 1 }
func isLong(key string) bool  { return !isShort(key) }

// flagSpelling lists a flag's keys, short ones first: "-v, --verbose".
// With padShort, a flag without a short key is indented to line up.
func (a *App) flagSpelling(keys []string, padShort bool) string {
	var short, long []string
	for _, k := range keys {
		if isShort(k) {
			short = append(short, a.displayKey(k))
		} else {
			long = append(long, a.displayKey(k))
		}
	}
	s := strings.Join(append(short, long...), ", ")
	if padShort && len(short) == 0 {
		s = "    " + s
	}
	return s
}

// valueUsage describes the value tokens a flag consumes: " FILE",
// " X X X", " PORTS...", " CMD... ;".
func valueUsage(e *flagEntry) string {
	ph := e.placeholder
	if ph == "" {
		ph = strings.ToUpper(strings.ReplaceAll(e.name, "-", "_"))
	}
	switch {
	case e.nargs == 0:
		return ""
	case e.nargs == 1:
		return " " + ph
	case e.nargs > 1:
		return strings.Repeat(" "+ph, e.nargs)
	case e.stopToken != "":
		return " " + ph + "... " + e.stopToken
	default:
		return " " + ph + "..."
	}
}

func argUsage(arg *argEntry) string {
	s := "<" + arg.name + ">"
	if arg.repeat {
		s += "..."
	}
	if !arg.required {
		s = "[" + s + "]"
	}
	return s
}

func withDefault(help, def string) string {
	switch {
	case def == "":
		return help
	case help == "":
		return "(default: " + def + ")"
	}
	return help + " (default: " + def + ")"
}

func writeSection(b *strings.Builder, title string, rows [][2]string) {
	width := 0
	for _, r := range rows {
		width = max(width, utf8.RuneCountInString(r[0]))
	}
	fmt.Fprintf(b, "\n%s\n", title)
	for _, r := range rows {
		if r[1] == "" {
			fmt.Fprintf(b, "  %s\n", r[0])
			continue
		}
		fmt.Fprintf(b, "  %-*s   %s\n", width, r[0], r[1])
	}
}
