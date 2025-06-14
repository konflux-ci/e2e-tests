package options

import "encoding/json"
import "fmt"
import "os"
import "time"
import "strings"

// Struct to hold command line options
type Opts struct {
	ApplicationsCount               int
	BuildPipelineSelectorBundle     string
	ComponentContainerContext       string
	ComponentContainerFile          string
	ComponentRepoRevision           string
	ComponentRepoUrl                string
	ComponentsCount                 int
	Concurrency                     int
	FailFast                        bool
	ForkTarget                      string
	JourneyDuration                 string
	JourneyRepeats                  int
	JourneyUntil                    time.Time
	LogDebug                        bool
	LogTrace                        bool
	LogInfo                         bool
	OutputDir                       string
	PipelineMintmakerDisabled       bool
	PipelineRepoTemplating          bool
	PipelineRepoTemplatingSource    string
	PipelineRepoTemplatingSourceDir string
	PipelineImagePullSecrets        []string
	Purge                           bool
	PurgeOnly                       bool
	QuayRepo                        string
	Stage                           bool
	TestScenarioGitURL              string
	TestScenarioPathInRepo          string
	TestScenarioRevision            string
	UsernamePrefix                  string
	WaitIntegrationTestsPipelines   bool
	WaitPipelines                   bool
}

// Pre-process load-test options before running the test
func (o *Opts) ProcessOptions() error {
	// Parse '--journey-duration' and populate JourneyUntil
	parsed, err := time.ParseDuration(o.JourneyDuration)
	if err != nil {
		return err
	}
	o.JourneyUntil = time.Now().UTC().Add(parsed)

	// Option '--purge-only' implies '--purge'
	if o.PurgeOnly {
		o.Purge = true
	}

	// If we are templating, set default values for relevant options if empty
	if o.PipelineRepoTemplating {
		if o.PipelineRepoTemplatingSource == "" {
			o.PipelineRepoTemplatingSource = o.ComponentRepoUrl
		}
		if o.PipelineRepoTemplatingSourceDir == "" {
			o.PipelineRepoTemplatingSourceDir = ".template/"
		}
		if strings.HasSuffix(o.PipelineRepoTemplatingSourceDir, "/") != true {
			o.PipelineRepoTemplatingSourceDir = o.PipelineRepoTemplatingSourceDir + "/"
		}
	}

	// If forking target directory was empty, use MY_GITHUB_ORG env variable
	if o.ForkTarget == "" {
		o.ForkTarget = os.Getenv("MY_GITHUB_ORG")
		if o.ForkTarget == "" {
			return fmt.Errorf("Was not able to get fork target")
		}
	}

	// Convert options struct to pretty JSON
	jsonOptions, err2 := json.MarshalIndent(o, "", "  ")
	if err2 != nil {
		return fmt.Errorf("Error marshalling options: %v", err2)
	}

	// Dump options to JSON file in putput directory for refference
	err3 := os.WriteFile(o.OutputDir + "/load-test-options.json", jsonOptions, 0600)
	if err3 != nil {
		return fmt.Errorf("Error writing to file: %v", err3)
	}

	return nil
}
