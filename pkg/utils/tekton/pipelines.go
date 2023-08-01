package tekton

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	app "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineRunGenerator interface {
	Generate() *v1beta1.PipelineRun
}

type BuildahDemo struct {
	Image     string
	Bundle    string
	Name      string
	Namespace string
}

// This is a demo pipeline to create test image and task signing
func (g BuildahDemo) Generate() *v1beta1.PipelineRun {

	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.Name,
			Namespace: g.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			Params: []v1beta1.Param{
				{
					Name:  "dockerfile",
					Value: *v1beta1.NewArrayOrString("Dockerfile"),
				},
				{
					Name:  "output-image",
					Value: *v1beta1.NewArrayOrString(g.Image),
				},
				{
					Name:  "git-url",
					Value: *v1beta1.NewArrayOrString("https://github.com/ziwoshixianzhe/simple_docker_app.git"),
				},
				{
					Name:  "skip-checks",
					Value: *v1beta1.NewArrayOrString("true"),
				},
			},
			PipelineRef: &v1beta1.PipelineRef{
				Name:   "docker-build",
				Bundle: g.Bundle,
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "app-studio-default-workspace",
					},
				},
			},
		},
	}
}

type VerifyEnterpriseContract struct {
	ApplicationName     string
	Bundle              string
	ComponentName       string
	Image               string
	Name                string
	Namespace           string
	PolicyConfiguration string
	PublicKey           string
	SSLCertDir          string
	Strict              bool
	EffectiveTime       string
}

func (p VerifyEnterpriseContract) Generate() *v1beta1.PipelineRun {
	var snapshot app.SnapshotSpec
	err := json.Unmarshal([]byte(p.Image), &snapshot)
	if err != nil {
		fmt.Printf("Application Snapshot doesn't exist: %s\n", err)
	}

	if len(snapshot.Components) == 0 {
		p.Image = `{
			"application": "` + p.ApplicationName + `",
			"components": [
			  {
				"name": "` + p.ComponentName + `",
				"containerImage": "` + p.Image + `"
			  }
			]
		  }`
	}
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-run-", p.Name),
			Namespace:    p.Namespace,
			Labels: map[string]string{
				"appstudio.openshift.io/application": p.ApplicationName,
				"appstudio.openshift.io/component":   p.ComponentName,
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "verify-enterprise-contract",
						Params: []v1beta1.Param{
							{
								Name: "IMAGES",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.Image,
								},
							},
							{
								Name: "POLICY_CONFIGURATION",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.PolicyConfiguration,
								},
							},
							{
								Name: "PUBLIC_KEY",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.PublicKey,
								},
							},
							{
								Name: "SSL_CERT_DIR",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.SSLCertDir,
								},
							},
							{
								Name: "STRICT",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: strconv.FormatBool(p.Strict),
								},
							},
							{
								Name: "EFFECTIVE_TIME",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.EffectiveTime,
								},
							},
						},
						TaskRef: &v1beta1.TaskRef{
							Name:   "verify-enterprise-contract",
							Bundle: p.Bundle,
						},
					},
				},
			},
		},
	}
}

// Create a tekton pipeline and return the pipeline or error
func (s *SuiteController) CreatePipeline(pipeline *v1beta1.Pipeline, ns string) (*v1beta1.Pipeline, error) {
	return s.PipelineClient().TektonV1beta1().Pipelines(ns).Create(context.TODO(), pipeline, metav1.CreateOptions{})
}

func (s *SuiteController) DeletePipeline(name, ns string) error {
	return s.PipelineClient().TektonV1beta1().Pipelines(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (k KubeController) RunPipeline(g PipelineRunGenerator, taskTimeout int) (*v1beta1.PipelineRun, error) {
	pr := g.Generate()
	pvcs := k.Commonctrl.KubeInterface().CoreV1().PersistentVolumeClaims(pr.Namespace)
	for _, w := range pr.Spec.Workspaces {
		if w.PersistentVolumeClaim != nil {
			pvcName := w.PersistentVolumeClaim.ClaimName
			if _, err := pvcs.Get(context.TODO(), pvcName, metav1.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					err := createPVC(pvcs, pvcName)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}

	return k.createAndWait(pr, taskTimeout)
}
