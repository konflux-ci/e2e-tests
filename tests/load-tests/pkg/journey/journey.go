package journey

import "sync"
import "time"
import "math/rand"

import options "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/options"
import logging "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/logging"
import loadtestutils "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/loadtestutils"
import types "github.com/konflux-ci/e2e-tests/tests/load-tests/pkg/types"

// Pointers to all user journey thread contexts
var PerUserContexts []*types.PerUserContext

// Just to create user
func initUserThread(perUserCtx *types.PerUserContext) {
	defer perUserCtx.PerUserWG.Done()

	var err error

	// Create user if needed
	_, err = logging.Measure(
		perUserCtx,
		HandleUser,
		perUserCtx,
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
func PerUserSetup(fn func(*types.PerUserContext), opts *options.Opts) (string, error) {
	perUserWG := &sync.WaitGroup{}
	perUserWG.Add(opts.Concurrency)

	var stageUsers []loadtestutils.User
	var err error
	if opts.Stage {
		stageUsers, err = loadtestutils.LoadStageUsers("users.json")
		if err != nil {
			logging.Logger.Fatal("Failed to load Stage users: %v", err)
		}
	}

	// Initialize all user thread contexts
	for userIndex := 0; userIndex < opts.Concurrency; userIndex++ {
		startupPause := computeStartupPause(userIndex, opts.StartupDelay, opts.StartupJitter)

		logging.Logger.Info("Initiating per user thread %d with pause %v", userIndex, startupPause)

		perUserCtx := &types.PerUserContext{
			PerUserWG:        perUserWG,
			UserIndex:        userIndex,
			StartupPause:     startupPause,
			Opts:             opts,
			StageUsers:       &stageUsers,
			Username:         "",
			Namespace:        "",
		}

		PerUserContexts = append(PerUserContexts, perUserCtx)
	}

	// Create all users (if necessary) and initialize their frameworks
	for _, perUserCtx := range PerUserContexts {
		go initUserThread(perUserCtx)
	}

	perUserWG.Wait()

	// If we are supposed to only purge resources, now when frameworks are initialized, we are done
	if opts.PurgeOnly {
		logging.Logger.Info("Skipping rest of user journey as we were asked to just purge resources")
		return "", nil
	}

	// Fork repositories sequentially as GitHub do not allow more than 3 running forks in parallel anyway
	for _, perUserCtx := range PerUserContexts {
		_, err = logging.Measure(
			perUserCtx,
			HandleRepoForking,
			perUserCtx,
		)
		if err != nil {
			return "", err
		}
	}

	perUserWG.Add(opts.Concurrency)

	// Run actual user thread function
	for _, perUserCtx := range PerUserContexts {
		go fn(perUserCtx)
	}

	perUserWG.Wait()

	return "", nil
}

// Start all the threads to process all applications per user
func PerApplicationSetup(fn func(*types.PerApplicationContext), parentContext *types.PerUserContext) (string, error) {
	perApplicationWG := &sync.WaitGroup{}
	perApplicationWG.Add(parentContext.Opts.ApplicationsCount)

	for applicationIndex := 0; applicationIndex < parentContext.Opts.ApplicationsCount; applicationIndex++ {
		startupPause := computeStartupPause(applicationIndex, parentContext.Opts.StartupDelay, parentContext.Opts.StartupJitter)

		logging.Logger.Info("Initiating per application thread %d-%d with pause %v", parentContext.UserIndex, applicationIndex, startupPause)

		perApplicationCtx := &types.PerApplicationContext{
			PerApplicationWG:            perApplicationWG,
			ApplicationIndex:            applicationIndex,
			StartupPause:                startupPause,
			ParentContext:               parentContext,
			ApplicationName:             "",
			IntegrationTestScenarioName: "",
			ReleasePlanName:             "",
			ReleasePlanAdmissionName:    "",
		}

		if parentContext.Opts.JourneyReuseApplications && applicationIndex != 0 {
			perApplicationCtx.ApplicationName = parentContext.PerApplicationContexts[0].ApplicationName
			perApplicationCtx.IntegrationTestScenarioName = parentContext.PerApplicationContexts[0].IntegrationTestScenarioName
			perApplicationCtx.ReleasePlanName = parentContext.PerApplicationContexts[0].ReleasePlanName
			perApplicationCtx.ReleasePlanAdmissionName = parentContext.PerApplicationContexts[0].ReleasePlanAdmissionName
			logging.Logger.Debug("Reusing application %s and others in thread %d-%d", perApplicationCtx.ApplicationName, parentContext.UserIndex, applicationIndex)
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

		logging.Logger.Info("Initiating per component thread %d-%d-%d with pause %s", parentContext.ParentContext.UserIndex, parentContext.ApplicationIndex, componentIndex, startupPause)

		perComponentCtx := &types.PerComponentContext{
			PerComponentWG: perComponentWG,
			ComponentIndex: componentIndex,
			StartupPause:   startupPause,
			ParentContext:  parentContext,
			ComponentName:  "",
		}

		if parentContext.ParentContext.Opts.JourneyReuseComponents && componentIndex != 0 {
			perComponentCtx.ComponentName = parentContext.PerComponentContexts[0].ComponentName
			logging.Logger.Debug("Reusing component %s in thread %d-%d-%d", perComponentCtx.ComponentName, parentContext.ParentContext.UserIndex, parentContext.ApplicationIndex, componentIndex)
		}

		parentContext.PerComponentContexts = append(parentContext.PerComponentContexts, perComponentCtx)

		go fn(perComponentCtx)
	}

	perComponentWG.Wait()

	return "", nil
}
