package release

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
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
func (s *SuiteController) CreateApplicationSnapshot(name string, namespace string, applicationName string, snapshotComponents []gitopsv1alpha1.ApplicationSnapshotComponent) (*gitopsv1alpha1.ApplicationSnapshot, error) {
	applicationSnapshot := &gitopsv1alpha1.ApplicationSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gitopsv1alpha1.ApplicationSnapshotSpec{
			Application: applicationName,
			Components:  snapshotComponents,
		},
	}

	return applicationSnapshot, s.KubeRest().Create(context.TODO(), applicationSnapshot)
}

// CreateRelease creates a new Release using the given parameters.
func (s *SuiteController) CreateRelease(name, source_namespace, snapshot, sourceReleasePlan string) (*appstudiov1alpha1.Release, error) {
	release := &appstudiov1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: source_namespace,
		},
		Spec: v1alpha1.ReleaseSpec{
			ApplicationSnapshot: snapshot,
			ReleasePlan:         sourceReleasePlan,
		},
	}

	return release, s.KubeRest().Create(context.TODO(), release)
}

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
func (s *SuiteController) CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string, service_account string, params []appstudiov1alpha1.Params) (*appstudiov1alpha1.ReleaseStrategy, error) {
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
			ServiceAccount: service_account,
		},
	}

	return releaseStrategy, s.KubeRest().Create(context.TODO(), releaseStrategy)
}

// GetPipelineRunInNamespace returns the Release PipelineRun referencing the given release.
func (s *SuiteController) GetPipelineRunInNamespace(namespace, releaseName, releaseNamespace string) (*v1beta1.PipelineRun, error) {
	pipelineRuns := &v1beta1.PipelineRunList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"release.appstudio.openshift.io/name":      releaseName,
			"release.appstudio.openshift.io/workspace": releaseNamespace,
		},
		client.InNamespace(namespace),
	}
	err := s.KubeRest().List(context.TODO(), pipelineRuns, opts...)

	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, err
}

// GetRelease returns the release with the given name in the given namespace.
func (s *SuiteController) GetRelease(releaseName, releaseNamespace string) (*v1alpha1.Release, error) {
	release := &v1alpha1.Release{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releaseName,
		Namespace: releaseNamespace,
	}, release)

	return release, err
}

// GetReleasePlan returns the releasePlan with the given name in the given namespace.
func (s *SuiteController) GetReleasePlanAdmission(releasePlanAdmissionName, devNamespace string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	releasePlanAdmission := &appstudiov1alpha1.ReleasePlanAdmission{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releasePlanAdmissionName,
		Namespace: devNamespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// DeleteReleasePlan deletes the releasePlan resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
//  specify 'false', if it's likely the ReleasePlan has already been deleted (for example, because the Namespace was deleted)
func (s *SuiteController) DeleteReleasePlanAdmission(releasePlanAdmissionName, devNamespace string, reportErrorOnNotFound bool) error {
	releasePlanAdmission := appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releasePlanAdmissionName,
			Namespace: devNamespace,
		},
	}
	err := s.KubeRest().Delete(context.TODO(), &releasePlanAdmission)
	if err != nil && !reportErrorOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// CreateReleasePlan creates a new ReleasePlan using the given parameters.
func (s *SuiteController) CreateReleasePlan(name, devNamespace, application, managedNamespace, AutoReleaseLabel string) (*appstudiov1alpha1.ReleasePlan, error) {
	var releasePlan *appstudiov1alpha1.ReleasePlan

	if AutoReleaseLabel != "" { // AutoReleaseLabel in ReleasePlan is not missing
		releasePlan = &appstudiov1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: name,
				Name:         name,
				Namespace:    devNamespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: AutoReleaseLabel,
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanSpec{
				DisplayName: name,
				Application: application,
				Target: kcp.NamespaceReference{
					Namespace: managedNamespace,
				},
			},
		}
	} else { // AutoReleaseLabel in ReleasePlanAdmission is missing
		releasePlan = &appstudiov1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: name,
				Name:         name,
				Namespace:    devNamespace,
				Labels:       map[string]string{},
			},
			Spec: appstudiov1alpha1.ReleasePlanSpec{
				DisplayName: name,
				Application: application,
				Target: kcp.NamespaceReference{
					Namespace: managedNamespace,
				},
			},
		}
	}
	return releasePlan, s.KubeRest().Create(context.TODO(), releasePlan)
}

// GetReleasePlan returns the releasePlan with the given name in the given namespace.
func (s *SuiteController) GetReleasePlan(releasePlanName, devNamespace string) (*appstudiov1alpha1.ReleasePlan, error) {
	releasePlan := &appstudiov1alpha1.ReleasePlan{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releasePlanName,
		Namespace: devNamespace,
	}, releasePlan)

	return releasePlan, err
}

//  specify 'false', if it's likely the ReleasePlan has already been deleted (for example, because the Namespace was deleted)
func (s *SuiteController) DeleteReleasePlan(releasePlanName, devNamespace string, reportErrorOnNotFound bool) error {
	releasePlan := &appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releasePlanName,
			Namespace: devNamespace,
		},
	}
	err := s.KubeRest().Delete(context.TODO(), releasePlan)
	if err != nil && !reportErrorOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// CreateReleasePlanAdmission creates a new ReleasePlanAdmission using the given parameters.
func (s *SuiteController) CreateReleasePlanAdmission(name, devNamespace, application, managedNamespace, environment, AutoReleaseLabel string, releaseStrategy string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	var releasePlanAdmission *appstudiov1alpha1.ReleasePlanAdmission

	if AutoReleaseLabel != "" { // AutoReleaseLabel in ReleasePlanAdmission is not missing
		releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: managedNamespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: AutoReleaseLabel,
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
				DisplayName: name,
				Application: application,
				Origin: kcp.NamespaceReference{
					Namespace: devNamespace,
				},
				Environment:     environment,
				ReleaseStrategy: releaseStrategy,
			},
		}
	} else { // AutoReleaseLabel in ReleasePlanAdmission is missing
		releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: managedNamespace,
				Labels:    map[string]string{},
			},
			Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
				DisplayName: name,
				Application: application,
				Origin: kcp.NamespaceReference{
					Namespace: devNamespace,
				},
				Environment:     environment,
				ReleaseStrategy: releaseStrategy,
			},
		}
	}
	return releasePlanAdmission, s.KubeRest().Create(context.TODO(), releasePlanAdmission)
}

// CreateRegistryJsonSecret creates a secret for registry repositry in namespace given with key passed
func (s *SuiteController) CreateRegistryJsonSecret(name string, namespace string, authKey string, keytName string) (*corev1.Secret, error) {
	klog.Info("Key is : ", authKey)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": []byte(authKey)}, //[]byte(fmt.Sprintf("{\"auths\":{\"quay.io\":{\"username\":\"%s\",\"password\":\"%s\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}", keytName, authKey))},
	}

	err := s.KubeRest().Create(context.TODO(), secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
