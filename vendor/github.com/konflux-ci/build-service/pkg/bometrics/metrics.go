package bometrics

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const (
	MetricsNamespace = "redhat_appstudio"
	MetricsSubsystem = "buildservice"
)

var (
	HistogramBuckets              = []float64{5, 10, 15, 20, 30, 60, 120, 300}
	ComponentOnboardingTimeMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystem,
		Buckets:   HistogramBuckets,
		Name:      "component_onboarding_time",
		Help:      "The time in seconds spent from the moment of Component creation till simple build pipeline submission, or PaC provision.",
	})
	SimpleBuildPipelineCreationTimeMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystem,
		Buckets:   HistogramBuckets,
		Name:      "simple_build_pipeline_creation_time",
		Help:      "The time in seconds spent from the moment of requesting simple build for Component till build pipeline submission.",
	})
	PipelinesAsCodeComponentProvisionTimeMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystem,
		Buckets:   HistogramBuckets,
		Name:      "PaC_configuration_time",
		Help:      "The time in seconds spent from the moment of requesting PaC provision till Pipelines-as-Code configuration done in the Component source repository.",
	})
	PipelinesAsCodeComponentUnconfigureTimeMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystem,
		Buckets:   HistogramBuckets,
		Name:      "PaC_unconfiguration_time",
		Help:      "The time in seconds spent from the moment of requesting PaC unprovision till Pipelines-as-Code configuration is removed in the Component source repository.",
	})
	PushPipelineRebuildTriggerTimeMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: MetricsNamespace,
		Subsystem: MetricsSubsystem,
		Buckets:   HistogramBuckets,
		Name:      "Push_pipeline_rebuild_trigger_time",
		Help:      "The time in seconds spent from the moment of requesting push pipeline rebuild till Pipelines-as-Code API trigger.",
	})
	ComponentTimesForMetrics = map[string]ComponentMetricsInfo{}
)

type ComponentMetricsInfo struct {
	StartTimestamp  time.Time
	RequestedAction string
}

// BuildMetrics represents a collection of metrics to be registered on a
// Prometheus metrics registry for a build service.
type BuildMetrics struct {
	probes []AvailabilityProbe
}

func NewBuildMetrics(probes []AvailabilityProbe) *BuildMetrics {
	return &BuildMetrics{probes: probes}
}

func (m *BuildMetrics) InitMetrics(registerer prometheus.Registerer) error {
	registerer.MustRegister(ComponentOnboardingTimeMetric, SimpleBuildPipelineCreationTimeMetric, PipelinesAsCodeComponentProvisionTimeMetric, PipelinesAsCodeComponentUnconfigureTimeMetric, PushPipelineRebuildTriggerTimeMetric)
	for _, probe := range m.probes {
		if err := registerer.Register(probe.AvailabilityGauge()); err != nil {
			return fmt.Errorf("failed to register the availability metric: %w", err)
		}
	}

	return nil
}
func (m *BuildMetrics) StartAvailabilityProbes(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	log := ctrllog.FromContext(ctx)
	log.Info("starting availability probes")
	go func() {
		for {
			select {
			case <-ctx.Done(): // Shutdown if context is canceled
				log.Info("Shutting down metrics")
				ticker.Stop()
				return
			case <-ticker.C:
				m.checkProbes(ctx)
			}
		}
	}()
}

func (m *BuildMetrics) checkProbes(ctx context.Context) {
	for _, probe := range m.probes {
		if probe.CheckAvailability(ctx) {
			probe.AvailabilityGauge().Set(1)
		} else {
			probe.AvailabilityGauge().Set(0)
		}
	}
}

// AvailabilityProbe represents a probe that checks the availability of a certain aspects of the service
type AvailabilityProbe interface {
	CheckAvailability(ctx context.Context) bool
	AvailabilityGauge() prometheus.Gauge
}
