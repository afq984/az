package az

// flagEntry is the type-erased flag descriptor. Closures capture the typed
// Ref and Conv, so dispatch never needs generics.
type flagEntry struct {
	name        string
	keys        []string // name, then aliases
	help        string
	placeholder string
	defaultText string
	hidden      bool
	required    bool
	seen        bool // given on the command line

	nargs     int    // 0 = no value, 1 = one value, n >= 2 = exactly n, VariadicNArgs = until the next flag or stopToken
	stopToken string // non-empty for variadic flags terminated by an exact token (find -exec ... ;)

	setValue  func(s string) error       // one value token, or an attached value of any arity
	setValues func(parts []string) error // the value tokens of a multi-token flag
	setTrue   func() error               // an occurrence of a zero-arg flag
	complete  func(partial string) []string
}

// argEntry is the type-erased positional argument descriptor.
type argEntry struct {
	name        string
	help        string
	defaultText string
	required    bool
	repeat      bool // consumes all remaining positionals
	seen        bool

	setValue func(s string) error
	complete func(partial string) []string
}

// FlagClause configures a flag and gives access to its value.
type FlagClause[T any] struct {
	entry  *flagEntry
	ref    *Ref[T]
	format func(T) string
}

// Default sets the value reported when the flag is not given. For repeated
// flags, the first occurrence replaces the default rather than appending
// to it.
func (f *FlagClause[T]) Default(v T) *FlagClause[T] {
	f.ref.value = v
	f.entry.defaultText = f.format(v)
	return f
}

// Required makes parsing fail unless the flag is given. A default does not
// satisfy it.
func (f *FlagClause[T]) Required() *FlagClause[T] {
	f.entry.required = true
	return f
}

// Hidden omits the flag from help and completion.
func (f *FlagClause[T]) Hidden() *FlagClause[T] {
	f.entry.hidden = true
	return f
}

// PlaceHolder sets the value name shown in help. It defaults to the flag
// name in upper case with dashes as underscores: --git-dir GIT_DIR.
func (f *FlagClause[T]) PlaceHolder(name string) *FlagClause[T] {
	f.entry.placeholder = name
	return f
}

// Ref returns the flag's value container.
func (f *FlagClause[T]) Ref() *Ref[T] { return f.ref }

// ArgClause configures a positional argument and gives access to its value.
type ArgClause[T any] struct {
	entry  *argEntry
	ref    *Ref[T]
	format func(T) string
}

// Default sets the value reported when the argument is not given.
func (a *ArgClause[T]) Default(v T) *ArgClause[T] {
	a.ref.value = v
	a.entry.defaultText = a.format(v)
	return a
}

// Required makes parsing fail unless the argument is given. For ArgRepeat,
// at least one value is required.
func (a *ArgClause[T]) Required() *ArgClause[T] {
	a.entry.required = true
	return a
}

// Ref returns the argument's value container.
func (a *ArgClause[T]) Ref() *Ref[T] { return a.ref }
