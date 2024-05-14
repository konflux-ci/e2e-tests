package options


// Struct to hold command line options
type Opts struct {
	BuildPipelineSelectorBundle   string
	ComponentRepoRevision         string
	ComponentRepoTemplate         bool
	ComponentRepoUrl              string
	ComponentsCount               int
	Concurrency                   int
	FailFast                      bool
	LogDebug                      bool
	LogTrace                      bool
	LogVerbose                    bool
	NumberOfUsers                 int
	OutputDir                     string
	PipelineRequestConfigurePac   bool
	PipelineSkipInitialChecks     bool
	Purge                         bool
	QuayRepo                      string
	Stage                         bool
	TestScenarioGitURL            string
	TestScenarioPathInRepo        string
	TestScenarioRevision          string
	UsernamePrefix                string
	WaitIntegrationTestsPipelines bool
	WaitPipelines                 bool
}
