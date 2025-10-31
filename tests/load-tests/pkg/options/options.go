package options

import "encoding/json"
import "fmt"
import "os"
import "time"
import "strings"
import "sync"

// Struct to hold command line options
type Opts struct {
	ApplicationsCount                int
	BuildPipelineSelectorBundle      string
	ComponentContainerContext        string
	ComponentContainerFile           string
	ComponentRepoRevision            string
	ComponentRepoUrl                 string
	ComponentsCount                  int
	Concurrency                      int
	FailFast                         bool
	ForkTarget                       string
	JourneyDuration                  string
	JourneyRepeats                   int
	JourneyUntil                     time.Time
	JourneyReuseApplications         bool
	JourneyReuseComponents           bool
	LogDebug                         bool
	LogInfo                          bool
	LogTrace                         bool
	OutputDir                        string
	PipelineImagePullSecrets         []string
	PipelineMintmakerDisabled        bool
	PipelineRepoTemplating           bool
	PipelineRepoTemplatingSourceDir  string
	PipelineRepoTemplatingSource     string
	Purge                            bool
	PurgeOnly                        bool
	QuayRepo                         string
	ReleasePipelinePath              string
	ReleasePipelineRevision          string
	ReleasePipelineServiceAccount    string
	ReleasePipelineUrl               string
	ReleasePolicy                    string
	RunPrefix                        string
	SerializeComponentOnboarding     bool
	SerializeComponentOnboardingLock sync.Mutex
	Stage                            bool
	StartupDelay                     time.Duration
	StartupJitter                    time.Duration
	TestScenarioGitURL               string
	TestScenarioPathInRepo           string
	TestScenarioRevision             string
	WaitIntegrationTestsPipelines    bool
	WaitPipelines                    bool
	WaitRelease                      bool
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
		if !strings.HasSuffix(o.PipelineRepoTemplatingSourceDir, "/") {
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

	// If startup delay specified, make sure jitter is not bigger than 2 * delay
	if o.StartupDelay != 0 {
		if o.StartupJitter > o.StartupDelay * 2 {
			fmt.Print("Warning: Lowering startup jitter as it was bigger than delay\n")
			o.StartupJitter = o.StartupDelay * 2
		}
	}

	// If we are supposed to reuse components on additional journeys, we have to reuse applications
	if o.JourneyRepeats > 1 {
		if o.JourneyReuseComponents {
			if ! o.JourneyReuseApplications {
				fmt.Print("Warning: We are supposed to reuse components so will reuse applications as well\n")
				o.JourneyReuseApplications = true
			}
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
