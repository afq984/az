package az

import "strings"

// CompleteResult lists completion candidates for one word.
type CompleteResult struct {
	Flags    []string // flag spellings, e.g. "--verbose", "-v"
	Values   []string // values for the pending flag or the next positional argument
	Commands []string // subcommand names
}

// Complete returns candidates for the word partial, given the complete
// tokens before it. It replays tokens through the same state machine as
// ParseArgs, but runs no converters, setters, or OnFlag callbacks, and
// tolerates invalid input. Candidates are in registration order, except
// that flags of the selected command come before inherited ones.
func (a *App) Complete(tokens []string, partial string) CompleteResult {
	w := a.newWalker(true)
	for _, tok := range tokens {
		_ = w.step(tok) // best effort: keep going past invalid tokens
	}

	var res CompleteResult
	if e := w.pending; e != nil {
		if e.complete != nil {
			res.Values = e.complete(partial)
		}
		// A greedy flag that has a value ends at the next flag.
		if e.nargs == VariadicNArgs && e.stopToken == "" && len(w.pendingTokens) > 0 {
			res.Flags = w.flagCandidates(partial)
		}
		return res
	}
	if !w.forcePositional {
		res.Flags = w.flagCandidates(partial)
		for _, c := range w.cur().cmds {
			if !c.hidden && strings.HasPrefix(c.name, partial) {
				res.Commands = append(res.Commands, c.name)
			}
		}
	}
	if args := w.cur().args; w.posIdx < len(args) && args[w.posIdx].complete != nil {
		res.Values = args[w.posIdx].complete(partial)
	}
	return res
}

// flagCandidates returns the spellings of visible flags in effect that
// start with partial.
func (w *walker) flagCandidates(partial string) []string {
	var out []string
	for _, e := range w.flags() {
		if e.hidden {
			continue
		}
		for _, k := range w.keys(e) {
			if s := w.app.displayKey(k); strings.HasPrefix(s, partial) {
				out = append(out, s)
			}
		}
	}
	return out
}
