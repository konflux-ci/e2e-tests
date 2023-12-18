/*
This is modified JUnit XML Reporter for Ginkgo framework.
Original version is available on https://github.com/onsi/ginkgo/blob/master/reporters/junit_report.go

Ginkgo project repository: https://github.com/onsi/ginkgo

MIT License:
Copyright (c) 2013-2014 Onsi Fakhouri

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package framework

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2/reporters"
	types "github.com/onsi/ginkgo/v2/types"
	"github.com/redhat-appstudio/e2e-tests/pkg/logs"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"k8s.io/klog/v2"
)

/*
These custom structures are copied from https://github.com/onsi/ginkgo/blob/v2.13.1/reporters/junit_report.go

These custom structures are modified to use Skipped field instead of Disabled and the Status field is removed
See RHTAP-1902 for more details.
*/
type CustomJUnitTestSuites struct {
	XMLName xml.Name `xml:"testsuites"`
	// Tests maps onto the total number of specs in all test suites (this includes any suite nodes such as BeforeSuite)
	Tests int `xml:"tests,attr"`
	// Skipped maps onto specs that are pending and/or disabled
	Skipped int `xml:"skipped,attr"`
	// Errors maps onto specs that panicked or were interrupted
	Errors int `xml:"errors,attr"`
	// Failures maps onto specs that failed
	Failures int `xml:"failures,attr"`
	// Time is the time in seconds to execute all test suites
	Time float64 `xml:"time,attr"`

	//The set of all test suites
	TestSuites []CustomJUnitTestSuite `xml:"testsuite"`
}

type CustomJUnitTestSuite struct {
	// Name maps onto the description of the test suite - maps onto Report.SuiteDescription
	Name string `xml:"name,attr"`
	// Package maps onto the absolute path to the test suite - maps onto Report.SuitePath
	Package string `xml:"package,attr"`
	// Tests maps onto the total number of specs in the test suite (this includes any suite nodes such as BeforeSuite)
	Tests int `xml:"tests,attr"`
	// Skiped maps onto specs that are skipped/pending
	Skipped int `xml:"skipped,attr"`
	// Errors maps onto specs that panicked or were interrupted
	Errors int `xml:"errors,attr"`
	// Failures maps onto specs that failed
	Failures int `xml:"failures,attr"`
	// Time is the time in seconds to execute all the test suite - maps onto Report.RunTime
	Time float64 `xml:"time,attr"`
	// Timestamp is the ISO 8601 formatted start-time of the suite - maps onto Report.StartTime
	Timestamp string `xml:"timestamp,attr"`

	//Properties captures the information stored in the rest of the Report type (including SuiteConfig) as key-value pairs
	Properties JUnitProperties `xml:"properties"`

	//TestCases capture the individual specs
	TestCases []CustomJUnitTestCase `xml:"testcase"`
}

type CustomJUnitTestCase struct {
	// Name maps onto the full text of the spec - equivalent to "[SpecReport.LeafNodeType] SpecReport.FullText()"
	Name string `xml:"name,attr"`
	// Classname maps onto the name of the test suite - equivalent to Report.SuiteDescription
	Classname string `xml:"classname,attr"`
	// Time is the time in seconds to execute the spec - maps onto SpecReport.RunTime
	Time float64 `xml:"time,attr"`
	//Skipped is populated with a message if the test was skipped or pending
	Skipped *JUnitSkipped `xml:"skipped,omitempty"`
	//Error is populated if the test panicked or was interrupted
	Error *JUnitError `xml:"error,omitempty"`
	//Failure is populated if the test failed
	Failure *JUnitFailure `xml:"failure,omitempty"`
	//SystemOut maps onto any captured stdout/stderr output - maps onto SpecReport.CapturedStdOutErr
	SystemOut string `xml:"system-out,omitempty"`
	//SystemOut maps onto any captured GinkgoWriter output - maps onto SpecReport.CapturedGinkgoWriterOutput
	SystemErr string `xml:"system-err,omitempty"`
}

func GenerateCustomJUnitReport(report types.Report, dst string) error {
	return GenerateCustomJUnitReportWithConfig(report, dst, JunitReportConfig{OmitTimelinesForSpecState: types.SpecStatePassed | types.SpecStateSkipped | types.SpecStatePending})
}

func GenerateCustomJUnitReportWithConfig(report types.Report, dst string, config JunitReportConfig) error {
	suite := CustomJUnitTestSuite{
		Name:      report.SuiteDescription,
		Package:   report.SuitePath,
		Time:      report.RunTime.Seconds(),
		Timestamp: report.StartTime.Format("2006-01-02T15:04:05"),
		Properties: JUnitProperties{
			Properties: []JUnitProperty{
				{"SuiteSucceeded", fmt.Sprintf("%t", report.SuiteSucceeded)},
				{"SuiteHasProgrammaticFocus", fmt.Sprintf("%t", report.SuiteHasProgrammaticFocus)},
				{"SpecialSuiteFailureReason", strings.Join(report.SpecialSuiteFailureReasons, ",")},
				{"SuiteLabels", fmt.Sprintf("[%s]", strings.Join(report.SuiteLabels, ","))},
				{"RandomSeed", fmt.Sprintf("%d", report.SuiteConfig.RandomSeed)},
				{"RandomizeAllSpecs", fmt.Sprintf("%t", report.SuiteConfig.RandomizeAllSpecs)},
				{"LabelFilter", report.SuiteConfig.LabelFilter},
				{"FocusStrings", strings.Join(report.SuiteConfig.FocusStrings, ",")},
				{"SkipStrings", strings.Join(report.SuiteConfig.SkipStrings, ",")},
				{"FocusFiles", strings.Join(report.SuiteConfig.FocusFiles, ";")},
				{"SkipFiles", strings.Join(report.SuiteConfig.SkipFiles, ";")},
				{"FailOnPending", fmt.Sprintf("%t", report.SuiteConfig.FailOnPending)},
				{"FailFast", fmt.Sprintf("%t", report.SuiteConfig.FailFast)},
				{"FlakeAttempts", fmt.Sprintf("%d", report.SuiteConfig.FlakeAttempts)},
				{"DryRun", fmt.Sprintf("%t", report.SuiteConfig.DryRun)},
				{"ParallelTotal", fmt.Sprintf("%d", report.SuiteConfig.ParallelTotal)},
				{"OutputInterceptorMode", report.SuiteConfig.OutputInterceptorMode},
			},
		},
	}
	for _, spec := range report.SpecReports {

		if spec.LeafNodeType != types.NodeTypeIt {
			continue
		}
		test := CustomJUnitTestCase{
			Name:      logs.ShortenTestName(spec),
			Classname: logs.GetClassnameFromReport(spec),
			Time:      spec.RunTime.Seconds(),
		}
		if !spec.State.Is(config.OmitTimelinesForSpecState) {
			test.SystemErr = systemErrForUnstructuredReporters(spec)
		}
		if !config.OmitCapturedStdOutErr {
			test.SystemOut = systemOutForUnstructuredReporters(spec)
		}
		suite.Tests += 1

		switch spec.State {
		// pending tests are also counted to skipped instead of disabled. See RHTAP-1902 for more details.
		case types.SpecStateSkipped:
			message := "skipped"
			if spec.Failure.Message != "" {
				message += " - " + spec.Failure.Message
			}
			test.Skipped = &JUnitSkipped{Message: message}
			suite.Skipped += 1
		case types.SpecStatePending:
			test.Skipped = &JUnitSkipped{Message: "pending"}
			suite.Skipped += 1
		case types.SpecStateFailed:
			test.Failure = &JUnitFailure{
				Message:     spec.Failure.Message,
				Type:        "failed",
				Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
			}
			suite.Failures += 1
		case types.SpecStateInterrupted:
			test.Error = &JUnitError{
				Message:     "interrupted",
				Type:        "interrupted",
				Description: interruptDescriptionForUnstructuredReporters(spec.Failure),
			}
			suite.Errors += 1
		case types.SpecStateAborted:
			test.Failure = &JUnitFailure{
				Message:     spec.Failure.Message,
				Type:        "aborted",
				Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
			}
			suite.Errors += 1
		case types.SpecStatePanicked:
			test.Error = &JUnitError{
				Message:     spec.Failure.ForwardedPanic,
				Type:        "panicked",
				Description: fmt.Sprintf("%s\n%s", spec.Failure.Location.String(), spec.Failure.Location.FullStackTrace),
			}
			suite.Errors += 1
		}

		suite.TestCases = append(suite.TestCases, test)
	}

	junitReport := CustomJUnitTestSuites{
		Tests:      suite.Tests,
		Skipped:    suite.Skipped,
		Errors:     suite.Errors,
		Failures:   suite.Failures,
		Time:       suite.Time,
		TestSuites: []CustomJUnitTestSuite{suite},
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err2 := f.WriteString(xml.Header)
	if err2 != nil {
		klog.Error(err2)
	}
	encoder := xml.NewEncoder(f)
	encoder.Indent("  ", "    ")
	err3 := encoder.Encode(junitReport)
	if err3 != nil {
		klog.Error(err3)
	}
	return f.Close()
}

// This function generates folder structure for the rp_preproc tool with logs for upload in Report Portal
func GenerateRPPreprocReport(report types.Report, rpPreprocParentDir string) {
	rpPreprocDir := rpPreprocParentDir + "/rp_preproc"
	//Delete directory, if exists
	if _, err := os.Stat(rpPreprocDir); !os.IsNotExist(err) {
		err2 := os.RemoveAll(rpPreprocDir)
		if err2 != nil {
			klog.Error(err2)
		}
	}

	wd, _ := os.Getwd()
	artifactDir := utils.GetEnv("ARTIFACT_DIR", fmt.Sprintf("%s/tmp", wd))

	// Generate folder structure for RPPreproc with logs
	for i := range report.SpecReports {
		reportSpec := report.SpecReports[i]
		name := logs.ShortenTestName(reportSpec)
		artifactsDirPath := artifactDir + "/" + name
		reportPortalDirPath := rpPreprocDir + "/attachments/xunit/" + name
		//generate folders only for failed tests
		if !reportSpec.Failure.IsZero() {
			if reportSpec.LeafNodeType == types.NodeTypeIt {
				if err3 := os.MkdirAll(reportPortalDirPath, os.ModePerm); err3 != nil {
					klog.Error(err3)
				} else {
					writeLogInFile(reportPortalDirPath+"/ginkgoWriter.log", reportSpec.CapturedGinkgoWriterOutput)
					writeLogInFile(reportPortalDirPath+"/stdOutErr.log", reportSpec.CapturedStdOutErr)
					writeLogInFile(reportPortalDirPath+"/failureMessage.log", reportSpec.FailureMessage())
					writeLogInFile(reportPortalDirPath+"/failureLocation.log", reportSpec.FailureLocation().FullStackTrace)
				}
			}
		}

		// Move files matching report portal structure stored directly in artifacts dir to rp_preproc subdirectory
		if _, err := os.Stat(artifactsDirPath); os.IsNotExist(err) {
			continue
		}

		if _, err := os.Stat(reportPortalDirPath); os.IsNotExist(err) {
			if err = os.MkdirAll(reportPortalDirPath, os.ModePerm); err != nil {
				klog.Error(err)
				continue
			}
		}

		files, err := os.ReadDir(artifactsDirPath)
		if err != nil {
			klog.Error(err)
		}

		for _, file := range files {
			sourcePath := filepath.Join(artifactsDirPath, file.Name())
			destPath := filepath.Join(reportPortalDirPath, file.Name())

			if err := os.Rename(sourcePath, destPath); err != nil {
				klog.Error(err)
			}
		}

		if err := os.Remove(artifactsDirPath); err != nil {
			klog.Error(err)
		}
	}
}

func writeLogInFile(filePath string, log string) {
	// Do not create empty files
	if len(log) != 0 {
		f, err := os.Create(filePath)
		if err != nil {
			klog.Error(err)
		}
		defer f.Close()

		_, err2 := f.WriteString(log)

		if err2 != nil {
			klog.Error(err2)
		}
	}
}

func interruptDescriptionForUnstructuredReporters(failure types.Failure) string {
	out := &strings.Builder{}
	out.WriteString(failure.Message + "\n")
	NewDefaultReporter(types.ReporterConfig{NoColor: true}, out).EmitProgressReport(failure.ProgressReport)
	return out.String()
}

func systemErrForUnstructuredReporters(spec types.SpecReport) string {
	out := &strings.Builder{}
	gw := spec.CapturedGinkgoWriterOutput
	cursor := 0
	for _, pr := range spec.ProgressReports {
		if cursor < pr.TimelineLocation.Offset {
			if pr.TimelineLocation.Offset < len(gw) {
				out.WriteString(gw[cursor:pr.TimelineLocation.Offset])
				cursor = pr.TimelineLocation.Offset
			} else if cursor < len(gw) {
				out.WriteString(gw[cursor:])
				cursor = len(gw)
			}
		}
		NewDefaultReporter(types.ReporterConfig{NoColor: true}, out).EmitProgressReport(pr)
	}

	if cursor < len(gw) {
		out.WriteString(gw[cursor:])
	}
	return out.String()
}

func systemOutForUnstructuredReporters(spec types.SpecReport) string {
	return spec.CapturedStdOutErr
}
