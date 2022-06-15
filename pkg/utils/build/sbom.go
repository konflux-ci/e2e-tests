package build

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/oc/pkg/cli/image/extract"
	"github.com/openshift/oc/pkg/cli/image/imagesource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog"
)

func ValidateSbomFilesPresentInImage(image string) error {
	dockerImageRef, err := reference.Parse(image)
	if err != nil {
		return fmt.Errorf("cannot parse docker pull spec (image) %s, error: %+v", image, err)
	}
	tmpDir, err := os.MkdirTemp(os.TempDir(), "sbom")
	if err != nil {
		return fmt.Errorf("error when creating a temp directory for extracting files: %+v", err)
	}
	klog.Infof("extracting contents of container image %s to dir: %s", image, tmpDir)
	eMapping := extract.Mapping{
		ImageRef: imagesource.TypedImageReference{Type: "docker", Ref: dockerImageRef},
		To:       tmpDir,
	}
	e := extract.NewExtractOptions(genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr})
	e.Mappings = []extract.Mapping{eMapping}

	if err := e.Run(); err != nil {
		return fmt.Errorf("error: %+v", err)
	}

	if err := verifySbomPurl(tmpDir); err != nil {
		return fmt.Errorf("failed to verify sbom purl: %+v", err)
	}

	if err := verifySbomCyclonedx(tmpDir); err != nil {
		return fmt.Errorf("failed to verify sbom cyclonedx: %+v", err)
	}
	return nil
}

type SbomPurl struct {
	ImageContents struct {
		Dependencies []struct {
			Purl string `json:"purl"`
		} `json:"dependencies"`
	} `json:"image_contents"`
}

type SbomCyclonedx struct {
	Components []struct {
		Name string `json:"name"`
	} `json:"components"`
}

func verifySbomPurl(rootDir string) error {
	sbomPurlFilePath := rootDir + "/root/buildinfo/content_manifests/sbom-purl.json"
	file, err := os.Stat(sbomPurlFilePath)
	if err != nil {
		return err
	}
	if file.Size() == 0 {
		return fmt.Errorf("sbom file %s is empty", sbomPurlFilePath)
	}

	b, err := os.ReadFile(sbomPurlFilePath)
	if err != nil {
		return err
	}
	p := &SbomPurl{}
	if err := json.Unmarshal(b, p); err != nil {
		return fmt.Errorf("error when parsing sbom purl json: %v", err)
	}

	return nil
}

func verifySbomCyclonedx(rootDir string) error {
	sbomCyclonedxFilePath := rootDir + "/root/buildinfo/content_manifests/sbom-cyclonedx.json"
	file, err := os.Stat(sbomCyclonedxFilePath)
	if err != nil {
		return err
	}
	if file.Size() == 0 {
		return fmt.Errorf("sbom file %s is empty", sbomCyclonedxFilePath)
	}

	b, err := os.ReadFile(sbomCyclonedxFilePath)
	if err != nil {
		return err
	}
	p := &SbomCyclonedx{}
	if err := json.Unmarshal(b, p); err != nil {
		return fmt.Errorf("error when parsing sbom purl json: %v", err)
	}

	return nil
}
