/*
Copyright 2023 Red Hat, Inc.

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
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var environmentlog = logf.Log.WithName("environment-resource")

func (r *Environment) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-appstudio-redhat-com-v1alpha1-environment,mutating=true,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=environments,verbs=create;update,versions=v1alpha1,name=menvironment.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Environment{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Environment) Default() {

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-appstudio-redhat-com-v1alpha1-environment,mutating=false,failurePolicy=fail,sideEffects=None,groups=appstudio.redhat.com,resources=environments,verbs=create;update,versions=v1alpha1,name=venvironment.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Environment{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Environment) ValidateCreate() error {
	environmentlog := environmentlog.WithValues("controllerKind", "Environment").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	environmentlog.Info("validating the create request")

	// We use the DNS-1123 format for environment names, so ensure it conforms to that specification
	if len(validation.IsDNS1123Label(r.Name)) != 0 {
		return fmt.Errorf("invalid environment name: %s, an environment resource name must start with a lower case alphabetical character, be under 63 characters, and can only consist of lower case alphanumeric characters or ‘-’", r.Name)
	}

	return r.validateIngressDomain()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Environment) ValidateUpdate(old runtime.Object) error {
	environmentlog := environmentlog.WithValues("controllerKind", "Environment").WithValues("name", r.Name).WithValues("namespace", r.Namespace)
	environmentlog.Info("validating the update request")

	return r.validateIngressDomain()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Environment) ValidateDelete() error {

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// validateIngressDomain validates the ingress domain
func (r *Environment) validateIngressDomain() error {
	unstableConfig := r.Spec.UnstableConfigurationFields
	if unstableConfig != nil {
		// if cluster type is Kubernetes, then Ingress Domain should be set
		if unstableConfig.ClusterType == ConfigurationClusterType_Kubernetes && unstableConfig.IngressDomain == "" {
			return fmt.Errorf(MissingIngressDomain)
		}

		// if Ingress Domain is provided, we use the DNS-1123 format for ingress domain, so ensure it conforms to that specification
		if unstableConfig.IngressDomain != "" && len(validation.IsDNS1123Subdomain(unstableConfig.IngressDomain)) != 0 {
			return fmt.Errorf(InvalidDNS1123Subdomain, unstableConfig.IngressDomain)
		}

	}

	return nil
}
