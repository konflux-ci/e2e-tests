// backup_suite_test.go provides a test runner for running the backup suite
// directly via `go test ./tests/backup/` or `ginkgo ./tests/backup/`.
//
// When running from cmd/ (the standard CI path via `ginkgo ./cmd/`), this
// file is NOT compiled — Ginkgo discovers the specs via the blank import in
// cmd/e2e_test.go instead, and each Describe block handles its own setup
// in BeforeAll.
package backup

import (
	"testing"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck
	. "github.com/onsi/gomega"    //nolint:staticcheck
)

func TestBackup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DR backup/restore e2e suite")
}
