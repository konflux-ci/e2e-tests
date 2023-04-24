package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appservice "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/vcluster"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const QuarkusDevfileSource string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"

var _ = framework.E2ESuiteDescribe(Label("byoc"), Ordered, func() {
	defer GinkgoRecover()

	var vc vcluster.Vcluster
	var fw *framework.Framework
	var stagingApiServerUrl string
	var stagingKubeconfig string

	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	Describe("Ni idea", func() {
		BeforeAll(func() {
			fw, err = framework.NewFramework(utils.GetGeneratedNamespace(fw.UserNamespace))
			Expect(err).NotTo(HaveOccurred())
			vc = vcluster.NewVclusterController(fmt.Sprintf("%s/tmp", cwd), fw.AsKubeAdmin.CommonController.CustomClient)
		})

		It("start staging cluster", func() {
			stagingKubeconfig, err = vc.InitializeVCluster(fw.UserNamespace, fw.UserNamespace)
			Expect(err).NotTo(HaveOccurred())
			config, err := clientcmd.LoadFromFile(stagingKubeconfig)
			Expect(err).NotTo(HaveOccurred())
			stagingApiServerUrl = config.Clusters[config.CurrentContext].Server
		})

		It("checks if staging cluster API is available", func() {
			Eventually(func() bool {
				tc := &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				}
				client := http.Client{Transport: tc}
				res, err := client.Get(stagingApiServerUrl)
				if err != nil || res.StatusCode > 499 {
					return false
				}
				return true
			}, 10*time.Minute, 10*time.Second).Should(BeTrue(), "timed out when waiting for staging cluster API")
		})

		It("creates managed-gitops.redhat.com/managed-environment secret type", func() {
			kubeConfData, err := os.ReadFile(stagingKubeconfig)
			data := make(map[string][]byte)
			data["kubeconfig"] = kubeConfData
			Expect(err).NotTo(HaveOccurred())

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kubernetes-credentials",
					Namespace: fw.UserNamespace,
				},
				Type: v1.SecretType("managed-gitops.redhat.com/managed-environment"),
				Data: data,
			}

			_, err = fw.AsKubeAdmin.CommonController.CreateSecret(fw.UserNamespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates environment", func() {
			env := appservice.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "development",
					Namespace: fw.UserNamespace,
				},
				Spec: appservice.EnvironmentSpec{
					DeploymentStrategy: appservice.DeploymentStrategy_AppStudioAutomated,
					Configuration: appservice.EnvironmentConfiguration{
						Env: []appservice.EnvVarPair{
							{
								Name:  "POC",
								Value: "POC",
							},
						},
					},
					UnstableConfigurationFields: &appservice.UnstableEnvironmentConfiguration{
						ClusterType: appservice.ConfigurationClusterType_Kubernetes,
						KubernetesClusterCredentials: appservice.KubernetesClusterCredentials{
							TargetNamespace:            "byoc-ns",
							APIURL:                     stagingApiServerUrl,
							ClusterCredentialsSecret:   "kubernetes-credentials",
							AllowInsecureSkipTLSVerify: true,
						},
					},
				},
			}
			err := fw.AsKubeAdmin.CommonController.CustomClient.KubeRest().Create(context.TODO(), &env)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
