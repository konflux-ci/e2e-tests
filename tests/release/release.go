package release

import (
	"fmt"
	"strings"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

const (
	DEFAULT_RELEASE_SERVICE_USER = "release-service-e2e"
)

var snapshotComponents = []applicationapiv1alpha1.SnapshotComponent{
	{Name: "component-1", ContainerImage: "quay.io/redhat-appstudio/component1@sha256:d5e85e49c89df42b221d972f5b96c6507a8124717a6e42e83fd3caae1031d514"},
	{Name: "component-2", ContainerImage: "quay.io/redhat-appstudio/component2@sha256:a01dfd18cf8ca8b68770b09a9b6af0fd7c6d1f8644c7ab97f0e06c34dfc5860e"},
	{Name: "component-3", ContainerImage: "quay.io/redhat-appstudio/component3@sha256:d90a0a33e4c5a1daf5877f8dd989a570bfae4f94211a8143599245e503775b1f"},
}

var _ = framework.ReleaseSuiteDescribe("[HACBS-1108]test-release-service-happy-path", Label("release", "happy-path", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error

	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()
	var ecPolicy ecp.EnterpriseContractPolicySpec

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework(DEFAULT_RELEASE_SERVICE_USER)
		Expect(err).NotTo(HaveOccurred())
		// Create the dev namespace
		_, err := fw.AsKubeAdmin.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)
		GinkgoWriter.Println("Dev Namespace :", devNamespace)

		// Create the managed namespace
		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

		GinkgoWriter.Println("Managed Namespace :", managedNamespace)

		// get the ec configmap to configure the policy and data sources
		cm, err := fw.AsKubeAdmin.CommonController.GetConfigMap("ec-defaults", "enterprise-contract-service")
		Expect(err).ToNot(HaveOccurred())
		// the default policy source
		ecPolicy = ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			Sources: []ecp.Source{
				{
					Name:   "ec-policies",
					Policy: []string{cm.Data["ec_policy_source"]},
					Data:   []string{cm.Data["ec_data_source"]},
				},
			},
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Exclude: []string{"tasks", "attestation_task_bundle", "java", "test", "not_useful"},
			},
		}
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.TektonController.DeleteAllPipelineRunsInASpecificNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).NotTo(BeFalse())
		}
	})

	var _ = Describe("Creation of the 'Happy path' resources", func() {

		It("creates a Snapshot in dev namespace.", func(ctx SpecContext) {
			_, err := fw.AsKubeAdmin.ReleaseController.CreateSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
			Expect(err).NotTo(HaveOccurred())
			// We add the namespace creation timeout as this is the first test so must also take into account the code in BeforeAll
		}, SpecTimeout(snapshotCreationTimeout+namespaceCreationTimeout*2))

		It("creates Release Strategy in managed namespace.", func(ctx SpecContext) {
			_, err := fw.AsKubeAdmin.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, serviceAccount, paramsReleaseStrategyM6)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releaseStrategyCreationTimeout))

		It("creates ReleasePlan in dev namespace.", func(ctx SpecContext) {
			_, err := fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releasePlanCreationTimeout))

		It("creates EnterpriseContractPolicy in managed namespace.", func(ctx SpecContext) {
			_, err := fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicy, managedNamespace, ecPolicy)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(EnterpriseContractPolicyTimeout))

		It("creates ReleasePlanAdmission in managed namespace.", func(ctx SpecContext) {
			_, err := fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, devNamespace, applicationName, managedNamespace, "", "", releaseStrategyName)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releasePlanAdmissionCreationTimeout))

		It("creates a Release in dev namespace.", func(ctx SpecContext) {
			_, err := fw.AsKubeAdmin.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleasePlanName)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releaseCreationTimeout))
	})

	var _ = Describe("post-release verification.", func() {

		It("makes sure a PipelineRun should have been created in the managed namespace.", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					GinkgoWriter.Println(err)
					return false
				}

				return strings.Contains(prList.Items[0].Name, releaseName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("makes sure the PipelineRun exists and succeeded", func() {
			Eventually(func() bool {
				prList, err := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					GinkgoWriter.Println(err)
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeout, defaultInterval).Should(BeTrue())
		})

		It("makes sure that the Release should have succeeded.", func() {
			Eventually(func() bool {
				release, err := fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
				if err != nil || release == nil {
					return false
				}

				return release.IsReleased()
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("makes sure the Release references the release PipelineRun.", func(ctx SpecContext) {
			var pipelineRunList *v1beta1.PipelineRunList

			Eventually(func() bool {
				pipelineRunList, err = fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || pipelineRunList == nil {
					return false
				}

				return len(pipelineRunList.Items) > 0 && err == nil
			}, avgControllerQueryTimeout, defaultInterval).Should(BeTrue())

			release, err := fw.AsKubeAdmin.ReleaseController.GetRelease(releaseName, "", devNamespace)
			if err != nil {
				GinkgoWriter.Println(err)
			}
			Expect(release.Status.Processing.PipelineRun == (fmt.Sprintf("%s/%s", pipelineRunList.Items[0].Namespace, pipelineRunList.Items[0].Name))).Should(BeTrue())
			// We add the namespace deletion timeout as this is the last test so must also take into account the code in AfterAll
		}, SpecTimeout(avgControllerQueryTimeout*2+namespaceDeletionTimeout*2))
	})
})
