package o11y

import (
	"fmt"
	"strconv"
)

// convertBytesToMB converts Bytes to Megabytes.
func convertBytesToMB(valueInBytes float64) int {
	valueInMegabytes := valueInBytes / (1000 * 1000)
	return int(valueInMegabytes)
}

// ConvertValuesToMB converts results (strings in bytes) in podNamesWithResult map to ints in MB.
func (o *O11yController) ConvertValuesToMB(podNamesWithResult map[string]string) (map[string]int, error) {
	podNameWithMB := make(map[string]int)

	for podName, value := range podNamesWithResult {
		valueStr := value

		valueInBytes, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing value for %s: %s", podName, err)
		}

		valueInMegabytes := convertBytesToMB(valueInBytes)
		podNameWithMB[podName] = int(valueInMegabytes)
	}

	return podNameWithMB, nil
}
