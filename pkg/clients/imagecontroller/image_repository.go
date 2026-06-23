package imagecontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/image-controller/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateImageRepositoryCR creates new ImageRepository
func (i *ImageController) CreateImageRepositoryCR(imageRepoName, namespace, visibility, userDefinedImageName, applicationName, componentName string, addUpdateAnnotation bool) (*v1alpha1.ImageRepository, error) {
	var imageParams v1alpha1.ImageParameters
	if userDefinedImageName != "" {
		imageParams = v1alpha1.ImageParameters{Name: userDefinedImageName, Visibility: v1alpha1.ImageVisibility(visibility)}
	} else {
		imageParams = v1alpha1.ImageParameters{Visibility: v1alpha1.ImageVisibility(visibility)}
	}

	var labels map[string]string
	if applicationName != "" && componentName != "" {
		labels = map[string]string{"appstudio.redhat.com/application": applicationName, "appstudio.redhat.com/component": componentName}
	}

	var annotations map[string]string
	if addUpdateAnnotation {
		annotations = map[string]string{"image-controller.appstudio.redhat.com/update-component-image": "true"}
	}

	imageRepository := &v1alpha1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:        imageRepoName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1alpha1.ImageRepositorySpec{
			Image: imageParams,
		},
	}

	err := i.KubeRest().Create(context.Background(), imageRepository)
	if err != nil {
		return nil, err
	}
	return imageRepository, nil
}

// WaitForImageRepositoryToBeReady waits for the image repository status to be in ready state
func (i *ImageController) WaitForImageRepositoryToBeReady(name, namespace string) error {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}

	err := utils.WaitUntil(func() (done bool, err error) {
		if err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository); err != nil {
			fmt.Printf("Image repository %q do not have right state ('%s' != 'ready') yet but it has status %v.\n", name, imageRepository.Status.State, imageRepository.Status)
			return false, nil
		}
		return imageRepository.Status.State == "ready", nil
	}, 2*time.Minute)

	return err
}

// GetImageRepositoryCR returns the requested ImageRepository object
func (i *ImageController) GetImageRepositoryCR(name, namespace string) (*v1alpha1.ImageRepository, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	imageRepository := v1alpha1.ImageRepository{}

	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return nil, err
	}
	return &imageRepository, nil
}

// GetGenerateTimestamp return the generationTimestamp of credentials of image repository
func (i *ImageController) GetGenerateTimestamp(imageRepoName, namespace string) (string, error) {
	namespacedName := types.NamespacedName{
		Name:      imageRepoName,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}
	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return "", err
	}
	return imageRepository.Status.Credentials.GenerationTimestamp.String(), nil
}

// DeleteImageRepositoryCR removes the ImageRepository object
func (i *ImageController) DeleteImageRepositoryCR(name, namespace string) error {
	imageRepository := v1alpha1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := i.KubeRest().Delete(context.Background(), &imageRepository); err != nil {
		if !k8sErrors.IsNotFound(err) {
			return fmt.Errorf("error deleting a imageRepository: %+v", err)
		}
	}
	return nil
}

// GetImageRepositoryByComponent returns the ImageRepository CR for the component
func (i *ImageController) GetImageRepositoryByComponent(namespace, componentName string) (*v1alpha1.ImageRepository, error) {
	imageRepositoryList := &v1alpha1.ImageRepositoryList{}
	imageRepoLabels := map[string]string{"appstudio.redhat.com/component": componentName}
	err := i.KubeRest().List(context.Background(), imageRepositoryList, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(imageRepoLabels), Namespace: namespace})
	if err != nil {
		return nil, err
	}
	if len(imageRepositoryList.Items) == 0 {
		return nil, fmt.Errorf("no image repository found for component %s in namespace %s", componentName, namespace)
	}
	return &imageRepositoryList.Items[0], nil
}

// ChangeVisibilityToPrivate changes ImageRepository visibility to private
func (i *ImageController) ChangeVisibilityToPrivate(namespace, applicationName, componentName string) (*v1alpha1.ImageRepository, error) {
	imageRepositoryList := &v1alpha1.ImageRepositoryList{}
	imageRepoLabels := map[string]string{"appstudio.redhat.com/component": componentName}
	err := i.KubeRest().List(context.Background(), imageRepositoryList, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(imageRepoLabels), Namespace: namespace})
	if err != nil {
		return nil, err
	}
	imageRepository := &imageRepositoryList.Items[0]
	// update visibility to private
	imageRepository.Spec.Image.Visibility = "private"

	err = i.KubeRest().Update(context.Background(), imageRepository)
	if err != nil {
		return nil, err
	}
	return imageRepository, nil
}

// RegenerateToken will set the spec.credentials.regenerate-token to true for credential rotation of robot accounts
func (i *ImageController) RegenerateToken(imageRepoCRName, namespace string) error {
	namespacedName := types.NamespacedName{
		Name:      imageRepoCRName,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}
	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return err
	}

	regenerateToken := true
	credentials := &v1alpha1.ImageCredentials{
		RegenerateToken: &regenerateToken,
	}
	imageRepository.Spec.Credentials = credentials

	err = i.KubeRest().Update(context.Background(), &imageRepository)
	if err != nil {
		return err
	}
	return nil
}

// VerifyLinking will set the spec.credentials.verify-linking to true for verifying the secret linking to service account
func (i *ImageController) VerifyLinking(imageRepoCRName, namespace string) error {
	namespacedName := types.NamespacedName{
		Name:      imageRepoCRName,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}
	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return err
	}

	verifyLinking := true
	credentials := &v1alpha1.ImageCredentials{
		VerifyLinking: &verifyLinking,
	}
	imageRepository.Spec.Credentials = credentials

	err = i.KubeRest().Update(context.Background(), &imageRepository)
	if err != nil {
		return err
	}
	return nil
}

// GetImageName returns the image repo name for the component
func (i *ImageController) GetImageName(namespace, componentName string) (string, error) {
	imageRepositoryList := &v1alpha1.ImageRepositoryList{}
	imageRepoLabels := map[string]string{"appstudio.redhat.com/component": componentName}
	err := i.KubeRest().List(context.Background(), imageRepositoryList, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(imageRepoLabels), Namespace: namespace})
	if err != nil {
		return "", err
	}
	return imageRepositoryList.Items[0].Spec.Image.Name, err
}

// GetImageNameFromImageRepositoryCR returns the image repo name from the image repository CR
func (i *ImageController) GetImageNameFromImageRepositoryCR(namespace, imageRepoCRName string) (string, error) {
	namespacedName := types.NamespacedName{
		Name:      imageRepoCRName,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}

	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return "", err
	}
	return imageRepository.Spec.Image.Name, err
}

// GetRobotAccounts returns the pull and push robot accounts for the component
func (i *ImageController) GetRobotAccounts(namespace, componentName string) (string, string, error) {
	imageRepositoryList := &v1alpha1.ImageRepositoryList{}
	imageRepoLabels := map[string]string{"appstudio.redhat.com/component": componentName}
	err := i.KubeRest().List(context.Background(), imageRepositoryList, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(imageRepoLabels), Namespace: namespace})
	if err != nil {
		return "", "", err
	}
	return imageRepositoryList.Items[0].Status.Credentials.PullRobotAccountName, imageRepositoryList.Items[0].Status.Credentials.PushRobotAccountName, nil
}

// GetRobotAccountsFromImageRepositoryCR returns the pull and push robot accounts from the image repository CR
func (i *ImageController) GetRobotAccountsFromImageRepositoryCR(namespace, imageRepoCRName string) (string, string, error) {
	namespacedName := types.NamespacedName{
		Name:      imageRepoCRName,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}

	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return "", "", err
	}

	return imageRepository.Status.Credentials.PullRobotAccountName, imageRepository.Status.Credentials.PushRobotAccountName, nil
}

// GetSecretsFromImageRepositoryCR returns the pull and push secrets from the image repository CR
func (i *ImageController) GetSecretsFromImageRepositoryCR(namespace, imageRepoCRName string) (string, string, error) {
	namespacedName := types.NamespacedName{
		Name:      imageRepoCRName,
		Namespace: namespace,
	}
	imageRepository := v1alpha1.ImageRepository{}

	err := i.KubeRest().Get(context.Background(), namespacedName, &imageRepository)
	if err != nil {
		return "", "", err
	}

	return imageRepository.Status.Credentials.PullSecretName, imageRepository.Status.Credentials.PushSecretName, nil
}

// IsVisibilityPublic returns true if imageRepository CR has spec.image.visibility == "public", otherwise false
func (i *ImageController) IsVisibilityPublic(namespace, componentName string) (bool, error) {
	imageRepositoryList := &v1alpha1.ImageRepositoryList{}
	imageRepoLabels := map[string]string{"appstudio.redhat.com/component": componentName}
	err := i.KubeRest().List(context.Background(), imageRepositoryList, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(imageRepoLabels), Namespace: namespace})
	if err != nil {
		return false, err
	}
	return imageRepositoryList.Items[0].Spec.Image.Visibility == "public", nil
}
