package has

import (
	"context"
	"fmt"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/logs"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetComponentDetectionQuery return the status from the ComponentDetectionQuery Custom Resource object
func (h *HasController) GetComponentDetectionQuery(name, namespace string) (*appservice.ComponentDetectionQuery, error) {
	componentDetectionQuery := &appservice.ComponentDetectionQuery{
		Spec: appservice.ComponentDetectionQuerySpec{},
	}

	if err := h.KubeRest().Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, componentDetectionQuery); err != nil {
		return nil, err
	}

	return componentDetectionQuery, nil
}

// CreateComponentDetectionQuery create a has componentdetectionquery from a given name, namespace, and git source
func (h *HasController) CreateComponentDetectionQuery(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool) (*appservice.ComponentDetectionQuery, error) {
	return h.CreateComponentDetectionQueryWithTimeout(name, namespace, gitSourceURL, gitSourceRevision, gitSourceContext, secret, isMultiComponent, 6*time.Minute)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()
	if err := h.KubeRest().Create(ctx, componentDetectionQuery); err != nil {
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
	if err := h.KubeRest().DeleteAllOf(context.Background(), &appservice.ComponentDetectionQuery{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting component detection queries from the namespace %s: %+v", namespace, err)
	}

	return utils.WaitUntil(func() (done bool, err error) {
		componentDetectionQueriesList, err := h.ListAllComponentDetectionQueries(namespace)
		if err != nil {
			return false, nil
		}
		return len(componentDetectionQueriesList.Items) == 0, nil
	}, timeout)
}

// ListAllComponentDetectionQueries returns a list of all ComponentDetectionQueries in a given namespace.
func (h *HasController) ListAllComponentDetectionQueries(namespace string) (*appservice.ComponentDetectionQueryList, error) {
	componentDetectionQueryList := &appservice.ComponentDetectionQueryList{}
	err := h.KubeRest().List(context.Background(), componentDetectionQueryList, &rclient.ListOptions{Namespace: namespace})
	return componentDetectionQueryList, err
}

// StoreComponentDetectionQuery stores a given ComponentDetectionQuery as an artifact.
func (h *HasController) StoreComponentDetectionQuery(ComponentDetectionQuery *appservice.ComponentDetectionQuery) error {
	return logs.StoreResourceYaml(ComponentDetectionQuery, "componentDetectionQuery-"+ComponentDetectionQuery.Name)
}

// StoreAllComponentDetectionQueries stores all ComponentDetectionQueries in a given namespace.
func (h *HasController) StoreAllComponentDetectionQueries(namespace string) error {
	componentDetectionQueryList, err := h.ListAllComponentDetectionQueries(namespace)
	if err != nil {
		return err
	}

	for _, componentDetectionQuery := range componentDetectionQueryList.Items {
		if err := h.StoreComponentDetectionQuery(&componentDetectionQuery); err != nil {
			return err
		}
	}
	return nil
}

// UpdateComponent updates a component
func (h *HasController) UpdateComponent(component *appservice.Component) error {
	err := h.KubeRest().Update(context.Background(), component, &rclient.UpdateOptions{})

	if err != nil {
		return err
	}
	return nil
}
