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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
)

type QuayService interface {
	CreateRepository(repositoryRequest RepositoryRequest) (*Repository, error)
	DeleteRepository(organization, imageRepository string) (bool, error)
	ChangeRepositoryVisibility(organization, imageRepository, visibility string) error
	GetRobotAccount(organization string, robotName string) (*RobotAccount, error)
	CreateRobotAccount(organization string, robotName string) (*RobotAccount, error)
	DeleteRobotAccount(organization string, robotName string) (bool, error)
	AddPermissionsForRepositoryToRobotAccount(organization, imageRepository, robotAccountName string, isWrite bool) error
	RegenerateRobotAccountToken(organization string, robotName string) (*RobotAccount, error)
	GetAllRepositories(organization string) ([]Repository, error)
	GetAllRobotAccounts(organization string) ([]RobotAccount, error)
	GetTagsFromPage(organization, repository string, page int) ([]Tag, bool, error)
	DeleteTag(organization, repository, tag string) (bool, error)
}

var _ QuayService = (*QuayClient)(nil)

type QuayClient struct {
	url        string
	httpClient *http.Client
	AuthToken  string
}

func NewQuayClient(c *http.Client, authToken, url string) *QuayClient {
	return &QuayClient{
		httpClient: c,
		AuthToken:  authToken,
		url:        url,
	}
}

// QuayResponse wraps http.Response in order to provide custom methods, e.g. GetJson
type QuayResponse struct {
	response *http.Response
}

func (r *QuayResponse) GetJson(obj interface{}) error {
	defer r.response.Body.Close()
	body, err := io.ReadAll(r.response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %s", err)
	}
	if err := json.Unmarshal(body, obj); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %s, got body: %s", err, string(body))
	}
	return nil
}

func (r *QuayResponse) GetStatusCode() int {
	return r.response.StatusCode
}

func (c *QuayClient) makeRequest(url, method string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.AuthToken))
	req.Header.Add("Content-Type", "application/json")
	return req, nil
}

func (c *QuayClient) doRequest(url, method string, body io.Reader) (*QuayResponse, error) {
	req, err := c.makeRequest(url, method, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to Do request: %w", err)
	}
	return &QuayResponse{response: resp}, nil
}

// CreateRepository creates a new Quay.io image repository.
func (c *QuayClient) CreateRepository(repositoryRequest RepositoryRequest) (*Repository, error) {
	url := fmt.Sprintf("%s/%s", c.url, "repository")

	b, err := json.Marshal(repositoryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal repository request data: %w", err)
	}

	resp, err := c.doRequest(url, http.MethodPost, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	statusCode := resp.GetStatusCode()

	data := &Repository{}
	if err := resp.GetJson(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response, got response code %d with error: %w", statusCode, err)
	}

	if statusCode != 200 {
		if statusCode == 402 {
			// Current plan doesn't allow private image repositories
			return nil, errors.New("payment required")
		} else if statusCode == 400 && data.ErrorMessage == "Repository already exists" {
			data.Name = repositoryRequest.Repository
		} else if data.ErrorMessage != "" {
			return data, errors.New(data.ErrorMessage)
		}
	}

	return data, nil
}

// DoesRepositoryExist checks if the specified image repository exists in quay.
func (c *QuayClient) DoesRepositoryExist(organization, imageRepository string) (bool, error) {
	url := fmt.Sprintf("%s/repository/%s/%s", c.url, organization, imageRepository)

	resp, err := c.doRequest(url, http.MethodGet, nil)
	if err != nil {
		return false, err
	}

	if resp.GetStatusCode() == 404 {
		return false, fmt.Errorf("repository %s does not exist in %s organization", imageRepository, organization)
	} else if resp.GetStatusCode() == 200 {
		return true, nil
	}

	data := &QuayError{}
	if err := resp.GetJson(data); err != nil {
		return false, err
	}
	if data.Error != "" {
		return false, errors.New(data.Error)
	}
	return false, errors.New(data.ErrorMessage)
}

// IsRepositoryPublic checks if the specified image repository has visibility public in quay.
func (c *QuayClient) IsRepositoryPublic(organization, imageRepository string) (bool, error) {
	url := fmt.Sprintf("%s/repository/%s/%s", c.url, organization, imageRepository)

	resp, err := c.doRequest(url, http.MethodGet, nil)
	if err != nil {
		return false, err
	}

	if resp.GetStatusCode() == 404 {
		return false, fmt.Errorf("repository %s does not exist in %s organization", imageRepository, organization)
	}

	if resp.GetStatusCode() == 200 {
		repo := &Repository{}
		if err := resp.GetJson(repo); err != nil {
			return false, err
		}
		if repo.IsPublic {
			return true, nil
		} else {
			return false, nil
		}
	}

	data := &QuayError{}
	if err := resp.GetJson(data); err != nil {
		return false, err
	}
	if data.Error != "" {
		return false, errors.New(data.Error)
	}
	return false, errors.New(data.ErrorMessage)
}

// DeleteRepository deletes specified image repository.
func (c *QuayClient) DeleteRepository(organization, imageRepository string) (bool, error) {
	url := fmt.Sprintf("%s/repository/%s/%s", c.url, organization, imageRepository)

	resp, err := c.doRequest(url, http.MethodDelete, nil)
	if err != nil {
		return false, err
	}

	statusCode := resp.GetStatusCode()

	if statusCode == 204 {
		return true, nil
	}
	if statusCode == 404 {
		return false, nil
	}

	data := &QuayError{}
	if err := resp.GetJson(data); err != nil {
		return false, err
	}
	if data.Error != "" {
		return false, errors.New(data.Error)
	}
	return false, errors.New(data.ErrorMessage)
}

// ChangeRepositoryVisibility makes existing repository public or private.
func (c *QuayClient) ChangeRepositoryVisibility(organization, imageRepositoryName, visibility string) error {
	if !(visibility == "public" || visibility == "private") {
		return fmt.Errorf("invalid repository visibility: %s", visibility)
	}

	// https://quay.io/api/v1/repository/user-org/repo-name/changevisibility
	url := fmt.Sprintf("%s/repository/%s/%s/changevisibility", c.url, organization, imageRepositoryName)
	requestData := strings.NewReader(fmt.Sprintf(`{"visibility": "%s"}`, visibility))

	resp, err := c.doRequest(url, http.MethodPost, requestData)
	if err != nil {
		return err
	}

	statusCode := resp.GetStatusCode()

	if statusCode == 200 {
		return nil
	}

	if statusCode == 402 {
		// Current plan doesn't allow private image repositories
		return errors.New("payment required")
	}

	data := &QuayError{}
	if err := resp.GetJson(data); err != nil {
		return err
	}
	if data.ErrorMessage != "" {
		return errors.New(data.ErrorMessage)
	}
	return errors.New(resp.response.Status)
}

func (c *QuayClient) GetRobotAccount(organization string, robotName string) (*RobotAccount, error) {
	url := fmt.Sprintf("%s/%s/%s/%s/%s", c.url, "organization", organization, "robots", robotName)

	resp, err := c.doRequest(url, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	data := &RobotAccount{}
	if err := resp.GetJson(data); err != nil {
		return nil, err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.New(data.Message)
	}

	return data, nil
}

// CreateRobotAccount creates a new Quay.io robot account in the organization.
func (c *QuayClient) CreateRobotAccount(organization string, robotName string) (*RobotAccount, error) {
	robotName, err := handleRobotName(robotName)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/%s/%s/%s", c.url, "organization", organization, "robots", robotName)
	payload := strings.NewReader(`{"description": "Robot account for AppStudio Component"}`)
	resp, err := c.doRequest(url, http.MethodPut, payload)
	if err != nil {
		return nil, err
	}

	statusCode := resp.GetStatusCode()

	//400 has special handling
	//see below
	if statusCode > 400 {
		message := "Failed to create robot account"
		data := &QuayError{}
		if err := resp.GetJson(data); err == nil {
			if data.ErrorMessage != "" {
				message = data.ErrorMessage
			} else {
				message = data.Error
			}
		}
		return nil, fmt.Errorf("failed to create robot account. Status code: %d, message: %s", statusCode, message)
	}

	data := &RobotAccount{}
	if err := resp.GetJson(data); err != nil {
		return nil, err
	}

	if statusCode == 400 && strings.Contains(data.Message, "Existing robot with name") {
		data, err = c.GetRobotAccount(organization, robotName)
		if err != nil {
			return nil, err
		}
	} else if statusCode == 400 {
		return nil, fmt.Errorf("failed to create robot account. Status code: %d, message: %s", statusCode, data.Message)
	}
	return data, nil
}

// DeleteRobotAccount deletes given Quay.io robot account in the organization.
func (c *QuayClient) DeleteRobotAccount(organization string, robotName string) (bool, error) {
	robotName, err := handleRobotName(robotName)
	if err != nil {
		return false, err
	}
	url := fmt.Sprintf("%s/organization/%s/robots/%s", c.url, organization, robotName)

	resp, err := c.doRequest(url, http.MethodDelete, nil)
	if err != nil {
		return false, err
	}

	if resp.GetStatusCode() == 204 {
		return true, nil
	}
	if resp.GetStatusCode() == 404 {
		return false, nil
	}

	data := &QuayError{}
	if err := resp.GetJson(data); err != nil {
		return false, err
	}
	if data.Error != "" {
		return false, errors.New(data.Error)
	}
	return false, errors.New(data.ErrorMessage)
}

// AddPermissionsForRepositoryToRobotAccount allows given robot account to access to the given repository.
// If isWrite is true, then pull and push permissions are added, otherwise - pull access only.
func (c *QuayClient) AddPermissionsForRepositoryToRobotAccount(organization, imageRepository, robotAccountName string, isWrite bool) error {
	var robotAccountFullName string
	if robotName, err := handleRobotName(robotAccountName); err == nil {
		robotAccountFullName = organization + "+" + robotName
	} else {
		return err
	}

	// url := "https://quay.io/api/v1/repository/redhat-appstudio/test-repo-using-api/permissions/user/redhat-appstudio+createdbysbose"
	url := fmt.Sprintf("%s/repository/%s/%s/permissions/user/%s", c.url, organization, imageRepository, robotAccountFullName)

	role := "read"
	if isWrite {
		role = "write"
	}
	body := strings.NewReader(fmt.Sprintf(`{"role": "%s"}`, role))
	resp, err := c.doRequest(url, http.MethodPut, body)
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != 200 {
		var message string
		data := &QuayError{}
		if err := resp.GetJson(data); err == nil {
			if data.ErrorMessage != "" {
				message = data.ErrorMessage
			} else {
				message = data.Error
			}
		}
		return fmt.Errorf("failed to add permissions to the robot account. Status code: %d, message: %s", resp.GetStatusCode(), message)
	}
	return nil
}

func (c *QuayClient) RegenerateRobotAccountToken(organization string, robotName string) (*RobotAccount, error) {
	url := fmt.Sprintf("%s/organization/%s/robots/%s/regenerate", c.url, organization, robotName)

	resp, err := c.doRequest(url, http.MethodPost, nil)
	if err != nil {
		return nil, err
	}

	data := &RobotAccount{}
	if err := resp.GetJson(data); err != nil {
		return nil, err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.New(data.Message)
	}

	return data, nil
}

// GetAllRepositories returns all repositories of the DEFAULT_QUAY_ORG organization (used in e2e-tests)
// Returns all repositories of the DEFAULT_QUAY_ORG organization (used in e2e-tests)
func (c *QuayClient) GetAllRepositories(organization string) ([]Repository, error) {
	url, _ := neturl.Parse(fmt.Sprintf("%s/repository", c.url))
	values := neturl.Values{}
	values.Add("last_modified", "true")
	values.Add("namespace", organization)
	url.RawQuery = values.Encode()

	req, err := c.makeRequest(url.String(), http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	type Response struct {
		Repositories []Repository `json:"repositories"`
		NextPage     string       `json:"next_page"`
	}
	var response Response
	var repositories []Repository

	for {
		res, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to Do request, error: %s", err)
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("error getting repositories, got status code %d", res.StatusCode)
		}

		resp := QuayResponse{response: res}
		if err := resp.GetJson(&response); err != nil {
			return nil, err
		}

		repositories = append(repositories, response.Repositories...)

		if response.NextPage == "" || values.Get("next_page") == response.NextPage {
			break
		}

		values.Set("next_page", response.NextPage)
		req.URL.RawQuery = values.Encode()
	}
	return repositories, nil
}

// GetAllRobotAccounts returns all robot accounts of the DEFAULT_QUAY_ORG organization (used in e2e-tests)
func (c *QuayClient) GetAllRobotAccounts(organization string) ([]RobotAccount, error) {
	url := fmt.Sprintf("%s/organization/%s/robots", c.url, organization)

	resp, err := c.doRequest(url, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	if resp.GetStatusCode() != 200 {
		return nil, fmt.Errorf("failed to get robot accounts. Status code: %d", resp.GetStatusCode())
	}

	type Response struct {
		Robots []RobotAccount
	}
	var response Response
	if err := resp.GetJson(&response); err != nil {
		return nil, err
	}
	return response.Robots, nil
}

// If robotName is in longform, return shortname
// e.g. `org+robot` will be changed to `robot`, `robot` will stay `robot`
func handleRobotName(robotName string) (string, error) {
	// Regexp from quay api `^([a-z0-9]+(?:[._-][a-z0-9]+)*)$` with one plus sign in the middle allowed (representing longname)
	r := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\+[a-z0-9]+(?:[._-][a-z0-9]+)*)?$`)
	robotName = strings.TrimSpace(robotName)
	if !r.MatchString(robotName) {
		return "", fmt.Errorf("robot name is invalid, must match `^([a-z0-9]+(?:[._-][a-z0-9]+)*)$` (one plus sign in the middle is also allowed)")
	}
	if strings.Contains(robotName, "+") {
		robotName = strings.Split(robotName, "+")[1]
	}
	return robotName, nil
}

func (c *QuayClient) GetTagsFromPage(organization, repository string, page int) ([]Tag, bool, error) {
	url := fmt.Sprintf("%s/repository/%s/%s/tag/?page=%d", c.url, organization, repository, page)

	resp, err := c.doRequest(url, http.MethodGet, nil)
	if err != nil {
		return nil, false, err
	}

	statusCode := resp.GetStatusCode()
	if statusCode != 200 {
		return nil, false, fmt.Errorf("failed to get repository tags. Status code: %d", statusCode)
	}

	var response struct {
		Tags          []Tag `json:"tags"`
		Page          int   `json:"page"`
		HasAdditional bool  `json:"has_additional"`
	}
	err = resp.GetJson(&response)
	if err != nil {
		return nil, false, err
	}
	return response.Tags, response.HasAdditional, nil
}

func (c *QuayClient) DeleteTag(organization, repository, tag string) (bool, error) {
	url := fmt.Sprintf("%s/repository/%s/%s/tag/%s", c.url, organization, repository, tag)

	resp, err := c.doRequest(url, http.MethodDelete, nil)
	if err != nil {
		return false, err
	}

	if resp.GetStatusCode() == 204 {
		return true, nil
	}
	if resp.GetStatusCode() == 404 {
		return false, nil
	}

	data := &QuayError{}
	if err := resp.GetJson(data); err != nil {
		return false, err
	}
	if data.Error != "" {
		return false, errors.New(data.Error)
	}
	return false, errors.New(data.ErrorMessage)
}
