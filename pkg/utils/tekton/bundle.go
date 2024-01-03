package tekton

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/tektoncd/cli/pkg/bundle"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

// ExtractTektonObjectFromBundle extracts specified Tekton object from specified bundle reference
func ExtractTektonObjectFromBundle(bundleRef, kind, name string) (runtime.Object, error) {
	var obj runtime.Object
	var err error

	resolver := oci.NewResolver(bundleRef, authn.DefaultKeychain)
	if obj, _, err = resolver.Get(context.Background(), kind, name); err != nil {
		return nil, fmt.Errorf("failed to fetch the tekton object %s with name %s: %v", kind, name, err)
	}
	return obj, nil
}

// BuildAndPushTektonBundle builds a Tekton bundle from YAML and pushes to remote container registry
func BuildAndPushTektonBundle(YamlContent []byte, ref name.Reference, remoteOption remoteimg.Option) error {
	img, err := bundle.BuildTektonBundle([]string{string(YamlContent)}, os.Stdout)
	if err != nil {
		return fmt.Errorf("error when building a bundle %s: %v", ref.String(), err)
	}

	outDigest, err := bundle.Write(img, ref, remoteOption)
	if err != nil {
		return fmt.Errorf("error when pushing a bundle %s to a container image registry repo: %v", ref.String(), err)
	}
	klog.Infof("image digest for a new tekton bundle %s: %+v", ref.String(), outDigest)

	return nil
}

// GetBundleRef returns the bundle reference from a pipelineRef
func GetBundleRef(pipelineRef *tektonv1.PipelineRef) string {
	_, bundleRef := GetPipelineNameAndBundleRef(pipelineRef)
	return bundleRef
}

// GetDefaultPipelineBundleRef gets the specific Tekton pipeline bundle reference from a Build pipeline selector
// (in a YAML format) from a URL specified in the parameter
func GetDefaultPipelineBundleRef(buildPipelineSelectorYamlURL, selectorName string) (string, error) {
	request, err := http.NewRequest("GET", buildPipelineSelectorYamlURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating GET request: %s", err)
	}

	client := &http.Client{}
	res, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("failed to get a build pipeline selector from url %s: %v", buildPipelineSelectorYamlURL, err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read the body response of a build pipeline selector: %v", err)
	}
	ps := &buildservice.BuildPipelineSelector{}
	if err = yaml.Unmarshal(body, ps); err != nil {
		return "", fmt.Errorf("failed to unmarshal build pipeline selector: %v", err)
	}
	for i := range ps.Spec.Selectors {
		s := &ps.Spec.Selectors[i]
		if s.Name == selectorName {
			return GetBundleRef(&s.PipelineRef), nil
		}
	}

	return "", fmt.Errorf("could not find %s pipeline bundle in build pipeline selector fetched from %s", selectorName, buildPipelineSelectorYamlURL)
}
