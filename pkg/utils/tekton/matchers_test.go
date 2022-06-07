package tekton

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func TestTaskRunResultMatcherStringValue(t *testing.T) {
	match, err := MatchTaskRunResult("a", "b").Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: "b",
	})

	assert.True(t, match)
	assert.Nil(t, err)
}

func TestTaskRunResultMatcherJSONValue(t *testing.T) {
	match, err := MatchTaskRunResultWithJSONValue("a", `{"b":1}`).Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: `{ "b" : 1 }`,
	})

	assert.True(t, match)
	assert.Nil(t, err)
}
