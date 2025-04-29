package util

// Zero returns the zero value of any type `T`.
func Zero[T any]() T {
	return *new(T)
}

// ValueOrDefault returns `val` if it is not the zero value of its type.
// Otherwise, it returns the provided default value `def`.
func ValueOrDefault[T comparable](val T, def T) T {
	zero := Zero[T]()

	if val != zero {
		return val
	}

	return def
}
