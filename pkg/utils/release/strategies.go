package release

import (
	"context"

	releaseApi "github.com/redhat-appstudio/release-service/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StrategiesInterface interface {
	// Creates a release strategy.
	CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string, serviceAccount string, params []releaseApi.Params) (*releaseApi.ReleaseStrategy, error)

	// Generates a release strategy config.
	GenerateReleaseStrategyConfig(components []Component) *StrategyConfig
}

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
func (r *releaseFactory) CreateReleaseStrategy(name, namespace, pipelineName, bundle string, policy string, serviceAccount string, params []releaseApi.Params) (*releaseApi.ReleaseStrategy, error) {
	releaseStrategy := &releaseApi.ReleaseStrategy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: releaseApi.ReleaseStrategySpec{
			Pipeline:       pipelineName,
			Bundle:         bundle,
			Policy:         policy,
			Params:         params,
			ServiceAccount: serviceAccount,
		},
	}

	return releaseStrategy, r.KubeRest().Create(context.TODO(), releaseStrategy)
}

// GenerateReleaseStrategyConfig generates release strategy config.
func (r *releaseFactory) GenerateReleaseStrategyConfig(components []Component) *StrategyConfig {
	return &StrategyConfig{
		Mapping{Components: components},
	}
}
