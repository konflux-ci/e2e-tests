package jvmbuildservice

import (
	"context"

	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListDependencyBuilds returns a list of all dependencyBuilds in a given namespace.
func (j *JvmbuildserviceController) ListDependencyBuilds(namespace string) (*v1alpha1.DependencyBuildList, error) {
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().DependencyBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
}

// DeleteDependencyBuild removes a dependencyBuilds from a given namespace.
func (j *JvmbuildserviceController) DeleteDependencyBuild(name, namespace string) error {
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().DependencyBuilds(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
