package jbsconfig

import (
	"github.com/redhat-appstudio/image-controller/pkg/quay"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func SetupNewReconcilerWithManager(mgr ctrl.Manager, spiPresent bool, quayClient *quay.QuayClient, quayOrgName string) error {
	r := newReconciler(mgr, spiPresent, quayClient, quayOrgName)
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.JBSConfig{})
	if spiPresent {
		builder.Watches(&source.Kind{Type: &v1beta1.SPIAccessTokenBinding{}}, &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.JBSConfig{}, IsController: false})
	}
	return builder.Complete(r)
}
