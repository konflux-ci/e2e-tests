package framework

import (
	ginkgo "github.com/onsi/ginkgo/v2"
)

// CommonSuiteDescribe annotates the common tests with the application label.
func CommonSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[common-suite "+text+"]", args, ginkgo.Ordered)
}

func BuildSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[build-service-suite "+text+"]", args)
}

func JVMBuildSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[jvm-build-service-suite "+text+"]", args, ginkgo.Ordered)
}

func MultiPlatformBuildSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[multi-platform-build-service-suite "+text+"]", args, ginkgo.Ordered)
}

func EnterpriseContractSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[enterprise-contract-suite "+text+"]", args, ginkgo.Ordered)
}

func UpgradeSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[upgrade-suite "+text+"]", args, ginkgo.Ordered)
}

func TknBundleSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[task-suite "+text+"]", args, ginkgo.Ordered)
}

func DisasterRecoverySuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[disaster-recovery "+text+"]", args, ginkgo.Ordered)
}
