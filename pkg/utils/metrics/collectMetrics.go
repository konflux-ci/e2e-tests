package metrics

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils/authorization"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/prometheus"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/getters"
	k8sutil "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const MB = 1 << 20


type Collector struct {
	k8sClient     client.Client
	queryInterval time.Duration
	mgetters      []getters.Getter
	results       map[string]CollectResult
}

type CollectResult struct {
	sCount 		int
	max         float64
	sum         float64
}

func CreateNewInstance(cl client.Client, token string, interval time.Duration) *Collector {

	prometheusClient := prometheus.GetPrometheusClient(cl, token)

	instance := &Collector{
		k8sClient:     cl,
		queryInterval: interval,
	}

	// Add default getters
	instance.AddGetters(
		getters.GetClusterCPUUtilisation(prometheusClient),
		getters.GetClusterMemoryUtilisation(prometheusClient),
		getters.GetNodeMemoryUtilisation(prometheusClient),
		getters.GetEtcdMemoryUsage(prometheusClient),
		getters.GetWorkloadCPUUsage(prometheusClient, constants.OLMOperatorNamespace, constants.OLMOperatorWorkload),
		getters.GetWorkloadMemoryUsage(prometheusClient, constants.OLMOperatorNamespace, constants.OLMOperatorWorkload),
		getters.GetOpenshiftKubeAPIMemoryUtilisation(prometheusClient),
		getters.GetWorkloadCPUUsage(prometheusClient, constants.OSAPIServerNamespace, constants.OSAPIServerWorkload),
		getters.GetWorkloadMemoryUsage(prometheusClient, constants.OSAPIServerNamespace, constants.OSAPIServerWorkload),
	)
	instance.results = make(map[string]CollectResult, len(instance.mgetters))

	return instance
}

func (g *Collector) AddGetters(getters ...getters.Getter) {
	g.mgetters = append(g.mgetters, getters...)
}

func (i *Collector) StartCollecting() chan struct{} {
	if len(i.mgetters) == 0 {
		klog.Infof("Metrics collectors has no getters defined, skipping metrics gathering...")
		return nil
	}

	stop := make(chan struct{})
	go func() {
		k8sutil.Until(func() {
			for _, q := range i.mgetters {
				err := i.runGetters(q)
				if err != nil {
					klog.Fatalf("metrics error: %v", err)
				}
			}
		}, i.queryInterval, stop)
	}()
	return stop
}

func (i *Collector) runGetters(g getters.Getter) error {
	val, warnings, err := g.Execute()
	if err != nil {
		if strings.Contains(err.Error(), "client error: 403") {
			url, tokenErr := authorization.FindTokenRequestURI(i.k8sClient)
			if tokenErr != nil {
				return errors.Wrapf(err, "metrics getter failed with 403 (Forbidden)")
			}
			return errors.Wrapf(err, "metrics getter failed with 403 (Forbidden) - retrieve a new token from %s", url)
		}
		return errors.Wrapf(err, "metrics getter failed - check whether prometheus is still healthy in the cluster")
	} else if len(warnings) > 0 {
		return errors.Wrapf(fmt.Errorf("warnings: %v", warnings), "metrics getter had unexpected warnings")
	}

	vector := val.(model.Vector)
	if len(vector) == 0 {
		return fmt.Errorf("metrics value could not be retrieved for getter %s", g.Name())
	}

	// if a result returns multiple vector samples we'll take the average of the values to get a single datapoint for the sake of simplicity
	var vectorSum float64
	for _, v := range vector {
		vectorSum += float64(v.Value)
	}
	datapoint := vectorSum / float64(len(vector))

	r := i.results[g.Name()]
	r.max = math.Max(r.max, datapoint)
	r.sum += datapoint
	r.sCount++
	i.results[g.Name()] = r
	return nil
}

// PrintResults iterates through each query and prints the aggregated results to the terminal
func (i *Collector) PrintResults() {
	for _, g := range i.mgetters {
		switch g.Result() {
		case "percentage":
			PrintPercent(g.Name(), i.results[g.Name()])
		case "memory":
			PrintMem(g.Name(), i.results[g.Name()])
		case "simple":
			PrintSimple(g.Name(), i.results[g.Name()])
		default:
			klog.Fatalf("query %s is missing a result type", g.Name(), "invalid query")
		}
	}
}

func PrintPercent(name string, r CollectResult) {
	avg := r.sum / float64(r.sCount)
	klog.Infof("Average %s: %.2f %%", name, avg*100)
	klog.Infof("Max %s: %.2f %%", name, r.max*100)
}

func PrintMem(name string, r CollectResult) {
	avg := r.sum / float64(r.sCount)
	klog.Infof("Average %s: %s", name, bytesToMBString(avg))
	klog.Infof("Max %s: %s", name, bytesToMBString(r.max))
}

func PrintSimple(name string, r CollectResult) {
	avg := r.sum / float64(r.sCount)
	klog.Infof("Average %s: %s", name, simple(avg))
	klog.Infof("Max %s: %s", name, simple(r.max))
}

func bytesToMBString(bytes float64) string {
	return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
}

// simple returns the provided number as a string formatted to 4 decimal places
func simple(value float64) string {
	return fmt.Sprintf("%.4f", value)
}