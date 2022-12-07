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
	"time"

	"github.com/redhat-appstudio/release-service/metrics"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseSpec defines the desired state of Release.
type ReleaseSpec struct {
	// Snapshot to be released
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Snapshot string `json:"snapshot"`

	// ReleasePlan to use for this particular Release
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	ReleasePlan string `json:"releasePlan"`
}

// ReleaseReason represents a reason for the release "Succeeded" condition.
type ReleaseReason string

const (
	// releaseConditionType is the type used when setting a release status condition
	releaseConditionType string = "Succeeded"

	// ReleaseReasonValidationError is the reason set when the Release validation failed
	ReleaseReasonValidationError ReleaseReason = "ReleaseValidationError"

	// ReleaseReasonPipelineFailed is the reason set when the release PipelineRun failed
	ReleaseReasonPipelineFailed ReleaseReason = "ReleasePipelineFailed"

	// ReleaseReasonReleasePlanValidationError is the reason set when there is a validation error with the ReleasePlan
	ReleaseReasonReleasePlanValidationError ReleaseReason = "ReleasePlanValidationError"

	// ReleaseReasonTargetDisabledError is the reason set when releases to the target are disabled
	ReleaseReasonTargetDisabledError ReleaseReason = "ReleaseTargetDisabledError"

	// ReleaseReasonRunning is the reason set when the release PipelineRun starts running
	ReleaseReasonRunning ReleaseReason = "Running"

	// ReleaseReasonSucceeded is the reason set when the release PipelineRun has succeeded
	ReleaseReasonSucceeded ReleaseReason = "Succeeded"
)

func (rr ReleaseReason) String() string {
	return string(rr)
}

const (
	// AutoReleaseLabel is the label name for the auto-release setting
	AutoReleaseLabel = "release.appstudio.openshift.io/auto-release"
)

// ReleaseStatus defines the observed state of Release.
type ReleaseStatus struct {
	// StartTime is the time when the Release PipelineRun was created and set to run
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the Release PipelineRun completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions represent the latest available observations for the release
	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// SnapshotEnvironmentBinding contains the namespaced name of the SnapshotEnvironmentBinding created as part of
	// this release
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	SnapshotEnvironmentBinding string `json:"snapshotEnvironmentBinding,omitempty"`

	// ReleasePipelineRun contains the namespaced name of the release PipelineRun executed as part of this release
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ReleasePipelineRun string `json:"releasePipelineRun,omitempty"`

	// ReleaseStrategy contains the namespaced name of the ReleaseStrategy used for this release
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	ReleaseStrategy string `json:"releaseStrategy,omitempty"`

	// Target references where this relesae is intended to be released to
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +optional
	Target string `json:"target,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Snapshot",type=string,JSONPath=`.spec.snapshot`
// +kubebuilder:printcolumn:name="Succeeded",type=string,JSONPath=`.status.conditions[?(@.type=="Succeeded")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Succeeded")].reason`
// +kubebuilder:printcolumn:name="PipelineRun",type=string,priority=1,JSONPath=`.status.releasePipelineRun`
// +kubebuilder:printcolumn:name="Start Time",type=date,priority=1,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="Completion Time",type=date,priority=1,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Release is the Schema for the releases API
type Release struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseSpec   `json:"spec,omitempty"`
	Status ReleaseStatus `json:"status,omitempty"`
}

// HasBeenDeployed checks whether the Release has an associated SnapshotEnvironmentBinding.
func (r *Release) HasBeenDeployed() bool {
	return r.Status.SnapshotEnvironmentBinding != ""
}

// HasStarted checks whether the Release has a valid start time set in its status.
func (r *Release) HasStarted() bool {
	return r.Status.StartTime != nil && !r.Status.StartTime.IsZero()
}

// HasSucceeded checks whether the Release has succeeded or not.
func (r *Release) HasSucceeded() bool {
	return meta.IsStatusConditionTrue(r.Status.Conditions, releaseConditionType)
}

// IsDone returns a boolean indicating whether the Release's status indicates that it is done or not.
func (r *Release) IsDone() bool {
	condition := meta.FindStatusCondition(r.Status.Conditions, releaseConditionType)
	if condition != nil {
		return condition.Status != metav1.ConditionUnknown
	}

	return false
}

// MarkFailed registers the completion time and changes the Succeeded condition to False with
// the provided reason and message.
func (r *Release) MarkFailed(reason ReleaseReason, message string) {
	if r.IsDone() && r.Status.CompletionTime != nil {
		return
	}

	r.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	r.setStatusConditionWithMessage(metav1.ConditionFalse, reason, message)

	go metrics.RegisterCompletedRelease(reason.String(), r.Status.ReleaseStrategy, r.Status.Target,
		r.Status.StartTime, r.Status.CompletionTime, false)
}

// MarkInvalid changes the Succeeded condition to False with the provided reason and message.
func (r *Release) MarkInvalid(reason ReleaseReason, message string) {
	if r.IsDone() {
		return
	}

	r.setStatusConditionWithMessage(metav1.ConditionFalse, reason, message)

	go metrics.RegisterInvalidRelease(reason.String())
}

// MarkRunning registers the start time and changes the Succeeded condition to Unknown.
func (r *Release) MarkRunning() {
	if r.HasStarted() && r.Status.StartTime != nil {
		return
	}

	r.Status.StartTime = &metav1.Time{Time: time.Now()}
	r.setStatusCondition(metav1.ConditionUnknown, ReleaseReasonRunning)

	go metrics.RegisterNewRelease(r.GetCreationTimestamp(), r.Status.StartTime)
}

// MarkSucceeded registers the completion time and changes the Succeeded condition to True.
func (r *Release) MarkSucceeded() {
	if r.IsDone() && r.Status.CompletionTime != nil {
		return
	}

	r.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	r.setStatusCondition(metav1.ConditionTrue, ReleaseReasonSucceeded)

	go metrics.RegisterCompletedRelease(ReleaseReasonSucceeded.String(), r.Status.ReleaseStrategy, r.Status.Target,
		r.Status.StartTime, r.Status.CompletionTime, false)
}

// SetCondition creates a new condition with the given status and reason. Then, it sets this new condition,
// unsetting previous conditions with the same type as necessary.
func (r *Release) setStatusCondition(status metav1.ConditionStatus, reason ReleaseReason) {
	r.setStatusConditionWithMessage(status, reason, "")
}

// SetCondition creates a new condition with the given status, reason and message. Then, it sets this new condition,
// unsetting previous conditions with the same type as necessary.
func (r *Release) setStatusConditionWithMessage(status metav1.ConditionStatus, reason ReleaseReason, message string) {
	meta.SetStatusCondition(&r.Status.Conditions, metav1.Condition{
		Type:    releaseConditionType,
		Status:  status,
		Reason:  reason.String(),
		Message: message,
	})
}

// +kubebuilder:object:root=true

// ReleaseList contains a list of Release
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Release `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Release{}, &ReleaseList{})
}
