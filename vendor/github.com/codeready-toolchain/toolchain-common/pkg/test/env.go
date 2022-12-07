package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// UnsetEnvVarAndRestore unsets the given environment variable with the key (if
// present). It returns a function to be called whenever you want to restore the
// original environment (ideally using defer).
//
// In a test you can use this to temporarily set an environment variable:
//
//    func TestFoo(t *testing.T) {
//        restoreFunc := UnsetEnvVarAndRestore(t, "foo")
//        defer restoreFunc()
//        os.Setenv(key, "bar")
//
//        // continue as if foo=bar
//    }
func UnsetEnvVarAndRestore(t *testing.T, key string) func() {
	realEnvValue, present := os.LookupEnv(key)
	err := os.Unsetenv(key)
	require.NoError(t, err)
	return func() {
		if present {
			err := os.Setenv(key, realEnvValue)
			require.NoError(t, err)
		} else {
			err := os.Unsetenv(key)
			require.NoError(t, err)
		}
	}
}

// SetEnvVarAndRestore sets the given environment variable with the key to the
// given value. It returns a function to be called whenever you want to restore
// the original environment (ideally using defer).
//
// In a test you can use this to set an environment variable:
//
//    func TestFoo(t *testing.T) {
//        restoreFunc := SetEnvVarAndRestore(t, "foo", "bar")
//        defer restoreFunc()
//        // continue as if foo=bar
//    }
func SetEnvVarAndRestore(t *testing.T, key, newValue string) func() {
	oldEnvValue, present := os.LookupEnv(key)
	err := os.Setenv(key, newValue)
	require.NoError(t, err)
	return func() {
		if present {
			err := os.Setenv(key, oldEnvValue)
			require.NoError(t, err)
		} else {
			err := os.Unsetenv(key)
			require.NoError(t, err)
		}
	}
}

type EnvVariable func() (key, value string)

// SetEnvVarsAndRestore sets the given environment variables with the keys to the
// given values. It returns a function to be called whenever you want to restore
// the original environment (ideally using defer).
//
// In a test you can use this to set an environment variables:
//
//    func TestFoo(t *testing.T) {
//        restoreFunc := SetEnvVarsAndRestore(t, Env("foo", "bar"), Env("boo", "far"))
//        defer restoreFunc()
//        // continue as if foo=bar and boo=far
//    }
func SetEnvVarsAndRestore(t *testing.T, envs ...EnvVariable) func() {
	var restores []func()
	for _, envVar := range envs {
		key, value := envVar()
		restores = append(restores, SetEnvVarAndRestore(t, key, value))
	}
	return func() {
		for _, restore := range restores {
			restore()
		}
	}
}

func Env(key, value string) EnvVariable {
	return func() (string, string) {
		return key, value
	}
}
