package release

import (
	"context"
	"fmt"

	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with release objects CRUD operations.
type ReleasesInterface interface {
	// Creates a release.
	CreateRelease(name, namespace, snapshot, releasePlan string) (*releaseApi.Release, error)

	// Returns a release.
	GetRelease(releaseName, snapshotName, namespace string) (*releaseApi.Release, error)

	// Returns all releases in a namespace.
	GetReleases(namespace string) (*releaseApi.ReleaseList, error)

	// Returns the first release in a namespace.
	GetFirstReleaseInNamespace(namespace string) (*releaseApi.Release, error)
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
	releaseList := &releaseApi.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := r.KubeRest().List(context.TODO(), releaseList, opts...)
	if err != nil || len(releaseList.Items) < 1 {
		return nil, fmt.Errorf("could not find any Releases in namespace %s: %+v", namespace, err)
	}
	return &releaseList.Items[0], nil
}
