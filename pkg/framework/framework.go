package framework

import g "github.com/onsi/ginkgo/v2"

// HASSuiteDescribe annotates the application service tests with the application label.
func HASSuiteDescribe(text string, body func()) bool {
	return g.Describe("[has-suite "+text+"]", body)
}

// CommonSuiteDescribe annotates the common tests with the application label.
func CommonSuiteDescribe(text string, body func()) bool {
	return g.Describe("[common-suite "+text+"]", body)
}
