package tekton

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo/v2"
)

type CosignResult struct {
	signatureImageRef   string
	attestationImageRef string
}

// FindCosignResultsForImage looks for .sig and .att image tags in the OpenShift image stream for the provided image reference.
// If none can be found errors.IsNotFound(err) is true, when err is nil CosignResult contains image references for signature and attestation images, otherwise other errors could be returned.
func (k KubeController) FindCosignResultsForImage(imageRef string) (*CosignResult, error) {
	return findCosignResultsForImage(imageRef)
}

func findCosignResultsForImage(imageRef string) (*CosignResult, error) {
	var errMsg string
	// Split the image ref into image repo+tag (e.g quay.io/repo/name:tag), and image digest (sha256:abcd...)
	imageInfo := strings.Split(imageRef, "@")
	imageRegistryName := strings.Split(imageInfo[0], "/")[0]
	// imageRepoName is stripped from container registry name and a tag e.g. "quay.io/<org>/<repo>:tagprefix" => "<org>/<repo>"
	imageRepoName := strings.Split(strings.TrimPrefix(imageInfo[0], fmt.Sprintf("%s/", imageRegistryName)), ":")[0]
	// Cosign creates tags for attestation and signature based on the image digest. Compute
	// the expected prefix for later usage: sha256:abcd... -> sha256-abcd...
	// Also, this prefix is really the prefix of the image tag resource which follows the
	// format: <image-repo>:<tag-name>
	imageTagPrefix := strings.Replace(imageInfo[1], ":", "-", 1)

	results := CosignResult{}
	signatureTag, err := getImageInfoFromQuay(imageRepoName, imageTagPrefix+".sig")
	if err != nil {
		errMsg += fmt.Sprintf("error when getting signature tag: %+v\n", err)
	} else {
		results.signatureImageRef = signatureTag.ImageRef
	}

	attestationTag, err := getImageInfoFromQuay(imageRepoName, imageTagPrefix+".att")
	if err != nil {
		errMsg += fmt.Sprintf("error when getting attestation tag: %+v\n", err)
	} else {
		results.attestationImageRef = attestationTag.ImageRef
	}

	if len(errMsg) > 0 {
		return &results, fmt.Errorf("failed to find cosign results for image %s: %s", imageRef, errMsg)
	}

	return &results, nil
}

func (c CosignResult) IsPresent() bool {
	return c.signatureImageRef != "" && c.attestationImageRef != ""
}

func (c CosignResult) Missing(prefix string) string {
	var ret []string = make([]string, 0, 2)
	if c.signatureImageRef == "" {
		ret = append(ret, prefix+".sig")
	}

	if c.attestationImageRef == "" {
		ret = append(ret, prefix+".att")
	}

	return strings.Join(ret, " and ")
}

func (k KubeController) AwaitAttestationAndSignature(image string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		if _, err := k.FindCosignResultsForImage(image); err != nil {
			g.GinkgoWriter.Printf("failed to get cosign result for image %s: %+v\n", image, err)
			return false, nil
		}

		return true, nil
	})
}
