package slices

// TODO review this package after migration to Go 1.21

// Filter returns a new slice containing only the elements of slice that satisfy the test function.
func Filter[T any](slice []T, testFunction func(T) bool) (ret []T) {
	for _, s := range slice {
		if testFunction(s) {
			ret = append(ret, s)
		}
	}
	return
}

// Intersection returns the number of elements intersection between two slices with respect to the order,
// i.e. Intersection(["a", "b", "c"], ["a", "b", "d"]) == 2 but Intersection(["a", "b", "c"], ["b", "a", "c"]) == 0
func Intersection(s1, s2 []string) int {
	var count int
	for i, e1 := range s1 {
		if i < len(s2) && e1 == s2[i] {
			count++
		} else {
			break
		}
	}
	return count
}
