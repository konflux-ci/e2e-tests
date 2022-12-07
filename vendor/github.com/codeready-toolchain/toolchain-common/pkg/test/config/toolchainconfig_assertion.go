package config

import (
	"context"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ToolchainConfigAssertion struct {
	config         *toolchainv1alpha1.ToolchainConfig
	client         client.Client
	namespacedName types.NamespacedName
	t              test.T
}

func (a *ToolchainConfigAssertion) loadToolchainConfig() error {
	toolchainConfig := &toolchainv1alpha1.ToolchainConfig{}
	err := a.client.Get(context.TODO(), a.namespacedName, toolchainConfig)
	a.config = toolchainConfig
	return err
}

func AssertThatToolchainConfig(t test.T, namespace string, client client.Client) *ToolchainConfigAssertion {
	return &ToolchainConfigAssertion{
		client:         client,
		namespacedName: test.NamespacedName(namespace, "config"),
		t:              t,
	}
}

func (a *ToolchainConfigAssertion) NotExists() *ToolchainConfigAssertion {
	err := a.loadToolchainConfig()
	require.Error(a.t, err)
	require.True(a.t, errors.IsNotFound(err))
	return a
}

func (a *ToolchainConfigAssertion) Exists() *ToolchainConfigAssertion {
	err := a.loadToolchainConfig()
	require.NoError(a.t, err)
	return a
}

func (a *ToolchainConfigAssertion) HasConditions(expected ...toolchainv1alpha1.Condition) *ToolchainConfigAssertion {
	err := a.loadToolchainConfig()
	require.NoError(a.t, err)
	test.AssertConditionsMatch(a.t, a.config.Status.Conditions, expected...)
	return a
}

func (a *ToolchainConfigAssertion) HasNoSyncErrors() *ToolchainConfigAssertion {
	err := a.loadToolchainConfig()
	require.NoError(a.t, err)
	require.Empty(a.t, a.config.Status.SyncErrors)
	return a
}

func (a *ToolchainConfigAssertion) HasSyncErrors(expectedSyncErrors map[string]string) *ToolchainConfigAssertion {
	err := a.loadToolchainConfig()
	require.NoError(a.t, err)
	require.Equal(a.t, expectedSyncErrors, a.config.Status.SyncErrors)
	return a
}
