package build

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	"github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
	tektonpipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	testTaskName = "buildah-min"
	testBundle   = "quay.io/konflux-ci/tekton-catalog/task-buildah-min:0.2"
)

func GetDockerBuildPipelineBundle() map[string]string {
	var buildPipelineAnnotation map[string]string
	customDockerBuildBundle := os.Getenv(constants.CUSTOM_DOCKER_BUILD_PIPELINE_BUNDLE_ENV)
	if customDockerBuildBundle != "" {
		buildPipelineAnnotation = map[string]string{
			"build.appstudio.openshift.io/pipeline": fmt.Sprintf(`{"name":"docker-build", "bundle": "%s"}`, customDockerBuildBundle),
		}
	} else {
		buildPipelineAnnotation = constants.DefaultDockerBuildPipelineBundle
	}
	return buildPipelineAnnotation
}

func CreateCustomBuildBundle(pipelineName string) (string, error) {
	var tektonObj runtime.Object
	var bundleParam *tektonpipeline.Param
	var nameParam *tektonpipeline.Param
	var pipelineBundle string
	var newPipelineYaml []byte
	var err error

	if err = utils.CreateDockerConfigFile(os.Getenv("QUAY_TOKEN")); err != nil {
		return "", fmt.Errorf("failed to create docker config file: %+v", err)
	}

	if pipelineBundle, err = tekton.GetDefaultPipelineBundleRef(constants.BuildPipelineConfigConfigMapYamlURL, pipelineName); err != nil {
		return "", fmt.Errorf("failed to get the pipeline bundle ref: %+v", err)
	}

	// Extract docker-build pipeline as tekton object from the bundle
	if tektonObj, err = tekton.ExtractTektonObjectFromBundle(pipelineBundle, "pipeline", pipelineName); err != nil {
		return "", fmt.Errorf("failed to extract the Tekton Pipeline from bundle: %+v", err)
	}

	pipelineObject := tektonObj.(*tektonpipeline.Pipeline)

	// Update build-container step task ref to buildah-min:0.2 instead of buildah
	for i := range pipelineObject.PipelineSpec().Tasks {
		t := &pipelineObject.PipelineSpec().Tasks[i]
		if t.Name == "build-container" {
			for k, param := range t.TaskRef.Params {
				if param.Name == "bundle" {
					bundleParam = &t.TaskRef.Params[k]
				}
				if param.Name == "name" {
					nameParam = &t.TaskRef.Params[k]
				}
			}
		}
	}

	bundleParam.Value = *tektonpipeline.NewStructuredValues(testBundle)
	nameParam.Value = *tektonpipeline.NewStructuredValues(testTaskName)

	if newPipelineYaml, err = yaml.Marshal(pipelineObject); err != nil {
		return "", fmt.Errorf("error when marshalling a new pipeline to YAML: %v", err)
	}

	keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
	authOption := remoteimg.WithAuthFromKeychain(keychain)

	tag := fmt.Sprintf("%d-%s", time.Now().Unix(), util.GenerateRandomString(4))
	quayOrg := utils.GetEnv(constants.DEFAULT_QUAY_ORG_ENV, constants.DefaultQuayOrg)
	newBuildPipelineImg := strings.ReplaceAll(constants.DefaultImagePushRepo, constants.DefaultQuayOrg, quayOrg)

	var newBuildPipeline, _ = name.ParseReference(fmt.Sprintf("%s:pipeline-bundle-%s", newBuildPipelineImg, tag))
	// Build and Push the tekton bundle
	if err = tekton.BuildAndPushTektonBundle(newPipelineYaml, newBuildPipeline, authOption); err != nil {
		return "", fmt.Errorf("error when building/pushing a tekton pipeline bundle: %v", err)
	}
	return newBuildPipeline.String(), nil
}
