package build

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/e2e-tests/pkg/clients/tekton"
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func parseDockerfileUsedForBuild(
	c client.Client, tektonController *tekton.TektonController, pr *pipeline.PipelineRun,
) *build.Dockerfile {
	dockerfileContent, err := build.ReadDockerfileUsedForBuild(c, tektonController, pr)
	Expect(err).ShouldNot(HaveOccurred())

	parsedDockerfile, err := build.ParseDockerfile(dockerfileContent)
	Expect(err).ShouldNot(HaveOccurred())

	return parsedDockerfile
}

// CheckParentSources checks the sources coming from parent image are all included in the built source image.
// This check is applied to every build for which source build is enabled, then the several prerequisites
// for including parent sources are handled as well.
func CheckParentSources(c client.Client, tektonController *tekton.TektonController, pr *pipeline.PipelineRun) {
	buildResult, err := build.ReadSourceBuildResult(c, tektonController, pr)
	Expect(err).ShouldNot(HaveOccurred())

	var baseImagesDigests []string
	if build.IsDockerBuild(pr) {
		parsedDockerfile := parseDockerfileUsedForBuild(c, tektonController, pr)
		if parsedDockerfile.IsBuildFromScratch() {
			Expect(buildResult.BaseImageSourceIncluded).Should(BeFalse())
			return
		}
		baseImagesDigests, err = parsedDockerfile.ConvertParentImagesToBaseImagesDigestsForm()
		Expect(err).ShouldNot(HaveOccurred())
	} else {
		Fail("CheckParentSources only works for docker-build pipelines")
	}

	lastBaseImage := baseImagesDigests[len(baseImagesDigests)-1]
	// Remove <none> part if there is. Otherwise, reference.Parse will fail.
	imageWithoutTag := strings.Replace(lastBaseImage, ":<none>", "", 1)
	ref, err := reference.Parse(imageWithoutTag)
	Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("can't parse image reference %s", imageWithoutTag))
	imageWithoutTag = ref.Exact() // drop the tag

	allowed, err := build.IsImagePulledFromAllowedRegistry(imageWithoutTag)
	Expect(err).ShouldNot(HaveOccurred())

	var parentSourceImage string

	if allowed {
		parentSourceImage, err = build.ResolveSourceImageByVersionRelease(imageWithoutTag)
	} else {
		parentSourceImage, err = build.ResolveKonfluxSourceImage(imageWithoutTag)
	}
	Expect(err).ShouldNot(HaveOccurred())

	allIncluded, err := build.AllParentSourcesIncluded(parentSourceImage, buildResult.ImageUrl)

	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "parent source image manifest") && strings.Contains(msg, "MANIFEST_UNKNOWN:") {
			return
		} else {
			Fail(fmt.Sprintf("failed to check parent sources: %v", err))
		}
	}

	Expect(allIncluded).Should(BeTrue())
	Expect(buildResult.BaseImageSourceIncluded).Should(BeTrue())
}

func CheckSourceImage(srcImage, gitUrl string, hub *framework.ControllerHub, pr *pipeline.PipelineRun) {
	//Check if hermetic enabled
	isHermeticBuildEnabled := build.IsHermeticBuildEnabled(pr)
	GinkgoWriter.Printf("HERMETIC STATUS: %v\n", isHermeticBuildEnabled)

	//Get prefetch input value
	prefetchInputValue := build.GetPrefetchValue(pr)
	GinkgoWriter.Printf("PRE-FETCH VALUE: %v\n", prefetchInputValue)

	filesExists, err := build.IsSourceFilesExistsInSourceImage(
		srcImage, gitUrl, isHermeticBuildEnabled, prefetchInputValue)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(filesExists).To(BeTrue())

	c := hub.CommonController.KubeRest()
	CheckParentSources(c, hub.TektonController, pr)
}
