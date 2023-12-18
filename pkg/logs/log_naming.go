package logs

import (
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

func GetClassnameFromReport(report types.SpecReport) string {
	texts := report.ContainerHierarchyTexts
	if len(texts) > 0 {
		classStrings := strings.Fields(texts[0])
		return classStrings[0][1:len(classStrings[0])]
	}
	return report.LeafNodeText
}

// This function is used to shorten classname
func ShortenStringAddHash(report types.SpecReport) string {
	s := report.FullText()

	reg := regexp.MustCompile("\\[+.*]+\\s*")
	removedClass := reg.ReplaceAllString(s, "")

	return removedClass
}
