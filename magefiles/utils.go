package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"k8s.io/klog/v2"

	sprig "github.com/go-task/slim-sprig"
	"github.com/magefile/mage/sh"
	"github.com/redhat-appstudio/image-controller/pkg/quay"
)

const quayPrefixesToDeleteRegexp = "e2e-demos|has-e2e"

func getRemoteAndBranchNameFromPRLink(url string) (remote, branchName string, err error) {
	ghRes := &GithubPRInfo{}
	if err := sendHttpRequestAndParseResponse(url, "GET", ghRes); err != nil {
		return "", "", err
	}

	if ghRes.Head.Label == "" {
		return "", "", fmt.Errorf("failed to get an information about the remote and branch name from PR %s", url)
	}

	split := strings.Split(ghRes.Head.Label, ":")
	remote, branchName = split[0], split[1]

	return remote, branchName, nil
}

func gitCheckoutRemoteBranch(remoteName, branchName string) error {
	var git = sh.RunCmd("git")
	for _, arg := range [][]string{
		{"remote", "add", remoteName, fmt.Sprintf("https://github.com/%s/e2e-tests.git", remoteName)},
		{"fetch", remoteName},
		{"checkout", branchName},
	} {
		if err := git(arg...); err != nil {
			return fmt.Errorf("error when checkout out remote branch %s from remote %s: %v", branchName, remoteName, err)
		}
	}
	return nil
}

func sendHttpRequestAndParseResponse(url, method string, v interface{}) error {
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error when sending request to '%s': %+v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error when reading the response body from URL '%s': %+v", url, err)
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("unexpected status code: %d, response body: %s", res.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("error when unmarshalling the response body from URL '%s': %+v", url, err)
	}

	return nil
}

func retry(f func() error, attempts int, delay time.Duration) error {
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			klog.Infof("got an error: %+v - will retry in %v", err, delay)
			time.Sleep(delay)
		}
		err = f()
		if err != nil {
			continue
		} else {
			return nil
		}
	}
	return fmt.Errorf("reached maximum number of attempts (%d). error: %+v", attempts, err)
}

func goFmt(path string) error {
	err := sh.RunV("go", "fmt", path)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Could not fmt:\n%s\n", path), err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func renderTemplate(destination, templatePath string, templateData interface{}, appendDestination bool) error {

	var templateText string
	var f *os.File
	var err error

	/* This decision logic feels a little clunky cause initially I wanted to
	to have this func create the new file and render the template into the new
	file. But with the updating the pkg/framework/describe.go use case
	I wanted to reuse leveraging the txt/template package rather than
	rendering/updating using strings/regex.
	*/
	if appendDestination {

		f, err = os.OpenFile(destination, os.O_APPEND|os.O_WRONLY, 0664)
		if err != nil {
			klog.Infof("Failed to open file: %v", err)
		}
	} else {

		if fileExists(destination) {
			return fmt.Errorf("%s already exists", destination)
		}
		f, err = os.Create(destination)
		if err != nil {
			klog.Infof("Failed to create file: %v", err)
		}
	}

	defer f.Close()

	tpl, err := os.ReadFile(templatePath)
	if err != nil {
		klog.Infof("error reading file: %v", err)

	}
	var tmplText = string(tpl)
	templateText = fmt.Sprintf("\n%s", tmplText)
	specTemplate, err := template.New("spec").Funcs(sprig.TxtFuncMap()).Parse(templateText)
	if err != nil {
		klog.Infof("error parsing template file: %v", err)

	}

	err = specTemplate.Execute(f, templateData)
	if err != nil {
		klog.Infof("error rendering template file: %v", err)
	}

	return nil
}

func cleanupQuayReposAndRobots(quayService quay.QuayService, quayOrg string) error {
	r, err := regexp.Compile(fmt.Sprintf(`^(%s)`, quayPrefixesToDeleteRegexp))
	if err != nil {
		return err
	}

	repos, err := quayService.GetAllRepositories(quayOrg)
	if err != nil {
		return err
	}

	// Key is the repo name without slashes which is the same as robot name
	// Value is the repo name with slashes
	reposMap := make(map[string]string)

	for _, repo := range repos {
		if r.MatchString(repo.Name) {
			sanitizedRepoName := strings.ReplaceAll(repo.Name, "/", "") // repo name without slashes
			reposMap[sanitizedRepoName] = repo.Name
		}
	}

	robots, err := quayService.GetAllRobotAccounts(quayOrg)
	if err != nil {
		return err
	}

	r, err = regexp.Compile(fmt.Sprintf(`^%s\+(%s)`, quayOrg, quayPrefixesToDeleteRegexp))
	if err != nil {
		return err
	}

	const timeFormat = "Mon, 02 Jan 2006 15:04:05 -0700"

	// Deletes robots and their repos with correct prefix if created more than 24 hours ago
	for _, robot := range robots {
		parsed, err := time.Parse(timeFormat, robot.Created)
		if err != nil {
			return err
		}

		// If robot.Name has correct prefix and was created more than 24 hours ago
		if r.MatchString(robot.Name) && time.Since(parsed) > 24*time.Hour {
			// Robot name without the name of org which is the same as previous sanitizedRepoName
			// redhat-appstudio-qe+e2e-demos turns to e2e-demos
			splitRobotName := strings.Split(robot.Name, "+")
			if len(splitRobotName) != 2 {
				return fmt.Errorf("failed to split robot name into 2 parts, got %d parts", len(splitRobotName))
			}
			sanitizedRepoName := splitRobotName[1] // Same as robot shortname
			if repo, exists := reposMap[sanitizedRepoName]; exists {
				deleted, err := quayService.DeleteRepository(quayOrg, repo)
				if err != nil {
					return fmt.Errorf("failed to delete repository %s, error: %s", repo, err)
				}
				if !deleted {
					fmt.Printf("repository %s has already been deleted, skipping\n", repo)
				}
			}
			// DeleteRobotAccount uses robot shortname, so e2e-demos instead of redhat-appstudio-qe+e2e-demos
			deleted, err := quayService.DeleteRobotAccount(quayOrg, splitRobotName[1])
			if err != nil {
				return fmt.Errorf("failed to delete robot account %s, error: %s", robot.Name, err)
			}
			if !deleted {
				fmt.Printf("robot account %s has already been deleted, skipping\n", robot.Name)
			}
		}
	}
	return nil
}

func cleanupQuayTags(quayService quay.QuayService, organization, repository string) error {
	workerCount := 10
	var wg sync.WaitGroup

	var allTags []quay.Tag
	var errors []error

	page := 1
	for {
		tags, hasAdditional, err := quayService.GetTagsFromPage(organization, repository, page)
		page++
		if err != nil {
			errors = append(errors, fmt.Errorf("error getting tags of `%s` repository of `%s` organization on page `%d`, error: %s", repository, organization, page, err))
			continue
		}
		allTags = append(allTags, tags...)
		if !hasAdditional {
			break
		}
	}

	wg.Add(workerCount)

	var errorsMutex sync.Mutex
	for i := 0; i < workerCount; i++ {
		go func(startIdx int, allTags []quay.Tag, errors []error, errorsMutex *sync.Mutex, wg *sync.WaitGroup) {
			defer wg.Done()
			for idx := startIdx; idx < len(allTags); idx += workerCount {
				tag := allTags[idx]
				if time.Unix(tag.StartTS, 0).Before(time.Now().AddDate(0, 0, -7)) {
					deleted, err := quayService.DeleteTag(organization, repository, tag.Name)
					if err != nil {
						errorsMutex.Lock()
						errors = append(errors, fmt.Errorf("error during deletion of tag `%s` in repository `%s` of organization `%s`, error: `%s`\n", tag.Name, repository, organization, err))
						errorsMutex.Unlock()
					} else if !deleted {
						fmt.Printf("tag `%s` in repository `%s` of organization `%s` was not deleted\n", tag.Name, repository, organization)
					}
				}
			}
		}(i, allTags, errors, &errorsMutex, &wg)
	}

	wg.Wait()

	if len(errors) == 0 {
		return nil
	}

	var errBuilder strings.Builder
	for _, err := range errors {
		errBuilder.WriteString(fmt.Sprintf("%s\n", err))
	}
	return fmt.Errorf("encountered errors during CleanupQuayTags: %s", errBuilder.String())
}
