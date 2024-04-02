package build

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/redhat-appstudio/e2e-tests/pkg/clients/tekton"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/build"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ensureBaseImagesDigestsOrder(
	c client.Client, tektonController *tekton.TektonController, pr *pipeline.PipelineRun, baseImagesDigests []string,
) *build.Dockerfile {
	dockerfileContent, err := build.ReadDockerfileUsedForBuild(c, tektonController, pr)
	Expect(err).ShouldNot(HaveOccurred())

	parsedDockerfile, err := build.ParseDockerfile(dockerfileContent)
	Expect(err).ShouldNot(HaveOccurred())

	// Check the order of BASE_IMAGES_DIGESTS in order to get the correct parent image used in the last build stage.
	convertedBaseImages, err := parsedDockerfile.ConvertParentImagesToBaseImagesDigestsForm()
	Expect(err).ShouldNot(HaveOccurred())
	GinkgoWriter.Println("converted base images:", convertedBaseImages)
	GinkgoWriter.Println("base_images_digests:", baseImagesDigests)
	n := len(convertedBaseImages)
	Expect(n).Should(Equal(len(baseImagesDigests)))
	for i := 0; i < n; i++ {
		Expect(convertedBaseImages[i]).Should(Equal(baseImagesDigests[i]))
	}

	return parsedDockerfile
}

// CheckParentSources checks the sources coming from parent image are all included in the built source image.
// This check is applied to every build for which source build is enabled, then the several prerequisites
// for including parent sources are handled as well.
func CheckParentSources(c client.Client, tektonController *tekton.TektonController, pr *pipeline.PipelineRun) {
	value, err := tektonController.GetTaskRunResult(c, pr, "build-container", "BASE_IMAGES_DIGESTS")
	Expect(err).ShouldNot(HaveOccurred())
	baseImagesDigests := strings.Split(strings.TrimSpace(value), "\n")
	Expect(baseImagesDigests).ShouldNot(BeEmpty(),
		"checkParentSources: no parent image presents in result BASE_IMAGES_DIGESTS")
	GinkgoWriter.Println("BASE_IMAGES_DIGESTS used to build:", baseImagesDigests)

	buildResult, err := build.ReadSourceBuildResult(c, tektonController, pr)
	Expect(err).ShouldNot(HaveOccurred())

	if build.IsDockerBuild(pr) {
		parsedDockerfile := ensureBaseImagesDigestsOrder(c, tektonController, pr, baseImagesDigests)
		if parsedDockerfile.IsBuildFromScratch() {
			Expect(buildResult.BaseImageSourceIncluded).Should(BeFalse())
			return
		}
	}

	lastBaseImage := baseImagesDigests[len(baseImagesDigests)-1]
	// Remove <none> part if there is. Otherwise, reference.Parse will fail.
	imageWithoutTag := strings.Replace(lastBaseImage, ":<none>", "", 1)
	ref, err := reference.Parse(imageWithoutTag)
	Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("can't parse image reference %s", imageWithoutTag))
	imageWithoutTag = ref.Exact() // drop the tag

	allowed, err := build.IsImagePulledFromAllowedRegistry(imageWithoutTag)
	Expect(err).ShouldNot(HaveOccurred())
	if !allowed {
		Expect(buildResult.BaseImageSourceIncluded).Should(BeFalse())
		containsLog, err := build.SourceBuildTaskRunLogsContain(
			tektonController, pr,
			fmt.Sprintf("Image %s does not come from supported allowed registry", lastBaseImage),
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(containsLog).Should(BeTrue())
		return
	}

	parentSourceImage, err := build.ResolveSourceImage(imageWithoutTag)
	Expect(err).ShouldNot(HaveOccurred())
	allIncluded, err := build.AllParentSourcesIncluded(parentSourceImage, buildResult.ImageUrl)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(allIncluded).Should(BeTrue())
	Expect(buildResult.BaseImageSourceIncluded).Should(BeTrue())
}
