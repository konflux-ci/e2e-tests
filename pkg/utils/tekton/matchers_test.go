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

func TestApplyTemplateToJson(t *testing.T) {
	input := `{"words": {"greetings": ["hola", "bonjour"]}}`
	template := "{{ index .words.greetings 1 }} {{ .notThere }}"
	output, err := applyTemplateToJson(input, template)

	assert.Equal(t, output, "bonjour <no value>")
	assert.Nil(t, err)
}

func TestApplyTemplateToJsonWithList(t *testing.T) {
	input := `[{"result":"ok", "score": 42 }]`
	template := "{{ (index . 0).result }} {{ (index . 0).score }}"
	output, err := applyTemplateToJson(input, template)

	assert.Equal(t, output, "ok 42")
	assert.Nil(t, err)
}

func TestTaskRunResultWithTemplate(t *testing.T) {
	match, err := MatchTaskRunResultWithTemplate("a", "{{.b}}", "c").Match(v1beta1.TaskRunResult{
		Name:  "a",
		Value: `{ "b" : "c" }`,
	})

	assert.True(t, match)
	assert.Nil(t, err)
}
