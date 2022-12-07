/*
Copyright 2022.

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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (rp *ReleasePlan) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(rp).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaseplan,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplans,verbs=create,versions=v1alpha1,name=mreleaseplan.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ReleasePlan{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (rp *ReleasePlan) Default() {
	if _, found := rp.GetLabels()[AutoReleaseLabel]; !found {
		if rp.Labels == nil {
			rp.Labels = map[string]string{
				AutoReleaseLabel: "true",
			}
		}
	}
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaseplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplans,verbs=create;update,versions=v1alpha1,name=vreleaseplan.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ReleasePlan{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (rp *ReleasePlan) ValidateCreate() error {
	return rp.validateAutoReleaseLabel()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (rp *ReleasePlan) ValidateUpdate(old runtime.Object) error {
	return rp.validateAutoReleaseLabel()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (rp *ReleasePlan) ValidateDelete() error {
	return nil
}

// validateAutoReleaseLabel throws an error if the auto-release label value is set to anything besides true or false.
func (rp *ReleasePlan) validateAutoReleaseLabel() error {
	if value, found := rp.GetLabels()[AutoReleaseLabel]; found {
		if value != "true" && value != "false" {
			return fmt.Errorf("'%s' label can only be set to true or false", AutoReleaseLabel)
		}
	}
	return nil
}
