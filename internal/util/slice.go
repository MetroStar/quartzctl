package util

// ToInterfaceSlice converts a typed slice to a slice of `interface{}`.
func ToInterfaceSlice[T any](s []T) []interface{} {
	res := make([]interface{}, len(s))

	for i, v := range s {
		res[i] = v
	}

	return res
}

// ToTypedSlice converts a slice of `interface{}` to a typed slice.
func ToTypedSlice[T any](s []interface{}) []T {
	res := make([]T, len(s))

	for i, v := range s {
		res[i] = v.(T)
	}

	return res
}

// DistinctSlice returns a new slice with duplicate entries removed.
// The input slice must contain elements of a comparable type.
func DistinctSlice[T comparable](s []T) []T {
	var res []T
	found := make(map[T]bool)

	for _, v := range s {
		if _, ok := found[v]; ok {
			continue
		}

		found[v] = true
		res = append(res, v)
	}

	return res
}
