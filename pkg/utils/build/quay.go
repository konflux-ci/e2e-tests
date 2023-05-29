package build

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	quay "github.com/redhat-appstudio/image-controller/pkg/quay"
)

var (
	quayApiUrl = "https://quay.io/api/v1"
	quayOrg    = utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")
	quayToken  = utils.GetEnv("DEFAULT_QUAY_ORG_TOKEN", "")
	quayClient = quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayToken, quayApiUrl)
)

func GetQuayImageName(annotations map[string]string) (string, error) {
	type imageAnnotation struct {
		Image  string `json:"Image"`
		Secret string `json:"Secret"`
	}
	image_annotation_str := annotations["image.redhat.com/image"]
	var ia imageAnnotation
	err := json.Unmarshal([]byte(image_annotation_str), &ia)
	if err != nil {
		return "", err
	}
	tokens := strings.Split(ia.Image, "/")
	return strings.Join(tokens[2:], "/"), nil
}

func GetRobotAccountName(imageName string) string {
	tokens := strings.Split(imageName, "/")
	return strings.Join(tokens, "")
}

func DoesImageRepoExistInQuay(quayImageRepoName string) (bool, error) {
	exists, err := quayClient.DoesRepositoryExist(quayOrg, quayImageRepoName)
	if exists {
		return true, nil
	} else if !exists && strings.Contains(err.Error(), "does not exist") {
		return false, nil
	} else {
		return false, err
	}
}

func DoesRobotAccountExistInQuay(robotAccountName string) (bool, error) {
	_, err := quayClient.GetRobotAccount(quayOrg, robotAccountName)
	if err != nil {
		if err.Error() == "Could not find robot with specified username" {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func DeleteImageRepo(imageName string) (bool, error) {
	_, err := quayClient.DeleteRepository(quayOrg, imageName)
	if err != nil {
		return false, err
	}
	return true, nil
}

// imageURL format example: quay.io/redhat-appstudio-qe/devfile-go-rhtap-uvv7:build-66d4e-1685533053
func DoesTagExistsInQuay(imageURL string) (bool, error) {
	urlParts := strings.Split(imageURL, ":")
	if len(urlParts) != 2 {
		return false, fmt.Errorf("Image URL %s has incorrect format", imageURL)
	}
	repoParts := strings.Split(urlParts[0], "/")
	if len(repoParts) <= 2 {
		return false, fmt.Errorf("Image URL %s is not complete", imageURL)
	}
	repoName := strings.Join(repoParts[2:], "/")
	tagList, _, err := quayClient.GetTagsFromPage(quayOrg, repoName, 0)
	if err != nil {
		return false, err
	}
	for _, tag := range tagList {
		if tag.Name == urlParts[1] {
			return true, nil
		}
	}
	return false, nil
}
