package status

import (
	"fmt"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/condition"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewComponentReadyCondition(reason string) *toolchainv1alpha1.Condition {
	currentTime := metav1.Now()
	return &toolchainv1alpha1.Condition{
		Type:               toolchainv1alpha1.ConditionReady,
		Status:             corev1.ConditionTrue,
		Reason:             reason,
		LastTransitionTime: currentTime,
		LastUpdatedTime:    &currentTime,
	}
}

func NewComponentErrorCondition(reason, msg string) *toolchainv1alpha1.Condition {
	currentTime := metav1.Now()
	return &toolchainv1alpha1.Condition{
		Type:               toolchainv1alpha1.ConditionReady,
		Status:             corev1.ConditionFalse,
		Reason:             reason,
		Message:            msg,
		LastTransitionTime: currentTime,
		LastUpdatedTime:    &currentTime,
	}
}

// ValidateComponentConditionReady checks whether the provided conditions signal that the component is ready, returns an error otherwise
func ValidateComponentConditionReady(conditions ...toolchainv1alpha1.Condition) error {
	c, found := condition.FindConditionByType(conditions, toolchainv1alpha1.ConditionReady)
	if !found {
		return fmt.Errorf("a ready condition was not found")
	} else if c.Status != corev1.ConditionTrue {
		return fmt.Errorf(c.Message) // return an error with the message from the condition
	}

	return nil
}
