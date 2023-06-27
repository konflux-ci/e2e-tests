package build

import (
	"fmt"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
)

const (
	COMPONENT_REPO_URLS_ENV string = "COMPONENT_REPO_URLS"

	containerImageSource        = "quay.io/redhat-appstudio-qe/busybox-loop:latest"
	pythonComponentGitSourceURL = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic.git"
	dummyPipelineBundleRef      = "quay.io/redhat-appstudio-qe/dummy-pipeline-bundle:latest"
	buildTemplatesTestLabel     = "build-templates-e2e"
	buildTemplatesKcpTestLabel  = "build-templates-kcp-e2e"

	helloWorldComponentGitSourceRepoName = "devfile-sample-hello-world"
	helloWorldComponentDefaultBranch     = "default"
)

var (
	componentUrls                   = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitSourceURL), ",") //multiple urls
	componentNames                  []string
	helloWorldComponentGitSourceURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), helloWorldComponentGitSourceRepoName)
)
