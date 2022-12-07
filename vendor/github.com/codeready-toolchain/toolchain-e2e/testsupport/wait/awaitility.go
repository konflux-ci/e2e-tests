package wait

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"github.com/codeready-toolchain/toolchain-common/pkg/status"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/cleanup"
	"github.com/codeready-toolchain/toolchain-e2e/testsupport/metrics"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"k8s.io/kubectl/pkg/util/podutils"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	k8smetrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultRetryInterval             = time.Millisecond * 100 // make it short because a "retry interval" is waited before the first test
	DefaultTimeout                   = time.Second * 60
	MemberNsVar                      = "MEMBER_NS"
	MemberNsVar2                     = "MEMBER_NS_2"
	HostNsVar                        = "HOST_NS"
	RegistrationServiceVar           = "REGISTRATION_SERVICE_NS"
	ToolchainClusterConditionTimeout = 180 * time.Second
)

type Awaitility struct {
	T             *testing.T
	Client        client.Client
	RestConfig    *rest.Config
	ClusterName   string
	Namespace     string
	Type          cluster.Type
	RetryInterval time.Duration
	Timeout       time.Duration
	MetricsURL    string
}

func (a *Awaitility) GetClient() client.Client {
	return a.Client
}

func (a *Awaitility) GetT() *testing.T {
	return a.T
}

func (a *Awaitility) ForTest(t *testing.T) *Awaitility {
	await := a.copy()
	await.T = t
	return await
}

func (a *Awaitility) copy() *Awaitility {
	result := new(Awaitility)
	*result = *a
	return result
}

// ReadyToolchainCluster is a ClusterCondition that represents cluster that is ready
var ReadyToolchainCluster = &toolchainv1alpha1.ToolchainClusterCondition{
	Type:   toolchainv1alpha1.ToolchainClusterReady,
	Status: corev1.ConditionTrue,
}

// WithRetryOptions returns a new Awaitility with the given "RetryOption"s applied
func (a *Awaitility) WithRetryOptions(options ...RetryOption) *Awaitility {
	result := a.copy()
	for _, option := range options {
		option.apply(result)
	}
	return result
}

// RetryOption is some configuration that modifies options for an Awaitility.
type RetryOption interface {
	apply(*Awaitility)
}

// RetryInterval an option to configure the RetryInterval
type RetryInterval time.Duration

var _ RetryOption = RetryInterval(0)

func (o RetryInterval) apply(a *Awaitility) {
	a.RetryInterval = time.Duration(o)
}

// TimeoutOption an option to configure the Timeout
type TimeoutOption time.Duration

var _ RetryOption = TimeoutOption(0)

func (o TimeoutOption) apply(a *Awaitility) {
	a.Timeout = time.Duration(o)
}

// WaitForMetricsService waits until there's a service with the given name in the current namespace
func (a *Awaitility) WaitForMetricsService(name string) (corev1.Service, error) {
	a.T.Logf("waiting for Service '%s' in namespace '%s'", name, a.Namespace)
	var metricsSvc *corev1.Service
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		metricsSvc = &corev1.Service{}
		// retrieve the metrics service from the namespace
		err = a.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: a.Namespace,
				Name:      name,
			},
			metricsSvc)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	return *metricsSvc, err
}

// WaitForToolchainClusterWithCondition waits until there is a ToolchainCluster representing a operator of the given type
// and running in the given expected namespace. If the given condition is not nil, then it also checks
// if the CR has the ClusterCondition
func (a *Awaitility) WaitForToolchainClusterWithCondition(clusterType cluster.Type, namespace string, condition *toolchainv1alpha1.ToolchainClusterCondition) (toolchainv1alpha1.ToolchainCluster, error) {
	a.T.Logf("waiting for ToolchainCluster for cluster type '%s' in namespace '%s'", clusterType, namespace)
	timeout := a.Timeout
	if condition != nil {
		timeout = ToolchainClusterConditionTimeout
	}
	var c toolchainv1alpha1.ToolchainCluster
	err := wait.Poll(a.RetryInterval, timeout, func() (done bool, err error) {
		var ready bool
		if c, ready, err = a.GetToolchainCluster(clusterType, namespace, condition); ready {
			return true, nil
		}
		return false, err
	})
	return c, err
}

// WaitForNamedToolchainClusterWithCondition waits until there is a ToolchainCluster with the given name
// and with the given ClusterCondition (if it the condition is nil, then it skips this check)
func (a *Awaitility) WaitForNamedToolchainClusterWithCondition(name string, condition *toolchainv1alpha1.ToolchainClusterCondition) (toolchainv1alpha1.ToolchainCluster, error) {
	a.T.Logf("waiting for ToolchainCluster '%s' in namespace '%s' to have condition '%v'", name, a.Namespace, condition)
	timeout := a.Timeout
	if condition != nil {
		timeout = ToolchainClusterConditionTimeout
	}
	c := toolchainv1alpha1.ToolchainCluster{}
	err := wait.Poll(a.RetryInterval, timeout, func() (done bool, err error) {
		c = toolchainv1alpha1.ToolchainCluster{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Namespace: a.Namespace, Name: name}, &c); err != nil {
			return false, err
		}
		if containsClusterCondition(c.Status.Conditions, condition) {
			return true, nil
		}
		return false, err
	})
	return c, err
}

// GetToolchainCluster retrieves and returns a ToolchainCluster representing a operator of the given type
// and running in the given expected namespace. If the given condition is not nil, then it also checks
// if the CR has the ClusterCondition
func (a *Awaitility) GetToolchainCluster(clusterType cluster.Type, namespace string, condition *toolchainv1alpha1.ToolchainClusterCondition) (toolchainv1alpha1.ToolchainCluster, bool, error) {
	clusters := &toolchainv1alpha1.ToolchainClusterList{}
	if err := a.Client.List(context.TODO(), clusters, client.InNamespace(a.Namespace), client.MatchingLabels{
		"namespace": namespace,
		"type":      string(clusterType),
	}); err != nil {
		return toolchainv1alpha1.ToolchainCluster{}, false, err
	}
	if len(clusters.Items) == 0 {
		a.T.Logf("no toolchaincluster resource with expected labels: namespace='%s', type='%s'", namespace, string(clusterType))
	}
	// assume there is zero or 1 match only
	for _, cl := range clusters.Items {
		if containsClusterCondition(cl.Status.Conditions, condition) {
			return cl, true, nil
		}
	}
	return toolchainv1alpha1.ToolchainCluster{}, false, nil
}

func containsClusterCondition(conditions []toolchainv1alpha1.ToolchainClusterCondition, contains *toolchainv1alpha1.ToolchainClusterCondition) bool {
	if contains == nil {
		return true
	}
	for _, c := range conditions {
		if c.Type == contains.Type {
			return contains.Status == c.Status
		}
	}
	return false
}

// SetupRouteForService if needed, creates a route for the given service (with the same namespace/name)
// It waits until the route is available (or returns an error) by first checking the resource status
// and then making a call to the given endpoint
func (a *Awaitility) SetupRouteForService(serviceName, endpoint string) (routev1.Route, error) {
	a.T.Logf("setting up route for service '%s' with endpoint '%s'", serviceName, endpoint)
	service, err := a.WaitForMetricsService(serviceName)
	if err != nil {
		return routev1.Route{}, err
	}

	// now, create the route for the service (if needed)
	route := routev1.Route{}
	if err := a.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}, &route); err != nil {
		require.True(a.T, apierrors.IsNotFound(err), "failed to get route to access the '%s' service: %s", service.Name, err.Error())
		route = routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: service.Namespace,
				Name:      service.Name,
			},
			Spec: routev1.RouteSpec{
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString("https"),
				},
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				},
				To: routev1.RouteTargetReference{
					Kind: service.Kind,
					Name: service.Name,
				},
			},
		}
		if err = a.Client.Create(context.TODO(), &route); err != nil {
			return route, err
		}
	}
	return a.WaitForRouteToBeAvailable(route.Namespace, route.Name, endpoint)
}

// WaitForRouteToBeAvailable wais until the given route is available, ie, it has an Ingress with a host configured
// and the endpoint is reachable (with a `200 OK` status response)
func (a *Awaitility) WaitForRouteToBeAvailable(ns, name, endpoint string) (routev1.Route, error) {
	a.T.Logf("waiting for route '%s' in namespace '%s'", name, ns)
	route := routev1.Route{}
	// retrieve the route for the registration service
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		if err = a.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: ns,
				Name:      name,
			}, &route); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		// assume there's a single Ingress and that its host will not be empty when the route is ready
		if len(route.Status.Ingress) == 0 || route.Status.Ingress[0].Host == "" {
			return false, nil
		}
		// verify that the endpoint gives a `200 OK` response on a GET request
		client := http.Client{
			Timeout: time.Duration(5 * time.Second), // because sometimes the network connection may be a bit slow
		}
		var request *http.Request

		if route.Spec.TLS != nil {
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // nolint:gosec
				},
			}
			request, err = http.NewRequest("Get", "https://"+route.Status.Ingress[0].Host+endpoint, nil)
			if err != nil {
				return false, err
			}
			request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", a.RestConfig.BearerToken))

		} else {
			request, err = http.NewRequest("Get", "http://"+route.Status.Ingress[0].Host+endpoint, nil)
			if err != nil {
				return false, err
			}
		}
		resp, err := client.Do(request)
		urlError := &url.Error{}
		if errors.As(err, &urlError) && urlError.Timeout() {
			// keep waiting if there was a timeout: the endpoint is not available yet (pod is still re-starting)
			return false, nil
		} else if err != nil {
			return false, err
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return false, nil
		}
		return true, nil
	})
	return route, err
}

// GetMetricValue gets the value of the metric with the given family and label key-value pair
// fails if the metric with the given labelAndValues does not exist
func (a *Awaitility) GetMetricValue(family string, labelAndValues ...string) float64 {
	value, err := metrics.GetMetricValue(a.RestConfig, a.MetricsURL, family, labelAndValues)
	require.NoError(a.T, err)
	return value
}

// GetMetricValue gets the value of the metric with the given family and label key-value pair
// return 0 if the metric with the given labelAndValues does not exist
func (a *Awaitility) GetMetricValueOrZero(family string, labelAndValues ...string) float64 {
	if len(labelAndValues)%2 != 0 {
		a.T.Fatal("`labelAndValues` must be pairs of labels and values")
	}
	if value, err := metrics.GetMetricValue(a.RestConfig, a.MetricsURL, family, labelAndValues); err == nil {
		return value
	}
	return 0
}

// WaitUntiltMetricHasValue asserts that the exposed metric with the given family
// and label key-value pair reaches the expected value
func (a *Awaitility) WaitUntiltMetricHasValue(family string, expectedValue float64, labels ...string) {
	a.T.Logf("waiting for metric '%s{%v}' to reach '%v'", family, labels, expectedValue)
	var value float64
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		value, err := metrics.GetMetricValue(a.RestConfig, a.MetricsURL, family, labels)
		// if error occurred, ignore and return `false` to keep waiting (may be due to endpoint temporarily unavailable)
		// unless the expected value is `0`, in which case the metric is bot exposed (value==0 and err!= nil), but it's fine too.
		return (value == expectedValue && err == nil) || (expectedValue == 0 && value == 0), nil
	})
	require.NoError(a.T, err, "waited for metric '%s{%v}' to reach '%v'. Current value: %v", family, labels, expectedValue, value)
}

// WaitUntilMetricHasValueOrMore waits until the exposed metric with the given family
// and label key-value pair has reached the expected value (or more)
func (a *Awaitility) WaitUntilMetricHasValueOrMore(family string, expectedValue float64, labels ...string) error {
	a.T.Logf("waiting for metric '%s{%v}' to reach '%v' or more", family, labels, expectedValue)
	var value float64
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		value, err = metrics.GetMetricValue(a.RestConfig, a.MetricsURL, family, labels)
		// if error occurred, return `false` to keep waiting (may be due to endpoint temporarily unavailable)
		return value >= expectedValue && err == nil, nil
	})
	if err != nil {
		a.T.Logf("waited for metric '%s{%v}' to reach '%v' or more. Current value: %v", family, labels, expectedValue, value)
	}
	return err
}

// WaitUntilMetricHasValueOrLess waits until the exposed metric with the given family
// and label key-value pair has reached the expected value (or less)
func (a *Awaitility) WaitUntilMetricHasValueOrLess(family string, expectedValue float64, labels ...string) error {
	a.T.Logf("waiting for metric '%s{%v}' to reach '%v' or less", family, labels, expectedValue)
	var value float64
	err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		value, err = metrics.GetMetricValue(a.RestConfig, a.MetricsURL, family, labels)
		// if error occurred, return `false` to keep waiting (may be due to endpoint temporarily unavailable)
		return value <= expectedValue && err == nil, nil
	})
	if err != nil {
		a.T.Logf("waited for metric '%s{%v}' to reach '%v' or less. Current value: %v", family, labels, expectedValue, value)
	}
	return err
}

// DeletePods deletes the pods matching the given criteria
func (a *Awaitility) DeletePods(criteria ...client.ListOption) error {
	pods := corev1.PodList{}
	err := a.Client.List(context.TODO(), &pods, criteria...)
	if err != nil {
		return err
	}
	for _, p := range pods.Items {
		if err := a.Client.Delete(context.TODO(), &p); err != nil { // nolint:gosec
			return err
		}
	}
	return nil
}

// GetMemoryUsage retrieves the memory usage (in KB) of a given the pod
func (a *Awaitility) GetMemoryUsage(podname, ns string) (int64, error) {
	var containerMetrics k8smetrics.ContainerMetrics
	if err := wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		podMetrics := k8smetrics.PodMetrics{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: ns,
			Name:      podname,
		}, &podMetrics); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		for _, c := range podMetrics.Containers {
			if c.Name == "manager" {
				containerMetrics = c
				return true, nil
			}
		}
		return false, nil // keep waiting
	}); err != nil {
		return -1, err
	}
	// the pod contains multiple
	return containerMetrics.Usage.Memory().ScaledValue(resource.Kilo), nil
}

// CreateNamespace creates a namespace with the given name and waits until it gets active
// it also adds a deletion of the namespace at the end of the test
func (a *Awaitility) CreateNamespace(name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := a.Client.Create(context.TODO(), ns)
	require.NoError(a.T, err)
	err = wait.Poll(a.RetryInterval, a.Timeout, func() (done bool, err error) {
		ns := &corev1.Namespace{}
		if err := a.Client.Get(context.TODO(), types.NamespacedName{Name: name}, ns); err != nil && apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return ns.Status.Phase == corev1.NamespaceActive, nil
	})
	require.NoError(a.T, err)
	a.T.Cleanup(func() {
		if err := a.Client.Delete(context.TODO(), ns); err != nil && !apierrors.IsNotFound(err) {
			require.NoError(a.T, err)
		}
	})
}

// WaitForDeploymentToGetReady waits until the deployment with the given name is ready together with the given number of replicas
func (a *Awaitility) WaitForDeploymentToGetReady(name string, replicas int, criteria ...DeploymentCriteria) *appsv1.Deployment {
	a.T.Logf("waiting until deployment '%s' in namespace '%s' is ready", name, a.Namespace)
	deployment := &appsv1.Deployment{}
	err := wait.Poll(a.RetryInterval, 6*a.Timeout, func() (done bool, err error) {
		deploymentConditions := status.GetDeploymentStatusConditions(a.Client, name, a.Namespace)
		if err := status.ValidateComponentConditionReady(deploymentConditions...); err != nil {
			return false, nil // nolint:nilerr
		}
		deployment = &appsv1.Deployment{}
		require.NoError(a.T, a.Client.Get(context.TODO(), test.NamespacedName(a.Namespace, name), deployment))
		if int(deployment.Status.AvailableReplicas) != replicas {
			return false, nil
		}
		pods := &corev1.PodList{}
		require.NoError(a.T, a.Client.List(context.TODO(), pods, client.InNamespace(a.Namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)))
		if len(pods.Items) != replicas {
			return false, nil
		}
		for _, pod := range pods.Items { // nolint
			if util.IsBeingDeleted(&pod) || !podutils.IsPodReady(&pod) {
				return false, nil
			}
		}
		for _, criteriaMatch := range criteria {
			if !criteriaMatch(deployment) {
				return false, nil
			}
		}
		return true, nil
	})
	require.NoError(a.T, err)
	return deployment
}

type DeploymentCriteria func(*appsv1.Deployment) bool

func DeploymentHasContainerWithImage(containerName, image string) DeploymentCriteria {
	return func(deployment *appsv1.Deployment) bool {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == containerName && container.Image == image {
				return true
			}
		}
		return false
	}
}

// CreateWithCleanup creates the given object via client.Client.Create() and schedules the cleanup of the object at the end of the current test
func (a *Awaitility) CreateWithCleanup(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if err := a.Client.Create(ctx, obj, opts...); err != nil {
		return err
	}
	cleanup.AddCleanTasks(a, obj)
	return nil
}

// Clean triggers cleanup of all resources that were marked to be cleaned before that
func (a *Awaitility) Clean() {
	cleanup.ExecuteAllCleanTasks(a)
}

func (a *Awaitility) listAndPrint(resourceKind, namespace string, list client.ObjectList, additionalOptions ...client.ListOption) {
	a.T.Logf(a.listAndReturnContent(resourceKind, namespace, list, additionalOptions...))
}

func (a *Awaitility) listAndReturnContent(resourceKind, namespace string, list client.ObjectList, additionalOptions ...client.ListOption) string {
	listOptions := additionalOptions
	if a.Namespace != "" {
		listOptions = append(additionalOptions, client.InNamespace(namespace))
	}
	if err := a.Client.List(context.TODO(), list, listOptions...); err != nil {
		return fmt.Sprintf("unable to list %s: %s", resourceKind, err)
	}
	content, _ := StringifyObjects(list)
	return fmt.Sprintf("\n%s present in the namespace:\n%s\n", resourceKind, string(content))
}
