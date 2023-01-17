package tekton

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	ecp "github.com/hacbs-contract/enterprise-contract-controller/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	g "github.com/onsi/ginkgo/v2"
)

type KubeController struct {
	Commonctrl common.SuiteController
	Tektonctrl SuiteController
	Namespace  string
}

type Bundles struct {
	BuildTemplatesBundle string
	HACBSTemplatesBundle string
}

func newBundles(client kubernetes.Interface) (*Bundles, error) {
	buildPipelineDefaults, err := client.CoreV1().ConfigMaps("build-templates").Get(context.TODO(), "build-pipelines-defaults", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	bundle := buildPipelineDefaults.Data["default_build_bundle"]

	r := regexp.MustCompile(`([/:])(?:build|base)-`)

	return &Bundles{
		BuildTemplatesBundle: bundle,
		HACBSTemplatesBundle: r.ReplaceAllString(bundle, "${1}hacbs-"),
	}, nil
}

// Create the struct for kubernetes clients
type SuiteController struct {
	*kubeCl.K8sClient

	Bundles Bundles
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

	bundles, err := newBundles(kube.KubeInterface())
	if err != nil {
		return nil, err
	}

	return &SuiteController{
		kube,
		*bundles,
	}, nil
}

func (s *SuiteController) GetPipelineRun(pipelineRunName, namespace string) (*v1beta1.PipelineRun, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
}

func (s *SuiteController) fetchContainerLog(podName, containerName, namespace string) (string, error) {
	podClient := s.K8sClient.KubeInterface().CoreV1().Pods(namespace)
	req := podClient.GetLogs(podName, &corev1.PodLogOptions{Container: containerName})
	readCloser, err := req.Stream(context.TODO())
	log := ""
	if err != nil {
		return log, err
	}
	defer readCloser.Close()
	b, err := io.ReadAll(readCloser)
	if err != nil {
		return log, err
	}
	return string(b[:]), nil
}

func (s *SuiteController) GetPipelineRunLogs(pipelineRunName, namespace string) (string, error) {
	podClient := s.K8sClient.KubeInterface().CoreV1().Pods(namespace)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	podLog := ""
	for _, pod := range podList.Items {
		if !strings.HasPrefix(pod.Name, pipelineRunName) {
			continue
		}
		for _, c := range pod.Spec.InitContainers {
			var err error
			var cLog string
			cLog, err = s.fetchContainerLog(pod.Name, c.Name, namespace)
			podLog = podLog + fmt.Sprintf("\ninit container %s: \n", c.Name) + cLog
			if err != nil {
				return podLog, err
			}
		}
		for _, c := range pod.Spec.Containers {
			var err error
			var cLog string
			cLog, err = s.fetchContainerLog(pod.Name, c.Name, namespace)
			podLog = podLog + fmt.Sprintf("\ncontainer %s: \n", c.Name) + cLog
			if err != nil {
				return podLog, err
			}
		}
	}
	return podLog, nil
}

func (s *SuiteController) CheckPipelineRunStarted(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := s.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, nil
		}
		if pr.Status.StartTime != nil {
			return true, nil
		}
		return false, nil
	}
}

func (s *SuiteController) CheckPipelineRunFinished(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := s.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, nil
		}
		if pr.Status.CompletionTime != nil {
			return true, nil
		}
		return false, nil
	}
}

// Create a tekton task and return the task or error
func (s *SuiteController) CreateTask(task *v1beta1.Task, ns string) (*v1beta1.Task, error) {
	return s.PipelineClient().TektonV1beta1().Tasks(ns).Create(context.TODO(), task, metav1.CreateOptions{})
}

func (s *SuiteController) DeleteTask(name, ns string) error {
	return s.PipelineClient().TektonV1beta1().Tasks(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// Create a tekton pipelineRun and return the pipelineRun or error
func (s *SuiteController) CreatePipelineRun(pipelineRun *v1beta1.PipelineRun, ns string) (*v1beta1.PipelineRun, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(ns).Create(context.TODO(), pipelineRun, metav1.CreateOptions{})
}

func (s *SuiteController) DeletePipelineRun(name, ns string) error {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// Create a tekton pipeline and return the pipeline or error
func (s *SuiteController) CreatePipeline(pipeline *v1beta1.Pipeline, ns string) (*v1beta1.Pipeline, error) {
	return s.PipelineClient().TektonV1beta1().Pipelines(ns).Create(context.TODO(), pipeline, metav1.CreateOptions{})
}

func (s *SuiteController) DeletePipeline(name, ns string) error {
	return s.PipelineClient().TektonV1beta1().Pipelines(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (s *SuiteController) ListTaskRuns(ns string, labelKey string, labelValue string, selectorLimit int64) (*v1beta1.TaskRunList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         selectorLimit,
	}
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).List(context.TODO(), listOptions)
}

func (s *SuiteController) ListAllTaskRuns(ns string) (*v1beta1.TaskRunList, error) {
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) ListAllPipelineRuns(ns string) (*v1beta1.PipelineRunList, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(ns).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) DeleteTaskRun(name, ns string) error {
	return s.PipelineClient().TektonV1beta1().TaskRuns(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (k KubeController) WatchPipelineRun(pipelineRunName string, taskTimeout int) error {
	g.GinkgoWriter.Printf("Waiting for pipeline %q to finish\n", pipelineRunName)
	return utils.WaitUntil(k.Tektonctrl.CheckPipelineRunFinished(pipelineRunName, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (k KubeController) GetTaskRunResult(pr *v1beta1.PipelineRun, pipelineTaskName string, result string) (string, error) {
	for _, tr := range pr.Status.TaskRuns {
		if tr.PipelineTaskName != pipelineTaskName {
			continue
		}

		for _, trResult := range tr.Status.TaskRunResults {
			if trResult.Name == result {
				// for some reason the result might contain \n suffix
				return strings.TrimSuffix(trResult.Value.StringVal, "\n"), nil
			}
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRuns of PipelineRun %s/%s", result, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

func (k KubeController) GetTaskRunStatus(pr *v1beta1.PipelineRun, pipelineTaskName string) (*v1beta1.PipelineRunTaskRunStatus, error) {
	for _, tr := range pr.Status.TaskRuns {
		if tr.PipelineTaskName == pipelineTaskName {
			return tr, nil
		}
	}
	return nil, fmt.Errorf(
		"TaskRun status for pipeline task name %q not found in the status of PipelineRun %s/%s", pipelineTaskName, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

func (k KubeController) RunPipeline(g PipelineRunGenerator, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pr := g.Generate()
	pvcs := k.Commonctrl.KubeInterface().CoreV1().PersistentVolumeClaims(pr.Namespace)
	for _, w := range pr.Spec.Workspaces {
		if w.PersistentVolumeClaim != nil {
			pvcName := w.PersistentVolumeClaim.ClaimName
			if _, err := pvcs.Get(context.TODO(), pvcName, metav1.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					err := createPVC(pvcs, pvcName)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}

	return k.createAndWait(pr, taskTimeout)
}

// DeleteAllPipelineRunsInASpecificNamespace deletes all PipelineRuns in a given namespace (removing the finalizers field, first)
func (s *SuiteController) DeleteAllPipelineRunsInASpecificNamespace(ns string) error {

	pipelineRunList, err := s.ListAllPipelineRuns(ns)
	if err != nil || pipelineRunList == nil {
		return fmt.Errorf("unable to delete all PipelineRuns in '%s': %v", ns, err)
	}

	for _, pipelineRun := range pipelineRunList.Items {
		err := wait.PollImmediate(time.Second, 30*time.Second, func() (done bool, err error) {
			pipelineRunCR := v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: ns,
				},
			}
			if err := s.K8sClient.KubeRest().Get(context.TODO(), crclient.ObjectKeyFromObject(&pipelineRunCR), &pipelineRunCR); err != nil {
				if errors.IsNotFound(err) {
					// PipelinerRun CR is already removed
					return true, nil
				}
				g.GinkgoWriter.Printf("unable to retrieve PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil

			}

			// Remove the finalizer, so that it can be deleted.
			pipelineRunCR.Finalizers = []string{}
			if err := s.K8sClient.KubeRest().Update(context.TODO(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to remove finalizers from PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}

			if err := s.K8sClient.KubeRest().Delete(context.TODO(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to delete PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("deletion of PipelineRun '%s' in '%s' timed out", pipelineRun.Name, ns)
		}

	}

	return nil
}

func createPVC(pvcs v1.PersistentVolumeClaimInterface, pvcName string) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	if _, err := pvcs.Create(context.TODO(), pvc, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
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

func (k KubeController) createAndWait(pr *v1beta1.PipelineRun, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pipelineRun, err := k.Tektonctrl.CreatePipelineRun(pr, k.Namespace)
	if err != nil {
		return nil, err
	}
	g.GinkgoWriter.Printf("Creating Pipeline %q\n", pipelineRun.Name)
	return pipelineRun, utils.WaitUntil(k.Tektonctrl.CheckPipelineRunStarted(pipelineRun.Name, k.Namespace), time.Duration(taskTimeout)*time.Second)
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

	// Cosign creates tags for attestation and signature based on the image digest. Compute
	// the expected prefix for later usage: sha256:abcd... -> sha256-abcd...
	// Also, this prefix is really the prefix of the ImageStreamTag resource which follows the
	// format: <image stream name>:<tag-name>
	cosignImagePrefix := fmt.Sprintf("%s:%s", imageStreamName, strings.Replace(imageDigest, ":", "-", 1))

	results := CosignResult{}

	if signatureTag, err := findTagWithName(client, namespace, cosignImagePrefix+".sig"); err == nil {
		results.signatureImageRef = signatureTag.GetName()
	}

	if attestationTag, err := findTagWithName(client, namespace, cosignImagePrefix+".att"); err == nil {
		// we want two layers, one for TaskRun and one for PipelineRun
		// attestations, i.e. that the Chains controller reconcilled both and
		// uploaded them as layers
		img, ok := attestationTag.Object["image"]
		if ok {
			img, ok = img.(map[string]interface{})
		}
		var layers []interface{}
		if ok {
			layers, ok = img.(map[string]interface{})["dockerImageLayers"].([]interface{})
		}
		// this needs to change if/when Chains controller does not produce two layers
		if ok && len(layers) == 2 {
			results.attestationImageRef = attestationTag.GetName()
		}
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

func findTagWithName(client crclient.Client, namespace, name string) (*unstructured.Unstructured, error) {
	tag := unstructured.Unstructured{}
	tag.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "image.openshift.io",
		Kind:    "ImageStreamTag",
		Version: "v1",
	})
	tag.SetName(name)
	tag.SetNamespace(namespace)
	if err := client.Get(context.TODO(), crclient.ObjectKeyFromObject(&tag), &tag); err != nil {
		return nil, err
	}

	return &tag, nil
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

func (k KubeController) CreateOrUpdatePolicyConfiguration(namespace string, policy ecp.EnterpriseContractPolicySpec) error {
	ecPolicy := ecp.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ec-policy",
			Namespace: namespace,
		},
	}

	// fetch to see if it exists
	err := k.Tektonctrl.K8sClient.KubeRest().Get(context.TODO(), crclient.ObjectKey{
		Namespace: namespace,
		Name:      "ec-policy",
	}, &ecPolicy)

	exists := true
	if err != nil {
		if errors.IsNotFound(err) {
			exists = false
		} else {
			return err
		}
	}

	ecPolicy.Spec = policy
	if !exists {
		// it doesn't, so create
		if err := k.Tektonctrl.K8sClient.KubeRest().Create(context.TODO(), &ecPolicy); err != nil {
			return err
		}
	} else {
		// it does, so update
		if err := k.Tektonctrl.K8sClient.KubeRest().Update(context.TODO(), &ecPolicy); err != nil {
			return err
		}
	}

	return nil
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

// CreateEnterpriseContractPolicy creates an EnterpriseContractPolicy.
func (s *SuiteController) CreateEnterpriseContractPolicy(name, namespace string, ecpolicy ecp.EnterpriseContractPolicySpec) (*ecp.EnterpriseContractPolicy, error) {
	ec := &ecp.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ecpolicy,
	}
	return ec, s.K8sClient.KubeRest().Create(context.TODO(), ec)
}
