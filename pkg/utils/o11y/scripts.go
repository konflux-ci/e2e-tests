package o11y

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getImagePushScript returns ImagePush script.
func (o *O11yController) getImagePushScript(secret, quayOrg string) string {
	return fmt.Sprintf(`#!/bin/sh
authFilePath="/tekton/creds-secrets/%s/.dockerconfigjson"
destImageRef="quay.io/%s/o11y-workloads"
# Set Permissions
sed -i 's/^\s*short-name-mode\s*=\s*.*/short-name-mode = "disabled"/' /etc/containers/registries.conf
echo 'root:1:4294967294' | tee -a /etc/subuid >> /etc/subgid
# Pull Image
echo -e "FROM quay.io/libpod/alpine:latest\nRUN dd if=/dev/urandom of=/100mbfile bs=1M count=100" > Dockerfile
unshare -Ufp --keep-caps -r --map-users 1,1,65536 --map-groups 1,1,65536 -- buildah bud --tls-verify=false --no-cache -f ./Dockerfile -t "$destImageRef" .
IMAGE_SHA_DIGEST=$(buildah images --digests | grep ${destImageRef} | awk '{print $4}')
TAGGED_IMAGE_NAME="${destImageRef}:${IMAGE_SHA_DIGEST}"
buildah tag ${destImageRef} ${TAGGED_IMAGE_NAME}
buildah images
buildah push --authfile "$authFilePath" --disable-compression --tls-verify=false ${TAGGED_IMAGE_NAME}
if [ $? -eq 0 ]; then
  # Scraping Interval Period, Pod must stay alive
  sleep 1m
  echo "Image push completed"
else
  echo "Image push failed"
  exit 1
fi`, secret, quayOrg)
}

// labelsToSelector creates new Selector using given labelMap.
func labelsToSelector(labelMap map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range labelMap {
		req, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*req)
	}
	return selector
}

// WaitForScriptCompletion waits for the script to complete.
func (o *O11yController) WaitForScriptCompletion(deployment *appsv1.Deployment, successMessage string, timeout time.Duration) error {
	namespace := deployment.Namespace
	deploymentName := deployment.Name

	// Get the pod associated with the deployment
	podList := &corev1.PodList{}
	labels := deployment.Spec.Selector.MatchLabels
	labelSelector := labelsToSelector(labels)
	err := o.KubeRest().List(context.Background(), podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: labelSelector})
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found for deployment %s", deploymentName)
	}

	pod := podList.Items[0]

	// Wait for the success message in the pod's log output
	podLogOpts := &corev1.PodLogOptions{}
	req := o.KubeInterface().CoreV1().Pods(namespace).GetLogs(pod.Name, podLogOpts)

	err = wait.PollUntilContextTimeout(context.Background(), time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		readCloser, err := req.Stream(context.Background())
		if err != nil {
			return false, err
		}
		defer readCloser.Close()

		scanner := bufio.NewScanner(readCloser)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), successMessage) {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}
