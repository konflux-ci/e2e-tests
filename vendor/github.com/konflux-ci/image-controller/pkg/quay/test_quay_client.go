/*
Copyright 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package quay

import (
	. "github.com/onsi/ginkgo/v2"
)

const (
	TestQuayOrg = "user-workloads"
)

// TestQuayClient is a QuayClient for testing the controller
type TestQuayClient struct{}

var _ QuayService = (*TestQuayClient)(nil)

var (
	CreateRepositoryFunc                          func(repository RepositoryRequest) (*Repository, error)
	DeleteRepositoryFunc                          func(organization, imageRepository string) (bool, error)
	ChangeRepositoryVisibilityFunc                func(organization, imageRepository string, visibility string) error
	GetRobotAccountFunc                           func(organization string, robotName string) (*RobotAccount, error)
	CreateRobotAccountFunc                        func(organization string, robotName string) (*RobotAccount, error)
	DeleteRobotAccountFunc                        func(organization string, robotName string) (bool, error)
	AddPermissionsForRepositoryToRobotAccountFunc func(organization, imageRepository, robotAccountName string, isWrite bool) error
	RegenerateRobotAccountTokenFunc               func(organization string, robotName string) (*RobotAccount, error)
	GetNotificationsFunc                          func(organization, repository string) ([]Notification, error)
	CreateNotificationFunc                        func(organization, repository string, notification Notification) (*Notification, error)
)

func ResetTestQuayClient() {
	CreateRepositoryFunc = func(repository RepositoryRequest) (*Repository, error) { return &Repository{}, nil }
	DeleteRepositoryFunc = func(organization, imageRepository string) (bool, error) { return true, nil }
	ChangeRepositoryVisibilityFunc = func(organization, imageRepository string, visibility string) error { return nil }
	GetRobotAccountFunc = func(organization, robotName string) (*RobotAccount, error) { return &RobotAccount{}, nil }
	CreateRobotAccountFunc = func(organization, robotName string) (*RobotAccount, error) { return &RobotAccount{}, nil }
	DeleteRobotAccountFunc = func(organization, robotName string) (bool, error) { return true, nil }
	AddPermissionsForRepositoryToRobotAccountFunc = func(organization, imageRepository, robotAccountName string, isWrite bool) error { return nil }
	RegenerateRobotAccountTokenFunc = func(organization, robotName string) (*RobotAccount, error) { return &RobotAccount{}, nil }
	GetNotificationsFunc = func(organization, repository string) ([]Notification, error) { return []Notification{}, nil }
	CreateNotificationFunc = func(organization, repository string, notification Notification) (*Notification, error) {
		return &Notification{}, nil
	}

}

func ResetTestQuayClientToFails() {
	CreateRepositoryFunc = func(repository RepositoryRequest) (*Repository, error) {
		defer GinkgoRecover()
		Fail("CreateRepositoryFunc invoked")
		return nil, nil
	}
	DeleteRepositoryFunc = func(organization, imageRepository string) (bool, error) {
		defer GinkgoRecover()
		Fail("DeleteRepository invoked")
		return true, nil
	}
	ChangeRepositoryVisibilityFunc = func(organization, imageRepository string, visibility string) error {
		defer GinkgoRecover()
		Fail("ChangeRepositoryVisibility invoked")
		return nil
	}
	GetRobotAccountFunc = func(organization, robotName string) (*RobotAccount, error) {
		defer GinkgoRecover()
		Fail("GetRobotAccount invoked")
		return nil, nil
	}
	CreateRobotAccountFunc = func(organization, robotName string) (*RobotAccount, error) {
		defer GinkgoRecover()
		Fail("CreateRobotAccount invoked")
		return nil, nil
	}
	DeleteRobotAccountFunc = func(organization, robotName string) (bool, error) {
		defer GinkgoRecover()
		Fail("DeleteRobotAccount invoked")
		return true, nil
	}
	AddPermissionsForRepositoryToRobotAccountFunc = func(organization, imageRepository, robotAccountName string, isWrite bool) error {
		defer GinkgoRecover()
		Fail("AddPermissionsForRepositoryToRobotAccount invoked")
		return nil
	}
	RegenerateRobotAccountTokenFunc = func(organization, robotName string) (*RobotAccount, error) {
		defer GinkgoRecover()
		Fail("RegenerateRobotAccountToken invoked")
		return nil, nil
	}
	GetNotificationsFunc = func(organization, repository string) ([]Notification, error) {
		defer GinkgoRecover()
		Fail("RegenerateRobotAccountToken invoked")
		return nil, nil
	}
	CreateNotificationFunc = func(organization, repository string, notification Notification) (*Notification, error) {
		defer GinkgoRecover()
		Fail("CreateNotification invoked")
		return nil, nil
	}
}

func (c TestQuayClient) CreateRepository(repositoryRequest RepositoryRequest) (*Repository, error) {
	return CreateRepositoryFunc(repositoryRequest)
}
func (c TestQuayClient) DeleteRepository(organization, imageRepository string) (bool, error) {
	return DeleteRepositoryFunc(organization, imageRepository)
}
func (TestQuayClient) ChangeRepositoryVisibility(organization, imageRepository string, visibility string) error {
	return ChangeRepositoryVisibilityFunc(organization, imageRepository, visibility)
}
func (c TestQuayClient) GetRobotAccount(organization string, robotName string) (*RobotAccount, error) {
	return GetRobotAccountFunc(organization, robotName)
}
func (c TestQuayClient) CreateRobotAccount(organization string, robotName string) (*RobotAccount, error) {
	return CreateRobotAccountFunc(organization, robotName)
}
func (c TestQuayClient) DeleteRobotAccount(organization string, robotName string) (bool, error) {
	return DeleteRobotAccountFunc(organization, robotName)
}
func (c TestQuayClient) AddPermissionsForRepositoryToRobotAccount(organization, imageRepository, robotAccountName string, isWrite bool) error {
	return AddPermissionsForRepositoryToRobotAccountFunc(organization, imageRepository, robotAccountName, isWrite)
}
func (c TestQuayClient) RegenerateRobotAccountToken(organization string, robotName string) (*RobotAccount, error) {
	return RegenerateRobotAccountTokenFunc(organization, robotName)
}
func (c TestQuayClient) GetAllRepositories(organization string) ([]Repository, error) {
	return nil, nil
}
func (c TestQuayClient) GetAllRobotAccounts(organization string) ([]RobotAccount, error) {
	return nil, nil
}
func (TestQuayClient) DeleteTag(organization string, repository string, tag string) (bool, error) {
	return true, nil
}
func (TestQuayClient) GetTagsFromPage(organization string, repository string, page int) ([]Tag, bool, error) {
	return nil, false, nil
}
func (TestQuayClient) GetNotifications(organization string, repository string) ([]Notification, error) {
	return GetNotificationsFunc(organization, repository)
}

func (TestQuayClient) CreateNotification(organization, repository string, notification Notification) (*Notification, error) {
	return CreateNotificationFunc(organization, repository, notification)
}
