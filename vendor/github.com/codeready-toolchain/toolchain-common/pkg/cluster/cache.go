package cluster

import (
	"sync"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var clusterCache = toolchainClusterClients{clusters: map[string]*CachedToolchainCluster{}}

type toolchainClusterClients struct {
	sync.RWMutex
	clusters     map[string]*CachedToolchainCluster
	refreshCache func()
}

type Config struct {
	// RestConfig contains rest config data
	RestConfig *rest.Config
	// Name is the name of the cluster. Has to be unique - is used as a key in a map.
	Name string
	// APIEndpoint is the API endpoint of the corresponding ToolchainCluster. This can be a hostname,
	// hostname:port, IP or IP:port.
	APIEndpoint string
	// Type is a type of the cluster (either host or member)
	Type Type
	// OperatorNamespace is a name of a namespace (in the cluster) the operator is running in
	OperatorNamespace string
	// OwnerClusterName keeps the name of the cluster the ToolchainCluster resource is created in
	// eg. if this ToolchainCluster identifies a Host cluster (and thus is created in Member)
	// then the OwnerClusterName has a name of the member - it has to be same name as the name
	// that is used for identifying the member in a Host cluster
	OwnerClusterName string
}

// CachedToolchainCluster stores cluster client; cluster related info and previous health check probe results
type CachedToolchainCluster struct {
	*Config
	// Client is the kube client for the cluster.
	Client client.Client
	// ClusterStatus is the cluster result as of the last health check probe.
	ClusterStatus *toolchainv1alpha1.ToolchainClusterStatus
}

func (c *toolchainClusterClients) addCachedToolchainCluster(cluster *CachedToolchainCluster) {
	c.Lock()
	defer c.Unlock()
	c.clusters[cluster.Name] = cluster
}

func (c *toolchainClusterClients) deleteCachedToolchainCluster(name string) {
	c.Lock()
	defer c.Unlock()
	delete(c.clusters, name)
}

func (c *toolchainClusterClients) getCachedToolchainCluster(name string, canRefreshCache bool) (*CachedToolchainCluster, bool) {
	c.RLock()
	defer c.RUnlock()
	_, ok := c.clusters[name]
	if !ok && canRefreshCache && c.refreshCache != nil {
		c.RUnlock()
		c.refreshCache()
		c.RLock()
	}
	cluster, ok := c.clusters[name]
	return cluster, ok
}

// Condition an expected cluster condition
type Condition func(cluster *CachedToolchainCluster) bool

// Ready checks that the cluster is in a 'Ready' status condition
var Ready Condition = func(cluster *CachedToolchainCluster) bool {
	return IsReady(cluster.ClusterStatus)
}

func (c *toolchainClusterClients) getCachedToolchainClustersByType(clusterType Type, conditions ...Condition) []*CachedToolchainCluster {
	c.RLock()
	defer c.RUnlock()
	return Filter(clusterType, c.clusters, conditions...)
}
func Filter(clusterType Type, clusters map[string]*CachedToolchainCluster, conditions ...Condition) []*CachedToolchainCluster {
	filteredClusters := make([]*CachedToolchainCluster, 0, len(clusters))
clusters:
	for _, cluster := range clusters {
		if cluster.Type == clusterType {
			for _, match := range conditions {
				if !match(cluster) {
					continue clusters
				}
			}
			filteredClusters = append(filteredClusters, cluster)
		}
	}
	return filteredClusters
}

// GetCachedToolchainCluster returns a kube client for the cluster (with the given name) and info if the client exists
func GetCachedToolchainCluster(name string) (*CachedToolchainCluster, bool) {
	return clusterCache.getCachedToolchainCluster(name, true)
}

// GetHostClusterFunc a func that returns the Host cluster from the cache,
// along with a bool to indicate if there was a match or not
type GetHostClusterFunc func() (*CachedToolchainCluster, bool)

// HostCluster the func to retrieve the host cluster
var HostCluster GetHostClusterFunc = GetHostCluster

// GetHostCluster returns the kube client for the host cluster from the cache of the clusters
// and info if such a client exists
func GetHostCluster() (*CachedToolchainCluster, bool) {
	clusters := clusterCache.getCachedToolchainClustersByType(Host)
	if len(clusters) == 0 {
		if clusterCache.refreshCache != nil {
			clusterCache.refreshCache()
		}
		clusters = clusterCache.getCachedToolchainClustersByType(Host)
		if len(clusters) == 0 {
			return nil, false
		}
	}
	return clusters[0], true
}

// GetMemberClustersFunc a func that returns the member clusters from the cache
type GetMemberClustersFunc func(conditions ...Condition) []*CachedToolchainCluster

// MemberClusters the func to retrieve the member clusters
var MemberClusters GetMemberClustersFunc = GetMemberClusters

// GetMemberClusters returns the kube clients for the host clusters from the cache of the clusters
func GetMemberClusters(conditions ...Condition) []*CachedToolchainCluster {
	clusters := clusterCache.getCachedToolchainClustersByType(Member, conditions...)
	if len(clusters) == 0 {
		if clusterCache.refreshCache != nil {
			clusterCache.refreshCache()
		}
		clusters = clusterCache.getCachedToolchainClustersByType(Member, conditions...)
	}
	return clusters
}

// Type is a cluster type (either host or member)
type Type string

const (
	Member Type = "member"
	Host   Type = "host"
)
