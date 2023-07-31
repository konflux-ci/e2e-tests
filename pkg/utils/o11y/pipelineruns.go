package o11y

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// QuayImagePushPipelineRun returns quayImagePush pipelineRun.
func (o *O11yController) QuayImagePushPipelineRun(quayOrg, secret, namespace string) (*v1beta1.PipelineRun, error) {
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipelinerun-egress-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type": "test",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "buildah-quay",
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{
									{
										Name:  "pull-and-push-image",
										Image: "quay.io/redhat-appstudio/buildah:v1.28",
										Env: []corev1.EnvVar{
											{Name: "BUILDAH_FORMAT", Value: "oci"},
											{Name: "STORAGE_DRIVER", Value: "vfs"},
										},
										Script: o.getImagePushScript(secret, quayOrg),
										SecurityContext: &corev1.SecurityContext{
											RunAsUser: pointer.Int64(0),
											Capabilities: &corev1.Capabilities{
												Add: []corev1.Capability{
													"SETFCAP",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := o.KubeRest().Create(context.Background(), pipelineRun); err != nil {
		return nil, err
	}

	return pipelineRun, nil
}

// VCPUMinutesPipelineRun returns VCPUMinutes pipelineRun.
func (o *O11yController) VCPUMinutesPipelineRun(namespace string) (*v1beta1.PipelineRun, error) {
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pipelinerun-vcpu-",
			Namespace:    namespace,
			Labels: map[string]string{
				"pipelines.appstudio.openshift.io/type": "test",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "vcpu-minutes",
						TaskSpec: &v1beta1.EmbeddedTask{
							TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{
									{
										Name:   "resource-constraint",
										Image:  "registry.access.redhat.com/ubi9/ubi-micro",
										Script: "#!/usr/bin/env bash\nsleep 1m\necho 'vCPU Deployment Completed'\n",
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("200Mi"),
												corev1.ResourceCPU:    resource.MustParse("200m"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("200Mi"),
												corev1.ResourceCPU:    resource.MustParse("200m"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := o.KubeRest().Create(context.Background(), pipelineRun); err != nil {
		return nil, err
	}

	return pipelineRun, nil
}
