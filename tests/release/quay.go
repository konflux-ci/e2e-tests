package common

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	quay "github.com/redhat-appstudio/image-controller/pkg/quay"
)

var (
	quayApiUrl = "https://quay.io/api/v1"
	quayOrg    = utils.GetEnv("IMAGE_CONTROLLER_QUAY_ORG", "hacbs-release-tests")
	quayToken  = utils.GetEnv("IMAGE_CONTROLLER_QUAY_ORG_TOKEN", "")
	quayClient = quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayToken, quayApiUrl)
)

// imageURL format example: quay.io/redhat-appstudio-qe/devfile-go-rhtap-uvv7:latest
func GetDigestWithTagInQuay(imageURL string) (string, error) {
	urlParts := strings.Split(imageURL, ":")
	if len(urlParts) != 2 {
		return "", fmt.Errorf("image URL %s has incorrect format", imageURL)
	}
	repoParts := strings.Split(urlParts[0], "/")
	if len(repoParts) <= 2 {
		return "", fmt.Errorf("image URL %s is not complete", imageURL)
	}
	repoName := strings.Join(repoParts[2:], "/")
	tagList, _, err := quayClient.GetTagsFromPage(quayOrg, repoName, 0)
	if err != nil {
		return "", err
	}
	for _, tag := range tagList {
		if tag.Name == urlParts[1] {
			return tag.ManifestDigest, nil
		}
	}
        return "", fmt.Errorf("no image is found")
}
