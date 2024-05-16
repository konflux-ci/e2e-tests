package imagecontroller

import (
	"context"

	"github.com/konflux-ci/image-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateImageRepositoryCR creates new ImageRepository
func (i *ImageController) CreateImageRepositoryCR(name, namespace, applicationName, componentName string) (*v1alpha1.ImageRepository, error) {
	imageRepository := &v1alpha1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"appstudio.redhat.com/application": applicationName,
				"appstudio.redhat.com/component":   componentName,
			},
		},
	}

	err := i.KubeRest().Create(context.Background(), imageRepository)
	if err != nil {
		return nil, err
	}
	return imageRepository, nil
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
