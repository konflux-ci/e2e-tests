package tekton

import (
	"context"
	"fmt"
	"strings"
	"time"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type KubeController struct {
	Commonctrl common.SuiteController
	Tektonctrl SuiteController
	Namespace  string
}

// Create the struct for kubernetes clients
type SuiteController struct {
	*kubeCl.K8sClient
}

type CosignResult struct {
	signatureImageRef   string
	attestationImageRef string
}

func (c CosignResult) IsPresent() bool {
	return c.signatureImageRef != "" && c.attestationImageRef != ""
}

func (c CosignResult) Missing(prefix string) string {
	var ret []string = make([]string, 0, 2)
	if c.signatureImageRef == "" {
		ret = append(ret, prefix+".sig")
	}

	if c.attestationImageRef == "" {
		ret = append(ret, prefix+".att")
	}

	return strings.Join(ret, " and ")
}

// Create controller for Application/Component crud operations
func NewSuiteController(kube *kubeCl.K8sClient) (*SuiteController, error) {

	return &SuiteController{
		kube,
	}, nil
}

func (s *SuiteController) GetTaskRun(taskName, namespace string) (*v1beta1.TaskRun, error) {
	return s.PipelineClient().TektonV1beta1().TaskRuns(namespace).Get(context.TODO(), taskName, metav1.GetOptions{})
}

func (s *SuiteController) CheckTaskPodExists(taskName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		tr, err := s.GetTaskRun(taskName, namespace)
		if err != nil {
			return false, nil
		}
		if tr.Status.PodName != "" {
			return true, nil
		}
		return false, nil
	}
}

// Create a tekton task and return the task or error
func (s *SuiteController) CreateTask(task *v1beta1.Task, ns string) (*v1beta1.Task, error) {
	return s.PipelineClient().TektonV1beta1().Tasks(ns).Create(context.TODO(), task, metav1.CreateOptions{})
}

// Create a tekton taskRun and return the taskRun or error
func (s *SuiteController) CreateTaskRun(taskRun *v1beta1.TaskRun, ns string) (*v1beta1.TaskRun, error) {
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).Create(context.TODO(), taskRun, metav1.CreateOptions{})
}

func (s *SuiteController) ListTaskRuns(ns string, labelKey string, labelValue string, selectorLimit int64) (*v1beta1.TaskRunList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         selectorLimit,
	}
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).List(context.TODO(), listOptions)
}

func (k KubeController) RunBuildahDemoTask(image string, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := buildahDemoTaskRun(image)
	return k.createAndWait(tr, taskTimeout)
}

func (k KubeController) WatchTaskPod(tr string, taskTimeout int) error {
	trUpdated, err := k.Tektonctrl.GetTaskRun(tr, k.Namespace)
	if err != nil {
		return err
	}
	pod, err := k.Commonctrl.GetPod(k.Namespace, trUpdated.Status.PodName)
	if err != nil {
		return err
	}
	return k.Commonctrl.WaitForPod(k.Commonctrl.IsPodSuccessful(pod.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (k KubeController) GetTaskRunResult(tr *v1beta1.TaskRun, result string) (string, error) {
	for _, trResult := range tr.Status.TaskRunResults {
		if trResult.Name == result {
			return trResult.Value, nil
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRun %s/%s", result, tr.ObjectMeta.Namespace, tr.ObjectMeta.Name)
}

func (k KubeController) RunVerifyTask(taskName, image string, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := verifyTaskRun(image, taskName)
	return k.createAndWait(tr, taskTimeout)
}

type VerifyECTaskParams struct {
	TaskName     string
	ImageRef     string
	PublicSecret string
	PipelineName string
	RekorHost    string
	SslCertDir   string
	StrictPolicy string
}

func (k KubeController) RunVerifyECTask(params VerifyECTaskParams, taskTimeout int) (*v1beta1.TaskRun, error) {
	tr := verifyEnterpriseContractTaskRun(params)
	return k.createAndWait(tr, taskTimeout)
}

func (k KubeController) AwaitAttestationAndSignature(image string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		if _, err := k.FindCosignResultsForImage(image); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}

			return true, err
		}

		return true, nil
	})
}

func (k KubeController) createAndWait(tr *v1beta1.TaskRun, taskTimeout int) (*v1beta1.TaskRun, error) {
	taskRun, err := k.Tektonctrl.CreateTaskRun(tr, k.Namespace)
	if err != nil {
		return nil, err
	}
	return taskRun, k.Commonctrl.WaitForPod(k.Tektonctrl.CheckTaskPodExists(taskRun.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

// FindCosignResultsForImage looks for .sig and .att image tags in the OpenShift image stream for the provided image reference.
// If none can be found errors.IsNotFound(err) is true, when err is nil CosignResult contains image references for signature and attestation images, otherwise other errors could be returned.
func (k KubeController) FindCosignResultsForImage(imageRef string) (*CosignResult, error) {
	return findCosignResultsForImage(imageRef, k.Commonctrl.KubeRest())
}

func findCosignResultsForImage(imageRef string, client crclient.Client) (*CosignResult, error) {
	imageInfo := strings.Split(imageRef, "/")
	namespace := imageInfo[1]
	// When using the integrated OpenShift registry, the name of the repository corresponds to
	// an ImageStream resource of the same name. We use this name to easily find the tags later.
	imageNameInfo := strings.Split(imageInfo[2], "@")
	imageStreamName, imageDigest := imageNameInfo[0], imageNameInfo[1]

	tags := &unstructured.UnstructuredList{}
	tags.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Kind:    "ImageStreamTag",
		Version: "v1",
	})

	if err := client.List(context.TODO(), tags, &crclient.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	// Cosign creates tags for attestation and signature based on the image digest. Compute
	// the expected prefix for later usage: sha256:abcd... -> sha256-abcd...
	// Also, this prefix is really the prefix of the ImageStreamTag resource which follows the
	// format: <image stream name>:<tag-name>
	cosignImagePrefix := fmt.Sprintf("%s:%s", imageStreamName, strings.Replace(imageDigest, ":", "-", 1))

	results := CosignResult{}

	if signatureTag := findTagWithName(tags, cosignImagePrefix+".sig"); signatureTag != nil {
		results.signatureImageRef = signatureTag.GetName()
	}

	if attestationTag := findTagWithName(tags, cosignImagePrefix+".att"); attestationTag != nil {
		results.attestationImageRef = attestationTag.GetName()
	}

	// we found both
	if results.IsPresent() {
		return &results, nil
	}

	return nil, errors.NewNotFound(schema.GroupResource{
		Group:    "image.openshift.io",
		Resource: "ImageStreamTag",
	}, results.Missing(cosignImagePrefix))
}

func findTagWithName(tags *unstructured.UnstructuredList, name string) *unstructured.Unstructured {
	for _, tag := range tags.Items {
		if tag.GetName() == name {
			return &tag
		}
	}

	return nil
}

func (k KubeController) CreateOrUpdateSigningSecret(publicKey []byte, name, namespace string) (err error) {
	api := k.Tektonctrl.K8sClient.KubeInterface().CoreV1().Secrets(namespace)
	ctx := context.TODO()

	expectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       map[string][]byte{"cosign.pub": publicKey},
	}

	s, err := api.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return
		}
		if _, err = api.Create(ctx, expectedSecret, metav1.CreateOptions{}); err != nil {
			return
		}
	} else {
		if string(s.Data["cosign.pub"]) != string(publicKey) {
			if _, err = api.Update(ctx, expectedSecret, metav1.UpdateOptions{}); err != nil {
				return
			}
		}
	}
	return
}

func (k KubeController) GetPublicKey(name, namespace string) (publicKey []byte, err error) {
	api := k.Tektonctrl.K8sClient.KubeInterface().CoreV1().Secrets(namespace)
	ctx := context.TODO()

	secret, err := api.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return
	}

	publicKey = secret.Data["cosign.pub"]
	return
}

func (k KubeController) CreateOrUpdateConfigPolicy(namespace string, policy string) (err error) {
	api := k.Tektonctrl.K8sClient.KubeInterface().CoreV1().ConfigMaps(namespace)
	ctx := context.TODO()

	configPolicyName := "ec-policy"
	expectedConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configPolicyName},
		Data:       map[string]string{"policy.json": policy},
	}

	cm, err := api.Get(ctx, configPolicyName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return
		}
		if _, err = api.Create(ctx, expectedConfigMap, metav1.CreateOptions{}); err != nil {
			return
		}
	} else {
		if cm.Data["policy.json"] != policy {
			if _, err = api.Update(ctx, expectedConfigMap, metav1.UpdateOptions{}); err != nil {
				return
			}
		}
	}
	return
}

func (k KubeController) GetRekorHost() (rekorHost string, err error) {
	api := k.Tektonctrl.K8sClient.KubeInterface().CoreV1().ConfigMaps("tekton-chains")
	ctx := context.TODO()

	cm, err := api.Get(ctx, "chains-config", metav1.GetOptions{})
	if err != nil {
		return
	}

	rekorHost, ok := cm.Data["transparency.url"]
	if !ok || rekorHost == "" {
		rekorHost = "https://rekor.sigstore.dev"
	}
	return
}
