package gitops

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	routev1 "github.com/openshift/api/route/v1"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	managedgitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/backend/apis/managed-gitops/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

func (h *SuiteController) CreateGitOpsCR(name string, namespace string, repoUrl string, repoPath string, repoRevision string) (*managedgitopsv1alpha1.GitOpsDeployment, error) {
	gitOpsDeployment := &managedgitopsv1alpha1.GitOpsDeployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    namespace,
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
func (h *SuiteController) DeleteGitOpsCR(name string, namespace string) error {
	gitOpsDeployment := &managedgitopsv1alpha1.GitOpsDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), gitOpsDeployment)
}

// GetGitOpsDeployedImage return the image used by the given component deployment
func (h *SuiteController) GetGitOpsDeployedImage(deployment *appsv1.Deployment) (string, error) {
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		return deployment.Spec.Template.Spec.Containers[0].Image, nil
	} else {
		return "", fmt.Errorf("error when getting the deployed image")
	}
}

// Checks that the deployed backend component is actually reachable and returns 200
func (h *SuiteController) CheckGitOpsEndpoint(route *routev1.Route, endpoint string) error {
	if len(route.Spec.Host) > 0 {
		routeUrl := "https://" + route.Spec.Host + endpoint

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

// Remove all gitopsdeployments from a given namespace. Useful when creating a lot of resources and want to remove all of them
func (h *SuiteController) DeleteAllGitOpsDeploymentInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &managedgitopsv1alpha1.GitOpsDeployment{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error when deleting gitopsdeployments in %s namespace: %+v", namespace, err)
	}

	gdList := &managedgitopsv1alpha1.GitOpsDeploymentList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err = h.KubeRest().List(context.Background(), gdList, client.InNamespace(namespace)); err != nil {
			return false, nil
		}
		return len(gdList.Items) == 0, nil
	}, timeout)
}

// CreateEnvironment creates a new environment
func (h *SuiteController) CreateEnvironment(name, namespace string) (*appservice.Environment, error) {
	environment := &appservice.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appservice.EnvironmentSpec{
			Configuration: appservice.EnvironmentConfiguration{
				Env: []appservice.EnvVarPair{},
			},
			DeploymentStrategy: appservice.DeploymentStrategy_AppStudioAutomated,
			DisplayName:        name,
			Type:               appservice.EnvironmentType_NonPOC,
		},
	}

	err := h.KubeRest().Create(context.TODO(), environment)
	if err != nil {
		return nil, err
	}
	return environment, nil
}

// DeleteAllEnvironmentsInASpecificNamespace removes all environments from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *SuiteController) DeleteAllEnvironmentsInASpecificNamespace(namespace string, timeout time.Duration) error {
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
