package tests

import (
	"fmt"

	. "github.com/onsi/gomega"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	
	"knative.dev/pkg/apis"
)

func ExpectPipelineRunNotToFail(pr *v1beta1.PipelineRun) {
	failed := pr.IsDone() && pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse()
	if failed {
		Expect(failed, fmt.Sprintf("did not expect pr %s:%s to fail", pr.Namespace, pr.Name)).NotTo(BeTrue())
	}
}
