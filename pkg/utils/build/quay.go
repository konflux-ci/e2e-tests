package build

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	quay "github.com/redhat-appstudio/image-controller/pkg/quay"
	corev1 "k8s.io/api/core/v1"
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

func IsImageAnnotationPresent(annotations map[string]string) bool {
	image_annotation_str := annotations["image.redhat.com/image"]
	return image_annotation_str != ""
}

func ImageRepoCreationSucceeded(annotations map[string]string) bool {
	imageAnnotationValue := annotations["image.redhat.com/image"]
	return !strings.Contains(imageAnnotationValue, "failed to generete image repository")
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
	if imageName == "" {
		return false, nil
	}
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
		return false, fmt.Errorf("image URL %s has incorrect format", imageURL)
	}
	repoParts := strings.Split(urlParts[0], "/")
	if len(repoParts) <= 2 {
		return false, fmt.Errorf("image URL %s is not complete", imageURL)
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

func IsImageRepoPublic(quayImageRepoName string) (bool, error) {
	return quayClient.IsRepositoryPublic(quayOrg, quayImageRepoName)
}

func DoesQuayOrgSupportPrivateRepo() (bool, error) {
	repositoryRequest := quay.RepositoryRequest{
		Namespace:   quayOrg,
		Visibility:  "private",
		Description: "Test private repository",
		Repository:  constants.SamplePrivateRepoName,
	}
	repo, err := quayClient.CreateRepository(repositoryRequest)
	if err != nil {
		if err.Error() == "payment required" {
			return false, nil
		} else {
			return false, err
		}
	}
	if repo == nil {
		return false, fmt.Errorf("%v repository created is nil", repo)
	}
	// Delete the created image repo
	_, err = DeleteImageRepo(constants.SamplePrivateRepoName)
	if err != nil {
		return true, fmt.Errorf("error while deleting private image repo: %v", err)
	}
	return true, nil
}

// GetRobotAccountToken gets the robot account token from a given robot account name
func GetRobotAccountToken(robotAccountName string) (string, error) {
	ra, err := quayClient.GetRobotAccount(quayOrg, robotAccountName)
	if err != nil {
		return "", err
	}

	return ra.Token, nil
}

// GetRobotAccountInfoFromSecret gets robot account name and token from secret data
func GetRobotAccountInfoFromSecret(secret *corev1.Secret) (string, string) {
	uploadSecretDockerconfigJson := string(secret.Data[corev1.DockerConfigJsonKey])
	var authDataJson interface{}
	Expect(json.Unmarshal([]byte(uploadSecretDockerconfigJson), &authDataJson)).To(Succeed())

	authRegexp := regexp.MustCompile(`.*{"auth":"([A-Za-z0-9+/=]*)"}.*`)
	uploadSecretAuthString, err := base64.StdEncoding.DecodeString(authRegexp.FindStringSubmatch(uploadSecretDockerconfigJson)[1])
	Expect(err).To(Succeed())

	auth := strings.Split(string(uploadSecretAuthString), ":")
	Expect(auth).To(HaveLen(2))

	robotAccountName := strings.TrimPrefix(auth[0], quayOrg+"+")
	robotAccountToken := auth[1]

	return robotAccountName, robotAccountToken
}
