package common

import (
	"context"
	"fmt"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// DeleteNamespace deletes the give namespace.
func (s *SuiteController) DeleteNamespace(namespace string) error {
	_, err := s.KubeInterface().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("could not check for namespace '%s' existence: %v", namespace, err)
	}

	if err := s.KubeInterface().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{}); err != nil {
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
	ns, err := s.KubeInterface().CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			// Create the E2E test namespace if it doesn't exist
			nsTemplate := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{constants.ArgoCDLabelKey: constants.ArgoCDLabelValue},
				}}
			ns, err = s.KubeInterface().CoreV1().Namespaces().Create(context.Background(), &nsTemplate, metav1.CreateOptions{})
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
		ns, err = s.KubeInterface().CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("error when updating labels in '%s' namespace: %v", name, err)
		}
	}

	// Create ServiceAccount which is used by Pipelines but created by Toolchain host operator
	_, err = s.KubeInterface().CoreV1().ServiceAccounts(name).Get(context.Background(), constants.DefaultPipelineServiceAccount, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			saTemplate := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.DefaultPipelineServiceAccount,
				},
			}
			_, err = s.KubeInterface().CoreV1().ServiceAccounts(name).Create(context.Background(), &saTemplate, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when creating %s serviceaccount: %v", constants.DefaultPipelineServiceAccount, err)
			}
		} else {
			return nil, fmt.Errorf("error when getting the '%s' serviceaccount: %v", constants.DefaultPipelineServiceAccount, err)
		}
	}

	_, err = s.KubeInterface().RbacV1().RoleBindings(name).Get(context.Background(), constants.DefaultPipelineServiceAccountRoleBinding, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			roleBindingTemplate := rbacv1.RoleBinding{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{Name: constants.DefaultPipelineServiceAccountRoleBinding},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      constants.DefaultPipelineServiceAccount,
						Namespace: name,
					},
				},
				RoleRef: rbacv1.RoleRef{
					Kind: "ClusterRole",
					Name: constants.DefaultPipelineServiceAccountClusterRole,
				},
			}
			_, err = s.KubeInterface().RbacV1().RoleBindings(name).Create(context.Background(), &roleBindingTemplate, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when creating %s roleBinding: %v", constants.DefaultPipelineServiceAccountRoleBinding, err)
			}
		} else {
			return nil, fmt.Errorf("error when getting the '%s' roleBinding: %v", constants.DefaultPipelineServiceAccountRoleBinding, err)
		}
	}

	// Argo CD role/rolebinding need to be present in the namespace before we create GitOpsDeployments.
	// - These role bindings are created in namespaces labeled with 'argocd.argoproj.io/managed-by' (see above)
	if err := utils.WaitUntil(s.argoCDNamespaceRBACPresent(name), time.Second*120); err != nil {
		return nil, fmt.Errorf("argo CD Namespace RBAC was never present in '%s': %v", name, err)
	}

	return ns, nil
}

// namespaceDoesNotExist returns a condition that can be used to wait for the namespace to not exist
func (s *SuiteController) namespaceDoesNotExist(namespace string) wait.ConditionFunc {
	return func() (bool, error) {

		_, err := s.KubeInterface().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})

		return err != nil && k8sErrors.IsNotFound(err), nil
	}
}

// GetNamespace returns the requested Namespace object
func (s *SuiteController) GetNamespace(namespace string) (*corev1.Namespace, error) {
	return s.KubeInterface().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
}
