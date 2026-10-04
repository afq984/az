package az

// flagEntry is the non-generic, type-erased flag descriptor.
// Closures capture the typed Ref and Conv, so the dispatch loop
// never needs generics.
type flagEntry struct {
	name      string
	help      string
	hidden    bool
	nargs     int                  // 0 = zero-arg, 1 = one value, n >= 2 = fixed multi, VariadicNArgs = until next flag / stop token
	stopToken string               // non-empty for variadic flags terminated by an exact token (find -exec ... ;)
	setValue  func(s string) error // single token or attached value
	setValues func(parts []string) error
	setTrue   func() // for zero-arg flags (bool, action)
	addCount  func() // for counter flags
	complete  func(partial string) []string
}

// argEntry is the non-generic positional argument descriptor.
type argEntry struct {
	name     string
	help     string
	required bool
	setValue func(s string) error
	complete func(partial string) []string
}

// FlagClause is a fluent builder for configuring a flag.
type FlagClause[T any] struct {
	entry *flagEntry
	ref   *Ref[T]
}

func (f *FlagClause[T]) Default(v T) *FlagClause[T] {
	f.ref.value = v
	f.ref.hasDefault = true
	return f
}

func (f *FlagClause[T]) Required() *FlagClause[T] {
	return f
}

func (f *FlagClause[T]) Hidden() *FlagClause[T] {
	f.entry.hidden = true
	return f
}

func (f *FlagClause[T]) Ref() *Ref[T] { return f.ref }

// ArgClause is a fluent builder for configuring a positional argument.
type ArgClause[T any] struct {
	entry *argEntry
	ref   *Ref[T]
}

func (a *ArgClause[T]) Default(v T) *ArgClause[T] {
	a.ref.value = v
	a.ref.hasDefault = true
	return a
}

func (a *ArgClause[T]) Required() *ArgClause[T] {
	a.entry.required = true
	return a
}

func (a *ArgClause[T]) Ref() *Ref[T] { return a.ref }
