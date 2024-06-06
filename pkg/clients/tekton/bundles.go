package tekton

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
)

type Bundles struct {
	FBCBuilderBundle    string
	DockerBuildBundle   string
	JavaBuilderBundle   string
	NodeJSBuilderBundle string
}

// NewBundles returns new Bundles.
func (t *TektonController) NewBundles() (*Bundles, error) {
	namespacedName := types.NamespacedName{
		Name:      "build-pipeline-config",
		Namespace: "build-service",
	}
	bundles := &Bundles{}
	configMap := &corev1.ConfigMap{}
	err := t.KubeRest().Get(context.Background(), namespacedName, configMap)
	if err != nil {
		return nil, err
	}

	bpc := &tekton.BuildPipelineConfig{}
	if err = yaml.Unmarshal([]byte(configMap.Data["config.yaml"]), bpc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build pipeline config: %v", err)
	}

	for i := range bpc.Pipelines {
		pipeline := bpc.Pipelines[i]
		switch pipeline.Name {
		case "docker-build":
			bundles.DockerBuildBundle = pipeline.Bundle
		case "fbc-builder":
			bundles.FBCBuilderBundle = pipeline.Bundle
		case "java-builder":
			bundles.JavaBuilderBundle = pipeline.Bundle
		case "nodejs-builder":
			bundles.NodeJSBuilderBundle = pipeline.Bundle
		}
	}
	return bundles, nil
}
