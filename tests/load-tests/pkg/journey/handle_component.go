package journey

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"

	constants "github.com/konflux-ci/e2e-tests/pkg/constants"

	framework "github.com/konflux-ci/e2e-tests/pkg/framework"

	utils "github.com/konflux-ci/e2e-tests/pkg/utils"

	appstudioApi "github.com/konflux-ci/application-api/api/v1alpha1"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// Parse PR number out of PR url
func getPRNumberFromPRUrl(prUrl string) (int, error) {
	regex := regexp.MustCompile(`/([0-9]+)/?$`)
	match := regex.FindStringSubmatch(prUrl)
	if match == nil {
		return 0, fmt.Errorf("Failed to parse PR number out of url %s", prUrl)
	}

	prNumber, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("Failed to convert PR number %s to int: %v", match[1], err)
	}

	return prNumber, nil
}

// Get PR URL from PaC component annotation "build.appstudio.openshift.io/status"
func getPaCPull(annotations map[string]string) (string, error) {
	var buildStatusAnn string = "build.appstudio.openshift.io/status"
	var buildStatusValue string
	var buildStatusMap map[string]interface{}

	// Get annotation we are interested in
	buildStatusValue, exists := annotations[buildStatusAnn]
	if !exists {
		return "", nil
	}

	// Parse JSON
	err := json.Unmarshal([]byte(buildStatusValue), &buildStatusMap)
	if err != nil {
		return "", fmt.Errorf("Error unmarshalling JSON: %v", err)
	}

	// Access the nested value using type assertion
	if pac, ok := buildStatusMap["pac"].(map[string]interface{}); ok {
		var data string
		var ok bool

		// Example: '{"pac":{"state":"enabled","merge-url":"https://github.com/rhtap-test-local/multi-platform-test-test-rhtap-1/pull/1","configuration-time":"Thu, 23 May 2024 07:06:43 UTC"},"message":"done"}'

		// Check "state" is "enabled"
		if data, ok = pac["state"].(string); ok {
			if data != "enabled" {
				return "", fmt.Errorf("Incorrect state: %s", buildStatusValue)
			}
		} else {
			return "", fmt.Errorf("Failed parsing state: %s", buildStatusValue)
		}

		// Get "merge-url"
		if data, ok = pac["merge-url"].(string); ok {
			return data, nil
		} else {
			return "", fmt.Errorf("Failed parsing state: %s", buildStatusValue)
		}
	} else {
		return "", fmt.Errorf("Failed parsing: %s", buildStatusValue)
	}
}

func createComponent(f *framework.Framework, namespace, name, repoUrl, repoRevision, containerContext, containerFile, buildPipelineSelector, appName string, mintmakerDisabled bool) error {
	// Prepare annotations to add to component
	annotationsMap := constants.DefaultDockerBuildPipelineBundleAnnotation
	if buildPipelineSelector != "" {
		// Custom build pipeline selector
		annotationsMap["build.appstudio.openshift.io/pipeline"] = fmt.Sprintf(`{"name": "docker-build", "bundle": "%s"}`, buildPipelineSelector)
	}
	if mintmakerDisabled {
		// Stop Mintmaker creating update PRs for your component
		for key, value := range constants.ComponentMintmakerDisabledAnnotation {
			annotationsMap[key] = value
		}
	}

	componentObj := appstudioApi.ComponentSpec{
		ComponentName: name,
		Source: appstudioApi.ComponentSource{
			ComponentSourceUnion: appstudioApi.ComponentSourceUnion{
				GitSource: &appstudioApi.GitSource{
					URL:           repoUrl,
					Revision:      repoRevision,
					Context:       containerContext,
					DockerfileURL: containerFile,
				},
			},
		},
	}

	_, err := f.AsKubeDeveloper.HasController.CreateComponent(componentObj, namespace, "", "", appName, false, annotationsMap)
	if err != nil {
		return fmt.Errorf("Unable to create the Component %s: %v", name, err)
	}
	return nil
}

func getPaCPullNumber(f *framework.Framework, namespace, name string) (int, error) {
	interval := time.Second * 20
	timeout := time.Minute * 15
	var comp *appstudioApi.Component
	var pull string
	var pullNumber int

	// TODO It would be much better to watch this resource for a condition
	err := utils.WaitUntilWithInterval(func() (done bool, err error) {
		comp, err = f.AsKubeDeveloper.HasController.GetComponent(name, namespace)
		if err != nil {
			logging.Logger.Debug("Unable to get created Component %s in namespace %s: %v", name, namespace, err)
			return false, nil
		}

		// Check for right annotation
		pull, err = getPaCPull(comp.Annotations)
		if err != nil {
			return false, fmt.Errorf("PaC component %s in namespace %s failed on PR annotation: %v", name, namespace, err)
		}
		if pull == "" {
			logging.Logger.Debug("PaC component %s in namespace %s do not have PR yet", name, namespace)
			return false, nil
		}

		return true, nil
	}, interval, timeout)
	if err != nil {
		return -1, fmt.Errorf("Unable to get PaC pull number for component %s in namespace %s: %v", name, namespace, err)
	}

	// Get merge request number
	pullNumber, err = getPRNumberFromPRUrl(pull)
	if err != nil {
		return -1, fmt.Errorf("Parsing merge request number failed: %+v", err)
	}

	return pullNumber, err
}

func listPipelineRunsWithTimeout(f *framework.Framework, namespace, appName, compName, sha string, expectedCount int) (*[]pipeline.PipelineRun, error) {
	var prs *[]pipeline.PipelineRun
	var err error

	interval := time.Second * 20
	timeout := time.Minute * 60

	err = utils.WaitUntilWithInterval(func() (done bool, err error) {
		prs, err = f.AsKubeDeveloper.HasController.GetComponentPipelineRunsWithType(compName, appName, namespace, "build", sha)
		if err != nil {
			logging.Logger.Debug("Waiting for PipelineRun for component %s in namespace %s", compName, namespace)
			return false, nil
		}
		if len(*prs) < expectedCount {
			logging.Logger.Debug("Not enough PipelineRuns for component %s in namespace %s: %d/%d", compName, namespace, len(*prs), expectedCount)
			return false, nil
		}
		return true, nil
	}, interval, timeout)
	if err != nil {
		return nil, fmt.Errorf("Unable to list PipelineRuns for component %s in namespace %s: %v", compName, namespace, err)
	}

	logging.Logger.Debug("Found %d/%d PipelineRuns matching %s/%s/%s/%s", len(*prs), expectedCount, namespace, appName, compName, sha)
	return prs, nil
}

func listAndDeletePipelineRunsWithTimeout(f *framework.Framework, namespace, appName, compName, sha string, expectedCount int) error {
	var prs *[]pipeline.PipelineRun
	var err error

	prs, err = listPipelineRunsWithTimeout(f, namespace, appName, compName, sha, expectedCount)
	if err != nil {
		return err
	}
	for _, pr := range *prs {
		err = f.AsKubeDeveloper.TektonController.DeletePipelineRunIgnoreFinalizers(namespace, pr.Name)
		if err != nil {
			return fmt.Errorf("Error when deleting PipelineRun %s in namespace %s: %v", pr.Name, namespace, err)
		}
		logging.Logger.Debug("Deleted PipelineRun %s/%s", namespace, pr.Name)
	}

	return nil
}

// This handles post-component creation tasks for multi-arch PaC workflow
func utilityRepoTemplatingComponentCleanup(f *framework.Framework, namespace, appName, compName, repoUrl, repoRev string, mergeReqNum int, placeholders *map[string]string) error {
	var repoName string
	var err error

	// Delete on-pull-request default pipeline run
	err = listAndDeletePipelineRunsWithTimeout(f, namespace, appName, compName, "", 1)
	if err != nil {
		return fmt.Errorf("Error deleting on-pull-request default PipelineRun in namespace %s: %v", namespace, err)
	}
	logging.Logger.Debug("Repo-templating workflow: Cleaned up (first cleanup) for %s/%s/%s", namespace, appName, compName)

	// Merge default PaC pipelines PR
	repoName, err = getRepoNameFromRepoUrl(repoUrl)
	if err != nil {
		return fmt.Errorf("Failed parsing repo name: %v", err)
	}
	_, err = f.AsKubeAdmin.CommonController.Github.MergePullRequest(repoName, mergeReqNum)
	if err != nil {
		return fmt.Errorf("Merging %d failed: %v", mergeReqNum, err)
	}
	logging.Logger.Debug("Repo-templating workflow: Merged PR %d in %s", mergeReqNum, repoName)

	// Delete all pipeline runs as we do not care about these
	err = listAndDeletePipelineRunsWithTimeout(f, namespace, appName, compName, "", 1)
	if err != nil {
		return fmt.Errorf("Error deleting on-push merged PipelineRun in namespace %s: %v", namespace, err)
	}
	logging.Logger.Debug("Repo-templating workflow: Cleaned up (second cleanup) for %s/%s/%s", namespace, appName, compName)

	// Template our multi-arch PaC files
	shaMap, err := templateFiles(f, repoUrl, repoRev, placeholders)
	if err != nil {
		return fmt.Errorf("Error templating PaC files: %v", err)
	}
	logging.Logger.Debug("Repo-templating workflow: Our PaC files templated in %s", repoUrl)

	// Delete pipeline run we do not care about
	for file, sha := range *shaMap {
		if !strings.HasSuffix(file, "-push.yaml") {
			err = listAndDeletePipelineRunsWithTimeout(f, namespace, appName, compName, sha, 1)
			if err != nil {
				return fmt.Errorf("Error deleting on-push merged PipelineRun in namespace %s: %v", namespace, err)
			}
		}
	}
	logging.Logger.Debug("Repo-templating workflow: Cleaned up (third cleanup) for %s/%s/%s", namespace, appName, compName)

	return nil
}

func HandleComponent(ctx *PerComponentContext) error {
	var err error

	logging.Logger.Debug("Creating component %s in namespace %s", ctx.ComponentName, ctx.ParentContext.ParentContext.Namespace)

	// Create component
	_, err = logging.Measure(
		createComponent,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.ComponentName,
		ctx.ParentContext.ParentContext.ComponentRepoUrl,
		ctx.ParentContext.ParentContext.Opts.ComponentRepoRevision,
		ctx.ParentContext.ParentContext.Opts.ComponentContainerContext,
		ctx.ParentContext.ParentContext.Opts.ComponentContainerFile,
		ctx.ParentContext.ParentContext.Opts.BuildPipelineSelectorBundle,
		ctx.ParentContext.ApplicationName,
		ctx.ParentContext.ParentContext.Opts.PipelineMintmakerDisabled,
	)
	if err != nil {
		return logging.Logger.Fail(60, "Component failed creation: %v", err)
	}

	var pullIface interface{}
	pullIface, err = logging.Measure(
		getPaCPullNumber,
		ctx.Framework,
		ctx.ParentContext.ParentContext.Namespace,
		ctx.ComponentName,
	)
	if err != nil {
		return logging.Logger.Fail(61, "Component failed validation: %v", err)
	}

	// Get merge request number
	var ok bool
	ctx.MergeRequestNumber, ok = pullIface.(int)
	if !ok {
		return logging.Logger.Fail(62, "Type assertion failed on pull: %+v", pullIface)
	}

	// If this is multi-arch build, we do not care about this build, we just merge it, update pipelines and trigger actual multi-arch build
	if ctx.ParentContext.ParentContext.Opts.PipelineRepoTemplating {
		// Placeholders for template multi-arch PaC pipeline files
		placeholders := &map[string]string{
			"NAMESPACE":   ctx.ParentContext.ParentContext.Namespace,
			"QUAY_REPO":   ctx.ParentContext.ParentContext.Opts.QuayRepo,
			"APPLICATION": ctx.ParentContext.ApplicationName,
			"COMPONENT":   ctx.ComponentName,
			"BRANCH":      ctx.ParentContext.ParentContext.Opts.ComponentRepoRevision,
			"REPOURL":     ctx.ParentContext.ParentContext.ComponentRepoUrl,
		}

		// Skip what we do not care about
		_, err = logging.Measure(
			utilityRepoTemplatingComponentCleanup,
			ctx.Framework,
			ctx.ParentContext.ParentContext.Namespace,
			ctx.ParentContext.ApplicationName,
			ctx.ComponentName,
			ctx.ParentContext.ParentContext.ComponentRepoUrl,
			ctx.ParentContext.ParentContext.Opts.ComponentRepoRevision,
			ctx.MergeRequestNumber,
			placeholders,
		)
		if err != nil {
			return logging.Logger.Fail(63, "Repo-templating workflow component cleanup failed: %v", err)
		}

	}

	return nil
}
