/*
This is modified JUnit XML Reporter for Ginkgo.
Original version is available on https://github.com/onsi/ginkgo/blob/master/reporters/junit_report.go
*/

package framework

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2/reporters"
	types "github.com/onsi/ginkgo/v2/types"
	"k8s.io/klog/v2"
)

func GenerateCustomJUnitReport(report types.Report, dst string) error {
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
				{"EmitSpecProgress", fmt.Sprintf("%t", report.SuiteConfig.EmitSpecProgress)},
				{"DryRun", fmt.Sprintf("%t", report.SuiteConfig.DryRun)},
				{"ParallelTotal", fmt.Sprintf("%d", report.SuiteConfig.ParallelTotal)},
				{"OutputInterceptorMode", report.SuiteConfig.OutputInterceptorMode},
			},
		},
	}
	for _, spec := range report.SpecReports {
		name := spec.FullText()
		labels := spec.Labels()
		if len(labels) > 0 {
			name = name + " [" + strings.Join(labels, ", ") + "]"
		}

		test := JUnitTestCase{
			Name:      shortenStringAddHash(spec),
			Classname: getClassnameFromReport(spec),
			Status:    spec.State.String(),
			Time:      spec.RunTime.Seconds(),
			SystemOut: systemOutForUnstructuredReporters(spec),
			SystemErr: systemErrForUnstructuredReporters(spec),
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
	f.WriteString(xml.Header)
	encoder := xml.NewEncoder(f)
	encoder.Indent("  ", "    ")
	encoder.Encode(junitReport)

	return f.Close()
}

// This function generates folder structure for the rp_preproc tool with logs for upload in Report Portal
func GenerateRPPreprocReport(report types.Report) {
	for i := range report.SpecReports {
		reportSpec := report.SpecReports[i]
		//generate folders only for failed tests
		if !reportSpec.Failure.IsZero() {
			if reportSpec.LeafNodeType == types.NodeTypeIt {
				name := getClassnameFromReport(reportSpec) + "." + shortenStringAddHash(reportSpec)
				filePath := "rp_preproc/attachments/xunit/" + name
				if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
					klog.Fatal(err)
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
			klog.Fatal(err)
		}
		defer f.Close()

		_, err2 := f.WriteString(log)

		if err2 != nil {
			klog.Fatal(err2)
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
		if cursor < pr.GinkgoWriterOffset {
			if pr.GinkgoWriterOffset < len(gw) {
				out.WriteString(gw[cursor:pr.GinkgoWriterOffset])
				cursor = pr.GinkgoWriterOffset
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
	systemOut := spec.CapturedStdOutErr
	if len(spec.ReportEntries) > 0 {
		systemOut += "\nReport Entries:\n"
		for i, entry := range spec.ReportEntries {
			systemOut += fmt.Sprintf("%s\n%s\n%s\n", entry.Name, entry.Location, entry.Time.Format(time.RFC3339Nano))
			if representation := entry.StringRepresentation(); representation != "" {
				systemOut += representation + "\n"
			}
			if i+1 < len(spec.ReportEntries) {
				systemOut += "--\n"
			}
		}
	}
	return systemOut
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
