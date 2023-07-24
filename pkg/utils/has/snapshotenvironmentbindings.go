package has

import (
	"context"
	"fmt"
	"time"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteAllSnapshotEnvBindingsInASpecificNamespace removes all snapshotEnvironmentBindings from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *HasController) DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &appservice.SnapshotEnvironmentBinding{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting snapshotEnvironmentBindings from the namespace %s: %+v", namespace, err)
	}

	snapshotEnvironmentBindingList := &appservice.SnapshotEnvironmentBindingList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), snapshotEnvironmentBindingList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(snapshotEnvironmentBindingList.Items) == 0, nil
	}, timeout)
}
