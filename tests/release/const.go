package release

// "time"

const (
	serviceAccountName = "m7-service-account"
	roleName           = "role-m7-service-account"
	roleBindingName    = "role-m7-service-account-binding"
	subjectKind        = "ServiceAccount"
	roleRefKind        = "Role"
	roleRefName        = "role-m7-service-account"
	roleRefApiGroup    = "rbac.authorization.k8s.io"
)
