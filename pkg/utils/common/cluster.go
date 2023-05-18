package common

import (
	"context"

	openshiftApi "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Obtain the Openshift ingress specs
func (s *SuiteController) GetOpenshiftIngress() (ingress *openshiftApi.Ingress, err error) {
	var pene = &openshiftApi.Ingress{}
	if err := s.KubeRest().Get(context.TODO(), types.NamespacedName{Name: "cluster"}, pene); err != nil {
		return nil, err
	}

	return pene, nil
}
