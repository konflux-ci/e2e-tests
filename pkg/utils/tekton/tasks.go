package tekton

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TaskRunGenerator interface {
	Generate() *v1beta1.TaskRun
}

type BuildahDemoTask struct {
	Image string
}

// This is a demo task to create test image and task signing
func (g BuildahDemoTask) Generate() *v1beta1.TaskRun {
	imageInfo := strings.Split(g.Image, "/")
	namespace := imageInfo[1]
	// Make the TaskRun name predictable.
	name := imageInfo[2]

	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Results: []v1beta1.TaskResult{
					{Name: "IMAGE_DIGEST"},
					{Name: "IMAGE_URL"},
				},
				Steps: []v1beta1.Step{
					{
						Container: corev1.Container{
							Image:      "registry.access.redhat.com/ubi8:latest",
							Name:       "add-dockerfile",
							WorkingDir: "$(workspaces.source.path)",
						},
						Script: "set -e\necho \"FROM scratch\" | tee ./Dockerfile\n",
					},
					{
						Container: corev1.Container{
							Image: "registry.access.redhat.com/ubi8/buildah:latest",
							Name:  "build",
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/var/lib/containers",
									Name:      "varlibcontainers",
								},
							},
							WorkingDir: "$(workspaces.source.path)",
						},
						Script: "buildah --storage-driver=vfs bud \\\n  --format=oci \\\n  --no-cache \\\n  -t " + g.Image + " .\n",
					},
					{
						Container: corev1.Container{
							Image: "registry.access.redhat.com/ubi8/buildah:latest",
							Name:  "push",
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/var/lib/containers",
									Name:      "varlibcontainers",
								},
							},
							WorkingDir: "$(workspaces.source.path)",
						},
						Script: "buildah --storage-driver=vfs push \\\n  --digestfile $(workspaces.source.path)/image-digest " + g.Image + " \\\n  docker://" + g.Image + "\n",
					},
					{
						Container: corev1.Container{
							Image: "registry.access.redhat.com/ubi8/buildah:latest",
							Name:  "digest-to-results",
						},
						Script: "cat \"$(workspaces.source.path)\"/image-digest | tee $(results.IMAGE_DIGEST.path)\necho -n \"" + g.Image + "\" | tee $(results.IMAGE_URL.path)\n",
					},
				},
				Workspaces: []v1beta1.WorkspaceDeclaration{
					{Name: "source"},
				},
				Volumes: []corev1.Volume{
					{
						Name: "varlibcontainers",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:     "source",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}
}

type CosignVerifyTask struct {
	TaskName string
	Image    string
}

// image is full url to the image, e.g.:
// image-registry.openshift-image-registry.svc:5000/tekton-chains/buildah-demo@sha256:abc...
func (t CosignVerifyTask) Generate() *v1beta1.TaskRun {
	imageInfo := strings.Split(t.Image, "/")
	namespace := imageInfo[1]

	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", t.TaskName),
			Namespace:    namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{
				{
					Name: "IMAGE",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.Image,
					},
				},
				{
					Name: "PUBLIC_KEY",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: "k8s://tekton-chains/signing-secrets",
					},
				},
			},
			TaskRef: &v1beta1.TaskRef{
				Kind:   v1beta1.NamespacedTaskKind,
				Name:   t.TaskName,
				Bundle: "quay.io/redhat-appstudio/appstudio-tasks:861e28f4eb2380fd1531ee30a9e74fb6ce496b9f-1",
			},
		},
	}
}

type VerifyEnterpriseContractTask struct {
	TaskName     string
	ImageRef     string
	PublicSecret string
	PipelineName string
	RekorHost    string
	SslCertDir   string
	StrictPolicy string
}

func (t VerifyEnterpriseContractTask) Generate() *v1beta1.TaskRun {
	imageInfo := strings.Split(t.ImageRef, "/")
	namespace := imageInfo[1]

	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", t.TaskName),
			Namespace:    namespace,
		},
		Spec: v1beta1.TaskRunSpec{
			Params: []v1beta1.Param{
				{
					Name: "IMAGE_REF",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.ImageRef,
					},
				},
				{
					Name: "PIPELINERUN_NAME",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.PipelineName,
					},
				},
				{
					Name: "PUBLIC_KEY",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.PublicSecret,
					},
				},
				{
					Name: "REKOR_HOST",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.RekorHost,
					},
				},
				{
					Name: "SSL_CERT_DIR",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.SslCertDir,
					},
				},
				{
					Name: "STRICT_POLICY",
					Value: v1beta1.ArrayOrString{
						Type:      v1beta1.ParamTypeString,
						StringVal: t.StrictPolicy,
					},
				},
			},
			TaskRef: &v1beta1.TaskRef{
				// TODO: Use the most up to date bundle, https://issues.redhat.com/browse/HACBS-424
				Kind:   v1beta1.NamespacedTaskKind,
				Name:   t.TaskName,
				Bundle: "quay.io/redhat-appstudio/appstudio-tasks:861e28f4eb2380fd1531ee30a9e74fb6ce496b9f-2",
			},
		},
	}
}
