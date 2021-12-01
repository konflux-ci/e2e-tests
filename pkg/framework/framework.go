package framework

import "github.com/onsi/ginkgo/v2"

// HASSuiteDescribe annotates the application service tests with the application label.
func HASSuiteDescribe(text string, body func()) bool {
	return ginkgo.Describe("[has-suite] "+text, body)
}

// CommonSuiteDescribe annotates the common tests with the application label.
func CommonSuiteDescribe(text string, body func()) bool {
	return ginkgo.Describe("[common-suite] "+text, body)
}
