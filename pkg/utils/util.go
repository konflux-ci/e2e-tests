package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type FailedPipelineRunDetails struct {
	FailedTaskRunName   string
	PodName             string
	FailedContainerName string
}

type Options struct {
	ToolchainApiUrl string
	KeycloakUrl     string
	OfflineToken    string
}

// check options are valid or not
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
		if err := c.Get(context.Background(), taskRunKey, taskRun); err != nil {
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
	return wait.PollUntilContextTimeout(context.Background(), interval, timeout, true, func(ctx context.Context) (bool, error) { return cond() })
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
	if obj, _, err = resolver.Get(context.Background(), kind, name); err != nil {
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
	for i := range ps.Spec.Selectors {
		s := &ps.Spec.Selectors[i]
		if s.Name == selectorName {
			return GetBundleRef(&s.PipelineRef), nil
		}
	}

	return "", fmt.Errorf("could not find %s pipeline bundle in build pipeline selector fetched from %s", selectorName, buildPipelineSelectorYamlURL)
}

// GetPipelineNameAndBundleRef returns the pipeline name and bundle reference from a pipelineRef
// https://tekton.dev/docs/pipelines/pipelineruns/#tekton-bundles
func GetPipelineNameAndBundleRef(pipelineRef *v1beta1.PipelineRef) (string, string) {
	var name string
	var bundleRef string

	// Prefer the v1 style
	if pipelineRef.Resolver != "" {
		for _, param := range pipelineRef.Params {
			switch param.Name {
			case "name":
				name = param.Value.StringVal
			case "bundle":
				bundleRef = param.Value.StringVal
			}
		}
	} else {
		// Support the v1beta1 style
		name = pipelineRef.Name
		bundleRef = pipelineRef.Bundle //nolint:all
	}

	return name, bundleRef
}

// GetBundleRef returns the bundle reference from a pipelineRef
func GetBundleRef(pipelineRef *v1beta1.PipelineRef) string {
	_, bundleRef := GetPipelineNameAndBundleRef(pipelineRef)
	return bundleRef
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
	podLogs, err := req.Stream(context.Background())
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

func ExtractGitRepositoryNameFromURL(url string) (name string) {
	repoName := url[strings.LastIndex(url, "/")+1:]
	return strings.TrimSuffix(repoName, ".git")
}

func GetGithubAppID() (int64, error) {
	appIDStr := GetEnv("E2E_PAC_GITHUB_APP_ID", constants.DefaultPaCGitHubAppID)

	id, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Build a kubeconfig string from an existing client config
func CreateKubeconfigFileForRestConfig(restConfig rest.Config) ([]byte, error) {
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["default-cluster"] = &clientcmdapi.Cluster{
		Server:                   restConfig.Host,
		CertificateAuthorityData: restConfig.CAData,
		InsecureSkipTLSVerify:    true,
	}
	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:  "default-cluster",
		AuthInfo: "default-user",
	}
	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	authinfos["default-user"] = &clientcmdapi.AuthInfo{
		Token: string(restConfig.BearerToken),
	}
	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authinfos,
	}
	kubeconfiString, err := clientcmd.Write(clientConfig)
	if err != nil {
		return []byte{}, nil
	}
	return kubeconfiString, nil
}

func GetFileNamesFromDir(dirPath string) ([]string, error) {
	var filesInDir []string
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %v", err)
	}
	for _, file := range files {
		filesInDir = append(filesInDir, file.Name())
	}
	return filesInDir, nil
}

func CheckFileExistsInDir(rootDir, filename string) (bool, error) {
	absFilePath := filepath.Join(rootDir, filename)
	_, err := os.Stat(absFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func Untar(dst string, tarPath string) error {

	tr, err := ReadTarFile(tarPath)
	if err != nil {
		return err
	}

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name) // nolint:gosec

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			err := CreateDir(target)
			if err != nil {
				return err
			}
		// if it's a file create it
		case tar.TypeReg:
			err = CreateFile(target, header, tr)
			if err != nil {
				return err
			}
		}
	}
}

func ReadTarFile(tarPath string) (*tar.Reader, error) {
	tarFile, err := os.Open(tarPath) // nolint:gosec
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(tarFile)
	if strings.HasSuffix(tarPath, ".gz") || strings.HasSuffix(tarPath, ".gzip") {
		gz, err := gzip.NewReader(tarFile)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		tr = tar.NewReader(gz)
	}
	return tr, nil
}

func CreateDir(target string) error {
	if _, err := os.Stat(target); err != nil {
		if err := os.MkdirAll(target, 0755); err != nil { // nolint:gosec
			return err
		}
	}
	return nil
}

func CreateFile(target string, header *tar.Header, tr *tar.Reader) error {
	f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)) // nolint:gosec
	if err != nil {
		return err
	}

	// copy over contents
	if _, err := io.Copy(f, tr); err != nil { // nolint:gosec
		return err
	}

	// manually close here after each file operation; defering would cause each file close
	// to wait until all operations have completed.
	return f.Close()
}

func GetRepoName(repoUrl string) string {
	return strings.Split(strings.TrimSuffix(repoUrl, ".git"), "/")[4]
}

func FilterSliceUsingPattern(pattern string, lString []string) []string {
	var results []string
	re := regexp.MustCompile(pattern)
	for _, str := range lString {
		if re.MatchString(str) {
			results = append(results, str)
		}
	}
	return results
}
