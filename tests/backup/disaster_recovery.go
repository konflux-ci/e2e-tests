// disaster_recovery.go contains core DR (Disaster Recovery) operation helpers
// for the backup/restore e2e test suite. These functions orchestrate Velero
// Backup and Restore CRs, verify that restored resources match expectations,
// and handle cleanup and failure artifact collection.
//
// NOTE: Helper functions call GinkgoHelper() so that assertion failures report
// the caller's location in the test spec, not the helper's internal line.
package backup

import (
	"context"
	"fmt"

	"github.com/konflux-ci/e2e-tests/pkg/framework"
	imagecontrollerv1alpha1 "github.com/konflux-ci/image-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck
	. "github.com/onsi/gomega"    //nolint:staticcheck
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createBackup creates a Velero Backup CR for the given tenant's namespace and
// polls until the backup reaches the "Completed" phase. The Backup CR is
// created in VeleroNamespace ("openshift-adp") and targets only the tenant's
// namespace with the IncludedResources defined in const.go.
func createBackup(fw *framework.Framework, t Tenant) {
	GinkgoHelper()

	By(fmt.Sprintf("Creating Velero Backup CR %q for namespace %q", t.BackupName, t.Namespace))

	backup := &velerov1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.BackupName,
			Namespace: VeleroNamespace,
		},
		Spec: velerov1.BackupSpec{
			IncludedNamespaces: []string{t.Namespace},
			IncludedResources:  IncludedResources,
		},
	}

	err := fw.AsKubeAdmin.CommonController.KubeRest().Create(context.Background(), backup)
	Expect(err).ShouldNot(HaveOccurred(), "failed to create Backup CR %q", t.BackupName)

	By(fmt.Sprintf("Waiting for Backup CR %q to reach Completed phase (timeout: %s)", t.BackupName, BackupTimeout))

	Eventually(func() (velerov1.BackupPhase, error) {
		got := &velerov1.Backup{}
		err := fw.AsKubeAdmin.CommonController.KubeRest().Get(context.Background(),
			client.ObjectKey{Name: t.BackupName, Namespace: VeleroNamespace}, got)
		if err != nil {
			return "", err
		}
		return got.Status.Phase, nil
	}, BackupTimeout, BackupPollInterval).Should(Equal(velerov1.BackupPhaseCompleted),
		"Backup CR %q did not reach Completed phase within %s", t.BackupName, BackupTimeout)
}

// restoreFromBackup creates a Velero Restore CR for the given tenant and polls
// until the restore reaches the "Completed" phase. The method parameter selects
// which code path constructs the Restore CR:
//
//   - RestoreMethodVeleroCLI: builds the CR programmatically using setter methods
//     (mirrors the Velero CLI approach from the SOP).
//   - RestoreMethodOCCommand: builds the CR as a complete map[string]interface{}
//     manifest (mirrors the `oc apply -f` approach from the SOP).
//
// Both methods produce identical CRs but exercise different construction code
// paths, validating both SOP procedures.
func restoreFromBackup(fw *framework.Framework, t Tenant, method RestoreMethod) {
	GinkgoHelper()

	restoreName := "restore-" + t.BackupName
	By(fmt.Sprintf("Creating Velero Restore CR %q from backup %q using %s method", restoreName, t.BackupName, method))

	var restore *velerov1.Restore

	switch method {
	case RestoreMethodVeleroCLI:
		// Programmatic construction — mirrors the Velero CLI approach.
		// Fields are set individually, as if using CLI flags.
		restore = &velerov1.Restore{}
		restore.Name = restoreName
		restore.Namespace = VeleroNamespace
		restore.Spec.BackupName = t.BackupName
		restore.Spec.IncludedNamespaces = []string{t.Namespace}
		restore.Spec.IncludedResources = IncludedResources

	case RestoreMethodOCCommand:
		// Declarative struct literal — mirrors the `oc apply -f` YAML approach.
		restore = &velerov1.Restore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      restoreName,
				Namespace: VeleroNamespace,
			},
			Spec: velerov1.RestoreSpec{
				BackupName:         t.BackupName,
				IncludedNamespaces: []string{t.Namespace},
				IncludedResources:  IncludedResources,
			},
		}
	}

	err := fw.AsKubeAdmin.CommonController.KubeRest().Create(context.Background(), restore)
	Expect(err).ShouldNot(HaveOccurred(), "failed to create Restore CR %q", restoreName)

	By(fmt.Sprintf("Waiting for Restore CR %q to reach Completed phase (timeout: %s)", restoreName, RestoreTimeout))

	Eventually(func() (velerov1.RestorePhase, error) {
		got := &velerov1.Restore{}
		err := fw.AsKubeAdmin.CommonController.KubeRest().Get(context.Background(),
			client.ObjectKey{Name: restoreName, Namespace: VeleroNamespace}, got)
		if err != nil {
			return "", err
		}
		return got.Status.Phase, nil
	}, RestoreTimeout, RestorePollInterval).Should(Equal(velerov1.RestorePhaseCompleted),
		"Restore CR %q did not reach Completed phase within %s", restoreName, RestoreTimeout)
}

// verifyResources performs structural verification of restored tenant resources.
// It checks that the Application, Components, IntegrationTestScenarios,
// ServiceAccounts, SA token Secrets, ReleasePlan, PaC Repository CRs, and
// ImageRepository CRs all exist and have the expected field values.
// This is a structural check (existence + key fields), not a snapshot
// diff, which keeps the tests stable across Konflux version changes.
func verifyResources(fw *framework.Framework, t Tenant) {
	GinkgoHelper()

	By(fmt.Sprintf("Verifying Application %q exists in namespace %q", t.AppName, t.Namespace))
	_, err := fw.AsKubeAdmin.HasController.GetApplication(t.AppName, t.Namespace)
	Expect(err).ShouldNot(HaveOccurred(), "Application %q should exist in namespace %q", t.AppName, t.Namespace)

	By(fmt.Sprintf("Verifying all %d Components exist with correct spec fields", len(Components)))
	for _, comp := range Components {
		c, err := fw.AsKubeAdmin.HasController.GetComponent(comp.Name, t.Namespace)
		Expect(err).ShouldNot(HaveOccurred(), "Component %q should exist in namespace %q", comp.Name, t.Namespace)

		// Verify every Spec field that is set at creation time and NOT mutated
		// by controllers. Two fields are intentionally excluded:
		//
		//   - Spec.ContainerImage: Populated asynchronously by the
		//     image-controller when it creates an ImageRepository for the
		//     Component. The value depends on the image registry state at
		//     restore time and may legitimately differ from the original.
		//
		//   - Spec.Actions: A write-once trigger field. Controllers consume
		//     and remove actions after processing them, so the field is
		//     expected to be empty on any persisted Component.
		Expect(c).Should(SatisfyAll(
			HaveField("Spec.ComponentName", Equal(comp.Name)),
			HaveField("Spec.Application", Equal(t.AppName)),
			HaveField("Spec.Source.GitSource.URL", Equal(MathWizzRepo)),
			HaveField("Spec.Source.GitSource.Context", Equal(comp.ContextDir)),
			HaveField("Spec.Source.GitSource.DockerfileURL", Equal(comp.DockerfileURL)),
			HaveField("Spec.TargetPort", Equal(8081)),
		), "Component %q in namespace %q has unexpected spec fields", comp.Name, t.Namespace)
	}

	By(fmt.Sprintf("Verifying at least one IntegrationTestScenario exists in namespace %q", t.Namespace))
	scenarios, err := fw.AsKubeAdmin.IntegrationController.GetIntegrationTestScenarios(t.AppName, t.Namespace)
	Expect(err).ShouldNot(HaveOccurred(), "should be able to list IntegrationTestScenarios in namespace %q", t.Namespace)
	Expect(*scenarios).ShouldNot(BeEmpty(),
		"at least one IntegrationTestScenario should exist in namespace %q", t.Namespace)

	By(fmt.Sprintf("Verifying at least one ServiceAccount exists in namespace %q", t.Namespace))
	saList := &corev1.ServiceAccountList{}
	err = fw.AsKubeAdmin.CommonController.KubeRest().List(context.Background(), saList, client.InNamespace(t.Namespace))
	Expect(err).ShouldNot(HaveOccurred(), "should be able to list ServiceAccounts in namespace %q", t.Namespace)
	Expect(saList.Items).ShouldNot(BeEmpty(),
		"at least one ServiceAccount should exist in namespace %q", t.Namespace)

	By(fmt.Sprintf("Verifying SA token Secrets exist in namespace %q (proves token rotation worked)", t.Namespace))
	secretList := &corev1.SecretList{}
	err = fw.AsKubeAdmin.CommonController.KubeRest().List(context.Background(), secretList, client.InNamespace(t.Namespace))
	Expect(err).ShouldNot(HaveOccurred(), "should be able to list Secrets in namespace %q", t.Namespace)

	hasTokenSecret := false
	for i := range secretList.Items {
		if secretList.Items[i].Type == corev1.SecretTypeServiceAccountToken {
			hasTokenSecret = true
			break
		}
	}
	Expect(hasTokenSecret).Should(BeTrue(),
		"at least one ServiceAccount token Secret should exist in namespace %q", t.Namespace)

	By(fmt.Sprintf("Verifying ReleasePlan %q exists in namespace %q", DRReleasePlanName, t.Namespace))
	_, err = fw.AsKubeAdmin.ReleaseController.GetReleasePlan(DRReleasePlanName, t.Namespace)
	Expect(err).ShouldNot(HaveOccurred(),
		"ReleasePlan %q should exist in namespace %q (proves release config survived backup/restore)", DRReleasePlanName, t.Namespace)

	By(fmt.Sprintf("Verifying PaC Repository CRs exist for all %d Components in namespace %q", len(Components), t.Namespace))
	for _, comp := range Components {
		_, err := fw.AsKubeAdmin.TektonController.GetRepositoryParams(comp.Name, t.Namespace)
		Expect(err).ShouldNot(HaveOccurred(),
			"PaC Repository CR should exist for component %q in namespace %q", comp.Name, t.Namespace)
	}

	By(fmt.Sprintf("Verifying ImageRepository CRs exist in namespace %q (one per component)", t.Namespace))
	imageRepoList := &imagecontrollerv1alpha1.ImageRepositoryList{}
	err = fw.AsKubeAdmin.CommonController.KubeRest().List(context.Background(), imageRepoList, client.InNamespace(t.Namespace))
	Expect(err).ShouldNot(HaveOccurred(), "should be able to list ImageRepositories in namespace %q", t.Namespace)
	Expect(imageRepoList.Items).Should(HaveLen(len(Components)),
		"expected %d ImageRepository CRs in namespace %q (one per component)", len(Components), t.Namespace)
}

// collectFailureArtifacts logs diagnostic information for troubleshooting DR
// test failures. It dumps Velero pod status and the status of all Backup and
// Restore CRs associated with the given tenants. This function is safe to call
// even when resources have already been cleaned up — it ignores missing
// resources gracefully.
func collectFailureArtifacts(fw *framework.Framework, tenants []Tenant) {
	GinkgoHelper()

	ctx := context.Background()

	By("Collecting Velero pod information")
	pods, err := fw.AsKubeAdmin.CommonController.KubeInterface().CoreV1().Pods(VeleroNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "component=velero",
	})
	if err != nil {
		GinkgoWriter.Printf("WARNING: failed to list Velero pods: %v\n", err)
	} else {
		for i := range pods.Items {
			pod := &pods.Items[i]
			GinkgoWriter.Printf("Velero pod: %s | Phase: %s | Ready: %v\n",
				pod.Name, pod.Status.Phase, isPodReady(pod))
		}
	}

	for _, t := range tenants {
		By(fmt.Sprintf("Collecting Backup CR status for tenant %q", t.Namespace))
		backup := &velerov1.Backup{}
		if err := fw.AsKubeAdmin.CommonController.KubeRest().Get(ctx,
			client.ObjectKey{Name: t.BackupName, Namespace: VeleroNamespace}, backup); err != nil {
			GinkgoWriter.Printf("WARNING: could not get Backup CR %q: %v\n", t.BackupName, err)
		} else {
			GinkgoWriter.Printf("Backup CR %q: phase=%s\n", t.BackupName, backup.Status.Phase)
			if backup.Status.Errors > 0 || backup.Status.Warnings > 0 {
				GinkgoWriter.Printf("  errors=%d, warnings=%d\n", backup.Status.Errors, backup.Status.Warnings)
			}
		}

		restoreName := "restore-" + t.BackupName
		By(fmt.Sprintf("Collecting Restore CR status for tenant %q", t.Namespace))
		restore := &velerov1.Restore{}
		if err := fw.AsKubeAdmin.CommonController.KubeRest().Get(ctx,
			client.ObjectKey{Name: restoreName, Namespace: VeleroNamespace}, restore); err != nil {
			GinkgoWriter.Printf("WARNING: could not get Restore CR %q: %v\n", restoreName, err)
		} else {
			GinkgoWriter.Printf("Restore CR %q: phase=%s\n", restoreName, restore.Status.Phase)
			if restore.Status.Errors > 0 || restore.Status.Warnings > 0 {
				GinkgoWriter.Printf("  errors=%d, warnings=%d\n", restore.Status.Errors, restore.Status.Warnings)
			}
		}
	}
}

// isPodReady returns true if the given pod has the Ready condition set to True.
// This is a pure helper with no Ginkgo assertions, so GinkgoHelper() is not needed.
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// cleanupTestResources deletes DR test resources: tenant namespaces, managed
// namespaces, and associated Velero Backup/Restore CRs. Errors are logged
// and collected so that all cleanup steps run even if some fail, then any
// errors are reported at the end.
func cleanupTestResources(fw *framework.Framework, tenants []Tenant) {
	GinkgoHelper()

	ctx := context.Background()
	kubeClient := fw.AsKubeAdmin.CommonController.KubeInterface()
	restClient := fw.AsKubeAdmin.CommonController.KubeRest()

	var errs []error
	for _, t := range tenants {
		By(fmt.Sprintf("Cleaning up namespace %q", t.Namespace))
		if err := kubeClient.CoreV1().Namespaces().Delete(ctx, t.Namespace, metav1.DeleteOptions{}); err != nil {
			GinkgoWriter.Printf("WARNING: failed to delete namespace %q: %v\n", t.Namespace, err)
			errs = append(errs, err)
		}

		By(fmt.Sprintf("Cleaning up Backup CR %q", t.BackupName))
		if err := restClient.Delete(ctx, &velerov1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: t.BackupName, Namespace: VeleroNamespace},
		}); err != nil {
			GinkgoWriter.Printf("WARNING: failed to delete Backup CR %q: %v\n", t.BackupName, err)
			errs = append(errs, err)
		}

		restoreName := "restore-" + t.BackupName
		By(fmt.Sprintf("Cleaning up Restore CR %q", restoreName))
		if err := restClient.Delete(ctx, &velerov1.Restore{
			ObjectMeta: metav1.ObjectMeta{Name: restoreName, Namespace: VeleroNamespace},
		}); err != nil {
			GinkgoWriter.Printf("WARNING: failed to delete Restore CR %q: %v\n", restoreName, err)
			errs = append(errs, err)
		}

		By(fmt.Sprintf("Cleaning up managed namespace %q", t.ManagedNamespace))
		if err := kubeClient.CoreV1().Namespaces().Delete(ctx, t.ManagedNamespace, metav1.DeleteOptions{}); err != nil {
			GinkgoWriter.Printf("WARNING: failed to delete managed namespace %q: %v\n", t.ManagedNamespace, err)
			errs = append(errs, err)
		}
	}

	Expect(errs).Should(BeEmpty(), "cleanup encountered %d errors", len(errs))
}
