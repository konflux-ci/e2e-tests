package has

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	appservice "github.com/konflux-ci/application-api/api/v1alpha1"
	"github.com/konflux-ci/e2e-tests/pkg/clients/tekton"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/logs"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	imagecontroller "github.com/konflux-ci/image-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	"knative.dev/pkg/apis"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	RequiredLabelNotFound = "cannot retrigger PipelineRun - required label %q not found"
)

// GetComponent return a component object from kubernetes cluster
func (h *HasController) GetComponent(name string, namespace string) (*appservice.Component, error) {
	component := &appservice.Component{}
	if err := h.KubeRest().Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, component); err != nil {
		return nil, err
	}

	return component, nil
}

// GetComponentByApplicationName returns a component from kubernetes cluster given a application name.
func (h *HasController) GetComponentByApplicationName(applicationName string, namespace string) (*appservice.Component, error) {
	components := &appservice.ComponentList{}
	opts := []rclient.ListOption{
		rclient.InNamespace(namespace),
	}
	err := h.KubeRest().List(context.Background(), components, opts...)
	if err != nil {
		return nil, err
	}
	for _, component := range components.Items {
		if component.Spec.Application == applicationName {
			return &component, nil
		}
	}

	return &appservice.Component{}, fmt.Errorf("no component found %s", utils.GetAdditionalInfo(applicationName, namespace))
}

// GetComponentPipeline returns first pipeline run for a given component labels
func (h *HasController) GetComponentPipelineRun(componentName string, applicationName string, namespace, sha string) (*pipeline.PipelineRun, error) {
	return h.GetComponentPipelineRunWithType(componentName, applicationName, namespace, "", sha)
}

// GetComponentPipelineRunWithType returns first pipeline run for a given component labels with pipeline type within label "pipelines.appstudio.openshift.io/type" ("build", "test")
func (h *HasController) GetComponentPipelineRunWithType(componentName string, applicationName string, namespace, pipelineType string, sha string) (*pipeline.PipelineRun, error) {
	prs, err := h.GetComponentPipelineRunsWithType(componentName, applicationName, namespace, "", sha)
	if err != nil {
		return nil, err
	} else {
		prsVal := *prs
		return &prsVal[0], nil
	}
}

// GetComponentPipelineRunsWithType returns all pipeline runs for a given component labels with pipeline type within label "pipelines.appstudio.openshift.io/type" ("build", "test")
func (h *HasController) GetComponentPipelineRunsWithType(componentName string, applicationName string, namespace, pipelineType string, sha string) (*[]pipeline.PipelineRun, error) {
	pipelineRunLabels := map[string]string{"appstudio.openshift.io/component": componentName, "appstudio.openshift.io/application": applicationName}
	if pipelineType != "" {
		pipelineRunLabels["pipelines.appstudio.openshift.io/type"] = pipelineType
	}

	if sha != "" {
		pipelineRunLabels["pipelinesascode.tekton.dev/sha"] = sha
	}

	list := &pipeline.PipelineRunList{}
	err := h.KubeRest().List(context.Background(), list, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels), Namespace: namespace})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace: %v", namespace, err)
	}

	// If we hit any other error, while fetching pipelineRun list
	if err != nil {
		return nil, fmt.Errorf("error while trying to get pipelinerun list in %s namespace: %v", namespace, err)
	}

	if len(list.Items) > 0 {
		return &list.Items, nil
	}

	return nil, fmt.Errorf("no pipelinerun found for component %s", componentName)
}

// GetAllPipelineRunsForApplication returns the pipelineruns for a given application in the namespace
func (h *HasController) GetAllPipelineRunsForApplication(applicationName, namespace string) (*pipeline.PipelineRunList, error) {
	pipelineRunLabels := map[string]string{"appstudio.openshift.io/application": applicationName}

	list := &pipeline.PipelineRunList{}
	err := h.KubeRest().List(context.Background(), list, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels), Namespace: namespace})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace: %v", namespace, err)
	}

	if len(list.Items) > 0 {
		return list, nil
	}

	return nil, fmt.Errorf("no pipelinerun found for application %s", applicationName)
}

// GetAllGroupSnapshotsForApplication returns the groupSnapshots for a given application in the namespace
func (h *HasController) GetAllGroupSnapshotsForApplication(applicationName, namespace string) (*appservice.SnapshotList, error) {
	snapshotLabels := map[string]string{"appstudio.openshift.io/application": applicationName, "test.appstudio.openshift.io/type": "group"}

	list := &appservice.SnapshotList{}
	err := h.KubeRest().List(context.Background(), list, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(snapshotLabels), Namespace: namespace})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing snapshots in %s namespace: %v", namespace, err)
	}

	if len(list.Items) > 0 {
		return list, nil
	}

	return nil, fmt.Errorf("no snapshot found for application %s", applicationName)
}

// Set of options to retrigger pipelineRuns in CI to fight against flakynes
type RetryOptions struct {
	// Indicate how many times a pipelineRun should be retriggered in case of flakines
	Retries int

	// If is set to true the PipelineRun will be retriggered always in case if pipelinerun fail for any reason. Time to time in RHTAP CI
	// we see that there are a lot of components which fail with QPS in build-container which cannot be controlled.
	// By default is false will retrigger a pipelineRun only when meet CouldntGetTask or TaskRunImagePullFailed conditions
	Always bool
}

// WaitForComponentPipelineToBeFinished waits for a given component PipelineRun to be finished
// In case of hitting issues like `TaskRunImagePullFailed` or `CouldntGetTask` it will re-trigger the PLR.
// Due to re-trigger mechanism this function can invalidate the related PLR object which might be used later in the test
// (by deleting the original PLR and creating a new one in case the PLR fails on one of the attempts).
// For that case this function gives an option to pass in a pointer to a related PLR object (`prToUpdate`) which will be updated (with a valid PLR object) before the end of this function
// and the PLR object can be then used for making assertions later in the test.
// If there's no intention for using the original PLR object later in the test, use `nil` instead of the pointer.
func (h *HasController) WaitForComponentPipelineToBeFinished(component *appservice.Component, sha string, t *tekton.TektonController, r *RetryOptions, prToUpdate *pipeline.PipelineRun) error {
	attempts := 1
	app := component.Spec.Application
	pr := &pipeline.PipelineRun{}

	for {
		err := wait.PollUntilContextTimeout(context.Background(), constants.PipelineRunPollingInterval, 30*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			pr, err = h.GetComponentPipelineRun(component.GetName(), app, component.GetNamespace(), sha)

			if err != nil {
				GinkgoWriter.Printf("PipelineRun has not been created yet for the Component %s/%s\n", component.GetNamespace(), component.GetName())
				return false, nil
			}

			GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pr.Name, pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason())

			if !pr.IsDone() {
				return false, nil
			}

			if pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
				return true, nil
			}

			var prLogs string
			if err = t.StorePipelineRun(component.GetName(), pr); err != nil {
				GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pr.GetNamespace(), pr.GetName(), err.Error())
			}
			if prLogs, err = t.GetPipelineRunLogs(component.GetName(), pr.Name, pr.Namespace); err != nil {
				GinkgoWriter.Printf("failed to get logs for PipelineRun %s:%s: %s\n", pr.GetNamespace(), pr.GetName(), err.Error())
			}
			return false, fmt.Errorf("%s", prLogs)
		})

		if err != nil {
			if pr == nil {
				return fmt.Errorf("PipelineRun cannot be created for the Component %s/%s", component.GetNamespace(), component.GetName())
			}
			GinkgoWriter.Printf("attempt %d/%d: PipelineRun %q failed: %+v", attempts, r.Retries+1, pr.GetName(), err)
			// CouldntGetTask: Retry the PipelineRun only in case we hit the known issue https://issues.redhat.com/browse/SRVKP-2749
			// TaskRunImagePullFailed: Retry in case of https://issues.redhat.com/browse/RHTAPBUGS-985 and https://github.com/tektoncd/pipeline/issues/7184
			if attempts == r.Retries+1 || (!r.Always && pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() != "CouldntGetTask" && pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() != "TaskRunImagePullFailed") {
				return err
			}
			if err = t.RemoveFinalizerFromPipelineRun(pr, constants.E2ETestFinalizerName); err != nil {
				return fmt.Errorf("failed to remove the finalizer from pipelinerun %s:%s in order to retrigger it: %+v", pr.GetNamespace(), pr.GetName(), err)
			}
			if err = h.PipelineClient().TektonV1().PipelineRuns(pr.GetNamespace()).Delete(context.Background(), pr.GetName(), metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("failed to delete PipelineRun %q from %q namespace with error: %v", pr.GetName(), pr.GetNamespace(), err)
			}
			if sha, err = h.RetriggerComponentPipelineRun(component, pr); err != nil {
				return fmt.Errorf("unable to retrigger pipelinerun for component %s:%s: %+v", component.GetNamespace(), component.GetName(), err)
			}
			attempts++
		} else {
			break
		}
	}

	// If prToUpdate variable was passed to this function, update it with the latest version of the PipelineRun object
	if prToUpdate != nil {
		pr.DeepCopyInto(prToUpdate)
	}

	return nil
}

// Universal method to create a component in the kubernetes clusters.
func (h *HasController) CreateComponent(componentSpec appservice.ComponentSpec, namespace string, outputContainerImage string, secret string, applicationName string, skipInitialChecks bool, annotations map[string]string) (*appservice.Component, error) {
	componentObject := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentSpec.ComponentName,
			Namespace: namespace,
			Annotations: map[string]string{
				"skip-initial-checks": strconv.FormatBool(skipInitialChecks),
			},
		},
		Spec: componentSpec,
	}
	componentObject.Spec.Secret = secret
	componentObject.Spec.Application = applicationName

	if len(annotations) > 0 {
		componentObject.Annotations = utils.MergeMaps(componentObject.Annotations, annotations)

	}

	if componentObject.Spec.TargetPort == 0 {
		componentObject.Spec.TargetPort = 8081
	}
	if outputContainerImage != "" {
		componentObject.Spec.ContainerImage = outputContainerImage
	} else if componentObject.Annotations["image.redhat.com/generate"] == "" {
		// Generate default public image repo since nothing is mentioned specifically
		componentObject.Annotations = utils.MergeMaps(componentObject.Annotations, constants.ImageControllerAnnotationRequestPublicRepo)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()
	if err := h.KubeRest().Create(ctx, componentObject); err != nil {
		return nil, err
	}

	if utils.WaitUntil(h.CheckImageRepositoryExists(namespace, componentSpec.ComponentName), time.Minute*5) != nil {
		return nil, fmt.Errorf("timed out when waiting for image-controller annotations to be updated on component %s in namespace %s. component: %s", componentSpec.ComponentName, namespace, utils.ToPrettyJSONString(componentObject))
	}
	return componentObject, nil
}

// CreateComponentWithDockerSource creates a component based on container image source.
func (h *HasController) CreateComponentWithDockerSource(applicationName, componentName, namespace, gitSourceURL, containerImageSource, outputContainerImage, secret string) (*appservice.Component, error) {
	component := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      componentName,
			Namespace: namespace,
		},
		Spec: appservice.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appservice.ComponentSource{
				ComponentSourceUnion: appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL:           gitSourceURL,
						DockerfileURL: containerImageSource,
					},
				},
			},
			Secret:         secret,
			ContainerImage: outputContainerImage,
			Replicas:       pointer.To[int](1),
			TargetPort:     8081,
			Route:          "",
		},
	}
	err := h.KubeRest().Create(context.Background(), component)
	if err != nil {
		return nil, err
	}
	return component, nil
}

// ScaleDeploymentReplicas scales the replicas of a given deployment
func (h *HasController) ScaleComponentReplicas(component *appservice.Component, replicas *int) (*appservice.Component, error) {
	component.Spec.Replicas = replicas

	err := h.KubeRest().Update(context.Background(), component, &rclient.UpdateOptions{})
	if err != nil {
		return &appservice.Component{}, err
	}
	return component, nil
}

// DeleteComponent delete an has component from a given name and namespace
func (h *HasController) DeleteComponent(name string, namespace string, reportErrorOnNotFound bool) error {
	// temporary logs
	start := time.Now()
	GinkgoWriter.Printf("Start to delete component '%s' at %s\n", name, start.Format(time.RFC3339))

	component := appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := h.KubeRest().Delete(context.Background(), &component); err != nil {
		if !k8sErrors.IsNotFound(err) || (k8sErrors.IsNotFound(err) && reportErrorOnNotFound) {
			return fmt.Errorf("error deleting a component: %+v", err)
		}
	}

	// RHTAPBUGS-978: temporary timeout to 15min
	err := utils.WaitUntil(h.ComponentDeleted(&component), 15*time.Minute)

	// temporary logs
	deletionTime := time.Since(start).Minutes()
	GinkgoWriter.Printf("Finish to delete component '%s' at %s. It took '%f' minutes\n", name, time.Now().Format(time.RFC3339), deletionTime)

	return err
}

// DeleteAllComponentsInASpecificNamespace removes all component CRs from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *HasController) DeleteAllComponentsInASpecificNamespace(namespace string, timeout time.Duration) error {
	// temporary logs
	start := time.Now()
	GinkgoWriter.Printf("Start to delete all components in namespace '%s' at %s\n", namespace, start.String())

	if err := h.KubeRest().DeleteAllOf(context.Background(), &appservice.Component{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting components from the namespace %s: %+v", namespace, err)
	}

	componentList := &appservice.ComponentList{}

	err := utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), componentList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(componentList.Items) == 0, nil
	}, timeout)

	// temporary logs
	deletionTime := time.Since(start).Minutes()
	GinkgoWriter.Printf("Finish to delete all components in namespace '%s' at %s. It took '%f' minutes\n", namespace, time.Now().Format(time.RFC3339), deletionTime)

	return err
}

// Waits for a component until is deleted and if not will return an error
func (h *HasController) ComponentDeleted(component *appservice.Component) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := h.GetComponent(component.Name, component.Namespace)
		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}

// Get the message from the status of a component. Usefull for debugging purposes.
func (h *HasController) GetComponentConditionStatusMessages(name, namespace string) (messages []string, err error) {
	c, err := h.GetComponent(name, namespace)
	if err != nil {
		return messages, fmt.Errorf("error getting HAS component: %v", err)
	}
	for _, condition := range c.Status.Conditions {
		messages = append(messages, condition.Message)
	}
	return
}

// Universal method to retrigger pipelineruns in kubernetes cluster
func (h *HasController) RetriggerComponentPipelineRun(component *appservice.Component, pr *pipeline.PipelineRun) (sha string, err error) {
	prLabels := pr.GetLabels()
	prAnnotations := pr.GetAnnotations()
	// In case of PipelineRun managed by PaC we are able to retrigger the pipeline only
	// by updating the related branch
	if prLabels["app.kubernetes.io/managed-by"] == "pipelinesascode.tekton.dev" {
		var ok bool
		var repoName, eventType, branchName, gitProvider string
		pacRepoNameLabelName := "pipelinesascode.tekton.dev/url-repository"
		gitProviderLabelName := "pipelinesascode.tekton.dev/git-provider"
		pacEventTypeLabelName := "pipelinesascode.tekton.dev/event-type"
		componentLabelName := "appstudio.openshift.io/component"
		targetBranchAnnotationName := "build.appstudio.redhat.com/target_branch"

		if repoName, ok = prLabels[pacRepoNameLabelName]; !ok {
			return "", fmt.Errorf(RequiredLabelNotFound, pacRepoNameLabelName)
		}
		if eventType, ok = prLabels[pacEventTypeLabelName]; !ok {
			return "", fmt.Errorf(RequiredLabelNotFound, pacEventTypeLabelName)
		}
		// since not all build PipelineRuns contains this annotation
		gitProvider = prAnnotations[gitProviderLabelName]

		// PipelineRun is triggered from a pull request, need to update the PaC PR source branch
		if eventType == "pull_request" || eventType == "Merge_Request" {
			if len(prLabels[componentLabelName]) < 1 {
				return "", fmt.Errorf(RequiredLabelNotFound, componentLabelName)
			}
			branchName = constants.PaCPullRequestBranchPrefix + prLabels[componentLabelName]
		} else {
			// No straightforward way to get a target branch from PR labels -> using annotation
			if branchName, ok = pr.GetAnnotations()[targetBranchAnnotationName]; !ok {
				return "", fmt.Errorf("cannot retrigger PipelineRun - required annotation %q not found", targetBranchAnnotationName)
			}
		}

		if gitProvider == "gitlab" {
			gitlabOrg := utils.GetEnv(constants.GITLAB_QE_ORG_ENV, constants.DefaultGitLabQEOrg)
			projectID, ok := prLabels["pipelinesascode.tekton.dev/source-project-id"]
			if !ok {
				projectID = fmt.Sprintf("%s/%s", gitlabOrg, repoName)
			}
			_, err := h.GitLab.CreateFile(projectID, util.GenerateRandomString(5), "test", branchName)
			if err != nil {
				return "", fmt.Errorf("failed to retrigger PipelineRun %s in %s namespace: %+v", pr.GetName(), pr.GetNamespace(), err)
			}
		} else {
			file, err := h.Github.CreateFile(repoName, util.GenerateRandomString(5), "test", branchName)
			if err != nil {
				return "", fmt.Errorf("failed to retrigger PipelineRun %s in %s namespace: %+v", pr.GetName(), pr.GetNamespace(), err)
			}
			sha = file.GetSHA()
		}

		// To retrigger simple build PipelineRun we just need to update the initial build annotation
		// in Component CR
	} else {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			component, err := h.GetComponent(component.GetName(), component.GetNamespace())
			if err != nil {
				return fmt.Errorf("failed to get component for PipelineRun %q in %q namespace: %+v", pr.GetName(), pr.GetNamespace(), err)
			}
			component.Annotations = utils.MergeMaps(component.Annotations, constants.ComponentTriggerSimpleBuildAnnotation)
			if err = h.KubeRest().Update(context.Background(), component); err != nil {
				return fmt.Errorf("failed to update Component %q in %q namespace", component.GetName(), component.GetNamespace())
			}
			return err
		})

		if err != nil {
			return "", err
		}
	}
	watch, err := h.PipelineClient().TektonV1().PipelineRuns(component.GetNamespace()).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("error when initiating watch for new PipelineRun after retriggering it for component %s:%s", component.GetNamespace(), component.GetName())
	}
	newPRFound := false
	for {
		select {
		case <-time.After(5 * time.Minute):
			return "", fmt.Errorf("timed out waiting for new PipelineRun to appear after retriggering it for component %s:%s", component.GetNamespace(), component.GetName())
		case event := <-watch.ResultChan():
			if event.Object == nil {
				continue
			}
			newPR, ok := event.Object.(*pipeline.PipelineRun)
			if !ok {
				continue
			}
			if pr.GetGenerateName() == newPR.GetGenerateName() && pr.GetName() != newPR.GetName() {
				newPRFound = true
			}
		}
		if newPRFound {
			break
		}
	}

	return sha, nil
}

func (h *HasController) CheckImageRepositoryExists(namespace, componentName string) wait.ConditionFunc {
	return func() (bool, error) {
		imageRepositoryList := &imagecontroller.ImageRepositoryList{}
		imageRepoLabels := map[string]string{"appstudio.redhat.com/component": componentName}
		err := h.KubeRest().List(context.Background(), imageRepositoryList, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(imageRepoLabels), Namespace: namespace})
		if err != nil {
			return false, err
		}
		return len(imageRepositoryList.Items) == 1 && imageRepositoryList.Items[0].Status.State == "ready", nil
	}
}

// Gets value of a specified annotation in a component
func (h *HasController) GetComponentAnnotation(componentName, annotationKey, namespace string) (string, error) {
	component, err := h.GetComponent(componentName, namespace)
	if err != nil {
		return "", fmt.Errorf("error when getting component: %+v", err)
	}
	return component.Annotations[annotationKey], nil
}

// Sets annotation in a component
func (h *HasController) SetComponentAnnotation(componentName, annotationKey, annotationValue, namespace string) error {
	component, err := h.GetComponent(componentName, namespace)
	if err != nil {
		return fmt.Errorf("error when getting component: %+v", err)
	}
	newAnnotations := component.GetAnnotations()
	newAnnotations[annotationKey] = annotationValue
	component.SetAnnotations(newAnnotations)
	err = h.KubeRest().Update(context.Background(), component)
	if err != nil {
		return fmt.Errorf("error when updating component: %+v", err)
	}
	return nil
}

// StoreComponent stores a given Component as an artifact.
func (h *HasController) StoreComponent(component *appservice.Component) error {
	artifacts := make(map[string][]byte)

	componentConditionStatus, err := h.GetComponentConditionStatusMessages(component.Name, component.Namespace)
	if err != nil {
		return err
	}
	artifacts["component-condition-status-"+component.Name+".log"] = []byte(strings.Join(componentConditionStatus, "\n"))

	componentYaml, err := yaml.Marshal(component)
	if err != nil {
		return err
	}
	artifacts["component-"+component.Name+".yaml"] = componentYaml

	if err := logs.StoreArtifacts(artifacts); err != nil {
		return err
	}

	return nil
}

// StoreAllComponents stores all Components in a given namespace.
func (h *HasController) StoreAllComponents(namespace string) error {
	componentList := &appservice.ComponentList{}
	if err := h.KubeRest().List(context.Background(), componentList, &rclient.ListOptions{Namespace: namespace}); err != nil {
		return err
	}

	for _, component := range componentList.Items {
		if err := h.StoreComponent(&component); err != nil {
			return err
		}
	}
	return nil
}

// UpdateComponent updates a component
func (h *HasController) UpdateComponent(component *appservice.Component) error {
	err := h.KubeRest().Update(context.Background(), component, &rclient.UpdateOptions{})

	if err != nil {
		return err
	}
	return nil
}
