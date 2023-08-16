package has

import (
	"context"
	"fmt"
	"time"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetComponentDetectionQuery return the status from the ComponentDetectionQuery Custom Resource object
func (h *HasController) GetComponentDetectionQuery(name, namespace string) (*appservice.ComponentDetectionQuery, error) {
	componentDetectionQuery := &appservice.ComponentDetectionQuery{
		Spec: appservice.ComponentDetectionQuerySpec{},
	}

	if err := h.KubeRest().Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, componentDetectionQuery); err != nil {
		return nil, err
	}

	return componentDetectionQuery, nil
}

// CreateComponentDetectionQuery create a has componentdetectionquery from a given name, namespace, and git source
func (h *HasController) CreateComponentDetectionQuery(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool) (*appservice.ComponentDetectionQuery, error) {
	return h.CreateComponentDetectionQueryWithTimeout(name, namespace, gitSourceURL, gitSourceRevision, gitSourceContext, secret, isMultiComponent, 5*time.Minute)
}

// CreateComponentDetectionQueryWithTimeout create a has componentdetectionquery from a given name, namespace, and git source and waits for it to be read
func (h *HasController) CreateComponentDetectionQueryWithTimeout(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool, timeout time.Duration) (*appservice.ComponentDetectionQuery, error) {
	componentDetectionQuery := &appservice.ComponentDetectionQuery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appservice.ComponentDetectionQuerySpec{
			GitSource: appservice.GitSource{
				URL:      gitSourceURL,
				Revision: gitSourceRevision,
				Context:  gitSourceContext,
			},
			Secret:                secret,
			GenerateComponentName: true,
		},
	}

	if err := h.KubeRest().Create(context.TODO(), componentDetectionQuery); err != nil {
		return nil, err
	}

	err := utils.WaitUntil(func() (done bool, err error) {
		componentDetectionQuery, err = h.GetComponentDetectionQuery(componentDetectionQuery.Name, componentDetectionQuery.Namespace)
		if err != nil {
			return false, err
		}
		for _, condition := range componentDetectionQuery.Status.Conditions {
			if condition.Type == "Completed" && len(componentDetectionQuery.Status.ComponentDetected) > 0 {
				return true, nil
			}
		}
		return false, nil
	}, timeout)

	if err != nil {
		return nil, fmt.Errorf("error waiting for cdq to be ready: %v", err)
	}

	return componentDetectionQuery, nil
}

// DeleteAllComponentDetectionQueriesInASpecificNamespace removes all CDQs CRs from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *HasController) DeleteAllComponentDetectionQueriesInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &appservice.ComponentDetectionQuery{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting component detection queries from the namespace %s: %+v", namespace, err)
	}

	componentDetectionQueriesList := &appservice.ComponentDetectionQueryList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), componentDetectionQueriesList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(componentDetectionQueriesList.Items) == 0, nil
	}, timeout)
}
