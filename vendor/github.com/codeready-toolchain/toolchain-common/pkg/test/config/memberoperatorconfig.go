package config

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MemberOperatorConfigOptionFunc func(config *toolchainv1alpha1.MemberOperatorConfig)

type MemberOperatorConfigOption interface {
	Apply(config *toolchainv1alpha1.MemberOperatorConfig)
}

type MemberOperatorConfigOptionImpl struct {
	toApply []MemberOperatorConfigOptionFunc
}

func (option *MemberOperatorConfigOptionImpl) Apply(config *toolchainv1alpha1.MemberOperatorConfig) {
	for _, apply := range option.toApply {
		apply(config)
	}
}

func (option *MemberOperatorConfigOptionImpl) addFunction(funcToAdd MemberOperatorConfigOptionFunc) {
	option.toApply = append(option.toApply, funcToAdd)
}

type AuthOption struct {
	*MemberOperatorConfigOptionImpl
}

func Auth() *AuthOption {
	o := &AuthOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Auth = toolchainv1alpha1.AuthConfig{}
	})
	return o
}

func (o AuthOption) Idp(value string) AuthOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Auth.Idp = &value
	})
	return o
}

type AutoscalerOption struct {
	*MemberOperatorConfigOptionImpl
}

func Autoscaler() *AutoscalerOption {
	o := &AutoscalerOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Autoscaler = toolchainv1alpha1.AutoscalerConfig{}
	})
	return o
}

func (o AutoscalerOption) Deploy(value bool) AutoscalerOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Autoscaler.Deploy = &value
	})
	return o
}

func (o AutoscalerOption) BufferMemory(value string) AutoscalerOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Autoscaler.BufferMemory = &value
	})
	return o
}

func (o AutoscalerOption) BufferReplicas(value int) AutoscalerOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Autoscaler.BufferReplicas = &value
	})
	return o
}

type CheOption struct {
	*MemberOperatorConfigOptionImpl
}

func Che() *CheOption {
	o := &CheOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che = toolchainv1alpha1.CheConfig{}
	})
	return o
}

func (o CheOption) Required(value bool) CheOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.Required = &value
	})
	return o
}

func (o CheOption) UserDeletionEnabled(value bool) CheOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.UserDeletionEnabled = &value
	})
	return o
}

func (o CheOption) KeycloakRouteName(value string) CheOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.KeycloakRouteName = &value
	})
	return o
}

func (o CheOption) Namespace(value string) CheOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.Namespace = &value
	})
	return o
}

func (o CheOption) RouteName(value string) CheOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.RouteName = &value
	})
	return o
}

type CheSecretOption struct {
	*MemberOperatorConfigOptionImpl
}

func (o CheOption) Secret() *CheSecretOption {
	c := &CheSecretOption{
		MemberOperatorConfigOptionImpl: o.MemberOperatorConfigOptionImpl,
	}
	return c
}

func (o CheSecretOption) Ref(value string) CheSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.Secret.Ref = &value
	})
	return o
}

func (o CheSecretOption) CheAdminUsernameKey(value string) CheSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.Secret.CheAdminUsernameKey = &value
	})
	return o
}

func (o CheSecretOption) CheAdminPasswordKey(value string) CheSecretOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Che.Secret.CheAdminPasswordKey = &value
	})
	return o
}

type ConsoleOption struct {
	*MemberOperatorConfigOptionImpl
}

func Console() *ConsoleOption {
	o := &ConsoleOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Console = toolchainv1alpha1.ConsoleConfig{}
	})
	return o
}

func (o ConsoleOption) Namespace(value string) ConsoleOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Console.Namespace = &value
	})
	return o
}

func (o ConsoleOption) RouteName(value string) ConsoleOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Console.RouteName = &value
	})
	return o
}

type MemberStatusOption struct {
	*MemberOperatorConfigOptionImpl
}

func MemberStatus() *MemberStatusOption {
	o := &MemberStatusOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.MemberStatus = toolchainv1alpha1.MemberStatusConfig{}
	})
	return o
}

func (o MemberStatusOption) RefreshPeriod(value string) MemberStatusOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.MemberStatus.RefreshPeriod = &value
	})
	return o
}

type ToolchainClusterOption struct {
	*MemberOperatorConfigOptionImpl
}

func ToolchainCluster() *ToolchainClusterOption {
	o := &ToolchainClusterOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.ToolchainCluster = toolchainv1alpha1.ToolchainClusterConfig{}
	})
	return o
}

func (o ToolchainClusterOption) HealthCheckPeriod(value string) ToolchainClusterOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.ToolchainCluster.HealthCheckPeriod = &value
	})
	return o
}

func (o ToolchainClusterOption) HealthCheckTimeout(value string) ToolchainClusterOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.ToolchainCluster.HealthCheckTimeout = &value
	})
	return o
}

type WebhookOption struct {
	*MemberOperatorConfigOptionImpl
}

func Webhook() *WebhookOption {
	o := &WebhookOption{
		MemberOperatorConfigOptionImpl: &MemberOperatorConfigOptionImpl{},
	}
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Webhook = toolchainv1alpha1.WebhookConfig{}
	})
	return o
}

func (o WebhookOption) Deploy(value bool) WebhookOption {
	o.addFunction(func(config *toolchainv1alpha1.MemberOperatorConfig) {
		config.Spec.Webhook.Deploy = &value
	})
	return o
}

func NewMemberOperatorConfigObj(options ...MemberOperatorConfigOption) *toolchainv1alpha1.MemberOperatorConfig {
	memberOperatorConfig := &toolchainv1alpha1.MemberOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: test.MemberOperatorNs,
			Name:      "config",
		},
	}
	for _, option := range options {
		option.Apply(memberOperatorConfig)
	}
	return memberOperatorConfig
}

func ModifyMemberOperatorConfigObj(memberOperatorConfig *toolchainv1alpha1.MemberOperatorConfig, options ...MemberOperatorConfigOption) *toolchainv1alpha1.MemberOperatorConfig {
	for _, option := range options {
		option.Apply(memberOperatorConfig)
	}
	return memberOperatorConfig
}
