package wait

import (
	"context"
	"time"

	"github.com/codeready-toolchain/toolchain-e2e/setup/configuration"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ForNamespace(cl client.Client, namespace string) error {
	ns := &corev1.Namespace{}
	if err := k8swait.Poll(configuration.DefaultRetryInterval, configuration.DefaultTimeout, func() (bool, error) {
		err := cl.Get(context.TODO(), types.NamespacedName{
			Name: namespace,
		}, ns)
		if k8serrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		return errors.Wrapf(err, "namespace '%s' does not exist", namespace)
	}
	return nil
}

func HasSubscriptionWithCriteria(cl client.Client, name, namespace string, criteria ...subCriteria) (bool, error) {
	sub := &v1alpha1.Subscription{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sub); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	for _, crit := range criteria {
		if !crit(sub) {
			return false, nil
		}
	}
	return true, nil
}

func ForSubscriptionWithCriteria(cl client.Client, name, namespace string, criteria ...subCriteria) error {
	if err := k8swait.Poll(configuration.DefaultRetryInterval, configuration.DefaultTimeout, func() (bool, error) {
		return HasSubscriptionWithCriteria(cl, name, namespace, criteria...)
	}); err != nil {
		return errors.Wrapf(err, "could not find a Subscription with name '%s' in namespace '%s' that meets the expected criteria", name, namespace)
	}
	return nil
}

func HasCSVWithCriteria(cl client.Client, name, namespace string, criteria ...csvCriteria) (bool, error) {
	csv := &v1alpha1.ClusterServiceVersion{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, csv); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	for _, crit := range criteria {
		if !crit(csv) {
			return false, nil
		}
	}
	return true, nil
}

func ForCSVWithCriteria(cl client.Client, name, namespace string, timeout time.Duration, criteria ...csvCriteria) error {
	if err := k8swait.Poll(configuration.DefaultRetryInterval, timeout, func() (bool, error) {
		return HasCSVWithCriteria(cl, name, namespace, criteria...)
	}); err != nil {
		return errors.Wrapf(err, "could not find a CSV with name '%s' in namespace '%s' that meets the expected criteria", name, namespace)
	}
	return nil
}

type csvCriteria func(csv *v1alpha1.ClusterServiceVersion) bool

type subCriteria func(csv *v1alpha1.Subscription) bool
