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
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	KubeClient, err := client.NewK8SClient()
	if err != nil {
		panic(err)
	}
	secret, err := KubeClient.KubeInterface().CoreV1().Secrets("application-service").Get(context.TODO(), "has-github-token", metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	gomega.Expect(secret).NotTo(gomega.BeNil())

	return nil
}, func(data []byte) {})

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}
