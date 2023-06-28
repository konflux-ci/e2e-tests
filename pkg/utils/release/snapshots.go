package release

import (
	"context"
	"fmt"
	"time"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with snapshot objects CRUD operations.
type SnapshotsInterface interface {
	// Creates a snapshot.
	CreateSnapshot(name string, namespace string, applicationName string, snapshotComponents []appstudioApi.SnapshotComponent) (*appstudioApi.Snapshot, error)

	// Returns the first snapshot in a namespace.
	GetSnapshotByComponent(namespace string) (*appstudioApi.Snapshot, error)

	// Deletes all snapshot in a namespace.
	DeleteAllSnapshotsInASpecificNamespace(namespace string, timeout time.Duration) error
}

// CreateSnapshot creates a Snapshot using the given parameters.
func (r *releaseFactory) CreateSnapshot(name string, namespace string, applicationName string, snapshotComponents []appstudioApi.SnapshotComponent) (*appstudioApi.Snapshot, error) {
	snapshot := &appstudioApi.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appstudioApi.SnapshotSpec{
			Application: applicationName,
			Components:  snapshotComponents,
		},
	}
	return snapshot, r.KubeRest().Create(context.TODO(), snapshot)
}

// GetSnapshotByComponent returns the first snapshot in namespace if exist, else will return nil
func (r *releaseFactory) GetSnapshotByComponent(namespace string) (*appstudioApi.Snapshot, error) {
	snapshot := &appstudioApi.SnapshotList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"test.appstudio.openshift.io/type": "component",
		},
		client.InNamespace(namespace),
	}
	err := r.KubeRest().List(context.TODO(), snapshot, opts...)

	if err == nil && len(snapshot.Items) > 0 {
		return &snapshot.Items[0], nil
	}
	return nil, err
}

// DeleteAllSnapshotsInASpecificNamespace removes all snapshots from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (r *releaseFactory) DeleteAllSnapshotsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := r.KubeRest().DeleteAllOf(context.TODO(), &appstudioApi.Snapshot{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting snapshots from the namespace %s: %+v", namespace, err)
	}

	snapshotList := &appstudioApi.SnapshotList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := r.KubeRest().List(context.Background(), snapshotList, &client.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(snapshotList.Items) == 0, nil
	}, timeout)
}
