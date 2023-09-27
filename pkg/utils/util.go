package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/devfile/library/pkg/util"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mitchellh/go-homedir"
	"github.com/redhat-appstudio/application-service/pkg/devfile"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/tektoncd/cli/pkg/bundle"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	"k8s.io/klog/v2"

	"sigs.k8s.io/yaml"
)

type FailedPipelineRunDetails struct {
	FailedTaskRunName   string
	PodName             string
	FailedContainerName string
}

// CheckIfEnvironmentExists return true/false if the environment variable exists
func CheckIfEnvironmentExists(env string) bool {
	_, exist := os.LookupEnv(env)
	return exist
}

// Retrieve an environment variable. If will not exists will be used a default value
func GetEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// Retrieve an environment variable. If it doesn't exist, returns result of a call to `defaultFunc`.
func GetEnvOrFunc(key string, defaultFunc func() (string, error)) (string, error) {
	if val := os.Getenv(key); val != "" {
		return val, nil
	}
	return defaultFunc()
}

/*
Right now DevFile status in HAS is a string:
metadata:

	attributes:
		appModelRepository.url: https://github.com/redhat-appstudio-qe/pet-clinic-application-service-establish-danger
		gitOpsRepository.url: https://github.com/redhat-appstudio-qe/pet-clinic-application-service-establish-danger
	name: pet-clinic
	schemaVersion: 2.1.0

The ObtainGitUrlFromDevfile extract from the string the git url associated with a application
*/
func ObtainGitOpsRepositoryName(devfileStatus string) string {
	appDevfile, err := devfile.ParseDevfileModel(devfileStatus)
	if err != nil {
		err = fmt.Errorf("error parsing devfile: %v", err)
	}
	// Get the devfile attributes from the parsed object
	devfileAttributes := appDevfile.GetMetadata().Attributes
	gitOpsRepository := devfileAttributes.GetString("gitOpsRepository.url", &err)
	parseUrl, err := url.Parse(gitOpsRepository)
	if err != nil {
		err = fmt.Errorf("fatal: %v", err)
	}
	repoParsed := strings.Split(parseUrl.Path, "/")

	return repoParsed[len(repoParsed)-1]
}

func ObtainGitOpsRepositoryUrl(devfileStatus string) string {
	appDevfile, err := devfile.ParseDevfileModel(devfileStatus)
	if err != nil {
		err = fmt.Errorf("error parsing devfile: %v", err)
	}
	// Get the devfile attributes from the parsed object
	devfileAttributes := appDevfile.GetMetadata().Attributes
	gitOpsRepository := devfileAttributes.GetString("gitOpsRepository.url", &err)

	return gitOpsRepository
}

func GetQuayIOOrganization() string {
	return GetEnv(constants.QUAY_E2E_ORGANIZATION_ENV, "redhat-appstudio-qe")
}

func GetDockerConfigJson() string {
	return GetEnv(constants.DOCKER_CONFIG_JSON, "")
}

func IsPrivateHostname(url string) bool {
	// https://www.ibm.com/docs/en/networkmanager/4.2.0?topic=translation-private-address-ranges
	privateIPAddressPrefixes := []string{"10.", "172.1", "172.2", "172.3", "192.168"}
	addr, err := net.LookupIP(url)
	if err != nil {
		klog.Infof("Unknown host: %v", err)
		return true
	}

	ip := addr[0]
	for _, ipPrefix := range privateIPAddressPrefixes {
		if strings.HasPrefix(ip.String(), ipPrefix) {
			return true
		}
	}
	return false
}

func GetOpenshiftToken() (token string, err error) {
	// Get the token for the current openshift user
	tokenBytes, err := exec.Command("oc", "whoami", "--show-token").Output()
	if err != nil {
		return "", fmt.Errorf("Error obtaining oc token %s", err.Error())
	}
	return strings.TrimSuffix(string(tokenBytes), "\n"), nil
}

func GetFailedPipelineRunDetails(pipelineRun *v1beta1.PipelineRun) *FailedPipelineRunDetails {
	d := &FailedPipelineRunDetails{}
	for trName, trs := range pipelineRun.Status.PipelineRunStatusFields.TaskRuns {
		for _, c := range trs.Status.Conditions {
			if c.Reason == "Failed" {
				d.FailedTaskRunName = trName
				d.PodName = trs.Status.PodName
				for _, s := range trs.Status.TaskRunStatusFields.Steps {
					if s.Terminated.Reason == "Error" {
						d.FailedContainerName = s.ContainerName
						return d
					}
				}
			}
		}
	}
	return d
}

func GetGeneratedNamespace(name string) string {
	return name + "-" + util.GenerateRandomString(4)
}

func WaitUntil(cond wait.ConditionFunc, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, cond)
}

func ExecuteCommandInASpecificDirectory(command string, args []string, directory string) error {
	cmd := exec.Command(command, args...) // nolint:gosec
	cmd.Dir = directory

	stdin, err := cmd.StdinPipe()

	if err != nil {
		return err
	}
	defer stdin.Close() // the doc says subProcess.Wait will close it, but I'm not sure, so I kept this line

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		klog.Errorf("an error ocurred: %s", err)

		return err
	}

	_, _ = io.WriteString(stdin, "4\n")

	if err := cmd.Wait(); err != nil {
		return err
	}

	return err
}

func ToPrettyJSONString(v interface{}) string {
	s, _ := json.MarshalIndent(v, "", "  ")
	return string(s)
}

// GetAdditionalInfo adds information regarding the application name and namespace of the test
func GetAdditionalInfo(applicationName, namespace string) string {
	return fmt.Sprintf("(application: %s, namespace: %s)", applicationName, namespace)
}

// CreateDockerConfigFile takes base64 encoded dockerconfig.json and saves it locally (/<home-directory/.docker/config.json)
func CreateDockerConfigFile(base64EncodedString string) error {
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

// ExtractTektonObjectFromBundle extracts specified Tekton object from specified bundle reference
func ExtractTektonObjectFromBundle(bundleRef, kind, name string) (runtime.Object, error) {
	var obj runtime.Object
	var err error

	resolver := oci.NewResolver(bundleRef, authn.DefaultKeychain)
	if obj, _, err = resolver.Get(context.TODO(), kind, name); err != nil {
		return nil, fmt.Errorf("failed to fetch the tekton object %s with name %s: %v", kind, name, err)
	}
	return obj, nil
}

// BuildAndPushTektonBundle builds a Tekton bundle from YAML and pushes to remote container registry
func BuildAndPushTektonBundle(YamlContent []byte, ref name.Reference, remoteOption remoteimg.Option) error {
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

// GetDefaultPipelineBundleRef gets the specific Tekton pipeline bundle reference from a Build pipeline selector
// (in a YAML format) from a URL specified in the parameter
func GetDefaultPipelineBundleRef(buildPipelineSelectorYamlURL, selectorName string) (string, error) {
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
