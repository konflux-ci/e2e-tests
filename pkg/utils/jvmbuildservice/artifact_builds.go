package jvmbuildservice

import (
	"context"

	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListArtifactBuilds returns a list of all artifactBuilds in a given namespace.
func (j *JvmbuildserviceController) ListArtifactBuilds(namespace string) (*v1alpha1.ArtifactBuildList, error) {
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
}

// DeleteArtifactBuild removes an artifactBuild from a given namespace.
func (j *JvmbuildserviceController) DeleteArtifactBuild(name, namespace string) error {
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
