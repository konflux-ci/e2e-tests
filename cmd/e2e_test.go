package cmd

import (
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/framework/application-service"
)

var (
	testResultsDirectory = "/home/flacatusu/WORKSPACE/appstudio-qe/e2e-tests/output"
	jUnitOutputFilename  = "e2e-junit.xml"
)

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(data []byte) {})

func TestE2E(t *testing.T) {
	var r []ginkgo.Reporter

	r = append(r, reporters.NewJUnitReporter(filepath.Join(testResultsDirectory, jUnitOutputFilename)))
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Red Hat App Studio E2E tests", r)
}
