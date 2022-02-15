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
	APPLICATION_SERVICE_NAMESPACE = "application-service"
	APPLICATION_SERVICE_NAME      = "has-github-token"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	KubeClient, err := client.NewK8SClient()
	Expect(err).To(BeNil(), "Error when trying to start a new K8S client")
	klog.Info("New K8S client has been created successfully")

	secret, err := KubeClient.KubeInterface().CoreV1().Secrets(APPLICATION_SERVICE_NAMESPACE).Get(context.TODO(), APPLICATION_SERVICE_NAME, metav1.GetOptions{})
	Expect(err).To(BeNil(), "Error when trying to retrieve information from kube-api")
	klog.Info("Secret information successfully gathered")

	Expect(secret).NotTo(BeNil())

	return nil
}, func(data []byte) {})

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}
