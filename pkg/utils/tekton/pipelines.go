package tekton

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/ginkgo/v2"

	app "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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

// GetFailedPipelineRunLogs gets the logs of the pipelinerun failed task
func GetFailedPipelineRunLogs(c crclient.Client, ki kubernetes.Interface, pipelineRun *v1beta1.PipelineRun) (string, error) {
	var d *utils.FailedPipelineRunDetails
	var err error
	failMessage := fmt.Sprintf("Pipelinerun '%s' didn't succeed\n", pipelineRun.Name)
	if d, err = utils.GetFailedPipelineRunDetails(c, pipelineRun); err != nil {
		return "", err
	}
	if d.FailedContainerName != "" {
		logs, _ := utils.GetContainerLogs(ki, d.PodName, d.FailedContainerName, pipelineRun.Namespace)
		failMessage += fmt.Sprintf("Logs from failed container '%s': \n%s", d.FailedContainerName, logs)
	}
	return failMessage, nil
}

// StorePipelineRunLogs stores logs and parsed yamls of pipelineRuns into directory of pipelineruns' namespace under ARTIFACT_DIR env.
// In case the files can't be stored in ARTIFACT_DIR, they will be recorder in GinkgoWriter.
func StorePipelineRun(pipelineRun *v1beta1.PipelineRun, c crclient.Client, ki kubernetes.Interface) error {
	wd, _ := os.Getwd()
	artifactDir := utils.GetEnv("ARTIFACT_DIR", fmt.Sprintf("%s/tmp", wd))
	testLogsDir := fmt.Sprintf("%s/%s", artifactDir, pipelineRun.GetNamespace())

	pipelineRunLog, err := GetFailedPipelineRunLogs(c, ki, pipelineRun)
	if err != nil {
		return fmt.Errorf("failed to store PipelineRun: %+v", err)
	}

	pipelineRunYaml, prYamlErr := yaml.Marshal(pipelineRun)
	if prYamlErr != nil {
		GinkgoWriter.Printf("\nfailed to get pipelineRunYaml: %s\n", prYamlErr.Error())
	}

	err = os.MkdirAll(testLogsDir, os.ModePerm)

	if err != nil {
		GinkgoWriter.Printf("\n%s\nFailed pipelineRunLog:\n%s\n---END OF THE LOG---\n", pipelineRun.GetName(), pipelineRunLog)
		if prYamlErr == nil {
			GinkgoWriter.Printf("Failed pipelineRunYaml:\n%s\n", pipelineRunYaml)
		}
		return err
	}

	filename := fmt.Sprintf("%s-pr-%s.log", pipelineRun.Namespace, pipelineRun.Name)
	filePath := fmt.Sprintf("%s/%s", testLogsDir, filename)
	if err := os.WriteFile(filePath, []byte(pipelineRunLog), 0644); err != nil {
		GinkgoWriter.Printf("cannot write to %s: %+v\n", filename, err)
		GinkgoWriter.Printf("\n%s\nFailed pipelineRunLog:\n%s\n", filename, pipelineRunLog)
	}

	if prYamlErr == nil {
		filename = fmt.Sprintf("%s-pr-%s.yaml", pipelineRun.Namespace, pipelineRun.Name)
		filePath = fmt.Sprintf("%s/%s", testLogsDir, filename)
		if err := os.WriteFile(filePath, pipelineRunYaml, 0644); err != nil {
			GinkgoWriter.Printf("cannot write to %s: %+v\n", filename, err)
			GinkgoWriter.Printf("\n%s\nFailed pipelineRunYaml:\n%s\n", filename, pipelineRunYaml)
		}
	}

	return nil
}

func (s *SuiteController) StorePipelineRuns(componentPipelineRun *v1beta1.PipelineRun, testLogsDir, testNamespace string) error {
	if err := os.MkdirAll(testLogsDir, os.ModePerm); err != nil {
		return err
	}

	filepath := fmt.Sprintf("%s/%s-pr-%s.log", testLogsDir, testNamespace, componentPipelineRun.Name)
	pipelineLogs, err := s.GetPipelineRunLogs(componentPipelineRun.Name, testNamespace)
	if err != nil {
		g.GinkgoWriter.Printf("got error fetching PR logs: %v\n", err.Error())
	} else {
		if err := os.WriteFile(filepath, []byte(pipelineLogs), 0644); err != nil {
			g.GinkgoWriter.Printf("cannot write to %s: %+v\n", filepath, err)
		}
	}

	pipelineRuns, err := s.ListAllPipelineRuns(testNamespace)

	if err != nil {
		return fmt.Errorf("got error fetching PR list: %v\n", err.Error())
	}

	for _, pipelineRun := range pipelineRuns.Items {
		filepath := fmt.Sprintf("%s/%s-pr-%s.log", testLogsDir, testNamespace, pipelineRun.Name)
		pipelineLogs, err := s.GetPipelineRunLogs(pipelineRun.Name, testNamespace)
		if err != nil {
			g.GinkgoWriter.Printf("got error fetching PR logs: %v\n", err.Error())
			continue
		}

		if err := os.WriteFile(filepath, []byte(pipelineLogs), 0644); err != nil {
			g.GinkgoWriter.Printf("cannot write to %s: %+v\n", filepath, err)
		}
	}

	return nil
}
