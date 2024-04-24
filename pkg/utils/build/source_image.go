package build

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/openshift/library-go/pkg/image/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/redhat-appstudio/e2e-tests/pkg/clients/tekton"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

const (
	extraSourceSubDir     = "extra_src_dir"
	rpmSubDir             = "rpm_dir"
	srcTarFileRegex       = "extra-src-[0-9a-f]+.tar"
	shaValueRegex         = "[a-f0-9]{40}"
	tarGzFileRegex        = ".tar.gz$"
	gomodDependencySubDir = "deps/gomod/pkg/mod/cache/download/"
	pipDependencySubDir   = "deps/pip/"
)

func GetBinaryImage(pr *pipeline.PipelineRun) string {
	for _, p := range pr.Spec.Params {
		if p.Name == "output-image" {
			return p.Value.StringVal
		}
	}
	return ""
}

func IsSourceBuildEnabled(pr *pipeline.PipelineRun) bool {
	for _, p := range pr.Status.PipelineRunStatusFields.PipelineSpec.Params {
		if p.Name == "build-source-image" {
			if p.Default.StringVal == "true" {
				return true
			}
		}
	}
	return false
}

func IsHermeticBuildEnabled(pr *pipeline.PipelineRun) bool {
	for _, p := range pr.Spec.Params {
		if p.Name == "hermetic" {
			if p.Value.StringVal == "true" {
				return true
			}
		}
	}
	return false
}

func GetPrefetchValue(pr *pipeline.PipelineRun) string {
	for _, p := range pr.Spec.Params {
		if p.Name == "prefetch-input" {
			return p.Value.StringVal
		}
	}
	return ""
}

func IsSourceFilesExistsInSourceImage(srcImage string, gitUrl string, isHermetic bool, prefetchValue string) (bool, error) {
	//Extract the src image locally
	tmpDir, err := ExtractImage(srcImage)
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return false, err
	}

	// Check atleast one file present under extra_src_dir
	absExtraSourceDirPath := filepath.Join(tmpDir, extraSourceSubDir)
	fileNames, err := utils.GetFileNamesFromDir(absExtraSourceDirPath)
	if err != nil {
		return false, fmt.Errorf("error while getting files: %v", err)
	}
	if len(fileNames) == 0 {
		return false, fmt.Errorf("no tar file found in extra_src_dir, found files %v", fileNames)
	}

	// Get all the extra-src-*.tar files
	extraSrcTarFiles := utils.FilterSliceUsingPattern(srcTarFileRegex, fileNames)
	if len(extraSrcTarFiles) == 0 {
		return false, fmt.Errorf("no tar file found with pattern %s", srcTarFileRegex)
	}
	fmt.Printf("Files found with pattern %s: %v\n", srcTarFileRegex, extraSrcTarFiles)

	//Untar all the extra-src-[0-9]+.tar files
	for _, tarFile := range extraSrcTarFiles {
		absExtraSourceTarPath := filepath.Join(absExtraSourceDirPath, tarFile)
		err = utils.Untar(absExtraSourceDirPath, absExtraSourceTarPath)
		if err != nil {
			return false, fmt.Errorf("error while untaring %s: %v", tarFile, err)
		}
	}
	//Check if application source files exists
	_, err = IsAppSourceFilesExists(absExtraSourceDirPath, gitUrl)
	if err != nil {
		return false, err
	}
	// Check the pre-fetch dependency related files
	if isHermetic {
		_, err := IsPreFetchDependencysFilesExists(absExtraSourceDirPath, isHermetic, prefetchValue)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func IsAppSourceFilesExists(absExtraSourceDirPath string, gitUrl string) (bool, error) {
	//Get the file list from extra_src_dir
	fileNames, err := utils.GetFileNamesFromDir(absExtraSourceDirPath)
	if err != nil {
		return false, fmt.Errorf("error while getting files: %v", err)
	}

	//Get the component source with pattern <repo-name>-<git-sha>.tar.gz
	repoName := utils.GetRepoName(gitUrl)
	filePatternToFind := repoName + "-" + shaValueRegex + tarGzFileRegex
	resultFiles := utils.FilterSliceUsingPattern(filePatternToFind, fileNames)
	if len(resultFiles) == 0 {
		return false, fmt.Errorf("did not found the component source inside extra_src_dir, files found are: %v", fileNames)
	}
	sourceGzTarFileName := resultFiles[0]

	//Untar the <repo-name>-<git-sha>.tar.gz file
	err = utils.Untar(absExtraSourceDirPath, filepath.Join(absExtraSourceDirPath, sourceGzTarFileName))
	if err != nil {
		return false, fmt.Errorf("error while untaring %s: %v", sourceGzTarFileName, err)
	}

	//Get the file list from extra_src_dir/<repo-name>-<sha>
	sourceGzTarDirName := strings.TrimSuffix(sourceGzTarFileName, ".tar.gz")
	absSourceGzTarPath := filepath.Join(absExtraSourceDirPath, sourceGzTarDirName)
	fileNames, err = utils.GetFileNamesFromDir(absSourceGzTarPath)
	if err != nil {
		return false, fmt.Errorf("error while getting files from %s: %v", sourceGzTarDirName, err)
	}
	if len(fileNames) == 0 {
		return false, fmt.Errorf("no file found under extra_src_dir/<repo-name>-<git-sha>")
	}
	return true, nil
}

func IsPreFetchDependencysFilesExists(absExtraSourceDirPath string, isHermetic bool, prefetchValue string) (bool, error) {
	var absDependencyPath string
	if prefetchValue == "gomod" {
		fmt.Println("Checking go dependency files")
		absDependencyPath = filepath.Join(absExtraSourceDirPath, gomodDependencySubDir)
	} else if prefetchValue == "pip" {
		fmt.Println("Checking python dependency files")
		absDependencyPath = filepath.Join(absExtraSourceDirPath, pipDependencySubDir)
	} else {
		return false, fmt.Errorf("pre-fetch value type is not implemented")
	}

	fileNames, err := utils.GetFileNamesFromDir(absDependencyPath)
	if err != nil {
		return false, fmt.Errorf("error while getting files from %s: %v", absDependencyPath, err)
	}
	if len(fileNames) == 0 {
		return false, fmt.Errorf("no file found under extra_src_dir/deps/")
	}
	return true, nil
}

// readDockerfile reads Dockerfile dockerfile from repository repoURL.
// The Dockerfile is resolved by following the logic applied to the buildah task definition.
func readDockerfile(pathContext, dockerfile, repoURL, repoRevision string) ([]byte, error) {
	tempRepoDir, err := os.MkdirTemp("", "-test-repo")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempRepoDir)
	testRepo, err := git.PlainClone(tempRepoDir, false, &git.CloneOptions{URL: repoURL})
	if err != nil {
		return nil, err
	}

	// checkout to the revision. use go-git ResolveRevision since revision could be a branch, tag or commit hash
	commitHash, err := testRepo.ResolveRevision(plumbing.Revision(repoRevision))
	if err != nil {
		return nil, err
	}
	workTree, err := testRepo.Worktree()
	if err != nil {
		return nil, err
	}
	if err := workTree.Checkout(&git.CheckoutOptions{Hash: *commitHash}); err != nil {
		return nil, err
	}

	// check dockerfile in different paths
	var dockerfilePath string
	dockerfilePath = filepath.Join(tempRepoDir, dockerfile)
	if content, err := os.ReadFile(dockerfilePath); err == nil {
		return content, nil
	}
	dockerfilePath = filepath.Join(tempRepoDir, pathContext, dockerfile)
	if content, err := os.ReadFile(dockerfilePath); err == nil {
		return content, nil
	}
	if strings.HasPrefix(dockerfile, "https://") {
		if resp, err := http.Get(dockerfile); err == nil {
			defer resp.Body.Close()
			if body, err := io.ReadAll(resp.Body); err == nil {
				return body, err
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return nil, fmt.Errorf(
		fmt.Sprintf("resolveDockerfile: can't resolve Dockerfile from path context %s and dockerfile %s",
			pathContext, dockerfile),
	)
}

// ReadDockerfileUsedForBuild reads the Dockerfile and return its content.
func ReadDockerfileUsedForBuild(c client.Client, tektonController *tekton.TektonController, pr *pipeline.PipelineRun) ([]byte, error) {
	var paramDockerfileValue, paramPathContextValue string
	var paramUrlValue, paramRevisionValue string
	var err error
	getParam := tektonController.GetTaskRunParam

	if paramDockerfileValue, err = getParam(c, pr, "build-container", "DOCKERFILE"); err != nil {
		return nil, err
	}

	if paramPathContextValue, err = getParam(c, pr, "build-container", "CONTEXT"); err != nil {
		return nil, err
	}

	// get git-clone param url and revision
	if paramUrlValue, err = getParam(c, pr, "clone-repository", "url"); err != nil {
		return nil, err
	}

	if paramRevisionValue, err = getParam(c, pr, "clone-repository", "revision"); err != nil {
		return nil, err
	}

	dockerfileContent, err := readDockerfile(paramPathContextValue, paramDockerfileValue, paramUrlValue, paramRevisionValue)
	if err != nil {
		return nil, err
	}
	return dockerfileContent, nil
}

type SourceBuildResult struct {
	Status                  string `json:"status"`
	Message                 string `json:"message,omitempty"`
	DependenciesIncluded    bool   `json:"dependencies_included"`
	BaseImageSourceIncluded bool   `json:"base_image_source_included"`
	ImageUrl                string `json:"image_url"`
	ImageDigest             string `json:"image_digest"`
}

// ReadSourceBuildResult reads source-build task result BUILD_RESULT and returns the decoded data.
func ReadSourceBuildResult(c client.Client, tektonController *tekton.TektonController, pr *pipeline.PipelineRun) (*SourceBuildResult, error) {
	sourceBuildResult, err := tektonController.GetTaskRunResult(c, pr, "build-source-image", "BUILD_RESULT")
	if err != nil {
		return nil, err
	}
	var buildResult SourceBuildResult
	if err = json.Unmarshal([]byte(sourceBuildResult), &buildResult); err != nil {
		return nil, err
	}
	return &buildResult, nil
}

type Dockerfile struct {
	parsedContent *parser.Result
}

func ParseDockerfile(content []byte) (*Dockerfile, error) {
	parsedContent, err := parser.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	df := Dockerfile{
		parsedContent: parsedContent,
	}
	return &df, nil
}

func (d *Dockerfile) ParentImages() []string {
	parentImages := make([]string, 0, 5)
	for _, child := range d.parsedContent.AST.Children {
		if child.Value == "FROM" {
			parentImages = append(parentImages, child.Next.Value)
		}
	}
	return parentImages
}

func (d *Dockerfile) IsBuildFromScratch() bool {
	parentImages := d.ParentImages()
	return parentImages[len(parentImages)-1] == "scratch"
}

// convertImageToBuildahOutputForm converts an image pullspec to the corresponding form within
// BASE_IMAGES_DIGESTS output by buildah task.
func convertImageToBuildahOutputForm(imagePullspec string) (string, error) {
	ref, err := reference.Parse(imagePullspec)
	if err != nil {
		return "", fmt.Errorf("fail to parse image %s: %s", imagePullspec, err)
	}
	var tag string
	digest := ref.ID
	if digest == "" {
		val, err := FetchImageDigest(imagePullspec)
		if err != nil {
			return "", fmt.Errorf("fail to fetch image digest of %s: %s", imagePullspec, err)
		}
		digest = val

		tag = ref.Tag
		if tag == "" {
			tag = "latest"
		}
	} else {
		tag = "<none>"
	}
	digest = strings.TrimPrefix(digest, "sha256:")
	// image could have no namespace.
	converted := strings.TrimSuffix(filepath.Join(ref.Registry, ref.Namespace), "/")
	return fmt.Sprintf("%s/%s:%s@sha256:%s", converted, ref.Name, tag, digest), nil
}

// ConvertParentImagesToBaseImagesDigestsForm is a helper function for testing the order is matched
// between BASE_IMAGES_DIGESTS and parent images within Dockerfile.
// ConvertParentImagesToBaseImagesDigestsForm de-duplicates the images what buildah task does for BASE_IMAGES_DIGESTS.
func (d *Dockerfile) ConvertParentImagesToBaseImagesDigestsForm() ([]string, error) {
	convertedImagePullspecs := make([]string, 0, 5)
	seen := make(map[string]int)
	parentImages := d.ParentImages()
	for _, imagePullspec := range parentImages {
		if imagePullspec == "scratch" {
			continue
		}
		if _, exists := seen[imagePullspec]; exists {
			continue
		}
		seen[imagePullspec] = 1
		if converted, err := convertImageToBuildahOutputForm(imagePullspec); err == nil {
			convertedImagePullspecs = append(convertedImagePullspecs, converted)
		} else {
			return nil, err
		}
	}
	return convertedImagePullspecs, nil
}

func isRegistryAllowed(registry string) bool {
	// For the list of allowed registries, refer to source-build task definition.
	allowedRegistries := map[string]int{
		"registry.access.redhat.com": 1,
		"registry.redhat.io":         1,
	}
	_, exists := allowedRegistries[registry]
	return exists
}

func IsImagePulledFromAllowedRegistry(imagePullspec string) (bool, error) {
	if ref, err := reference.Parse(imagePullspec); err == nil {
		return isRegistryAllowed(ref.Registry), nil
	} else {
		return false, err
	}
}

func SourceBuildTaskRunLogsContain(
	tektonController *tekton.TektonController, pr *pipeline.PipelineRun, message string) (bool, error) {
	logs, err := tektonController.GetTaskRunLogs(pr.GetName(), "build-source-image", pr.GetNamespace())
	if err != nil {
		return false, err
	}
	for _, logMessage := range logs {
		if strings.Contains(logMessage, message) {
			return true, nil
		}
	}
	return false, nil
}

func ResolveSourceImage(image string) (string, error) {
	config, err := FetchImageConfig(image)
	if err != nil {
		return "", err
	}
	labels := config.Config.Labels
	var version, release string
	var exists bool
	if version, exists = labels["version"]; !exists {
		return "", fmt.Errorf("cannot find out version label from image config")
	}
	if release, exists = labels["release"]; !exists {
		return "", fmt.Errorf("cannot find out release label from image config")
	}
	ref, err := reference.Parse(image)
	if err != nil {
		return "", err
	}
	ref.ID = ""
	ref.Tag = fmt.Sprintf("%s-%s-source", version, release)
	return ref.Exact(), nil
}

func AllParentSourcesIncluded(parentSourceImage, builtSourceImage string) (bool, error) {
	parentConfig, err := FetchImageConfig(parentSourceImage)
	if err != nil {
		return false, err
	}
	builtConfig, err := FetchImageConfig(builtSourceImage)
	if err != nil {
		return false, err
	}
	srpmSha256Sums := make(map[string]int)
	var parts []string
	for _, history := range builtConfig.History {
		// Example history: #(nop) bsi version 0.2.0-dev adding artifact: 5f526f4
		parts = strings.Split(history.CreatedBy, " ")
		// The last part 5f526f4 is the checksum calculated from the file included in the generated blob.
		srpmSha256Sums[parts[len(parts)-1]] = 1
	}
	for _, history := range parentConfig.History {
		parts = strings.Split(history.CreatedBy, " ")
		if _, exists := srpmSha256Sums[parts[len(parts)-1]]; !exists {
			return false, nil
		}
	}
	return true, nil
}
