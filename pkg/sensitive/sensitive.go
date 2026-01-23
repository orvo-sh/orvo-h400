package sensitive

type Sensitive[T any] struct {
	value T
}

func New[T any](value T) Sensitive[T] {
	return Sensitive[T]{value: value}
}

func (s Sensitive[T]) String() string {
	return "<redacted>"
}

func (s Sensitive[T]) GoString() string {
	return "<redacted>"
}

func (s Sensitive[T]) Value() T {
	return s.value
}
