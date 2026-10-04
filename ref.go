package az

// Ref is a type-safe container for a parsed CLI value.
type Ref[T any] struct {
	value      T
	isSet      bool
	hasDefault bool
}

func (r *Ref[T]) Get() T      { return r.value }
func (r *Ref[T]) IsSet() bool { return r.isSet }
