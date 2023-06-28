package release

import (
	"context"

	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	releaseMetadata "github.com/redhat-appstudio/release-service/metadata"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Contains all methods related with plan objects CRUD operations.
type PlansInterface interface {
	// Creates a release plan.
	CreateReleasePlan(name, namespace, application, targetNamespace, autoReleaseLabel string) (*releaseApi.ReleasePlan, error)

	// Creates a release plan admission.
	CreateReleasePlanAdmission(name, originNamespace, application, namespace, environment, autoRelease, releaseStrategy string) (*releaseApi.ReleasePlanAdmission, error)

	// Returns a release plan.
	GetReleasePlan(name, namespace string) (*releaseApi.ReleasePlan, error)

	// Returns a release plan admission.
	GetReleasePlanAdmission(name, namespace string) (*releaseApi.ReleasePlanAdmission, error)

	// Deletes a release plan.
	DeleteReleasePlan(name, namespace string, failOnNotFound bool) error

	// Deletes a release plan admission.
	DeleteReleasePlanAdmission(name, namespace string, failOnNotFound bool) error
}

// CreateReleasePlan creates a new ReleasePlan using the given parameters.
func (r *releaseFactory) CreateReleasePlan(name, namespace, application, targetNamespace, autoReleaseLabel string) (*releaseApi.ReleasePlan, error) {
	var releasePlan *releaseApi.ReleasePlan = &releaseApi.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Name:         name,
			Namespace:    namespace,
			Labels: map[string]string{
				releaseMetadata.AutoReleaseLabel: autoReleaseLabel,
				releaseMetadata.AttributionLabel: "true",
			},
		},
		Spec: releaseApi.ReleasePlanSpec{
			DisplayName: name,
			Application: application,
			Target:      targetNamespace,
		},
	}
	if autoReleaseLabel == "" || autoReleaseLabel == "true" {
		releasePlan.ObjectMeta.Labels[releaseMetadata.AutoReleaseLabel] = "true"
	} else {
		releasePlan.ObjectMeta.Labels[releaseMetadata.AutoReleaseLabel] = "false"
	}

	return releasePlan, r.KubeRest().Create(context.TODO(), releasePlan)
}

// CreateReleasePlanAdmission creates a new ReleasePlanAdmission using the given parameters.
func (r *releaseFactory) CreateReleasePlanAdmission(name, originNamespace, application, namespace, environment, autoRelease, releaseStrategy string) (*releaseApi.ReleasePlanAdmission, error) {
	var releasePlanAdmission *releaseApi.ReleasePlanAdmission = &releaseApi.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleasePlanAdmissionSpec{
			DisplayName:     name,
			Application:     application,
			Origin:          originNamespace,
			Environment:     environment,
			ReleaseStrategy: releaseStrategy,
		},
	}
	if autoRelease != "" {
		releasePlanAdmission.ObjectMeta.Labels[releaseMetadata.AutoReleaseLabel] = autoRelease
	}
	return releasePlanAdmission, r.KubeRest().Create(context.TODO(), releasePlanAdmission)
}

// GetReleasePlan returns the ReleasePlan with the given name in the given namespace.
func (r *releaseFactory) GetReleasePlan(name, namespace string) (*releaseApi.ReleasePlan, error) {
	releasePlan := &releaseApi.ReleasePlan{}

	err := r.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlan)

	return releasePlan, err
}

// GetReleasePlanAdmission returns the ReleasePlanAdmission with the given name in the given namespace.
func (r *releaseFactory) GetReleasePlanAdmission(name, namespace string) (*releaseApi.ReleasePlanAdmission, error) {
	releasePlanAdmission := &releaseApi.ReleasePlanAdmission{}

	err := r.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// DeleteReleasePlan deletes a given ReleasePlan name in given namespace.
func (r *releaseFactory) DeleteReleasePlan(name, namespace string, failOnNotFound bool) error {
	releasePlan := &releaseApi.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := r.KubeRest().Delete(context.TODO(), releasePlan)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// DeleteReleasePlanAdmission deletes the ReleasePlanAdmission resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
// specify 'false', if it's likely the ReleasePlanAdmission has already been deleted (for example, because the Namespace was deleted)
func (r *releaseFactory) DeleteReleasePlanAdmission(name, namespace string, failOnNotFound bool) error {
	releasePlanAdmission := releaseApi.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := r.KubeRest().Delete(context.TODO(), &releasePlanAdmission)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}
