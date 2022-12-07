package test

import (
	"fmt"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssertConditionsMatch asserts that the specified list A of conditions is equal to specified
// list B of conditions ignoring the order of the elements. We can't use assert.ElementsMatch
// because the LastTransitionTime of the actual conditions can be modified but the conditions
// still should be treated as matched
func AssertConditionsMatch(t T, actual []toolchainv1alpha1.Condition, expected ...toolchainv1alpha1.Condition) {
	require.Equal(t, len(expected), len(actual))
	for _, c := range expected {
		AssertContainsCondition(t, actual, c)
	}
}

// AssertContainsCondition asserts that the specified list of conditions contains the specified condition.
// LastTransitionTime is ignored.
func AssertContainsCondition(t T, conditions []toolchainv1alpha1.Condition, contains toolchainv1alpha1.Condition) {
	for _, c := range conditions {
		if c.Type == contains.Type {
			assert.Equal(t, contains.Status, c.Status)
			assert.Equal(t, contains.Reason, c.Reason)
			assert.Equal(t, contains.Message, c.Message)
			return
		}
	}
	assert.FailNow(t, fmt.Sprintf("the list of conditions %v doesn't contain the expected condition %v", conditions, contains))
}

// AssertConditionsMatchAndRecentTimestamps asserts that the specified list of conditions match AND asserts that the timestamps are recent
func AssertConditionsMatchAndRecentTimestamps(t T, actual []toolchainv1alpha1.Condition, expected ...toolchainv1alpha1.Condition) {
	AssertConditionsMatch(t, actual, expected...)
	AssertTimestampsAreRecent(t, actual)
}

// AssertTimestampsAreRecent asserts that the timestamps for the provided list of conditions are recent
func AssertTimestampsAreRecent(t T, conditions []toolchainv1alpha1.Condition) {
	var secs int64 = 5
	recentTime := metav1.Now().Add(time.Duration(-secs) * time.Second)
	for _, c := range conditions {
		assert.True(t, c.LastTransitionTime.After(recentTime), "LastTransitionTime was not updated within the last %d seconds", secs)
		assert.True(t, (*c.LastUpdatedTime).After(recentTime), "LastUpdatedTime was not updated within the last %d seconds", secs)
	}
}

// ConditionsMatch returns true if the specified list A of conditions is equal to specified
// list B of conditions ignoring the order of the elements
func ConditionsMatch(actual []toolchainv1alpha1.Condition, expected ...toolchainv1alpha1.Condition) bool {
	if len(expected) != len(actual) {
		return false
	}
	for _, c := range expected {
		if !ContainsCondition(actual, c) {
			return false
		}
	}
	for _, c := range actual {
		if !ContainsCondition(expected, c) {
			return false
		}
	}
	return true
}

// ContainsCondition returns true if the specified list of conditions contains the specified condition.
// LastTransitionTime is ignored.
func ContainsCondition(conditions []toolchainv1alpha1.Condition, contains toolchainv1alpha1.Condition) bool {
	for _, c := range conditions {
		if c.Type == contains.Type {
			return contains.Status == c.Status && contains.Reason == c.Reason && contains.Message == c.Message
		}
	}
	return false
}
