package has

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/apis"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains all methods related with components objects CRUD operations.
type ComponentsInterface interface {
	// Returns an application obj from the kubernetes cluster.
	GetComponent(name string, namespace string) (*appservice.Component, error)

	// Returns an pipelinerun obj related with a given component name from the kubernetes cluster.
	GetComponentPipelineRun(componentName, applicationName, namespace, sha string) (*v1beta1.PipelineRun, error)

	// Waits for a given component to be finished and in case of hitting issue: https://issues.redhat.com/browse/SRVKP-2749 do a given retries.
	WaitForComponentPipelineToBeFinished(component *appservice.Component, sha string, maxRetries int) error

	// Creates an component object in the kubernetes cluster.
	CreateComponent(componentSpec appservice.ComponentSpec, namespace string, outputContainerImage string, secret string, applicationName string, skipInitialChecks bool, annotations map[string]string) (*appservice.Component, error)

	// Modifies the replicas of a component.
	ScaleComponentReplicas(component *appservice.Component, replicas *int) (*appservice.Component, error)

	// Deletes a component object from the given namespace in the kubernetes cluster.
	DeleteComponent(name string, namespace string, reportErrorOnNotFound bool) error

	// Deletes all components from the given namespace in the kubernetes cluster.
	DeleteAllComponentsInASpecificNamespace(namespace string, timeout time.Duration) error
}

// GetHasComponent returns the Appstudio Component Custom Resource object
func (h *hasFactory) GetComponent(name string, namespace string) (*appservice.Component, error) {
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

// GetComponentPipeline returns the pipeline for a given component labels
func (h *hasFactory) GetComponentPipelineRun(componentName string, applicationName string, namespace, sha string) (*v1beta1.PipelineRun, error) {
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

// Waits for a given component to be finished and in case of hitting issue: https://issues.redhat.com/browse/SRVKP-2749 do a given retries.
func (h *hasFactory) WaitForComponentPipelineToBeFinished(component *appservice.Component, sha string, maxRetries int) error {
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

			GinkgoWriter.Printf("PipelineRun %s reason: %s\n", pr.Name, pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason())

			if !pr.IsDone() {
				return false, nil
			}

			if pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
				return true, nil
			} else {
				var prLogs string
				if err = tekton.StorePipelineRun(pr, h.KubeRest(), h.KubeInterface()); err != nil {
					GinkgoWriter.Printf("failed to store PipelineRun %s:%s: %s\n", pr.GetNamespace(), pr.GetName(), err.Error())
				}
				if prLogs, err = tekton.GetFailedPipelineRunLogs(h.KubeRest(), h.KubeInterface(), pr); err != nil {
					GinkgoWriter.Printf("failed to get logs for PipelineRun %s:%s: %s\n", pr.GetNamespace(), pr.GetName(), err.Error())
				}
				return false, fmt.Errorf(prLogs)
			}
		})

		if err != nil {
			GinkgoWriter.Printf("attempt %d/%d: PipelineRun %q failed: %+v", attempts, maxRetries+1, pr.GetName(), err)
			// Retry the PipelineRun only in case we hit the known issue https://issues.redhat.com/browse/SRVKP-2749
			if attempts == maxRetries+1 || pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).GetReason() != "CouldntGetTask" {
				return err
			}
			if sha, err = h.RetriggerComponentPipelineRun(component, pr); err != nil {
				return fmt.Errorf("unable to retrigger component %s:%s: %+v", component.GetNamespace(), component.GetName(), err)
			}
			attempts++
		} else {
			break
		}
	}

	return nil
}

// Universal method to create a component in the kubernetes clusters.
func (h *hasFactory) CreateComponent(componentSpec appservice.ComponentSpec, namespace string, outputContainerImage string, secret string, applicationName string, skipInitialChecks bool, annotations map[string]string) (*appservice.Component, error) {
	componentObject := &appservice.Component{
		ObjectMeta: metav1.ObjectMeta{
			// adding default label because of the BuildPipelineSelector in build test
			Labels:    constants.ComponentDefaultLabel,
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
	componentObject.Annotations = utils.MergeMaps(componentObject.Annotations, annotations)

	if componentObject.Spec.TargetPort == 0 {
		componentObject.Spec.TargetPort = 8081
	}
	if outputContainerImage != "" {
		componentObject.Spec.ContainerImage = outputContainerImage
	} else {
		componentObject.Annotations = utils.MergeMaps(constants.ComponentPaCRequestAnnotation, constants.ImageControllerAnnotationRequestPublicRepo)
	}

	if err := h.KubeRest().Create(context.TODO(), componentObject); err != nil {
		return nil, err
	}
	if err := utils.WaitUntil(h.ComponentReady(componentObject), time.Minute*10); err != nil {
		componentObject = h.refreshComponentForErrorDebug(componentObject)
		return nil, fmt.Errorf("timed out when waiting for component %s to be ready in %s namespace. component: %s", componentSpec.ComponentName, namespace, utils.ToPrettyJSONString(componentObject))
	}
	return componentObject, nil
}

// ScaleDeploymentReplicas scales the replicas of a given deployment
func (h *hasFactory) ScaleComponentReplicas(component *appservice.Component, replicas *int) (*appservice.Component, error) {
	component.Spec.Replicas = replicas

	err := h.KubeRest().Update(context.TODO(), component, &rclient.UpdateOptions{})
	if err != nil {
		return &appservice.Component{}, err
	}
	return component, nil
}

// DeleteComponent delete an has component from a given name and namespace
func (h *hasFactory) DeleteComponent(name string, namespace string, reportErrorOnNotFound bool) error {
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

// DeleteAllComponentsInASpecificNamespace removes all component CRs from a specific namespace. Useful when creating a lot of resources and want to remove all of them
func (h *hasFactory) DeleteAllComponentsInASpecificNamespace(namespace string, timeout time.Duration) error {
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

// Waits for a component to be reconciled in the application service.
func (h *hasFactory) ComponentReady(component *appservice.Component) wait.ConditionFunc {
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

// Waits for a component until is deleted and if not will return an error
func (h *hasFactory) ComponentDeleted(component *appservice.Component) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := h.GetComponent(component.Name, component.Namespace)
		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}

// Get the message from the status of a component. Usefull for debugging purposes.
func (h *hasFactory) GetHasComponentConditionStatusMessages(name, namespace string) (messages []string, err error) {
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
func (h *hasFactory) RetriggerComponentPipelineRun(component *appservice.Component, pr *v1beta1.PipelineRun) (sha string, err error) {
	if err = h.KubeRest().Delete(context.TODO(), pr); err != nil {
		return "", fmt.Errorf("failed to delete PipelineRun %q from %q namespace", pr.GetName(), pr.GetNamespace())
	}

	prLabels := pr.GetLabels()
	// In case of PipelineRun managed by PaC we are able to retrigger the pipeline only
	// by updating the related branch
	if prLabels["app.kubernetes.io/managed-by"] == "pipelinesascode.tekton.dev" {
		var ok bool
		var repoName, eventType, branchName string
		pacRepoNameLabelName := "pipelinesascode.tekton.dev/url-repository"
		pacEventTypeLabelName := "pipelinesascode.tekton.dev/event-type"
		componentLabelName := "appstudio.openshift.io/component"
		targetBranchAnnotationName := "build.appstudio.redhat.com/target_branch"

		if repoName, ok = prLabels[pacRepoNameLabelName]; !ok {
			return "", fmt.Errorf("cannot retrigger PipelineRun - required label %q not found", pacRepoNameLabelName)
		}
		if eventType, ok = prLabels[pacEventTypeLabelName]; !ok {
			return "", fmt.Errorf("cannot retrigger PipelineRun - required label %q not found", pacEventTypeLabelName)
		}
		// PipelineRun is triggered from a pull request, need to update the PaC PR source branch
		if eventType == "pull_request" {
			if len(prLabels[componentLabelName]) < 1 {
				return "", fmt.Errorf("cannot retrigger PipelineRun - required label %q not found", componentLabelName)
			}
			branchName = constants.PaCPullRequestBranchPrefix + prLabels[componentLabelName]
		} else {
			// No straightforward way to get a target branch from PR labels -> using annotation
			if branchName, ok = pr.GetAnnotations()[targetBranchAnnotationName]; !ok {
				return "", fmt.Errorf("cannot retrigger PipelineRun - required annotation %q not found", targetBranchAnnotationName)
			}
		}
		file, err := h.Github.CreateFile(repoName, util.GenerateRandomString(5), "test", branchName)
		if err != nil {
			return "", fmt.Errorf("failed to retrigger PipelineRun %s in %s namespace: %+v", pr.GetName(), pr.GetNamespace(), err)
		}
		sha = file.GetSHA()

		// To retrigger simple build PipelineRun we just need to update the initial build annotation
		// in Component CR
	} else {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			component, err := h.GetComponent(component.GetName(), component.GetNamespace())
			if err != nil {
				return fmt.Errorf("failed to get component for PipelineRun %q in %q namespace: %+v", pr.GetName(), pr.GetNamespace(), err)
			}
			delete(component.Annotations, constants.ComponentInitialBuildAnnotationKey)
			if err = h.KubeRest().Update(context.Background(), component); err != nil {
				return fmt.Errorf("failed to update Component %q in %q namespace", component.GetName(), component.GetNamespace())
			}
			return err
		})

		if err != nil {
			return "", err
		}
	}
	watch, err := h.PipelineClient().TektonV1beta1().PipelineRuns(component.GetNamespace()).Watch(context.Background(), metav1.ListOptions{})
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
			newPR, ok := event.Object.(*v1beta1.PipelineRun)
			if !ok {
				continue
			}
			if pr.GetName() != newPR.GetName() {
				newPRFound = true
			}
		}
		if newPRFound {
			break
		}
	}

	return sha, nil
}

// refreshApplicationForErrorDebug return the latest component object from the kubernetes cluster.
func (h *hasFactory) refreshComponentForErrorDebug(component *appservice.Component) *appservice.Component {
	retComp := &appservice.Component{}
	key := rclient.ObjectKeyFromObject(component)
	err := h.KubeRest().Get(context.Background(), key, retComp)
	if err != nil {
		//TODO let's log this somehow, but return the original component obj, as that is better than nothing
		return component
	}
	return retComp
}
