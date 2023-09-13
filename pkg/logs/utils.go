package logs

import (
	"fmt"
	"os"

	. "github.com/redhat-appstudio/e2e-tests/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/yaml"
)

// createArtifactDirectory creates directory for storing artifacts of current spec.
func createArtifactDirectory() (string, error) {
	wd, _ := os.Getwd()
	artifactDir := GetEnv("ARTIFACT_DIR", fmt.Sprintf("%s/tmp", wd))
	classname := ShortenStringAddHash(CurrentSpecReport())
	testLogsDir := fmt.Sprintf("%s/%s", artifactDir, classname)

	if err := os.MkdirAll(testLogsDir, os.ModePerm); err != nil {
		return "", err
	}

	return testLogsDir, nil

}

// StoreResourceYaml stores yaml of given resource.
func StoreResourceYaml(resource any, name string) error {
	resourceYaml, err := yaml.Marshal(resource)
	if err != nil {
		return fmt.Errorf("error getting resource yaml: %v", err)
	}

	resources := map[string][]byte{
		name + ".yaml": resourceYaml,
	}

	return StoreArtifacts(resources)
}

// StoreArtifacts stores given artifacts under artifact directory.
func StoreArtifacts(artifacts map[string][]byte) error {
	artifactsDirectory, err := createArtifactDirectory()
	if err != nil {
		return err
	}

	for artifact_name, artifact_value := range artifacts {
		filePath := fmt.Sprintf("%s/%s", artifactsDirectory, artifact_name)
		if err := os.WriteFile(filePath, []byte(artifact_value), 0644); err != nil {
			return err
		}
	}

	return nil
}
