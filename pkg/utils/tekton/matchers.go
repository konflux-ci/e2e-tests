package tekton

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

type TaskRunResultMatcher struct {
	name        string
	template    string
	value       *string
	jsonValue   *interface{}
	jsonMatcher types.GomegaMatcher
}

func applyTemplateToJson(jsonInput string, tmplString string) (string, error) {
	// Convert the json input to data
	var inputData interface{}
	err := json.Unmarshal([]byte(jsonInput), &inputData)
	if err != nil {
		return "", err
	}

	// Prepare the template
	tmpl, err := template.New("").Parse(tmplString)
	if err != nil {
		return "", err
	}

	// Apply the template to the input data and return the result
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, inputData)
	return buf.String(), err
}

func (matcher *TaskRunResultMatcher) FailureMessage(actual interface{}) (message string) {
	if matcher.value != nil {
		return fmt.Sprintf("%v to equal %v", actual, v1beta1.TaskRunResult{
			Name:  matcher.name,
			Value: *matcher.value,
		})
	}

	return matcher.jsonMatcher.FailureMessage(actual)
}

func (matcher *TaskRunResultMatcher) Match(actual interface{}) (success bool, err error) {
	if tr, ok := actual.(v1beta1.TaskRunResult); !ok {
		return false, fmt.Errorf("not given TaskRunResult")
	} else {
		if tr.Name != matcher.name {
			return false, nil
		}

		if matcher.value != nil {
			if matcher.template != "" {
				// If a template is provided then apply it to the task run result
				// value before doing the comparison
				valueForComparison, err := applyTemplateToJson(tr.Value, matcher.template)
				if err != nil {
					return false, err
				}
				return valueForComparison == *matcher.value, nil
			} else {
				return strings.TrimSpace(tr.Value) == *matcher.value, nil
			}

		} else {
			matcher.jsonMatcher = gomega.MatchJSON(*matcher.jsonValue)
			return matcher.jsonMatcher.Match(tr.Value)
		}
	}
}

func (matcher *TaskRunResultMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	if matcher.value != nil {
		return fmt.Sprintf("%v not to equal %v", actual, v1beta1.TaskRunResult{
			Name:  matcher.name,
			Value: strings.TrimSpace(*matcher.value),
		})
	}

	return matcher.jsonMatcher.NegatedFailureMessage(actual)
}

func MatchTaskRunResult(name, value string) types.GomegaMatcher {
	return &TaskRunResultMatcher{name: name, value: &value}
}

func MatchTaskRunResultWithJSONValue(name string, json interface{}) types.GomegaMatcher {
	return &TaskRunResultMatcher{name: name, jsonValue: &json}
}

func MatchTaskRunResultWithTemplate(name string, template string, value string) types.GomegaMatcher {
	return &TaskRunResultMatcher{name: name, template: template, value: &value}
}
