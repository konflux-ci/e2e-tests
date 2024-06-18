package options

import "encoding/json"
import "fmt"
import "os"
import "time"

// Struct to hold command line options
type Opts struct {
	ApplicationsCount             int
	BuildPipelineSelectorBundle   string
	ComponentContainerContext     string
	ComponentContainerFile        string
	ComponentRepoRevision         string
	ComponentRepoUrl              string
	ComponentsCount               int
	Concurrency                   int
	FailFast                      bool
	JourneyDuration               string
	JourneyRepeats                int
	JourneyUntil                  time.Time
	LogDebug                      bool
	LogTrace                      bool
	LogInfo                       bool
	MultiarchWorkflow             bool
	OutputDir                     string
	PipelineRequestConfigurePac   bool
	PipelineSkipInitialChecks     bool
	Purge                         bool
	PurgeOnly                     bool
	QuayRepo                      string
	Stage                         bool
	TestScenarioGitURL            string
	TestScenarioPathInRepo        string
	TestScenarioRevision          string
	UsernamePrefix                string
	WaitIntegrationTestsPipelines bool
	WaitPipelines                 bool
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

	// Option '--multiarch-workflow' implies '--pipeline-request-configure-pac'
	if o.MultiarchWorkflow {
		o.PipelineRequestConfigurePac = true
	}

	// Option '--pipeline-request-configure-pac' implies '--pipeline-skip-initial-checks' is false (not present)
	if o.PipelineRequestConfigurePac {
		o.PipelineSkipInitialChecks = false
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
