package has

import (
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils/common"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/has"

	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
)

const (
	ApplicationName        string = "pet-clinic"
	HASArgoApplicationName string = "has"
	ApplicationNamespace   string = "application-service"
)

type Devfile struct {
	Metadata Metadata `yaml:"metadata"`
}

type Metadata struct {
	Attributes Attributes `yaml:"attributes"`
}

type Attributes struct {
	AppModel string `yaml:"appModelRepository.url"`
}

var _ = framework.HASSuiteDescribe("Application E2E tests", func() {
	defer g.GinkgoRecover()
	hasController, err := has.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())
	commonController, err := common.NewSuiteController()
	Expect(err).NotTo(HaveOccurred())

	g.Context("Crud operation:", func() {
		g.It("HAS Application is healthy", func() {
			g.Skip("")
			err := commonController.WaitForArgoCDApplicationToBeReady(HASArgoApplicationName, "openshift-gitops")
			Expect(err).NotTo(HaveOccurred(), "HAS Argo application didn't start in 5 minutes")
		})
		g.It("Create application", func() {
			_, err := hasController.CreateHasApplication(ApplicationName, ApplicationNamespace)

			Expect(err).NotTo(HaveOccurred())
		})
		g.It("Get application", func() {
			application, err := hasController.GetHasApplication(ApplicationName, ApplicationNamespace)
			for _, status := range application.Status.Conditions {
				Expect(string(status.Reason)).To(Equal("OK"))
			}
			//klog.Infof("Repository %s created: ", ObtainGitUrlFromDevfile(application.Status.Devfile))
			hasController.CreateComponent(ApplicationName, ApplicationNamespace)

			Expect(err).NotTo(HaveOccurred())
		})
		g.It("Delete application", func() {
			g.Skip("AAA")
			err := hasController.DeleteHasApplication(ApplicationName, ApplicationNamespace)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

/*
	Right now DevFile status in HAS is a string:
	metadata:
		attributes:
			appModelRepository.url: https://github.com/redhat-appstudio-qe/pet-clinic-application-service-establish-danger
			gitOpsRepository.url: https://github.com/redhat-appstudio-qe/pet-clinic-application-service-establish-danger
		name: pet-clinic
		schemaVersion: 2.1.0
	The ObtainGitUrlFromDevfile extract from the string the git url associated with a application
*/
func ObtainGitUrlFromDevfile(devfile string) string {
	devfileLines := strings.Split(devfile, "\n")
	for _, line := range devfileLines {
		if strings.Contains(line, "appModelRepository.url") {
			gitUrl := strings.Split(line, "url: ")
			if len(gitUrl) == 2 {
				return strings.TrimSpace(gitUrl[1])
			}
		}
	}
	return ""
}
