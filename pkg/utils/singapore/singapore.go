package singapore

import (
	"context"
	"crypto/md5" // nolint:gosec
	"encoding/hex"
	"os/exec"

	"github.com/google/uuid"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	singaporev1alpha1 "github.com/stolostron/cluster-registration-operator/api/singapore/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"

	ocmv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

//
func (h *SuiteController) CreateRegisteredClusterCR(name string, namespace string) error {
	registeredClusterCR := singaporev1alpha1.RegisteredCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: singaporev1alpha1.RegisteredClusterSpec{},
	}

	err := h.KubeRest().Create(context.TODO(), &registeredClusterCR)
	if err != nil {
		return err
	}
	return nil
}

func (h *SuiteController) GetRegisteredClusterCR(name string, namespace string) (*singaporev1alpha1.RegisteredCluster, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	registeredClusterCR := &singaporev1alpha1.RegisteredCluster{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, registeredClusterCR)
	if err != nil {
		return &singaporev1alpha1.RegisteredCluster{}, err
	}
	return registeredClusterCR, nil
}

func (h *SuiteController) CheckIfRegisteredClusterHasJoined(name string, namespace string) bool {
	importedCluster, err := h.GetRegisteredClusterCR(name, namespace)
	if err != nil {
		return false
	}
	for _, condition := range importedCluster.Status.Conditions {
		if condition.Type == "ManagedClusterJoined" && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false

}

func (h *SuiteController) DeleteRegisteredClusterCR(name string, namespace string) error {

	registeredClusterCR := singaporev1alpha1.RegisteredCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: singaporev1alpha1.RegisteredClusterSpec{},
	}

	return h.KubeRest().Delete(context.TODO(), &registeredClusterCR)
}

func (h *SuiteController) CreateAppstudioSandboxWorkspaceUserSignUp(username string, namespace string) error {

	userEmail := username + "@example.com"
	md5hash := md5.New() // nolint:gosec
	_, _ = md5hash.Write([]byte(userEmail))
	emailHash := hex.EncodeToString(md5hash.Sum(nil))

	userSignup := &toolchainv1alpha1.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: namespace,
			Annotations: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailAnnotationKey:           userEmail,
				toolchainv1alpha1.UserSignupVerificationCounterAnnotationKey: "0",
			},
			Labels: map[string]string{
				toolchainv1alpha1.UserSignupUserEmailHashLabelKey: emailHash,
			},
		},
		Spec: toolchainv1alpha1.UserSignupSpec{
			TargetCluster: "",
			Userid:        uuid.New().String(),
			Username:      username,
		},
	}

	err := h.KubeRest().Create(context.TODO(), userSignup)
	if err != nil {
		return err
	}
	return nil
}

func (h *SuiteController) DeleteAppstudioSandboxWorkspaceUserSignUp(username string, namespace string) error {

	userSignup := &toolchainv1alpha1.UserSignup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: namespace,
		},
	}

	return h.KubeRest().Delete(context.TODO(), userSignup)
}

//
func (h *SuiteController) CheckIfAppstudioSpaceExists(name string) bool {
	space := &toolchainv1alpha1.SpaceList{}
	if err := h.KubeRest().List(context.TODO(), space, &rclient.ListOptions{}); err != nil {
		return false
	}
	for _, sp := range space.Items {
		if sp.Name == name {
			return true
		}
	}
	return false
}

//
func (h *SuiteController) CheckIfClusterSetExists(name string) bool {
	clusterSet := &ocmv1alpha1.ManagedClusterSetList{}
	if err := h.KubeRest().List(context.TODO(), clusterSet, &rclient.ListOptions{}); err != nil {
		return false
	}
	for _, cs := range clusterSet.Items {
		if cs.Name == name {
			return true
		}
	}
	return false
}

//
func (h *SuiteController) ExecuteImportCommand(AppstudioImpoterdClusterKubeconfigEnv string, importCommand string) error {
	// TDB
	// Should this run inside a pod by itself?

	cmd, err := exec.Command("bash", "-c", "KUBECONFIG=$"+AppstudioImpoterdClusterKubeconfigEnv+" && "+importCommand).Output()
	if err != nil {
		return err
	}
	klog.Infof("Importing cluster into Appstudio: %s", string(cmd))
	return nil
}
