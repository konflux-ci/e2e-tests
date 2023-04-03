package main

import (
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
	quayClientMock := QuayClientMock{
		AllRepositories: []quay.Repository{
			{Name: "e2e-demos/test-old"},
			{Name: "e2e-demos/test-new"},
			{Name: "has-e2e/test-old"},
			{Name: "has-e2e/test-new"},
			{Name: "other/test-new"},
			{Name: "other/test-old"},
		},
		AllRobotAccounts: []quay.RobotAccount{
			{Name: "test-org+e2e-demostest-old", Created: time.Now().Add(-25 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")},
			{Name: "test-org+e2e-demostest-new", Created: time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700")},
			{Name: "test-org+has-e2etest-old", Created: time.Now().Add(-25 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")},
			{Name: "test-org+has-e2etest-new", Created: time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700")},
			{Name: "test-org+othertest-old", Created: time.Now().Add(-25 * time.Hour).Format("Mon, 02 Jan 2006 15:04:05 -0700")},
			{Name: "test-org+othertest-new", Created: time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700")},
		},
		DeleteRepositoryCalls:   make(map[string]bool),
		DeleteRobotAccountCalls: make(map[string]bool),
	}
	err := cleanupQuay(&quayClientMock, "test-org")
	if err != nil {
		t.Errorf("error during quay cleanup, error: %s", err)
	}

	if !quayClientMock.DeleteRepositoryCalls["e2e-demos/test-old"] {
		t.Error("DeleteRepository() should have been called for 'e2e-demos/test-old'")
	}
	if quayClientMock.DeleteRepositoryCalls["e2e-demos/test-new"] {
		t.Error("DeleteRepository() should not have been called for 'e2e-demos/test-new'")
	}

	if !quayClientMock.DeleteRepositoryCalls["has-e2e/test-old"] {
		t.Error("DeleteRepository() should have been called for 'has-e2e/test-old'")
	}
	if quayClientMock.DeleteRepositoryCalls["has-e2e/test-new"] {
		t.Error("DeleteRepository() should not have been called for 'has-e2e/test-new'")
	}

	if quayClientMock.DeleteRepositoryCalls["other/test-old"] {
		t.Error("DeleteRepository() should not have been called for 'other/test-old'")
	}
	if quayClientMock.DeleteRepositoryCalls["other/test-new"] {
		t.Error("DeleteRepository() should not have been called for 'other/test-new'")
	}

	if !quayClientMock.DeleteRobotAccountCalls["e2e-demostest-old"] {
		t.Error("DeleteRobotAccount() should have been called for 'test-org+e2e-demostest-old'")
	}
	if quayClientMock.DeleteRobotAccountCalls["e2e-demostest-new"] {
		t.Error("DeleteRobotAccount() should not have been called for 'test-org+e2e-demostest-new'")
	}

	if !quayClientMock.DeleteRobotAccountCalls["has-e2etest-old"] {
		t.Error("DeleteRobotAccount() should have been called for 'test-org+has-e2etest-old'")
	}
	if quayClientMock.DeleteRobotAccountCalls["has-e2etest-new"] {
		t.Error("DeleteRobotAccount() should not have been called for 'test-org+has-e2etest-new'")
	}

	if quayClientMock.DeleteRobotAccountCalls["othertest-old"] {
		t.Error("DeleteRobotAccount() should not have been called for 'test-org+othertest-old'")
	}
	if quayClientMock.DeleteRobotAccountCalls["othertest-new"] {
		t.Error("DeleteRobotAccount() should not have been called for 'test-org+othertest-new'")
	}

}
