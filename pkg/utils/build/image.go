package build

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/oc/pkg/cli/image/extract"
	"github.com/openshift/oc/pkg/cli/image/imagesource"
	imageInfo "github.com/openshift/oc/pkg/cli/image/info"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func ExtractImage(image string) (string, error) {
	dockerImageRef, err := reference.Parse(image)
	if err != nil {
		return "", fmt.Errorf("cannot parse docker pull spec (image) %s, error: %+v", image, err)
	}
	tmpDir, err := os.MkdirTemp(os.TempDir(), "eimage")
	if err != nil {
		return "", fmt.Errorf("error when creating a temp directory for extracting files: %+v", err)
	}
	GinkgoWriter.Printf("extracting contents of container image %s to dir: %s\n", image, tmpDir)
	eMapping := extract.Mapping{
		ImageRef: imagesource.TypedImageReference{Type: "docker", Ref: dockerImageRef},
		To:       tmpDir,
	}
	e := extract.NewExtractOptions(genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr})
	e.Mappings = []extract.Mapping{eMapping}

	if err := e.Run(); err != nil {
		return "", fmt.Errorf("error: %+v", err)
	}
	return tmpDir, nil
}

func ImageFromPipelineRun(pipelineRun *tektonv1.PipelineRun) (*imageInfo.Image, error) {
	var outputImage string
	for _, parameter := range pipelineRun.Spec.Params {
		if parameter.Name == "output-image" {
			outputImage = parameter.Value.StringVal
		}
	}
	if outputImage == "" {
		return nil, fmt.Errorf("output-image in PipelineRun not found")
	}

	dockerImageRef, err := reference.Parse(outputImage)
	if err != nil {
		return nil, fmt.Errorf("error parsing outputImage to dockerImageRef, %w", err)
	}

	imageRetriever := imageInfo.ImageRetriever{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	image, err := imageRetriever.Image(ctx, imagesource.TypedImageReference{Type: "docker", Ref: dockerImageRef})
	if err != nil {
		return nil, fmt.Errorf("error getting image from imageRetriver, %w", err)
	}
	return image, nil
}
