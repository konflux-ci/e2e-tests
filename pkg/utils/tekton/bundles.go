package tekton

import (
	"context"

	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
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
	err := t.KubeRest().Get(context.TODO(), namespacedName, pipelineSelector)
	if err != nil {
		return nil, err
	}
	for _, selector := range pipelineSelector.Spec.Selectors {
		bundleName := selector.PipelineRef.Name
		bundleRef := selector.PipelineRef.Bundle
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
