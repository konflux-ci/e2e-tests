package journey

import "context"
import "fmt"

import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"
import timeandlog "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/timeandlog"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"


func CollectPersistentVolumeClaims(f *framework.Framework, namespace string) error {
	pvcs, err := f.AsKubeAdmin.TektonController.KubeInterface().CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("Error getting PVC: %v\n", err)
	}
	for _, pvc := range pvcs.Items {
		pv, err := f.AsKubeAdmin.TektonController.KubeInterface().CoreV1().PersistentVolumes().Get(context.Background(), pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			_ = logging.Logger.Fail(76, "Error getting PV: %v\n", err)
			continue
		}
		waittime := (pv.ObjectMeta.CreationTimestamp.Time).Sub(pvc.ObjectMeta.CreationTimestamp.Time)
		timeandlog.LogMeasurement("PVC_to_PV_CreationTimestamp", pv.Name, waittime, "", nil)
	}
	return nil
}

func HandlePersistentVolumeClaim(ctx *MainContext) error {
	if ! ctx.Opts.WaitPipelines {
		return nil // if build pipeline runs are not done yet, it does not make sense to collect PV timings
	}

	if ctx.Opts.Stage {
		return nil // if we are running agains Stage, we to not have admin account required for this
	}

	var err error

	logging.Logger.Debug("Collecting persistent volume claim wait times in namespace %s", ctx.Namespace)

	err = CollectPersistentVolumeClaims(ctx.Framework, ctx.Namespace)
	if err != nil {
		return logging.Logger.Fail(75, "Collecting persistent volume claim failed: %v", err)
	}

	return nil
}
