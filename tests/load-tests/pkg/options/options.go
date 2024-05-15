package options

import "time"


// Struct to hold command line options
type Opts struct {
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

func (o *Opts) ProcessOptions() error {
	parsed, err := time.ParseDuration(o.JourneyDuration)
	if err != nil {
		return err
	}

	o.JourneyUntil = time.Now().UTC().Add(parsed)
	return nil
}
