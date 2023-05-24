package has

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"knative.dev/pkg/apis"

	. "github.com/onsi/ginkgo/v2"
	routev1 "github.com/openshift/api/route/v1"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	Github *github.Github
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SuiteController, error) {
	// Check if a github organization env var is set, if not use by default the redhat-appstudio-qe org. See: https://github.com/redhat-appstudio-qe
	org := utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	token := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
	gh, err := github.NewGithubClient(token, org)
	if err != nil {
		return nil, err
	}
	return &SuiteController{
		gh,
		kube,
	}, nil
}

func (h *SuiteController) refreshComponentForErrorDebug(component *appservice.Component) *appservice.Component {
	retComp := &appservice.Component{}
	key := rclient.ObjectKeyFromObject(component)
	err := h.KubeRest().Get(context.Background(), key, retComp)
	if err != nil {
		//TODO let's log this somehow, but return the original component obj, as that is better than nothing
		return component
	}
	return retComp
}

func (h *SuiteController) refreshApplicationForErrorDebug(application *appservice.Application) *appservice.Application {
	retApp := &appservice.Application{}
	key := rclient.ObjectKeyFromObject(application)
	err := h.KubeRest().Get(context.Background(), key, retApp)
	if err != nil {
		return application
	}
	return retApp
}

// GetHasApplication return the Application Custom Resource object
func (h *SuiteController) GetHasApplication(name, namespace string) (*appservice.Application, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	application := appservice.Application{
		Spec: appservice.ApplicationSpec{},
	}
	err := h.KubeRest().Get(context.TODO(), namespacedName, &application)
	if err != nil {
		return nil, err
	}
	return &application, nil
}

// CreateHasApplication create an application Custom Resource object
func (h *SuiteController) CreateHasApplication(name, namespace string) (*appservice.Application, error) {
	return h.CreateHasApplicationWithTimeout(name, namespace, time.Minute*10)
}

// CreateHasApplicationWithTimeout create an application Custom Resource object
func (h *SuiteController) CreateHasApplicationWithTimeout(name string, namespace string, timeout time.Duration) (*appservice.Application, error) {
	application := &appservice.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appservice.ApplicationSpec{
			DisplayName: name,
		},
	}
	err := h.KubeRest().Create(context.TODO(), application)
	if err != nil {
		return nil, err
	}

	if err := utils.WaitUntil(h.ApplicationDevfilePresent(application), timeout); err != nil {
		application = h.refreshApplicationForErrorDebug(application)
		return nil, fmt.Errorf("timed out when waiting for devfile content creation for application %s in %s namespace: %+v. applicattion: %s", name, namespace, err, utils.ToPrettyJSONString(application))
	}

	return application, nil
}

func (h *SuiteController) ApplicationDevfilePresent(application *appservice.Application) wait.ConditionFunc {
	return func() (bool, error) {
		app, err := h.GetHasApplication(application.Name, application.Namespace)
		if err != nil {
			return false, nil
		}
		application.Status = app.Status
		return application.Status.Devfile != "", nil
	}
}

// DeleteHasApplication delete a HAS Application resource from the namespace.
// Optionally, it can avoid returning an error if the resource did not exist:
// - specify 'false', if it's likely the Application has already been deleted (for example, because the Namespace was deleted)
func (h *SuiteController) DeleteHasApplication(name, namespace string, reportErrorOnNotFound bool) error {
	application := appservice.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := h.KubeRest().Delete(context.TODO(), &application); err != nil {
		if !k8sErrors.IsNotFound(err) || (k8sErrors.IsNotFound(err) && reportErrorOnNotFound) {
			return fmt.Errorf("error deleting an application: %+v", err)
		}
	}
	return utils.WaitUntil(h.ApplicationDeleted(&application), 1*time.Minute)
}

func (h *SuiteController) ApplicationDeleted(application *appservice.Application) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := h.GetHasApplication(application.Name, application.Namespace)
		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}

// GetHasComponent returns the Appstudio Component Custom Resource object
func (h *SuiteController) GetHasComponent(name, namespace string) (*appservice.Component, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	component := appservice.Component{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, &component)
	if err != nil {
		return nil, err
	}
	return &component, nil
}

// ScaleDeploymentReplicas scales the replicas of a given deployment
func (h *SuiteController) ScaleComponentReplicas(component *appservice.Component, replicas *int) (*appservice.Component, error) {
	component.Spec.Replicas = replicas

	err := h.KubeRest().Update(context.TODO(), component, &rclient.UpdateOptions{})
	if err != nil {
		return &appservice.Component{}, err
	}
	return component, nil
}

// DeleteHasComponent delete an has component from a given name and namespace
func (h *SuiteController) DeleteHasComponent(name string, namespace string, reportErrorOnNotFound bool) error {
	component := appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := h.KubeRest().Delete(context.TODO(), &component); err != nil {
		if !k8sErrors.IsNotFound(err) || (k8sErrors.IsNotFound(err) && reportErrorOnNotFound) {
			return fmt.Errorf("error deleting a component: %+v", err)
		}
	}

	return utils.WaitUntil(h.ComponentDeleted(&component), 1*time.Minute)
}

// CreateComponent create an has component from a given name, namespace, application, devfile and a container image
func (h *SuiteController) CreateComponent(applicationName, componentName, namespace, gitSourceURL, gitSourceRevision, containerImageSource, outputContainerImage, secret string, skipInitialChecks bool) (*appservice.Component, error) {
	var containerImage string
	annotations := map[string]string{
		// PLNSRVCE-957 - if true, run only basic build pipeline tasks
		"skip-initial-checks": strconv.FormatBool(skipInitialChecks),
	}
	if outputContainerImage != "" {
		containerImage = outputContainerImage
	} else if containerImageSource != "" {
		containerImage = containerImageSource
	} else {
		// When no image image is selected then add annotatation to generate new image repository
		annotations = utils.MergeMaps(annotations, constants.ImageControllerAnnotationDeleteRepoTrue)
	}
	component := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Labels:      constants.ComponentDefaultLabel,
			Name:        componentName,
			Namespace:   namespace,
		},
		Spec: appservice.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appservice.ComponentSource{
				ComponentSourceUnion: appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL:      gitSourceURL,
						Revision: gitSourceRevision,
					},
				},
			},
			Secret:         secret,
			ContainerImage: containerImage,
			Replicas:       pointer.Int(1),
			TargetPort:     8081,
			Route:          "",
		},
	}
	err := h.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	if err = utils.WaitUntil(h.ComponentReady(component), time.Minute*10); err != nil {
		component = h.refreshComponentForErrorDebug(component)
		return nil, fmt.Errorf("timed out when waiting for component %s to be ready in %s namespace. component: %s", componentName, namespace, utils.ToPrettyJSONString(component))
	}
	return component, nil
}

func (h *SuiteController) ComponentReady(component *appservice.Component) wait.ConditionFunc {
	return func() (bool, error) {
		messages, err := h.GetHasComponentConditionStatusMessages(component.Name, component.Namespace)
		if err != nil {
			return false, nil
		}
		for _, m := range messages {
			if strings.Contains(m, "success") {
				return true, nil
			}
		}
		return false, nil
	}
}

func (h *SuiteController) ComponentDeleted(component *appservice.Component) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := h.GetHasComponent(component.Name, component.Namespace)
		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}

// CreateComponentWithPaCEnabled creates a component with "pipelinesascode: '1'" annotation that is used for triggering PaC builds
func (h *SuiteController) CreateComponentWithPaCEnabled(applicationName, componentName, namespace, gitSourceURL, baseBranch string, deleteRepo bool) (*appservice.Component, error) {

	var annotations map[string]string
	if deleteRepo {
		annotations = utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationDeleteRepoTrue)
	} else {
		annotations = utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationDeleteRepoFalse)
	}

	component := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Name:        componentName,
			Namespace:   namespace,
		},
		Spec: appservice.ComponentSpec{
			ComponentName: componentName,
			Application:   applicationName,
			Source: appservice.ComponentSource{
				ComponentSourceUnion: appservice.ComponentSourceUnion{
					GitSource: &appservice.GitSource{
						URL:      gitSourceURL,
						Revision: baseBranch,
					},
				},
			},
		},
	}
	err := h.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	if err = utils.WaitUntil(h.ComponentReady(component), time.Minute*10); err != nil {
		component = h.refreshComponentForErrorDebug(component)
		return nil, fmt.Errorf("timed out when waiting for component %s to be ready in %s namespace. component: %s", componentName, namespace, utils.ToPrettyJSONString(component))
	}
	return component, nil
}

func (h *SuiteController) CreateComponentFromStubSkipInitialChecks(compDetected appservice.ComponentDetectionDescription, namespace string, outputContainerImage string, secret string, applicationName string, skipInitialChecks bool) (*appservice.Component, error) {
	component := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"skip-initial-checks": strconv.FormatBool(skipInitialChecks),
			},
			Name:      compDetected.ComponentStub.ComponentName,
			Namespace: namespace,
		},
		Spec: compDetected.ComponentStub,
	}

	if outputContainerImage != "" {
		component.Spec.ContainerImage = outputContainerImage
	} else {
		component.Annotations = utils.MergeMaps(component.Annotations, constants.ImageControllerAnnotationDeleteRepoTrue)
	}

	if component.Spec.TargetPort == 0 {
		component.Spec.TargetPort = 8081
	}

	component.Spec.Secret = secret
	component.Spec.Application = applicationName

	err := h.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	if err = utils.WaitUntil(h.ComponentReady(component), time.Minute*10); err != nil {
		component = h.refreshComponentForErrorDebug(component)
		return nil, fmt.Errorf("timed out when waiting for component %s to be ready in %s namespace. component: %s", compDetected.ComponentStub.ComponentName, namespace, utils.ToPrettyJSONString(component))
	}
	return component, nil
}

// CreateComponentFromStub create a HAS Component resource from a Completed CDQ resource, which includes a stub Component CR
// The Component from the CDQ is only a template, and needs things like name filled in
func (h *SuiteController) CreateComponentFromStub(compDetected appservice.ComponentDetectionDescription, namespace string, outputContainerImage string, secret string, applicationName string) (*appservice.Component, error) {
	return h.CreateComponentFromStubSkipInitialChecks(compDetected, namespace, outputContainerImage, secret, applicationName, true)
}

// DeleteHasComponent delete an has component from a given name and namespace
func (h *SuiteController) DeleteHasComponentDetectionQuery(name string, namespace string) error {
	component := appservice.ComponentDetectionQuery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return h.KubeRest().Delete(context.TODO(), &component)
}

// CreateComponentDetectionQuery create a has componentdetectionquery from a given name, namespace, and git source
func (h *SuiteController) CreateComponentDetectionQuery(cdqName, namespace, gitSourceURL, gitSourceRevision, gitSourceContext, secret string, isMultiComponent bool) (*appservice.ComponentDetectionQuery, error) {
	return h.CreateComponentDetectionQueryWithTimeout(cdqName, namespace, gitSourceURL, gitSourceRevision, gitSourceContext, secret, isMultiComponent, 5*time.Minute)
}

// CreateComponentDetectionQueryWithTimeout create a has componentdetectionquery from a given name, namespace, and git source and waits for it to be read
func (h *SuiteController) CreateComponentDetectionQueryWithTimeout(cdqName, namespace, gitSourceURL, gitSourceRevision, gitSourceContext, secret string, isMultiComponent bool, timeout time.Duration) (*appservice.ComponentDetectionQuery, error) {
	componentDetectionQuery := &appservice.ComponentDetectionQuery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cdqName,
			Namespace: namespace,
		},
		Spec: appservice.ComponentDetectionQuerySpec{
			GitSource: appservice.GitSource{
				URL:      gitSourceURL,
				Revision: gitSourceRevision,
				Context:  gitSourceContext,
			},
			Secret: secret,
		},
	}
	err := h.KubeRest().Create(context.TODO(), componentDetectionQuery)
	if err != nil {
		return nil, err
	}

	err = utils.WaitUntil(func() (done bool, err error) {
		componentDetectionQuery, err = h.GetComponentDetectionQuery(componentDetectionQuery.Name, componentDetectionQuery.Namespace)
		if err != nil {
			return false, err
		}
		for _, condition := range componentDetectionQuery.Status.Conditions {
			if condition.Type == "Completed" && len(componentDetectionQuery.Status.ComponentDetected) > 0 {
				return true, nil
			}
		}
		return false, nil
	}, timeout)

	if err != nil {
		return nil, fmt.Errorf("error waiting for cdq to be ready: %v", err)
	}

	return componentDetectionQuery, nil
}

// GetComponentDetectionQuery return the status from the ComponentDetectionQuery Custom Resource object
func (h *SuiteController) GetComponentDetectionQuery(name, namespace string) (*appservice.ComponentDetectionQuery, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	componentDetectionQuery := appservice.ComponentDetectionQuery{
		Spec: appservice.ComponentDetectionQuerySpec{},
	}
	err := h.KubeRest().Get(context.TODO(), namespacedName, &componentDetectionQuery)
	if err != nil {
		return nil, err
	}
	return &componentDetectionQuery, nil
}

// GetComponentPipeline returns the pipeline for a given component labels
func (h *SuiteController) GetComponentPipelineRun(componentName, applicationName, namespace, sha string) (*v1beta1.PipelineRun, error) {
	pipelineRunLabels := map[string]string{"appstudio.openshift.io/component": componentName, "appstudio.openshift.io/application": applicationName}

	if sha != "" {
		pipelineRunLabels["pipelinesascode.tekton.dev/sha"] = sha
	}

	list := &v1beta1.PipelineRunList{}
	err := h.KubeRest().List(context.TODO(), list, &rclient.ListOptions{LabelSelector: labels.SelectorFromSet(pipelineRunLabels), Namespace: namespace})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing pipelineruns in %s namespace: %v", namespace, err)
	}

	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}

	return nil, fmt.Errorf("no pipelinerun found for component %s", componentName)
}

// GetEventListenerRoute returns the route for a given component name's event listener
func (h *SuiteController) GetEventListenerRoute(componentName string, componentNamespace string) (*routev1.Route, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("el%s", componentName),
		Namespace: componentNamespace,
	}
	route := &routev1.Route{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, route)
	if err != nil {
		return &routev1.Route{}, err
	}
	return route, nil
}

// GetComponentDeployment returns the deployment for a given component name
func (h *SuiteController) GetComponentDeployment(componentName string, componentNamespace string) (*appsv1.Deployment, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("el-%s", componentName),
		Namespace: componentNamespace,
	}

	deployment := &appsv1.Deployment{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		return &appsv1.Deployment{}, err
	}
	return deployment, nil
}

// GetComponentService returns the service for a given component name
func (h *SuiteController) GetComponentService(componentName string, componentNamespace string) (*corev1.Service, error) {
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("el-%s", componentName),
		Namespace: componentNamespace,
	}

	service := &corev1.Service{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, service)
	if err != nil {
		return &corev1.Service{}, err
	}
	return service, nil
}

func (h *SuiteController) WaitForComponentPipelineToBeFinished(component *appservice.Component, sha string, maxRetries int) error {
	attempts := 1
	app := component.Spec.Application
	var pr *v1beta1.PipelineRun

	for {
		err := wait.PollImmediate(20*time.Second, 30*time.Minute, func() (done bool, err error) {
			pr, err = h.GetComponentPipelineRun(component.GetName(), app, component.GetNamespace(), sha)

			if err != nil {
				GinkgoWriter.Println("PipelineRun has not been created yet")
				return false, nil
			}

			for _, condition := range pr.Status.Conditions {
				GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pr.Name, condition.Reason)

				if !pr.IsDone() {
					return false, nil
				}

				if pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
					return true, nil
				} else {
					err = tekton.StorePipelineRun(pr, h.KubeRest(), h.KubeInterface())
					if err != nil {
						GinkgoWriter.Printf("failed to store pipelineRun: %s\n", err.Error())
					}
					return false, fmt.Errorf(tekton.GetFailedPipelineRunLogs(h.KubeRest(), h.KubeInterface(), pr))
				}
			}
			return false, nil
		})

		if err != nil {
			if attempts == maxRetries+1 {
				return err
			}
			GinkgoWriter.Printf("attempt %s/%s: PipelineRun %q failed: %+v", attempts, maxRetries+1, pr.GetName(), err)
			if err = h.KubeRest().Delete(context.TODO(), pr); err != nil {
				return fmt.Errorf("failed to delete PipelineRun %q from %q namespace", pr.GetName(), pr.GetNamespace())
			}

			if err := utils.WaitUntil(h.PipelineRunDeleted(pr), 1*time.Minute); err != nil {
				return fmt.Errorf("timed out waiting for PipelineRun %q from namespace %q to get deleted: %+v", pr.GetName(), pr.GetNamespace(), err)
			}

			component, err := h.GetHasComponent(component.GetName(), component.GetNamespace())
			if err != nil {
				return fmt.Errorf("failed to get component %q for PipelineRun %q in %q namespace: %+v", component.GetName(), pr.GetName(), pr.GetNamespace(), err)
			}
			delete(component.Annotations, constants.ComponentInitialBuildAnnotationKey)

			if err = h.KubeRest().Update(context.Background(), component); err != nil {
				return fmt.Errorf("failed to update Component %q in %q namespace", component.GetName(), component.GetNamespace())
			}
			attempts++
		} else {
			break
		}
	}

	return nil

}

// CreateComponentFromDevfile creates a has component from a given name, namespace, application, devfile and a container image
func (h *SuiteController) CreateComponentFromDevfile(applicationName, componentName, namespace, gitSourceURL, devfile, containerImageSource, outputContainerImage, secret string) (*appservice.Component, error) {
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
						URL:        gitSourceURL,
						DevfileURL: devfile,
					},
				},
			},
			Secret:     secret,
			Replicas:   pointer.Int(1),
			TargetPort: 8080,
			Route:      "",
		},
	}
	if outputContainerImage != "" {
		component.Spec.ContainerImage = outputContainerImage
	} else if containerImageSource != "" {
		component.Spec.ContainerImage = containerImageSource
	} else {
		component.Annotations = constants.ImageControllerAnnotationDeleteRepoTrue
	}
	err := h.KubeRest().Create(context.TODO(), component)
	if err != nil {
		return nil, err
	}
	if err = utils.WaitUntil(h.ComponentReady(component), time.Minute*2); err != nil {
		component = h.refreshComponentForErrorDebug(component)
		return nil, fmt.Errorf("timed out when waiting for component %s to be ready in %s namespace. component: %s", componentName, namespace, utils.ToPrettyJSONString(component))
	}
	return component, nil
}

// DeleteAllComponentsInASpecificNamespace removes all component CRs from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *SuiteController) DeleteAllComponentsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &appservice.Component{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting components from the namespace %s: %+v", namespace, err)
	}

	componentList := &appservice.ComponentList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), componentList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(componentList.Items) == 0, nil
	}, timeout)
}

// DeleteAllApplicationsInASpecificNamespace removes all application CRs from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *SuiteController) DeleteAllApplicationsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &appservice.Application{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting applications from the namespace %s: %+v", namespace, err)
	}

	applicationList := &appservice.ApplicationList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), applicationList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(applicationList.Items) == 0, nil
	}, timeout)
}

func (h *SuiteController) GetHasComponentConditionStatusMessages(name, namespace string) (messages []string, err error) {
	c, err := h.GetHasComponent(name, namespace)
	if err != nil {
		return messages, fmt.Errorf("error getting HAS component: %v", err)
	}
	for _, condition := range c.Status.Conditions {
		messages = append(messages, condition.Message)
	}
	return
}

// DeleteAllSnapshotEnvBindingsInASpecificNamespace removes all snapshotEnvironmentBindings from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *SuiteController) DeleteAllSnapshotEnvBindingsInASpecificNamespace(namespace string, timeout time.Duration) error {
	if err := h.KubeRest().DeleteAllOf(context.TODO(), &appservice.SnapshotEnvironmentBinding{}, rclient.InNamespace(namespace)); err != nil {
		return fmt.Errorf("error deleting snapshotEnvironmentBindings from the namespace %s: %+v", namespace, err)
	}

	snapshotEnvironmentBindingList := &appservice.SnapshotEnvironmentBindingList{}
	return utils.WaitUntil(func() (done bool, err error) {
		if err := h.KubeRest().List(context.Background(), snapshotEnvironmentBindingList, &rclient.ListOptions{Namespace: namespace}); err != nil {
			return false, nil
		}
		return len(snapshotEnvironmentBindingList.Items) == 0, nil
	}, timeout)
}

func (s *SuiteController) ApplicationGitopsRepoExists(devfileContent string) wait.ConditionFunc {
	return func() (bool, error) {
		gitOpsRepoURL := utils.ObtainGitOpsRepositoryName(devfileContent)
		return s.Github.CheckIfRepositoryExist(gitOpsRepoURL), nil
	}
}

func (h *SuiteController) PipelineRunDeleted(pr *v1beta1.PipelineRun) wait.ConditionFunc {
	return func() (bool, error) {
		o := &v1beta1.PipelineRun{}
		err := h.KubeRest().Get(context.TODO(), types.NamespacedName{Name: pr.GetName(), Namespace: pr.GetNamespace()}, o)
		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}
