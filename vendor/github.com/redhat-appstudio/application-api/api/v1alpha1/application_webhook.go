/*
Copyright 2022 Red Hat, Inc.

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
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var applicationlog = logf.Log.WithName("application-resource")

func (r *Application) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-application,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=applications,verbs=create;update,versions=v1alpha1,name=mapplication.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Application{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Application) Default() {

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-application,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=applications,verbs=create;update,versions=v1alpha1,name=vapplication.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Application{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Application) ValidateCreate() error {
	applicationlog := applicationlog.WithValues("controllerKind", "Application").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	applicationlog.Info("validating the create request")
	// We use the DNS-1035 format for application names, so ensure it conforms to that specification
	if len(validation.IsDNS1035Label(r.Name)) != 0 {
		return fmt.Errorf("invalid application name: %q: an application resource name must start with a lower case alphabetical character, be under 63 characters, and can only consist of lower case alphanumeric characters or ‘-’,", r.Name)
	}
	if r.Spec.DisplayName == "" {
		return fmt.Errorf("display name must be provided when creating an Application")
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Application) ValidateUpdate(old runtime.Object) error {
	applicationlog := applicationlog.WithValues("controllerKind", "Application").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	applicationlog.Info("validating the update request")

	switch old := old.(type) {
	case *Application:

		if !reflect.DeepEqual(r.Spec.AppModelRepository, old.Spec.AppModelRepository) {
			return fmt.Errorf("app model repository cannot be updated to %+v", r.Spec.AppModelRepository)
		}

		if !reflect.DeepEqual(r.Spec.GitOpsRepository, old.Spec.GitOpsRepository) {
			return fmt.Errorf("gitops repository cannot be updated to %+v", r.Spec.GitOpsRepository)
		}
	default:
		return fmt.Errorf("runtime object is not of type Application")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Application) ValidateDelete() error {

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
