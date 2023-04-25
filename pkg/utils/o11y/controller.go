package o11y

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SuiteController struct {
	*kubeCl.CustomClient
}

type MetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type ThanosResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string         `json:"resultType"`
		Result     []MetricResult `json:"result"`
	} `json:"data"`
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

func (h *SuiteController) convertBytesToMB(valueInBytes float64) int {
	valueInMegabytes := valueInBytes / (1000 * 1000)
	return int(valueInMegabytes)
}

func (h *SuiteController) GetSecretName(namespace, pattern string) (string, error) {
	secrets, err := h.KubeInterface().CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, secret := range secrets.Items {
		if strings.Contains(secret.Name, pattern) {
			return secret.Name, nil
		}
	}

	return "", fmt.Errorf("no matching secrets found")
}

func (h *SuiteController) GetDecodedTokenFromSecret(namespace, secretName string) (string, error) {
	secret, err := h.KubeInterface().CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	tokenData, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("token not found in secret %s", secretName)
	}
	token := string(tokenData)

	return token, nil
}

func (h *SuiteController) RunSampleQuery(namespace, thanosQuerierHost, token string) ([]MetricResult, error) {
	var result ThanosResult

	ok, err := h.runQuery("up", thanosQuerierHost, token, &result)
	if err != nil {
		return nil, fmt.Errorf("sample query failed: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("no result returned for sample query")
	}

	return result.Data.Result, nil
}

func (h *SuiteController) QueryThanosAPI(query, thanosQuerierHost, token string) ([]MetricResult, error) {
	var result ThanosResult
	maxRetries := 3
	waitTime := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		ok, err := h.runQuery(query, thanosQuerierHost, token, &result)
		if err != nil {
			log.Printf("Error in running query, attempt %d: %v", i+1, err)
		}
		if ok {
			return result.Data.Result, nil
		}
		time.Sleep(waitTime)
	}
	return nil, fmt.Errorf("the result is empty after %d retries", maxRetries)
}

func (h *SuiteController) runQuery(query, thanosQuerierHost, token string, result *ThanosResult) (bool, error) {
	encodedQuery := url.QueryEscape(query)
	url := fmt.Sprintf("https://%s/api/v1/query?query=%s", thanosQuerierHost, encodedQuery)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create a new request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP request failed with status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(bodyBytes, result)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return len(result.Data.Result) != 0, nil
}

func (h *SuiteController) GetRegexPodNameWithResult(podNameRegex string, results []MetricResult) (map[string]string, error) {
	podNamesWithResult := make(map[string]string)
	regex, err := regexp.Compile(podNameRegex)
	if err != nil {
		return podNamesWithResult, fmt.Errorf("error compiling regex pattern: %v", err)
	}

	for _, res := range results {
		if podName, ok := res.Metric["pod"]; ok {
			if regex.MatchString(podName) {
				value := res.Value[1].(string)
				podNamesWithResult[podName] = value
			}
		}
	}

	if len(podNamesWithResult) == 0 {
		return nil, fmt.Errorf("no pods matching the regex pattern were found")
	}

	return podNamesWithResult, nil
}

func (h *SuiteController) ConvertValuesToMB(podNamesWithResult map[string]string) (map[string]int, error) {
	podNameWithMB := make(map[string]int)

	for podName, value := range podNamesWithResult {
		valueStr := value

		valueInBytes, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing value for %s: %s", podName, err)
		}

		valueInMegabytes := h.convertBytesToMB(valueInBytes)
		podNameWithMB[podName] = int(valueInMegabytes)
	}

	return podNameWithMB, nil
}

func labelsToSelector(labelMap map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range labelMap {
		req, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*req)
	}
	return selector
}

func (s *SuiteController) WaitForScriptCompletion(deployment *appsv1.Deployment, successMessage string, timeout time.Duration) error {
	namespace := deployment.Namespace
	deploymentName := deployment.Name

	// Get the pod associated with the deployment
	podList := &corev1.PodList{}
	labels := deployment.Spec.Selector.MatchLabels
	labelSelector := labelsToSelector(labels)
	err := s.KubeRest().List(context.Background(), podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: labelSelector})
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	pod := podList.Items[0]

	// Wait for the success message in the pod's log output
	podLogOpts := &corev1.PodLogOptions{}
	req := s.KubeInterface().CoreV1().Pods(namespace).GetLogs(pod.Name, podLogOpts)

	err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		readCloser, err := req.Stream(context.Background())
		if err != nil {
			return false, err
		}
		defer readCloser.Close()

		scanner := bufio.NewScanner(readCloser)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), successMessage) {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}

func (h *SuiteController) getImagePushScript(secret, quayOrg string) string {
	return fmt.Sprintf(`#!/bin/sh
authFilePath="/tekton/creds-secrets/%s/.dockerconfigjson"
destImageRef="quay.io/%s/test-images"
# Set Permissions
sed -i 's/^\s*short-name-mode\s*=\s*.*/short-name-mode = "disabled"/' /etc/containers/registries.conf
echo 'root:1:4294967294' | tee -a /etc/subuid >> /etc/subgid
# Pull Image
echo -e "FROM quay.io/libpod/alpine:latest\nRUN dd if=/dev/urandom of=/100mbfile bs=1M count=100" > Dockerfile
unshare -Ufp --keep-caps -r --map-users 1,1,65536 --map-groups 1,1,65536 -- buildah bud --tls-verify=false --no-cache -f ./Dockerfile -t "$destImageRef" .
IMAGE_SHA_DIGEST=$(buildah images --digests | grep ${destImageRef} | awk '{print $4}')
TAGGED_IMAGE_NAME="${destImageRef}:${IMAGE_SHA_DIGEST}"
buildah tag ${destImageRef} ${TAGGED_IMAGE_NAME}
buildah images
buildah push --authfile "$authFilePath" --disable-compression --tls-verify=false ${TAGGED_IMAGE_NAME}
if [ $? -eq 0 ]; then
  # Scraping Interval Period, Pod must stay alive
  sleep 1m
  echo "Image push completed"
else
  echo "Image push failed"
  exit 1
fi`, secret, quayOrg)
}

func (h *SuiteController) QuayImagePushPipelineRun(quayOrg, secret, namespace string) (*v1beta1.PipelineRun, error) {
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipelinerun-egress-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type": "test",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "buildah-quay",
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{
									{
										Name:  "pull-and-push-image",
										Image: "quay.io/redhat-appstudio/buildah:v1.28",
										Env: []corev1.EnvVar{
											{Name: "BUILDAH_FORMAT", Value: "oci"},
											{Name: "STORAGE_DRIVER", Value: "vfs"},
										},
										Script: h.getImagePushScript(secret, quayOrg),
										SecurityContext: &corev1.SecurityContext{
											RunAsUser: pointer.Int64(0),
											Capabilities: &corev1.Capabilities{
												Add: []corev1.Capability{
													"SETFCAP",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := h.KubeRest().Create(context.Background(), pipelineRun); err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

func (h *SuiteController) VCPUMinutesPipelineRun(namespace string) (*v1beta1.PipelineRun, error) {
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipelinerun-vcpu-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type": "test",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "vcpu-minutes",
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{
									{
										Name:   "resource-constraint",
										Image:  "registry.access.redhat.com/ubi9/ubi-micro",
										Script: "#!/usr/bin/env bash\nsleep 1m\necho 'vCPU Deployment Completed'\n",
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("100Mi"),
												corev1.ResourceCPU:    resource.MustParse("100m"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("100Mi"),
												corev1.ResourceCPU:    resource.MustParse("100m"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := h.KubeRest().Create(context.Background(), pipelineRun); err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

func (h *SuiteController) QuayImagePushDeployment(quayOrg, secret, namespace string) (*appsv1.Deployment, error) {
	Deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-egress",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deployment-egress",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "deployment-egress",
					},
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: secret,
						},
					},
					ServiceAccountName: constants.DefaultPipelineServiceAccount,
					Containers: []corev1.Container{
						{
							Name:  "quay-image-push-container",
							Image: "quay.io/redhat-appstudio/buildah:v1.28",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-config",
									MountPath: "/tekton/creds-secrets/o11y-tests-token/",
									ReadOnly:  true,
								},
							},
							Env: []corev1.EnvVar{
								{Name: "BUILDAH_FORMAT", Value: "oci"},
								{Name: "STORAGE_DRIVER", Value: "vfs"},
							},
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{h.getImagePushScript(secret, quayOrg) + "; while true; do sleep 30; done"},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: pointer.Int64(0),
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"SETFCAP",
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secret,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := h.KubeRest().Create(context.Background(), Deployment); err != nil {
		return &appsv1.Deployment{}, err
	}

	return Deployment, nil
}

func (h *SuiteController) VCPUMinutesDeployment(namespace string) (*appsv1.Deployment, error) {
	Deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-vcpu",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deployment-vcpu",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "deployment-vcpu",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vcpu-minutes",
							Image: "registry.access.redhat.com/ubi9/ubi-micro",
							Command: []string{
								"/bin/bash",
								"-c",
								"sleep 1m; echo 'vCPU Deployment Completed'; while true; do sleep 30; done",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
		},
	}

	if err := h.KubeRest().Create(context.Background(), Deployment); err != nil {
		return nil, err
	}

	return Deployment, nil
}
