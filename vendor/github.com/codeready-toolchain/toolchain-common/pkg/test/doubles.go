package test

import "k8s.io/apimachinery/pkg/types"

const (
	HostClusterName    = "host-cluster"
	MemberOperatorNs   = "toolchain-member-operator"
	MemberClusterName  = "member-cluster"
	Member2ClusterName = "member2-cluster"
	HostOperatorNs     = "toolchain-host-operator"
)

func NamespacedName(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}
