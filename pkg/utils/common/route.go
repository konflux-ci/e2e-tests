package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// GetOpenshiftRoute returns the route for a given component name
func (h *SuiteController) GetOpenshiftRoute(routeName string, routeNamespace string) (*routev1.Route, error) {
	namespacedName := types.NamespacedName{
		Name:      routeName,
		Namespace: routeNamespace,
	}

	route := &routev1.Route{}
	err := h.KubeRest().Get(context.TODO(), namespacedName, route)
	if err != nil {
		return &routev1.Route{}, err
	}
	return route, nil
}

// GetOpenshiftRouteByComponentName returns a route associated with the given component
// Routes that belong to a given component will have the following label: 'app.kubernetes.io/name: <component-name>'
func (h *SuiteController) GetOpenshiftRouteByComponentName(componentName string, componentNamespace string) (*routev1.Route, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", componentName),
	}
	routeList, err := h.CustomClient.RouteClient().RouteV1().Routes(componentNamespace).List(context.Background(), listOptions)
	if err != nil {
		return &routev1.Route{}, err
	}
	if len(routeList.Items) == 0 {
		return &routev1.Route{}, fmt.Errorf("unable to find routes with label %v:%v", "app.kubernetes.io/name", componentName)
	}
	return &routeList.Items[0], nil
}

func (h *SuiteController) RouteHostnameIsAccessible(routeName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		namespacedName := types.NamespacedName{
			Name:      routeName,
			Namespace: namespace,
		}
		route := &routev1.Route{}
		if err := h.KubeRest().Get(context.TODO(), namespacedName, route); err != nil {
			return false, nil
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		client := http.Client{Transport: tr}
		res, err := client.Get("https://" + route.Spec.Host)
		if err != nil || res.StatusCode > 299 {
			return false, nil
		}

		return true, nil
	}
}
