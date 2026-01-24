package optional

import "fmt"

type Optional[T any] struct {
	value T
	set   bool
}

func New[T any](value T, set ...bool) Optional[T] {
	if len(set) > 0 {
		return Optional[T]{value: value, set: set[0]}
	}
	return Optional[T]{value: value, set: true}
}

func (o Optional[T]) IsSet() bool {
	return o.set
}

func (o Optional[T]) Value() T {
	return o.value
}

func (o Optional[T]) String() string {
	if o.set {
		return fmt.Sprintf("%v", o.value)
	}
	return "<UNSET>"
}
