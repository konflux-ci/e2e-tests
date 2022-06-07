package release

import (
	"context"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	release "github.com/redhat-appstudio/release-service/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

// Creats a realse link resource
func (s *SuiteController) CreateReleaseLink(name string, namespace string, displayName string, application string, target string, releaseStrategy string) (*release.ReleaseLink, error) {
	releaseLinkObj := release.ReleaseLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: release.ReleaseLinkSpec{
			DisplayName:     displayName,
			Application:     application,
			Target:          target,
			ReleaseStrategy: releaseStrategy,
		},
	}

	if releaseStrategy != "" {
		releaseLinkObj.Spec.ReleaseStrategy = releaseStrategy
	}

	err := s.KubeRest().Create(context.TODO(), &releaseLinkObj)
	return &releaseLinkObj, err
}

//  Creates a resource of release startegy
func (s *SuiteController) CreateReleaseStrategy(name string, namespace string, pipelineName string, bundle string) (*release.ReleaseStrategy, error) {
	releaseStrObj := release.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: release.ReleaseStrategySpec{
			Pipeline: pipelineName,
			Bundle:   bundle,
		},
	}
	err := s.KubeRest().Create(context.TODO(), &releaseStrObj)
	return &releaseStrObj, err
}

// Get Release resource in a given state [This is for the demo4 and will be changed in demo 5 and ahead]
func (s *SuiteController) GetRelease(namespace string) (*release.Release, error) {

	releases := &release.ReleaseList{}

	err := s.KubeRest().List(context.TODO(), releases, rclient.InNamespace(namespace))
	if len(releases.Items) == 1 {
		return &releases.Items[0], nil
	}
	return nil, err
}

// Creats a realse  resource (M5)
func (s *SuiteController) CreateRelease(name string, namespace string, applicationSnapshot string, releaseLink string) (*release.Release, error) {
	releaseObj := release.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: release.ReleaseSpec{
			ApplicationSnapshot: applicationSnapshot,
			ReleaseLink:         releaseLink,
		},
	}

	if applicationSnapshot != "" {
		releaseObj.Spec.ApplicationSnapshot = applicationSnapshot
	}

	err := s.KubeRest().Create(context.TODO(), &releaseObj)
	return &releaseObj, err
}

// Create a ApplicationSnapshot in Release
func (s *SuiteController) CreateApplicationSnapshot(name string, namespace string, image_1 string, image_2 string, image_3 string, app_name string) (*release.ApplicationSnapshot, error) {

	applicationSnapshotObj := &release.ApplicationSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: release.ApplicationSnapshotSpec{
			Application: app_name,
			Images: []release.Image{
				{
					Component: "component1",
					PullSpec:  image_1,
				},
				{
					Component: "component2",
					PullSpec:  image_2,
				},
				{
					Component: "component3",
					PullSpec:  image_3,
				},
			},
		},
	}
	err := s.KubeRest().Create(context.TODO(), applicationSnapshotObj)
	return applicationSnapshotObj, err
}
