package logs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2/types"
)

func GetClassnameFromReport(report types.SpecReport) string {
	texts := report.ContainerHierarchyTexts
	if len(texts) > 0 {
		classStrings := strings.Fields(texts[0])
		return classStrings[0][1:]
	}
	return report.LeafNodeText
}

// This function is used to shorten classname and add hash to prevent issues with filesystems(255 chars for folder name) and to avoid conflicts(same shortened name of a classname)
func ShortenStringAddHash(report types.SpecReport) string {
	const maxNameLength = 209 // Max 255 chars minus SHA-1 (40 chars) and " sha: " is 6 chars => 255 - 40 - 6 = 209

	s := report.FullText()
	if s == "" {
		return ""
	}

	className := GetClassnameFromReport(report)
	reg := regexp.MustCompile(fmt.Sprintf("\\s*%s\\s*", className))
	removedClass := reg.ReplaceAllString(s, "")

	if len(removedClass) > maxNameLength {
		removedClass = removedClass[:maxNameLength]
	}
	h := sha1.New()
	h.Write([]byte(removedClass))
	sha1Hash := hex.EncodeToString(h.Sum(nil))
	return removedClass + " sha: " + sha1Hash
}
