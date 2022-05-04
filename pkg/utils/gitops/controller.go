package gitops

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	routev1 "github.com/openshift/api/route/v1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend/apis/managed-gitops/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

func (h *SuiteController) CreateGitOpsDeployment(name string, namespace string, repoUrl string, repoPath string, repoRevision string) (*managedgitopsv1alpha1.GitOpsDeployment, error) {
	gitOpsDeployment := &managedgitopsv1alpha1.GitOpsDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: managedgitopsv1alpha1.GitOpsDeploymentSpec{
			Source: managedgitopsv1alpha1.ApplicationSource{
				RepoURL:        repoUrl,
				Path:           repoPath,
				TargetRevision: repoRevision,
			},
			Type: managedgitopsv1alpha1.GitOpsDeploymentSpecType_Automated,
		},
	}

	err := h.KubeRest().Create(context.TODO(), gitOpsDeployment)
	if err != nil {
		return nil, err
	}
	return gitOpsDeployment, nil
}

// DeleteGitOpsDeployment deletes an gitops deployment from a given name and namespace
func (h *SuiteController) DeleteGitOpsDeployment(name string, namespace string) error {
	gitOpsDeployment := &managedgitopsv1alpha1.GitOpsDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), gitOpsDeployment)
}

// GetComponentRoute returns the route for a given component name
func (h *SuiteController) GetComponentRoute(componentName string, componentNamespace string) (*routev1.Route, error) {
	namespacedName := types.NamespacedName{
		Name:      componentName,
		Namespace: componentNamespace,
	}

	route := &routev1.Route{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, route)
	if err != nil {
		return &routev1.Route{}, err
	}
	return route, nil
}

// GetComponentDeployment returns the deployment for a given component name
func (h *SuiteController) GetComponentDeployment(componentName string, componentNamespace string) (*appsv1.Deployment, error) {
	namespacedName := types.NamespacedName{
		Name:      componentName,
		Namespace: componentNamespace,
	}

	deployment := &appsv1.Deployment{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		return &appsv1.Deployment{}, err
	}
	return deployment, nil
}

// GetComponentService returns the service for a given component name
func (h *SuiteController) GetComponentService(componentName string, componentNamespace string) (*corev1.Service, error) {
	namespacedName := types.NamespacedName{
		Name:      componentName,
		Namespace: componentNamespace,
	}

	service := &corev1.Service{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, service)
	if err != nil {
		return &corev1.Service{}, err
	}
	return service, nil
}

//GetGitOpsDeployedImage return the image used by the given component deployment
func (h *SuiteController) GetGitOpsDeployedImage(componentName string, componentNamespace string) (string, error) {
	deployment, err := h.GetComponentDeployment(componentName, componentNamespace)
	if err != nil {
		return "", err
	}
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		return deployment.Spec.Template.Spec.Containers[0].Image, nil
	} else {
		return "", fmt.Errorf("error when getting the '%s' deployed image", componentName)
	}
}

//Checks that the deployed backend component is actually reachable and returns 200
func (h *SuiteController) CheckGitOpsEndpoint(componentName string, componentNamespace string) error {
	route, err := h.GetComponentRoute(componentName, componentNamespace)
	if err != nil {
		return fmt.Errorf("error when getting the '%s' route", componentName)
	}

	if len(route.Spec.Host) > 0 {
		routeUrl := "http://" + route.Spec.Host

		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		resp, err := http.Get(routeUrl)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("route responded with '%d' status code", resp.StatusCode)
		}
	} else {
		return fmt.Errorf("route is invalid: '%s'", route.Spec.Host)
	}

	return nil
}
