package common

import (
	"context"
	"fmt"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateProxyPlugin creates an object of ProxyPlugin for the OpenShift route target
func (s *SuiteController) CreateProxyPlugin(proxyPluginName, proxyPluginNamespace, routeName, routeNamespace string) (*toolchainv1alpha1.ProxyPlugin, error) {

	// Create the ProxyPlugin object
	proxyPlugin := NewProxyPlugin(proxyPluginName, proxyPluginNamespace, routeName, routeNamespace)

	if err := s.KubeRest().Create(context.Background(), proxyPlugin); err != nil {
		return nil, fmt.Errorf("unable to create proxy plugin due to %v", err)
	}
	return proxyPlugin, nil
}

// DeleteProxyPlugin deletes the ProxyPlugin object
func (s *SuiteController) DeleteProxyPlugin(proxyPluginName, proxyPluginNamespace string) (bool, error) {
	proxyPlugin := &toolchainv1alpha1.ProxyPlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyPluginName,
			Namespace: proxyPluginNamespace,
		},
	}

	if err := s.KubeRest().Delete(context.Background(), proxyPlugin); err != nil {
		return false, err
	}
	err := utils.WaitUntil(func() (done bool, err error) {
		err = s.KubeRest().Get(context.Background(), types.NamespacedName{
			Namespace: proxyPluginNamespace,
			Name:      proxyPluginName,
		}, proxyPlugin)

		if err != nil {
			if k8sErrors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("deletion of proxy plugin has been timedout:: %v", err)
		}
		return false, nil
	}, 5*time.Minute)

	if err != nil {
		return false, err
	}

	return true, nil
}

// NewProxyPlugin gives the proxyplugin resource template
func NewProxyPlugin(proxyPluginName, proxyPluginNamespace, routeName, routeNamespace string) *toolchainv1alpha1.ProxyPlugin {
	return &toolchainv1alpha1.ProxyPlugin{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: proxyPluginNamespace,
			Name:      proxyPluginName,
		},
		Spec: toolchainv1alpha1.ProxyPluginSpec{
			OpenShiftRouteTargetEndpoint: &toolchainv1alpha1.OpenShiftRouteTarget{
				Namespace: routeNamespace,
				Name:      routeName,
			},
		},
	}
}
