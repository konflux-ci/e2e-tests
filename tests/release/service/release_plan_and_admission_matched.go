package service

import (
	"fmt"
	"time"

	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("ReleasePlan and ReleasePlanAdmission match", ginkgo.Label("release-service", "release_plan_and_admission"), func() {
	defer ginkgo.GinkgoRecover()

	var fw *framework.Framework
	var err error
	var devNamespace = "rel-plan-admis"
	var managedNamespace = "plan-and-admission-managed"

	var releasePlanCR, secondReleasePlanCR *releaseApi.ReleasePlan
	var releasePlanAdmissionCR *releaseApi.ReleasePlanAdmission

	ginkgo.AfterEach(framework.ReportFailure(&fw))

	ginkgo.BeforeAll(func() {
		fw, err = framework.NewFramework(utils.GetGeneratedNamespace(devNamespace))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error when creating managedNamespace: %v", err)

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		//Create ReleasePlan
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "true", nil, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(gomega.HaveOccurred())
			gomega.Expect(fw.AsKubeAdmin.HasController.DeleteAllComponentsInASpecificNamespace(devNamespace, time.Minute*2)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.HasController.DeleteAllApplicationsInASpecificNamespace(devNamespace, time.Minute*2)).To(gomega.Succeed())
		}
	})

	ginkgo.Describe("RP and PRA status change verification", func() {
		ginkgo.It("verifies that the ReleasePlan CR is unmatched in the beginning", func() {
			var condition *metav1.Condition
			gomega.Eventually(func() error {
				releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(releasecommon.SourceReleasePlanName, devNamespace)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
				if condition == nil {
					return fmt.Errorf("the MatchedConditon of %s is still not set", releasePlanCR.Name)
				}
				return nil
			}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
			condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
			gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionFalse))
		})

		ginkgo.It("Creates ReleasePlanAdmission CR in corresponding managed namespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace, "", devNamespace, releasecommon.ReleaseStrategyPolicyDefault, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: releasecommon.RelSvcCatalogURL},
					{Name: "revision", Value: releasecommon.RelSvcCatalogRevision},
					{Name: "pathInRepo", Value: "pipelines/managed/e2e/e2e.yaml"},
				},
			}, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.When("ReleasePlanAdmission CR is created in managed namespace", func() {
			ginkgo.It("verifies that the ReleasePlan CR is set to matched", func() {
				var condition *metav1.Condition
				gomega.Eventually(func() error {
					releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(releasecommon.SourceReleasePlanName, devNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition = meta.FindStatusCondition(releasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("the MatchedConditon of %s is still not set", releasePlanCR.Name)
					}
					// it may need a period of time for the ReleasePlanCR to be reconciled
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("the MatchedConditon of %s has not reconciled yet", releasePlanCR.Name)
					}
					return nil
				}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
				gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
				gomega.Expect(releasePlanCR.Status.ReleasePlanAdmission.Name).To(gomega.Equal(managedNamespace + "/" + releasecommon.TargetReleasePlanAdmissionName))
				gomega.Expect(releasePlanCR.Status.ReleasePlanAdmission.Active).To(gomega.BeTrue())
			})

			ginkgo.It("verifies that the ReleasePlanAdmission CR is set to matched", func() {
				var condition *metav1.Condition
				gomega.Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition = meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("the MatchedConditon of %s has not reconciled yet", releasePlanCR.Name)
					}
					return nil
				}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed(), "time out when waiting for ReleasePlanAdmission being reconciled to matched")
				gomega.Expect(condition).NotTo(gomega.BeNil())
				gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.HaveLen(1))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + releasecommon.SourceReleasePlanName, Active: true}}))
			})
		})

		ginkgo.It("Creates a manual release ReleasePlan CR in devNamespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasecommon.SecondReleasePlanName, devNamespace, releasecommon.ApplicationNameDefault, managedNamespace, "false", nil, nil, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.When("the second ReleasePlan CR is created", func() {
			ginkgo.It("verifies that the second ReleasePlan CR is set to matched", func() {
				var condition *metav1.Condition
				gomega.Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(releasecommon.SecondReleasePlanName, devNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition = meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())

					if condition == nil {
						return fmt.Errorf("the MatchedConditon of %s is still not set", secondReleasePlanCR.Name)
					}
					// it may need a period of time for the secondReleasePlanCR to be reconciled
					if condition.Status == metav1.ConditionFalse {
						return fmt.Errorf("the MatchedConditon of %s has not reconciled yet", secondReleasePlanCR.Name)
					}
					return nil
				}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
				gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
				gomega.Expect(secondReleasePlanCR.Status.ReleasePlanAdmission.Name).To(gomega.Equal(managedNamespace + "/" + releasecommon.TargetReleasePlanAdmissionName))
				gomega.Expect(secondReleasePlanCR.Status.ReleasePlanAdmission.Active).To(gomega.BeTrue())
			})

			ginkgo.It("verifies that the ReleasePlanAdmission CR has two matched ReleasePlan CRs", func() {
				gomega.Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the MatchedConditon of RPA %s ", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) < 2 {
						return fmt.Errorf("the second ReleasePlan CR has not being added to %s", releasePlanAdmissionCR.Name)
					}
					gomega.Expect(condition).NotTo(gomega.BeNil())
					gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
					return nil
				}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled to matched", releasePlanAdmissionCR.Name))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.HaveLen(2))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + releasecommon.SourceReleasePlanName, Active: true}, {Name: devNamespace + "/" + releasecommon.SecondReleasePlanName, Active: false}}))
			})
		})

		ginkgo.It("deletes one ReleasePlan CR", func() {
			err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlan(releasecommon.SourceReleasePlanName, devNamespace, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.When("One ReleasePlan CR is deleted in managed namespace", func() {
			ginkgo.It("verifies that the ReleasePlanAdmission CR has only one matching ReleasePlan", func() {
				gomega.Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to find the MatchedConditon of %s", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) > 1 {
						return fmt.Errorf("ReleasePlan CR is deleted, but ReleasePlanAdmission CR %s has not been reconciled", releasePlanAdmissionCR.Name)
					}
					gomega.Expect(condition).NotTo(gomega.BeNil())
					gomega.Expect(condition.Status).To(gomega.Equal(metav1.ConditionTrue))
					return nil
				}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled after one ReleasePlan is deleted", releasePlanAdmissionCR.Name))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.HaveLen(1))
				gomega.Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(gomega.Equal([]releaseApi.MatchedReleasePlan{{Name: devNamespace + "/" + releasecommon.SecondReleasePlanName, Active: false}}))
			})
		})

		ginkgo.It("deletes the ReleasePlanAdmission CR", func() {
			err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlanAdmission(releasecommon.TargetReleasePlanAdmissionName, managedNamespace, true)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.When("ReleasePlanAdmission CR is deleted in managed namespace", func() {
			ginkgo.It("verifies that the ReleasePlan CR has no matched ReleasePlanAdmission", func() {
				gomega.Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(releasecommon.SecondReleasePlanName, devNamespace)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					condition := meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, releaseApi.MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the MatchedConditon of %s", secondReleasePlanCR.Name)
					}

					// it may need a period of time for the secondReleasePlanCR to be reconciled
					if condition.Status == metav1.ConditionTrue {
						return fmt.Errorf("the MatchedConditon of %s has not reconciled yet", secondReleasePlanCR.Name)
					}
					return nil
				}, releasecommon.ReleasePlanStatusUpdateTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
				gomega.Expect(secondReleasePlanCR.Status.ReleasePlanAdmission).To(gomega.Equal(releaseApi.MatchedReleasePlanAdmission{}))
			})
		})
	})
})
