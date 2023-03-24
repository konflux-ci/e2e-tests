package o11y

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
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
