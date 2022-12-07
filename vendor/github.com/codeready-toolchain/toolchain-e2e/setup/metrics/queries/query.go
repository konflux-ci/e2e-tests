package queries

import (
	"context"
	"fmt"
	"time"

	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type ResultType string

const (
	Percentage ResultType = "percentage"
	Memory     ResultType = "memory"
	Simple     ResultType = "simple"
)

type Query interface {
	Name() string
	Execute() (model.Value, prometheus.Warnings, error)
	ResultType() string
}

type BaseQuery struct {
	apiClient  prometheus.API
	name       string
	query      string
	resultType ResultType
}

func (b BaseQuery) Name() string {
	return b.name
}

func (b *BaseQuery) Execute() (model.Value, prometheus.Warnings, error) {
	return b.apiClient.Query(context.TODO(), b.query, time.Now())
}

func (b BaseQuery) ResultType() string {
	return string(b.resultType)
}

func QueryOpenshiftKubeAPIMemoryUtilisation(apiClient prometheus.API) *BaseQuery {
	return &BaseQuery{
		apiClient:  apiClient,
		name:       "openshift-kube-apiserver",
		query:      `sum(container_memory_usage_bytes{namespace="openshift-kube-apiserver", pod=~"kube-apiserver-.*"})`,
		resultType: Memory,
	}
}

func QueryEtcdMemoryUsage(apiClient prometheus.API) *BaseQuery {
	return &BaseQuery{
		apiClient:  apiClient,
		name:       "etcd Instance Memory Usage",
		query:      `process_resident_memory_bytes{job="etcd"}`,
		resultType: Memory,
	}
}

func QueryClusterCPUUtilisation(apiClient prometheus.API) *BaseQuery {
	return &BaseQuery{
		apiClient:  apiClient,
		name:       "Cluster CPU Utilisation",
		query:      `1 - avg(rate(node_cpu_seconds_total{mode="idle", cluster=""}[5m]))`,
		resultType: Percentage,
	}
}

func QueryClusterMemoryUtilisation(apiClient prometheus.API) *BaseQuery {
	return &BaseQuery{
		apiClient:  apiClient,
		name:       "Cluster Memory Utilisation",
		query:      `1 - sum(:node_memory_MemAvailable_bytes:sum{cluster=""}) / sum(node_memory_MemTotal_bytes{cluster=""})`,
		resultType: Percentage,
	}
}

func QueryWorkloadCPUUsage(apiClient prometheus.API, namespace, name string) *BaseQuery {
	query := fmt.Sprintf(`sum(
		node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{cluster="", namespace="%[1]s"}
	  * on(namespace,pod)
		group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="", namespace="%[1]s", workload="%[2]s", workload_type="deployment"}
	) by (pod)`, namespace, name)
	return &BaseQuery{
		apiClient:  apiClient,
		name:       fmt.Sprintf("%s CPU Usage", name),
		query:      query,
		resultType: Simple,
	}
}

func QueryWorkloadMemoryUsage(apiClient prometheus.API, namespace, name string) *BaseQuery {
	query := fmt.Sprintf(`sum(
		container_memory_working_set_bytes{cluster="", namespace="%[1]s", container!="", image!=""}
	  * on(namespace,pod)
		group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{cluster="", namespace="%[1]s", workload="%[2]s", workload_type="deployment"}
	) by (pod)`, namespace, name)
	return &BaseQuery{
		apiClient:  apiClient,
		name:       fmt.Sprintf("%s Memory Usage", name),
		query:      query,
		resultType: Memory,
	}
}

func QueryNodeMemoryUtilisation(apiClient prometheus.API) *BaseQuery {
	query := `1 - sum (node_memory_MemAvailable_bytes * on(instance) (group by(instance)(label_replace(kube_node_role{role="master"}, "instance", "$1", "node", "(.*)"))))/
	sum (node_memory_MemTotal_bytes * on(instance) (group by(instance)(label_replace(kube_node_role{role="master"}, "instance", "$1", "node", "(.*)"))))`
	return &BaseQuery{
		apiClient:  apiClient,
		name:       "Node Memory Usage",
		query:      query,
		resultType: Percentage,
	}
}
