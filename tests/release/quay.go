package common

import (
	"fmt"
	"net/http"
	"strings"

	quay "github.com/konflux-ci/image-controller/pkg/quay"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

var (
	quayApiUrl = "https://quay.io/api/v1"
	// quayOrg    = utils.GetEnv("IMAGE_CONTROLLER_QUAY_ORG", "hacbs-release-tests")
	quayToken  = utils.GetEnv("IMAGE_CONTROLLER_QUAY_ORG_TOKEN", "")
	quayClient = quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayToken, quayApiUrl)
)

// repoURL format example: quay.io/redhat-appstudio-qe/dcmetromap
func DoesDigestExistInQuay(repoURL string, digest string) (bool, error) {
	repoParts := strings.Split(repoURL, "/")
	if len(repoParts) <= 2 {
		return false, fmt.Errorf("repo URL %s is not complete", repoURL)
	}

	repoName := strings.Join(repoParts[2:], "/")
	tagList, _, err := quayClient.GetTagsFromPage(repoParts[1], repoName, 0)
	if err != nil {
		return false, err
	}

	for _, tag := range tagList {
		if tag.ManifestDigest == digest {
			return true, nil
		}
	}

	return false, fmt.Errorf("no image is found")
}
