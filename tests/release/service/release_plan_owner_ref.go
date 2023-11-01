package service

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
)

var _ = framework.ReleaseServiceSuiteDescribe("[HACBS-2469]test-releaseplan-owner-ref-added", Label("release-service", "releaseplan-ownerref", "HACBS"), func() {
	defer GinkgoRecover()
	var fw *framework.Framework
	var err error
	var devNamespace string
	var releasePlan *releaseApi.ReleasePlan
	AfterEach(framework.ReportFailure(&fw))

	BeforeAll(func() {
		fw, err = framework.NewFramework("releaseplan-ownerref")
		Expect(err).NotTo(HaveOccurred())
		devNamespace = fw.UserNamespace

		_, err = fw.AsKubeAdmin.HasController.CreateApplication(applicationNameDefault, devNamespace)
		Expect(err).NotTo(HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationNameDefault, "managed", "true")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		if !CurrentSpecReport().Failed() {
			Expect(fw.SandboxController.DeleteUserSignup(fw.UserName)).To(BeTrue())
		}
	})

	var _ = Describe("ReleasePlan verification", Ordered, func() {
		It("verifies that the ReleasePlan has an owner reference for the application", func() {
			Eventually(func() error {
				releasePlan, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(sourceReleasePlanName, devNamespace)
				Expect(err).NotTo(HaveOccurred())

				if len(releasePlan.OwnerReferences) != 1 {
					return fmt.Errorf("OwnerReference not updated yet for ReleasePlan %s", releasePlan.Name)
				}

				ownerRef := releasePlan.OwnerReferences[0]
				if ownerRef.Name != applicationNameDefault {
					return fmt.Errorf("ReleasePlan %s have OwnerReference Name %s and it's not as expected in Application Name %s", releasePlan.Name, ownerRef.Name, applicationNameDefault)
				}
				return nil
			}, releasePlanOwnerReferencesTimeout, defaultInterval).Should(Succeed(), "timed out waiting for ReleasePlan OwnerReference to be set.")
		})

		It("verifies that the ReleasePlan is deleted if the application is deleted", func() {
			Expect(fw.AsKubeAdmin.HasController.DeleteApplication(applicationNameDefault, devNamespace, true)).To(Succeed())
			Eventually(func() error {
				releasePlan, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(sourceReleasePlanName, devNamespace)
				if !errors.IsNotFound(err) {
					return fmt.Errorf("ReleasePlan %s for application %s still not deleted\n", releasePlan.GetName(), applicationNameDefault)
				}
				return nil
			}, cacheSyncTimeout, defaultInterval).Should(Succeed(), "timed out waiting for ReleasePlan to be deleted in %s namespace", devNamespace)
		})
	})
})
