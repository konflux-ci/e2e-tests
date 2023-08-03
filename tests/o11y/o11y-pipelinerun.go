package o11y

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

var _ = framework.O11ySuiteDescribe("O11Y E2E tests for Pipelinerun", Label("o11y", "HACBS"), Pending, func() {

	defer GinkgoRecover()
	var f *framework.Framework
	var err error

	Describe("O11y test", func() {
		var testNamespace string

		BeforeAll(func() {

			f, err = framework.NewFramework(O11yUserPipelineruns)
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			// Get Quay Token from ENV
			quayToken := utils.GetEnv("QUAY_TOKEN", "")
			Expect(quayToken).ToNot(BeEmpty())

			_, err := f.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(o11yUserSecret, testNamespace, quayToken)
			Expect(err).ToNot(HaveOccurred())

			err = f.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(testNamespace, o11yUserSecret, O11ySA, true)
			Expect(err).ToNot(HaveOccurred())

		})
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).To(BeTrue())
			}
		})

		It("E2E test to measure Egress pod by pushing images to quay - PipelineRun", func() {

			// Get Quay Organization from ENV
			quayOrg := utils.GetEnv("QUAY_E2E_ORGANIZATION", "")
			Expect(quayOrg).ToNot(BeEmpty())

			pipelineRun, err := f.AsKubeAdmin.O11yController.QuayImagePushPipelineRun(quayOrg, o11yUserSecret, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Wait for the pipeline run to succeed
			Expect(f.AsKubeAdmin.TektonController.WatchPipelineRunSucceeded(pipelineRun.Name, testNamespace, pipelineRunTimeout)).To(Succeed())

			podNameRegex := ".*-buildah-quay-pod"
			query := fmt.Sprintf("last_over_time(container_network_transmit_bytes_total{namespace='%s', pod=~'%s'}[1h])", testNamespace, podNameRegex)
			result, err := f.AsKubeAdmin.O11yController.GetMetrics(query)
			Expect(err).NotTo(HaveOccurred())

			podNamesWithResult, err := f.AsKubeAdmin.O11yController.GetRegexPodNameWithResult(podNameRegex, result)
			Expect(err).NotTo(HaveOccurred())

			podNameWithMB, err := f.AsKubeAdmin.O11yController.ConvertValuesToMB(podNamesWithResult)
			Expect(err).NotTo(HaveOccurred())

			for podName, podSize := range podNameWithMB {
				// Range limits are measured as part of STONEO11Y-15
				Expect(podSize).To(And(
					BeNumerically(">=", 106),
					BeNumerically("<=", 109),
				), fmt.Sprintf("%s: %d MB is not within the expected range.\n", podName, podSize))
			}
		})

		It("E2E test to measure vCPU Minutes - PipelineRun", func() {

			pipelineRun, err := f.AsKubeAdmin.O11yController.VCPUMinutesPipelineRun(testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Wait for the pipeline run to succeed
			Expect(f.AsKubeAdmin.TektonController.WatchPipelineRunSucceeded(pipelineRun.Name, testNamespace, pipelineRunTimeout)).To(Succeed())

			podNameRegex := "pipelinerun-vcpu-.*"
			query := fmt.Sprintf("{__name__=~'kube_pod_container_resource_limits', namespace='%s', resource='cpu', pod=~'%s', container!='None'}", testNamespace, podNameRegex)
			metricsResult, err := f.AsKubeAdmin.O11yController.GetMetrics(query)
			Expect(err).NotTo(HaveOccurred())

			podNamesWithResult, err := f.AsKubeAdmin.O11yController.GetRegexPodNameWithResult(podNameRegex, metricsResult)
			Expect(err).NotTo(HaveOccurred())

			for podName, result := range podNamesWithResult {
				// CPU Limits of 200 millicores set within deployments
				Expect(result).To(Equal("0.2"), fmt.Sprintf("%s: %s millicores is not within the expected range.\n", podName, result))
			}
		})
	})
})
