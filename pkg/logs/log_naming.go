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
		firstClass := classStrings[0]
		reg := regexp.MustCompile("^\\s*\\[+\\s*|\\s*]+\\s*$") // Remove whitespace and square brackets on both sides
		return reg.ReplaceAllString(firstClass, "")
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
