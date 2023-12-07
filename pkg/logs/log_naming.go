package logs

import (
	"strings"

	types "github.com/onsi/ginkgo/v2/types"
)

func GetClassnameFromReport(report types.SpecReport) string {
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
func ShortenStringAddHash(report types.SpecReport) string {
	className := GetClassnameFromReport(report)
	s := report.FullText()
	replacedClass := strings.Replace(s, className, "", 1)
	if len(replacedClass) > 255 {
		return replacedClass[0:255]
	}
	return replacedClass
}
