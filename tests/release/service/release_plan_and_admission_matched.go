package service

import (
	"fmt"

	tektonutils "github.com/redhat-appstudio/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/api/meta"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/contract"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service happy path", Label("release-service", "release_plan_and_admission", "HACBS"), func() {
	defer GinkgoRecover()

	var fw *framework.Framework
	var err error
	var devNamespace string
	var managedNamespace = utils.GetGeneratedNamespace("release-plan-and-admission-managed")

	var releasePlanCR, secondReleasePlanCR  *releaseApi.ReleasePlan
	var releasePlanAdmissionCR *releaseApi.ReleasePlanAdmission
	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		// Initialize the tests controllers
		fw, err = framework.NewFramework("release-plan-and-admission")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(managedNamespace)
		Expect(err).NotTo(HaveOccurred(), "Error when creating managedNamespace: %v", err)

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())
		
		//Create ReleasePlan with "auto" mode
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "true")
		Expect(err).NotTo(HaveOccurred())


	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("ReleasePlan verification", func() {
		It("verifies that MatchedCondition of the ReleasePlan CRs are not set", func() {
			releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(sourceReleasePlanName, devNamespace)
			Expect(err).NotTo(HaveOccurred())
			//condition := meta.FindStatusCondition(releasePlanCR.Status.Conditions, MatchedConditionType.String())
			//Expect(condition).To(BeNil())
			Expect(releasePlanCR.IsMatched()).To(BeFalse())
		})

		It("Creates that ReleasePlanAdmission CR in corresponding managed namespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, managedNamespace, "", devNamespace, releaseStrategyPolicyDefault, releasePipelineServiceAccountDefault, []string{applicationNameDefault}, true, &tektonutils.PipelineRef{
				Resolver: "git",
				Params: []tektonutils.Param{
					{Name: "url", Value: "https://github.com/redhat-appstudio/release-service-catalog"},
					{Name: "revision", Value: "main"},
					{Name: "pathInRepo", Value: "pipelines/e2e/e2e.yaml"},
				},
			}, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		When("ReleasePlanAdmission CR is created in managed namespace", func() {
			It("verifies that MatchedCondition of the ReleasePlan CR are set to matched", func() {
				Eventually(func() error {
					releasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(sourceReleasePlanName, devNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanCR.Status.Conditions, MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("the MatchedConditon of %s is still not set", releasePlanCR.Name)
					}
					return nil
				}, releasePlanStatusUpdateTimeout, defaultInterval).Should(Succeed())
				Expect(releasePlanCR.IsMatched()).To(BeTrue())
				Expect(releasePlanCR.status.ReleasePlanAdmission.Name).To(Equal(targetReleasePlanAdmissionName))
				Expect(releasePlanCR.status.ReleasePlanAdmission.Active).To(BeTrue())
			})

			It("verifies that MatchedCondition of the ReleasePlanAdmission CR is set to matched", func() {
				Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(targetReleasePlanAdmissionName, managedNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("the MatchedConditon of %s is still not set", releasePlanAdmissionCR.Name)
					}
					return nil
				}, releasePlanStatusUpdateTimeout, defaultInterval).Should(Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled to matched", releasePlanAdmissionCR.Name))
				condition = meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, MatchedConditionType.String())
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(HaveLen(1))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(Equal([]MatchedReleasePlan{{Name: devNamespace+ "/" + sourceReleasePlanName, Active: true}}))
			})
		})

		It("Creates a manual release ReleasePlan CR in devNamespace", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(secondReleasePlanName, devNamespace, applicationNameDefault, managedNamespace, "false")
			Expect(err).NotTo(HaveOccurred())
		})
		
		When("the second ReleasePlan CR is created", func() {
			It("verifies that MatchedCondition of the second ReleasePlan CR are set to matched", func() {
				Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(secondReleasePlanName, devNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("the MatchedConditon of %s is still not set", secondReleasePlanCR.Name)
					}
					return nil
				}, releasePlanStatusUpdateTimeout, defaultInterval).Should(Succeed())
				Expect(secondReleasePlanCR.IsMatched()).To(BeTrue())
				Expect(secondReleasePlanCR.status.ReleasePlanAdmission.Name).To(Equal(targetReleasePlanAdmissionName))
				Expect(secondReleasePlanCR.status.ReleasePlanAdmission.Active).To(BeFalse())
			})

			It("verifies that the ReleasePlanAdmission CR has two matched ReleasePlan CRs", func() {
				Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(targetReleasePlanAdmissionName, managedNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the MatchedConditon of RPA %s ", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) < 2  {
						return fmt.Errorf("the second ReleasePlan CR has not being added to %s", releasePlanAdmissionCR.Name)
					}
					return nil
				}, releasePlanStatusUpdateTimeout, defaultInterval).Should(Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled to matched", releasePlanAdmissionCR.Name))
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(HaveLen(2))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(Equal([]MatchedReleasePlan{{Name: devNamespace+ "/" + sourceReleasePlanName, Active: true}, {Name: devNamespace + "/" + secondReleasePlanName, Active: false}}))
			})
		})

		It("deletes one ReleasePlan CR", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlan(sourceReleasePlanName, devNamespace, true)
			Expect(err).NotTo(HaveOccurred())
		})
		
		When("One ReleasePlan CR is deleted in managed namespace", func() {
			It("verifies that the ReleasePlanAdmission CR has only one matching ReleasePlan", func() {
				Eventually(func() error {
					releasePlanAdmissionCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlanAdmission(targetReleasePlanAdmissionName, managedNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(releasePlanAdmissionCR.Status.Conditions, MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to find the MatchedConditon of %s", releasePlanAdmissionCR.Name)
					}

					if len(releasePlanAdmissionCR.Status.ReleasePlans) > 1  {
						return fmt.Errorf("ReleasePlan CR is deleted, but ReleasePlanAdmission CR %s has not been reconciled", releasePlanAdmissionCR.Name)
					}
					return true
				}, releasePlanStatusUpdateTimeout, defaultInterval).Should(Succeed(), fmt.Sprintf("time out when waiting for ReleasePlanAdmission %s being reconciled after one ReleasePlan is deleted", releasePlanAdmissionCR.Name))
				Expect(condition).NotTo(BeNil())
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(HaveLen(1))
				Expect(releasePlanAdmissionCR.Status.ReleasePlans).To(Equal([]MatchedReleasePlan{{Name: devNamespace + "/" + secondReleasePlanName, Active: false}}))
			})
		})

		It("deletes the ReleasePlanAdmission CR", func() {
			_, err = fw.AsKubeAdmin.ReleaseController.DeleteReleasePlanAdmission(targetReleasePlanAdmissionName, managedNamespace, true)
			Expect(err).NotTo(HaveOccurred())
		})

		When("ReleasePlanAdmission CR is deleted in managed namespace", func() {
			It("verifies that the ReleasePlan CR has no matched ReleasePlanAdmission", func() {
				Eventually(func() error {
					secondReleasePlanCR, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(secondReleasePlanName, devNamespace)
					Expect(err).NotTo(HaveOccurred())
					condition := meta.FindStatusCondition(secondReleasePlanCR.Status.Conditions, MatchedConditionType.String())
					if condition == nil {
						return fmt.Errorf("failed to get the MatchedConditon of %s", secondReleasePlanCR.Name)
					}
					return nil
				}, releasePlanStatusUpdateTimeout, defaultInterval).Should(Succeed())
				Expect(secondReleasePlanCR.IsMatched()).To(BeFalse())
				Expect(secondReleasePlanCR.Status.ReleasePlanAdmission).To(Equal(MatchedReleasePlanAdmission{}))
			})
		})
	})
})
