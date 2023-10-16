package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionReason is a string representing a Kubernetes condition reason.
type ConditionReason string

func (cr ConditionReason) String() string {
	return string(cr)
}

// ConditionType is a string representing a Kubernetes condition type.
type ConditionType string

// String returns a string representation of the ConditionType.
func (ct ConditionType) String() string {
	return string(ct)
}

// SetCondition creates a new condition with the given conditionType, status and reason. Then, it sets this new condition,
// unsetting previous conditions with the same type as necessary.
func SetCondition(conditions *[]metav1.Condition, conditionType ConditionType, status metav1.ConditionStatus, reason ConditionReason) {
	SetConditionWithMessage(conditions, conditionType, status, reason, "")
}

// SetConditionWithMessage creates a new condition with the given conditionType, status, reason and message. Then, it sets this new condition,
// unsetting previous conditions with the same type as necessary.
func SetConditionWithMessage(conditions *[]metav1.Condition, conditionType ConditionType, status metav1.ConditionStatus, reason ConditionReason, message string) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:    conditionType.String(),
		Status:  status,
		Reason:  reason.String(),
		Message: message,
	})
}
