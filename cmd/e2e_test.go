package cmd

import (
	"context"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/client"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/tests/common"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/tests/has"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	APPLICATION_SERVICE_GITHUB_TOKEN_SECRET = "has-github-token"
	APPLICATION_SERVICE_NAMESPACE           = "application-service"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	KubeClient, err := client.NewK8SClient()
	gomega.Expect(err).ShouldNot(gomega.HaveOcurred(), "Error when trying to start a new K8S client")
	klog.Info("New K8S client has been created successfully")

	secret, err := KubeClient.KubeInterface().CoreV1().Secrets(APPLICATION_SERVICE_NAMESPACE).Get(context.TODO(), APPLICATION_SERVICE_GITHUB_TOKEN_SECRET, metav1.GetOptions{})
	gomega.Expect(err).ShouldNot(gomega.HaveOcurred(), "Error when trying to retrieve secret information")
	klog.Infof("Secret %s information successfully gathered", secret.Name)

	return nil
}, func(data []byte) {})

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}
