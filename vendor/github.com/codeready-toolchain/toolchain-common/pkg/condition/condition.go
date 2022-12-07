package condition

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddOrUpdateStatusConditions appends the new conditions to the condition slice. If there is already a condition
// with the same type in the current condition array then the condition is updated in the result slice.
// If the condition is not changed then the same unmodified slice is returned.
// Also returns a bool flag which indicates if the conditions where updated/added
func AddOrUpdateStatusConditions(conditions []toolchainv1alpha1.Condition, newConditions ...toolchainv1alpha1.Condition) ([]toolchainv1alpha1.Condition, bool) {
	return addOrUpdateStatusConditions(conditions, false, newConditions...)
}

// AddOrUpdateStatusConditionsWithLastUpdatedTimestamp appends the new conditions to the condition slice. If there is already a condition
// with the same type in the current condition array then the condition is updated in the result slice.
// The condition's LastUpdatedTime is always updated to the current time even if nothing else is changed.
func AddOrUpdateStatusConditionsWithLastUpdatedTimestamp(conditions []toolchainv1alpha1.Condition, newConditions ...toolchainv1alpha1.Condition) []toolchainv1alpha1.Condition {
	cs, _ := addOrUpdateStatusConditions(conditions, true, newConditions...)
	return cs
}

func addOrUpdateStatusConditions(conditions []toolchainv1alpha1.Condition, updateLastUpdatedTimestamp bool, newConditions ...toolchainv1alpha1.Condition) ([]toolchainv1alpha1.Condition, bool) {
	var atLeastOneUpdated bool
	var updated bool
	for _, cond := range newConditions {
		conditions, updated = addOrUpdateStatusCondition(conditions, cond, updateLastUpdatedTimestamp)
		atLeastOneUpdated = atLeastOneUpdated || updated
	}

	return conditions, atLeastOneUpdated
}

// AddStatusConditions adds the given conditions *without* checking for duplicate types (as opposed to `AddOrUpdateStatusConditions`)
// Also, it sets the `LastTransitionTime` to `metav1.Now()` for each given condition if needed
func AddStatusConditions(conditions []toolchainv1alpha1.Condition, newConditions ...toolchainv1alpha1.Condition) []toolchainv1alpha1.Condition {
	for _, cond := range newConditions {
		if cond.LastTransitionTime.IsZero() {
			cond.LastTransitionTime = metav1.Now()
		}
		conditions = append(conditions, cond)
	}
	return conditions
}

// FindConditionByType returns first Condition with given conditionType
// along with bool flag which indicates if the Condition is found or not
func FindConditionByType(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType) (toolchainv1alpha1.Condition, bool) {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition, true
		}
	}
	return toolchainv1alpha1.Condition{}, false
}

// IsTrue returns `true` if the condition with the given condition type is found among the conditions
// and its status is set to `true`.
// Returns false for unknown conditions and conditions with status set to False.
func IsTrue(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType) bool {
	c, found := FindConditionByType(conditions, conditionType)
	return found && c.Status == apiv1.ConditionTrue
}

// IsFalse returns `true` if the condition with the given condition type is found among the conditions
// and its status is set to `false`.
// Returns false for unknown conditions and conditions with status set to True.
func IsFalse(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType) bool {
	c, found := FindConditionByType(conditions, conditionType)
	return found && c.Status == apiv1.ConditionFalse
}

// IsNotTrue returns `true` if the condition with the given condition type has an Unknown or `false`` status
func IsNotTrue(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType) bool {
	c, found := FindConditionByType(conditions, conditionType)
	return !found || c.Status != apiv1.ConditionTrue
}

// IsFalseWithReason returns `true` if the condition with the given condition type is found among the conditions
// and its status is set to `false` with the given reason.
func IsFalseWithReason(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType, reason string) bool {
	c, found := FindConditionByType(conditions, conditionType)
	return found && c.Status == apiv1.ConditionFalse && c.Reason == reason
}

// IsTrueWithReason returns `true` if the condition with the given condition type is found among the conditions
// and its status is set to `true` with the given reason.
func IsTrueWithReason(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType, reason string) bool {
	c, found := FindConditionByType(conditions, conditionType)
	return found && c.Status == apiv1.ConditionTrue && c.Reason == reason
}

// Count counts the conditions that match the given type/status/reason
func Count(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType, status apiv1.ConditionStatus, reason string) int {
	count := 0
	for _, c := range conditions {
		if c.Type == conditionType &&
			c.Status == status &&
			c.Reason == reason {
			count++
		}
	}
	return count
}

func addOrUpdateStatusCondition(conditions []toolchainv1alpha1.Condition, newCondition toolchainv1alpha1.Condition, updateLastUpdatedTimestamp bool) ([]toolchainv1alpha1.Condition, bool) {
	now := metav1.Now()
	newCondition.LastTransitionTime = now
	if updateLastUpdatedTimestamp {
		newCondition.LastUpdatedTime = &now
	}

	if conditions == nil {
		return []toolchainv1alpha1.Condition{newCondition}, true
	}
	for i, cond := range conditions {
		if cond.Type == newCondition.Type {
			// Condition already present. Update it if needed. Always update if "updateLastUpdatedTimestamp" is set to true.
			if !updateLastUpdatedTimestamp &&
				cond.Status == newCondition.Status &&
				cond.Reason == newCondition.Reason &&
				cond.Message == newCondition.Message {
				// Nothing changed. No need to update.
				return conditions, false
			}

			// Update LastTransitionTime only if the status changed otherwise keep the old time
			if newCondition.Status == cond.Status {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}
			// Don't modify the currentConditions slice. Generate a new slice instead.
			res := make([]toolchainv1alpha1.Condition, len(conditions))
			copy(res, conditions)
			res[i] = newCondition
			return res, true
		}
	}
	return append(conditions, newCondition), true
}

// HasConditionReason returns true if the first Condition with given conditionType from the given slice has the specified reason
func HasConditionReason(conditions []toolchainv1alpha1.Condition, conditionType toolchainv1alpha1.ConditionType, reason string) bool {
	con, found := FindConditionByType(conditions, conditionType)
	return found && con.Reason == reason
}
