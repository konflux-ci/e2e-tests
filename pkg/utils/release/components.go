package release

import (
	"context"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// Contains all methods related with component objects CRUD operations.
type ComponentsInterface interface {
	// Creates a component based on container image.
	CreateComponentWithDockerSource(applicationName, componentName, namespace, gitSourceURL, containerImageSource, outputContainerImage, secret string) (*appstudioApi.Component, error)
}

// CreateComponentWithDockerSource creates a component based on container image source.
func (r *releaseFactory) CreateComponentWithDockerSource(applicationName, componentName, namespace, gitSourceURL, containerImageSource, outputContainerImage, secret string) (*appstudioApi.Component, error) {
	component := &appstudioApi.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: appstudioApi.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appstudioApi.ComponentSource{
				ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						URL:           gitSourceURL,
						DockerfileURL: containerImageSource,
					},
				},
			},
			Secret:         secret,
			ContainerImage: outputContainerImage,
			Replicas:       pointer.Int(1),
			TargetPort:     8081,
			Route:          "",
		},
	}
	err := r.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	return component, nil
}
