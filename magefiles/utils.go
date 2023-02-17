package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"sigs.k8s.io/yaml"

	sprig "github.com/go-task/slim-sprig"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/magefile/mage/sh"
	"github.com/mitchellh/go-homedir"
	routev1 "github.com/openshift/api/route/v1"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	client "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/tektoncd/cli/pkg/bundle"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
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
	klog.Infof("response status code: '%d'", res.StatusCode)

	if err != nil {
		return fmt.Errorf("error when sending request to '%s': %+v", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)

	if err != nil {
		return fmt.Errorf("error when reading the response body from URL '%s': %+v", url, err)
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

func getRouteHost(name, namespace string) (string, error) {
	kubeClient, err := client.NewK8SClient()
	if err != nil {
		return "", err
	}
	route := &routev1.Route{}
	err = kubeClient.KubeRest().Get(context.TODO(), types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, route)
	if err != nil {
		return "", err
	}
	return route.Spec.Host, nil
}

func getKeycloakUrl() (string, error) {
	keycloakHost, err := getRouteHost("keycloak", "dev-sso")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s", keycloakHost), nil
}

func getToolchainApiUrl() (string, error) {
	toolchainHost, err := getRouteHost("api", "toolchain-host-operator")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s:443", toolchainHost), nil
}

func getKeycloakToken(keycloakUrl, username, password, client_id string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	data := url.Values{}
	data.Set("client_id", client_id)
	data.Set("password", username)
	data.Set("username", password)
	data.Set("grant_type", "password")

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/auth/realms/testrealm/protocol/openid-connect/token", keycloakUrl), bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	fmt.Printf("Response: %+v\n", resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	token := struct {
		AccessToken string `json:"access_token"`
	}{}

	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// getDefaultPipelineBundleRef gets the specific Tekton pipeline bundle reference from a Build pipeline selector
// (in a YAML format) from a URL specified in the parameter
func getDefaultPipelineBundleRef(buildPipelineSelectorYamlURL, selectorName string) (string, error) {
	res, err := http.Get(buildPipelineSelectorYamlURL)
	if err != nil {
		return "", fmt.Errorf("failed to get a build pipeline selector from url %s: %v", buildPipelineSelectorYamlURL, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read the body response of a build pipeline selector: %v", err)
	}
	ps := &buildservice.BuildPipelineSelector{}
	if err = yaml.Unmarshal(body, ps); err != nil {
		return "", fmt.Errorf("failed to unmarshal build pipeline selector: %v", err)
	}

	for _, s := range ps.Spec.Selectors {
		if s.Name == selectorName {
			return s.PipelineRef.Bundle, nil
		}
	}

	return "", fmt.Errorf("could not find %s pipeline bundle in build pipeline selector fetched from %s", selectorName, buildPipelineSelectorYamlURL)
}

// createDockerConfigFile takes base64 encoded dockerconfig.json and saves it locally (/<home-directory/.docker/config.json)
func createDockerConfigFile(base64EncodedString string) error {
	var rawRegistryCreds []byte
	var homeDir string
	var err error

	if rawRegistryCreds, err = base64.StdEncoding.DecodeString(base64EncodedString); err != nil {
		return fmt.Errorf("unable to decode container registry credentials: %v", err)
	}
	if homeDir, err = homedir.Dir(); err != nil {
		return fmt.Errorf("unable to locate home directory: %v", err)
	}
	if err = os.MkdirAll(homeDir+"/.docker", 0775); err != nil {
		return fmt.Errorf("failed to create '.docker' config directory: %v", err)
	}
	if err = os.WriteFile(homeDir+"/.docker/config.json", rawRegistryCreds, 0644); err != nil {
		return fmt.Errorf("failed to create a docker config file: %v", err)
	}

	return nil
}

// extractTektonObjectFromBundle extracts specified Tekton object from specified bundle reference
func extractTektonObjectFromBundle(bundleRef, kind, name string) (runtime.Object, error) {
	var obj runtime.Object
	var err error

	resolver := oci.NewResolver(bundleRef, authn.DefaultKeychain)
	if obj, _, err = resolver.Get(context.TODO(), kind, name); err != nil {
		return nil, fmt.Errorf("failed to fetch the tekton object %s with name %s: %v", kind, name, err)
	}
	return obj, nil
}

// buildAndPushTektonBundle builds a Tekton bundle from YAML and pushes to remote container registry
func buildAndPushTektonBundle(YamlContent []byte, ref name.Reference, remoteOption remoteimg.Option) error {
	img, err := bundle.BuildTektonBundle([]string{string(YamlContent)}, os.Stdout)
	if err != nil {
		return fmt.Errorf("error when building a bundle %s: %v", ref.String(), err)
	}

	outDigest, err := bundle.Write(img, ref, remoteOption)
	if err != nil {
		return fmt.Errorf("error when pushing a bundle %s to a container image registry repo: %v", ref.String(), err)
	}
	klog.Infof("image digest for a new tekton bundle %s: %+v", ref.String(), outDigest)

	return nil
}
