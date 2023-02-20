package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Create the struct for kubernetes clients
type SuiteController struct {
	*kubeCl.CustomClient
	Github *github.Github
}

// Create controller for Application/Component crud operations
func NewSuiteController(kubeC *kubeCl.CustomClient) (*SuiteController, error) {
	// Check if a github organization env var is set, if not use by default the redhat-appstudio-qe org. See: https://github.com/redhat-appstudio-qe
	org := utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	token := utils.GetEnv(constants.GITHUB_TOKEN_ENV, "")
	gh := github.NewGithubClient(token, org)
	return &SuiteController{
		kubeC,
		gh,
	}, nil
}

// GetClusterTask return a clustertask object from cluster and if don't exist returns an error
func (s *SuiteController) GetClusterTask(name string, namespace string) (*v1beta1.ClusterTask, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	clusterTask := &v1beta1.ClusterTask{}
	if err := s.KubeRest().Get(context.TODO(), namespacedName, clusterTask); err != nil {
		return nil, err
	}
	return clusterTask, nil
}

// ListClusterTask return a list of ClusterTasks with a specific label selectors
func (s *SuiteController) CheckIfClusterTaskExists(name string) bool {
	clusterTasks := &v1beta1.ClusterTaskList{}
	if err := s.KubeRest().List(context.TODO(), clusterTasks, &rclient.ListOptions{}); err != nil {
		return false
	}
	for _, ctasks := range clusterTasks.Items {
		if ctasks.Name == name {
			return true
		}
	}
	return false
}

// Creates a new secret in a specified namespace
func (s *SuiteController) CreateSecret(ns string, secret *corev1.Secret) (*corev1.Secret, error) {
	return s.KubeInterface().CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
}

// Check if a secret exists, return secret and error
func (s *SuiteController) GetSecret(ns string, name string) (*corev1.Secret, error) {
	return s.KubeInterface().CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
}

// Deleted a secret in a specified namespace
func (s *SuiteController) DeleteSecret(ns string, name string) error {
	return s.KubeInterface().CoreV1().Secrets(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// Links a secret to a specified serviceaccount
func (s *SuiteController) LinkSecretToServiceAccount(ns, secret, serviceaccount string) error {
	serviceAccountObject, err := s.KubeInterface().CoreV1().ServiceAccounts(ns).Get(context.TODO(), serviceaccount, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, credentialSecret := range serviceAccountObject.Secrets {
		if credentialSecret.Name == secret {
			// The secret is present in the service account, no updates needed
			return nil
		}
	}
	serviceAccountObject.Secrets = append(serviceAccountObject.Secrets, corev1.ObjectReference{Name: secret})
	_, err = s.KubeInterface().CoreV1().ServiceAccounts(ns).Update(context.TODO(), serviceAccountObject, metav1.UpdateOptions{})
	return err
}

func (s *SuiteController) GetPod(namespace, podName string) (*corev1.Pod, error) {
	return s.KubeInterface().CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
}

func (s *SuiteController) IsPodRunning(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := s.GetPod(namespace, podName)
		if err != nil {
			return false, nil
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, fmt.Errorf("pod %q ran to completion", pod.Name)
		}
		return false, nil
	}
}

func (s *SuiteController) IsPodSuccessful(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := s.GetPod(namespace, podName)
		if err != nil {
			return false, nil
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			return false, fmt.Errorf("pod %q has failed", pod.Name)
		}
		return false, nil
	}
}

func TaskPodExists(tr *v1beta1.TaskRun) wait.ConditionFunc {
	return func() (bool, error) {
		if tr.Status.PodName != "" {
			return true, nil
		}
		return false, nil
	}
}

func (s *SuiteController) ListPods(namespace, labelKey, labelValue string, selectionLimit int64) (*corev1.PodList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         selectionLimit,
	}
	return s.KubeInterface().CoreV1().Pods(namespace).List(context.TODO(), listOptions)
}

func (s *SuiteController) ListRoles(namespace string) (*rbacv1.RoleList, error) {

	listOptions := metav1.ListOptions{}
	return s.KubeInterface().RbacV1().Roles(namespace).List(context.TODO(), listOptions)
}

func (s *SuiteController) ListRoleBindings(namespace string) (*rbacv1.RoleBindingList, error) {

	listOptions := metav1.ListOptions{}
	return s.KubeInterface().RbacV1().RoleBindings(namespace).List(context.TODO(), listOptions)
}

func (s *SuiteController) GetContainerLogs(podName, containerName, namespace string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}

	req := s.KubeInterface().CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error in opening the stream: %v", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error in copying logs to buf, %v", err)
	}
	return buf.String(), nil
}

func (s *SuiteController) WaitForPodSelector(
	fn func(podName, namespace string) wait.ConditionFunc, namespace, labelKey string, labelValue string,
	timeout int, selectionLimit int64) error {
	podList, err := s.ListPods(namespace, labelKey, labelValue, selectionLimit)
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods in %s with label key %s and label value %s", namespace, labelKey, labelValue)
	}

	for i := range podList.Items {
		if err := utils.WaitUntil(fn(podList.Items[i].Name, namespace), time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (s *SuiteController) GetRole(roleName, namespace string) (*rbacv1.Role, error) {
	return s.KubeInterface().RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
}

func (s *SuiteController) GetRoleBinding(rolebindingName, namespace string) (*rbacv1.RoleBinding, error) {
	return s.KubeInterface().RbacV1().RoleBindings(namespace).Get(context.TODO(), rolebindingName, metav1.GetOptions{})
}

func (s *SuiteController) GetServiceAccount(saName, namespace string) (*corev1.ServiceAccount, error) {
	return s.KubeInterface().CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saName, metav1.GetOptions{})
}

// GetOpenshiftRoute returns the route for a given component name
func (h *SuiteController) GetOpenshiftRoute(routeName string, routeNamespace string) (*routev1.Route, error) {
	namespacedName := types.NamespacedName{
		Name:      routeName,
		Namespace: routeNamespace,
	}

	route := &routev1.Route{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, route)
	if err != nil {
		return &routev1.Route{}, err
	}
	return route, nil
}

// GetAppDeploymentByName returns the deployment for a given component name
func (h *SuiteController) GetAppDeploymentByName(deploymentName string, deploymentNamespace string) (*appsv1.Deployment, error) {
	namespacedName := types.NamespacedName{
		Name:      deploymentName,
		Namespace: deploymentNamespace,
	}

	deployment := &appsv1.Deployment{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		return &appsv1.Deployment{}, err
	}
	return deployment, nil
}

// GetServiceByName returns the service for a given component name
func (h *SuiteController) GetServiceByName(serviceName string, serviceNamespace string) (*corev1.Service, error) {
	namespacedName := types.NamespacedName{
		Name:      serviceName,
		Namespace: serviceNamespace,
	}

	service := &corev1.Service{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, service)
	if err != nil {
		return &corev1.Service{}, err
	}
	return service, nil
}

func (s *SuiteController) CreateConfigMap(cm *corev1.ConfigMap, namespace string) (*corev1.ConfigMap, error) {
	return s.KubeInterface().CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
}

func (s *SuiteController) UpdateConfigMap(cm *corev1.ConfigMap, namespace string) (*corev1.ConfigMap, error) {
	return s.KubeInterface().CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
}

func (s *SuiteController) GetConfigMap(name, namespace string) (*corev1.ConfigMap, error) {
	return s.KubeInterface().CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// DeleteConfigMaps delete a ConfigMap. Optionally, it can avoid returning an error if the resource did not exist:
// - specify 'false' if it's likely the ConfigMap has already been deleted (for example, because the Namespace was deleted)
func (s *SuiteController) DeleteConfigMap(name, namespace string, returnErrorOnNotFound bool) error {
	err := s.KubeInterface().CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && k8sErrors.IsNotFound(err) && !returnErrorOnNotFound {
		err = nil // Ignore not found errors, if requested
	}
	return err
}

func (s *SuiteController) CreateRegistryAuthSecret(secretName, namespace, secretData string) (*corev1.Secret, error) {
	rawDecodedText, err := base64.StdEncoding.DecodeString(secretData)
	if err != nil {
		return nil, err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type:       "kubernetes.io/dockerconfigjson",
		StringData: map[string]string{".dockerconfigjson": string(rawDecodedText)},
	}
	er := s.KubeRest().Create(context.TODO(), secret)
	if er != nil {
		return nil, er
	}
	return secret, nil
}

// DeleteNamespace deletes the give namespace.
func (s *SuiteController) DeleteNamespace(namespace string) error {
	_, err := s.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("could not check for namespace '%s' existence: %v", namespace, err)
	}

	if err := s.KubeInterface().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("unable to delete namespace '%s': %v", namespace, err)
	}

	// Wait for the namespace to no longer exist. The namespace may remain stuck in 'Terminating' state
	// if it contains with finalizers that are not handled. We detect this case here, and report any resources still
	// in the Namespace.
	if err := utils.WaitUntil(s.namespaceDoesNotExist(namespace), time.Minute*10); err != nil {

		// On failure to delete, list all namespace-scoped resources still in the namespace.
		resourcesInNamespace := s.ListNamespaceScopedResourcesAsString(namespace, s.KubeInterface(), s.DynamicClient())

		return fmt.Errorf("namespace was not deleted in expected timeframe: '%s': %v. Remaining resources in namespace: %s", namespace, err, resourcesInNamespace)
	}

	return nil

}

// ListNamespaceScopedResourcesAsString returns a list of resources in a namespace as a string, for test debugging purposes.
func (s *SuiteController) ListNamespaceScopedResourcesAsString(namespace string, k8sInterface kubernetes.Interface, dynamicInterface dynamic.Interface) string {
	crdList, err := k8sInterface.Discovery().ServerPreferredNamespacedResources()
	if err != nil {
		// Ignore errors: this function is for diagnostic purposes only.
		return ""
	}
	resourceList := ""

	for _, crd := range crdList {

		for _, apiResource := range crd.APIResources {

			if !apiResource.Namespaced {
				continue
			}

			name := apiResource.Name

			// package manifests is projected into every Namespace: so just ignore it.
			if name == "packagemanifests" {
				continue
			}

			groupResource, err := schema.ParseGroupVersion(crd.GroupVersion)
			if err != nil {
				// Ignore errors: this function is for diagnostic purposes only.
				continue
			}

			group := apiResource.Group
			if group == "" {
				group = groupResource.Group
			}

			version := apiResource.Version
			if version == "" {
				version = groupResource.Version
			}

			gvr := schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: apiResource.Name,
			}

			unstructuredList, err := dynamicInterface.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				// Ignore errors: this function is for diagnostic purposes only.
				continue
			}
			if len(unstructuredList.Items) > 0 {
				resourceList += "( " + name + ": "
				for _, unstructuredItem := range unstructuredList.Items {
					resourceList += unstructuredItem.GetName() + " "
				}
				resourceList += ")\n"
			}

		}

	}

	return resourceList

}

// CreateTestNamespace creates a namespace where Application and Component CR will be created
func (s *SuiteController) CreateTestNamespace(name string) (*corev1.Namespace, error) {

	// Check if the E2E test namespace already exists
	ns, err := s.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Create the E2E test namespace if it doesn't exist
			nsTemplate := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{constants.ArgoCDLabelKey: constants.ArgoCDLabelValue},
				}}
			ns, err = s.KubeInterface().CoreV1().Namespaces().Create(context.TODO(), &nsTemplate, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when creating %s namespace: %v", name, err)
			}
		} else {
			return nil, fmt.Errorf("error when getting the '%s' namespace: %v", name, err)
		}
	} else {
		// Check whether the test namespace contains correct label
		if val, ok := ns.Labels[constants.ArgoCDLabelKey]; ok && val == constants.ArgoCDLabelValue {
			return ns, nil
		}
		// Update test namespace labels in case they are missing argoCD label
		ns.Labels[constants.ArgoCDLabelKey] = constants.ArgoCDLabelValue
		ns, err = s.KubeInterface().CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("error when updating labels in '%s' namespace: %v", name, err)
		}
	}

	// "pipeline" service account needs to be present in the namespace before we start with creating tekton resources
	// TODO: STONE-442 - decrease the timeout here back to 30 seconds once this issue is resolved.
	if err := utils.WaitUntil(s.ServiceaccountPresent("pipeline", name), time.Second*60); err != nil {
		return nil, fmt.Errorf("'pipeline' service account wasn't created in %s namespace: %+v", name, err)
	}

	// Argo CD role/rolebinding need to be present in the namespace before we create GitOpsDeployments.
	// - These role bindings are created in namespaces labeled with 'argocd.argoproj.io/managed-by' (see above)
	if err := utils.WaitUntil(s.argoCDNamespaceRBACPresent(name), time.Second*120); err != nil {
		return nil, fmt.Errorf("argo CD Namespace RBAC was never present in '%s': %v", name, err)
	}

	return ns, nil
}

func (s *SuiteController) ServiceaccountPresent(saName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := s.GetServiceAccount(saName, namespace)
		if err != nil {
			return false, nil
		}
		return true, nil
	}
}

// argoCDNamespaceRBACPresent returns a condition which waits for the Argo CD role/rolebindings to be set on the namespace.
//   - This Role/RoleBinding allows Argo cd to deploy into the namespace (which is referred to as 'managing the namespace'), and
//     is created by the GitOps Operator.
func (s *SuiteController) argoCDNamespaceRBACPresent(namespace string) wait.ConditionFunc {
	return func() (bool, error) {

		roles, err := s.ListRoles(namespace)
		if err != nil || roles == nil {
			return false, nil
		}

		// The namespace should contain a 'gitops-service-argocd-' Role
		roleFound := false
		for _, role := range roles.Items {
			if strings.HasPrefix(role.Name, constants.ArgoCDLabelValue+"-") {
				roleFound = true
			}
		}
		if !roleFound {
			return false, nil
		}

		// The namespace should contain a 'gitops-service-argocd-' RoleBinding
		roleBindingFound := false
		roleBindings, err := s.ListRoleBindings(namespace)
		if err != nil || roleBindings == nil {
			return false, nil
		}
		for _, roleBinding := range roleBindings.Items {
			if strings.HasPrefix(roleBinding.Name, constants.ArgoCDLabelValue+"-") {
				roleBindingFound = true
			}
		}

		return roleBindingFound, nil
	}
}

// namespaceDoesNotExist returns a condition that can be used to wait for the namespace to not exist
func (s *SuiteController) namespaceDoesNotExist(namespace string) wait.ConditionFunc {
	return func() (bool, error) {

		_, err := s.KubeInterface().CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})

		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}

func (s *SuiteController) ApplicationGitopsRepoExists(devfileContent string) wait.ConditionFunc {
	return func() (bool, error) {
		gitOpsRepoURL := utils.ObtainGitOpsRepositoryName(devfileContent)
		return s.Github.CheckIfRepositoryExist(gitOpsRepoURL), nil
	}
}

// CreateServiceAccount creates a service account with the provided name and namespace using the given list of secrets.
func (s *SuiteController) CreateServiceAccount(name, namespace string, serviceAccountSecretList []corev1.ObjectReference) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Secrets: serviceAccountSecretList,
	}
	return s.KubeInterface().CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
}

// CreateRole creates a role with the provided name and namespace using the given list of rules
func (s *SuiteController) CreateRole(roleName, namespace string, roleRules map[string][]string) (*rbacv1.Role, error) {

	rules := &rbacv1.PolicyRule{
		APIGroups: roleRules["apiGroupsList"],
		Resources: roleRules["roleResources"],
		Verbs:     roleRules["roleVerbs"],
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			*rules,
		},
	}
	createdRole, err := s.KubeInterface().RbacV1().Roles(namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdRole, nil
}

// CreateRoleBinding creates an object of Role Binding in namespace with service account provided and role reference api group.
func (s *SuiteController) CreateRoleBinding(roleBindingName, namespace, subjectKind, serviceAccountName, roleRefKind, roleRefName, roleRefApiGroup string) (*rbacv1.RoleBinding, error) {

	roleBindingSubjects := []rbacv1.Subject{
		{
			Kind:      subjectKind,
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	roleBindingRoleRef := rbacv1.RoleRef{
		Kind:     roleRefKind,
		Name:     roleRefName,
		APIGroup: roleRefApiGroup,
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: namespace,
		},
		Subjects: roleBindingSubjects,
		RoleRef:  roleBindingRoleRef,
	}

	createdRoleBinding, err := s.KubeInterface().RbacV1().RoleBindings(namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdRoleBinding, nil
}
