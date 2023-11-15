package gitops

import (
	"fmt"
	"net/url"
	"strings"

	devfilePkg "github.com/devfile/library/v2/pkg/devfile"
	"github.com/devfile/library/v2/pkg/devfile/parser"
	"github.com/devfile/library/v2/pkg/devfile/parser/data"
)

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
