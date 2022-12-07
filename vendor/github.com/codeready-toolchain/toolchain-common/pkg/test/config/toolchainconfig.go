package config

import (
	"context"
	"os"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnvName string

const (
	Prod EnvName = "prod"
	E2E  EnvName = "e2e-tests"
	Dev  EnvName = "dev"
)

type ToolchainConfigOptionFunc func(config *toolchainv1alpha1.ToolchainConfig)

type ToolchainConfigOption interface {
	Apply(config *toolchainv1alpha1.ToolchainConfig)
}

type ToolchainConfigOptionImpl struct {
	toApply []ToolchainConfigOptionFunc
}

func (option *ToolchainConfigOptionImpl) Apply(config *toolchainv1alpha1.ToolchainConfig) {
	for _, apply := range option.toApply {
		apply(config)
	}
}

func (option *ToolchainConfigOptionImpl) addFunction(funcToAdd ToolchainConfigOptionFunc) {
	option.toApply = append(option.toApply, funcToAdd)
}

type PerMemberClusterOptionInt func(map[string]int)

func PerMemberCluster(name string, value int) PerMemberClusterOptionInt {
	return func(clusters map[string]int) {
		clusters[name] = value
	}
}

//---Host Configurations---//

type EnvironmentOption struct {
	*ToolchainConfigOptionImpl
}

// Environments: Prod, E2E, Dev
func Environment(value EnvName) *EnvironmentOption {
	o := &EnvironmentOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		val := string(value)
		config.Spec.Host.Environment = &val
	})
	return o
}

type AutomaticApprovalOption struct {
	*ToolchainConfigOptionImpl
}

func AutomaticApproval() *AutomaticApprovalOption {
	o := &AutomaticApprovalOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o AutomaticApprovalOption) Enabled(value bool) AutomaticApprovalOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.AutomaticApproval.Enabled = &value
	})
	return o
}

func (o AutomaticApprovalOption) ResourceCapacityThreshold(defaultThreshold int, perMember ...PerMemberClusterOptionInt) AutomaticApprovalOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.AutomaticApproval.ResourceCapacityThreshold.DefaultThreshold = &defaultThreshold
		config.Spec.Host.AutomaticApproval.ResourceCapacityThreshold.SpecificPerMemberCluster = map[string]int{}
		for _, add := range perMember {
			add(config.Spec.Host.AutomaticApproval.ResourceCapacityThreshold.SpecificPerMemberCluster)
		}
	})
	return o
}

func (o AutomaticApprovalOption) MaxNumberOfUsers(overall int, perMember ...PerMemberClusterOptionInt) AutomaticApprovalOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.AutomaticApproval.MaxNumberOfUsers.Overall = &overall
		config.Spec.Host.AutomaticApproval.MaxNumberOfUsers.SpecificPerMemberCluster = map[string]int{}
		for _, add := range perMember {
			add(config.Spec.Host.AutomaticApproval.MaxNumberOfUsers.SpecificPerMemberCluster)
		}
	})
	return o
}

type DeactivationOption struct {
	*ToolchainConfigOptionImpl
}

func Deactivation() *DeactivationOption {
	o := &DeactivationOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o DeactivationOption) DeactivatingNotificationDays(value int) DeactivationOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Deactivation.DeactivatingNotificationDays = &value
	})
	return o
}

func (o DeactivationOption) DeactivationDomainsExcluded(value string) DeactivationOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Deactivation.DeactivationDomainsExcluded = &value
	})
	return o
}

func (o DeactivationOption) UserSignupDeactivatedRetentionDays(value int) DeactivationOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Deactivation.UserSignupDeactivatedRetentionDays = &value
	})
	return o
}

func (o DeactivationOption) UserSignupUnverifiedRetentionDays(value int) DeactivationOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Deactivation.UserSignupUnverifiedRetentionDays = &value
	})
	return o
}

type MetricsOption struct {
	*ToolchainConfigOptionImpl
}

func Metrics() *MetricsOption {
	o := &MetricsOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o MetricsOption) ForceSynchronization(value bool) MetricsOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Metrics.ForceSynchronization = &value
	})
	return o
}

type NotificationsOption struct {
	*ToolchainConfigOptionImpl
}

func Notifications() *NotificationsOption {
	o := &NotificationsOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o NotificationsOption) NotificationDeliveryService(value string) NotificationsOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.NotificationDeliveryService = &value
	})
	return o
}

func (o NotificationsOption) DurationBeforeNotificationDeletion(value string) NotificationsOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.DurationBeforeNotificationDeletion = &value
	})
	return o
}

func (o NotificationsOption) AdminEmail(value string) NotificationsOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.AdminEmail = &value
	})
	return o
}

type NotificationSecretOption struct {
	*ToolchainConfigOptionImpl
}

func (o NotificationsOption) Secret() *NotificationSecretOption {
	c := &NotificationSecretOption{
		ToolchainConfigOptionImpl: o.ToolchainConfigOptionImpl,
	}
	return c
}

func (o NotificationSecretOption) Ref(value string) NotificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.Secret.Ref = &value
	})
	return o
}

func (o NotificationSecretOption) MailgunDomain(value string) NotificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.Secret.MailgunDomain = &value
	})
	return o
}

func (o NotificationSecretOption) MailgunAPIKey(value string) NotificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.Secret.MailgunAPIKey = &value
	})
	return o
}

func (o NotificationSecretOption) MailgunSenderEmail(value string) NotificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.Secret.MailgunSenderEmail = &value
	})
	return o
}

func (o NotificationSecretOption) MailgunReplyToEmail(value string) NotificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Notifications.Secret.MailgunReplyToEmail = &value
	})
	return o
}

type RegistrationServiceOption struct {
	*ToolchainConfigOptionImpl
}

func RegistrationService() *RegistrationServiceOption {
	o := &RegistrationServiceOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o RegistrationServiceOption) Environment(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Environment = &value
	})
	return o
}

func (o RegistrationServiceOption) LogLevel(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.LogLevel = &value
	})
	return o
}

func (o RegistrationServiceOption) Namespace(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Namespace = &value
	})
	return o
}

func (o RegistrationServiceOption) Replicas(value int32) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Replicas = &value
	})
	return o
}

func (o RegistrationServiceOption) RegistrationServiceURL(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.RegistrationServiceURL = &value
	})
	return o
}

func (o RegistrationServiceOption) Analytics() RegistrationServiceAnalyticsOption {
	c := RegistrationServiceAnalyticsOption{
		ToolchainConfigOptionImpl: o.ToolchainConfigOptionImpl,
		parent:                    o,
	}
	return c
}

func (o RegistrationServiceOption) Auth() RegistrationServiceAuthOption {
	c := RegistrationServiceAuthOption{
		ToolchainConfigOptionImpl: o.ToolchainConfigOptionImpl,
		parent:                    o,
	}
	return c
}

func (o RegistrationServiceOption) Verification() RegistrationServiceVerificationOption {
	c := RegistrationServiceVerificationOption{
		ToolchainConfigOptionImpl: o.ToolchainConfigOptionImpl,
		parent:                    o,
	}
	return c
}

type RegistrationServiceAnalyticsOption struct {
	*ToolchainConfigOptionImpl
	parent RegistrationServiceOption
}

func (o RegistrationServiceAnalyticsOption) SegmentWriteKey(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Analytics.SegmentWriteKey = &value
	})
	return o.parent
}

func (o RegistrationServiceAnalyticsOption) WoopraDomain(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Analytics.WoopraDomain = &value
	})
	return o.parent
}

type RegistrationServiceAuthOption struct {
	*ToolchainConfigOptionImpl
	parent RegistrationServiceOption
}

func (o RegistrationServiceAuthOption) AuthClientConfigContentType(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Auth.AuthClientConfigContentType = &value
	})
	return o.parent
}

func (o RegistrationServiceAuthOption) AuthClientLibraryURL(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Auth.AuthClientLibraryURL = &value
	})
	return o.parent
}

func (o RegistrationServiceAuthOption) AuthClientConfigRaw(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Auth.AuthClientConfigRaw = &value
	})
	return o.parent
}

func (o RegistrationServiceAuthOption) AuthClientPublicKeysURL(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Auth.AuthClientPublicKeysURL = &value
	})
	return o.parent
}

type RegistrationServiceVerificationOption struct {
	*ToolchainConfigOptionImpl
	parent RegistrationServiceOption
}

func (o RegistrationServiceVerificationOption) Enabled(value bool) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.Enabled = &value
	})
	return o.parent
}

func (o RegistrationServiceVerificationOption) DailyLimit(value int) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.DailyLimit = &value
	})
	return o.parent
}

func (o RegistrationServiceVerificationOption) AttemptsAllowed(value int) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.AttemptsAllowed = &value
	})
	return o.parent
}

func (o RegistrationServiceVerificationOption) MessageTemplate(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.MessageTemplate = &value
	})
	return o.parent
}

func (o RegistrationServiceVerificationOption) ExcludedEmailDomains(value string) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.ExcludedEmailDomains = &value
	})
	return o.parent
}

func (o RegistrationServiceVerificationOption) CodeExpiresInMin(value int) RegistrationServiceOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.CodeExpiresInMin = &value
	})
	return o.parent
}

func (o RegistrationServiceVerificationOption) Secret() *RegistrationVerificationSecretOption {
	c := &RegistrationVerificationSecretOption{
		ToolchainConfigOptionImpl: o.ToolchainConfigOptionImpl,
	}
	return c
}

type RegistrationVerificationSecretOption struct {
	*ToolchainConfigOptionImpl
}

func (o RegistrationVerificationSecretOption) Ref(value string) RegistrationVerificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.Secret.Ref = &value
	})
	return o
}

func (o RegistrationVerificationSecretOption) TwilioAccountSID(value string) *RegistrationVerificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.Secret.TwilioAccountSID = &value
	})
	return &o
}

func (o RegistrationVerificationSecretOption) TwilioAuthToken(value string) *RegistrationVerificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.Secret.TwilioAuthToken = &value
	})
	return &o
}

func (o RegistrationVerificationSecretOption) TwilioFromNumber(value string) *RegistrationVerificationSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.RegistrationService.Verification.Secret.TwilioFromNumber = &value
	})
	return &o
}

type TiersOption struct {
	*ToolchainConfigOptionImpl
}

func Tiers() *TiersOption {
	o := &TiersOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o TiersOption) DefaultTier(value string) TiersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Tiers.DefaultTier = &value
	})
	return o
}

func (o TiersOption) DefaultSpaceTier(value string) TiersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Tiers.DefaultSpaceTier = &value
	})
	return o
}

func (o TiersOption) DurationBeforeChangeTierRequestDeletion(value string) TiersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Tiers.DurationBeforeChangeTierRequestDeletion = &value
	})
	return o
}

type ToolchainStatusOption struct {
	*ToolchainConfigOptionImpl
}

func ToolchainStatus() *ToolchainStatusOption {
	o := &ToolchainStatusOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o ToolchainStatusOption) ToolchainStatusRefreshTime(value string) ToolchainStatusOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.ToolchainStatus.ToolchainStatusRefreshTime = &value
	})
	return o
}

type UsersOption struct {
	*ToolchainConfigOptionImpl
}

func Users() *UsersOption {
	o := &UsersOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o UsersOption) MasterUserRecordUpdateFailureThreshold(value int) UsersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Users.MasterUserRecordUpdateFailureThreshold = &value
	})
	return o
}

func (o UsersOption) ForbiddenUsernamePrefixes(value string) UsersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Users.ForbiddenUsernamePrefixes = &value
	})
	return o
}

func (o UsersOption) ForbiddenUsernameSuffixes(value string) UsersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Host.Users.ForbiddenUsernameSuffixes = &value
	})
	return o
}

//---End of Host Configurations---//

//---Member Configurations---//
type MembersOption struct {
	*ToolchainConfigOptionImpl
}

func Members() *MembersOption {
	o := &MembersOption{
		ToolchainConfigOptionImpl: &ToolchainConfigOptionImpl{},
	}
	return o
}

func (o MembersOption) Default(memberConfigSpec toolchainv1alpha1.MemberOperatorConfigSpec) MembersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		config.Spec.Members.Default = memberConfigSpec
	})
	return o
}

func (o MembersOption) SpecificPerMemberCluster(clusterName string, memberConfigSpec toolchainv1alpha1.MemberOperatorConfigSpec) MembersOption {
	o.addFunction(func(config *toolchainv1alpha1.ToolchainConfig) {
		if config.Spec.Members.SpecificPerMemberCluster == nil {
			config.Spec.Members.SpecificPerMemberCluster = make(map[string]toolchainv1alpha1.MemberOperatorConfigSpec)
		}
		config.Spec.Members.SpecificPerMemberCluster[clusterName] = memberConfigSpec
	})
	return o
}

//---End of Member Configurations---//

func NewToolchainConfigObj(t *testing.T, options ...ToolchainConfigOption) *toolchainv1alpha1.ToolchainConfig {
	namespace, found := os.LookupEnv("WATCH_NAMESPACE")
	if !found {
		t.Logf("WATCH_NAMESPACE env var is not set, defaulting to '%s'", test.HostOperatorNs)
		namespace = test.HostOperatorNs
	}
	toolchainConfig := &toolchainv1alpha1.ToolchainConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "config",
		},
	}
	for _, option := range options {
		option.Apply(toolchainConfig)
	}
	return toolchainConfig
}

func ModifyToolchainConfigObj(t *testing.T, cl client.Client, options ...ToolchainConfigOption) *toolchainv1alpha1.ToolchainConfig {
	namespace, found := os.LookupEnv("WATCH_NAMESPACE")
	if !found {
		t.Log("WATCH_NAMESPACE env var is not set")
		namespace = test.HostOperatorNs
	}
	currentConfig := &toolchainv1alpha1.ToolchainConfig{}
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "config"}, currentConfig)
	require.NoError(t, err)

	for _, option := range options {
		option.Apply(currentConfig)
	}
	return currentConfig
}
