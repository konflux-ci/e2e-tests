package o11y

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type SuiteController struct {
	*kubeCl.CustomClient
}

type MetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

func (h *SuiteController) convertBytesToMB(bytesValue float64) float64 {
	megabytesValue := bytesValue / (1000 * 1000)
	return math.Round(megabytesValue*10) / 10
}

// Fetch metrics for given query
func (h *SuiteController) GetMetrics(query string) ([]MetricResult, error) {

	var result struct {
		Data struct {
			Result []MetricResult `json:"result"`
		} `json:"data"`
	}

	// Temporary way to fetch the metrics, will be replaced by golang http client library
	// curl -X GET -kG "https://$THANOS_QUERIER_HOST/api/v1/query?" --data-urlencode "query="+query -H "Authorization: Bearer $TOKEN"
	curlCmd := exec.Command("curl", "-X", "GET", "-kG", "http://localhost:8080/api/v1/query", "--data-urlencode", "query="+query)
	output, err := curlCmd.Output()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(output, &result)
	if err != nil {
		return nil, err
	}

	return result.Data.Result, nil
}

func (h *SuiteController) GetRegexPodNameWithSize(podNameRegex string, results []MetricResult) (map[string]float64, error) {
	podNameWithSize := make(map[string]float64)
	regex, err := regexp.Compile(podNameRegex)
	if err != nil {
		return podNameWithSize, fmt.Errorf("error compiling regex pattern: %v", err)
	}

	for _, res := range results {
		if podName, ok := res.Metric["pod"]; ok {
			if regex.MatchString(podName) {
				value := res.Value[1].(string)
				valueInBytes, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return nil, fmt.Errorf("error parsing value for %s: %s", podName, err)
				}
				valueInMegabytes := h.convertBytesToMB(valueInBytes)
				podNameWithSize[podName] = valueInMegabytes
			}
		}
	}

	if len(podNameWithSize) == 0 {
		return nil, fmt.Errorf("no pods matching the regex pattern were found")
	}

	return podNameWithSize, nil
}

func (h *SuiteController) QuayImagePushPipelineRun(quayOrg, secret, namespace string) (*v1beta1.PipelineRun, error) {
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipeline-egress-",
			Namespace:    namespace,
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
										Script: fmt.Sprintf(`#!/bin/sh
												authFilePath="/tekton/creds-secrets/%s/.dockerconfigjson"
												destImageRef="quay.io/%s/o11y-workloads"
												# Set Permissions
												sed -i 's/^\s*short-name-mode\s*=\s*.*/short-name-mode = "disabled"/' /etc/containers/registries.conf
												# Setting new namespace to run buildah - 2^32-2
												echo 'root:1:4294967294' | tee -a /etc/subuid >> /etc/subgid
												# Pull Image
												echo -e "FROM quay.io/libpod/alpine:latest\nRUN dd if=/dev/urandom of=/100mbfile bs=1M count=100" > Dockerfile
												unshare -Ufp --keep-caps -r --map-users 1,1,65536 --map-groups 1,1,65536 -- buildah bud --tls-verify=false --no-cache -f ./Dockerfile -t "$destImageRef" .
												IMAGE_SHA_DIGEST=$(buildah images --digests | grep ${destImageRef} | awk '{print $4}')
												TAGGED_IMAGE_NAME="${destImageRef}:${IMAGE_SHA_DIGEST}"
												buildah tag ${destImageRef} ${TAGGED_IMAGE_NAME}
												buildah images
												buildah push --authfile "$authFilePath" --disable-compression --tls-verify=false ${TAGGED_IMAGE_NAME}
												echo "Successfully pushed Image"
												# Scraping Interval Period, Pod must stay alive:
												sleep 1m`, secret, quayOrg),
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
