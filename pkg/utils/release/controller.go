package release

import (
	"context"
	"fmt"
	"io/ioutil"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	release "github.com/redhat-appstudio/release-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

// DeleteTestNamespace deletes a namespace
func (h *SuiteController) DeleteTestNamespace(name string) error {

	// Check if the E2E test namespace already exists
	_, err := h.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {

		fmt.Printf("error while querying for the %s namespace : %v\n", name, err)
		return err

	} else {
		err = h.KubeInterface().CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})

		if err != nil {
			fmt.Printf("error while deleting the %s namespace : %v\n", name, err)
			return err
		}
	}

	return nil
}

func (h *SuiteController) CheckIfNamespaceExists(name string) bool {

	// Check if the E2E test namespace already exists
	ns, err := h.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		return false
	}

	fmt.Printf("namespace %s status: %s \n", ns.Name, ns.Status.Phase)
	return true
}

// // GetHasApplicationStatus return the status from the Application Custom Resource object
// func (h *SuiteController) GetHasApplication(name, namespace string) (*appservice.Application, error) {
// 	namespacedName := types.NamespacedName{
// 		Name:      name,
// 		Namespace: namespace,
// 	}

// 	application := appservice.Application{
// 		Spec: appservice.ApplicationSpec{},
// 	}
// 	err := h.KubeRest().Get(context.TODO(), namespacedName, &application)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &application, nil
// }

func (s *SuiteController) GetReleaseStrategy(name string, namespace string) (*release.ReleaseStrategy, error) {

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	releaseStrategy := &release.ReleaseStrategy{}
	if err := s.KubeRest().Get(context.TODO(), namespacedName, releaseStrategy); err != nil {
		return nil, err
	}
	return releaseStrategy, nil

}

func (s *SuiteController) GetRelease(name string, namespace string) (*release.Release, error) {

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	release := &release.Release{}
	if err := s.KubeRest().Get(context.TODO(), namespacedName, release); err != nil {
		return nil, err
	}
	return release, nil

}

func (s *SuiteController) GetPipelineRun(name string, namespace string) (*v1beta1.PipelineRun, error) {

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	pipelineRun := &v1beta1.PipelineRun{}
	if err := s.KubeRest().Get(context.TODO(), namespacedName, pipelineRun); err != nil {
		return nil, err
	}
	return pipelineRun, nil

}

func (s *SuiteController) GetReleaseLink(name string, namespace string) (*release.ReleaseLink, error) {

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	releaseLink := &release.ReleaseLink{}
	if err := s.KubeRest().Get(context.TODO(), namespacedName, releaseLink); err != nil {
		return nil, err
	}
	return releaseLink, nil

}

func (s *SuiteController) CreateReleaseLink(name string, namespace string, displayName string, application string, target string, releaseStrategy string) (*release.ReleaseLink, error) {
	releaseLinkObj := release.ReleaseLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: release.ReleaseLinkSpec{
			DisplayName:     displayName,
			Application:     application,
			Target:          target,
			ReleaseStrategy: releaseStrategy,
		},
	}

	if releaseStrategy != "" {
		releaseLinkObj.Spec.ReleaseStrategy = releaseStrategy
	}

	err := s.KubeRest().Create(context.TODO(), &releaseLinkObj)
	return &releaseLinkObj, err
}

func (s *SuiteController) CreateReleaseStrategy(name string, namespace string, pipelineName string, bundle string) (*release.ReleaseStrategy, error) {
	releaseStrObj := release.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: release.ReleaseStrategySpec{
			Pipeline: pipelineName,
			Bundle:   bundle,
		},
	}
	err := s.KubeRest().Create(context.TODO(), &releaseStrObj)
	return &releaseStrObj, err
}

// Create a secret, return secret and error
func (s *SuiteController) CreateSecretV2(secret_yaml_file_path string) (*corev1.Secret, error) {

	decode := scheme.Codecs.UniversalDeserializer().Decode
	stream, _ := ioutil.ReadFile(secret_yaml_file_path)
	obj, gKV, _ := decode(stream, nil, nil)
	if gKV.Kind == "Secret" {
		secret := obj.(*corev1.Secret)
		fmt.Printf("Secret: %s for namespace: %s was created \n", secret, gKV)
	}

	return nil, nil
}

// Create a secret, return secret and error
func (s *SuiteController) CreateSecretV3(name string, namespace string) (*corev1.Secret, error) {

	secretType := corev1.SecretTypeDockerConfigJson
	secretData := map[string][]byte{".dockerconfigjson": []byte("ewogICJhdXRocyI6IHsKICAgICJxdWF5LmlvIjogewogICAgICAiYXV0aCI6ICJjbVZzWldGelpTMWxNbVVyY21Wc1pXRnpaVjlsTW1VNlVUTkNRa1JOTWtaTk4waFNUVWxHVWxsR09GaFNOVWROTlZkUlFqWklXVXRKTjBwWVJrbzJTMXBQVUV4WFJVOVVWamxUUVVOVk9WRkZXRGxTU2pCRlJ3PT0iLAogICAgICAiZW1haWwiOiAiIgogICAgfQogIH0KfQ==")}

	secretStrObj := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},

		Type: secretType,
		Data: secretData,
	}

	err := s.KubeRest().Create(context.TODO(), &secretStrObj)

	if err == nil {
		fmt.Printf("Secret: %s for namespace: %s was created \n", name, namespace)
	} else {
		fmt.Printf("Secret: %s for namespace: %s creation has failed \n", name, namespace)
	}

	return &secretStrObj, err
}

// Create a secret, return secret and error
func (s *SuiteController) CreateSecret(secret_yaml_file_path string) (*corev1.Secret, error) {

	// Read secret specification from YAML file

	bytes, err := ioutil.ReadFile(secret_yaml_file_path)
	if err != nil {
		return nil, err
	}

	var secretSpec corev1.Secret
	err = yaml.Unmarshal(bytes, &secretSpec)
	if err != nil {
		return nil, err
	}

	namespace := secretSpec.GetNamespace()
	name := secretSpec.GetName()

	secretStrObj := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},

		Type: secretSpec.Type,
		Data: secretSpec.Data,
	}

	err = s.KubeRest().Create(context.TODO(), &secretStrObj)

	if err == nil {
		fmt.Printf("Secret: %s for namespace: %s was created \n", name, namespace)
	} else {
		fmt.Printf("Secret: %s for namespace: %s creation has failed \n", name, namespace)
	}

	return &secretStrObj, err
}
