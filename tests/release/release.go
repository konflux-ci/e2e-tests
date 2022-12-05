package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	applicationapiv1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	klog "k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var snapshotComponents = []applicationapiv1alpha1.SnapshotComponent{
	{Name: "component-1", ContainerImage: "quay.io/redhat-appstudio/component1@sha256:d5e85e49c89df42b221d972f5b96c6507a8124717a6e42e83fd3caae1031d514"},
	{Name: "component-2", ContainerImage: "quay.io/redhat-appstudio/component2@sha256:a01dfd18cf8ca8b68770b09a9b6af0fd7c6d1f8644c7ab97f0e06c34dfc5860e"},
	{Name: "component-3", ContainerImage: "quay.io/redhat-appstudio/component3@sha256:d90a0a33e4c5a1daf5877f8dd989a570bfae4f94211a8143599245e503775b1f"},
}

var ecPolicy = ecp.EnterpriseContractPolicySpec{
	Description: "Red Hat's enterprise requirements",
	Sources: []string{
		"https://github.com/hacbs-contract/ec-policies",
	},
	Exceptions: &ecp.EnterpriseContractPolicyExceptions{
		NonBlocking: []string{"tasks", "attestation_task_bundle", "java", "test", "not_useful"},
	},
}

var paramsReleaseStrategy = []appstudiov1alpha1.Params{}

var _ = framework.ReleaseSuiteDescribe("[HACBS-1108]test-release-service-happy-path", Label("release", "HACBS"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace = uuid.New().String()
	var managedNamespace = uuid.New().String()

	BeforeAll(func() {
		// Create the dev namespace
		_, err := framework.CommonController.CreateTestNamespace(devNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)
		klog.Info("Dev Namespace :", devNamespace)

		// Create the managed namespace
		_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)
		klog.Info("Managed Namespace :", managedNamespace)

		// Wait until the "pipeline" SA is created and ready with secrets by the openshift-pipelines operator
		klog.Infof("Wait until the 'pipeline' SA is created in %s namespace \n", managedNamespace)
		Eventually(func() bool {
			sa, err := framework.CommonController.GetServiceAccount(serviceAccount, managedNamespace)
			return sa != nil && err == nil
		}, 1*time.Minute, defaultInterval).Should(BeTrue(), "timed out when waiting for the \"pipeline\" SA to be created")
	})

	AfterAll(func() {

		// Delete the dev and managed namespaces with all the resources created in them
		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	})

	var _ = Describe("Creation of the 'Happy path' resources", func() {

		It("Create a Snapshot in dev namespace.", func(ctx SpecContext) {
			_, err := framework.ReleaseController.CreateSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(applicationSnapshotCreationTimeout))

		It("Create Release Strategy in managed namespace.", func(ctx SpecContext) {
			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy, serviceAccount, paramsReleaseStrategy)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releaseStrategyCreationTimeout))

		It("Create ReleasePlan in dev namespace.", func(ctx SpecContext) {
			_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, "")
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releasePlanCreationTimeout))

		It("Create EnterpriseContractPolicy in managed namespace.", func(ctx SpecContext) {
			_, err := framework.TektonController.CreateEnterpriseContractPolicy(releaseStrategyPolicy, managedNamespace, ecPolicy)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(EnterpriseContractPolicyTimeout))

		It("Create ReleasePlanAdmission in managed namespace.", func(ctx SpecContext) {
			_, err := framework.ReleaseController.CreateReleasePlanAdmission(destinationReleasePlanAdmissionName, devNamespace, applicationName, managedNamespace, "", "", releaseStrategyName)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releasePlanAdmissionCreationTimeout))

		It("Create a Release in dev namespace.", func(ctx SpecContext) {
			_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleasePlanName)
			Expect(err).NotTo(HaveOccurred())
		}, SpecTimeout(releaseCreationTimeout))
	})

	var _ = Describe("Post-release verification.", func() {

		It("A PipelineRun should have been created in the managed namespace.", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || prList == nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}

				return strings.Contains(prList.Items[0].Name, releaseName)
			}, releasePipelineRunCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("The PipelineRun should exist and succeed.", func() {
			Eventually(func() bool {
				prList, err := framework.TektonController.ListAllPipelineRuns(managedNamespace)
				if prList == nil || err != nil || len(prList.Items) < 1 {
					klog.Error(err)
					return false
				}

				return prList.Items[0].HasStarted() && prList.Items[0].IsDone() && prList.Items[0].Status.GetCondition(apis.ConditionSucceeded).IsTrue()
			}, releasePipelineRunCompletionTimeot, defaultInterval).Should(BeTrue())
		})

		It("The Release should have succeeded.", func() {
			Eventually(func() bool {
				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)
				if err != nil || release == nil {
					return false
				}

				return release.IsDone() && meta.IsStatusConditionTrue(release.Status.Conditions, "Succeeded")
			}, releaseCreationTimeout, defaultInterval).Should(BeTrue())
		})

		It("The Release should reference the release PipelineRun.", func() {
			var pipelineRunList *v1beta1.PipelineRunList

			Eventually(func() bool {
				pipelineRunList, err = framework.TektonController.ListAllPipelineRuns(managedNamespace)
				if err != nil || pipelineRunList == nil {
					return false
				}

				return len(pipelineRunList.Items) > 0 && err == nil
			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())

			release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)
			if err != nil {
				klog.Error(err)
			}
			Expect(release.Status.ReleasePipelineRun == (fmt.Sprintf("%s/%s", pipelineRunList.Items[0].Namespace, pipelineRunList.Items[0].Name))).Should(BeTrue())
		})
	})
})
