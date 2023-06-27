package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redhat-appstudio/image-controller/pkg/quay"
)

type QuayClientMock struct {
	AllRepositories         []quay.Repository
	AllRobotAccounts        []quay.RobotAccount
	AllTags                 []quay.Tag
	DeleteRepositoryCalls   map[string]bool
	DeleteRobotAccountCalls map[string]bool
	DeleteTagCalls          map[string]bool
	TagsOnPage              int
	TagPages                int
	Benchmark               bool
}

var _ quay.QuayService = (*QuayClientMock)(nil)

func (m *QuayClientMock) GetAllRepositories(organization string) ([]quay.Repository, error) {
	return m.AllRepositories, nil
}

func (m *QuayClientMock) GetAllRobotAccounts(organization string) ([]quay.RobotAccount, error) {
	return m.AllRobotAccounts, nil
}

func (m *QuayClientMock) DeleteRepository(organization, repoName string) (bool, error) {
	m.DeleteRepositoryCalls[repoName] = true
	return true, nil
}

func (m *QuayClientMock) DeleteRobotAccount(organization, robotName string) (bool, error) {
	m.DeleteRobotAccountCalls[robotName] = true
	return true, nil
}

func (m *QuayClientMock) GetTagsFromPage(organization, repository string, page int) ([]quay.Tag, bool, error) {
	if m.Benchmark {
		time.Sleep(100 * time.Millisecond) // Mock delay for request
	}
	if page == m.TagPages {
		return m.AllTags[(page-1)*m.TagsOnPage : (page * m.TagsOnPage)], false, nil
	}
	return m.AllTags[(page-1)*m.TagsOnPage : (page * m.TagsOnPage)], true, nil
}

var deleteTagCallsMutex = sync.RWMutex{}

func (m *QuayClientMock) DeleteTag(organization, repository, tag string) (bool, error) {
	if m.Benchmark {
		time.Sleep(100 * time.Millisecond) // Mock delay for request
	}
	deleteTagCallsMutex.Lock()
	m.DeleteTagCalls[tag] = true
	deleteTagCallsMutex.Unlock()
	return true, nil
}

// Dummy functions
func (m *QuayClientMock) AddWritePermissionsToRobotAccount(organization, imageRepository, robotAccountName string) error {
	return nil
}

func (m *QuayClientMock) CreateRepository(r quay.RepositoryRequest) (*quay.Repository, error) {
	return nil, nil
}

func (m *QuayClientMock) CreateRobotAccount(organization string, robotName string) (*quay.RobotAccount, error) {
	return nil, nil
}

func (m *QuayClientMock) GetRobotAccount(organization string, robotName string) (*quay.RobotAccount, error) {
	return nil, nil
}

func TestCleanupQuayReposAndRobots(t *testing.T) {
	timeFormat := "Mon, 02 Jan 2006 15:04:05 -0700"

	deletedRepos := []quay.Repository{
		{Name: "e2e-demos/test-old"},
		{Name: "has-e2e/test-old"},
	}
	preservedRepos := []quay.Repository{
		{Name: "e2e-demos/test-new"},
		{Name: "has-e2e/test-new"},
		{Name: "other/test-new"},
		{Name: "other/test-old"},
	}
	deletedRobots := []quay.RobotAccount{
		{Name: "test-org+e2e-demostest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)},
		{Name: "test-org+has-e2etest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)},
	}
	preservedRobots := []quay.RobotAccount{
		{Name: "test-org+e2e-demostest-new", Created: time.Now().Format(timeFormat)},
		{Name: "test-org+has-e2etest-new", Created: time.Now().Format(timeFormat)},
		{Name: "test-org+othertest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)},
		{Name: "test-org+othertest-new", Created: time.Now().Format(timeFormat)},
	}
	quayClientMock := QuayClientMock{
		AllRepositories:         append(deletedRepos, preservedRepos...),
		AllRobotAccounts:        append(deletedRobots, preservedRobots...),
		DeleteRepositoryCalls:   make(map[string]bool),
		DeleteRobotAccountCalls: make(map[string]bool),
	}
	err := cleanupQuayReposAndRobots(&quayClientMock, "test-org")
	if err != nil {
		t.Errorf("error during quay cleanup, error: %s", err)
	}

	for _, repo := range deletedRepos {
		if !quayClientMock.DeleteRepositoryCalls[repo.Name] {
			t.Errorf("DeleteRepository() should have been called for '%s'", repo.Name)
		}
	}
	for _, repo := range preservedRepos {
		if quayClientMock.DeleteRepositoryCalls[repo.Name] {
			t.Errorf("DeleteRepository() should not have been called for '%s'", repo.Name)
		}
	}
	for _, robot := range deletedRobots {
		shortName := strings.Split(robot.Name, "+")[1]
		if !quayClientMock.DeleteRobotAccountCalls[shortName] {
			t.Errorf("DeleteRobotAccount() should have been called for '%s'", shortName)
		}
	}
	for _, robot := range preservedRobots {
		shortName := strings.Split(robot.Name, "+")[1]
		if quayClientMock.DeleteRepositoryCalls[shortName] {
			t.Errorf("DeleteRobotAccount() should not have been called for '%s'", shortName)
		}
	}
}

func TestCleanupQuayTags(t *testing.T) {
	testOrg := "test-org"
	testRepo := "test-repo"

	tagsOnPage := 20
	tagPages := 20

	var deletedTags []quay.Tag
	var preservedTags []quay.Tag
	var allTags []quay.Tag

	// Randomly generate slices of deleted and preserved tags
	for i := 0; i < tagsOnPage*tagPages; i++ {
		tagName := fmt.Sprintf("tag%d", i)
		var tag quay.Tag
		if rand.Intn(2) == 0 {
			tag = quay.Tag{Name: tagName, StartTS: time.Now().AddDate(0, 0, -8).Unix()}
			deletedTags = append(deletedTags, tag)
		} else {
			tag = quay.Tag{Name: tagName, StartTS: time.Now().Unix()}
			preservedTags = append(preservedTags, tag)
		}
		allTags = append(allTags, tag)
	}

	quayClientMock := QuayClientMock{
		AllTags:        allTags,
		DeleteTagCalls: make(map[string]bool),
		TagsOnPage:     tagsOnPage,
		TagPages:       tagPages,
	}

	err := cleanupQuayTags(&quayClientMock, testOrg, testRepo)
	if err != nil {
		t.Errorf("error during quay tag cleanup, error: %s", err)
	}

	for _, tag := range deletedTags {
		if !quayClientMock.DeleteTagCalls[tag.Name] {
			t.Errorf("DeleteTag() should have been called for '%s'", tag.Name)
		}
	}
	for _, tag := range preservedTags {
		if quayClientMock.DeleteTagCalls[tag.Name] {
			t.Errorf("DeleteTag() should not have been called for '%s'", tag.Name)
		}
	}
}

func BenchmarkCleanupQuayTags(b *testing.B) {
	testOrg := "test-org"
	testRepo := "test-repo"
	var allTags []quay.Tag

	tagsOnPage := 20
	tagPages := 20

	// Randomly generate slices of deleted and preserved tags
	for i := 0; i < tagsOnPage*tagPages; i++ {
		tagName := fmt.Sprintf("tag%d", i)
		var tag quay.Tag
		if rand.Intn(2) == 0 {
			tag = quay.Tag{Name: tagName, StartTS: time.Now().AddDate(0, 0, -8).Unix()}
		} else {
			tag = quay.Tag{Name: tagName, StartTS: time.Now().Unix()}
		}
		allTags = append(allTags, tag)
	}

	quayClientMock := QuayClientMock{
		AllTags:        allTags,
		DeleteTagCalls: make(map[string]bool),
		TagsOnPage:     tagsOnPage,
		TagPages:       tagPages,
		Benchmark:      true,
	}
	err := cleanupQuayTags(&quayClientMock, testOrg, testRepo)
	if err != nil {
		b.Errorf("error during quay tag cleanup, error: %s", err)
	}
}
