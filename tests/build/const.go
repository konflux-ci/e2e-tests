package build

import (
	"fmt"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

const (
	COMPONENT_REPO_URLS_ENV string = "COMPONENT_REPO_URLS"

	containerImageSource             = "quay.io/redhat-appstudio-qe/busybox-loop@sha256:f698f1f2cf641fe9176d2a277c9052d872f6b1c39e56248a1dd259b96281dda9"
	pythonComponentGitSourceURL      = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic.git"
	gitRepoContainsSymlinkBranchName = "symlink"
	dummyPipelineBundleRef           = "quay.io/redhat-appstudio-qe/dummy-pipeline-bundle@sha256:9805fc3f309af8f838622e49d3e7705d8364eb5c8287043d5725f3ef12232f24"
	buildTemplatesTestLabel          = "build-templates-e2e"
	buildTemplatesKcpTestLabel       = "build-templates-kcp-e2e"

	helloWorldComponentGitSourceRepoName = "devfile-sample-hello-world"
	helloWorldComponentDefaultBranch     = "default"
	helloWorldComponentRevision          = "b915157dc9efac492ebc285d4a44ce67e6ab2075"

	multiComponentGitSourceRepoName = "sample-multi-component"
	multiComponentDefaultBranch     = "main"
	multiComponentGitRevision       = "2e006bd8b58483e4d7999c5931b65c4d4550d223"

	annotationsTestGitSourceRepoName = "multi-stage-build-go-sample"
	annotationsTestRevision          = "529f65798777a5fe145e33d58e1e91c4c03704a4"

	componentDependenciesParentRepoName      = "build-nudge-parent"
	componentDependenciesParentDefaultBranch = "main"
	componentDependenciesParentGitRevision   = "a8dce08dbdf290e5d616a83672ad3afcb4b455ef"
	componentDependenciesChildRepoName       = "build-nudge-child"
	componentDependenciesChildDefaultBranch  = "main"
	componentDependenciesChildGitRevision    = "56c13d17b1a8f801f2c41091e6f4e62cf16ee5f2"

	githubUrlFormat = "https://github.com/%s/%s"

	appstudioCrdsBuild = "appstudio-crds-build"
	computeBuild       = "compute-build"

	//Logging related
	buildStatusAnnotationValueLoggingFormat = "build status annotation value: %s\n"
)

var (
	componentUrls                   = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitSourceURL), ",") //multiple urls
	componentNames                  []string
	gihubOrg                        = utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	helloWorldComponentGitSourceURL = fmt.Sprintf(githubUrlFormat, gihubOrg, helloWorldComponentGitSourceRepoName)
	annotationsTestGitSourceURL     = fmt.Sprintf(githubUrlFormat, gihubOrg, annotationsTestGitSourceRepoName)
	multiComponentGitSourceURL      = fmt.Sprintf(githubUrlFormat, gihubOrg, multiComponentGitSourceRepoName)
	multiComponentContextDirs       = []string{"go-component", "python-component"}
)
