package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/konflux-ci/image-controller/pkg/quay"
)

type QuayClientMock struct {
	AllRepositories         []quay.Repository
	AllRobotAccounts        []quay.RobotAccount
	AllTags                 []quay.Tag
	DeleteRepositoryCalls   map[string]bool
	DeleteRobotAccountCalls map[string]bool
	DeleteTagCalls          map[string]bool
	DeleteRepositoryErrors  map[string]error
	DeleteRobotErrors       map[string]error
	TagsOnPage              int
	TagPages                int
	Benchmark               bool
}

var _ quay.QuayService = (*QuayClientMock)(nil)

func (m *QuayClientMock) AddPermissionsForRepositoryToRobotAccount(string, string, string, bool) error {
	return nil
}

func (m *QuayClientMock) RegenerateRobotAccountToken(string, string) (*quay.RobotAccount, error) {
	return nil, nil
}

func (m *QuayClientMock) GetAllRepositories(organization string) ([]quay.Repository, error) {
	return m.AllRepositories, nil
}

func (m *QuayClientMock) GetAllRobotAccounts(organization string) ([]quay.RobotAccount, error) {
	return m.AllRobotAccounts, nil
}

func (m *QuayClientMock) DeleteRepository(organization, repoName string) (bool, error) {
	m.DeleteRepositoryCalls[repoName] = true
	if err := m.DeleteRepositoryErrors[repoName]; err != nil {
		return false, err
	}
	return true, nil
}

func (m *QuayClientMock) DeleteRobotAccount(organization, robotName string) (bool, error) {
	m.DeleteRobotAccountCalls[robotName] = true
	if err := m.DeleteRobotErrors[robotName]; err != nil {
		return false, err
	}
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

func (m *QuayClientMock) AddPermissionsForRepositoryToAccount(organization, imageRepository, accountName string, isRobot, isWrite bool) error {
	return nil
}

func (m *QuayClientMock) AddReadPermissionsForRepositoryToTeam(organization, imageRepository, teamName string) error {
	return nil
}

func (m *QuayClientMock) AddUserToTeam(organization, teamName, userName string) (bool, error) {
	return false, nil
}

func (m *QuayClientMock) DeleteNotification(organization, repository string, notificationUuid string) (bool, error) {
	return false, nil
}

func (m *QuayClientMock) DeleteTeam(organization, teamName string) error {
	return nil
}

func (m *QuayClientMock) EnsureTeam(organization, teamName string) ([]quay.Member, error) {
	return nil, nil
}

func (m *QuayClientMock) GetTeamMembers(organization, teamName string) ([]quay.Member, error) {
	return nil, nil
}

func (m *QuayClientMock) ListPermissionsForRepository(organization, imageRepository string) (map[string]quay.UserAccount, error) {
	return nil, nil
}

func (m *QuayClientMock) ListRepositoryPermissionsForTeam(organization, teamName string) ([]quay.TeamPermission, error) {
	return nil, nil
}

func (m *QuayClientMock) RemovePermissionsForRepositoryFromAccount(organization, imageRepository, accountName string, isRobot bool) error {
	return nil
}

func (m *QuayClientMock) RemoveUserFromTeam(organization, teamName, userName string) error {
	return nil
}

func (m *QuayClientMock) RepositoryExists(organization, imageRepository string) (bool, error) {
	return false, nil
}

func (m *QuayClientMock) UpdateNotification(organization, repository string, notificationUuid string, notification quay.Notification) (*quay.Notification, error) {
	return nil, nil
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

func (m *QuayClientMock) ChangeRepositoryVisibility(organization, imageRepository string, visibility string) error {
	return nil
}

func (m *QuayClientMock) GetNotifications(organization, repository string) ([]quay.Notification, error) {
	return nil, nil
}

func (m *QuayClientMock) CreateNotification(organization, repository string, notification quay.Notification) (*quay.Notification, error) {
	return nil, nil
}

func TestCleanupQuayReposAndRobots(t *testing.T) {
	timeFormat := "Mon, 02 Jan 2006 15:04:05 -0700"
	deletionStagger = 1 * time.Millisecond

	deletedRepos := []quay.Repository{
		{Name: "rhtap-demo/test-old", LastModified: int(time.Now().Unix()) - 25*3600},
		{Name: "multi-platform/test-old", LastModified: int(time.Now().Unix()) - 25*3600},
	}
	preservedRepos := []quay.Repository{
		{Name: "konflux-demo/test-new", LastModified: int(time.Now().Unix()) - 21*3600},
		{Name: "multi-platform/test-new", LastModified: int(time.Now().Unix()) - 15*3600},
		{Name: "other/test-new"},
		{Name: "other/test-old"},
	}
	deletedRobots := []quay.RobotAccount{
		{Name: "test-org+rhtap-demotest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)},
		{Name: "test-org+multi-platformtest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)},
	}
	preservedRobots := []quay.RobotAccount{
		{Name: "test-org+konflux-demotest-new", Created: time.Now().Format(timeFormat)},
		{Name: "test-org+multi-platformtest-new", Created: time.Now().Format(timeFormat)},
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
		if quayClientMock.DeleteRobotAccountCalls[shortName] {
			t.Errorf("DeleteRobotAccount() should not have been called for '%s'", shortName)
		}
	}
}

func TestCleanupQuayReposAndRobots502Error(t *testing.T) {
	timeFormat := "Mon, 02 Jan 2006 15:04:05 -0700"
	deletionStagger = 1 * time.Millisecond

	// Both repos match a delete prefix and are old enough to be deleted.
	// The first one will have the server respond with a 502 Bad Gateway.
	failingRepo := quay.Repository{Name: "rhtap-demo/test-old", LastModified: int(time.Now().Unix()) - 25*3600}
	okRepo := quay.Repository{Name: "multi-platform/test-old", LastModified: int(time.Now().Unix()) - 25*3600}

	// Both robots match a delete prefix and are old enough to be deleted.
	// The first one will have the server respond with a 502 Bad Gateway.
	failingRobot := quay.RobotAccount{Name: "test-org+rhtap-demotest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)}
	okRobot := quay.RobotAccount{Name: "test-org+multi-platformtest-old", Created: time.Now().Add(-25 * time.Hour).Format(timeFormat)}

	// Robot error injection is keyed by the robot shortname (part after '+'),
	// which is what cleanupQuayReposAndRobots passes to DeleteRobotAccount.
	failingRobotShortName := strings.Split(failingRobot.Name, "+")[1]

	repo502Err := fmt.Errorf("failed to delete repository, got status code 502: 502 Bad Gateway")
	robot502Err := fmt.Errorf("failed to delete robot account, got status code 502: 502 Bad Gateway")

	quayClientMock := QuayClientMock{
		AllRepositories:         []quay.Repository{failingRepo, okRepo},
		AllRobotAccounts:        []quay.RobotAccount{failingRobot, okRobot},
		DeleteRepositoryCalls:   make(map[string]bool),
		DeleteRobotAccountCalls: make(map[string]bool),
		DeleteRepositoryErrors:  map[string]error{failingRepo.Name: repo502Err},
		DeleteRobotErrors:       map[string]error{failingRobotShortName: robot502Err},
	}

	err := cleanupQuayReposAndRobots(&quayClientMock, "test-org")

	// The 502 failures must be surfaced as a returned error.
	if err == nil {
		t.Fatal("expected an error to be returned when the server responds with 502, got nil")
	}
	for _, want := range []string{"502", failingRepo.Name, failingRobot.Name} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("returned error %q should mention %q", err.Error(), want)
		}
	}

	// Deletion must have been attempted for all matching entries, including the failing ones.
	if !quayClientMock.DeleteRepositoryCalls[failingRepo.Name] {
		t.Errorf("DeleteRepository() should have been attempted for '%s'", failingRepo.Name)
	}
	if !quayClientMock.DeleteRobotAccountCalls[failingRobotShortName] {
		t.Errorf("DeleteRobotAccount() should have been attempted for '%s'", failingRobotShortName)
	}

	// Cleanup must continue after a 502: the other matching entries are still deleted.
	if !quayClientMock.DeleteRepositoryCalls[okRepo.Name] {
		t.Errorf("DeleteRepository() should still have been called for '%s' after a 502 on another repo", okRepo.Name)
	}
	okRobotShortName := strings.Split(okRobot.Name, "+")[1]
	if !quayClientMock.DeleteRobotAccountCalls[okRobotShortName] {
		t.Errorf("DeleteRobotAccount() should still have been called for '%s' after a 502 on another robot", okRobotShortName)
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
