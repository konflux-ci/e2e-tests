package cluster

import (
	"context"
	"encoding/base64"
	"reflect"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelType             = "type"
	labelNamespace        = "namespace"
	labelOwnerClusterName = "ownerClusterName"

	defaultHostOperatorNamespace   = "toolchain-host-operator"
	defaultMemberOperatorNamespace = "toolchain-member-operator"

	toolchainAPIQPS   = 20.0
	toolchainAPIBurst = 30
	toolchainTokenKey = "token"
)

// ToolchainClusterService manages cached cluster kube clients and related ToolchainCluster CRDs
// it's used for adding/updating/deleting
type ToolchainClusterService struct {
	client    client.Client
	log       logr.Logger
	namespace string
	timeout   time.Duration
}

// NewToolchainClusterService creates a new instance of ToolchainClusterService object and assigns the refreshCache function to the cache instance
func NewToolchainClusterService(client client.Client, log logr.Logger, namespace string, timeout time.Duration) ToolchainClusterService {
	service := ToolchainClusterService{
		client:    client,
		log:       log,
		namespace: namespace,
		timeout:   timeout,
	}
	clusterCache.refreshCache = service.refreshCache
	return service
}

// AddOrUpdateToolchainCluster takes the ToolchainCluster CR object,
// creates CachedToolchainCluster with a kube client and stores it in a cache
func (s *ToolchainClusterService) AddOrUpdateToolchainCluster(cluster *toolchainv1alpha1.ToolchainCluster) error {
	log := s.enrichLogger(cluster)
	log.Info("observed a cluster")

	err := s.addToolchainCluster(log, cluster)
	if err != nil {
		return errors.Wrap(err, "the cluster was not added nor updated")
	}
	return nil
}

func (s *ToolchainClusterService) addToolchainCluster(log logr.Logger, toolchainCluster *toolchainv1alpha1.ToolchainCluster) error {
	// create the restclient of toolchainCluster
	clusterConfig, err := NewClusterConfig(s.client, toolchainCluster, s.timeout)
	if err != nil {
		return errors.Wrap(err, "cannot create ToolchainCluster Config")
	}

	var cl client.Client
	// check if there is already a cached ToolchainCluster so we could reuse the client
	// we cannot allow to refresh the cache, because the refresh function calls this addToolchainCluster method which results in a recursive loop
	cachedToolchainCluster, exists := clusterCache.getCachedToolchainCluster(toolchainCluster.Name, false)
	if !exists ||
		cachedToolchainCluster.Client == nil ||
		!reflect.DeepEqual(clusterConfig.RestConfig, cachedToolchainCluster.RestConfig) {

		log.Info("creating new client for the cached ToolchainCluster")
		scheme := runtime.NewScheme()
		if err := toolchainv1alpha1.AddToScheme(scheme); err != nil {
			return err
		}
		if err := v1.AddToScheme(scheme); err != nil {
			return err
		}
		cl, err = client.New(clusterConfig.RestConfig, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			return errors.Wrap(err, "cannot create ToolchainCluster client")
		}
	} else {
		log.Info("reusing the client for the cached ToolchainCluster")
		cl = cachedToolchainCluster.Client
	}

	cluster := &CachedToolchainCluster{
		Config:        clusterConfig,
		Client:        cl,
		ClusterStatus: &toolchainCluster.Status,
	}
	if cluster.Type == "" {
		cluster.Type = Member
	}
	if cluster.OperatorNamespace == "" {
		if cluster.Type == Host {
			cluster.OperatorNamespace = defaultHostOperatorNamespace
		} else {
			cluster.OperatorNamespace = defaultMemberOperatorNamespace
		}
	}
	clusterCache.addCachedToolchainCluster(cluster)
	return nil
}

// DeleteToolchainCluster takes the ToolchainCluster CR object
// and deletes CachedToolchainCluster instance that has same name from a cache (if exists)
func (s *ToolchainClusterService) DeleteToolchainCluster(name string) {
	s.log.WithValues("Request.Name", name).Info("observed a deleted cluster")
	clusterCache.deleteCachedToolchainCluster(name)
}

func (s *ToolchainClusterService) refreshCache() {
	toolchainClusters := &toolchainv1alpha1.ToolchainClusterList{}
	if err := s.client.List(context.TODO(), toolchainClusters, &client.ListOptions{Namespace: s.namespace}); err != nil {
		s.log.Error(err, "the cluster cache was not refreshed")
	}
	for i := range toolchainClusters.Items {
		cluster := toolchainClusters.Items[i] // avoids the `G601: Implicit memory aliasing in for loop` problem
		log := s.enrichLogger(&cluster)
		err := s.addToolchainCluster(log, &cluster)
		if err != nil {
			log.Error(err, "the cluster was not added", "cluster", cluster)
		}
	}
}

func (s *ToolchainClusterService) enrichLogger(cluster *toolchainv1alpha1.ToolchainCluster) logr.Logger {
	return s.log.
		WithValues("Request.Namespace", cluster.Namespace, "Request.Name", cluster.Name)
}

// NewClusterConfig generate a new cluster config by fetching the necessary info the given ToolchainCluster's associated Secret and taking all data from ToolchainCluster CR
func NewClusterConfig(cl client.Client, toolchainCluster *toolchainv1alpha1.ToolchainCluster, timeout time.Duration) (*Config, error) {
	clusterName := toolchainCluster.Name

	apiEndpoint := toolchainCluster.Spec.APIEndpoint
	if apiEndpoint == "" {
		return nil, errors.Errorf("the api endpoint of cluster %s is empty", clusterName)
	}

	secretName := toolchainCluster.Spec.SecretRef.Name
	if secretName == "" {
		return nil, errors.Errorf("cluster %s does not have a secret name", clusterName)
	}
	secret := &v1.Secret{}
	name := types.NamespacedName{
		Namespace: toolchainCluster.Namespace,
		Name:      secretName,
	}
	err := cl.Get(context.TODO(), name, secret)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get secret %s for cluster %s", name, clusterName)
	}

	token, tokenFound := secret.Data[toolchainTokenKey]
	if !tokenFound || len(token) == 0 {
		return nil, errors.Errorf("the secret for cluster %s is missing a non-empty value for %q", clusterName, toolchainTokenKey)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags(apiEndpoint, "")
	if err != nil {
		return nil, err
	}

	ca, err := base64.StdEncoding.DecodeString(toolchainCluster.Spec.CABundle)
	if err != nil {
		return nil, err
	}
	restConfig.CAData = ca
	restConfig.BearerToken = string(token)
	restConfig.QPS = toolchainAPIQPS
	restConfig.Burst = toolchainAPIBurst
	restConfig.Timeout = timeout

	return &Config{
		Name:              toolchainCluster.Name,
		APIEndpoint:       toolchainCluster.Spec.APIEndpoint,
		RestConfig:        restConfig,
		Type:              Type(toolchainCluster.Labels[labelType]),
		OperatorNamespace: toolchainCluster.Labels[labelNamespace],
		OwnerClusterName:  toolchainCluster.Labels[labelOwnerClusterName],
	}, nil
}

func IsReady(clusterStatus *toolchainv1alpha1.ToolchainClusterStatus) bool {
	for _, condition := range clusterStatus.Conditions {
		if condition.Type == toolchainv1alpha1.ToolchainClusterReady {
			if condition.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func ListToolchainClusterConfigs(cl client.Client, namespace string, clusterType Type, timeout time.Duration) ([]*Config, error) {
	toolchainClusters := &toolchainv1alpha1.ToolchainClusterList{}
	if err := cl.List(context.TODO(), toolchainClusters, client.InNamespace(namespace), client.MatchingLabels{labelType: string(clusterType)}); err != nil {
		return nil, err
	}
	var configs []*Config
	for _, cluster := range toolchainClusters.Items {
		clusterConfig, err := NewClusterConfig(cl, &cluster, timeout) // nolint:gosec
		if err != nil {
			return nil, err
		}
		configs = append(configs, clusterConfig)
	}
	return configs, nil
}
