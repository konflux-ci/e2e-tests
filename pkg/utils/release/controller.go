package release

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	releaseMetadata "github.com/redhat-appstudio/release-service/metadata"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

type StrategyConfig struct {
	Mapping Mapping `json:"mapping"`
}
type Mapping struct {
	Components []Component `json:"components"`
}
type Component struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}

func (s *SuiteController) GenerateReleaseStrategyConfig(componentName, targetRepository string) *StrategyConfig {
	return &StrategyConfig{
		Mapping{Components: []Component{{Name: componentName, Repository: targetRepository}}},
	}
}

// CreateSnapshot creates a Snapshot using the given parameters.
func (s *SuiteController) CreateSnapshot(name string, namespace string, applicationName string, snapshotComponents []appstudioApi.SnapshotComponent) (*appstudioApi.Snapshot, error) {
	snapshot := &appstudioApi.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appstudioApi.SnapshotSpec{
			Application: applicationName,
			Components:  snapshotComponents,
		},
	}
	return snapshot, s.KubeRest().Create(context.TODO(), snapshot)
}

// GetSnapshotByComponent returns the first snapshot in namespace if exist, else will return nil
func (s *SuiteController) GetSnapshotByComponent(namespace string) (*appstudioApi.Snapshot, error) {
	snapshot := &appstudioApi.SnapshotList{}
	opts := []client.ListOption{
		client.MatchingLabels{
			"test.appstudio.openshift.io/type": "component",
		},
		client.InNamespace(namespace),
	}
	err := s.KubeRest().List(context.TODO(), snapshot, opts...)

	if err == nil && len(snapshot.Items) > 0 {
		return &snapshot.Items[0], nil
	}
	return nil, err
}

// CreateRelease creates a new Release using the given parameters.
func (s *SuiteController) CreateRelease(name, namespace, snapshot, releasePlan string) (*releaseApi.Release, error) {
	release := &releaseApi.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseSpec{
			Snapshot:    snapshot,
			ReleasePlan: releasePlan,
		},
	}

	return release, s.KubeRest().Create(context.TODO(), release)
}

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
func (s *SuiteController) CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string, serviceAccount string, params []releaseApi.Params) (*releaseApi.ReleaseStrategy, error) {
	releaseStrategy := &releaseApi.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseStrategySpec{
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
		client.MatchingLabels{
			"release.appstudio.openshift.io/name":      releaseName,
			"release.appstudio.openshift.io/namespace": releaseNamespace,
		},
		client.InNamespace(namespace),
	}

	err := s.KubeRest().List(context.TODO(), pipelineRuns, opts...)

	if err == nil && len(pipelineRuns.Items) > 0 {
		return &pipelineRuns.Items[0], nil
	}

	return nil, fmt.Errorf("couldn't find PipelineRun in managed namespace '%s' for a release '%s' in '%s' namespace", namespace, releaseName, releaseNamespace)
}

// GetRelease returns the release with in the given namespace.
// It can find a Release CR based on provided name or a name of an associated Snapshot
func (s *SuiteController) GetRelease(releaseName, snapshotName, namespace string) (*releaseApi.Release, error) {
	ctx := context.Background()
	if len(releaseName) > 0 {
		release := &releaseApi.Release{}
		err := s.KubeRest().Get(ctx, types.NamespacedName{Name: releaseName, Namespace: namespace}, release)
		if err != nil {
			return nil, fmt.Errorf("failed to get Release with name '%s' in '%s' namespace", releaseName, namespace)
		}
		return release, nil
	}
	releaseList := &releaseApi.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	if err := s.KubeRest().List(context.TODO(), releaseList, opts...); err != nil {
		return nil, err
	}
	for _, r := range releaseList.Items {
		if len(snapshotName) > 0 && r.Spec.Snapshot == snapshotName {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("could not find Release CR based on associated Snapshot '%s' in '%s' namespace", snapshotName, namespace)
}

// GetRelease returns the list of Release CR in the given namespace.
func (s *SuiteController) GetReleases(namespace string) (*releaseApi.ReleaseList, error) {
	releaseList := &releaseApi.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	err := s.KubeRest().List(context.TODO(), releaseList, opts...)

	return releaseList, err
}

// GetFirstReleaseInNamespace returns the first Release from  list of releases in the given namespace.
func (s *SuiteController) GetFirstReleaseInNamespace(namespace string) (*releaseApi.Release, error) {
	releaseList := &releaseApi.ReleaseList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}

	err := s.KubeRest().List(context.TODO(), releaseList, opts...)
	if err != nil || len(releaseList.Items) < 1 {
		return nil, fmt.Errorf("could not find any Releases in namespace %s: %+v", namespace, err)
	}
	return &releaseList.Items[0], nil
}

// GetReleasePlanAdmission returns the ReleasePlanAdmission with the given name in the given namespace.
func (s *SuiteController) GetReleasePlanAdmission(name, namespace string) (*releaseApi.ReleasePlanAdmission, error) {
	releasePlanAdmission := &releaseApi.ReleasePlanAdmission{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlanAdmission)

	return releasePlanAdmission, err
}

// DeleteReleasePlanAdmission deletes the ReleasePlanAdmission resource with the given name from the given namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
// specify 'false', if it's likely the ReleasePlanAdmission has already been deleted (for example, because the Namespace was deleted)
func (s *SuiteController) DeleteReleasePlanAdmission(name, namespace string, failOnNotFound bool) error {
	releasePlanAdmission := releaseApi.ReleasePlanAdmission{
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
func (s *SuiteController) CreateReleasePlan(name, namespace, application, targetNamespace, autoReleaseLabel string) (*releaseApi.ReleasePlan, error) {
	var releasePlan *releaseApi.ReleasePlan = &releaseApi.ReleasePlan{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Name:         name,
			Namespace:    namespace,
			Labels: map[string]string{
				releaseMetadata.AutoReleaseLabel: autoReleaseLabel,
				releaseMetadata.AuthorLabel:      "username",
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

	return releasePlan, s.KubeRest().Create(context.TODO(), releasePlan)
}

// GetReleasePlan returns the ReleasePlan with the given name in the given namespace.
func (s *SuiteController) GetReleasePlan(name, namespace string) (*releaseApi.ReleasePlan, error) {
	releasePlan := &releaseApi.ReleasePlan{}

	err := s.KubeRest().Get(context.TODO(), types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, releasePlan)

	return releasePlan, err
}

// DeleteReleasePlan deletes a given ReleasePlan name in given namespace.
func (s *SuiteController) DeleteReleasePlan(name, namespace string, failOnNotFound bool) error {
	releasePlan := &releaseApi.ReleasePlan{
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
func (s *SuiteController) CreateReleasePlanAdmission(name, originNamespace, application, namespace, environment, autoRelease, releaseStrategy string) (*releaseApi.ReleasePlanAdmission, error) {
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
	return releasePlanAdmission, s.KubeRest().Create(context.TODO(), releasePlanAdmission)
}

// CreateRegistryJsonSecret creates a secret for registry repository in namespace given with key passed.
func (s *SuiteController) CreateRegistryJsonSecret(name, namespace, authKey, keyName string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{".dockerconfigjson": []byte(fmt.Sprintf("{\"auths\":{\"quay.io\":{\"username\":\"%s\",\"password\":\"%s\",\"auth\":\"dGVzdDp0ZXN0\",\"email\":\"\"}}}", keyName, authKey))},
	}
	err := s.KubeRest().Create(context.TODO(), secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// DeleteAllSnapshotsInASpecificNamespace removes all snapshots from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (s *SuiteController) DeleteAllSnapshotsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := s.KubeRest().DeleteAllOf(context.TODO(), &appstudioApi.Snapshot{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting snapshots from the namespace %s: %+v", namespace, err)
	}

	snapshotList := &appstudioApi.SnapshotList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := s.KubeRest().List(context.Background(), snapshotList, &client.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(snapshotList.Items) == 0, nil
	}, timeout)
}

// CreateComponentWithDockerSource creates a component based on container image source.
func (s *SuiteController) CreateComponentWithDockerSource(applicationName, componentName, namespace, gitSourceURL, containerImageSource, outputContainerImage, secret string) (*appstudioApi.Component, error) {
	component := &appstudioApi.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: appstudioApi.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appstudioApi.ComponentSource{
				ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
					GitSource: &appstudioApi.GitSource{
						URL:           gitSourceURL,
						DockerfileURL: containerImageSource,
					},
				},
			},
			Secret:         secret,
			ContainerImage: outputContainerImage,
			Replicas:       pointer.Int(1),
			TargetPort:     8081,
			Route:          "",
		},
	}
	err := s.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	return component, nil
}

// CreateComponentWithDockerSource creates a component based on container image source.
func (s *SuiteController) GetSbomPyxisByImageID(pyxisStageURL, imageID string,
	pyxisCertDecoded, pyxisKeyDecoded []byte) ([]byte, error) {

	url := fmt.Sprintf("%s%s", pyxisStageURL, imageID)

	// Create a TLS configuration with the key and certificate
	cert, err := tls.X509KeyPair(pyxisCertDecoded, pyxisKeyDecoded)
	if err != nil {
		fmt.Printf("\nError creating TLS certificate and key:%s", err)
		return nil, err
	}

	// Create a client with the custom TLS configuration
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		},
	}

	// Send GET request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("\nError creating GET request:%s", err)
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		fmt.Printf("\nError sending GET request:%s", err)
		return nil, err
	}

	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("\nError reading response body:%s", err)
		return nil, err
	}
	return body, nil
}
