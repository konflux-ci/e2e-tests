package v1alpha1

const (
	// AnnotationKeyPrefix is the prefix used for annotation key values
	AnnotationKeyPrefix = LabelKeyPrefix

	// SSOUserIDAnnotationKey is used to store the user's user_id claim value issued by the SSO provider
	SSOUserIDAnnotationKey = AnnotationKeyPrefix + "sso-user-id"

	// SSOAccountIDAnnotationKey is used to store the user's account_id claim value issued by the SSO provider
	SSOAccountIDAnnotationKey = AnnotationKeyPrefix + "sso-account-id"
)
