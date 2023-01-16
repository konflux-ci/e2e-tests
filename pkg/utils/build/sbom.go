package build

import (
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/oc/pkg/cli/image/extract"
	"github.com/openshift/oc/pkg/cli/image/imagesource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func GetParsedSbomFilesContentFromImage(image string) (*SbomPurl, *SbomCyclonedx, error) {
	dockerImageRef, err := reference.Parse(image)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse docker pull spec (image) %s, error: %+v", image, err)
	}
	tmpDir, err := os.MkdirTemp(os.TempDir(), "sbom")
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating a temp directory for extracting files: %+v", err)
	}
	GinkgoWriter.Printf("extracting contents of container image %s to dir: %s\n", image, tmpDir)
	eMapping := extract.Mapping{
		ImageRef: imagesource.TypedImageReference{Type: "docker", Ref: dockerImageRef},
		To:       tmpDir,
	}
	e := extract.NewExtractOptions(genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr})
	e.Mappings = []extract.Mapping{eMapping}

	if err := e.Run(); err != nil {
		return nil, nil, fmt.Errorf("error: %+v", err)
	}

	purl, err := getSbomPurlContent(tmpDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get sbom purl content: %+v", err)
	}

	cyclonedx, err := getSbomCyclonedxContent(tmpDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get sbom cyclonedx content: %+v", err)
	}
	return purl, cyclonedx, nil
}

type SbomPurl struct {
	ImageContents struct {
		Dependencies []struct {
			Purl string `json:"purl"`
		} `json:"dependencies"`
	} `json:"image_contents"`
}

type SbomCyclonedx struct {
	BomFormat   string
	SpecVersion string
	Version     int
	Components  []struct {
		Name    string `json:"name"`
		Purl    string `json:"purl"`
		Type    string `json:"type"`
		Version string `json:"version"`
	} `json:"components"`
}

func getSbomPurlContent(rootDir string) (*SbomPurl, error) {
	sbomPurlFilePath := rootDir + "/root/buildinfo/content_manifests/sbom-purl.json"
	file, err := os.Stat(sbomPurlFilePath)
	if err != nil {
		return nil, fmt.Errorf("sbom file not found in path %s", sbomPurlFilePath)
	}
	if file.Size() == 0 {
		return nil, fmt.Errorf("sbom file %s is empty", sbomPurlFilePath)
	}

	b, err := os.ReadFile(sbomPurlFilePath)
	if err != nil {
		return nil, fmt.Errorf("error when reading sbom file %s: %v", sbomPurlFilePath, err)
	}
	sbom := &SbomPurl{}
	if err := json.Unmarshal(b, sbom); err != nil {
		return nil, fmt.Errorf("error when parsing sbom PURL json: %v", err)
	}

	return sbom, nil
}

func getSbomCyclonedxContent(rootDir string) (*SbomCyclonedx, error) {
	sbomCyclonedxFilePath := rootDir + "/root/buildinfo/content_manifests/sbom-cyclonedx.json"
	file, err := os.Stat(sbomCyclonedxFilePath)
	if err != nil {
		return nil, fmt.Errorf("sbom file not found in path %s", sbomCyclonedxFilePath)
	}
	if file.Size() == 0 {
		return nil, fmt.Errorf("sbom file %s is empty", sbomCyclonedxFilePath)
	}

	b, err := os.ReadFile(sbomCyclonedxFilePath)
	if err != nil {
		return nil, fmt.Errorf("error when reading sbom file %s: %v", sbomCyclonedxFilePath, err)
	}
	sbom := &SbomCyclonedx{}
	if err := json.Unmarshal(b, sbom); err != nil {
		return nil, fmt.Errorf("error when parsing sbom CycloneDX json: %v", err)
	}

	return sbom, nil
}
