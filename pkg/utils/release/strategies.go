package release

import (
	"context"

	"github.com/redhat-appstudio/release-service/tekton/utils"

	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StrategyConfig struct {
	Mapping Mapping `json:"mapping"`
}
type Mapping struct {
	Components []Component `json:"components"`
}
type Component struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}

// CreateReleaseStrategy creates a new ReleaseStrategy using the given parameters.
func (r *ReleaseController) CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string, serviceAccount string, params []releaseApi.Params) (*releaseApi.ReleaseStrategy, error) {
	releaseStrategy := &releaseApi.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseStrategySpec{
			PipelineRef: utils.PipelineRef{
				Resolver: "bundles",
				Params: []utils.Param{
					{Name: "bundle", Value: bundle},
					{Name: "name", Value: pipelineName},
					{Name: "kind", Value: "pipeline"},
				},
			},
			Policy:         policy,
			Params:         params,
			ServiceAccount: serviceAccount,
		},
	}

	return releaseStrategy, r.KubeRest().Create(context.TODO(), releaseStrategy)
}

// GenerateReleaseStrategyConfig generates release strategy config.
func (r *ReleaseController) GenerateReleaseStrategyConfig(components []Component) *StrategyConfig {
	return &StrategyConfig{
		Mapping{Components: components},
	}
}

// DeleteReleaseStrategy deletes ReleaseStrategy.
func (r *ReleaseController) DeleteReleaseStrategy(name, namespace string, failOnNotFound bool) error {
	releasePlan := &releaseApi.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := r.KubeRest().Delete(context.TODO(), releasePlan)
	if err != nil && !failOnNotFound && k8sErrors.IsNotFound(err) {
		err = nil
	}
	return err
}
