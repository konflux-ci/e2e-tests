package release

import (
	"context"
	"fmt"

	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with release objects CRUD operations.
type ReleasesInterface interface {
	// Creates a release.
	CreateRelease(name, namespace, snapshot, releasePlan string) (*releaseApi.Release, error)

	//Creates a pipeline RoleBinding for service account.
	CreateReleasePipelineRoleBindingForServiceAccount(namespace string, serviceAccount *corev1.ServiceAccount) (*rbac.RoleBinding, error)

	// Returns a release.
	GetRelease(releaseName, snapshotName, namespace string) (*releaseApi.Release, error)

	// Returns all releases in a namespace.
	GetReleases(namespace string) (*releaseApi.ReleaseList, error)

	// Returns the first release in a namespace.
	GetFirstReleaseInNamespace(namespace string) (*releaseApi.Release, error)

	// Returns pipelirun referencing given release.
	GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*v1beta1.PipelineRun, error)
}

// CreateRelease creates a new Release using the given parameters.
func (r *releaseFactory) CreateRelease(name, namespace, snapshot, releasePlan string) (*releaseApi.Release, error) {
	release := &releaseApi.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseSpec{
			Snapshot:    snapshot,
			ReleasePlan: releasePlan,
		},
	}

	return release, r.KubeRest().Create(context.TODO(), release)
}

// CreateReleasePipelineRoleBindingForServiceAccount creates a RoleBinding for the passed serviceAccount to enable
// retrieving the necessary CRs from the passed namespace.
func (r *releaseFactory) CreateReleasePipelineRoleBindingForServiceAccount(namespace string, serviceAccount *corev1.ServiceAccount) (*rbac.RoleBinding, error) {
	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "release-service-pipeline-rolebinding-",
			Namespace:    namespace,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     "release-pipeline-resource-role",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}
	err := r.KubeRest().Create(context.TODO(), roleBinding)
	if err != nil {
		return nil, err
	}
	return roleBinding, nil
}

// GetRelease returns the release with in the given namespace.
// It can find a Release CR based on provided name or a name of an associated Snapshot
func (r *releaseFactory) GetRelease(releaseName, snapshotName, namespace string) (*releaseApi.Release, error) {
	ctx := context.Background()
	if len(releaseName) > 0 {
		release := &releaseApi.Release{}
		err := r.KubeRest().Get(ctx, types.NamespacedName{Name: releaseName, Namespace: namespace}, release)
		if err != nil {
			return nil, fmt.Errorf("failed to get Release with name '%s' in '%s' namespace", releaseName, namespace)
		}
		return release, nil
	}
	releaseList := &releaseApi.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	if err := r.KubeRest().List(context.TODO(), releaseList, opts...); err != nil {
		return nil, err
	}
	for _, r := range releaseList.Items {
		if len(snapshotName) > 0 && r.Spec.Snapshot == snapshotName {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("could not find Release CR based on associated Snapshot '%s' in '%s' namespace", snapshotName, namespace)
}

// GetRelease returns the list of Release CR in the given namespace.
func (r *releaseFactory) GetReleases(namespace string) (*releaseApi.ReleaseList, error) {
	releaseList := &releaseApi.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	err := r.KubeRest().List(context.TODO(), releaseList, opts...)

	return releaseList, err
}

// GetFirstReleaseInNamespace returns the first Release from  list of releases in the given namespace.
func (r *releaseFactory) GetFirstReleaseInNamespace(namespace string) (*releaseApi.Release, error) {
	releaseList, err := r.GetReleases(namespace)

	if err != nil || len(releaseList.Items) < 1 {
		return nil, fmt.Errorf("could not find any Releases in namespace %s: %+v", namespace, err)
	}
	return &releaseList.Items[0], nil
}

// GetPipelineRunInNamespace returns the Release PipelineRun referencing the given release.
func (r *releaseFactory) GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"release.appstudio.openshift.io/name":      releaseName,
			"release.appstudio.openshift.io/namespace": releaseNamespace,
		},
		client.InNamespace(namespace),
	}

	err := r.KubeRest().List(context.TODO(), pipelineRuns, opts...)

	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, fmt.Errorf("couldn't find PipelineRun in managed namespace '%s' for a release '%s' in '%s' namespace", namespace, releaseName, releaseNamespace)
}
