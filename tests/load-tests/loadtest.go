package main

import "fmt"
import "time"

import journey "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/journey"
import options "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/options"
import logging "github.com/redhat-appstudio/e2e-tests/tests/load-tests/pkg/logging"

import cobra "github.com/spf13/cobra"
import klog "k8s.io/klog/v2"

//import "os"
//import "context"
//import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//import schema "k8s.io/apimachinery/pkg/runtime/schema"
//import watch "k8s.io/apimachinery/pkg/watch"
////import fields "k8s.io/apimachinery/pkg/fields"
////import runtime "k8s.io/apimachinery/pkg/runtime"
//import appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
/////import eventsv1 "k8s.io/api/events/v1"
//import unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"


var opts = options.Opts{}

var rootCmd = &cobra.Command{
	Use:          "load-test",
	Short:        "Konflux performance test",
	Long:         `Konflux performance test`,
}

func init() {
	rootCmd.Flags().StringVar(&opts.ComponentRepoUrl, "component-repo", "https://github.com/devfile-samples/devfile-sample-code-with-quarkus", "the component repo URL to be used")
	rootCmd.Flags().IntVar(&opts.ComponentsCount, "components-count", 1, "number of components to create per application")
	rootCmd.Flags().StringVar(&opts.ComponentRepoRevision, "component-repo-revision", "main", "the component repo revision, git branch")
	rootCmd.Flags().BoolVarP(&opts.ComponentRepoTemplate, "component-repo-template", "e", false, "if you want to use per-user branch based on provided branch for PaC testing")
	rootCmd.Flags().StringVar(&opts.QuayRepo, "quay-repo", "redhat-user-workloads-stage", "the target quay repo for PaC templated image pushes")
	rootCmd.Flags().StringVar(&opts.UsernamePrefix, "username", "testuser", "the prefix used for usersignup names")
	rootCmd.Flags().BoolVarP(&opts.Stage, "stage", "s", false, "is you want to run the test on stage")
	rootCmd.Flags().BoolVarP(&opts.Purge, "purge", "p", false, "purge all users or resources (on stage) after test is done")
	rootCmd.Flags().BoolVarP(&opts.PurgeOnly, "purge-only", "u", false, "do not run test, only purge resources")
	rootCmd.Flags().StringVar(&opts.TestScenarioGitURL, "test-scenario-git-url", "https://github.com/konflux-ci/integration-examples.git", "test scenario GIT URL")
	rootCmd.Flags().StringVar(&opts.TestScenarioRevision, "test-scenario-revision", "main", "test scenario GIT URL repo revision to use")
	rootCmd.Flags().StringVar(&opts.TestScenarioPathInRepo, "test-scenario-path-in-repo", "pipelines/integration_resolver_pipeline_pass.yaml", "test scenario path in GIT repo")
	rootCmd.Flags().BoolVarP(&opts.WaitPipelines, "waitpipelines", "w", false, "if you want to wait for pipelines to finish")
	rootCmd.Flags().BoolVarP(&opts.WaitIntegrationTestsPipelines, "waitintegrationtestspipelines", "i", false, "if you want to wait for IntegrationTests (Integration Test Scenario) pipelines to finish")
	rootCmd.Flags().BoolVar(&opts.FailFast, "fail-fast", false, "if you want the test to fail fast at first failure")
	rootCmd.Flags().IntVarP(&opts.Concurrency, "concurrency", "c", 1, "number of concurrent threads to execute")
	rootCmd.Flags().IntVar(&opts.JourneyRepeats, "journey-repeats", 1, "number of times to repeat user journey (either this or --journey-duration)")
	rootCmd.Flags().StringVar(&opts.JourneyDuration, "journey-duration", "1h", "repeat user journey until this timeout (either this or --journey-repeats)")
	rootCmd.Flags().BoolVar(&opts.PipelineSkipInitialChecks, "pipeline-skip-initial-checks", true, "if build pipeline runs' initial checks are to be skipped")
	rootCmd.Flags().BoolVar(&opts.PipelineRequestConfigurePac, "pipeline-request-configure-pac", false, "if build pipeline should be taken from component repository .tekton directory")
	rootCmd.Flags().StringVarP(&opts.OutputDir, "output-dir", "o", ".", "directory where output files such as load-tests.log or load-tests.json are stored")
	rootCmd.Flags().StringVar(&opts.BuildPipelineSelectorBundle, "build-pipeline-selector-bundle", "", "BuildPipelineSelector bundle to use when testing with build-definition PR")
	rootCmd.Flags().BoolVarP(&opts.LogVerbose, "log-verbose", "v", false, "log messages with info level and above")
	rootCmd.Flags().BoolVarP(&opts.LogDebug, "log-debug", "d", false, "log messages with debug level and above")
	rootCmd.Flags().BoolVarP(&opts.LogTrace, "log-trace", "t", false, "log messages with trace level and above (i.e. everything)")
}

func main() {
	var err error

	// Setup argument parser
	err = rootCmd.Execute()
	if err != nil {
		klog.Fatalln(err)
	}
	if rootCmd.Flags().Lookup("help").Value.String() == "true" {
		fmt.Println(rootCmd.UsageString())
		return
	}
	err = opts.ProcessOptions()
	if err != nil {
		logging.Logger.Fatal("Failed to process options: %v", err)
	}

	// Setup logging
	logging.Logger.Level = logging.WARNING
	if opts.LogVerbose {
		logging.Logger.Level = logging.INFO
	}
	if opts.LogDebug {
		logging.Logger.Level = logging.DEBUG
	}
	if opts.LogTrace {
		logging.Logger.Level = logging.TRACE
	}

	// Show test options
	logging.Logger.Debug("Options: %+v", opts)

	// Tier up measurements logger
	logging.MeasurementsStart(opts.OutputDir)

	// Start given number of `userJourneyThread()` threads using `journey.Setup()` and wait for them to finish
	_, err = logging.Measure(journey.Setup, userJourneyThread, &opts)
	if err != nil {
		logging.Logger.Fatal("Threads setup failed: %v", err)
	}

	// Cleanup resources
	_, err = logging.Measure(journey.Purge)
	if err != nil {
		logging.Logger.Error("Purging failed: %v", err)
	}

	// Tier down measurements logger
	logging.MeasurementsStop()
}

func userJourneyThread(threadCtx *journey.MainContext) {
	defer threadCtx.ThreadsWG.Done()

	var err error

	// Create user if needed
	_, err = logging.Measure(journey.HandleUser, threadCtx)
	if err != nil {
		logging.Logger.Error("Thread failed: %v", err)
		return
	}

	// If we are supposed to only purge resources, now when frameworks are initialized, we are done
	if threadCtx.Opts.PurgeOnly {
		logging.Logger.Info("Skipping rest of user journey as we were asked to just purge resources")
		return
	}

	//watchCtx := context.Background()
	//gvr := schema.GroupVersionResource{
	//	Group:   "appstudio.redhat.com",
	//	Version: "v1alpha1",
	//	Resource: "applications",
	//}
	//timeOut := int64(60)
	////name := "test-rhtap-1-app-zxlst"
	//listOptions := metav1.ListOptions{
	//	TimeoutSeconds: &timeOut,
	//	//FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	//}
	//// Create watcher
	//fmt.Print("Creating watcher...\n")
	//watcher, err2 := threadCtx.Framework.AsKubeDeveloper.CommonController.DynamicClient().
	//	Resource(gvr).
	//	Namespace(threadCtx.Namespace).
	//	Watch(watchCtx, listOptions)
	//if err2 != nil {
	//	fmt.Printf("Can not get watcher: %v", err2)
	//}
	//// Process events from the watcher
	//fmt.Print("Processing events...\n")
	//for event := range watcher.ResultChan() {
	//	if event.Type == watch.Added || event.Type == watch.Modified || event.Type == watch.Deleted {
	//		// Handle the event based on its type and the received object
	//		// You can cast the object to your custom resource type for further processing
	//		// event.Object will be of type runtime.Object
	//		fmt.Printf("Event type: %s, Object type: %T, Object kind: %s, Object info: %+v\n", event.Type, event.Object, event.Object.GetObjectKind().GroupVersionKind().Kind, event.Object)
	//		typedObj := event.Object.(*unstructured.Unstructured)
	//		bytes, _ := typedObj.MarshalJSON()
	//		var crdObj *appstudioApi.Application
	//		//json.Unmarshal(bytes, &crdObj)
	//		fmt.Printf("Unstructured: %v\n", bytes)
	//		fmt.Printf("Unstructured2: %+v\n", crdObj)
	//		//unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event.Object)
	//		//if err != nil {
	//		//	fmt.Printf("Error: %+v", err)
	//		//}
	//		//fmt.Printf("Unstructured: %+v\n", unstructuredObj["status"]["conditions"])
	//		//// Try to cast the object to your custom type
	//		//customResource, ok := event.Object.(*appstudioApi.Application)
	//		////customResource, ok := event.Object.(*eventsv1.Event)
	//		//if ok {
	//		//	//// Access conditions through your custom resource type's getter method (if exists)
	//		//	//conditions := customResource.GetStatus().Conditions
	//		//	//// Process the conditions list
	//		//	//for _, condition := range conditions {
	//		//	//	fmt.Printf("Condition type: %s, Status: %s, Reason: %s\n", condition.Type, condition.Status, condition.Reason)
	//		//	//}
	//		//	fmt.Printf("customResource type: %T, content: %+v\n", customResource, customResource)
	//		//} else {
	//		//	// Handle unexpected object type
	//		//	fmt.Printf("Error: %+v %+v\n", ok, customResource)
	//		//}
	//		//watchObject, ok := event.Object.(watch.Object)
	//		//if ok {
	//		//	// Access the actual resource object from watchObject.Object
	//		//	resource := watchObject.Object
	//		//	// Now you can use approach 1 to access conditions based on your resource type
	//		//	conditions := resource.GetStatus().Conditions
	//		//	fmt.Printf("Condition %+v", conditions)
	//		//} else {
	//		//	// Handle unexpected object type
	//		//}
	//	}
	//	// Handle errors from the channel
	//	if err3 := watchCtx.Err(); err3 != nil {
	//		// Handle watch error
	//		fmt.Printf("Error watching for resource: %v\n", err3)
	//		// You can choose to retry watching, exit the program, etc. based on your logic
	//	}
	//}
	//// Close the watcher when finished
	//watcher.Stop()
	//os.Exit(10)

	// Cleanup and possibly create build pipline selector
	_, err = logging.Measure(journey.HandleBuildPipelineSelector, threadCtx)
	if err != nil {
		logging.Logger.Error("Thread failed: %v", err)
		return
	}

	for i := 1; i <= threadCtx.Opts.JourneyRepeats; i++ {

		// Create application
		_, err = logging.Measure(journey.HandleApplication, threadCtx)
		if err != nil {
			logging.Logger.Error("Thread failed: %v", err)
			return
		}

		// Create integration test scenario
		_, err = logging.Measure(journey.HandleIntegrationTestScenario, threadCtx)
		if err != nil {
			logging.Logger.Error("Thread failed: %v", err)
			return
		}

		// Template repo
		_, err = logging.Measure(journey.HandleRepoTemplating, threadCtx)
		if err != nil {
			logging.Logger.Error("Thread failed: %v", err)
			return
		}

		// Create component detection query
		_, err = logging.Measure(journey.HandleComponentDetectionQuery, threadCtx)
		if err != nil {
			logging.Logger.Error("Thread failed: %v", err)
			return
		}

		// Start given number of `perComponentThread()` threads using `journey.PerComponentSetup()` and wait for them to finish
		_, err = logging.Measure(journey.PerComponentSetup, perComponentThread, threadCtx)
		if err != nil {
			logging.Logger.Fatal("Per component threads setup failed: %v", err)
		}

		// Check if we are supposed to guit based on --journey-duration
		if time.Now().UTC().After(threadCtx.Opts.JourneyUntil) {
			logging.Logger.Debug("Done with user journey because of timeout")
			break
		}

	}

	// Collect info about PVCs
	_, err = logging.Measure(journey.HandlePersistentVolumeClaim, threadCtx)
	if err != nil {
		logging.Logger.Error("Thread failed: %v", err)
		return
	}

}

func perComponentThread(perComponentCtx *journey.PerComponentContext) {
	defer perComponentCtx.PerComponentWG.Done()

	var err error

	// Create framework so we do not have to share framework with parent thread
	_, err = logging.Measure(journey.HandleNewFramework, perComponentCtx)
	if err != nil {
		logging.Logger.Error("Per component thread failed: %v", err)
		return
	}

	// Create component
	_, err = logging.Measure(journey.HandleComponent, perComponentCtx)
	if err != nil {
		logging.Logger.Error("Per component thread failed: %v", err)
		return
	}

	// Wait for build pipiline run
	_, err = logging.Measure(journey.HandlePipelineRun, perComponentCtx)
	if err != nil {
		logging.Logger.Error("Per component thread failed: %v", err)
		return
	}

	// Wait for test pipiline run
	_, err = logging.Measure(journey.HandleTest, perComponentCtx)
	if err != nil {
		logging.Logger.Error("Per component thread failed: %v", err)
		return
	}
}
