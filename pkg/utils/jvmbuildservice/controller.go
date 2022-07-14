package jvmbuildservice

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteControler(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

func (s *SuiteController) ListArtifactBuilds(namespace string) (*v1alpha1.ArtifactBuildList, error) {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) DeleteArtifactBuild(name, namespace string) error {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (s *SuiteController) ListDependencyBuilds(namespace string) (*v1alpha1.DependencyBuildList, error) {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().DependencyBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) DeleteDependencyBuild(name, namespace string) error {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().DependencyBuilds(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
