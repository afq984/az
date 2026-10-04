package az

// Ref holds the parsed value of a flag or positional argument.
type Ref[T any] struct {
	value T
	isSet bool
}

// Get returns the parsed value, or the default if the flag or argument was
// not given.
func (r *Ref[T]) Get() T { return r.value }

// IsSet reports whether the value was given on the command line.
func (r *Ref[T]) IsSet() bool { return r.isSet }
