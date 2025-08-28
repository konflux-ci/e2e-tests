package git

import (
	"net/url"
	"strings"
)

const InternalDefaultBranch = "$DEFAULTBRANCH"

type ScmComponent struct {
	namespaceName string
	componentName string
	repositoryUrl *url.URL
	branch        string
	platform      string
}

func NewScmComponent(platform string, repositoryUrl string, revision string, componentName string, namespaceName string) (*ScmComponent, error) {
	url, err := url.Parse(strings.TrimSuffix(strings.TrimSuffix(repositoryUrl, ".git"), "/"))
	if err != nil {
		return nil, err
	}
	branch := revision
	if branch == "" {
		branch = InternalDefaultBranch
	}

	return &ScmComponent{platform: platform, branch: branch, repositoryUrl: url, componentName: componentName, namespaceName: namespaceName}, nil
}

func (s ScmComponent) Repository() string {
	return strings.Trim(s.repositoryUrl.Path, "/")
}

func (s ScmComponent) Platform() string {
	return s.platform
}

func (s ScmComponent) Branch() string {
	return s.branch
}

func (s ScmComponent) RepositoryUrl() *url.URL {
	return s.repositoryUrl
}
func (s ScmComponent) RepositoryUrlString() string {
	return s.repositoryUrl.String()
}
func (s ScmComponent) RepositoryHost() string {
	return s.repositoryUrl.Host
}

func (s ScmComponent) ComponentName() string {
	return s.componentName
}

func (s ScmComponent) NamespaceName() string {
	return s.namespaceName
}

func ComponentUrlToBranchesMap(components []*ScmComponent) map[string][]string {
	componentUrlToBranchesMap := make(map[string][]string)
	for _, component := range components {
		componentUrlToBranchesMap[component.RepositoryUrlString()] = append(componentUrlToBranchesMap[component.RepositoryUrlString()], component.Branch())
	}
	return componentUrlToBranchesMap
}

func ComponentRepoToBranchesMap(components []*ScmComponent) map[string][]string {
	componentRepoToBranchesMap := make(map[string][]string)
	for _, component := range components {
		componentRepoToBranchesMap[component.Repository()] = append(componentRepoToBranchesMap[component.Repository()], component.Branch())
	}
	return componentRepoToBranchesMap
}

func NamespaceToComponentMap(components []*ScmComponent) map[string][]*ScmComponent {
	componentNamespaceNameMap := make(map[string][]*ScmComponent)
	for _, component := range components {
		componentNamespaceNameMap[component.NamespaceName()] = append(componentNamespaceNameMap[component.NamespaceName()], component)
	}
	return componentNamespaceNameMap
}
func PlatformToComponentMap(components []*ScmComponent) map[string][]*ScmComponent {
	componentNamespaceNameMap := make(map[string][]*ScmComponent)
	for _, component := range components {
		componentNamespaceNameMap[component.Platform()] = append(componentNamespaceNameMap[component.Platform()], component)
	}
	return componentNamespaceNameMap
}
func HostToComponentMap(components []*ScmComponent) map[string][]*ScmComponent {
	componentNamespaceNameMap := make(map[string][]*ScmComponent)
	for _, component := range components {
		componentNamespaceNameMap[component.RepositoryHost()] = append(componentNamespaceNameMap[component.RepositoryHost()], component)
	}
	return componentNamespaceNameMap
}
