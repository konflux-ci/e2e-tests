package build

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
	quay "github.com/konflux-ci/image-controller/pkg/quay"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/image/reference"
	corev1 "k8s.io/api/core/v1"
)

var (
	quayApiUrl = "https://quay.io/api/v1"
	quayOrg    = utils.GetEnv("DEFAULT_QUAY_ORG", "redhat-appstudio-qe")
	quayToken  = utils.GetEnv("DEFAULT_QUAY_ORG_TOKEN", "")
	quayClient = quay.NewQuayClient(&http.Client{Transport: &http.Transport{}}, quayToken, quayApiUrl)
)

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
	ref, err := reference.Parse(imageURL)
	if err != nil {
		return false, err
	}
	if ref.Tag == "" {
		return false, fmt.Errorf("image URL %s does not have tag", imageURL)
	}
	if ref.Namespace == "" {
		return false, fmt.Errorf("image URL %s does not have namespace", imageURL)
	}
	tagList, _, err := quayClient.GetTagsFromPage(ref.Namespace, ref.Name, 0)
	if err != nil {
		return false, err
	}
	for _, tag := range tagList {
		if tag.Name == ref.Tag {
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

func GetImageTag(organization, repository, tagName string) (quay.Tag, error) {
	page := 0
	for {
		page++
		tags, hasAdditional, err := quayClient.GetTagsFromPage(organization, repository, page)
		if err != nil {
			return quay.Tag{}, err
		}
		for _, tag := range tags {
			if tag.Name == tagName {
				return tag, nil
			}
		}
		if !hasAdditional {
			return quay.Tag{}, fmt.Errorf("%s", fmt.Sprintf("cannot find tag %s", tagName))
		}
	}
}
