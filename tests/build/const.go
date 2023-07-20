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
	helloWorldComponentRevision          = "e17b15c4ea1fd4a2553e17ab24561bbe40d0e2f4"
)

var (
	componentUrls                   = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitSourceURL), ",") //multiple urls
	componentNames                  []string
	helloWorldComponentGitSourceURL = fmt.Sprintf("https://github.com/%s/%s", utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe"), helloWorldComponentGitSourceRepoName)
)
