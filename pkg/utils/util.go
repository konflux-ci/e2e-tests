package utils

import (
	"bytes"
	"context"
	"crypto/tls"
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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"

	devfilePkg "github.com/devfile/library/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/devfile/library/v2/pkg/devfile/parser/data"
	"github.com/devfile/library/v2/pkg/util"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mitchellh/go-homedir"
	buildservice "github.com/redhat-appstudio/build-service/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/tektoncd/cli/pkg/bundle"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	"k8s.io/klog/v2"

	"sigs.k8s.io/yaml"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type FailedPipelineRunDetails struct {
	FailedTaskRunName   string
	PodName             string
	FailedContainerName string
}

type Options struct {
	ToolchainApiUrl string
	KeycloakUrl 	string
	OfflineToken 	string
}

//check options are valid or not
func CheckOptions(optionsArr []Options) (bool, error) {
	if len(optionsArr) == 0 {
		return false, nil
	}

	if len(optionsArr) > 1 {
		return true, fmt.Errorf("options array contains more than 1 object")
	}

	options := optionsArr[0]

	if options.ToolchainApiUrl == "" {
		return true, fmt.Errorf("ToolchainApiUrl field is empty")
	}

	if options.KeycloakUrl == "" {
		return true, fmt.Errorf("KeycloakUrl field is empty")
	}

	if options.OfflineToken == "" {
		return true, fmt.Errorf("OfflineToken field is empty")
	}

	return true, nil
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
	appDevfile, err := ParseDevfileModel(devfileStatus)
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
	appDevfile, err := ParseDevfileModel(devfileStatus)
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
		return "", fmt.Errorf("error obtaining oc token %s", err.Error())
	}
	return strings.TrimSuffix(string(tokenBytes), "\n"), nil
}

func GetFailedPipelineRunDetails(c crclient.Client, pipelineRun *v1beta1.PipelineRun) (*FailedPipelineRunDetails, error) {
	d := &FailedPipelineRunDetails{}
	for _, chr := range pipelineRun.Status.PipelineRunStatusFields.ChildReferences {
		taskRun := &v1beta1.TaskRun{}
		taskRunKey := types.NamespacedName{Namespace: pipelineRun.Namespace, Name: chr.Name}
		if err := c.Get(context.TODO(), taskRunKey, taskRun); err != nil {
			return nil, fmt.Errorf("failed to get details for PR %s: %+v", pipelineRun.GetName(), err)
		}
		for _, c := range taskRun.Status.Conditions {
			if c.Reason == "Failed" {
				d.FailedTaskRunName = taskRun.Name
				d.PodName = taskRun.Status.PodName
				for _, s := range taskRun.Status.TaskRunStatusFields.Steps {
					if s.Terminated.Reason == "Error" {
						d.FailedContainerName = s.ContainerName
						return d, nil
					}
				}
			}
		}
	}
	return d, nil
}

func GetGeneratedNamespace(name string) string {
	return name + "-" + util.GenerateRandomString(4)
}

func WaitUntilWithInterval(cond wait.ConditionFunc, interval time.Duration, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, cond)
}

func WaitUntil(cond wait.ConditionFunc, timeout time.Duration) error {
	return WaitUntilWithInterval(cond, time.Second, timeout)
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
		klog.Errorf("an error occurred: %s", err)

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

// contains checks if a string is present in a slice
func Contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func MergeMaps(m1, m2 map[string]string) map[string]string {
	resultMap := make(map[string]string)
	for k, v := range m1 {
		resultMap[k] = v
	}
	for k, v := range m2 {
		resultMap[k] = v
	}
	return resultMap
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

// ParseDevfileModel calls the devfile library's parse and returns the devfile data
func ParseDevfileModel(devfileModel string) (data.DevfileData, error) {
	// Retrieve the devfile from the body of the resource
	devfileBytes := []byte(devfileModel)
	parserArgs := parser.ParserArgs{
		Data: devfileBytes,
	}
	devfileObj, _, err := devfilePkg.ParseDevfileAndValidate(parserArgs)
	return devfileObj.Data, err
}

func HostIsAccessible(host string) bool {
	tc := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := http.Client{Transport: tc}
	res, err := client.Get(host)
	if err != nil || res.StatusCode > 499 {
		return false
	}
	return true
}

func HasPipelineRunSucceeded(pr *v1beta1.PipelineRun) bool {
	return pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue()
}

func HasPipelineRunFailed(pr *v1beta1.PipelineRun) bool {
	return pr.IsDone() && pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsFalse()
}

// Return a container logs from a given pod and namespace
func GetContainerLogs(ki kubernetes.Interface, podName, containerName, namespace string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}

	req := ki.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error in opening the stream: %v", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error in copying logs to buf, %v", err)
	}
	return buf.String(), nil
}
