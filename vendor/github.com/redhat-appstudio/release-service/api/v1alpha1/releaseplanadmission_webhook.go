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

func (rp *ReleasePlanAdmission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(rp).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-releaseplanadmission,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplanadmissions,verbs=create,versions=v1alpha1,name=mreleaseplanadmission.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ReleasePlanAdmission{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (rp *ReleasePlanAdmission) Default() {
	if _, found := rp.GetLabels()[AutoReleaseLabel]; !found {
		if rp.Labels == nil {
			rp.Labels = map[string]string{
				AutoReleaseLabel: "true",
			}
		}
	}
}

// +kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-releaseplanadmission,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=releaseplanadmissions,verbs=create;update,versions=v1alpha1,name=vreleaseplanadmission.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ReleasePlanAdmission{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (rp *ReleasePlanAdmission) ValidateCreate() error {
	return rp.validateAutoReleaseLabel()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (rp *ReleasePlanAdmission) ValidateUpdate(old runtime.Object) error {
	return rp.validateAutoReleaseLabel()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (rp *ReleasePlanAdmission) ValidateDelete() error {
	return nil
}

// validateAutoReleaseLabel throws an error if the auto-release label value is set to anything besides true or false.
func (rp *ReleasePlanAdmission) validateAutoReleaseLabel() error {
	if value, found := rp.GetLabels()[AutoReleaseLabel]; found {
		if value != "true" && value != "false" {
			return fmt.Errorf("'%s' label can only be set to true or false", AutoReleaseLabel)
		}
	}
	return nil
}
