package framework

import . "github.com/onsi/ginkgo/v2"

// HASSuiteDescribe annotates the application service tests with the application label.
func HASSuiteDescribe(text string, body func()) bool {
	return Describe("[has-suite "+text+"]", Ordered, body)
}

// CommonSuiteDescribe annotates the common tests with the application label.
func CommonSuiteDescribe(text string, body func()) bool {
	return Describe("[common-suite "+text+"]", Ordered, body)
}
