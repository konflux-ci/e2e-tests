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

package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appstudiov1alpha1 "github.com/redhat-appstudio/application-api/api/v1alpha1"
	l "github.com/redhat-appstudio/build-service/pkg/logs"
	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// PaCPipelineRunPrunerReconciler watches AppStudio Component object in order to clean up
// running PipelineRuns created by Pipeline-as-Code when the Component gets deleted.
type PaCPipelineRunPrunerReconciler struct {
	Client        client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *PaCPipelineRunPrunerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appstudiov1alpha1.Component{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}

//+kubebuilder:rbac:groups=tekton.dev,resources=pipelineruns,verbs=get;list;watch;delete;deletecollection

func (r *PaCPipelineRunPrunerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithName("PaCPipelineRunPruner")

	var component appstudiov1alpha1.Component
	err := r.Client.Get(ctx, req.NamespacedName, &component)
	if err == nil {
		// The Component has been recreated.
		log.Info("Component exists, nothing to do.")
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "failed to check Component existance")
		return ctrl.Result{}, err
	}

	ctx = ctrllog.IntoContext(ctx, log)
	if err := r.PrunePipelineRuns(ctx, req); err != nil {
		log.Error(err, "failed to prune PipelineRuns for the Component")
	}

	return ctrl.Result{}, nil
}

// PrunePipelineRuns deletes PipelineRuns, if any, assocoated with the given Component.
func (r *PaCPipelineRunPrunerReconciler) PrunePipelineRuns(ctx context.Context, req ctrl.Request) error {
	log := ctrllog.FromContext(ctx)

	componentPipelineRunsRequirement, err := labels.NewRequirement(ComponentNameLabelName, selection.Equals, []string{req.Name})
	if err != nil {
		return err
	}
	componentPipelineRunsSelector := labels.NewSelector().Add(*componentPipelineRunsRequirement)
	componentPipelineRunsListOptions := client.ListOptions{
		LabelSelector: componentPipelineRunsSelector,
		Namespace:     req.Namespace,
	}

	componentPipelineRunsList := &tektonapi.PipelineRunList{}
	if err := r.Client.List(ctx, componentPipelineRunsList, &componentPipelineRunsListOptions); err != nil {
		return err
	}
	if len(componentPipelineRunsList.Items) == 0 {
		log.Info(fmt.Sprintf("No PipelineRuns to prune for Component %s/%s", req.Namespace, req.Name))
		return nil
	}

	deleteComponentPipelineRunsOptions := client.DeleteAllOfOptions{
		ListOptions: componentPipelineRunsListOptions,
	}
	if err := r.Client.DeleteAllOf(ctx, &tektonapi.PipelineRun{}, &deleteComponentPipelineRunsOptions); err != nil {
		log.Error(err, fmt.Sprintf("failed to delete PipelineRuns for Component %s/%s", req.Namespace, req.Name), l.Action, l.ActionDelete)
		return err
	}
	log.Info(fmt.Sprintf("Pruned %d PipelineRuns for Component %s/%s", len(componentPipelineRunsList.Items), req.Namespace, req.Name), l.Action, l.ActionDelete)

	return nil
}
