package journey

import "fmt"
import "sync"
import "time"
import "math/rand"

import options "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/options"
import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import loadtestutils "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/loadtestutils"
import types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

import util "github.com/devfile/library/v2/pkg/util"

// Pointers to all user journey thread contexts
var MainContexts []*types.MainContext

// Just to create user
func initUserThread(threadCtx *types.MainContext) {
	defer threadCtx.ThreadsWG.Done()

	var err error

	// Create user if needed
	_, err = logging.Measure(
		threadCtx.ThreadIndex,
		-1,
		-1,
		threadCtx.JourneyRepeatsCounter,
		HandleUser,
		threadCtx,
	)
	if err != nil {
		logging.Logger.Error("Thread failed: %v", err)
		return
	}
}

// Helper function to compute duration to delay startup of some threads based on StartupDelay and StartupJitter command-line options
// If this is a first thread, delay will be skipped as it would not help
func computeStartupPause(index int, delay, jitter time.Duration) time.Duration {
	if index == 0 || delay == 0 {
		return time.Duration(0)
	} else {
		// For delay = 10s and jitter = 3s, this computes random number from 8.5 to 11.5 seconds
		jitterSec := rand.Float64() * jitter.Seconds() - jitter.Seconds() / 2
		jitterDur := time.Duration(jitterSec) * time.Second
		return delay + jitterDur
	}
}

// Start all the user journey threads
// TODO split this to two functions and get PurgeOnly code out
func Setup(fn func(*types.MainContext), opts *options.Opts) (string, error) {
	threadsWG := &sync.WaitGroup{}
	threadsWG.Add(opts.Concurrency)

	var stageUsers []loadtestutils.User
	var err error
	if opts.Stage {
		stageUsers, err = loadtestutils.LoadStageUsers("users.json")
		if err != nil {
			logging.Logger.Fatal("Failed to load Stage users: %v", err)
		}
	}

	// Initialize all user thread contexts
	for threadIndex := 0; threadIndex < opts.Concurrency; threadIndex++ {
		startupPause := computeStartupPause(threadIndex, opts.StartupDelay, opts.StartupJitter)

		logging.Logger.Info("Initiating per user thread %d with pause %v", threadIndex, startupPause)

		threadCtx := &types.MainContext{
			ThreadsWG:        threadsWG,
			ThreadIndex:      threadIndex,
			StartupPause:     startupPause,
			Opts:             opts,
			StageUsers:       &stageUsers,
			Username:         "",
			Namespace:        "",
		}

		MainContexts = append(MainContexts, threadCtx)
	}

	// Create all users (if necessary) and initialize their frameworks
	for _, threadCtx := range MainContexts {
		go initUserThread(threadCtx)
	}

	threadsWG.Wait()

	// If we are supposed to only purge resources, now when frameworks are initialized, we are done
	if opts.PurgeOnly {
		logging.Logger.Info("Skipping rest of user journey as we were asked to just purge resources")
		return "", nil
	}

	// Fork repositories sequentially as GitHub do not allow more than 3 running forks in parallel anyway
	for _, threadCtx := range MainContexts {
		_, err = logging.Measure(
			threadCtx.ThreadIndex,
			-1,
			-1,
			threadCtx.JourneyRepeatsCounter,
			HandleRepoForking,
			threadCtx,
		)
		if err != nil {
			return "", err
		}
	}

	threadsWG.Add(opts.Concurrency)

	// Run actual user thread function
	for _, threadCtx := range MainContexts {
		go fn(threadCtx)
	}

	threadsWG.Wait()

	return "", nil
}

// Start all the threads to process all applications per user
func PerApplicationSetup(fn func(*types.PerApplicationContext), parentContext *types.MainContext) (string, error) {
	perApplicationWG := &sync.WaitGroup{}
	perApplicationWG.Add(parentContext.Opts.ApplicationsCount)

	for applicationIndex := 0; applicationIndex < parentContext.Opts.ApplicationsCount; applicationIndex++ {
		startupPause := computeStartupPause(applicationIndex, parentContext.Opts.StartupDelay, parentContext.Opts.StartupJitter)

		logging.Logger.Info("Initiating per application thread %d-%d with pause %v", parentContext.ThreadIndex, applicationIndex, startupPause)

		perApplicationCtx := &types.PerApplicationContext{
			PerApplicationWG: perApplicationWG,
			ApplicationIndex: applicationIndex,
			StartupPause:     startupPause,
			ParentContext:    parentContext,
			ApplicationName:  fmt.Sprintf("%s-app-%s", parentContext.Opts.RunPrefix, util.GenerateRandomString(5)),
		}

		parentContext.PerApplicationContexts = append(parentContext.PerApplicationContexts, perApplicationCtx)

		go fn(perApplicationCtx)
	}

	perApplicationWG.Wait()

	return "", nil
}

// Start all the threads to process all components per application
func PerComponentSetup(fn func(*types.PerComponentContext), parentContext *types.PerApplicationContext) (string, error) {
	perComponentWG := &sync.WaitGroup{}
	perComponentWG.Add(parentContext.ParentContext.Opts.ComponentsCount)

	for componentIndex := 0; componentIndex < parentContext.ParentContext.Opts.ComponentsCount; componentIndex++ {
		startupPause := computeStartupPause(componentIndex, parentContext.ParentContext.Opts.StartupDelay, parentContext.ParentContext.Opts.StartupJitter)

		logging.Logger.Info("Initiating per component thread %d-%d-%d with pause %s", parentContext.ParentContext.ThreadIndex, parentContext.ApplicationIndex, componentIndex, startupPause)

		perComponentCtx := &types.PerComponentContext{
			PerComponentWG: perComponentWG,
			ComponentIndex: componentIndex,
			StartupPause:   startupPause,
			ParentContext:  parentContext,
			ComponentName:  fmt.Sprintf("%s-comp-%d", parentContext.ApplicationName, componentIndex),
		}

		parentContext.PerComponentContexts = append(parentContext.PerComponentContexts, perComponentCtx)

		go fn(perComponentCtx)
	}

	perComponentWG.Wait()

	return "", nil
}
