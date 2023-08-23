package tekton

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	g "github.com/onsi/ginkgo/v2"
)

const quayBaseUrl = "https://quay.io/api/v1"

type KubeController struct {
	Commonctrl common.SuiteController
	Tektonctrl SuiteController
	Namespace  string
}

type Bundles struct {
	FBCBuilderBundle    string
	DockerBuildBundle   string
	JavaBuilderBundle   string
	NodeJSBuilderBundle string
}

type QuayImageInfo struct {
	ImageRef string
	Layers   []any
}

type TagResponse struct {
	Tags []Tag `json:"tags"`
}
type Tag struct {
	Digest string `json:"manifest_digest"`
}
type ManifestResponse struct {
	Layers []any `json:"layers"`
}

// Create the struct for kubernetes clients
type SuiteController struct {
	*kubeCl.CustomClient
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

// Create controller for Tekton Task/Pipeline CRUD operations
func NewSuiteController(kube *kubeCl.CustomClient) *SuiteController {
	return &SuiteController{kube}
}

func (s *SuiteController) NewBundles() (*Bundles, error) {
	namespacedName := types.NamespacedName{
		Name:      "build-pipeline-selector",
		Namespace: "build-service",
	}
	bundles := &Bundles{}
	pipelineSelector := &buildservice.BuildPipelineSelector{}
	err := s.KubeRest().Get(context.TODO(), namespacedName, pipelineSelector)
	if err != nil {
		return nil, err
	}
	for _, selector := range pipelineSelector.Spec.Selectors {
		bundleName := selector.PipelineRef.Name
		bundleRef := selector.PipelineRef.Bundle //nolint:all
		switch bundleName {
		case "docker-build":
			bundles.DockerBuildBundle = bundleRef
		case "fbc-builder":
			bundles.FBCBuilderBundle = bundleRef
		case "java-builder":
			bundles.JavaBuilderBundle = bundleRef
		case "nodejs-builder":
			bundles.NodeJSBuilderBundle = bundleRef
		}
	}
	return bundles, nil
}

func (s *SuiteController) GetPipelineRun(pipelineRunName, namespace string) (*v1beta1.PipelineRun, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
}

func (s *SuiteController) WatchPipelineRun(ctx context.Context, namespace string) (watch.Interface, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).Watch(ctx, metav1.ListOptions{})
}

func (s *SuiteController) fetchContainerLog(podName, containerName, namespace string) (string, error) {
	podClient := s.KubeInterface().CoreV1().Pods(namespace)
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
	podClient := s.KubeInterface().CoreV1().Pods(namespace)
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

func (s *SuiteController) GetTaskRunLogs(pipelineRunName, taskName, namespace string) (map[string]string, error) {
	tektonClient := s.PipelineClient().TektonV1beta1().PipelineRuns(namespace)
	pipelineRun, err := tektonClient.Get(context.TODO(), pipelineRunName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	podName := ""
	for _, childStatusReference := range pipelineRun.Status.ChildReferences {
		if childStatusReference.PipelineTaskName == taskName {
			taskRun := &v1beta1.TaskRun{}
			taskRunKey := types.NamespacedName{Namespace: pipelineRun.Namespace, Name: childStatusReference.Name}
			if err := s.KubeRest().Get(context.TODO(), taskRunKey, taskRun); err != nil {
				return nil, err
			}
			podName = taskRun.Status.PodName
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("task with %s name doesn't exist in %s pipelinerun", taskName, pipelineRunName)
	}

	podClient := s.KubeInterface().CoreV1().Pods(namespace)
	pod, err := podClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	logs := make(map[string]string)
	for _, container := range pod.Spec.Containers {
		containerName := container.Name
		if containerLogs, err := s.fetchContainerLog(podName, containerName, namespace); err == nil {
			logs[containerName] = containerLogs
		} else {
			logs[containerName] = "failed to get logs"
		}
	}
	return logs, nil
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

func (s *SuiteController) CheckPipelineRunSucceeded(pipelineRunName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pr, err := s.GetPipelineRun(pipelineRunName, namespace)
		if err != nil {
			return false, err
		}
		if len(pr.Status.Conditions) > 0 {
			for _, c := range pr.Status.Conditions {
				if c.Type == "Succeeded" && c.Status == "True" {
					return true, nil
				}
			}
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

func (k KubeController) WatchPipelineRunSucceeded(pipelineRunName string, taskTimeout int) error {
	g.GinkgoWriter.Printf("Waiting for pipeline %q to finish\n", pipelineRunName)
	return utils.WaitUntil(k.Tektonctrl.CheckPipelineRunSucceeded(pipelineRunName, k.Namespace), time.Duration(taskTimeout)*time.Second)
}

func (k KubeController) GetTaskRunFromPipelineRun(c crclient.Client, pr *v1beta1.PipelineRun, pipelineTaskName string) (*v1beta1.TaskRun, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName != pipelineTaskName {
			continue
		}

		taskRun := &v1beta1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
		if err := c.Get(context.TODO(), taskRunKey, taskRun); err != nil {
			return nil, err
		}
		return taskRun, nil
	}

	return nil, fmt.Errorf("task %q not found in PipelineRun %q/%q", pipelineTaskName, pr.Namespace, pr.Name)
}

func (k KubeController) GetTaskRunResult(c crclient.Client, pr *v1beta1.PipelineRun, pipelineTaskName string, result string) (string, error) {
	taskRun, err := k.GetTaskRunFromPipelineRun(c, pr, pipelineTaskName)
	if err != nil {
		return "", err
	}

	for _, trResult := range taskRun.Status.TaskRunResults {
		if trResult.Name == result {
			// for some reason the result might contain \n suffix
			return strings.TrimSuffix(trResult.Value.StringVal, "\n"), nil
		}
	}
	return "", fmt.Errorf(
		"result %q not found in TaskRuns of PipelineRun %s/%s", result, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

func (k KubeController) GetTaskRunStatus(c crclient.Client, pr *v1beta1.PipelineRun, pipelineTaskName string) (*v1beta1.PipelineRunTaskRunStatus, error) {
	for _, chr := range pr.Status.ChildReferences {
		if chr.PipelineTaskName == pipelineTaskName {
			taskRun := &v1beta1.TaskRun{}
			taskRunKey := types.NamespacedName{Namespace: pr.Namespace, Name: chr.Name}
			if err := c.Get(context.TODO(), taskRunKey, taskRun); err != nil {
				return nil, err
			}
			return &v1beta1.PipelineRunTaskRunStatus{PipelineTaskName: chr.PipelineTaskName, Status: &taskRun.Status}, nil
		}
	}
	return nil, fmt.Errorf(
		"TaskRun status for pipeline task name %q not found in the status of PipelineRun %s/%s", pipelineTaskName, pr.ObjectMeta.Namespace, pr.ObjectMeta.Name)
}

func (k KubeController) RunPipeline(g PipelineRunGenerator, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pr, err := g.Generate()
	if err != nil {
		return nil, err
	}
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
		err := wait.PollUntilContextTimeout(context.Background(), time.Second, 30*time.Second, true, func(ctx context.Context) (done bool, err error) {
			pipelineRunCR := v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pipelineRun.Name,
					Namespace: ns,
				},
			}
			if err := s.KubeRest().Get(context.TODO(), crclient.ObjectKeyFromObject(&pipelineRunCR), &pipelineRunCR); err != nil {
				if errors.IsNotFound(err) {
					// PipelinerRun CR is already removed
					return true, nil
				}
				g.GinkgoWriter.Printf("unable to retrieve PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil

			}

			// Remove the finalizer, so that it can be deleted.
			pipelineRunCR.Finalizers = []string{}
			if err := s.KubeRest().Update(context.TODO(), &pipelineRunCR); err != nil {
				g.GinkgoWriter.Printf("unable to remove finalizers from PipelineRun '%s' in '%s': %v\n", pipelineRunCR.Name, pipelineRunCR.Namespace, err)
				return false, nil
			}

			if err := s.KubeRest().Delete(context.TODO(), &pipelineRunCR); err != nil {
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
	return wait.PollUntilContextTimeout(context.Background(), time.Second, timeout, true, func(ctx context.Context) (done bool, err error) {
		if _, err := k.FindCosignResultsForImage(image); err != nil {
			g.GinkgoWriter.Printf("failed to get cosign result for image %s: %+v\n", image, err)
			return false, nil
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
	return findCosignResultsForImage(imageRef)
}

func findCosignResultsForImage(imageRef string) (*CosignResult, error) {
	var errMsg string
	// Split the image ref into image repo+tag (e.g quay.io/repo/name:tag), and image digest (sha256:abcd...)
	imageInfo := strings.Split(imageRef, "@")
	imageRegistryName := strings.Split(imageInfo[0], "/")[0]
	// imageRepoName is stripped from container registry name and a tag e.g. "quay.io/<org>/<repo>:tagprefix" => "<org>/<repo>"
	imageRepoName := strings.Split(strings.TrimPrefix(imageInfo[0], fmt.Sprintf("%s/", imageRegistryName)), ":")[0]
	// Cosign creates tags for attestation and signature based on the image digest. Compute
	// the expected prefix for later usage: sha256:abcd... -> sha256-abcd...
	// Also, this prefix is really the prefix of the image tag resource which follows the
	// format: <image-repo>:<tag-name>
	imageTagPrefix := strings.Replace(imageInfo[1], ":", "-", 1)

	results := CosignResult{}
	signatureTag, err := getImageInfoFromQuay(imageRepoName, imageTagPrefix+".sig")
	if err != nil {
		errMsg += fmt.Sprintf("error when getting signature tag: %+v\n", err)
	} else {
		results.signatureImageRef = signatureTag.ImageRef
	}

	attestationTag, err := getImageInfoFromQuay(imageRepoName, imageTagPrefix+".att")
	if err != nil {
		errMsg += fmt.Sprintf("error when getting attestation tag: %+v\n", err)
	} else {
		results.attestationImageRef = attestationTag.ImageRef
	}

	if len(errMsg) > 0 {
		return &results, fmt.Errorf("failed to find cosign results for image %s: %s", imageRef, errMsg)
	}

	return &results, nil
}

func getImageInfoFromQuay(imageRepo, imageTag string) (*QuayImageInfo, error) {

	res, err := http.Get(fmt.Sprintf("%s/repository/%s/tag/?specificTag=%s", quayBaseUrl, imageRepo, imageTag))
	if err != nil {
		return nil, fmt.Errorf("cannot get quay.io/%s:%s image from container registry: %+v", imageRepo, imageTag, err)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read body of a response from quay.io regarding quay.io/%s:%s image %+v", imageRepo, imageTag, err)
	}

	tagResponse := &TagResponse{}
	if err = json.Unmarshal(body, tagResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response from quay.io regarding quay.io/%s:%s image %+v", imageRepo, imageTag, err)
	}

	if len(tagResponse.Tags) < 1 {
		return nil, fmt.Errorf("cannot get manifest digest from quay.io/%s:%s image. response body: %+v", imageRepo, imageTag, string(body))
	}

	quayImageInfo := &QuayImageInfo{}
	quayImageInfo.ImageRef = fmt.Sprintf("quay.io/%s@%s", imageRepo, tagResponse.Tags[0].Digest)

	if strings.Contains(imageTag, ".att") {
		res, err = http.Get(fmt.Sprintf("%s/repository/%s/manifest/%s", quayBaseUrl, imageRepo, tagResponse.Tags[0].Digest))
		if err != nil {
			return nil, fmt.Errorf("cannot get quay.io/%s@%s image from container registry: %+v", imageRepo, quayImageInfo.ImageRef, err)
		}
		body, err = io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot read body of a response from quay.io regarding %s image: %+v", quayImageInfo.ImageRef, err)
		}
		manifestResponse := &ManifestResponse{}
		if err := json.Unmarshal(body, manifestResponse); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response from quay.io regarding %s image: %+v", quayImageInfo.ImageRef, err)
		}

		if len(manifestResponse.Layers) < 1 {
			return nil, fmt.Errorf("cannot get layers from %s image. response body: %+v", quayImageInfo.ImageRef, string(body))
		}
		quayImageInfo.Layers = manifestResponse.Layers
	}

	return quayImageInfo, nil
}

func (k KubeController) CreateOrUpdateSigningSecret(publicKey []byte, name, namespace string) (err error) {
	api := k.Tektonctrl.KubeInterface().CoreV1().Secrets(namespace)
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

func (k KubeController) GetTektonChainsPublicKey() ([]byte, error) {
	namespace := constants.TEKTON_CHAINS_NS
	secretName := "public-key"
	dataKey := "cosign.pub"

	secret, err := k.Tektonctrl.KubeInterface().CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("couldn't get the secret %s from %s namespace: %+v", secretName, namespace, err)
	}
	publicKey := secret.Data[dataKey]
	if len(publicKey) < 1 {
		return nil, fmt.Errorf("the content of the public key '%s' in secret %s in %s namespace is empty", dataKey, secretName, namespace)
	}
	return publicKey, err
}

func (k KubeController) CreateOrUpdatePolicyConfiguration(namespace string, policy ecp.EnterpriseContractPolicySpec) error {
	ecPolicy := ecp.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ec-policy",
			Namespace: namespace,
		},
	}

	// fetch to see if it exists
	err := k.Tektonctrl.KubeRest().Get(context.TODO(), crclient.ObjectKey{
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
		if err := k.Tektonctrl.KubeRest().Create(context.TODO(), &ecPolicy); err != nil {
			return err
		}
	} else {
		// it does, so update
		if err := k.Tektonctrl.KubeRest().Update(context.TODO(), &ecPolicy); err != nil {
			return err
		}
	}

	return nil
}

func (k KubeController) GetRekorHost() (rekorHost string, err error) {
	api := k.Tektonctrl.KubeInterface().CoreV1().ConfigMaps(constants.TEKTON_CHAINS_NS)
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
	return ec, s.KubeRest().Create(context.TODO(), ec)
}

// GetEnterpriseContractPolicy gets an EnterpriseContractPolicy from specified a namespace
func (k KubeController) GetEnterpriseContractPolicy(name, namespace string) (*ecp.EnterpriseContractPolicy, error) {
	defaultEcPolicy := ecp.EnterpriseContractPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := k.Tektonctrl.KubeRest().Get(context.TODO(), crclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &defaultEcPolicy)

	return &defaultEcPolicy, err
}

// CreatePVCInAccessMode creates a PVC with mode as passed in arguments.
func (s *SuiteController) CreatePVCInAccessMode(name, namespace string, accessMode corev1.PersistentVolumeAccessMode) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessMode,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	createdPVC, err := s.KubeInterface().CoreV1().PersistentVolumeClaims(namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdPVC, err
}

// GetListOfPipelineRunsInNamespace returns a List of all PipelineRuns in namespace.
func (s *SuiteController) GetListOfPipelineRunsInNamespace(namespace string) (*v1beta1.PipelineRunList, error) {
	return s.PipelineClient().TektonV1beta1().PipelineRuns(namespace).List(context.TODO(), metav1.ListOptions{})
}

// CreateTaskRunCopy creates a TaskRun that copies one image to a second image repository
func (s *SuiteController) CreateTaskRunCopy(name, namespace, serviceAccountName, srcImageURL, destImageURL string) (*v1beta1.TaskRun, error) {
	taskRun := v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: serviceAccountName,
			TaskRef: &v1beta1.TaskRef{
				Name: "skopeo-copy",
			},
			Params: []v1beta1.Param{
				{
					Name: "srcImageURL",
					Value: v1beta1.ParamValue{
						StringVal: srcImageURL,
						Type:      v1beta1.ParamTypeString,
					},
				},
				{
					Name: "destImageURL",
					Value: v1beta1.ParamValue{
						StringVal: destImageURL,
						Type:      v1beta1.ParamTypeString,
					},
				},
			},
			// workaround to avoid the error "container has runAsNonRoot and image will run as root"
			PodTemplate: &pod.Template{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: pointer.Bool(true),
					RunAsUser:    pointer.Int64(65532),
				},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:     "images-url",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}

	err := s.KubeRest().Create(context.TODO(), &taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// GetTaskRun returns the requested TaskRun object
func (s *SuiteController) GetTaskRun(name, namespace string) (*v1beta1.TaskRun, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	taskRun := v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := s.KubeRest().Get(context.TODO(), namespacedName, &taskRun)
	if err != nil {
		return nil, err
	}
	return &taskRun, nil
}

// CreateSkopeoCopyTask creates a skopeo copy task in the given namespace
func (s *SuiteController) CreateSkopeoCopyTask(namespace string) error {
	_, err := exec.Command(
		"oc",
		"apply",
		"-f",
		"https://api.hub.tekton.dev/v1/resource/tekton/task/skopeo-copy/0.2/raw",
		"-n",
		namespace).Output()

	return err
}

// Remove all Tasks from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllTasksInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.Task{}, crclient.InNamespace(namespace))
}

// Remove all TaskRuns from a given repository. Useful when creating a lot of resources and wanting to remove all of them
func (h *SuiteController) DeleteAllTaskRunsInASpecificNamespace(namespace string) error {
	return h.KubeRest().DeleteAllOf(context.TODO(), &v1beta1.TaskRun{}, crclient.InNamespace(namespace))
}

// GetTask returns the requested Task object
func (s *SuiteController) GetTask(name, namespace string) (*v1beta1.Task, error) {
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	task := v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := s.KubeRest().Get(context.TODO(), namespacedName, &task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}
