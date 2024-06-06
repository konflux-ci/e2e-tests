package journey

import "context"
import "fmt"
import "time"

import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

import buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
import rclient "sigs.k8s.io/controller-runtime/pkg/client"
import tekton "github.com/konflux-ci/e2e-tests/pkg/utils/tekton"
import utils "github.com/konflux-ci/e2e-tests/pkg/utils"

func ListAllBuildPipelineSelectors(f *framework.Framework, namespace string) (*buildservice.BuildPipelineSelectorList, error) {
	list := &buildservice.BuildPipelineSelectorList{}
	err := f.AsKubeDeveloper.HasController.KubeRest().List(context.Background(), list, &rclient.ListOptions{Namespace: namespace})
	return list, err
}

func DeleteAllBuildPipelineSelectors(f *framework.Framework, namespace string, timeout time.Duration) error {
	list, err := ListAllBuildPipelineSelectors(f, namespace)
	if err != nil {
		return fmt.Errorf("Error listing build pipeline selectors from %s: %v", namespace, err)
	}

	for _, bps := range list.Items {
		logging.Logger.Debug("Deleting build pipeline selectors %s from namespace %s", bps.Name, namespace)
		toDelete := bps
		err = f.AsKubeDeveloper.HasController.KubeRest().Delete(context.Background(), &toDelete)
		if err != nil {
			return fmt.Errorf("Error deleting build pipeline selector %s from %s: %v", bps.Name, namespace, err)
		}
	}

	return utils.WaitUntil(func() (done bool, err error) {
		list, err := ListAllBuildPipelineSelectors(f, namespace)
		if err != nil {
			return false, nil
		}
		return len(list.Items) == 0, nil
	}, timeout)
}

func CreateBuildPipelineSelector(f *framework.Framework, namespace string, bundle string) error {
	var err error

	bps := &buildservice.BuildPipelineSelector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-pipeline-selector",
			Namespace: namespace,
		},
		Spec: buildservice.BuildPipelineSelectorSpec{Selectors: []buildservice.PipelineSelector{
			{
				Name:        "all-pipelines",
				PipelineRef: *tekton.NewBundleResolverPipelineRef("docker-build", bundle),
			},
		}},
	}
	logging.Logger.Debug("Creating build pipeline selectors %s in namespace %s", bps.Name, namespace)
	err = f.AsKubeAdmin.CommonController.KubeRest().Create(context.TODO(), bps)
	if err != nil {
		return fmt.Errorf("Error creating build pipeline selector in %s with bundle %s: %v", namespace, bundle, err)
	}

	return nil
}

func HandleBuildPipelineSelector(ctx *MainContext) error {
	var err error

	err = DeleteAllBuildPipelineSelectors(ctx.Framework, ctx.Namespace, time.Minute)
	if err != nil {
		return logging.Logger.Fail(20, "Cleanup failed: %v", err)
	}

	if ctx.Opts.BuildPipelineSelectorBundle != "" {
		err = CreateBuildPipelineSelector(ctx.Framework, ctx.Namespace, ctx.Opts.BuildPipelineSelectorBundle)
		if err != nil {
			return logging.Logger.Fail(21, "Create failed: %v", err)
		}
	}

	return nil
}
