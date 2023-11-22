package tekton

import (
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPipelineExtraction(t *testing.T) {
	var defaultBundleRef string
	var err error
	if defaultBundleRef, err = GetDefaultPipelineBundleRef(constants.BuildPipelineSelectorYamlURL, "Java"); err != nil {
		assert.Error(t, err, "failed to parse bundle ref")
		panic(err)
	}
	assert.Contains(t, defaultBundleRef, "pipeline-java-builder", "failed to retrieve bundle ref")
}
