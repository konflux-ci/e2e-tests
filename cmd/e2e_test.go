package cmd

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	_ "github.com/redhat-appstudio/e2e-tests/tests/build"
	_ "github.com/redhat-appstudio/e2e-tests/tests/byoc"
	_ "github.com/redhat-appstudio/e2e-tests/tests/enterprise-contract"
	_ "github.com/redhat-appstudio/e2e-tests/tests/integration-service"
	_ "github.com/redhat-appstudio/e2e-tests/tests/release"
	_ "github.com/redhat-appstudio/e2e-tests/tests/remotesecret"
	_ "github.com/redhat-appstudio/e2e-tests/tests/rhtap-demo"
	_ "github.com/redhat-appstudio/e2e-tests/tests/spi"

	"flag"

	"k8s.io/klog/v2"
)

var generateRPPreprocReport bool
var rpPreprocDir string
var polarionOutputFile string
var polarionProjectID string
var generateTestCases bool

func init() {
	flag.BoolVar(&generateRPPreprocReport, "generate-rppreproc-report", false, "Generate report and folders for RP Preproc")
	flag.StringVar(&rpPreprocDir, "rp-preproc-dir", ".", "Folder for RP Preproc")
	flag.StringVar(&polarionOutputFile, "polarion-output-file", "polarion.xml", "Generated polarion test cases")
	flag.StringVar(&polarionProjectID, "project-id", "AppStudio", "Set the Polarion project ID")
	flag.BoolVar(&generateTestCases, "generate-test-cases", false, "Generate Test Cases for Polarion")

	klog.SetLogger(ginkgo.GinkgoLogr)

	verbosity := 1
	if v, err := strconv.ParseUint(os.Getenv("KLOG_VERBOSITY"), 10, 8); err == nil {
		verbosity = int(v)
	}

	flags := &flag.FlagSet{}
	klog.InitFlags(flags)
	if err := flags.Set("v", fmt.Sprintf("%d", verbosity)); err != nil {
		panic(err)
	}
}

func TestE2E(t *testing.T) {
	klog.Info("Starting Red Hat App Studio e2e tests...")
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Red Hat App Studio E2E tests")
}

var _ = ginkgo.ReportAfterSuite("RP Preproc reporter", func(report types.Report) {
	if generateRPPreprocReport {
		//Generate Logs in dirs
		framework.GenerateRPPreprocReport(report, rpPreprocDir)
		//Generate modified JUnit xml file
		resultsPath := rpPreprocDir + "/rp_preproc/results/"
		if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
			klog.Error(err)
		}
		err := framework.GenerateCustomJUnitReport(report, resultsPath+"xunit.xml")
		if err != nil {
			klog.Error(err)
		}
	}
})

var _ = ginkgo.ReportAfterSuite("Polarion reporter", func(report types.Report) {
	if generateTestCases {
		framework.GeneratePolarionReport(report, polarionOutputFile, polarionProjectID)
	}
})
