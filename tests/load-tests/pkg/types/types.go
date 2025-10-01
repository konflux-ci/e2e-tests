package types

import "sync"
import "time"

import framework "github.com/konflux-ci/e2e-tests/pkg/framework"
import loadtestutils "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/loadtestutils"
import options "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/options"

// Struct to hold user journey thread data
type PerUserContext struct {
	PerUserWG              *sync.WaitGroup
	UserIndex              int
	StartupPause           time.Duration
	JourneyRepeatsCounter  int
	Opts                   *options.Opts
	StageUsers             *[]loadtestutils.User
	Framework              *framework.Framework
	Username               string
	Namespace              string
	ComponentRepoUrl       string // overrides same value from Opts, needed when templating repos
	PerApplicationContexts []*PerApplicationContext
}

// Struct to hold data for thread to process each application
type PerApplicationContext struct {
	PerApplicationWG            *sync.WaitGroup
	ApplicationIndex            int
	StartupPause                time.Duration
	Framework                   *framework.Framework
	ParentContext               *PerUserContext
	ApplicationName             string
	IntegrationTestScenarioName string
	ReleasePlanName             string
	ReleasePlanAdmissionName    string
	PerComponentContexts        []*PerComponentContext
}

// Struct to hold data for thread to process each component
type PerComponentContext struct {
	PerComponentWG     *sync.WaitGroup
	ComponentIndex     int
	StartupPause       time.Duration
	Framework          *framework.Framework
	ParentContext      *PerApplicationContext
	ComponentName      string
	SnapshotName       string
	MergeRequestNumber int
	ReleaseName        string
}
