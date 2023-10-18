package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	extraSourceSubDir     = "extra_src_dir"
	rpmSubDir             = "rpm_dir"
	srcTarFileRegex       = "extra-src-[0-9]+.tar"
	shaValueRegex         = "[a-f0-9]{40}"
	tarGzFileRegex        = ".tar.gz$"
	gomodDependencySubDir = "deps/gomod/pkg/mod/cache/download/"
	pipDependencySubDir   = "deps/pip/"
)

func GetBinaryImage(pr *v1beta1.PipelineRun) string {
	for _, p := range pr.Spec.Params {
		if p.Name == "output-image" {
			return p.Value.StringVal
		}
	}
	return ""
}

func IsSourceBuildEnabled(pr *v1beta1.PipelineRun) bool {
	for _, p := range pr.Status.PipelineRunStatusFields.PipelineSpec.Params {
		if p.Name == "build-source-image" {
			if p.Default.StringVal == "true" {
				return true
			}
		}
	}
	return false
}

func IsHermeticBuildEnabled(pr *v1beta1.PipelineRun) bool {
	for _, p := range pr.Spec.Params {
		if p.Name == "hermetic" {
			if p.Value.StringVal == "true" {
				return true
			}
		}
	}
	return false
}

func GetPrefetchValue(pr *v1beta1.PipelineRun) string {
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
		fmt.Println("No tar file found in extra_src_dir")
		return false, nil
	}

	// Get all the extra-src-[0-9]+.tar files
	extraSrcTarFiles := utils.FilterSliceUsingPattern(srcTarFileRegex, fileNames)
	fmt.Printf("Files found with pattern extra-src-[0-9]+.tar: %v\n", extraSrcTarFiles)
	if len(extraSrcTarFiles) == 0 {
		fmt.Println("No tar file found with pattern extra-src-[0-9]+.tar")
		return false, nil
	}

	//Untar all the extra-src-[0-9]+.tar files
	for _, tarFile := range extraSrcTarFiles {
		absExtraSourceTarPath := filepath.Join(absExtraSourceDirPath, tarFile)
		err = utils.Untar(absExtraSourceDirPath, absExtraSourceTarPath)
		if err != nil {
			return false, fmt.Errorf("error while untaring %s: %v", tarFile, err)
		}
	}

	//After untarring, get the file list from extra_src_dir
	fileNames, err = utils.GetFileNamesFromDir(absExtraSourceDirPath)
	if err != nil {
		return false, fmt.Errorf("error while getting files: %v", err)
	}

	//Get the component source with pattern <repo-name>-<git-sha>.tar.gz
	repoName := utils.GetRepoName(gitUrl)
	filePatternToFind := repoName + "-" + shaValueRegex + tarGzFileRegex
	resultFiles := utils.FilterSliceUsingPattern(filePatternToFind, fileNames)
	if len(resultFiles) == 0 {
		fmt.Printf("did not found the component source inside extra_src_dir, files found are: %v\n", fileNames)
		return false, nil
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
		fmt.Println("No file found under extra_src_dir/<repo-name>-<git-sha>")
		return false, nil
	}

	//Check the pre-fetch dependency related files
	if isHermetic {
		if prefetchValue == "gomod" {
			fmt.Println("Checking go dependency files")
			absGoDependencyPath := filepath.Join(absExtraSourceDirPath, gomodDependencySubDir)
			fileNames, err = utils.GetFileNamesFromDir(absGoDependencyPath)
			if err != nil {
				return false, fmt.Errorf("error while getting files from %s: %v", absGoDependencyPath, err)
			}
			if len(fileNames) == 0 {
				fmt.Println("No file found under extra_src_dir/deps/gomod/pkg/mod/cache/download/")
				return false, nil
			}
		} else if prefetchValue == "pip" {
			fmt.Println("Checking python dependency files")
			absPythonDependencyPath := filepath.Join(absExtraSourceDirPath, pipDependencySubDir)
			fileNames, err = utils.GetFileNamesFromDir(absPythonDependencyPath)
			if err != nil {
				return false, fmt.Errorf("error while getting files from %s: %v", absPythonDependencyPath, err)
			}
			if len(fileNames) == 0 {
				fmt.Println("No file found under extra_src_dir/deps/pip")
				return false, nil
			}
		} else {
			fmt.Println("pre-fetch value type is not implemented")
			return false, nil
		}
	}

	return true, nil
}
