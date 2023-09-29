package common

import (
	"context"
	"fmt"
	"time"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/logs"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSnapshotEnvironmentBinding returns the SnapshotEnvironmentBinding related to the given App and Environment
func (s *SuiteController) GetSnapshotEnvironmentBinding(applicationName string, namespace string, environment *appservice.Environment) (*appservice.SnapshotEnvironmentBinding, error) {
	snapshotEnvironmentBindingList := &appservice.SnapshotEnvironmentBindingList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := s.KubeRest().List(context.TODO(), snapshotEnvironmentBindingList, opts...)
	if err != nil {
		return nil, err
	}

	for _, binding := range snapshotEnvironmentBindingList.Items {
		if binding.Spec.Application == applicationName && binding.Spec.Environment == environment.Name {
			return &binding, nil
		}
	}

	return nil, fmt.Errorf("no SnapshotEnvironmentBinding found in environment %s %s", environment.Name, utils.GetAdditionalInfo(applicationName, namespace))
}

// DeleteAllSnapshotEnvBindingsInASpecificNamespace removes all snapshotEnvironmentBindings from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (s *SuiteController) DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := s.KubeRest().DeleteAllOf(context.TODO(), &appservice.SnapshotEnvironmentBinding{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting snapshotEnvironmentBindings from the namespace %s: %+v", namespace, err)
	}

	snapshotEnvironmentBindingList := &appservice.SnapshotEnvironmentBindingList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := s.KubeRest().List(context.Background(), snapshotEnvironmentBindingList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(snapshotEnvironmentBindingList.Items) == 0, nil
	}, timeout)
}

// ListAllSnapshotEnvBindings returns a list of all SnapshotEnvBindings in a given namespace.
func (s *SuiteController) ListAllSnapshotEnvBindings(namespace string) (*appservice.SnapshotEnvironmentBindingList, error) {
	snapshotEnvironmentBindingList := &appservice.SnapshotEnvironmentBindingList{}
	err := s.KubeRest().List(context.Background(), snapshotEnvironmentBindingList, &rclient.ListOptions{Namespace: namespace})

	return snapshotEnvironmentBindingList, err
}

// StoreSnapshotEnvBinding stores a SnapshotEnvBinding as an artifact.
func (s *SuiteController) StoreSnapshotEnvBinding(snapshotEnvBinding *appservice.SnapshotEnvironmentBinding) error {
	return logs.StoreResourceYaml(snapshotEnvBinding, "snapshotEnvBinding-"+snapshotEnvBinding.Name)
}

// StoreAllSnapshotEnvironmentBindings stores all SnapshotEnvBindings in a given namespace.
func (s *SuiteController) StoreAllSnapshotEnvironmentBindings(namespace string) error {
	snapshotEnvBindingsList, err := s.ListAllSnapshotEnvBindings(namespace)
	if err != nil {
		return err
	}

	for _, snapshotEnvBinding := range snapshotEnvBindingsList.Items {
		if err := s.StoreSnapshotEnvBinding(&snapshotEnvBinding); err != nil {
			return err
		}
	}
	return nil
}
