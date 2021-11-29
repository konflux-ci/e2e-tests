package appservice

import "github.com/onsi/ginkgo"

const (
	ApplicationNamespace = "application-service"
)

// AppStudioDescribe annotates the test with the application label.
func HASDescribe(text string, body func()) bool {
	return ginkgo.Describe("[appstudio-has] "+text, body)
}
