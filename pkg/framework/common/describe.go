package common

import "github.com/onsi/ginkgo"

// commonDescribe annotates the test with the application label.
func commonDescribe(text string, body func()) bool {
	return ginkgo.Describe("[appstudio-common] "+text, body)
}
