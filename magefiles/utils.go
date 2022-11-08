package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/klog/v2"

	"github.com/magefile/mage/sh"
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/kcp"
)

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
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return err
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

func commandExists(cmd string) bool {
	// check a command is available in the system.
	_, err := exec.LookPath(cmd)
	return err == nil
}

func initKCPController() (*kcp.SuiteController, error) {
	kubeClient, err := kubeCl.NewK8SClient()
	if err != nil {
		return &kcp.SuiteController{}, fmt.Errorf("error creating client-go %v", err)
	}

	kcpController, err := kcp.NewSuiteController(kubeClient)
	if err != nil {
		return &kcp.SuiteController{}, err
	}

	return kcpController, nil
}

func redHatSSOAuthentication() error {
	// Authenticate to Red Hat SSO using an offline token
	// curl \
	//  --silent \
	//  --header "Accept: application/json" \
	//  --header "Content-Type: application/x-www-form-urlencoded" \
	//  --data-urlencode "grant_type=refresh_token" \
	//  --data-urlencode "client_id=cloud-services" \
	//  --data-urlencode "refresh_token=${OFFLINE_TOKEN}" \
	//  "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"

	// Check offline_token is set: it's required for SSO
	OFFLINE_TOKEN := os.Getenv("OFFLINE_TOKEN")
	if OFFLINE_TOKEN == "" {
		return fmt.Errorf("OFFLINE_TOKEN is required for Red Hat SSO")
	}

	// Get current home dir
	HOMEDIR, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Build the auth request and execute
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", "cloud-services")
	data.Add("refresh_token", OFFLINE_TOKEN)
	encodedData := data.Encode()

	req, err := http.NewRequest("POST", "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token", strings.NewReader(encodedData))
	if err != nil {
		return fmt.Errorf("can't complete Red Hat SSO: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("can't complete Red Hat SSO: %v", err)
	}
	defer resp.Body.Close()

	// Get response data
	var response_data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response_data)
	if err != nil {
		return err
	}

	// Create oidc-login cache folder and files to save tokens
	oidc_dir := HOMEDIR + "/.kube/cache/oidc-login/"
	if _, err := os.Stat(oidc_dir); os.IsNotExist(err) {
		err = os.MkdirAll(oidc_dir, 0700)
		if err != nil {
			return err
		}
	}

	oidc_filename := oidc_dir + "de0b44c30948a686e739661da92d5a6bf9c6b1fb85ce4c37589e089ba03d0ec6"
	f, err := os.OpenFile(oidc_filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer f.Close()

	token_json := make(map[string]string)
	// id_token is the obtained access_token
	token_json["id_token"] = response_data["access_token"].(string)
	token_json["refresh_token"] = response_data["refresh_token"].(string)

	// convert payload to json and save it into cache file
	token_string, err := json.Marshal(token_json)
	if err != nil {
		return err
	}

	if _, err = f.Write(token_string); err != nil {
		return err
	}

	return nil
}

func useKCPEnviroment(kcpEnvironment string) error {

	switch kcpEnvironment {
	case "kcp-stable":
		if err := sh.Run("kubectl", "config", "use-context", "kcp-stable-root"); err != nil {
			return fmt.Errorf("cannot switch context to %s", kcpEnvironment)
		}
	case "kcp-unstable":
		if err := sh.Run("kubectl", "config", "use-context", "kcp-unstable-root"); err != nil {
			return fmt.Errorf("cannot switch context to %s", kcpEnvironment)
		}
	default:
		return fmt.Errorf("invalid environment. please specify stable or unstable")
	}

	return nil
}
