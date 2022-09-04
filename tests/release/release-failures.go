package release

import (
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"k8s.io/apimachinery/pkg/api/meta"
	klog "k8s.io/klog/v2"
)

// const (
// 	missingPipelineName    string = "missing-release-pipeline"
// 	missingReleaseStrategy string = "missing-release-strategy"
// )

var _ = framework.ReleaseSuiteDescribe("test-release-service-failures", Label("release"), func() {
	defer GinkgoRecover()
	// Initialize the tests controllers
	framework, err := framework.NewFramework()
	Expect(err).NotTo(HaveOccurred())

	var devNamespace string
	var managedNamespace string

	var _ = Describe("Failure - AutoReleaseLabel value in ReleasePlanAdmission is not valid (<> true or false)", func() {
		BeforeAll(func() {
			// Recreate random namespaces names per each test because if using same namespace names, the next test will not be able to create the namespaces as they are terminating
			devNamespace = "user-" + uuid.New().String()
			managedNamespace = "managed-" + uuid.New().String()

			// Create the dev namespace
			_, err := framework.CommonController.CreateTestNamespace(devNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)

			// Create the managed namespace
			_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

			// Wait until the "pipeline" SA is created and ready with secrets by the openshift-pipelines operator
			klog.Infof("Wait until the 'pipeline' SA is created in %s namespace \n", managedNamespace)
			Eventually(func() bool {
				sa, err := framework.CommonController.GetServiceAccount("pipeline", managedNamespace)
				return sa != nil && err == nil
			}, 1*time.Minute, defaultInterval).Should(BeTrue(), "timed out when waiting for the \"pipeline\" SA to be created")
		})

		AfterAll(func() {
			// Delete the dev and managed namespaces with all the resources created in them
			Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
			Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
		})

		var _ = Describe("All required resources are created, but ReleasePlanAdmission fails to create succcessfully", func() {
			It("Create an ApplicationSnapshot.", func() {
				_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create Release Strategy", func() {
				_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, releasePipelineName, releasePipelineBundle, releaseStrategyPolicy)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Create ReleasePlan in dev namespace", func() {
				AutoReleaseLabel := "true"
				_, err := framework.ReleaseController.CreateReleasePlan(sourceReleasePlanName, devNamespace, applicationName, managedNamespace, AutoReleaseLabel)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Creation ReleasePlanAdmission in managed namespace with invalid auto-release label value gets rejected", func() {
				AutoReleaseLabel := "not-valid"
				_, err := framework.ReleaseController.CreateReleasePlanAdmission(targetReleasePlanAdmissionName, devNamespace, applicationName, managedNamespace, AutoReleaseLabel, releaseStrategyName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("'%s' label can only be set to true or false", AutoReleaseLabel))
			})

			It("Create a Release", func() {
				_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleasePlanName)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		var _ = Describe("Non-existing ReleasePlanAdmission causes other errors", func() {
			It("The PipelineRun should not exist in the managed namespace", func() {
				Eventually(func() bool {
					pipelineRun, err := framework.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseName, devNamespace)

					if pipelineRun != nil || err != nil {
						return false
					}

					return pipelineRun == nil
				}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
			})

			It("The Release should have failed", func() {
				Eventually(func() bool {
					release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

					if err != nil {
						return false
					}

					return release.IsDone() && meta.IsStatusConditionTrue(release.Status.Conditions, "Succeeded")
				}, avgPipelineCompletionTime, defaultInterval).ShouldNot(BeTrue())
			})
		})
	})

	// var _ = Describe("Failure - Missing matching ReleaseLink", func() {
	// 	BeforeAll(func() {
	// 		// Recreate random namespaces names per each test because if using same namespace names, the next test will not be able to create the namespaces as they are terminating
	// 		devNamespace = "user-" + uuid.New().String()
	// 		managedNamespace = "managed-" + uuid.New().String()

	// 		// Create the dev namespace
	// 		_, err := framework.CommonController.CreateTestNamespace(devNamespace)
	// 		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)

	// 		// Create the managed namespace
	// 		_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
	// 		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)

	// 		// Wait until the "pipeline" SA is created and ready with secrets by the openshift-pipelines operator
	// 		klog.Infof("Wait until the 'pipeline' SA is created in %s namespace \n", managedNamespace)
	// 		Eventually(func() bool {
	// 			sa, err := framework.CommonController.GetServiceAccount("pipeline", managedNamespace)
	// 			return sa != nil && err == nil
	// 		}, 1*time.Minute, defaultInterval).Should(BeTrue(), "timed out when waiting for the \"pipeline\" SA to be created")
	// 	})

	// 	AfterAll(func() {
	// 		// Delete the dev and managed namespaces with all the resources created in them
	// 		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
	// 		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	// 	})

	// 	var _ = Describe("All required resources are created successfully", func() {
	// 		It("Create an ApplicationSnapshot", func() {
	// 			_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create ReleaseLink in dev namespace", func() {
	// 			_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, applicationName, managedNamespace, "")
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create a Release in dev namespace", func() {
	// 			_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleaseLinkName)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})
	// 	})

	// 	var _ = Describe("A ReleaseLink has to have a matching one in a managed workspace", func() {
	// 		It("The Release has failed with the REASON field set to ReleaseValidationError", func() {
	// 			Eventually(func() bool {
	// 				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 				if err != nil || release == nil || len(release.Status.Conditions) == 0 {
	// 					return false
	// 				}

	// 				releaseReason := release.Status.Conditions[0].Reason
	// 				return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleaseValidationError"))
	// 			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 		})

	// 		It("Condition message describes an error finding a matching ReleaseLink", func() {
	// 			Eventually(func() bool {
	// 				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 				if err != nil || release == nil || len(release.Status.Conditions) == 0 {
	// 					return false
	// 				}

	// 				releaseMessage := release.Status.Conditions[0].Message
	// 				return Expect(releaseMessage).Should(ContainSubstring("no ReleaseLink found in target workspace"))
	// 			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 		})
	// 	})
	// })

	// var _ = Describe("Failure - Missing release pipeline", func() {
	// 	BeforeAll(func() {
	// 		devNamespace = "user-" + uuid.New().String()
	// 		managedNamespace = "managed-" + uuid.New().String()

	// 		// Create the dev namespace
	// 		_, err := framework.CommonController.CreateTestNamespace(devNamespace)
	// 		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)

	// 		// Create the managed namespace
	// 		_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
	// 		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)
	// 	})

	// 	AfterAll(func() {
	// 		// Delete the dev and managed namespaces with all the resources created in them
	// 		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
	// 		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	// 	})

	// 	var _ = Describe("All required resources are created successfully", func() {
	// 		It("Create an ApplicationSnapshot", func() {
	// 			_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create ReleaseLink in dev namespace", func() {
	// 			_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, applicationName, managedNamespace, "")
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create ReleaseLink in managed namespace", func() {
	// 			_, err := framework.ReleaseController.CreateReleaseLink(targetReleaseLinkName, managedNamespace, applicationName, devNamespace, releaseStrategyName)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create ReleaseStrategy in managed namespace", func() {
	// 			_, err := framework.ReleaseController.CreateReleaseStrategy(releaseStrategyName, managedNamespace, missingPipelineName, releasePipelineBundle, releaseStrategyPolicy)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create a Release in dev namespace", func() {
	// 			_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleaseLinkName)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})
	// 	})

	// 	var _ = Describe("A Release must relate to an existing pipeline in the managed workspace", func() {
	// 		It("The Release has failed with the REASON field set to ReleasePipelineFailed", func() {
	// 			Eventually(func() bool {
	// 				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 				// Avoid race condition where release.Status.Conditions field didn't have time to get data
	// 				if err != nil || release == nil || len(release.Status.Conditions) == 0 {
	// 					return false
	// 				}

	// 				releaseReason := release.Status.Conditions[0].Reason
	// 				return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleasePipelineFailed"))
	// 			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 		})

	// 		It("Condition message describes an error retrieving pipeline", func() {
	// 			Eventually(func() bool {
	// 				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 				// Avoid race condition where release.Status.Conditions field didn't have time to get data
	// 				if err != nil || release == nil || len(release.Status.Conditions) == 0 || release.Status.Conditions[0].Message == "" {
	// 					return false
	// 				}

	// 				tmpMessage := "could not find object in image with kind: pipeline and name: " + missingPipelineName
	// 				releaseMessage := release.Status.Conditions[0].Message
	// 				// could not find object in image with kind: pipeline and name: missing-release-pipeline
	// 				return Expect(releaseMessage).Should(ContainSubstring("Error retrieving pipeline")) && Expect(releaseMessage).Should(ContainSubstring(tmpMessage))
	// 			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 		})
	// 	})
	// })

	// var _ = Describe("Failure - Missing ReleaseStrategy", func() {
	// 	BeforeAll(func() {
	// 		devNamespace = "user-" + uuid.New().String()
	// 		managedNamespace = "managed-" + uuid.New().String()

	// 		// Create the dev namespace
	// 		_, err := framework.CommonController.CreateTestNamespace(devNamespace)
	// 		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", devNamespace, err)

	// 		// Create the managed namespace
	// 		_, err = framework.CommonController.CreateTestNamespace(managedNamespace)
	// 		Expect(err).NotTo(HaveOccurred(), "Error when creating namespace '%s': %v", managedNamespace, err)
	// 	})

	// 	AfterAll(func() {
	// 		// Delete the dev and managed namespaces with all the resources created in them
	// 		Expect(framework.CommonController.DeleteNamespace(devNamespace)).NotTo(HaveOccurred())
	// 		Expect(framework.CommonController.DeleteNamespace(managedNamespace)).NotTo(HaveOccurred())
	// 	})

	// 	var _ = Describe("All required resources are created successfully", func() {
	// 		It("Create an ApplicationSnapshot", func() {
	// 			_, err := framework.ReleaseController.CreateApplicationSnapshot(snapshotName, devNamespace, applicationName, snapshotComponents)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create ReleaseLink in dev namespace", func() {
	// 			_, err := framework.ReleaseController.CreateReleaseLink(sourceReleaseLinkName, devNamespace, applicationName, managedNamespace, "")
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create ReleaseLink in managed namespace", func() {
	// 			_, err := framework.ReleaseController.CreateReleaseLink(targetReleaseLinkName, managedNamespace, applicationName, devNamespace, missingReleaseStrategy)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})

	// 		It("Create a Release in dev namespace", func() {
	// 			_, err := framework.ReleaseController.CreateRelease(releaseName, devNamespace, snapshotName, sourceReleaseLinkName)
	// 			Expect(err).NotTo(HaveOccurred())
	// 		})
	// 	})

	// 	var _ = Describe("A Release must relate to an existing ReleaseStrategy in the managed workspace", func() {
	// 		It("The Release has failed with the REASON field set to ReleaseValidationError", func() {
	// 			Eventually(func() bool {
	// 				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 				// Avoid race condition where release.Status.Conditions field didn't have time to get data
	// 				if err != nil || release == nil || len(release.Status.Conditions) == 0 {
	// 					return false
	// 				}

	// 				releaseReason := release.Status.Conditions[0].Reason
	// 				return release.IsDone() && meta.IsStatusConditionFalse(release.Status.Conditions, "Succeeded") && Expect(releaseReason).To(Equal("ReleaseValidationError"))
	// 			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 		})

	// 		It("Condition message describes an error retrieving ReleaseStrategy", func() {
	// 			Eventually(func() bool {
	// 				release, err := framework.ReleaseController.GetRelease(releaseName, devNamespace)

	// 				// Avoid race condition where release.Status.Conditions field didn't have time to get data
	// 				if err != nil || release == nil || len(release.Status.Conditions) == 0 || release.Status.Conditions[0].Message == "" {
	// 					return false
	// 				}

	// 				tmpMessage := "\"" + missingReleaseStrategy + "\"" + " not found"
	// 				releaseMessage := release.Status.Conditions[0].Message
	// 				return Expect(releaseMessage).Should(ContainSubstring(tmpMessage))
	// 			}, avgPipelineCompletionTime, defaultInterval).Should(BeTrue())
	// 		})
	// 	})
	// })
})
