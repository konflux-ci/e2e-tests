package o11y

import (
	"context"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// QuayImagePushDeployment returns quayImagePush deployment.
func (o *O11yController) QuayImagePushDeployment(quayOrg, secret, namespace string) (*appsv1.Deployment, error) {
	Deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-egress",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deployment-egress",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "deployment-egress",
					},
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: secret,
						},
					},
					ServiceAccountName: constants.DefaultPipelineServiceAccount,
					Containers: []corev1.Container{
						{
							Name:  "quay-image-push-container",
							Image: "quay.io/redhat-appstudio/buildah:v1.28",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-config",
									MountPath: "/tekton/creds-secrets/o11y-tests-token/",
									ReadOnly:  true,
								},
							},
							Env: []corev1.EnvVar{
								{Name: "BUILDAH_FORMAT", Value: "oci"},
								{Name: "STORAGE_DRIVER", Value: "vfs"},
							},
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{o.getImagePushScript(secret, quayOrg)},
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
					Volumes: []corev1.Volume{
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secret,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := o.KubeRest().Create(context.Background(), Deployment); err != nil {
		return &appsv1.Deployment{}, err
	}

	return Deployment, nil
}

// VCPUMinutesDeployment returns VCPUMinutes deployment.
func (o *O11yController) VCPUMinutesDeployment(namespace string) (*appsv1.Deployment, error) {
	Deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-vcpu",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deployment-vcpu",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "deployment-vcpu",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vcpu-minutes",
							Image: "registry.access.redhat.com/ubi9/ubi-micro",
							Command: []string{
								"/bin/bash",
								"-c",
								"sleep 1m ; echo 'vCPU Deployment Completed'",
							},
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
	}

	if err := o.KubeRest().Create(context.Background(), Deployment); err != nil {
		return nil, err
	}

	return Deployment, nil
}
