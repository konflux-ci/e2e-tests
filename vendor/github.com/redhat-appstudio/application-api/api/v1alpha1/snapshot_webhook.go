/*
Copyright 2021-2022 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var snapshotlog = logf.Log.WithName("snapshot-resource")

func (r *Snapshot) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-snapshot,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=snapshots,verbs=create;update,versions=v1alpha1,name=vsnapshot.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Snapshot{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Snapshot) ValidateCreate() error {
	snapshotlog := snapshotlog.WithValues("controllerKind", "Snapshot").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotlog.Info("validating create")

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Snapshot) ValidateUpdate(old runtime.Object) error {
	snapshotlog := snapshotlog.WithValues("controllerKind", "Snapshot").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotlog.Info("validating update")

	switch old := old.(type) {
	case *Snapshot:

		if !reflect.DeepEqual(r.Spec.Application, old.Spec.Application) {
			return fmt.Errorf("application field cannot be updated to %+v", r.Spec.Application)
		}

		if !reflect.DeepEqual(r.Spec.Components, old.Spec.Components) {
			return fmt.Errorf("components cannot be updated to %+v", r.Spec.Components)
		}

	default:
		return fmt.Errorf("runtime object is not of type Snapshot")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Snapshot) ValidateDelete() error {
	snapshotlog := snapshotlog.WithValues("controllerKind", "Snapshot").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotlog.Info("validating delete")

	return nil
}
