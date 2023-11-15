package release

import (
	"context"
	"strconv"

	"github.com/redhat-appstudio/release-service/tekton/utils"
	"k8s.io/apimachinery/pkg/runtime"

	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	releaseMetadata "github.com/redhat-appstudio/release-service/metadata"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateReleasePlan creates a new ReleasePlan using the given parameters.
func (r *ReleaseController) CreateReleasePlan(name, namespace, application, targetNamespace, autoReleaseLabel string) (*releaseApi.ReleasePlan, error) {
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
			Application: application,
			Target:      targetNamespace,
		},
	}
	if autoReleaseLabel == "" || autoReleaseLabel == "true" {
		releasePlan.ObjectMeta.Labels[releaseMetadata.AutoReleaseLabel] = "true"
	} else {
		releasePlan.ObjectMeta.Labels[releaseMetadata.AutoReleaseLabel] = "false"
	}

	return releasePlan, r.KubeRest().Create(context.Background(), releasePlan)
}

// CreateReleasePlanAdmission creates a new ReleasePlanAdmission using the given parameters.
func (r *ReleaseController) CreateReleasePlanAdmission(name, namespace, environment, origin, policy, serviceAccount string, applications []string, autoRelease bool, pipelineRef *utils.PipelineRef, data *runtime.RawExtension) (*releaseApi.ReleasePlanAdmission, error) {
	releasePlanAdmission := &releaseApi.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				releaseMetadata.AutoReleaseLabel: strconv.FormatBool(autoRelease),
			},
		},
		Spec: releaseApi.ReleasePlanAdmissionSpec{
			Applications:   applications,
			Data:           data,
			Environment:    environment,
			Origin:         origin,
			PipelineRef:    pipelineRef,
			Policy:         policy,
			ServiceAccount: serviceAccount,
		},
	}

	return releasePlanAdmission, r.KubeRest().Create(context.Background(), releasePlanAdmission)
}

// GetReleasePlan returns the ReleasePlan with the given name in the given namespace.
func (r *ReleaseController) GetReleasePlan(name, namespace string) (*releaseApi.ReleasePlan, error) {
	releasePlan := &releaseApi.ReleasePlan{}

	err := r.KubeRest().Get(context.Background(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlan)

	return releasePlan, err
}

// GetReleasePlanAdmission returns the ReleasePlanAdmission with the given name in the given namespace.
func (r *ReleaseController) GetReleasePlanAdmission(name, namespace string) (*releaseApi.ReleasePlanAdmission, error) {
	releasePlanAdmission := &releaseApi.ReleasePlanAdmission{}

	err := r.KubeRest().Get(context.Background(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// DeleteReleasePlan deletes a given ReleasePlan name in given namespace.
func (r *ReleaseController) DeleteReleasePlan(name, namespace string, failOnNotFound bool) error {
	releasePlan := &releaseApi.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := r.KubeRest().Delete(context.Background(), releasePlan)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// DeleteReleasePlanAdmission deletes the ReleasePlanAdmission resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
// specify 'false', if it's likely the ReleasePlanAdmission has already been deleted (for example, because the Namespace was deleted)
func (r *ReleaseController) DeleteReleasePlanAdmission(name, namespace string, failOnNotFound bool) error {
	releasePlanAdmission := releaseApi.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := r.KubeRest().Delete(context.Background(), &releasePlanAdmission)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}
