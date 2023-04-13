package common

import (
	"context"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (s *SuiteController) ListRoles(namespace string) (*rbacv1.RoleList, error) {
	listOptions := metav1.ListOptions{}
	return s.KubeInterface().RbacV1().Roles(namespace).List(context.TODO(), listOptions)
}

func (s *SuiteController) ListRoleBindings(namespace string) (*rbacv1.RoleBindingList, error) {

	listOptions := metav1.ListOptions{}
	return s.KubeInterface().RbacV1().RoleBindings(namespace).List(context.TODO(), listOptions)
}

func (s *SuiteController) GetRole(roleName, namespace string) (*rbacv1.Role, error) {
	return s.KubeInterface().RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
}

func (s *SuiteController) GetRoleBinding(rolebindingName, namespace string) (*rbacv1.RoleBinding, error) {
	return s.KubeInterface().RbacV1().RoleBindings(namespace).Get(context.TODO(), rolebindingName, metav1.GetOptions{})
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
