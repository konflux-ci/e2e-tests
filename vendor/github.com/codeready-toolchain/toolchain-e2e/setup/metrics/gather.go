package metrics

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/codeready-toolchain/toolchain-e2e/setup/auth"
	cfg "github.com/codeready-toolchain/toolchain-e2e/setup/configuration"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics/queries"
	"github.com/codeready-toolchain/toolchain-e2e/setup/terminal"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"

	k8sutil "k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OpenshiftMonitoringNS = "openshift-monitoring"
	PrometheusRouteName   = "prometheus-k8s"

	OLMOperatorNamespace = "openshift-operator-lifecycle-manager"
	OLMOperatorWorkload  = "olm-operator"

	OSAPIServerNamespace = "openshift-apiserver"
	OSAPIServerWorkload  = "apiserver"
)

type Gatherer struct {
	k8sClient     client.Client
	queryInterval time.Duration
	mqueries      []queries.Query
	results       map[string]aggregateResult
	term          terminal.Terminal
}

type aggregateResult struct {
	sampleCount int
	max         float64
	sum         float64
}

// New creates a new gatherer with default queries
func New(t terminal.Terminal, cl client.Client, token string, interval time.Duration) *Gatherer {
	g := &Gatherer{
		k8sClient:     cl,
		queryInterval: interval,
		term:          t,
	}

	prometheusClient := GetPrometheusClient(t, cl, token)

	// Add default queries
	g.AddQueries(
		queries.QueryClusterCPUUtilisation(prometheusClient),
		queries.QueryClusterMemoryUtilisation(prometheusClient),
		queries.QueryNodeMemoryUtilisation(prometheusClient),
		queries.QueryEtcdMemoryUsage(prometheusClient),
		queries.QueryWorkloadCPUUsage(prometheusClient, OLMOperatorNamespace, OLMOperatorWorkload),
		queries.QueryWorkloadMemoryUsage(prometheusClient, OLMOperatorNamespace, OLMOperatorWorkload),
		queries.QueryOpenshiftKubeAPIMemoryUtilisation(prometheusClient),
		queries.QueryWorkloadCPUUsage(prometheusClient, OSAPIServerNamespace, OSAPIServerWorkload),
		queries.QueryWorkloadMemoryUsage(prometheusClient, OSAPIServerNamespace, OSAPIServerWorkload),
		queries.QueryWorkloadCPUUsage(prometheusClient, cfg.HostOperatorNamespace, cfg.HostOperatorWorkload),
		queries.QueryWorkloadMemoryUsage(prometheusClient, cfg.HostOperatorNamespace, cfg.HostOperatorWorkload),
		queries.QueryWorkloadCPUUsage(prometheusClient, cfg.MemberOperatorNamespace, cfg.MemberOperatorWorkload),
		queries.QueryWorkloadMemoryUsage(prometheusClient, cfg.MemberOperatorNamespace, cfg.MemberOperatorWorkload),
	)
	g.results = make(map[string]aggregateResult, len(g.mqueries))

	return g
}

//nolint
func NewEmpty(t terminal.Terminal, cl client.Client, interval time.Duration) *Gatherer {
	g := &Gatherer{
		k8sClient:     cl,
		queryInterval: interval,
		term:          t,
	}
	g.results = make(map[string]aggregateResult, len(g.mqueries))
	return g
}

func (g *Gatherer) AddQueries(queries ...queries.Query) {
	g.mqueries = append(g.mqueries, queries...)
}

func (g *Gatherer) StartGathering() chan struct{} {
	if len(g.mqueries) == 0 {
		g.term.Infof("Metrics gatherer has no queries defined, skipping metrics gathering...")
		return nil
	}

	// ensure metrics are dumped if there's a fatal error
	g.term.AddPreFatalExitHook(g.PrintResults)

	stop := make(chan struct{})
	go func() {
		k8sutil.Until(func() {
			for _, q := range g.mqueries {
				err := g.sample(q)
				if err != nil {
					g.term.Fatalf(err, "metrics error")
				}
			}
		}, g.queryInterval, stop)
	}()
	return stop
}

func (g *Gatherer) sample(q queries.Query) error {
	val, warnings, err := q.Execute()
	if err != nil {
		if strings.Contains(err.Error(), "client error: 403") {
			url, tokenErr := auth.GetTokenRequestURI(g.k8sClient)
			if tokenErr != nil {
				return errors.Wrapf(err, "metrics query failed with 403 (Forbidden)")
			}
			return errors.Wrapf(err, "metrics query failed with 403 (Forbidden) - retrieve a new token from %s", url)
		}
		return errors.Wrapf(err, "metrics query failed - check whether prometheus is still healthy in the cluster")
	} else if len(warnings) > 0 {
		return errors.Wrapf(fmt.Errorf("warnings: %v", warnings), "metrics query had unexpected warnings")
	}

	vector := val.(model.Vector)
	if len(vector) == 0 {
		return fmt.Errorf("metrics value could not be retrieved for query %s", q.Name())
	}

	// if a result returns multiple vector samples we'll take the average of the values to get a single datapoint for the sake of simplicity
	var vectorSum float64
	for _, v := range vector {
		vectorSum += float64(v.Value)
	}
	datapoint := vectorSum / float64(len(vector))

	r := g.results[q.Name()]
	r.max = math.Max(r.max, datapoint)
	r.sum += datapoint
	r.sampleCount++
	g.results[q.Name()] = r
	return nil
}

// PrintResults iterates through each query and prints the aggregated results to the terminal
func (g *Gatherer) PrintResults() {
	for _, q := range g.mqueries {
		switch q.ResultType() {
		case "percentage":
			PrintPercentage(g.term, q.Name(), g.results[q.Name()])
		case "memory":
			PrintMemory(g.term, q.Name(), g.results[q.Name()])
		case "simple":
			PrintSimple(g.term, q.Name(), g.results[q.Name()])
		default:
			g.term.Fatalf(fmt.Errorf("query %s is missing a result type", q.Name()), "invalid query")
		}
	}
}

func PrintPercentage(t terminal.Terminal, name string, r aggregateResult) {
	avg := r.sum / float64(r.sampleCount)
	t.Infof("Average %s: %.2f %%", name, avg*100)
	t.Infof("Max %s: %.2f %%", name, r.max*100)
}

func PrintMemory(t terminal.Terminal, name string, r aggregateResult) {
	avg := r.sum / float64(r.sampleCount)
	t.Infof("Average %s: %s", name, bytesToMBString(avg))
	t.Infof("Max %s: %s", name, bytesToMBString(r.max))
}

func PrintSimple(t terminal.Terminal, name string, r aggregateResult) {
	avg := r.sum / float64(r.sampleCount)
	t.Infof("Average %s: %s", name, simple(avg))
	t.Infof("Max %s: %s", name, simple(r.max))
}
