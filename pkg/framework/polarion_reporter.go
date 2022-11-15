package framework

import (
	"flag"
	"strings"

	types "github.com/onsi/ginkgo/v2/types"
	polarion_xml "kubevirt.io/qe-tools/pkg/polarion-xml"
)

func GeneratePolarionReport(report types.Report, outputFile string, polarionProjectID string) {
	dryRun := flag.String("dry-run", "false", "Dry-run property")

	var testCases = &polarion_xml.TestCases{
		ProjectID: polarionProjectID,
		Properties: polarion_xml.PolarionProperties{
			Property: []polarion_xml.PolarionProperty{
				{
					Name:  "lookup-method",
					Value: "custom",
				},
				{
					Name:  "custom-lookup-method-field-id",
					Value: "customId",
				},
				{
					Name:  "dry-run",
					Value: *dryRun,
				},
			},
		},
	}

	type PolarionCase struct {
		testCase string
		filename string
		labels   string
		steps    []string
	}

	polarionCaseMap := make(map[string]PolarionCase)
	for i := range report.SpecReports {
		specReport := report.SpecReports[i]
		if specReport.LeafNodeType == types.NodeTypeIt {
			if polarionCaseMod, found := polarionCaseMap[specReport.ContainerHierarchyTexts[0]]; found {
				polarionCaseMod.steps = append(polarionCaseMod.steps, strings.Join(append(specReport.ContainerHierarchyTexts[1:], specReport.LeafNodeText, ""), ""))
				polarionCaseMap[specReport.ContainerHierarchyTexts[0]] = polarionCaseMod
			} else {
				polarionCase := PolarionCase{specReport.ContainerHierarchyTexts[0], specReport.LeafNodeLocation.FileName, strings.Join(specReport.ContainerHierarchyLabels[0], ","), append([]string{}, strings.Join(append(specReport.ContainerHierarchyTexts[1:], specReport.LeafNodeText, ""), ""))}
				polarionCaseMap[specReport.ContainerHierarchyTexts[0]] = polarionCase
			}
		}
	}

	for testCaseSpec, testPolarionCase := range polarionCaseMap {
		testCase := &polarion_xml.TestCase{
			Title:       polarion_xml.Title{Content: testCaseSpec},
			Description: polarion_xml.Description{Content: testCaseSpec},
		}

		customFields := polarion_xml.TestCaseCustomFields{}
		addCustomField(&customFields, "caseautomation", "automated")
		addCustomField(&customFields, "testtype", "functional")
		addCustomField(&customFields, "automation_script", parseTestSourceFromFilename(testPolarionCase.filename))
		addCustomField(&customFields, "tags", testPolarionCase.labels)
		testCase.TestCaseCustomFields = customFields

		for _, testStep := range testPolarionCase.steps {
			if testCase.TestCaseSteps == nil {
				testCase.TestCaseSteps = &polarion_xml.TestCaseSteps{}
			}
			addTestStep(testCase.TestCaseSteps, testStep, true)
		}
		parseTagsFromTitle(testCase, testCaseSpec, parseComponentFromFilename(testPolarionCase.filename), testCases.ProjectID)
		testCases.TestCases = append(testCases.TestCases, *testCase)
	}
	// generate polarion test cases XML file
	polarion_xml.GeneratePolarionXmlFile(outputFile, testCases)
}

func addCustomField(customFields *polarion_xml.TestCaseCustomFields, id, content string) {
	customFields.CustomFields = append(
		customFields.CustomFields, polarion_xml.TestCaseCustomField{
			Content: content,
			ID:      id,
		})
}

func addTestStep(testCaseSteps *polarion_xml.TestCaseSteps, content string, prepend bool) {
	testCaseStep := polarion_xml.TestCaseStep{
		StepColumn: []polarion_xml.TestCaseStepColumn{
			{
				Content: content,
				ID:      "step",
			},
			{
				Content: "Succeeded",
				ID:      "expectedResult",
			},
		},
	}
	if prepend {
		testCaseSteps.Steps = append([]polarion_xml.TestCaseStep{testCaseStep}, testCaseSteps.Steps...)
	} else {
		testCaseSteps.Steps = append(testCaseSteps.Steps, testCaseStep)
	}
}

// How to use these tags is described here: https://github.com/redhat-appstudio/e2e-tests/blob/main/docs/LabelsNaming.md#tests-naming
func parseTagsFromTitle(testCase *polarion_xml.TestCase, title string, component string, projectID string) {
	posneg := "positive"
	caselevel := "component"
	criticality := "medium"
	rfeID := ""
	customID := ""
	title = strings.Replace(title, "]", ",", -1)
	title = strings.Replace(title, "[", ",", -1)
	attrList := strings.Split(title, ",")

	for i := 0; i < len(attrList); i++ {
		attrList[i] = strings.Trim(attrList[i], " ")
		if strings.Contains(attrList[i], "test_id:") {
			testID := strings.Split(strings.Trim(strings.Split(attrList[i], "test_id:")[1], " "), " ")[0]
			testCase.ID = projectID + "-" + testID
			customID = projectID + "-" + testID
		} else if strings.Contains(attrList[i], "rfe_id:") {
			rfeID = projectID + "-" + strings.Split(strings.Trim(strings.Split(attrList[i], "rfe_id:")[1], " "), " ")[0]
		} else if strings.Contains(attrList[i], "crit:") {
			criticality = strings.Split(strings.Trim(strings.Split(attrList[i], "crit:")[1], " "), " ")[0]

		} else if strings.Contains(attrList[i], "posneg:") {
			posneg = strings.Split(strings.Trim(strings.Split(attrList[i], "posneg:")[1], " "), " ")[0]

		} else if strings.Contains(attrList[i], "level:") {
			caselevel = strings.Split(strings.Trim(strings.Split(attrList[i], "level:")[1], " "), " ")[0]

		} else if strings.Contains(attrList[i], "component:") {
			component = strings.Split(strings.Trim(strings.Split(attrList[i], "component:")[1], " "), " ")[0]
		}
	}

	addCustomField(&testCase.TestCaseCustomFields, "customId", customID)
	addCustomField(&testCase.TestCaseCustomFields, "caseimportance", criticality)
	addCustomField(&testCase.TestCaseCustomFields, "caseposneg", posneg)
	addCustomField(&testCase.TestCaseCustomFields, "caselevel", caselevel)
	addLinkedWorkItem(&testCase.TestCaseLinkedWorkItems, rfeID)
	if component != "" {
		addCustomField(&testCase.TestCaseCustomFields, "casecomponent", component)
	}
}

func addLinkedWorkItem(linkedWorkItems *polarion_xml.TestCaseLinkedWorkItems, id string) {
	linkedWorkItems.LinkedWorkItems = append(
		linkedWorkItems.LinkedWorkItems, polarion_xml.TestCaseLinkedWorkItem{
			ID:   id,
			Role: "verifies",
		})
}

func parseComponentFromFilename(filename string) string {
	return strings.Split(parseTestSourceFromFilename(filename), "/")[3]
}

func parseTestSourceFromFilename(filename string) string {
	n := strings.LastIndex(filename, "/e2e-tests/tests/")
	return filename[n:]
}
