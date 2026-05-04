package build

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ginkgo "github.com/onsi/ginkgo/v2"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// ExtractImage pulls a container image and extracts its flattened filesystem to a temp directory.
func ExtractImage(image string) (string, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", fmt.Errorf("cannot parse image reference %s: %w", image, err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", fmt.Errorf("cannot pull image %s: %w", image, err)
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), "eimage")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %w", err)
	}

	ginkgo.GinkgoWriter.Printf("extracting contents of container image %s to dir: %s\n", image, tmpDir)

	reader := mutate.Extract(img)
	defer reader.Close()

	if err := extractTar(reader, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("error extracting image %s: %w", image, err)
	}

	return tmpDir, nil
}

// ImageFromPipelineRun extracts the output-image from a PipelineRun and fetches its config.
func ImageFromPipelineRun(pipelineRun *pipeline.PipelineRun) (*v1.ConfigFile, error) {
	var outputImage string
	for _, parameter := range pipelineRun.Spec.Params {
		if parameter.Name == "output-image" {
			outputImage = parameter.Value.StringVal
		}
	}
	if outputImage == "" {
		return nil, fmt.Errorf("output-image in PipelineRun not found")
	}

	return FetchImageConfig(outputImage)
}

// FetchImageConfig fetches image config from remote registry.
// It uses the registry authentication credentials stored in default place ~/.docker/config.json
func FetchImageConfig(imagePullspec string) (*v1.ConfigFile, error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("error while fetching image config %s: %w", imagePullspec, err)
	}
	ref, err := name.ParseReference(imagePullspec)
	if err != nil {
		return nil, wrapErr(err)
	}
	descriptor, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, wrapErr(err)
	}

	image, err := descriptor.Image()
	if err != nil {
		return nil, wrapErr(err)
	}
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, wrapErr(err)
	}
	return configFile, nil
}

// FetchImageDigest fetches image manifest digest.
// Digest is returned as a hex string without sha256: prefix.
func FetchImageDigest(imagePullspec string) (string, error) {
	ref, err := name.ParseReference(imagePullspec)
	if err != nil {
		return "", err
	}
	descriptor, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", err
	}
	return descriptor.Digest.Hex, nil
}

func extractTar(reader io.Reader, destDir string) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Symlink(header.Linkname, target)
		}
	}
	return nil
}
