package release

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	applicationapiv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	"github.com/redhat-appstudio/release-service/api/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	kcp "github.com/redhat-appstudio/release-service/kcp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

// CreateApplicationSnapshot creates a new ApplicationSnapshot using the given parameters.
func (s *SuiteController) CreateApplicationSnapshot(name string, namespace string, applicationName string, snapshotComponents []applicationapiv1alpha1.ApplicationSnapshotComponent) (*applicationapiv1alpha1.ApplicationSnapshot, error) {
	applicationSnapshot := &applicationapiv1alpha1.ApplicationSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: applicationapiv1alpha1.ApplicationSnapshotSpec{
			Application: applicationName,
			Components:  snapshotComponents,
		},
	}

	return applicationSnapshot, s.KubeRest().Create(context.TODO(), applicationSnapshot)
}

// CreateRelease creates a new Release using the given parameters.
func (s *SuiteController) CreateRelease(name, namespace, snapshot, releasePlan string) (*appstudiov1alpha1.Release, error) {
	release := &appstudiov1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ReleaseSpec{
			ApplicationSnapshot: snapshot,
			ReleasePlan:         releasePlan,
		},
	}

	return release, s.KubeRest().Create(context.TODO(), release)
}

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
func (s *SuiteController) CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string, serviceAccount string, params []appstudiov1alpha1.Params) (*appstudiov1alpha1.ReleaseStrategy, error) {
	releaseStrategy := &appstudiov1alpha1.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ReleaseStrategySpec{
			Pipeline:       pipelineName,
			Bundle:         bundle,
			Policy:         policy,
			Params:         params,
			ServiceAccount: serviceAccount,
		},
	}

	return releaseStrategy, s.KubeRest().Create(context.TODO(), releaseStrategy)
}

// GetPipelineRunInNamespace returns the Release PipelineRun referencing the given release.
func (s *SuiteController) GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}

	opts := []client.ListOption{
		client.Limit(1),
		client.InNamespace(namespace),
		client.MatchingLabels{
			"release.appstudio.openshift.io/name":      releaseName,
			"release.appstudio.openshift.io/workspace": releaseNamespace,
		},
	}
	err := s.KubeRest().List(context.TODO(), pipelineRuns, opts...)

	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, err
}

// GetRelease returns the release with the given name in the given namespace.
func (s *SuiteController) GetRelease(name, namespace string) (*v1alpha1.Release, error) {
	release := &v1alpha1.Release{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, release)

	return release, err
}

// GetReleasePlanAdmission returns the ReleasePlanAdmission with the given name in the given namespace.
func (s *SuiteController) GetReleasePlanAdmission(name, namespace string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	releasePlanAdmission := &appstudiov1alpha1.ReleasePlanAdmission{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// DeleteReleasePlanAdmission deletes the ReleasePlanAdmission resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
//  specify 'false', if it's likely the ReleasePlanAdmission has already been deleted (for example, because the Namespace was deleted)
func (s *SuiteController) DeleteReleasePlanAdmission(name, namespace string, failOnNotFound bool) error {
	releasePlanAdmission := appstudiov1alpha1.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := s.KubeRest().Delete(context.TODO(), &releasePlanAdmission)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// CreateReleasePlan creates a new ReleasePlan using the given parameters.
func (s *SuiteController) CreateReleasePlan(name, namespace, application, targetNamespace, autoReleaseLabel string) (*appstudiov1alpha1.ReleasePlan, error) {
	var releasePlan *appstudiov1alpha1.ReleasePlan = &appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Name:         name,
			Namespace:    namespace,
			Labels: map[string]string{
				appstudiov1alpha1.AutoReleaseLabel: autoReleaseLabel,
			},
		},
		Spec: appstudiov1alpha1.ReleasePlanSpec{
			DisplayName: name,
			Application: application,
			Target: kcp.NamespaceReference{
				Namespace: targetNamespace,
			},
		},
	}
	if autoReleaseLabel == "" || autoReleaseLabel == "true" {
		releasePlan.ObjectMeta.Labels[appstudiov1alpha1.AutoReleaseLabel] = "true"
	} else {
		releasePlan.ObjectMeta.Labels[appstudiov1alpha1.AutoReleaseLabel] = "false"
	}

	return releasePlan, s.KubeRest().Create(context.TODO(), releasePlan)
}

// GetReleasePlan returns the releasePlan with the given name in the given namespace.
func (s *SuiteController) GetReleasePlan(name, namespace string) (*appstudiov1alpha1.ReleasePlan, error) {
	releasePlan := &appstudiov1alpha1.ReleasePlan{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlan)

	return releasePlan, err
}

//  DeletetReleasePlan deletes a given releaseplan name in given namespace.
func (s *SuiteController) DeleteReleasePlan(name, namespace string, failOnNotFound bool) error {
	releasePlan := &appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := s.KubeRest().Delete(context.TODO(), releasePlan)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// CreateReleasePlanAdmission creates a new ReleasePlanAdmission using the given parameters.
func (s *SuiteController) CreateReleasePlanAdmission(name, originNamespace, application, namespace, environment, autoRelease, releaseStrategy string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	var releasePlanAdmission *appstudiov1alpha1.ReleasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
			DisplayName: name,
			Application: application,
			Origin: kcp.NamespaceReference{
				Namespace: originNamespace,
			},
			Environment:     environment,
			ReleaseStrategy: releaseStrategy,
		},
	}
	if autoRelease != "" {
		releasePlanAdmission.ObjectMeta.Labels[appstudiov1alpha1.AutoReleaseLabel] = autoRelease
	}
	return releasePlanAdmission, s.KubeRest().Create(context.TODO(), releasePlanAdmission)
}

// CreateRegistryJsonSecret creates a secret for registry repository in namespace given with key passed.
func (s *SuiteController) CreateRegistryJsonSecret(name, namespace, authKey, keyName string) (*corev1.Secret, error) {
	klog.Info("Key is : ", authKey)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": []byte(authKey)},
	}

	err := s.KubeRest().Create(context.TODO(), secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
