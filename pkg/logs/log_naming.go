package logs

import (
	"crypto/sha256"
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
	const maxNameLength = 255 - sha256.BlockSize - 6 // Max 255 chars minus SHA-256 (64 chars) and " sha: " is 6 chars

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
	h := sha256.New()
	h.Write([]byte(removedClass))
	shaHash := hex.EncodeToString(h.Sum(nil))
	return removedClass + " sha: " + shaHash
}
