package o11y

import (
	"fmt"
	"regexp"
)

// GetRegexPodNameWithResult returns podNameWithResults map of pods matching the podNameRegex pattern.
func (o *O11yController) GetRegexPodNameWithResult(podNameRegex string, results []MetricResult) (map[string]string, error) {
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
