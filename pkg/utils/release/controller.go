package release

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	kcp "github.com/redhat-appstudio/release-service/kcp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
func (s *SuiteController) CreateApplicationSnapshot(name, namespace, applicationName string, snapshotComponents []gitopsv1alpha1.ApplicationSnapshotComponent) (*gitopsv1alpha1.ApplicationSnapshot, error) {
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
func (s *SuiteController) CreateRelease(name, devNamespace, snapshot, sourceReleasePlan string) (*appstudiov1alpha1.Release, error) {
	release := &appstudiov1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: devNamespace,
		},
		Spec: appstudiov1alpha1.ReleaseSpec{
			ApplicationSnapshot: snapshot,
			ReleasePlan:         sourceReleasePlan,
		},
	}

	return release, s.KubeRest().Create(context.TODO(), release)
}

// GetRelease returns the release with the given name in the given namespace.
func (s *SuiteController) GetRelease(releaseName, releaseNamespace string) (*appstudiov1alpha1.Release, error) {
	release := &appstudiov1alpha1.Release{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releaseName,
		Namespace: releaseNamespace,
	}, release)

	return release, err
}

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
func (s *SuiteController) CreateReleaseStrategy(name, managedNamespace, pipelineName, bundle, policy, service_account string) (*appstudiov1alpha1.ReleaseStrategy, error) {
	releaseStrategy := &appstudiov1alpha1.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: managedNamespace,
		},
		Spec: appstudiov1alpha1.ReleaseStrategySpec{
			Pipeline: pipelineName,
			Bundle:   bundle,
			Policy:   policy,
			Params: []appstudiov1alpha1.Params{
				{Name: "extraConfigGitUrl", Value: "https://github.com/scoheb/strategy-configs.git", Values: []string{}},
				{Name: "extraConfigPath", Value: "m6.yaml", Values: []string{}},
				{Name: "extraConfigRevision", Value: "main", Values: []string{}},
			},

			// PersistentVolumeClaim: "test-pvc",
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

// CreateReleasePlan creates a new ReleasePlan using the given parameters.
func (s *SuiteController) CreateReleasePlan(name, devNamespace, application, managedNamespace, AutoReleaseLabel string) (*appstudiov1alpha1.ReleasePlan, error) {
	var releasePlan *appstudiov1alpha1.ReleasePlan = &appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: devNamespace,
		},
		Spec: appstudiov1alpha1.ReleasePlanSpec{
			DisplayName: name,
			Application: application,
			Target: kcp.NamespaceReference{
				Namespace: managedNamespace,
			},
		},
	}

	if AutoReleaseLabel != "" { // AutoReleaseLabel in ReleasePlan is not missing
		releasePlan.ObjectMeta.Labels[appstudiov1alpha1.AutoReleaseLabel] = AutoReleaseLabel
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

// DeleteReleasePlan deletes the releasePlan resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
//  specify 'false', if it's likely the ReleasePlan has already been deleted (for example, because the Namespace was deleted).
func (s *SuiteController) DeleteReleasePlan(releasePlanName, devNamespace string, reportErrorOnNotFound bool) error {
	releasePlan := appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releasePlanName,
			Namespace: devNamespace,
		},
	}
	err := s.KubeRest().Delete(context.TODO(), &releasePlan)
	if err != nil && !reportErrorOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}

// CreateReleasePlanAdmission creates a new ReleasePlanAdmission using the given parameters.
func (s *SuiteController) CreateReleasePlanAdmission(name, devNamespace, application, managedNamespace, AutoReleaseLabel, releaseStrategy string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {

	var releasePlanAdmission *appstudiov1alpha1.ReleasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: managedNamespace,
		},
		Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
			DisplayName: name,
			Application: application,
			Origin: kcp.NamespaceReference{
				Namespace: devNamespace,
			},
			Environment:     "test-environment",
			ReleaseStrategy: releaseStrategy,
		},
	}

	if AutoReleaseLabel != "" { // AutoReleaseLabel in ReleasePlan is not missing
		releasePlanAdmission.ObjectMeta.Labels[appstudiov1alpha1.AutoReleaseLabel] = AutoReleaseLabel
	}
	return releasePlanAdmission, s.KubeRest().Create(context.TODO(), releasePlanAdmission)
}

// GetReleasePlanAdmission returns the releasePlan with the given name in the given namespace.
func (s *SuiteController) GetReleasePlanAdmission(releasePlanAdmissionName, managedNamespace string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	releasePlanAdmission := &appstudiov1alpha1.ReleasePlanAdmission{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releasePlanAdmissionName,
		Namespace: managedNamespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// DeleteReleasePlanAdmission deletes the releasePlanAdmission resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
//  specify 'false', if it's likely the ReleasePlan has already been deleted (for example, because the Namespace was deleted)
func (s *SuiteController) DeleteReleasePlanAdmission(releasePlanAdmissionName, managedNamespace string, reportErrorOnNotFound bool) error {
	releasePlanAdmission := appstudiov1alpha1.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releasePlanAdmissionName,
			Namespace: managedNamespace,
		},
	}
	err := s.KubeRest().Delete(context.TODO(), &releasePlanAdmission)
	if err != nil && !reportErrorOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}
