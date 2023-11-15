package tekton

import (
	"knative.dev/pkg/apis"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// DidTaskRunSucceed checks if task succeeded.
func DidTaskRunSucceed(tr interface{}) bool {
	switch tr := tr.(type) {
	case *v1beta1.PipelineRunTaskRunStatus:
		return tr.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
	case *v1beta1.TaskRunStatus:
		return tr.Status.GetCondition(apis.ConditionSucceeded).IsTrue()
	}
	return false
}
