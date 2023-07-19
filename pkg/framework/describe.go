package framework

import (
	. "github.com/onsi/ginkgo/v2"
)

// E2ESuiteDescribe annotates the e2e scenarios tests with the e2e-scenarios label.
func E2ESuiteDescribe(args ...interface{}) bool {
	return Describe("[e2e-demos-suite]", args)
}

// CommonSuiteDescribe annotates the common tests with the application label.
func CommonSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[common-suite "+text+"]", args, Ordered)
}

func ChainsSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[chains-suite "+text+"]", args, Ordered)
}

func BuildSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[build-service-suite "+text+"]", args)
}

func JVMBuildSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[jvm-build-service-suite "+text+"]", args, Ordered)
}

func ClusterRegistrationSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[cluster-registration-suite "+text+"]", args, Ordered)
}

func ReleaseSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[release-suite "+text+"]", args, Ordered)
}

func IntegrationServiceSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[integration-service-suite "+text+"]", args, Ordered)
}

func RhtapDemoSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[rhtap-demo-suite "+text+"]", args, Ordered)
}

func O11ySuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[O11y-suite "+text+"]", args, Ordered)
}

func SPISuiteDescribe(args ...interface{}) bool {
	return Describe("[spi-suite]", args, Ordered)
}

func EnterpriseContractSuiteDescribe(text string, args ...interface{}) bool {
	return Describe("[enterprise-contract-suite "+text+"]", args, Ordered)
}
