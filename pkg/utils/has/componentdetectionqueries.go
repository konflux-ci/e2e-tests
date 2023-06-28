package has

import (
	"context"
	"fmt"
	"time"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Contains all methods related with componentdetectionquery objects CRUD operations.
type ComponentDetectionQueriesInterface interface {
	// Returns an componentdetectionquery obj from the kubernetes cluster.
	GetComponentDetectionQuery(name string, namespace string) (*appservice.ComponentDetectionQuery, error)

	// Creates an componentdetectionquery object in the kubernetes cluster.
	CreateComponentDetectionQuery(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool) (*appservice.ComponentDetectionQuery, error)

	// Creates an componentdetectionquery object in the kubernetes cluster and wait for a period of given timeout.
	CreateComponentDetectionQueryWithTimeout(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool, timeout time.Duration) (*appservice.ComponentDetectionQuery, error)
}

// GetComponentDetectionQuery return the status from the ComponentDetectionQuery Custom Resource object
func (h *hasFactory) GetComponentDetectionQuery(name, namespace string) (*appservice.ComponentDetectionQuery, error) {
	componentDetectionQuery := &appservice.ComponentDetectionQuery{
		Spec: appservice.ComponentDetectionQuerySpec{},
	}

	if err := h.KubeRest().Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, componentDetectionQuery); err != nil {
		return nil, err
	}

	return componentDetectionQuery, nil
}

// CreateComponentDetectionQuery create a has componentdetectionquery from a given name, namespace, and git source
func (h *hasFactory) CreateComponentDetectionQuery(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool) (*appservice.ComponentDetectionQuery, error) {
	return h.CreateComponentDetectionQueryWithTimeout(name, namespace, gitSourceURL, gitSourceRevision, gitSourceContext, secret, isMultiComponent, 5*time.Minute)
}

// CreateComponentDetectionQueryWithTimeout create a has componentdetectionquery from a given name, namespace, and git source and waits for it to be read
func (h *hasFactory) CreateComponentDetectionQueryWithTimeout(name string, namespace string, gitSourceURL string, gitSourceRevision string, gitSourceContext string, secret string, isMultiComponent bool, timeout time.Duration) (*appservice.ComponentDetectionQuery, error) {
	componentDetectionQuery := &appservice.ComponentDetectionQuery{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", name),
			Namespace:    namespace,
		},
		Spec: appservice.ComponentDetectionQuerySpec{
			GitSource: appservice.GitSource{
				URL:      gitSourceURL,
				Revision: gitSourceRevision,
				Context:  gitSourceContext,
			},
			Secret: secret,
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
