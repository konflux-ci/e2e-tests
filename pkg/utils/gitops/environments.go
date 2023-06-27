package gitops

import (
	"context"
	"fmt"
	"time"

	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with Environments CRUD operations.
type EnvironmentsInterface interface {
	// List all environments in a given namespace from the kubernetes cluster.
	GetEnvironmentsList(namespace string) (*appservice.EnvironmentList, error)

	// Create an RHTAP environment pointing to a valid Kubernetes/Openshift cluster.
	CreateEphemeralEnvironment(name string, namespace string, targetNamespace string, serverApi string, clusterCredentialsSecret string, clusterType appservice.ConfigurationClusterType, kubeIngressDomain string) (*appservice.Environment, error)

	// Creates a new poc environment in the kubernetes cluster and returns the created object from the cluster.
	CreatePocEnvironment(name string, namespace string) (*appservice.Environment, error)

	// removes all environments from a specific namespace. Useful when creating a lot of resources and want to remove all of them.
	DeleteAllEnvironmentsInASpecificNamespace(namespace string, timeout time.Duration) error
}

// GetEnvironmentsList returns a list of environments in a given namespace from a kubernetes cluster.
func (h *gitopsFactory) GetEnvironmentsList(namespace string) (*appservice.EnvironmentList, error) {
	environmentList := &appservice.EnvironmentList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := h.KubeRest().List(context.TODO(), environmentList, opts...)

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error occurred while trying to list environments in %s namespace: %v", namespace, err)
	}

	return environmentList, nil
}

/*
* CreateEphemeralEnvironment: create an RHTAP environment pointing to a valid Kubernetes/Openshift cluster
* Args:
*	- name: Environment name
*	- namespace: Namespace where to create the environment. Note: Should be in the same namespace where cluster credential secret it is
*	- targetNamespace: Cluster namespace where to create Gitops resources
*	- serverApi: A valid API kubernetes server for a specific Kubernetes/Openshift cluster
*   - clusterCredentialsSecret: Secret with a valid kubeconfig credentials
*   - clusterType: Openshift/Kubernetes
*   - kubeIngressDomain: If clusterType == "Kubernetes", ingressDomain is mandatory and is enforced by the webhook validation
 */
func (g *gitopsFactory) CreateEphemeralEnvironment(name string, namespace string, targetNamespace string, serverApi string, clusterCredentialsSecret string, clusterType appservice.ConfigurationClusterType, kubeIngressDomain string) (*appservice.Environment, error) {
	ephemeralEnvironmentObj := &appservice.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appservice.EnvironmentSpec{
			DeploymentStrategy: appservice.DeploymentStrategy_AppStudioAutomated,
			Configuration: appservice.EnvironmentConfiguration{
				Env: []appservice.EnvVarPair{
					{
						Name:  "POC",
						Value: "POC",
					},
				},
			},
			UnstableConfigurationFields: &appservice.UnstableEnvironmentConfiguration{
				ClusterType: clusterType,
				KubernetesClusterCredentials: appservice.KubernetesClusterCredentials{
					TargetNamespace:            targetNamespace,
					APIURL:                     serverApi,
					ClusterCredentialsSecret:   clusterCredentialsSecret,
					AllowInsecureSkipTLSVerify: true,
				},
			},
		},
	}

	if clusterType == appservice.ConfigurationClusterType_Kubernetes {
		ephemeralEnvironmentObj.Spec.UnstableConfigurationFields.IngressDomain = kubeIngressDomain
	}

	if err := g.KubeRest().Create(context.TODO(), ephemeralEnvironmentObj); err != nil {
		if err != nil {
			if k8sErrors.IsAlreadyExists(err) {
				environment := &appservice.Environment{}

				err := g.KubeRest().Get(context.TODO(), types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, environment)

				return environment, err
			} else {
				return nil, err
			}
		}
	}

	return ephemeralEnvironmentObj, nil
}

// CreateEnvironment creates a new poc environment in the kubernetes cluster and returns the created object from the cluster.
func (h *gitopsFactory) CreatePocEnvironment(name string, namespace string) (*appservice.Environment, error) {
	environmentObject := &appservice.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appservice.EnvironmentSpec{
			Type:               "POC",
			DisplayName:        "my-environment",
			DeploymentStrategy: appservice.DeploymentStrategy_Manual,
			ParentEnvironment:  "",
			Tags:               []string{},
			Configuration: appservice.EnvironmentConfiguration{
				Env: []appservice.EnvVarPair{
					{
						Name:  "var_name",
						Value: "test",
					},
				},
			},
		},
	}

	if err := h.KubeRest().Create(context.TODO(), environmentObject); err != nil {
		if err != nil {
			if k8sErrors.IsAlreadyExists(err) {
				environment := &appservice.Environment{}

				err := h.KubeRest().Get(context.TODO(), types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				}, environment)

				return environment, err
			} else {
				return nil, err
			}
		}
	}
	return environmentObject, nil
}

// DeleteAllEnvironmentsInASpecificNamespace removes all environments from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *gitopsFactory) DeleteAllEnvironmentsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &appservice.Environment{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting environments from the namespace %s: %+v", namespace, err)
	}

	environmentList := &appservice.EnvironmentList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), environmentList, &client.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(environmentList.Items) == 0, nil
	}, timeout)
}
