package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ecp "github.com/conforma/crds/api/v1alpha1"
	"github.com/devfile/library/v2/pkg/util"
	tektonutils "github.com/konflux-ci/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	releasecommon "github.com/konflux-ci/e2e-tests/tests/release"
	releaseApi "github.com/konflux-ci/release-service/api/v1alpha1"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	releasedImagePushRepo = "quay.io/redhat-appstudio-qe/dcmetromap"
	sampleImage           = "quay.io/redhat-appstudio-qe/dcmetromap@sha256:544259be8bcd9e6a2066224b805d854d863064c9b64fa3a87bfcd03f5b0f28e6"
	gitSourceURL          = "https://github.com/redhat-appstudio-qe/dc-metro-map-release"
	gitSourceRevision     = "d49914874789147eb2de9bb6a12cd5d150bfff92"
	pipelineExamplesURL   = "https://github.com/redhat-appstudio-qe/pipeline_examples"
	pipelineExamplesRev   = "main"

	releasePlanNameFailure  = releasecommon.SourceReleasePlanName
	releasePlanNameSuccess  = releasecommon.SourceReleasePlanName + "-success"
	managedNamespaceFailure = "final-finalizer-managed-failure"
	managedNamespaceSuccess = "final-finalizer-managed-success"
	rpaNameFailure          = releasecommon.TargetReleasePlanAdmissionName
	rpaNameSuccess          = releasecommon.TargetReleasePlanAdmissionName + "-success"
)

// scenarioParams is used by postReleaseVerification to find the right Release and managed namespace.
type scenarioParams struct {
	releasePlanName     string
	managedNamespace    string
	expectManagedToFail bool
}

var (
	fw           *framework.Framework
	err          error
	releaseCR    *releaseApi.Release
	devNamespace string
)

var _ = framework.ReleaseServiceSuiteDescribe("Release service final pipeline finalizer removed", ginkgo.Label("release-service", "final"), func() {
	defer ginkgo.GinkgoRecover()
	ginkgo.AfterEach(framework.ReportFailure(&fw))

	ginkgo.BeforeAll(func() {
		setupFinalPipelineFinalizerSuite()
	})

	ginkgo.AfterAll(func() {
		if !ginkgo.CurrentSpecReport().Failed() {
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespaceFailure)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(managedNamespaceSuccess)).To(gomega.Succeed())
			gomega.Expect(fw.AsKubeAdmin.CommonController.DeleteNamespace(fw.UserNamespace)).To(gomega.Succeed())
		}
	})

	ginkgo.Describe("when managed pipeline fails", func() {
		postReleaseVerification(scenarioParams{
			releasePlanName:     releasePlanNameFailure,
			managedNamespace:    managedNamespaceFailure,
			expectManagedToFail: true,
		})
	})

	ginkgo.Describe("when managed pipeline succeeds", func() {
		postReleaseVerification(scenarioParams{
			releasePlanName:     releasePlanNameSuccess,
			managedNamespace:    managedNamespaceSuccess,
			expectManagedToFail: false,
		})
	})
})

// setupFinalPipelineFinalizerSuite creates one dev namespace with one Application, one tenant SA,
// one PVC, one Snapshot, and two ReleasePlans (failure and success). It creates two managed
// namespaces, each with its own release-service SA, EC policy, and ReleasePlanAdmission.
func setupFinalPipelineFinalizerSuite() {
	fw, err = framework.NewFramework(utils.GetGeneratedNamespace("final-finalizer-dev"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	devNamespace = fw.UserNamespace

	tenantPullSecretName := "final-finalizer-pull-secret"
	tenantSAName := "final-finalizer-service-account"

	// Create both managed namespaces and their release-service SAs and RPAs
	for _, m := range []struct {
		managedNs    string
		rpaName      string
		pipelinePath string
	}{
		{managedNamespaceFailure, rpaNameFailure, "pipelines/failing_pipeline.yaml"},
		{managedNamespaceSuccess, rpaNameSuccess, "pipelines/simple_pipeline.yaml"},
	} {
		_, err = fw.AsKubeAdmin.CommonController.CreateTestNamespace(m.managedNs)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error when creating managedNamespace: %v", err)

		managedSA, err := fw.AsKubeAdmin.CommonController.CreateServiceAccount(releasecommon.ReleasePipelineServiceAccountDefault, m.managedNs, nil, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePipelineRoleBindingForServiceAccount(m.managedNs, managedSA)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		ecPolicyName := "ffinalizer-policy-" + util.GenerateRandomString(4)
		defaultEcPolicy, err := fw.AsKubeAdmin.TektonController.GetEnterpriseContractPolicy("default", "enterprise-contract-service")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defaultEcPolicySpec := ecp.EnterpriseContractPolicySpec{
			Description: "Red Hat's enterprise requirements",
			PublicKey:   fmt.Sprintf("k8s://%s/%s", m.managedNs, releasecommon.PublicSecretNameAuth),
			Sources:     defaultEcPolicy.Spec.Sources,
			Configuration: &ecp.EnterpriseContractPolicyConfiguration{
				Collections: []string{"@slsa3"},
				Exclude:     []string{"step_image_registries", "tasks.required_tasks_found:prefetch-dependencies"},
			},
		}
		_, err = fw.AsKubeAdmin.TektonController.CreateEnterpriseContractPolicy(ecPolicyName, m.managedNs, defaultEcPolicySpec)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		data, err := json.Marshal(map[string]interface{}{
			"mapping": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"component":  releasecommon.ComponentName,
						"repository": releasedImagePushRepo,
					},
				},
			},
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlanAdmission(m.rpaName, m.managedNs, "", devNamespace, ecPolicyName, releasecommon.ReleasePipelineServiceAccountDefault, []string{releasecommon.ApplicationNameDefault}, false, &tektonutils.PipelineRef{
			Resolver: "git",
			Params: []tektonutils.Param{
				{Name: "url", Value: pipelineExamplesURL},
				{Name: "revision", Value: pipelineExamplesRev},
				{Name: "pathInRepo", Value: m.pipelinePath},
			},
		}, &runtime.RawExtension{Raw: data})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}

	// Single tenant secret and SA in dev namespace
	sourceAuthJson := utils.GetEnv("QUAY_TOKEN", "")
	gomega.Expect(sourceAuthJson).ToNot(gomega.BeEmpty())
	_, err = fw.AsKubeAdmin.CommonController.GetSecret(devNamespace, tenantPullSecretName)
	if errors.IsNotFound(err) {
		_, err = fw.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(tenantPullSecretName, devNamespace, sourceAuthJson)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	_, err = fw.AsKubeAdmin.CommonController.CreateServiceAccount(tenantSAName, devNamespace, []corev1.ObjectReference{{Name: tenantPullSecretName}}, nil)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	// Single Application in dev namespace
	_, err = fw.AsKubeAdmin.HasController.CreateApplication(releasecommon.ApplicationNameDefault, devNamespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	data, err := json.Marshal(map[string]interface{}{
		"mapping": map[string]interface{}{
			"components": []map[string]interface{}{
				{
					"component":  releasecommon.ComponentName,
					"repository": releasedImagePushRepo,
				},
			},
		},
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	finalPipeline := &tektonutils.ParameterizedPipeline{}
	finalPipeline.ServiceAccountName = tenantSAName
	finalPipeline.Timeouts = tektonv1.TimeoutFields{
		Pipeline: &metav1.Duration{Duration: 10 * time.Minute},
	}
	finalPipeline.PipelineRef = tektonutils.PipelineRef{
		Resolver: "git",
		Params: []tektonutils.Param{
			{Name: "url", Value: pipelineExamplesURL},
			{Name: "revision", Value: pipelineExamplesRev},
			{Name: "pathInRepo", Value: "pipelines/simple_pipeline.yaml"},
		},
		UseEmptyDir: true,
	}

	// Two ReleasePlans in dev namespace (same final pipeline, different targets)
	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasePlanNameFailure, devNamespace, releasecommon.ApplicationNameDefault, managedNamespaceFailure, "", &runtime.RawExtension{Raw: data}, nil, finalPipeline)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	_, err = fw.AsKubeAdmin.ReleaseController.CreateReleasePlan(releasePlanNameSuccess, devNamespace, releasecommon.ApplicationNameDefault, managedNamespaceSuccess, "", &runtime.RawExtension{Raw: data}, nil, finalPipeline)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = fw.AsKubeAdmin.TektonController.CreatePVCInAccessMode(releasecommon.ReleasePvcName, devNamespace, corev1.ReadWriteOnce)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	_, err = releasecommon.CreateSnapshotWithImageSource(fw.AsKubeAdmin, releasecommon.ComponentName, releasecommon.ApplicationNameDefault, devNamespace, sampleImage, gitSourceURL, gitSourceRevision, "", "", "", "")
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
}

// getReleaseByRPName returns the Release in devNamespace whose spec.releasePlan matches releasePlanName.
func getReleaseByRPName(fw *framework.Framework, devNamespace, releasePlanName string) (*releaseApi.Release, error) {
	list, err := fw.AsKubeAdmin.ReleaseController.GetReleases(devNamespace)
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		r := &list.Items[i]
		if r.Spec.ReleasePlan == releasePlanName {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no Release found with spec.releasePlan %q in namespace %s", releasePlanName, devNamespace)
}

// postReleaseVerification registers the shared "Post-release verification" Its.
// It finds the Release for this scenario by releasePlanName and uses the scenario's managedNamespace.
func postReleaseVerification(p scenarioParams) {
	var _ = ginkgo.Describe("Post-release verification", func() {
		ginkgo.AfterEach(func() {
			if ginkgo.CurrentSpecReport().Failed() && releaseCR != nil {
				release, getErr := fw.AsKubeAdmin.ReleaseController.GetRelease(releaseCR.GetName(), "", devNamespace)
				if getErr == nil {
					releaseYaml, marshalErr := yaml.Marshal(release)
					if marshalErr == nil {
						ginkgo.GinkgoWriter.Printf("Release CR (spec failed):\n%s\n", string(releaseYaml))
					}
				}
			}
		})

		ginkgo.It("verifies that a Release CR should have been created in the dev namespace", func() {
			gomega.Eventually(func() error {
				var getErr error
				releaseCR, getErr = getReleaseByRPName(fw, devNamespace, p.releasePlanName)
				return getErr
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
		})

		ginkgo.It("verifies that managed PipelineRun finished with expected outcome", func() {
			assertManagedPipelineRunFinished(fw, p.managedNamespace, p.expectManagedToFail)
		})

		ginkgo.It("verifies that the Released condition on the Release is no longer Progressing.", func() {
			gomega.Eventually(func() error {
				var getErr error
				releaseCR, getErr = getReleaseByRPName(fw, devNamespace, p.releasePlanName)
				if getErr != nil {
					return getErr
				}
				if releaseCR.IsReleasing() {
					return fmt.Errorf("release %s/%s is not marked as finished yet", releaseCR.GetNamespace(), releaseCR.GetName())
				}
				return nil
			}, releasecommon.ReleaseCreationTimeout, releasecommon.DefaultInterval).Should(gomega.Succeed())
		})

		ginkgo.It("verifies that the finalizer was removed from the final pipeline", func() {
			assertFinalizerRemovedFromFinalPipeline(fw, devNamespace)
		})
	})
}

func assertManagedPipelineRunFinished(fw *framework.Framework, managedNamespace string, expectFailed bool) {
	var debugPrinted bool
	gomega.Eventually(func() error {
		managedPipelineRun, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(managedNamespace, releaseCR.GetName(), releaseCR.GetNamespace())
		if err != nil {
			if !debugPrinted {
				debugPrinted = true
				prList, listErr := fw.AsKubeAdmin.TektonController.ListAllPipelineRuns(managedNamespace)
				if listErr == nil {
					ginkgo.GinkgoWriter.Printf("PipelineRuns in %s: %d total\n", managedNamespace, len(prList.Items))
					for i := range prList.Items {
						pr := &prList.Items[i]
						ginkgo.GinkgoWriter.Printf("  - %s labels=%v\n", pr.GetName(), pr.GetLabels())
					}
				} else {
					ginkgo.GinkgoWriter.Printf("Failed to list PipelineRuns in %s: %v\n", managedNamespace, listErr)
				}
			}
			return fmt.Errorf("managed PipelineRun not found for release %s/%s: %w", releaseCR.GetNamespace(), releaseCR.GetName(), err)
		}
		if !managedPipelineRun.IsDone() {
			return fmt.Errorf("managed PipelineRun %s/%s has not finished yet", managedPipelineRun.GetNamespace(), managedPipelineRun.GetName())
		}
		if expectFailed && !tekton.HasPipelineRunFailed(managedPipelineRun) {
			return fmt.Errorf("expected managed PipelineRun %s/%s to have failed", managedPipelineRun.GetNamespace(), managedPipelineRun.GetName())
		}
		if !expectFailed && !tekton.HasPipelineRunSucceeded(managedPipelineRun) {
			return fmt.Errorf("expected managed PipelineRun %s/%s to have succeeded", managedPipelineRun.GetNamespace(), managedPipelineRun.GetName())
		}
		return nil
	}, 15*time.Minute, releasecommon.DefaultInterval).Should(gomega.Succeed(),
		"managed release PipelineRun should have finished with expected outcome")
}

func assertFinalizerRemovedFromFinalPipeline(fw *framework.Framework, devNamespace string) {
	err := wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 15*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		finalPipelineRun, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(devNamespace, releaseCR.GetName(), devNamespace)
		if err != nil {
			ginkgo.GinkgoWriter.Printf("Final PipelineRun has not been created yet for release %s/%s\n", devNamespace, releaseCR.GetName())
			return false, nil
		}
		for _, condition := range finalPipelineRun.Status.Conditions {
			ginkgo.GinkgoWriter.Printf("PipelineRun %s reason: %s\n", finalPipelineRun.Name, condition.Reason)
		}
		if !finalPipelineRun.IsDone() {
			return false, nil
		}
		return true, nil
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "timed out waiting for final PipelineRun to complete")

	gomega.Eventually(func() error {
		finalPipelineRun, err := fw.AsKubeAdmin.ReleaseController.GetPipelineRunInNamespace(devNamespace, releaseCR.GetName(), devNamespace)
		if err != nil {
			return fmt.Errorf("failed to get final PipelineRun for release %s/%s: %w", releaseCR.GetNamespace(), releaseCR.GetName(), err)
		}
		finalizers := finalPipelineRun.GetFinalizers()
		if len(finalizers) > 0 {
			ginkgo.GinkgoWriter.Printf("Final PipelineRun %s/%s finalizers: %v\n", finalPipelineRun.GetNamespace(), finalPipelineRun.GetName(), finalizers)
		}
		for _, f := range finalizers {
			if f == "appstudio.redhat.com/release-finalizer" {
				return fmt.Errorf("release finalizer still present on final PipelineRun %s/%s", finalPipelineRun.GetNamespace(), finalPipelineRun.GetName())
			}
		}
		return nil
	}, 30*time.Second, releasecommon.DefaultInterval).Should(gomega.Succeed(),
		"release finalizer should have been removed from final pipeline within 30 seconds")
}
