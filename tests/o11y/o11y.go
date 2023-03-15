package o11y

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const (
	O11yUser = "o11y-e2e"
)

var _ = framework.O11ySuiteDescribe("O11Y E2E tests", Label("o11y", "HACBS"), Pending, func() {

	defer GinkgoRecover()
	var f *framework.Framework
	var err error

	Describe("O11y test", Pending, func() {
		var testNamespace string

		BeforeAll(func() {

			f, err = framework.NewFramework(O11yUser)
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

		})
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				// err := f.AsKubeAdmin.CommonController.DeleteAllPipelineRunsAndTasks(testNamespace)
				// Expect(err).NotTo(HaveOccurred())
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		})

		It("E2E sample test for upload 50MB pod", Pending, func() {
			podNameRegex := ".*-upload-50mb-pod"
			query := fmt.Sprintf("last_over_time(container_network_transmit_bytes_total{namespace='%s', pod=~'%s'}[1h])", testNamespace, podNameRegex)

			result, err := f.AsKubeAdmin.O11yController.GetMetrics(query)
			Expect(err).NotTo(HaveOccurred())

			podNamesWithSize, err := f.AsKubeAdmin.O11yController.GetRegexPodNameWithSize(podNameRegex, result)
			Expect(err).NotTo(HaveOccurred())

			for podName, podSize := range podNamesWithSize {
				GinkgoWriter.Printf("Pod: %s, Size: %.2f MB", podName, podSize)
				// Range limits are measured as part of STONEO11Y-15
				Expect(podSize).To(And(
					BeNumerically(">=", 53.00),
					BeNumerically("<=", 53.59),
				), fmt.Sprintf("%s: %.2f MB is not within the expected range.", podName, podSize))
			}

		})
	})

})
