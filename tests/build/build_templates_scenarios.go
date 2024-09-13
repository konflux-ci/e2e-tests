package build

import (
	"strings"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

type ComponentScenarioSpec struct {
	GitURL              string
	Revision            string
	ContextDir          string
	DockerFilePath      string
	PipelineBundleNames []string
	EnableHermetic      bool
	PrefetchInput       string
	CheckAdditionalTags bool
}

func (s ComponentScenarioSpec) DeepCopy() ComponentScenarioSpec {
	pipelineBundleNames := make([]string, len(s.PipelineBundleNames))
	copy(pipelineBundleNames, s.PipelineBundleNames)
	return ComponentScenarioSpec{
		GitURL:              s.GitURL,
		Revision:            s.Revision,
		ContextDir:          s.ContextDir,
		DockerFilePath:      s.DockerFilePath,
		PipelineBundleNames: pipelineBundleNames,
		EnableHermetic:      s.EnableHermetic,
		PrefetchInput:       s.PrefetchInput,
		CheckAdditionalTags: s.CheckAdditionalTags,
	}
}

var componentScenarios = []ComponentScenarioSpec{
	{
		GitURL:         "https://github.com/konflux-qe-bd/devfile-sample-python-basic",
		Revision:       "47fc22092005aabebce233a9b6eab994a8152bbd",
		ContextDir:     ".",
		DockerFilePath: constants.DockerFilePath,
		// Cannot enable multi-platform builds until https://issues.redhat.com/browse/KFLUXINFRA-836 is resolved.
		PipelineBundleNames: []string{"docker-build", "docker-build-oci-ta"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/retrodep",
		Revision:            "d8e3195d1ab9dbee1f621e3b0625a589114ac80f",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      true,
		PrefetchInput:       "gomod",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/pip-e2e-test",
		Revision:            "1ecda839ba9ca55070d75c86c26a1bb07d777bba",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      true,
		PrefetchInput:       "pip",
		CheckAdditionalTags: true,
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/fbc-sample-repo",
		Revision:            "8e374e107fecf03f3c64c528bb53798039661414",
		ContextDir:          "4.13",
		DockerFilePath:      "catalog.Dockerfile",
		PipelineBundleNames: []string{"fbc-builder"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/docker-file-from-scratch",
		Revision:            "34de8caa4952b6214700699e6df4bb53d6f799e6",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/source-build-parent-image-with-digest-only",
		Revision:            "a4f744581c0768eb84a4345f11d04090bb14bdff",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/source-build-use-latest-parent-image",
		Revision:            "b4584ac47e1df84114a10debf262b6d40f6a95f8",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/source-build-parent-image-from-registry-rh-io",
		Revision:            "3f5dcac703a35dcb7b29312be72f86221d0f10ee",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
	{
		GitURL:              "https://github.com/konflux-qe-bd/source-build-base-on-konflux-image",
		Revision:            "86c4d160cfafb8976a23030d4bbc1216bfe8e14f",
		ContextDir:          ".",
		DockerFilePath:      "Dockerfile",
		PipelineBundleNames: []string{"docker-build"},
		EnableHermetic:      false,
		PrefetchInput:       "",
	},
}

func IsDockerBuildGitURL(gitURL string) bool {
	for _, componentScenario := range componentScenarios {
		//check repo name for both the giturls is same
		if utils.GetRepoName(componentScenario.GitURL) == utils.GetRepoName(gitURL) {
			for _, pipeline := range componentScenario.PipelineBundleNames {
				if !strings.HasPrefix(pipeline, "docker-build") {
					return false
				}
			}
			return true
		}
	}
	return false
}

func IsDockerBuildPipeline(pipelineName string) bool {
	return strings.HasPrefix(pipelineName, "docker-build")
}

func IsFBCBuildPipeline(pipelineName string) bool {
	return pipelineName == "fbc-builder"
}

func GetComponentScenarioDetailsFromGitUrl(gitUrl string) ComponentScenarioSpec {
	for _, componentScenario := range componentScenarios {
		//check repo name for both the giturls is same
		if utils.GetRepoName(componentScenario.GitURL) == utils.GetRepoName(gitUrl) {
			scenario := componentScenario.DeepCopy()
			scenario.GitURL = gitUrl
			return scenario
		}
	}
	return ComponentScenarioSpec{}
}
