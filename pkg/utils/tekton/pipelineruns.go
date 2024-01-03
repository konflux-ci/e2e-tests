package tekton

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const sslCertDir = "/var/run/secrets/kubernetes.io/serviceaccount"

type PipelineRunGenerator interface {
	Generate() (*tektonv1.PipelineRun, error)
}

type BuildahDemo struct {
	Image     string
	Bundle    string
	Name      string
	Namespace string
}

type ECIntegrationTestScenario struct {
	Image                 string
	Name                  string
	Namespace             string
	PipelineGitURL        string
	PipelineGitRevision   string
	PipelineGitPathInRepo string
}

type FailedPipelineRunDetails struct {
	FailedTaskRunName   string
	PodName             string
	FailedContainerName string
}

// This is a demo pipeline to create test image and task signing
func (b BuildahDemo) Generate() (*tektonv1.PipelineRun, error) {
	return &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.Name,
			Namespace: b.Namespace,
		},
		Spec: tektonv1.PipelineRunSpec{
			Params: []tektonv1.Param{
				{
					Name:  "dockerfile",
					Value: *tektonv1.NewStructuredValues("Dockerfile"),
				},
				{
					Name:  "output-image",
					Value: *tektonv1.NewStructuredValues(b.Image),
				},
				{
					Name:  "git-url",
					Value: *tektonv1.NewStructuredValues("https://github.com/ziwoshixianzhe/simple_docker_app.git"),
				},
				{
					Name:  "skip-checks",
					Value: *tektonv1.NewStructuredValues("true"),
				},
			},
			PipelineRef: NewBundleResolverPipelineRef("docker-build", b.Bundle),
			Workspaces: []tektonv1.WorkspaceBinding{
				{
					Name: "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "app-studio-default-workspace",
					},
				},
			},
		},
	}, nil
}

// Generates pipelineRun from VerifyEnterpriseContract.
func (p VerifyEnterpriseContract) Generate() (*tektonv1.PipelineRun, error) {
	var applicationSnapshotJSON, err = json.Marshal(p.Snapshot)
	if err != nil {
		return nil, err
	}
	return &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-run-", p.Name),
			Namespace:    p.Namespace,
			Labels: map[string]string{
				"appstudio.openshift.io/application": p.Snapshot.Application,
			},
		},
		Spec: tektonv1.PipelineRunSpec{
			PipelineSpec: &tektonv1.PipelineSpec{
				Tasks: []tektonv1.PipelineTask{
					{
						Name: "verify-enterprise-contract",
						Params: []tektonv1.Param{
							{
								Name: "IMAGES",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: string(applicationSnapshotJSON),
								},
							},
							{
								Name: "POLICY_CONFIGURATION",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: p.PolicyConfiguration,
								},
							},
							{
								Name: "PUBLIC_KEY",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: p.PublicKey,
								},
							},
							{
								Name: "SSL_CERT_DIR",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: sslCertDir,
								},
							},
							{
								Name: "STRICT",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: strconv.FormatBool(p.Strict),
								},
							},
							{
								Name: "EFFECTIVE_TIME",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: p.EffectiveTime,
								},
							},
							{
								Name: "IGNORE_REKOR",
								Value: tektonv1.ParamValue{
									Type:      tektonv1.ParamTypeString,
									StringVal: strconv.FormatBool(p.IgnoreRekor),
								},
							},
						},
						TaskRef: &tektonv1.TaskRef{
							Name: "verify-enterprise-contract",
							ResolverRef: tektonv1.ResolverRef{
								Resolver: "bundle",
								Params: []tektonv1.Param{
									{
										Name: "bundle",
										Value: tektonv1.ParamValue{
											Type:      tektonv1.ParamTypeString,
											StringVal: p.TaskBundle,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// Generates pipelineRun from ECIntegrationTestScenario.
func (p ECIntegrationTestScenario) Generate() (*tektonv1.PipelineRun, error) {

	snapshot := `{"components": [
		{"containerImage": "` + p.Image + `"}
	]}`

	return &tektonv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ec-integration-test-scenario-run-",
			Namespace:    p.Namespace,
		},
		Spec: tektonv1.PipelineRunSpec{
			PipelineRef: &tektonv1.PipelineRef{
				ResolverRef: tektonv1.ResolverRef{
					Resolver: "git",
					Params: []tektonv1.Param{
						{Name: "url", Value: *tektonv1.NewStructuredValues(p.PipelineGitURL)},
						{Name: "revision", Value: *tektonv1.NewStructuredValues(p.PipelineGitRevision)},
						{Name: "pathInRepo", Value: *tektonv1.NewStructuredValues(p.PipelineGitPathInRepo)},
					},
				},
			},
			Params: []tektonv1.Param{
				{Name: "SNAPSHOT", Value: *tektonv1.NewStructuredValues(snapshot)},
			},
		},
	}, nil
}

// GetFailedPipelineRunLogs gets the logs of the pipelinerun failed task
func GetFailedPipelineRunLogs(c crclient.Client, ki kubernetes.Interface, pipelineRun *tektonv1.PipelineRun) (string, error) {
	var d *FailedPipelineRunDetails
	var err error
	failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
	if d, err = GetFailedPipelineRunDetails(c, pipelineRun); err != nil {
		return "", err
	}
	if d.FailedContainerName != "" {
		logs, _ := utils.GetContainerLogs(ki, d.PodName, d.FailedContainerName, pipelineRun.Namespace)
		failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
	}
	return failMessage, nil
}

func HasPipelineRunSucceeded(pr *tektonv1.PipelineRun) bool {
	return pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue()
}

func HasPipelineRunFailed(pr *tektonv1.PipelineRun) bool {
	return pr.IsDone() && pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse()
}

func GetFailedPipelineRunDetails(c crclient.Client, pipelineRun *tektonv1.PipelineRun) (*FailedPipelineRunDetails, error) {
	d := &FailedPipelineRunDetails{}
	for _, chr := range pipelineRun.Status.PipelineRunStatusFields.ChildReferences {
		taskRun := &tektonv1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pipelineRun.Namespace, Name: chr.Name}
		if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
			return nil, fmt.Errorf("failed to get details for PR %s: %+v", pipelineRun.GetName(), err)
		}
		for _, c := range taskRun.Status.Conditions {
			if c.Reason == "Failed" {
				d.FailedTaskRunName = taskRun.Name
				d.PodName = taskRun.Status.PodName
				for _, s := range taskRun.Status.TaskRunStatusFields.Steps {
					if s.Terminated.Reason == "Error" {
						d.FailedContainerName = s.Name
						return d, nil
					}
				}
			}
		}
	}
	return d, nil
}
