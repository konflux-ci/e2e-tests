package build

import (
	"fmt"
	"strings"

	"github.com/konflux-ci/e2e-tests/pkg/constants"
	"github.com/konflux-ci/e2e-tests/pkg/utils"
)

const (
	COMPONENT_REPO_URLS_ENV string = "COMPONENT_REPO_URLS"
	PR_CHANGED_FILES_ENV    string = "PR_CHANGED_FILES"

	containerImageSource             = "quay.io/redhat-appstudio-qe/busybox-loop@sha256:f698f1f2cf641fe9176d2a277c9052d872f6b1c39e56248a1dd259b96281dda9"
	gitRepoContainsSymlinkBranchName = "symlink"
	symlinkBranchRevision            = "27ecfca9c9dad35e4f07ebbcd706f31cb7ce849f"
	dummyPipelineBundleRef           = "quay.io/redhat-appstudio-qe/dummy-pipeline-bundle@sha256:9805fc3f309af8f838622e49d3e7705d8364eb5c8287043d5725f3ef12232f24"
	buildTemplatesTestLabel          = "build-templates-e2e"
	buildTemplatesKcpTestLabel       = "build-templates-kcp-e2e"
	sourceBuildTestLabel             = "source-build-e2e"

	helloWorldComponentGitSourceRepoName = "devfile-sample-hello-world"
	helloWorldComponentDefaultBranch     = "default"
	helloWorldComponentRevision          = "d2d03e69de912e3827c29b4c5b71ffe8bcb5dad8"

	multiComponentGitSourceRepoName = "sample-multi-component"
	multiComponentDefaultBranch     = "main"
	multiComponentGitRevision       = "0d1835404efb8ab7bb1ab5b5b82cda1ebfda4b25"

	secretLookupGitSourceRepoOneName = "secret-lookup-sample-repo-one"
	secretLookupDefaultBranchOne     = "main"
	secretLookupGitRevisionOne       = "4b86bbfba19586f9ec8b648b3f47de3a5c62d460"
	secretLookupGitSourceRepoTwoName = "secret-lookup-sample-repo-two"
	secretLookupDefaultBranchTwo     = "main"
	secretLookupGitRevisionTwo       = "9fd1358a22212d03ed938ea3bed8df98dddd2652"

	annotationsTestGitSourceRepoName = "multi-stage-build-go-sample"
	annotationsTestRevision          = "529f65798777a5fe145e33d58e1e91c4c03704a4"

	componentDependenciesParentRepoName      = "build-nudge-parent"
	componentDependenciesParentDefaultBranch = "main"
	componentDependenciesParentGitRevision   = "cb87720e960c9d1d7f591dc69d75cfa7ef6b3b4a"
	componentDependenciesChildRepoName       = "build-nudge-child"
	componentDependenciesChildDefaultBranch  = "main"
	componentDependenciesChildGitRevision    = "56c13d17b1a8f801f2c41091e6f4e62cf16ee5f2"

	githubUrlFormat = "https://github.com/%s/%s"
	gitlabUrlFormat = "https://gitlab.com/%s"

	//Logging related
	buildStatusAnnotationValueLoggingFormat = "build status annotation value: %s\n"

	noAppOrgName            = "redhat-appstudio-qe-no-app"
	pythonComponentRepoName = "devfile-sample-python-basic"
)

var (
	additionalTags                     = []string{"test-tag1", "test-tag2"}
	componentUrls                      = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitHubURL), ",") //multiple urls
	githubOrg                          = utils.GetEnv(constants.GITHUB_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
	gitlabOrg                          = utils.GetEnv(constants.GITLAB_QE_ORG_ENV, constants.DefaultGitLabQEOrg)
	helloWorldComponentGitHubURL       = fmt.Sprintf(githubUrlFormat, githubOrg, helloWorldComponentGitSourceRepoName)
	annotationsTestGitHubURL           = fmt.Sprintf(githubUrlFormat, githubOrg, annotationsTestGitSourceRepoName)
	helloWorldComponentGitLabProjectID = fmt.Sprintf("%s/%s", gitlabOrg, helloWorldComponentGitSourceRepoName)
	helloWorldComponentGitLabURL       = fmt.Sprintf(gitlabUrlFormat, helloWorldComponentGitLabProjectID)
	multiComponentGitHubURL            = fmt.Sprintf(githubUrlFormat, githubOrg, multiComponentGitSourceRepoName)
	multiComponentContextDirs          = []string{"go-component", "python-component"}
	pythonComponentGitHubURL           = fmt.Sprintf(githubUrlFormat, githubOrg, pythonComponentRepoName)

	secretLookupComponentOneGitSourceURL = fmt.Sprintf(githubUrlFormat, noAppOrgName, secretLookupGitSourceRepoOneName)
	secretLookupComponentTwoGitSourceURL = fmt.Sprintf(githubUrlFormat, noAppOrgName, secretLookupGitSourceRepoTwoName)

	basicScenarioUrls    = []string{"https://github.com/konflux-qe-bd/devfile-sample-python-basic", "https://github.com/konflux-qe-bd/devfile-sample-python-basic-clone", "https://github.com/konflux-qe-bd/multiarch-sample-repo", "https://github.com/konflux-qe-bd/multiarch-sample-repo-clone", "https://github.com/konflux-qe-bd/fbc-sample-repo", "https://github.com/konflux-qe-bd/docker-file-from-scratch", "https://github.com/konflux-qe-bd/oci-archive-test"}
	hermeticScenarioUrls = []string{"https://github.com/konflux-qe-bd/retrodep", "https://github.com/konflux-qe-bd/pip-e2e-test", "https://github.com/konflux-qe-bd/ruby-bundler-sample-app", "https://github.com/konflux-qe-bd/rust-cargo-sample-app", "https://github.com/konflux-qe-bd/nodejs-npm-sample-repo", "https://github.com/konflux-qe-bd/nodejs-yarn-sample-app", "https://github.com/konflux-qe-bd/nodejs-yarn-modern-sample-app", "https://github.com/konflux-qe-bd/rpm-sample-app", "https://github.com/konflux-qe-bd/generic-fetcher-sample-app"}
)
