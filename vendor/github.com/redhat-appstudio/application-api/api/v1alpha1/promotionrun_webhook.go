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
var promotionrunlog = logf.Log.WithName("promotionrun-resource")

func (r *PromotionRun) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-promotionrun,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=promotionruns,verbs=create;update,versions=v1alpha1,name=mpromotionrun.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &PromotionRun{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *PromotionRun) Default() {
	promotionrunlog := promotionrunlog.WithValues("controllerKind", "PromotionRun").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	promotionrunlog.Info("default")
}

// change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-promotionrun,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=promotionruns,verbs=create;update,versions=v1alpha1,name=vpromotionrun.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &PromotionRun{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *PromotionRun) ValidateCreate() error {
	promotionrunlog := promotionrunlog.WithValues("controllerKind", "PromotionRun").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	promotionrunlog.Info("validating create")

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *PromotionRun) ValidateUpdate(old runtime.Object) error {
	promotionrunlog := promotionrunlog.WithValues("controllerKind", "PromotionRun").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	promotionrunlog.Info("validating update")

	switch old := old.(type) {
	case *PromotionRun:

		if !reflect.DeepEqual(r.Spec, old.Spec) {
			return fmt.Errorf("spec cannot be updated to %+v", r.Spec)
		}

	default:
		return fmt.Errorf("runtime object is not of type PromotionRun")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *PromotionRun) ValidateDelete() error {
	promotionrunlog := promotionrunlog.WithValues("controllerKind", "PromotionRun").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	promotionrunlog.Info("validating delete")

	return nil
}
