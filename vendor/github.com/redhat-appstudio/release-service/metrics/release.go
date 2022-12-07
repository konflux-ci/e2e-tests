package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReleaseAttemptConcurrentTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "release_attempt_concurrent_requests",
			Help: "Total number of concurrent release attempts",
		},
	)

	ReleaseAttemptDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "release_attempt_duration_seconds",
			Help:    "Release durations from the moment the release PipelineRun was created til the release is marked as finished",
			Buckets: []float64{60, 150, 300, 450, 600, 750, 900, 1050, 1200, 1800, 3600},
		},
		[]string{"reason", "strategy", "succeeded", "target"},
	)

	ReleaseAttemptInvalidTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_attempt_invalid_total",
			Help: "Number of invalid releases",
		},
		[]string{"reason"},
	)

	ReleaseAttemptRunningSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "release_attempt_running_seconds",
			Help:    "Release durations from the moment the release resource was created til the release is marked as running",
			Buckets: []float64{0.5, 1, 2, 3, 4, 5, 6, 7, 10, 15, 30},
		},
	)

	ReleaseAttemptTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "release_attempt_total",
			Help: "Total number of releases processed by the operator",
		},
		[]string{"reason", "strategy", "succeeded", "target"},
	)
)

// RegisterCompletedRelease decrements the 'release_attempt_concurrent_total' metric, increments `release_attempt_total`
// and registers a new observation for 'release_attempt_duration_seconds' with the elapsed time from the moment the
// Release attempt started (Release marked as 'Running').
func RegisterCompletedRelease(reason, strategy, target string, startTime, completionTime *metav1.Time, succeeded bool) {
	labels := prometheus.Labels{
		"reason":    reason,
		"strategy":  strategy,
		"succeeded": strconv.FormatBool(succeeded),
		"target":    target,
	}

	ReleaseAttemptConcurrentTotal.Dec()
	ReleaseAttemptDurationSeconds.With(labels).Observe(completionTime.Sub(startTime.Time).Seconds())
	ReleaseAttemptTotal.With(labels).Inc()
}

// RegisterInvalidRelease increments the 'release_attempt_invalid_total' and `release_attempt_total` metrics.
func RegisterInvalidRelease(reason string) {
	ReleaseAttemptInvalidTotal.With(prometheus.Labels{"reason": reason}).Inc()
	ReleaseAttemptTotal.With(prometheus.Labels{
		"reason":    reason,
		"strategy":  "",
		"succeeded": "false",
		"target":    "",
	}).Inc()
}

// RegisterNewRelease increments the 'release_attempt_concurrent_total' and registers a new observation for
// 'release_attempt_duration_seconds' with the elapsed time from the moment the Release was created to when
// it started (Release marked as 'Running').
func RegisterNewRelease(creationTime metav1.Time, startTime *metav1.Time) {
	ReleaseAttemptConcurrentTotal.Inc()
	ReleaseAttemptRunningSeconds.Observe(startTime.Sub(creationTime.Time).Seconds())
}

func init() {
	metrics.Registry.MustRegister(
		ReleaseAttemptConcurrentTotal,
		ReleaseAttemptDurationSeconds,
		ReleaseAttemptInvalidTotal,
		ReleaseAttemptRunningSeconds,
		ReleaseAttemptTotal,
	)
}
