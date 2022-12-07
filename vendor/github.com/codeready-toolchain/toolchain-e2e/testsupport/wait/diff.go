package wait

import (
	"github.com/google/go-cmp/cmp"
)

func Diff(expected, actual interface{}) string {
	return cmp.Diff(expected, actual)
}
