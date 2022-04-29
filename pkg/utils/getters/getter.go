package getters

import (
	"context"
	"fmt"
	"time"

	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type Result string

const (
	QUERY_OS_KUBE_API_MEMORY string = `sum(container_memory_usage_bytes{namespace="openshift-kube-apiserver", pod=~"kube-apiserver-.*"})`
	QUERY_ETCD_MEMORY string = `process_resident_memory_bytes{job="etcd"}`
	QUERY_CLUSTER_CPU string = `1 - avg(rate(node_cpu_seconds_total{mode="idle", cluster=""}[5m]))`
	QUERY_CLUSTER_MEMORY string = `1 - sum(:node_memory_MemAvailable_bytes:sum{cluster=""}) / sum(node_memory_MemTotal_bytes{cluster=""})`
	QUERY_NODE_MEMORY string = `1 - sum (node_memory_MemAvailable_bytes * on(instance) (group by(instance)(label_replace(kube_node_role{role="master"}, "instance", "$1", "node", "(.*)"))))/
	sum (node_memory_MemTotal_bytes * on(instance) (group by(instance)(label_replace(kube_node_role{role="master"}, "instance", "$1", "node", "(.*)"))))`
	Mem     Result = "memory"
	Percent Result = "percentage"
	Sim     Result = "simple"
)

type Getter interface {
	Name() string
	Execute() (model.Value, prometheus.Warnings, error)
	Result() string
}

type BaseGetter struct {
	apiClient  prometheus.API
	name       string
	query      string
	result Result
}

func (b BaseGetter) Name() string {
	return b.name
}

func (b *BaseGetter) Execute() (model.Value, prometheus.Warnings, error) {
	return b.apiClient.Query(context.TODO(), b.query, time.Now())
}

func (b BaseGetter) Result() string {
	return string(b.result)
}

func GetOpenshiftKubeAPIMemoryUtilisation(apiClient prometheus.API) *BaseGetter {
	return &BaseGetter{
		apiClient:  apiClient,
		name:       "openshift-kube-apiserver",
		query:      QUERY_OS_KUBE_API_MEMORY,
		result: Mem,
	}
}

func GetEtcdMemoryUsage(apiClient prometheus.API) *BaseGetter {
	return &BaseGetter{
		apiClient:  apiClient,
		name:       "etcd Instance Memory Usage",
		query:      QUERY_ETCD_MEMORY,
		result: Mem,
	}
}

func GetClusterCPUUtilisation(apiClient prometheus.API) *BaseGetter {
	return &BaseGetter{
		apiClient:  apiClient,
		name:       "Cluster CPU Utilisation",
		query:      QUERY_CLUSTER_CPU,
		result: Percent,
	}
}

func GetClusterMemoryUtilisation(apiClient prometheus.API) *BaseGetter {
	return &BaseGetter{
		apiClient:  apiClient,
		name:       "Cluster Memory Utilisation",
		query:      QUERY_CLUSTER_MEMORY,
		result: Percent,
	}
}

func GetWorkloadCPUUsage(apiClient prometheus.API, namespace, name string) *BaseGetter {
	query := fmt.Sprintf(`sum(
		node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{cluster="", namespace="%[1]s"}
	  * on(namespace,pod)
		group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="", namespace="%[1]s", workload="%[2]s", workload_type="deployment"}
	) by (pod)`, namespace, name)
	return &BaseGetter{
		apiClient:  apiClient,
		name:       fmt.Sprintf("%s CPU Usage", name),
		query:      query,
		result: Sim,
	}
}

func GetWorkloadMemoryUsage(apiClient prometheus.API, namespace, name string) *BaseGetter {
	query := fmt.Sprintf(`sum(
		container_memory_working_set_bytes{cluster="", namespace="%[1]s", container!="", image!=""}
	  * on(namespace,pod)
		group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="", namespace="%[1]s", workload="%[2]s", workload_type="deployment"}
	) by (pod)`, namespace, name)
	return &BaseGetter{
		apiClient:  apiClient,
		name:       fmt.Sprintf("%s Memory Usage", name),
		query:      query,
		result: Mem,
	}
}

func GetNodeMemoryUtilisation(apiClient prometheus.API) *BaseGetter {
	return &BaseGetter{
		apiClient:  apiClient,
		name:       "Node Memory Usage",
		query:      QUERY_NODE_MEMORY,
		result: Percent,
	}
}