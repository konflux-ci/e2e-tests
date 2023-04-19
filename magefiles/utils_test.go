package main

import (
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/image-controller/pkg/quay"
)

type QuayClientMock struct {
	AllRepositories         []quay.Repository
	AllRobotAccounts        []quay.RobotAccount
	DeleteRepositoryCalls   map[string]bool
	DeleteRobotAccountCalls map[string]bool
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

// Dummy functions
func (m *QuayClientMock) AddPermissionsToRobotAccount(organization, imageRepository, robotAccountName string) error {
	return nil
}

func (m *QuayClientMock) CreateRepository(r quay.RepositoryRequest) (*quay.Repository, error) {
	return nil, nil
}

func (m *QuayClientMock) CreateRobotAccount(organization string, robotName string) (*quay.RobotAccount, error) {
	return nil, nil
}

func TestCleanupQuay(t *testing.T) {
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
	err := cleanupQuay(&quayClientMock, "test-org")
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
