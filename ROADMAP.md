# Roadmap

az is usable but unstable. This file tracks open design questions and known
gaps. Items in each section are roughly in priority order.

## Open design questions

These may change the API, so they should be settled before a v0.1.0 tag.

1. **Unknown long flags.** Matching only recognizes registered keys. That
   keeps `-1` and `/usr/bin` positional, but a typo like `--verbsoe` also
   becomes a positional argument, or an "unexpected argument" error when no
   positional slot is left. Options: reject unregistered `--x` tokens under
   GNU matchers, add a strict mode, or suggest the nearest key.
2. **Missing subcommand.** When a scope has commands and none is given,
   ParseArgs returns the parent path with no error. Should this be an error,
   a default command, or stay the caller's job?
3. **Single-use parsing.** ParseArgs writes into Refs and per-flag state in
   place, so parsing the same App twice accumulates. A parse-result value,
   or a reset at the start of each parse, would lift this.
4. **Value access.** `app.Flag(...).Ref().Get()` is wordy. Options:
   registration returns `*Ref[T]` with configuration passed differently,
   or clauses get a `Get()`.
5. **OnFlag returns nothing.** As a result, callback flags can't be Hidden,
   Required, or given a PlaceHolder.
6. **Args and commands in one scope.** Kingpin forbids this. az allows it,
   and a token that names a command always selects the command.
7. **Interspersed is app-wide.** Some tools want POSIX parsing for one
   command only (an `exec`-style subcommand).

## Known gaps

- **Registration checks.** A required argument after an optional one, or a
  Greedy flag in a scope with positionals, is accepted but can't behave
  usefully. Arity is declared metadata, so both can be caught at
  registration.
- **Environment variables and config files.** A `Conv` is independent of
  the source, so something like `.Envar("NAME")` could reuse `FromString`.
- **Shell completion scripts.** bash, zsh, and fish glue on top of
  `App.Complete`. Completing attached values (`--format=js`) is also missing.
- **Typed errors.** Parse errors are plain `fmt` errors. Callers can't tell
  an unknown flag from an invalid value without matching strings.
- **Help formatting.** No line wrapping or terminal-width awareness, and no
  templates. Command help lists inherited flags together with the
  command's own flags, without a separate "global flags" group.
- **Custom converters.** A helper like `ConvFunc(from, to)` would save
  writing a three-method type for a simple custom type.
- **Converter set.** `IP` returns `net.IP`; `netip.Addr` is the modern
  choice. There are no unsigned-integer converters.
- **Short clusters.** The dispatcher re-matches the rest of a cluster as
  `"-" + rest`, which bakes a GNU assumption into a matcher-agnostic core.

## Releasing

- The module requires Go 1.27 (generic methods). CI tests on the minimum
  1.27.x and on the latest stable Go.
- Tag v0.x.y releases only. Any v0 release may break the API, and release
  notes should say how.
