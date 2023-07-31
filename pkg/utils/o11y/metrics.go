package o11y

import (
	"encoding/json"
	"os/exec"
)

type MetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// GetMetrics fetches metrics for given query.
func (o *O11yController) GetMetrics(query string) ([]MetricResult, error) {
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
