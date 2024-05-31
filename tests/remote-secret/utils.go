package remotesecret

import (
	"github.com/konflux-ci/e2e-tests/pkg/framework"
	"github.com/konflux-ci/e2e-tests/pkg/utils/build"
	image "github.com/konflux-ci/image-controller/api/v1alpha1"
	. "github.com/onsi/gomega"
	rs "github.com/redhat-appstudio/remote-secret/api/v1beta1"
)

func IsTargetSecretLinkedToRightSA(ns, imageRemoteSecretName, serviceAccountName string, target rs.TargetStatus) {
	Expect(target.Namespace).To(Equal(ns))
	Expect(target.DeployedSecret.Name).To(Equal(imageRemoteSecretName))
	Expect(target.ServiceAccountNames).To(HaveLen(1))
	Expect(target.ServiceAccountNames[0]).To(Equal(serviceAccountName))
}

func IsRobotAccountTokenCorrect(secretName, ns, secretType string, image *image.ImageRepository, fw *framework.Framework) {
	secret, err := fw.AsKubeAdmin.CommonController.GetSecret(ns, secretName)
	Expect(err).NotTo(HaveOccurred())

	// get robot account name and token from image secret
	robotAccountName, robotAccountToken := build.GetRobotAccountInfoFromSecret(secret)

	if image != nil {
		// get expected robot account name
		imageRepo, err := fw.AsKubeAdmin.ImageController.GetImageRepositoryCR(image.Name, image.Namespace)
		Expect(err).NotTo(HaveOccurred())

		// ensure that image secret points to the expected robot account name
		expectedRobotAccountName := ""
		if secretType == "pull" {
			expectedRobotAccountName = imageRepo.Status.Credentials.PullRobotAccountName
		} else {
			expectedRobotAccountName = imageRepo.Status.Credentials.PushRobotAccountName
		}
		Expect(robotAccountName).To(Equal(expectedRobotAccountName))
	}

	// get expected robot account token
	expectedRobotAccountToken, err := build.GetRobotAccountToken(robotAccountName)
	Expect(err).ShouldNot(HaveOccurred())

	// ensure secret points to the expected robot account token
	Expect(robotAccountToken).To(Equal(expectedRobotAccountToken))
}
