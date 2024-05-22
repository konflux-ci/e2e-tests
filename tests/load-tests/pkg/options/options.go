package options

import "encoding/json"
import "fmt"
import "os"
import "time"

// Struct to hold command line options
type Opts struct {
	ApplicationsCount             int
	BuildPipelineSelectorBundle   string
	ComponentRepoRevision         string
	ComponentRepoTemplate         bool
	ComponentRepoUrl              string
	ComponentsCount               int
	Concurrency                   int
	FailFast                      bool
	JourneyDuration               string
	JourneyUntil                  time.Time
	JourneyRepeats                int
	LogDebug                      bool
	LogTrace                      bool
	LogVerbose                    bool
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
	// Parse --journey-duration and populate JourneyUntil
	parsed, err := time.ParseDuration(o.JourneyDuration)
	if err != nil {
		return err
	}
	o.JourneyUntil = time.Now().UTC().Add(parsed)

	// Option '--purge-only' implies '--purge'
	if o.PurgeOnly {
		o.Purge = true
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
