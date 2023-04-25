package o11y

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

var _ = framework.O11ySuiteDescribe("O11Y E2E tests for Deployments", Label("o11y", "HACBS"), func() {

	defer GinkgoRecover()
	var (
		thanosQuerierHost string
		token             string
		f                 *framework.Framework
		err               error
	)

	Describe("O11y test", func() {
		var testNamespace string

		BeforeAll(func() {

			f, err = framework.NewFramework(O11yUserDeployments)
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace

			thanosQuerierRoute, err := f.AsKubeAdmin.CommonController.GetOpenshiftRoute("thanos-querier", monitoringNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			thanosQuerierHost = thanosQuerierRoute.Spec.Host
			secret, err := f.AsKubeAdmin.O11yController.GetSecretName(userWorkloadNamespace, userWorkloadToken)
			Expect(err).NotTo(HaveOccurred())
			token, err = f.AsKubeAdmin.O11yController.GetDecodedTokenFromSecret(userWorkloadNamespace, secret)
			Expect(err).NotTo(HaveOccurred())

			// Get Quay Token from ENV
			quayToken := utils.GetEnv("QUAY_TOKEN", "")
			Expect(quayToken).ToNot(BeEmpty())

			_, err = f.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(o11yUserSecret, testNamespace, quayToken)
			Expect(err).ToNot(HaveOccurred())

			err = f.AsKubeAdmin.CommonController.LinkSecretToServiceAccount(testNamespace, o11yUserSecret, O11ySA, true)
			Expect(err).ToNot(HaveOccurred())

		})
		AfterAll(func() {
			if !CurrentSpecReport().Failed() {
				Expect(f.SandboxController.DeleteUserSignup(f.UserName)).NotTo(BeFalse())
			}
		})

		It("E2E test to validate samplequery", func() {
			_, err = f.AsKubeAdmin.O11yController.RunSampleQuery(testNamespace, thanosQuerierHost, token)
			Expect(err).NotTo(HaveOccurred())
		})

		It("[OCPBUGS-4053]: E2E test to measure Egress pod by pushing images to quay - Deployments", Pending, func() {

			// Get Quay Organization from ENV
			quayOrg := utils.GetQuayIOOrganization()
			Expect(quayOrg).ToNot(BeEmpty())

			deployment, err := f.AsKubeAdmin.O11yController.QuayImagePushDeployment(quayOrg, o11yUserSecret, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.DeploymentIsCompleted(deployment.Name, testNamespace, 1), deploymentTimeout)).To(Succeed())

			err = f.AsKubeAdmin.O11yController.WaitForScriptCompletion(deployment, egressSuccessMessage, logScriptTimeout)
			Expect(err).NotTo(HaveOccurred())

			podNameRegex := "deployment-egress-.*"
			query := fmt.Sprintf("last_over_time(container_network_transmit_bytes_total{namespace='%s', pod=~'%s'}[1h])", testNamespace, podNameRegex)
			result, err := f.AsKubeAdmin.O11yController.QueryThanosAPI(query, thanosQuerierHost, token)
			Expect(err).NotTo(HaveOccurred())

			podNamesWithResult, err := f.AsKubeAdmin.O11yController.GetRegexPodNameWithResult(podNameRegex, result)
			Expect(err).NotTo(HaveOccurred())

			podNameWithMB, err := f.AsKubeAdmin.O11yController.ConvertValuesToMB(podNamesWithResult)
			Expect(err).NotTo(HaveOccurred())

			for podName, podSize := range podNameWithMB {
				// Range limits are measured as part of STONEO11Y-15
				Expect(podSize).To(And(
					BeNumerically(">=", 104),
					BeNumerically("<=", 109),
				), fmt.Sprintf("%s: %d MB is not within the expected range.\n", podName, podSize))
			}
		})

		It("E2E test to measure vCPU minutes - Deployments", func() {

			deployment, err := f.AsKubeAdmin.O11yController.VCPUMinutesDeployment(testNamespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(utils.WaitUntil(f.AsKubeAdmin.CommonController.DeploymentIsCompleted(deployment.Name, testNamespace, 1), deploymentTimeout)).To(Succeed())

			err = f.AsKubeAdmin.O11yController.WaitForScriptCompletion(deployment, vCPUSuccessMessage, logScriptTimeout)
			Expect(err).NotTo(HaveOccurred())

			podNameRegex := "deployment-vcpu-.*"
			query := fmt.Sprintf("{__name__=~'kube_pod_container_resource_limits', namespace='%s', resource='cpu', pod=~'%s'}", testNamespace, podNameRegex)
			metricsResult, err := f.AsKubeAdmin.O11yController.QueryThanosAPI(query, thanosQuerierHost, token)
			Expect(err).NotTo(HaveOccurred())

			podNamesWithResult, err := f.AsKubeAdmin.O11yController.GetRegexPodNameWithResult(podNameRegex, metricsResult)
			Expect(err).NotTo(HaveOccurred())

			for podName, result := range podNamesWithResult {
				// CPU Limits of 200 millicores set within deployments
				Expect(result).To(Equal("0.1"), fmt.Sprintf("%s: %s millicores is not within the expected range.\n", podName, result))
			}
		})

		It("E2E test to measure Memory minutes - Deployments", func() {
			// Calculate Memory minutes based on value set within vCPU deployment
			podNameRegex := "deployment-vcpu-.*"
			query := fmt.Sprintf("{__name__=~'kube_pod_container_resource_limits', namespace='%s', resource='memory', pod=~'%s'}", testNamespace, podNameRegex)
			metricsResult, err := f.AsKubeAdmin.O11yController.QueryThanosAPI(query, thanosQuerierHost, token)
			Expect(err).NotTo(HaveOccurred())

			podNamesWithResult, err := f.AsKubeAdmin.O11yController.GetRegexPodNameWithResult(podNameRegex, metricsResult)
			Expect(err).NotTo(HaveOccurred())

			podNameWithMB, err := f.AsKubeAdmin.O11yController.ConvertValuesToMB(podNamesWithResult)
			Expect(err).NotTo(HaveOccurred())

			for podName, result := range podNameWithMB {
				// Memory Limits of 100MB set within deployments
				Expect(result).To(Equal(104), fmt.Sprintf("%s: %d MB is not within the expected range.\n", podName, result))
			}
		})
	})
})
