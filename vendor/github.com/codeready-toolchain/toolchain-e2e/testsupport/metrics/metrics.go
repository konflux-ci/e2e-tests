package metrics

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/client-go/rest"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func GetMetricValue(restConfig *rest.Config, url string, family string, expectedLabels []string) (float64, error) {
	if len(expectedLabels)%2 != 0 {
		return -1, fmt.Errorf("received odd number of label arguments, labels must be key-value pairs")
	}
	uri := fmt.Sprintf("https://%s/metrics", url)
	var metrics []byte

	client := http.Client{
		Timeout: time.Duration(10 * time.Second),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	request, err := http.NewRequest("Get", uri, nil)
	if err != nil {
		return -1, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", restConfig.BearerToken))
	resp, err := client.Do(request)
	if err != nil {
		return -1, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	metrics, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}

	// parse the metrics
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(bytes.NewReader(metrics))
	if err != nil {
		return -1, err
	}
	for _, f := range families {
		if f.GetName() == family {
			metricType := f.GetType()
			// metric without labels
			if len(f.GetMetric()) == 1 && len(expectedLabels) == 0 {
				return getValue(metricType, f.GetMetric()[0])
			}

		metricSearch:
			for _, m := range f.GetMetric() {
				metricLabels := m.GetLabel()
				if len(metricLabels) != len(expectedLabels)/2 {
					continue
				}
				for i := 0; i < len(expectedLabels); {
					labelFound := false
					for _, l := range metricLabels {
						if l.GetName() == expectedLabels[i] && l.GetValue() == expectedLabels[i+1] {
							labelFound = true
						}
					}
					if !labelFound {
						continue metricSearch
					}
					i += 2
				}
				return getValue(metricType, m)
			}
		}
	}
	// here we can return `0` is the metric does not exist, which may be valid if the expected value is `0`, too.
	return 0, fmt.Errorf("metric '%s{%v}' not found", family, expectedLabels)
}

func getValue(t dto.MetricType, m *dto.Metric) (float64, error) {
	switch t { // nolint:exhaustive
	case dto.MetricType_COUNTER:
		return m.GetCounter().GetValue(), nil
	case dto.MetricType_GAUGE:
		return m.GetGauge().GetValue(), nil
	case dto.MetricType_UNTYPED:
		return m.GetUntyped().GetValue(), nil
	default:
		return -1, fmt.Errorf("unknown or unsupported metric type %s", t.String())
	}
}
