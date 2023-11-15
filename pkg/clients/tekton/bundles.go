package tekton

import (
	"context"

	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	"k8s.io/apimachinery/pkg/types"
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
		Name:      "build-pipeline-selector",
		Namespace: "build-service",
	}
	bundles := &Bundles{}
	pipelineSelector := &buildservice.BuildPipelineSelector{}
	err := t.KubeRest().Get(context.Background(), namespacedName, pipelineSelector)
	if err != nil {
		return nil, err
	}
	for i := range pipelineSelector.Spec.Selectors {
		selector := &pipelineSelector.Spec.Selectors[i]
		bundleName, bundleRef := tekton.GetPipelineNameAndBundleRef(&selector.PipelineRef)
		switch bundleName {
		case "docker-build":
			bundles.DockerBuildBundle = bundleRef
		case "fbc-builder":
			bundles.FBCBuilderBundle = bundleRef
		case "java-builder":
			bundles.JavaBuilderBundle = bundleRef
		case "nodejs-builder":
			bundles.NodeJSBuilderBundle = bundleRef
		}
	}
	return bundles, nil
}
