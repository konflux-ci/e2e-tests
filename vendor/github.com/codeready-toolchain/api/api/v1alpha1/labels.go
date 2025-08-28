package v1alpha1

const (
	// LabelKeyPrefix is a string prefix which will be added to all label and annotation keys
	LabelKeyPrefix = "toolchain.dev.openshift.com/"

	// StateLabelKey is used for setting the actual/expected state of an object like a UserSignup or a Space (not-ready, pending, banned, ...).
	// The main purpose of the label is easy selecting the objects based on the state - eg. get all UserSignups or Spaces on the waiting list (state=pending).
	// It may look like a duplication of the status conditions, but it more reflects the spec part combined with the actual state/configuration of the whole system.
	StateLabelKey = LabelKeyPrefix + "state"

	// StateLabelValuePending is used for identifying that the object is in a pending state.
	StateLabelValuePending = "pending"
)
