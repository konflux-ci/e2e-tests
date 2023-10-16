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
var snapshotenvironmentbindinglog = logf.Log.WithName("snapshotenvironmentbinding-resource")

func (r *SnapshotEnvironmentBinding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-snapshotenvironmentbinding,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=snapshotenvironmentbindings,verbs=create;update,versions=v1alpha1,name=msnapshotenvironmentbinding.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &SnapshotEnvironmentBinding{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *SnapshotEnvironmentBinding) Default() {
	snapshotenvironmentbindinglog := snapshotenvironmentbindinglog.WithValues("controllerKind", "SnapshotEnvironmentBinding").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotenvironmentbindinglog.Info("default", "name", r.Name)
}

// change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-snapshotenvironmentbinding,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=snapshotenvironmentbindings,verbs=create;update,versions=v1alpha1,name=vsnapshotenvironmentbinding.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &SnapshotEnvironmentBinding{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SnapshotEnvironmentBinding) ValidateCreate() error {
	snapshotenvironmentbindinglog := snapshotenvironmentbindinglog.WithValues("controllerKind", "SnapshotEnvironmentBinding").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotenvironmentbindinglog.Info("validating create")

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SnapshotEnvironmentBinding) ValidateUpdate(old runtime.Object) error {
	snapshotenvironmentbindinglog := snapshotenvironmentbindinglog.WithValues("controllerKind", "SnapshotEnvironmentBinding").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotenvironmentbindinglog.Info("validating update")

	switch old := old.(type) {
	case *SnapshotEnvironmentBinding:
		if !reflect.DeepEqual(r.Spec.Application, old.Spec.Application) {
			return fmt.Errorf("application field cannot be updated to %+v", r.Spec.Application)
		}

		if !reflect.DeepEqual(r.Spec.Environment, old.Spec.Environment) {
			return fmt.Errorf("environment field cannot be updated to %+v", r.Spec.Environment)
		}

	default:
		return fmt.Errorf("runtime object is not of type SnapshotEnvironmentBinding")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SnapshotEnvironmentBinding) ValidateDelete() error {
	snapshotenvironmentbindinglog := snapshotenvironmentbindinglog.WithValues("controllerKind", "SnapshotEnvironmentBinding").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	snapshotenvironmentbindinglog.Info("validating delete")

	return nil
}
