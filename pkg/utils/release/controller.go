package release

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	gitopsv1alpha1 "github.com/redhat-appstudio/managed-gitops/appstudio-shared/apis/appstudio.redhat.com/v1alpha1"
	appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	kcp "github.com/redhat-appstudio/release-service/kcp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// appstudiov1alpha1 "github.com/redhat-appstudio/release-service/api/v1alpha1"
	// "github.com/redhat-appstudio/release-service/kcp"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/apimachinery/pkg/types"
	// clientsetscheme "k8s.io/client-go/kubernetes/scheme"
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

// release_controller_test.go line 68
//
// release = &appstudiov1alpha1.Release{
// 	TypeMeta: metav1.TypeMeta{
// 		APIVersion: testApiVersion,
// 		Kind:       "Release",
// 	},
// 	ObjectMeta: metav1.ObjectMeta{
// 		GenerateName: "test-release-",
// 		Namespace:    testNamespace,
// 	},
// 	Spec: appstudiov1alpha1.ReleaseSpec{
// 		ApplicationSnapshot: "test-snapshot",
// 		ReleasePlan:         releasePlan.GetName(),
// 	},
// }

// CreateRelease creates a new Release using the given parameters.
func (s *SuiteController) CreateRelease(name, source_namespace, snapshot, sourceReleasePlan string) (*appstudiov1alpha1.Release, error) {
	release := &appstudiov1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: source_namespace,
		},
		Spec: appstudiov1alpha1.ReleaseSpec{
			ApplicationSnapshot: snapshot,
			ReleasePlan:         sourceReleasePlan,
		},
	}

	return release, s.KubeRest().Create(context.TODO(), release)
}

// CreateRelease creates a new Release using the given parameters.
// func (s *SuiteController) CreateRelease(name, namespace, snapshot, sourceReleaseLink string) (*v1alpha1.Release, error) {
// 	release := &v1alpha1.Release{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 		Spec: v1alpha1.ReleaseSpec{
// 			ApplicationSnapshot: snapshot,
// 			ReleaseLink:         sourceReleaseLink,
// 		},
// 	}

// 	return release, s.KubeRest().Create(context.TODO(), release)
// }

// release_controller_test.go line 51
//
// releasePlan = &appstudiov1alpha1.ReleasePlan{
// 	ObjectMeta: metav1.ObjectMeta{
// 		GenerateName: "test-releaseplan-",
// 		Namespace:    testNamespace,
// 		Labels: map[string]string{
// 			appstudiov1alpha1.AutoReleaseLabel: "true",
// 		},
// 	},
// 	Spec: appstudiov1alpha1.ReleasePlanSpec{
// 		Application: "test-app",
// 		Target: kcp.NamespaceReference{
// 			Namespace: testNamespace,
// 		},
// 	},
// }

// CreateReleasePlan creates a new ReleasePlan using the given parameters.
func (s *SuiteController) CreateReleasePlan(name, source_namespace, application, target_namespace, AutoReleaseLabel string) (*appstudiov1alpha1.ReleasePlan, error) {
	var releasePlan *appstudiov1alpha1.ReleasePlan

	if AutoReleaseLabel != "" { // AutoReleaseLabel in ReleasePlan is not missing
		releasePlan = &appstudiov1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: name,
				Name:         name,
				Namespace:    source_namespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: AutoReleaseLabel,
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanSpec{
				DisplayName: name,
				Application: application,
				Target: kcp.NamespaceReference{
					Namespace: target_namespace,
				},
			},
		}
	} else { // AutoReleaseLabel in ReleasePlanAdmission is missing
		releasePlan = &appstudiov1alpha1.ReleasePlan{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: name,
				Name:         name,
				Namespace:    source_namespace,
				Labels:       map[string]string{},
			},
			Spec: appstudiov1alpha1.ReleasePlanSpec{
				DisplayName: name,
				Application: application,
				Target: kcp.NamespaceReference{
					Namespace: target_namespace,
				},
			},
		}
	}
	return releasePlan, s.KubeRest().Create(context.TODO(), releasePlan)
}

// release_adapter_test.go line 86
//
// releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
// 	ObjectMeta: metav1.ObjectMeta{
// 		GenerateName: "test-releaseplanadmission-",
// 		Namespace:    testNamespace,
// 		Labels: map[string]string{
// 			appstudiov1alpha1.AutoReleaseLabel: "true",
// 		},
// 	},
// 	Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
// 		Application: "test-app",
// 		Origin: kcp.NamespaceReference{
// 			Namespace: testNamespace,
// 		},
// 		Environment:     "test-environment",
// 		ReleaseStrategy: releaseStrategy.GetName(),
// 	},
// }

// releaseplanadmission_webhook_test.go line 33

// releasePlanAdmission = &ReleasePlanAdmission{
// 	TypeMeta: metav1.TypeMeta{
// 		APIVersion: "appstudio.redhat.com/v1alpha1",
// 		Kind:       "ReleasePlanAdmission",
// 	},
// 	ObjectMeta: metav1.ObjectMeta{
// 		Name:      "releaseplanadmission",
// 		Namespace: "default",
// 	},
// 	Spec: ReleasePlanAdmissionSpec{
// 		DisplayName: "Test release plan",
// 		Application: "application",
// 		Origin: kcp.NamespaceReference{
// 			Namespace: "default",
// 		},
// 		Environment:     "environment",
// 		ReleaseStrategy: "strategy",
// 	},
// }

// CreateReleasePlanAdmission creates a new ReleasePlanAdmission using the given parameters.
func (s *SuiteController) CreateReleasePlanAdmission(name, source_namespace, application, target_namespace, AutoReleaseLabel string, releaseStrategy string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	var releasePlanAdmission *appstudiov1alpha1.ReleasePlanAdmission

	if AutoReleaseLabel != "" { // AutoReleaseLabel in ReleasePlanAdmission is not missing
		releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: target_namespace,
				Labels: map[string]string{
					appstudiov1alpha1.AutoReleaseLabel: AutoReleaseLabel,
				},
			},
			Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
				DisplayName: name,
				Application: application,
				Origin: kcp.NamespaceReference{
					Namespace: source_namespace,
				},
				Environment:     "test-environment",
				ReleaseStrategy: releaseStrategy,
			},
		}
	} else { // AutoReleaseLabel in ReleasePlanAdmission is missing
		releasePlanAdmission = &appstudiov1alpha1.ReleasePlanAdmission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: target_namespace,
				Labels:    map[string]string{},
			},
			Spec: appstudiov1alpha1.ReleasePlanAdmissionSpec{
				DisplayName: name,
				Application: application,
				Origin: kcp.NamespaceReference{
					Namespace: source_namespace,
				},
				Environment:     "test-environment",
				ReleaseStrategy: releaseStrategy,
			},
		}
	}
	return releasePlanAdmission, s.KubeRest().Create(context.TODO(), releasePlanAdmission)
}

// CreateReleaseLink creates a new ReleaseLink using the given parameters.
// func (s *SuiteController) CreateReleaseLink(name, namespace, application, targetNamespace, releaseStrategy string) (*v1alpha1.ReleaseLink, error) {
// 	releaseLink := &v1alpha1.ReleaseLink{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 		Spec: v1alpha1.ReleaseLinkSpec{
// 			DisplayName:     name,
// 			Application:     application,
// 			Target:          targetNamespace,
// 			ReleaseStrategy: releaseStrategy,
// 		},
// 	}

// 	return releaseLink, s.KubeRest().Create(context.TODO(), releaseLink)
// }

// release_adapter_test.go line 57
//
// releaseStrategy = &appstudiov1alpha1.ReleaseStrategy{
// 	ObjectMeta: metav1.ObjectMeta{
// 		GenerateName: "test-releasestrategy-",
// 		Namespace:    testNamespace,
// 	},
// 	Spec: appstudiov1alpha1.ReleaseStrategySpec{
// 		Pipeline:              "release-pipeline",
// 		Bundle:                "test-bundle",
// 		Policy:                "test-policy",
// 		PersistentVolumeClaim: "test-pvc",
// 		ServiceAccount:        "test-account",
// 	},
// }

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
func (s *SuiteController) CreateReleaseStrategy(name, target_namespace, pipelineName, bundle string, policy string) (*appstudiov1alpha1.ReleaseStrategy, error) {
	releaseStrategy := &appstudiov1alpha1.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: target_namespace,
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

			// 		PersistentVolumeClaim: "test-pvc",
			ServiceAccount: "m6-service-account",
		},
	}

	return releaseStrategy, s.KubeRest().Create(context.TODO(), releaseStrategy)
}

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
// func (s *SuiteController) CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string) (*v1alpha1.ReleaseStrategy, error) {
// 	releaseStrategy := &v1alpha1.ReleaseStrategy{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 		Spec: v1alpha1.ReleaseStrategySpec{
// 			Pipeline: pipelineName,
// 			Bundle:   bundle,
// 			Policy:   policy,
// 		},
// 	}

// 	return releaseStrategy, s.KubeRest().Create(context.TODO(), releaseStrategy)
// }

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
func (s *SuiteController) GetRelease(releaseName, releaseNamespace string) (*appstudiov1alpha1.Release, error) {
	release := &appstudiov1alpha1.Release{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releaseName,
		Namespace: releaseNamespace,
	}, release)

	return release, err
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

// GetRelease returns the release with the given name in the given namespace.
func (s *SuiteController) GetReleasePlanAdmission(releasePlanAdmissionName, releaseNamespace string) (*appstudiov1alpha1.ReleasePlanAdmission, error) {
	releasePlanAdmission := &appstudiov1alpha1.ReleasePlanAdmission{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      releasePlanAdmissionName,
		Namespace: releaseNamespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// Get ReleaseLink object from a given namespace
// func (s *SuiteController) GetReleaseLink(name string, namespace string) (*v1alpha1.ReleaseLink, error) {
// 	namespacedName := types.NamespacedName{
// 		Name:      name,
// 		Namespace: namespace,
// 	}

// 	releaseLink := &v1alpha1.ReleaseLink{}
// 	err := s.KubeRest().Get(context.TODO(), namespacedName, releaseLink)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return releaseLink, nil
// }
