package common

import (
	"context"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create the struct for kubernetes and github clients.
type SuiteController struct {
	// Wrap K8S client go to interact with Kube cluster
	*kubeCl.CustomClient

	// Github client to interact with GH apis
	Github *github.Github
}

/*
Create controller for the common kubernetes API crud operations. This controller should be used only to interact with non RHTAP/AppStudio APIS like routes, deployment, pods etc...
Check if a github organization env var is set, if not use by default the redhat-appstudio-qe org. See: https://github.com/redhat-appstudio-qe
*/
func NewSuiteController(kubeC *kubeCl.CustomClient) (*SuiteController, error) {
	gh, err := github.NewGithubClient(utils.GetEnv(constants.GITHUB_TOKEN_ENV, ""), utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"))
	if err != nil {
		return nil, err
	}
	return &SuiteController{
		kubeC,
		gh,
	}, nil
}

func (cs *SuiteController) StorePodLogs(testNamespace, jobName, testLogsDir string) error {
	podsDir := fmt.Sprintf("%s/pods", testLogsDir)
	if err := os.MkdirAll(podsDir, os.ModePerm); err != nil {
		return err
	}

	podList, err := cs.KubeInterface().CoreV1().Pods(jobName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing %s pods: %s\n", testNamespace, err.Error())
	}

	GinkgoWriter.Printf("found %d pods in namespace: %s\n", len(podList.Items), testNamespace)

	for _, pod := range podList.Items {
		var containers []corev1.Container
		containers = append(containers, pod.Spec.InitContainers...)
		containers = append(containers, pod.Spec.Containers...)
		for _, c := range containers {
			log, err := utils.GetContainerLogs(cs.KubeInterface(), pod.Name, c.Name, pod.Namespace)
			if err != nil {
				GinkgoWriter.Printf("error getting logs for pod/container %s/%s: %v\n", pod.Name, c.Name, err.Error())
				continue
			}

			filepath := fmt.Sprintf("%s/%s-pod-%s-%s.log", podsDir, pod.Namespace, pod.Name, c.Name)
			if err := os.WriteFile(filepath, []byte(log), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
