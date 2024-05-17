package journey

import "sync"

import options "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/options"
import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"
import loadtestutils "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/loadtestutils"

import framework "github.com/redhat-appstudio/e2e-tests/pkg/framework"
import appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"

// Pointers to all thread contexts
var MainContexts []*MainContext

// Struct to hold thread data
type MainContext struct {
	ThreadsWG              *sync.WaitGroup
	ThreadIndex            int
	Opts                   *options.Opts
	StageUsers             *[]loadtestutils.User
	TemplatingDoneWG       *sync.WaitGroup
	Framework              *framework.Framework
	Username               string
	Namespace              string
	ComponentRepoUrl       string // overrides same value from Opts, needed when templating repos
	PerApplicationContexts []*PerApplicationContext
}

func initUserThread(threadCtx *MainContext) {
	defer threadCtx.ThreadsWG.Done()

	var err error

	// Create user if needed
	_, err = logging.Measure(HandleUser, threadCtx)
	if err != nil {
		logging.Logger.Error("Thread failed: %v", err)
		return
	}
}

func Setup(fn func(*MainContext), opts *options.Opts) (string, error) {
	threadsWG := &sync.WaitGroup{}
	threadsWG.Add(opts.Concurrency)

	templatingDoneWG := &sync.WaitGroup{}
	templatingDoneWG.Add(opts.Concurrency)

	var stageUsers []loadtestutils.User
	var err error
	if opts.Stage {
		stageUsers, err = loadtestutils.LoadStageUsers("users.json")
		if err != nil {
			logging.Logger.Fatal("Failed to load Stage users: %v", err)
		}
	}

	for threadIndex := 0; threadIndex < opts.Concurrency; threadIndex++ {
		logging.Logger.Info("Initiating thread %d", threadIndex)

		threadCtx := &MainContext{
			ThreadsWG:        threadsWG,
			ThreadIndex:      threadIndex,
			Opts:             opts,
			StageUsers:       &stageUsers,
			TemplatingDoneWG: templatingDoneWG,
			Username:         "",
			Namespace:        "",
		}

		MainContexts = append(MainContexts, threadCtx)

		go initUserThread(threadCtx)
	}

	threadsWG.Wait()

	// If we are supposed to only purge resources, now when frameworks are initialized, we are done
	if opts.PurgeOnly {
		logging.Logger.Info("Skipping rest of user journey as we were asked to just purge resources")
		return "", nil
	}

	threadsWG.Add(opts.Concurrency)

	for _, threadCtx := range MainContexts {
		go fn(threadCtx)
	}

	threadsWG.Wait()

	return "", nil
}

// Struct to hold data for thread to process each application
type PerApplicationContext struct {
	PerApplicationWG            *sync.WaitGroup
	ApplicationIndex            int
	Framework                   *framework.Framework
	ParentContext               *MainContext
	ApplicationName             string
	IntegrationTestScenarioName string
	ComponentDetectionQueryName string
	ComponentStubList           []appstudioApi.ComponentDetectionDescription
	PerComponentContexts        []*PerComponentContext
}

func PerApplicationSetup(fn func(*PerApplicationContext), parentContext *MainContext) (string, error) {
	perApplicationWG := &sync.WaitGroup{}
	perApplicationWG.Add(parentContext.Opts.ApplicationsCount)

	for applicationIndex := 0; applicationIndex < parentContext.Opts.ApplicationsCount; applicationIndex++ {
		logging.Logger.Info("Initiating per application thread %d-%d", parentContext.ThreadIndex, applicationIndex)

		perApplicationCtx := &PerApplicationContext{
			PerApplicationWG: perApplicationWG,
			ApplicationIndex: applicationIndex,
			ParentContext:    parentContext,
		}

		parentContext.PerApplicationContexts = append(parentContext.PerApplicationContexts, perApplicationCtx)

		go fn(perApplicationCtx)
	}

	perApplicationWG.Wait()

	return "", nil
}

// Struct to hold data for thread to process each component
type PerComponentContext struct {
	PerComponentWG *sync.WaitGroup
	ComponentIndex int
	Framework      *framework.Framework
	ParentContext  *PerApplicationContext
	ComponentName  string
	SnapshotName   string
}

func PerComponentSetup(fn func(*PerComponentContext), parentContext *PerApplicationContext) (string, error) {
	perComponentWG := &sync.WaitGroup{}
	perComponentWG.Add(parentContext.ParentContext.Opts.ComponentsCount)

	for componentIndex := 0; componentIndex < parentContext.ParentContext.Opts.ComponentsCount; componentIndex++ {
		logging.Logger.Info("Initiating per component thread %d-%d-%d", parentContext.ParentContext.ThreadIndex, parentContext.ApplicationIndex, componentIndex)

		perComponentCtx := &PerComponentContext{
			PerComponentWG: perComponentWG,
			ComponentIndex: componentIndex,
			ParentContext:  parentContext,
		}

		parentContext.PerComponentContexts = append(parentContext.PerComponentContexts, perComponentCtx)

		go fn(perComponentCtx)
	}

	perComponentWG.Wait()

	return "", nil
}
