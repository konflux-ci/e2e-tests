package framework

import (
	ginkgo "github.com/onsi/ginkgo/v2"
)

func EnterpriseContractSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[enterprise-contract-suite "+text+"]", args, ginkgo.Ordered)
}

func UpgradeSuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[upgrade-suite "+text+"]", args, ginkgo.Ordered)
}

func DisasterRecoverySuiteDescribe(text string, args ...interface{}) bool {
	return ginkgo.Describe("[disaster-recovery "+text+"]", args, ginkgo.Ordered)
}
