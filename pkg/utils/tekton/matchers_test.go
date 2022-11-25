package tekton

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

func TestTaskRunResultMatcherStringValue(t *testing.T) {
	match, err := MatchTaskRunResult("a", "b").Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: *v1beta1.NewArrayOrString("b"),
	})

	assert.True(t, match)
	assert.Nil(t, err)
}

func TestTaskRunResultMatcherJSONValue(t *testing.T) {
	match, err := MatchTaskRunResultWithJSONValue("a", `{"b":1}`).Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: *v1beta1.NewArrayOrString(`{ "b" : 1 }`),
	})

	assert.True(t, match)
	assert.Nil(t, err)
}

func TestMatchTaskRunResultWithJSONPathValue(t *testing.T) {
	match, err := MatchTaskRunResultWithJSONPathValue("a", "{$.c[0].d}", "[2]").Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: *v1beta1.NewArrayOrString(`{"b":1, "c": [{"d": 2}]}`),
	})

	assert.True(t, match)
	assert.Nil(t, err)
}

func TestMatchTaskRunResultWithJSONPathValueMultiple(t *testing.T) {
	match, err := MatchTaskRunResultWithJSONPathValue("a", "{$.c[*].d}", "[2, 1]").Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: *v1beta1.NewArrayOrString(`{"b":1, "c": [{"d": 2}, {"d": 1}]}`),
	})

	assert.True(t, match)
	assert.Nil(t, err)
}
