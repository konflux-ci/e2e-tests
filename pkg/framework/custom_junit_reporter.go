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
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2/reporters"
	types "github.com/onsi/ginkgo/v2/types"
	"k8s.io/klog/v2"
)

func GenerateCustomJUnitReport(report types.Report, dst string) error {
	return GenerateCustomJUnitReportWithConfig(report, dst, JunitReportConfig{})
}

func GenerateCustomJUnitReportWithConfig(report types.Report, dst string, config JunitReportConfig) error {
	suite := JUnitTestSuite{
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
		test := JUnitTestCase{
			Name:      shortenStringAddHash(spec),
			Classname: getClassnameFromReport(spec),
			Status:    spec.State.String(),
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
		case types.SpecStateSkipped:
			message := "skipped"
			if spec.Failure.Message != "" {
				message += " - " + spec.Failure.Message
			}
			test.Skipped = &JUnitSkipped{Message: message}
			suite.Skipped += 1
		case types.SpecStatePending:
			test.Skipped = &JUnitSkipped{Message: "pending"}
			suite.Disabled += 1
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

	junitReport := JUnitTestSuites{
		Tests:      suite.Tests,
		Disabled:   suite.Disabled + suite.Skipped,
		Errors:     suite.Errors,
		Failures:   suite.Failures,
		Time:       suite.Time,
		TestSuites: []JUnitTestSuite{suite},
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
	//Generate folder structure for RPPreproc with logs
	for i := range report.SpecReports {
		reportSpec := report.SpecReports[i]
		//generate folders only for failed tests
		if !reportSpec.Failure.IsZero() {
			if reportSpec.LeafNodeType == types.NodeTypeIt {
				name := shortenStringAddHash(reportSpec)
				filePath := rpPreprocDir + "/attachments/xunit/" + name
				if err3 := os.MkdirAll(filePath, os.ModePerm); err3 != nil {
					klog.Error(err3)
				} else {
					writeLogInFile(filePath+"/ginkgoWriter.log", reportSpec.CapturedGinkgoWriterOutput)
					writeLogInFile(filePath+"/stdOutErr.log", reportSpec.CapturedStdOutErr)
					writeLogInFile(filePath+"/failureMessage.log", reportSpec.FailureMessage())
					writeLogInFile(filePath+"/failureLocation.log", reportSpec.FailureLocation().FullStackTrace)
				}
			}
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

func getClassnameFromReport(report types.SpecReport) string {
	texts := []string{}
	texts = append(texts, report.ContainerHierarchyTexts...)
	if report.LeafNodeText != "" {
		texts = append(texts, report.LeafNodeText)
	}
	if len(texts) > 0 {
		classStrings := strings.Fields(texts[0])
		return classStrings[0][1:]
	} else {
		return strings.Join(texts, " ")
	}
}

// This function is used to shorten classname and add hash to prevent issues with filesystems(255 chars for folder name) and to avoid conflicts(same shortened name of a classname)
func shortenStringAddHash(report types.SpecReport) string {
	className := getClassnameFromReport(report)
	s := report.FullText()
	replacedClass := strings.Replace(s, className, "", 1)
	if len(replacedClass) > 100 {
		stringToHash := replacedClass[100:]
		h := sha1.New()
		h.Write([]byte(stringToHash))
		sha1_hash := hex.EncodeToString(h.Sum(nil))
		stringWithHash := replacedClass[0:100] + " sha: " + sha1_hash
		return stringWithHash
	} else {
		return replacedClass
	}
}
