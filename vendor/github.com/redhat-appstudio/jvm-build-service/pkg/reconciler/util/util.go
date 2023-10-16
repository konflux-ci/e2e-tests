package util

import (
	"context"
	"crypto/md5" //#nosec G501
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ControllerDeploymentName = "hacbs-jvm-operator"
	ControllerNamespace      = "jvm-build-service"

	// goland:noinspection GoCommentStart
	// Label convention as per https://kubernetes.io/docs/reference/labels-annotations-taints is to use lower-case.
	StatusLabel     = "status"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusBuilding  = "building"
)

var (
	controllerName = types.NamespacedName{
		Namespace: ControllerNamespace,
		Name:      ControllerDeploymentName,
	}

	ImageTag  string
	ImageRepo string
)

func GetImageName(ctx context.Context, client client.Client, log logr.Logger, substr, envvar string) (string, error) {
	var err error
	imgTag := ""
	depImg := ""
	controllerDeployment := &appsv1.Deployment{}
	err = client.Get(ctx, controllerName, controllerDeployment)
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	}

	// Get the image name using a controller's env var (if the env var value is specified)
	ciImageName := os.Getenv(envvar)
	if len(ciImageName) != 0 {
		return ciImageName, nil
	}

	// not found errors are either fake/unit test path, or that we are on KCP and don't have access to the namespace
	// name the controller is running under, and hence cannot inspect its image ref; we distinguish between the two
	// via an env var that is set from infra-deployments as part of KCP+workload cluster bootstrap, or the test setup
	imgTag = ImageTag
	if len(strings.TrimSpace(imgTag)) > 0 {
		repo := ImageRepo
		if len(strings.TrimSpace(repo)) == 0 {
			repo = "redhat-appstudio"
		}
		return fmt.Sprintf("quay.io/%s/hacbs-jvm-%s:%s", repo, substr, imgTag), nil
	}
	if err == nil {
		if len(controllerDeployment.Spec.Template.Spec.Containers) == 0 {
			return "", fmt.Errorf("no containers in controller deployment !!!")
		}
		depImg = controllerDeployment.Spec.Template.Spec.Containers[0].Image
		log.Info(fmt.Sprintf("GetImageName controller image %s", depImg))
	}

	retImg := ""
	if strings.Contains(depImg, "controller") {
		retImg = strings.Replace(depImg, "controller", substr, 1)
		return retImg, nil
	}
	return retImg, fmt.Errorf("could not determine image for %s where image var is %s IMAGE_TAG env is %s and deployment get error is %+v", substr, depImg, imgTag, err)
}

func HashString(unique string) string {
	hash := md5.Sum([]byte(unique)) //#nosec
	depId := hex.EncodeToString(hash[:])
	return depId
}
