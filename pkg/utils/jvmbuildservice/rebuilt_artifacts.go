package jvmbuildservice

import (
	"context"

	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListRebuiltArtifacts returns a list of all RebuiltArtifacts in a given namespace.
func (j *JvmbuildserviceController) ListRebuiltArtifacts(namespace string) (*v1alpha1.RebuiltArtifactList, error) {
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().RebuiltArtifacts(namespace).List(context.Background(), metav1.ListOptions{})
}
