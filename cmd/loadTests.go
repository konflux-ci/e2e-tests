package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uitable/util/strutil"
	appstudioApi "github.com/redhat-appstudio/application-api/api/v1alpha1"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	loadtestUtils "github.com/redhat-appstudio/e2e-tests/pkg/utils/loadtests"
	integrationv1beta1 "github.com/redhat-appstudio/integration-service/api/v1beta1"
	"github.com/spf13/cobra"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var (
	componentRepoUrl              string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
	usernamePrefix                string = "testuser"
	numberOfUsers                 int
	testScenarioGitURL            string = "https://github.com/redhat-appstudio/integration-examples.git"
	testScenarioRevision          string = "main"
	testScenarioPathInRepo        string = "pipelines/integration_resolver_pipeline_pass.yaml"
	waitPipelines                 bool
	waitDeployments               bool
	waitIntegrationTestsPipelines bool
	verbose                       bool
	logConsole                    bool
	failFast                      bool
	disableMetrics                bool
	threadCount                   int
	randomString                  bool
	pipelineSkipInitialChecks     bool
	stage                         bool
	outputDir                     string = "."
	enableProgressBars            bool
)

var (
	UserCreationTimeMaxPerThread         []time.Duration
	ApplicationCreationTimeMaxPerThread  []time.Duration
	CDQCreationTimeMaxPerThread          []time.Duration
	ComponentCreationTimeMaxPerThread    []time.Duration
	PipelineRunSucceededTimeMaxPerThread []time.Duration

	DeploymentSucceededTimeMaxPerThread                  []time.Duration
	IntegrationTestsPipelineRunSucceededTimeMaxPerThread []time.Duration

	UserCreationTimeSumPerThread         []time.Duration
	ApplicationCreationTimeSumPerThread  []time.Duration
	CDQCreationTimeSumPerThread          []time.Duration
	ComponentCreationTimeSumPerThread    []time.Duration
	PipelineRunSucceededTimeSumPerThread []time.Duration
	PipelineRunFailedTimeSumPerThread    []time.Duration

	DeploymentSucceededTimeSumPerThread                  []time.Duration
	DeploymentFailedTimeSumPerThread                     []time.Duration
	IntegrationTestsPipelineRunSucceededTimeSumPerThread []time.Duration
	IntegrationTestsPipelineRunFailedTimeSumPerThread    []time.Duration

	SuccessfulUserCreationsPerThread                []int64
	SuccessfulApplicationCreationsPerThread         []int64
	SuccessfulCDQCreationsPerThread                 []int64
	SuccessfulComponentCreationsPerThread           []int64
	SuccessfulPipelineRunsPerThread                 []int64
	SuccessfulDeploymentsPerThread                  []int64
	SuccessfulIntegrationTestsPipelineRunsPerThread []int64

	FailedUserCreationsPerThread                []int64
	FailedApplicationCreationsPerThread         []int64
	FailedCDQCreationsPerThread                 []int64
	FailedComponentCreationsPerThread           []int64
	FailedPipelineRunsPerThread                 []int64
	FailedDeploymentsPerThread                  []int64
	FailedIntegrationTestsPipelineRunsPerThread []int64

	frameworkMap                      *sync.Map
	userComponentMap                  *sync.Map
	userTestScenarioMap               *sync.Map
	userComponentPipelineRunMap       *sync.Map
	errorCountMap                     map[int]ErrorCount
	errorMutex                        = &sync.Mutex{}
	usersBarMutex                     = &sync.Mutex{}
	applicationsBarMutex              = &sync.Mutex{}
	cdqsBarMutex                      = &sync.Mutex{}
	componentsBarMutex                = &sync.Mutex{}
	pipelinesBarMutex                 = &sync.Mutex{}
	deploymentsBarMutex               = &sync.Mutex{}
	integrationTestsPipelinesBarMutex = &sync.Mutex{}
	threadsWG                         *sync.WaitGroup
	logData                           LogData
	stageUsers                        []loadtestUtils.User
	selectedUsers                     []loadtestUtils.User
	CI                                bool
	JobName                           string
)

type ErrorOccurrence struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
}

type ErrorCount struct {
	ErrorCode int `json:"errorCode"`
	Count     int `json:"count"`
}

type LogData struct {
	Timestamp                         string  `json:"timestamp"`
	EndTimestamp                      string  `json:"endTimestamp"`
	MachineName                       string  `json:"machineName"`
	BinaryDetails                     string  `json:"binaryDetails"`
	ComponentRepoUrl                  string  `json:"componentRepoUrl"`
	NumberOfThreads                   int     `json:"threads"`
	NumberOfUsersPerThread            int     `json:"usersPerThread"`
	NumberOfUsers                     int     `json:"totalUsers"`
	PipelineSkipInitialChecks         bool    `json:"pipelineSkipInitialChecks"`
	LoadTestCompletionStatus          string  `json:"status"`
	AverageTimeToSpinUpUsers          float64 `json:"createUserTimeAvg"`
	MaxTimeToSpinUpUsers              float64 `json:"createUserTimeMax"`
	AverageTimeToCreateApplications   float64 `json:"createApplicationsTimeAvg"`
	MaxTimeToCreateApplications       float64 `json:"createApplicationsTimeMax"`
	AverageTimeToCreateCDQs           float64 `json:"createCDQsTimeAvg"`
	MaxTimeToCreateCDQs               float64 `json:"createCDQsTimeMax"`
	AverageTimeToCreateComponents     float64 `json:"createComponentsTimeAvg"`
	MaxTimeToCreateComponents         float64 `json:"createComponentsTimeMax"`
	AverageTimeToRunPipelineSucceeded float64 `json:"runPipelineSucceededTimeAvg"`
	MaxTimeToRunPipelineSucceeded     float64 `json:"runPipelineSucceededTimeMax"`
	AverageTimeToRunPipelineFailed    float64 `json:"runPipelineFailedTimeAvg"`

	AverageTimeToDeploymentSucceeded float64 `json:"deploymentSucceededTimeAvg"`
	MaxTimeToDeploymentSucceeded     float64 `json:"deploymentSucceededTimeMax"`
	AverageTimeToDeploymentFailed    float64 `json:"deploymentFailedTimeAvg"`

	IntegrationTestsAverageTimeToRunPipelineSucceeded float64 `json:"integrationTestsRunPipelineSucceededTimeAvg"`
	IntegrationTestsMaxTimeToRunPipelineSucceeded     float64 `json:"integrationTestsRunPipelineSucceededTimeMax"`
	IntegrationTestsAverageTimeToRunPipelineFailed    float64 `json:"integrationTestsRunPipelineFailedTimeAvg"`

	UserCreationSuccessCount        int64   `json:"createUserSuccesses"`
	UserCreationFailureCount        int64   `json:"createUserFailures"`
	UserCreationFailureRate         float64 `json:"createUserFailureRate"`
	ApplicationCreationSuccessCount int64   `json:"createApplicationsSuccesses"`
	ApplicationCreationFailureCount int64   `json:"createApplicationsFailures"`
	ApplicationCreationFailureRate  float64 `json:"createApplicationsFailureRate"`
	CDQCreationSuccessCount         int64   `json:"createCDQsSuccesses"`
	CDQCreationFailureCount         int64   `json:"createCDQsFailures"`
	CDQCreationFailureRate          float64 `json:"createCDQsFailureRate"`
	ComponentCreationSuccessCount   int64   `json:"createComponentsSuccesses"`
	ComponentCreationFailureCount   int64   `json:"createComponentsFailures"`
	ComponentCreationFailureRate    float64 `json:"createComponentsFailureRate"`
	PipelineRunSuccessCount         int64   `json:"runPipelineSuccesses"`
	PipelineRunFailureCount         int64   `json:"runPipelineFailures"`
	PipelineRunFailureRate          float64 `json:"runPipelineFailureRate"`

	DeploymentSuccessCount int64   `json:"deploymentSuccesses"`
	DeploymentFailureCount int64   `json:"deploymentFailures"`
	DeploymentFailureRate  float64 `json:"deploymentFailureRate"`

	IntegrationTestsPipelineRunSuccessCount int64   `json:"integrationTestsRunPipelineSuccesses"`
	IntegrationTestsPipelineRunFailureCount int64   `json:"integrationTestsRunPipelineFailures"`
	IntegrationTestsPipelineRunFailureRate  float64 `json:"integrationTestsRunPipelineFailureRate"`

	WorkloadKPI float64 `json:"workloadKPI"`

	ErrorCounts []ErrorCount      `json:"errorCounts"`
	Errors      []ErrorOccurrence `json:"errors"`
	ErrorsTotal int               `json:"errorsTotal"`
}

func createLogDataJSON(outputFile string, logDataInput LogData) error {
	jsonData, err := json.MarshalIndent(logDataInput, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %v", err)
	}

	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing JSON file: %v", err)
	}

	return nil
}

var rootCmd = &cobra.Command{
	Use:           "load-test",
	Short:         "Used to Generate Users and Run Load Tests on AppStudio.",
	Long:          `Used to Generate Users and Run Load Tests on AppStudio.`,
	SilenceErrors: true,
	SilenceUsage:  false,
	Args:          cobra.NoArgs,
	Run:           setup,
}

var AppStudioUsersBar *uiprogress.Bar
var ApplicationsBar *uiprogress.Bar
var CDQsBar *uiprogress.Bar
var ComponentsBar *uiprogress.Bar
var PipelinesBar *uiprogress.Bar
var IntegrationTestsPipelinesBar *uiprogress.Bar
var DeploymentsBar *uiprogress.Bar

func ExecuteLoadTest() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// generate random string from charset = "abcdefghijklmnopqrstuvwxyz0123456789"
// We shall use length = 5 characters length random string with 60,466,176 random combinations
const customCharset = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomStringFromCharset(length int) string {
	var result []byte
	for i := 0; i < length; i++ {
		index := rand.Intn(len(customCharset))
		result = append(result, customCharset[index])
	}
	return string(result)
}

func init() {
	rootCmd.Flags().StringVar(&componentRepoUrl, "component-repo", componentRepoUrl, "the component repo URL to be used")
	rootCmd.Flags().StringVar(&usernamePrefix, "username", usernamePrefix, "the prefix used for usersignup names")
	// TODO use a custom kubeconfig and introduce debug logging and trace
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "if 'debug' traces should be displayed in the console")
	rootCmd.Flags().BoolVarP(&stage, "stage", "s", false, "is you want to run the test on stage")
	rootCmd.Flags().IntVarP(&numberOfUsers, "users", "u", 5, "the number of user accounts to provision per thread")
	rootCmd.Flags().StringVar(&testScenarioGitURL, "test-scenario-git-url", testScenarioGitURL, "test scenario GIT URL")
	rootCmd.Flags().StringVar(&testScenarioRevision, "test-scenario-revision", testScenarioRevision, "test scenario GIT URL repo revision to use")
	rootCmd.Flags().StringVar(&testScenarioPathInRepo, "test-scenario-path-in-repo", testScenarioPathInRepo, "test scenario path in GIT repo")
	rootCmd.Flags().BoolVarP(&waitPipelines, "waitpipelines", "w", false, "if you want to wait for pipelines to finish")
	rootCmd.Flags().BoolVarP(&waitIntegrationTestsPipelines, "waitintegrationtestspipelines", "i", false, "if you want to wait for IntegrationTests (Integration Test Scenario) pipelines to finish")
	rootCmd.Flags().BoolVarP(&waitDeployments, "waitdeployments", "d", false, "if you want to wait for deployments to finish")
	rootCmd.Flags().BoolVarP(&logConsole, "log-to-console", "l", false, "if you want to log to console in addition to the log file")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "if you want the test to fail fast at first failure")
	rootCmd.Flags().BoolVar(&disableMetrics, "disable-metrics", false, "if you want to disable metrics gathering")
	rootCmd.Flags().IntVarP(&threadCount, "threads", "t", 1, "number of concurrent threads to execute")
	rootCmd.Flags().BoolVarP(&randomString, "randomstring", "r", false, "if you want to add random string to the user prefix")
	rootCmd.Flags().BoolVar(&pipelineSkipInitialChecks, "pipeline-skip-initial-checks", true, "if pipeline runs' initial checks are to be skipped")
	rootCmd.Flags().StringVarP(&outputDir, "output-dir", "o", ".", "directory where output files such as load-tests.log or load-tests.json are stored")
	rootCmd.Flags().BoolVar(&enableProgressBars, "enable-progress-bars", false, "if you want to enable progress bars")
}

func logError(errCode int, message string) {
	msg := fmt.Sprintf("Error #%d: %s", errCode, message)
	if failFast {
		klog.Fatalln(msg)
	} else {
		klog.Errorln(msg)
	}
	errorMutex.Lock()
	defer errorMutex.Unlock()

	errorCount, ok := errorCountMap[errCode]
	if ok {
		errorCount.Count = errorCount.Count + 1
		errorCountMap[errCode] = errorCount
	} else {
		errorCountMap[errCode] = ErrorCount{
			ErrorCode: errCode,
			Count:     1,
		}
	}

	errorOccurrence := ErrorOccurrence{
		ErrorCode: errCode,
		Message:   message,
	}
	logData.Errors = append(logData.Errors, errorOccurrence)
}

func setKlogFlag(fs flag.FlagSet, name string, value string) {
	err := fs.Set(name, value)
	if err != nil {
		klog.Fatalf("Unable to set klog flag %s: %v", name, err)
	}
}

func setup(cmd *cobra.Command, args []string) {
	cmd.SilenceUsage = true

	JobName = loadtestUtils.GetJobName()

	// waitDeployments sets waitIntegrationTestsPipelines=true implicitly
	waitIntegrationTestsPipelines = waitIntegrationTestsPipelines || waitDeployments

	// waitIntegrationTestsPipelines sets waitPipelines=true implicitly
	waitPipelines = waitPipelines || waitIntegrationTestsPipelines

	logFile, err := os.Create(fmt.Sprintf("%s/load-tests.log", outputDir))
	if err != nil {
		klog.Fatalf("Error creating log file: %v", err)
	}
	var fs flag.FlagSet
	klog.InitFlags(&fs)
	setKlogFlag(fs, "log_file", logFile.Name())
	setKlogFlag(fs, "logtostderr", "false")
	setKlogFlag(fs, "alsologtostderr", strconv.FormatBool(logConsole))

	overallCount := numberOfUsers * threadCount

	klog.Infof("Number of threads: %d", threadCount)
	klog.Infof("Number of users per thread: %d", numberOfUsers)
	klog.Infof("Number of users overall: %d", overallCount)
	klog.Infof("Pipeline run initial checks skipped: %t", pipelineSkipInitialChecks)

	klog.Infof("🕖 initializing...\n")

	if !disableMetrics {
		// add metrics releated code here
		klog.Infof("Sleeping till all metrics queries gets init")
		time.Sleep(time.Second * 10)
	}

	if stage {
		klog.Infof("Loading Stage Users...\n")
		stageUsers, err = loadtestUtils.LoadStageUsers(constants.JsonStageUsersPath)
		if err != nil {
			klog.Fatalf("Error Loading Stage Users from the given Path Please check file/contents exists: %v", err)
		}

		selectedUsers, err = loadtestUtils.SelectUsers(stageUsers, numberOfUsers, threadCount, len(stageUsers))
		if err != nil {
			klog.Fatalf("Error Selecting the Users Based on thread count: %v", err)
		}
	}

	machineName, err := os.Hostname()
	if err != nil {
		klog.Errorf("error getting hostname: %v\n", err)
		return
	}

	goVersion := runtime.Version()
	goOS := runtime.GOOS
	goArch := runtime.GOARCH
	binaryDetails := fmt.Sprintf("Built with %s for %s/%s", goVersion, goOS, goArch)

	logData = LogData{
		Timestamp:                 time.Now().Format("2006-01-02T15:04:05Z07:00"),
		MachineName:               machineName,
		BinaryDetails:             binaryDetails,
		ComponentRepoUrl:          componentRepoUrl,
		NumberOfThreads:           threadCount,
		NumberOfUsersPerThread:    numberOfUsers,
		NumberOfUsers:             overallCount,
		PipelineSkipInitialChecks: pipelineSkipInitialChecks,
		Errors:                    []ErrorOccurrence{},
		ErrorCounts:               []ErrorCount{},
	}

	klog.Infof("🍿 provisioning users...\n")

	uip := uiprogress.New()
	uip.Start()

	barLength := 60

	if enableProgressBars {
		userProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Users (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedUserCreationsPerThread)), barLength, ' ')
		})
		AppStudioUsersBar = userProgress

		applicationProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Applications (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedApplicationCreationsPerThread)), barLength, ' ')
		})
		ApplicationsBar = applicationProgress

		cdqProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.PadLeft(fmt.Sprintf("Creating AppStudio CDQs (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedCDQCreationsPerThread)), barLength, ' ')
		})
		CDQsBar = cdqProgress

		componentProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Components (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedComponentCreationsPerThread)), barLength, ' ')
		})
		ComponentsBar = componentProgress

		if waitPipelines {
			pipelineProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
				return strutil.PadLeft(fmt.Sprintf("Waiting for pipelines to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedPipelineRunsPerThread)), barLength, ' ')
			})
			PipelinesBar = pipelineProgress
		}

		if waitIntegrationTestsPipelines {
			integrationTestProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
				return strutil.PadLeft(fmt.Sprintf("Waiting for integration tests to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedIntegrationTestsPipelineRunsPerThread)), barLength, ' ')
			})
			IntegrationTestsPipelinesBar = integrationTestProgress
		}

		if waitDeployments {
			deploymentProgress := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
				return strutil.PadLeft(fmt.Sprintf("Waiting for deployments to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedDeploymentsPerThread)), barLength, ' ')
			})
			DeploymentsBar = deploymentProgress
		}
	} else {
		klog.Infoln("Progress bars are disabled by default. Please hold off until all iterations has completed. To enable the progress bars run with the --enable-progress-bars in [OPTIONS]")
	}

	UserCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	ApplicationCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	CDQCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	ComponentCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	PipelineRunSucceededTimeMaxPerThread = make([]time.Duration, threadCount)

	DeploymentSucceededTimeMaxPerThread = make([]time.Duration, threadCount)
	IntegrationTestsPipelineRunSucceededTimeMaxPerThread = make([]time.Duration, threadCount)

	UserCreationTimeSumPerThread = make([]time.Duration, threadCount)
	ApplicationCreationTimeSumPerThread = make([]time.Duration, threadCount)
	CDQCreationTimeSumPerThread = make([]time.Duration, threadCount)
	ComponentCreationTimeSumPerThread = make([]time.Duration, threadCount)
	PipelineRunSucceededTimeSumPerThread = make([]time.Duration, threadCount)
	PipelineRunFailedTimeSumPerThread = make([]time.Duration, threadCount)

	DeploymentSucceededTimeSumPerThread = make([]time.Duration, threadCount)
	DeploymentFailedTimeSumPerThread = make([]time.Duration, threadCount)

	IntegrationTestsPipelineRunSucceededTimeSumPerThread = make([]time.Duration, threadCount)
	IntegrationTestsPipelineRunFailedTimeSumPerThread = make([]time.Duration, threadCount)

	SuccessfulUserCreationsPerThread = make([]int64, threadCount)
	SuccessfulApplicationCreationsPerThread = make([]int64, threadCount)
	SuccessfulCDQCreationsPerThread = make([]int64, threadCount)
	SuccessfulComponentCreationsPerThread = make([]int64, threadCount)
	SuccessfulPipelineRunsPerThread = make([]int64, threadCount)

	SuccessfulDeploymentsPerThread = make([]int64, threadCount)
	SuccessfulIntegrationTestsPipelineRunsPerThread = make([]int64, threadCount)

	FailedUserCreationsPerThread = make([]int64, threadCount)
	FailedApplicationCreationsPerThread = make([]int64, threadCount)
	FailedCDQCreationsPerThread = make([]int64, threadCount)
	FailedComponentCreationsPerThread = make([]int64, threadCount)
	FailedPipelineRunsPerThread = make([]int64, threadCount)

	FailedDeploymentsPerThread = make([]int64, threadCount)
	FailedIntegrationTestsPipelineRunsPerThread = make([]int64, threadCount)

	frameworkMap = &sync.Map{}
	userComponentMap = &sync.Map{}
	userTestScenarioMap = &sync.Map{}
	userComponentPipelineRunMap = &sync.Map{}
	errorCountMap = make(map[int]ErrorCount)

	rand.Seed(time.Now().UnixNano())
	threadsWG = &sync.WaitGroup{}
	threadsWG.Add(threadCount)

	for thread := 0; thread < threadCount; thread++ {
		go userJourneyThread(frameworkMap, threadsWG, thread, AppStudioUsersBar, ApplicationsBar, CDQsBar, ComponentsBar, PipelinesBar, IntegrationTestsPipelinesBar, DeploymentsBar)
	}

	// Todo add cleanup functions that will delete user signups
	threadsWG.Wait()
	uip.Stop()

	logData.EndTimestamp = time.Now().Format("2006-01-02T15:04:05Z07:00")

	logData.LoadTestCompletionStatus = "Completed"

	// Compiling data about UserSignups
	userCreationSuccessCount := sumFromArray(SuccessfulUserCreationsPerThread)
	logData.UserCreationSuccessCount = userCreationSuccessCount

	userCreationFailureCount := sumFromArray(FailedUserCreationsPerThread)
	logData.UserCreationFailureCount = userCreationFailureCount

	averageTimeToSpinUpUsers := float64(0)
	if userCreationSuccessCount > 0 {
		averageTimeToSpinUpUsers = sumDurationFromArray(UserCreationTimeSumPerThread).Seconds() / float64(userCreationSuccessCount)
	}
	logData.AverageTimeToSpinUpUsers = averageTimeToSpinUpUsers

	logData.MaxTimeToSpinUpUsers = maxDurationFromArray(UserCreationTimeMaxPerThread).Seconds()

	userCreationFailureRate := float64(userCreationFailureCount) / float64(overallCount)
	logData.UserCreationFailureRate = userCreationFailureRate

	// Compiling data about Applications
	applicationCreationSuccessCount := sumFromArray(SuccessfulApplicationCreationsPerThread)
	logData.ApplicationCreationSuccessCount = applicationCreationSuccessCount

	applicationCreationFailureCount := sumFromArray(FailedApplicationCreationsPerThread)
	logData.ApplicationCreationFailureCount = applicationCreationFailureCount

	averageTimeToCreateApplications := float64(0)
	if applicationCreationSuccessCount > 0 {
		averageTimeToCreateApplications = sumDurationFromArray(ApplicationCreationTimeSumPerThread).Seconds() / float64(applicationCreationSuccessCount)
	}
	logData.AverageTimeToCreateApplications = averageTimeToCreateApplications

	logData.MaxTimeToCreateApplications = maxDurationFromArray(ApplicationCreationTimeMaxPerThread).Seconds()

	applicationCreationFailureRate := float64(applicationCreationFailureCount) / float64(overallCount)
	logData.ApplicationCreationFailureRate = applicationCreationFailureRate

	// Compiling data about CDQs
	cdqCreationSuccessCount := sumFromArray(SuccessfulCDQCreationsPerThread)
	logData.CDQCreationSuccessCount = cdqCreationSuccessCount

	cdqCreationFailureCount := sumFromArray(FailedCDQCreationsPerThread)
	logData.CDQCreationFailureCount = cdqCreationFailureCount

	averageTimeToCreateCDQs := float64(0)
	if cdqCreationSuccessCount > 0 {
		averageTimeToCreateCDQs = sumDurationFromArray(CDQCreationTimeSumPerThread).Seconds() / float64(cdqCreationSuccessCount)
	}
	logData.AverageTimeToCreateCDQs = averageTimeToCreateCDQs

	logData.MaxTimeToCreateCDQs = maxDurationFromArray(CDQCreationTimeMaxPerThread).Seconds()

	cdqCreationFailureRate := float64(cdqCreationFailureCount) / float64(overallCount)
	logData.CDQCreationFailureRate = cdqCreationFailureRate

	// Compiling data about Components
	componentCreationSuccessCount := sumFromArray(SuccessfulComponentCreationsPerThread)
	logData.ComponentCreationSuccessCount = componentCreationSuccessCount

	componentCreationFailureCount := sumFromArray(FailedComponentCreationsPerThread)
	logData.ComponentCreationFailureCount = componentCreationFailureCount

	averageTimeToCreateComponents := float64(0)
	if componentCreationSuccessCount > 0 {
		averageTimeToCreateComponents = sumDurationFromArray(ComponentCreationTimeSumPerThread).Seconds() / float64(cdqCreationSuccessCount)
	}
	logData.AverageTimeToCreateComponents = averageTimeToCreateComponents

	logData.MaxTimeToCreateComponents = maxDurationFromArray(ComponentCreationTimeMaxPerThread).Seconds()

	componentCreationFailureRate := float64(componentCreationFailureCount) / float64(overallCount)
	logData.ComponentCreationFailureRate = componentCreationFailureRate

	// Compile data about PipelineRuns
	pipelineRunSuccessCount := sumFromArray(SuccessfulPipelineRunsPerThread)
	logData.PipelineRunSuccessCount = pipelineRunSuccessCount

	pipelineRunFailureCount := sumFromArray(FailedPipelineRunsPerThread)
	logData.PipelineRunFailureCount = pipelineRunFailureCount

	averageTimeToRunPipelineSucceeded := float64(0)
	if pipelineRunSuccessCount > 0 {
		averageTimeToRunPipelineSucceeded = sumDurationFromArray(PipelineRunSucceededTimeSumPerThread).Seconds() / float64(pipelineRunSuccessCount)
	}
	logData.AverageTimeToRunPipelineSucceeded = averageTimeToRunPipelineSucceeded

	logData.MaxTimeToRunPipelineSucceeded = maxDurationFromArray(PipelineRunSucceededTimeMaxPerThread).Seconds()

	averageTimeToRunPipelineFailed := float64(0)
	if pipelineRunFailureCount > 0 {
		averageTimeToRunPipelineFailed = sumDurationFromArray(PipelineRunFailedTimeSumPerThread).Seconds() / float64(pipelineRunFailureCount)
	}
	logData.AverageTimeToRunPipelineFailed = averageTimeToRunPipelineFailed

	pipelineRunFailureRate := float64(pipelineRunFailureCount) / float64(overallCount)
	logData.PipelineRunFailureRate = pipelineRunFailureRate

	// Compile data about integration tests
	integrationTestsPipelineRunSuccessCount := sumFromArray(SuccessfulIntegrationTestsPipelineRunsPerThread)
	logData.IntegrationTestsPipelineRunSuccessCount = integrationTestsPipelineRunSuccessCount

	integrationTestsPipelineRunFailureCount := sumFromArray(FailedIntegrationTestsPipelineRunsPerThread)
	logData.IntegrationTestsPipelineRunFailureCount = integrationTestsPipelineRunFailureCount

	IntegrationTestsAverageTimeToRunPipelineSucceeded := float64(0)
	if integrationTestsPipelineRunSuccessCount > 0 {
		IntegrationTestsAverageTimeToRunPipelineSucceeded = sumDurationFromArray(IntegrationTestsPipelineRunSucceededTimeSumPerThread).Seconds() / float64(integrationTestsPipelineRunSuccessCount)
	}
	logData.IntegrationTestsAverageTimeToRunPipelineSucceeded = IntegrationTestsAverageTimeToRunPipelineSucceeded

	logData.IntegrationTestsMaxTimeToRunPipelineSucceeded = maxDurationFromArray(IntegrationTestsPipelineRunSucceededTimeMaxPerThread).Seconds()

	IntegrationTestsAverageTimeToRunPipelineFailed := float64(0)
	if integrationTestsPipelineRunFailureCount > 0 {
		IntegrationTestsAverageTimeToRunPipelineFailed = sumDurationFromArray(IntegrationTestsPipelineRunFailedTimeSumPerThread).Seconds() / float64(integrationTestsPipelineRunFailureCount)
	}
	logData.IntegrationTestsAverageTimeToRunPipelineFailed = IntegrationTestsAverageTimeToRunPipelineFailed

	IntegrationTestsPipelineRunFailureRate := float64(integrationTestsPipelineRunFailureCount) / float64(overallCount)
	logData.IntegrationTestsPipelineRunFailureRate = IntegrationTestsPipelineRunFailureRate

	// Compile data about Deployments
	deploymentSuccessCount := sumFromArray(SuccessfulDeploymentsPerThread)
	logData.DeploymentSuccessCount = deploymentSuccessCount

	deploymentFailureCount := sumFromArray(FailedDeploymentsPerThread)
	logData.DeploymentFailureCount = deploymentFailureCount

	averageTimeToDeploymentSucceeded := float64(0)
	if deploymentSuccessCount > 0 {
		averageTimeToDeploymentSucceeded = sumDurationFromArray(DeploymentSucceededTimeSumPerThread).Seconds() / float64(deploymentSuccessCount)
	}
	logData.AverageTimeToDeploymentSucceeded = averageTimeToDeploymentSucceeded

	logData.MaxTimeToDeploymentSucceeded = maxDurationFromArray(DeploymentSucceededTimeMaxPerThread).Seconds()

	averageTimeToDeploymentFailed := float64(0)
	if deploymentFailureCount > 0 {
		averageTimeToDeploymentFailed = sumDurationFromArray(DeploymentFailedTimeSumPerThread).Seconds() / float64(deploymentFailureCount)
	}
	logData.AverageTimeToDeploymentFailed = averageTimeToDeploymentFailed

	deploymentFailureRate := float64(deploymentFailureCount) / float64(overallCount)
	logData.DeploymentFailureRate = deploymentFailureRate

	workloadKPI := logData.AverageTimeToCreateApplications + logData.AverageTimeToCreateCDQs + logData.AverageTimeToCreateComponents + logData.AverageTimeToRunPipelineSucceeded + logData.AverageTimeToDeploymentSucceeded
	logData.WorkloadKPI = workloadKPI
	if stage {
		StageCleanup(selectedUsers)
	}

	klog.Infof("🏁 Load Test Completed!")
	klog.Infof("📈 Results 📉")

	klog.Infof("Workload KPI: %.2f", workloadKPI)

	klog.Infof("Avg/max time to spin up users: %.2f s/%.2f s", averageTimeToSpinUpUsers, logData.MaxTimeToSpinUpUsers)
	klog.Infof("Avg/max time to create application: %.2f s/%.2f s", averageTimeToCreateApplications, logData.MaxTimeToCreateApplications)
	klog.Infof("Avg/max time to create cdq: %.2f s/%.2f s", averageTimeToCreateCDQs, logData.MaxTimeToCreateCDQs)
	klog.Infof("Avg/max time to create component: %.2f s/%.2f s", averageTimeToCreateComponents, logData.MaxTimeToCreateComponents)
	klog.Infof("Avg/max time to complete pipelinesrun: %.2f s/%.2f s", averageTimeToRunPipelineSucceeded, logData.MaxTimeToRunPipelineSucceeded)
	klog.Infof("Avg/max time to complete integration test: %.2f s/%.2f s", IntegrationTestsAverageTimeToRunPipelineSucceeded, logData.IntegrationTestsMaxTimeToRunPipelineSucceeded)
	klog.Infof("Avg/max time to complete deployment: %.2f s/%.2f s", averageTimeToDeploymentSucceeded, logData.MaxTimeToDeploymentSucceeded)

	klog.Infof("Average time to fail pipelinerun: %.2f s", averageTimeToRunPipelineFailed)
	klog.Infof("Average time to fail integration test: %.2f s", IntegrationTestsAverageTimeToRunPipelineFailed)
	klog.Infof("Average time to fail deployment: %.2f s", averageTimeToDeploymentFailed)

	klog.Infof("Number of times user creation worked/failed: %d/%d (%.2f %%)", userCreationSuccessCount, userCreationFailureCount, userCreationFailureRate*100)
	klog.Infof("Number of times application creation worked/failed: %d/%d (%.2f %%)", applicationCreationSuccessCount, applicationCreationFailureCount, applicationCreationFailureRate*100)
	klog.Infof("Number of times cdq creation worked/failed: %d/%d (%.2f %%)", cdqCreationSuccessCount, cdqCreationFailureCount, cdqCreationFailureRate*100)
	klog.Infof("Number of times component creation worked/failed: %d/%d (%.2f %%)", componentCreationSuccessCount, componentCreationFailureCount, componentCreationFailureRate*100)
	klog.Infof("Number of times pipeline run worked/failed: %d/%d (%.2f %%)", pipelineRunSuccessCount, pipelineRunFailureCount, pipelineRunFailureRate*100)
	klog.Infof("Number of times integration tests' pipeline run worked/failed: %d/%d (%.2f %%)", integrationTestsPipelineRunSuccessCount, integrationTestsPipelineRunFailureCount, IntegrationTestsPipelineRunFailureRate*100)
	klog.Infof("Number of times deployment worked/failed: %d/%d (%.2f %%)", deploymentSuccessCount, deploymentFailureCount, deploymentFailureRate*100)

	klog.Infoln("Error summary:")
	for _, errorCount := range errorCountMap {
		klog.Infof("Number of error #%d occured: %d", errorCount.ErrorCode, errorCount.Count)
		logData.ErrorCounts = append(logData.ErrorCounts, errorCount)
	}
	logData.ErrorsTotal = len(logData.Errors)
	klog.Infof("Total number of errors occured: %d", logData.ErrorsTotal)

	err = createLogDataJSON(fmt.Sprintf("%s/load-tests.json", outputDir), logData)
	if err != nil {
		klog.Errorf("error while marshalling JSON: %v\n", err)
	}

	klog.StopFlushDaemon()
	klog.Flush()
}

func StageCleanup(users []loadtestUtils.User) {

	for _, user := range users {
		framework := frameworkForUser(user.Username)
		err := framework.AsKubeDeveloper.HasController.DeleteAllApplicationsInASpecificNamespace(framework.UserNamespace, 5*time.Minute)
		if err != nil {
			klog.Errorf("while deleting resources for user: %s, got error: %v\n", user.Username, err)
		}

		err = framework.AsKubeDeveloper.HasController.DeleteAllComponentDetectionQueriesInASpecificNamespace(framework.UserNamespace, 5*time.Minute)
		if err != nil {
			klog.Errorf("while deleting component detection queries for user: %s, got error: %v\n", user.Username, err)
		}
	}
}

func maxDurationFromArray(durations []time.Duration) time.Duration {
	max := time.Duration(0)
	for _, i := range durations {
		if i > max {
			max = i
		}
	}
	return max
}

func sumDurationFromArray(durations []time.Duration) time.Duration {
	sum := time.Duration(0)
	for _, i := range durations {
		sum += i
	}
	return sum
}

func sumFromArray(array []int64) int64 {
	sum := int64(0)
	for _, i := range array {
		sum += i
	}
	return sum
}

func increaseBar(bar *uiprogress.Bar, mutex *sync.Mutex) {
	if enableProgressBars {
		mutex.Lock()
		defer mutex.Unlock()
		bar.Incr()
	}
}

func componentForUser(username string) string {
	val, ok := userComponentMap.Load(username)
	if ok {
		componentName, ok2 := val.(string)
		if ok2 {
			return componentName
		} else {
			klog.Errorf("Invalid type of map value: %+v", val)
		}
	}
	return ""
}

func frameworkForUser(username string) *framework.Framework {
	val, ok := frameworkMap.Load(username)
	if ok {
		framework, ok2 := val.(*framework.Framework)
		if ok2 {
			return framework
		} else {
			klog.Errorf("Invalid type of map value: %+v", val)
		}
	}
	return nil
}

func testScenarioForUser(username string) string {
	val, ok := userTestScenarioMap.Load(username)
	if ok {
		testScenarioName, ok2 := val.(string)
		if ok2 {
			return testScenarioName
		} else {
			klog.Errorf("Invalid type of map value: %+v", val)
		}
	}
	return ""
}

func userComponentPipelineRunForUser(username string) string {
	val, ok := userComponentPipelineRunMap.Load(username)
	if ok {
		componentPipelineRunName, ok2 := val.(string)
		if ok2 {
			return componentPipelineRunName
		} else {
			klog.Errorf("Invalid type of map value: %+v", val)
		}
	}
	return ""
}

func tryNewFramework(username string, user loadtestUtils.User, timeout time.Duration) (*framework.Framework, error) {
	ch := make(chan *framework.Framework)
	var fw *framework.Framework
	var err error
	go func() {
		if stage {
			fw, err = framework.NewFrameworkWithTimeout(
				user.Username,
				time.Minute*60,
				utils.Options{
					ToolchainApiUrl: user.APIURL,
					KeycloakUrl:     user.SSOURL,
					OfflineToken:    user.Token,
				})
		} else {
			fw, err = framework.NewFrameworkWithTimeout(username, time.Minute*60)
		}
		ch <- fw
	}()

	var ret *framework.Framework

	select {
	case result := <-ch:
		ret = result
	case <-time.After(timeout):
		ret = nil
		err = fmt.Errorf("unable to create new framework for user %s within %v", username, timeout)
	}

	return ret, err
}

func userJourneyThread(frameworkMap *sync.Map, threadWaitGroup *sync.WaitGroup, threadIndex int, usersBar *uiprogress.Bar, applicationsBar *uiprogress.Bar, cdqsBar *uiprogress.Bar, componentsBar *uiprogress.Bar, pipelinesBar *uiprogress.Bar, integrationTestsPipelinesBar *uiprogress.Bar, deploymentsBar *uiprogress.Bar) {
	chUsers := make(chan string, numberOfUsers)
	chPipelines := make(chan string, numberOfUsers)
	chIntegrationTestsPipelines := make(chan string, numberOfUsers)
	chDeployments := make(chan string, numberOfUsers)

	defer threadWaitGroup.Done()
	var wg *sync.WaitGroup = &sync.WaitGroup{}

	if waitPipelines {
		if waitIntegrationTestsPipelines {
			if waitDeployments {
				wg.Add(5)
			} else {
				wg.Add(4)
			}
		} else {
			wg.Add(3)
		}
	} else {
		wg.Add(2)
	}

	go func() {
		defer wg.Done()
		for userIndex := 1; userIndex <= numberOfUsers; userIndex++ {
			startTime := time.Now()

			var username string
			if randomString {
				// Create a 5 characters wide random string to be added to username (https://issues.redhat.com/browse/RHTAP-1338)
				randomStr := randomStringFromCharset(5)
				username = fmt.Sprintf("%s-%s-%04d", usernamePrefix, randomStr, threadIndex*numberOfUsers+userIndex)
			} else {
				username = fmt.Sprintf("%s-%04d", usernamePrefix, threadIndex*numberOfUsers+userIndex)
			}

			var user loadtestUtils.User
			var framework *framework.Framework
			var err error
			if stage {
				user = selectedUsers[threadIndex*numberOfUsers+userIndex-1]
				username = user.Username
				framework, err = tryNewFramework(username, user, 60*time.Minute)
			} else {
				framework, err = tryNewFramework(username, user, 60*time.Minute)
			}
			if err != nil {
				logError(1, fmt.Sprintf("Unable to provision user '%s': %v", username, err))
				FailedUserCreationsPerThread[threadIndex] += 1
				increaseBar(usersBar, usersBarMutex)
				continue
			} else {
				frameworkMap.Store(username, framework)
			}

			chUsers <- username

			userCreationTime := time.Since(startTime)
			UserCreationTimeSumPerThread[threadIndex] += userCreationTime
			if userCreationTime > UserCreationTimeMaxPerThread[threadIndex] {
				UserCreationTimeMaxPerThread[threadIndex] = userCreationTime
			}

			SuccessfulUserCreationsPerThread[threadIndex] += 1
			increaseBar(usersBar, usersBarMutex)
		}
		close(chUsers)
	}()

	go func() {
		defer wg.Done()
		for username := range chUsers {
			framework := frameworkForUser(username)
			if framework == nil {
				logError(2, fmt.Sprintf("Framework not found for username %s", username))
				continue
			}
			usernamespace := framework.UserNamespace

			ApplicationName := fmt.Sprintf("%s-app", username)
			startTimeForApplication := time.Now()
			app, err := framework.AsKubeDeveloper.HasController.CreateApplicationWithTimeout(ApplicationName, usernamespace, 60*time.Minute)
			applicationCreationTime := time.Since(startTimeForApplication)
			if err != nil {
				logError(3, fmt.Sprintf("Unable to create the Application %s: %v", ApplicationName, err))
				FailedApplicationCreationsPerThread[threadIndex] += 1
				increaseBar(applicationsBar, applicationsBarMutex)
				continue
			}

			ApplicationCreationTimeSumPerThread[threadIndex] += applicationCreationTime
			if applicationCreationTime > ApplicationCreationTimeMaxPerThread[threadIndex] {
				ApplicationCreationTimeMaxPerThread[threadIndex] = applicationCreationTime
			}
			SuccessfulApplicationCreationsPerThread[threadIndex] += 1

			increaseBar(applicationsBar, applicationsBarMutex)

			gitopsRepoInterval := 5 * time.Second
			gitopsRepoTimeout := 60 * time.Minute
			repoUrl := utils.ObtainGitOpsRepositoryUrl(app.Status.Devfile)
			if err := utils.WaitUntilWithInterval(func() (done bool, err error) {
				resp, err := http.Get(repoUrl)
				if err != nil {
					return false, fmt.Errorf("unable to request gitops repo %s: %+v", repoUrl, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode == 404 {
					return false, nil
				} else if resp.StatusCode == 200 {
					return true, nil
				} else {
					return false, fmt.Errorf("unexpected response code when requesting gitop repo %s: %v", repoUrl, err)
				}
			}, gitopsRepoInterval, gitopsRepoTimeout); err != nil {
				logError(4, fmt.Sprintf("Unable to create application %s gitops repo within %v: %v", ApplicationName, gitopsRepoTimeout, err))
				continue
			}

			/*
				This part adds the Integration test scenario part
				It's considered also as part of the resources creation since Integration test scenarios are resources as well
			*/
			var integrationTestScenario *integrationv1beta1.IntegrationTestScenario
			integrationTestScenario, err = framework.AsKubeDeveloper.IntegrationController.CreateIntegrationTestScenario_beta1(ApplicationName, usernamespace, testScenarioGitURL, testScenarioRevision, testScenarioPathInRepo)
			if err != nil {
				logError(5, fmt.Sprintf("Unable to create integrationTestScenario : %v \n", err))
				continue
			}
			userTestScenarioMap.Store(username, integrationTestScenario.Name)

			ComponentDetectionQueryName := fmt.Sprintf("%s-cdq", username)
			startTimeForCDQ := time.Now()
			cdq, err := framework.AsKubeDeveloper.HasController.CreateComponentDetectionQueryWithTimeout(ComponentDetectionQueryName, usernamespace, componentRepoUrl, "", "", "", false, 60*time.Minute)
			cdqCreationTime := time.Since(startTimeForCDQ)

			if err != nil {
				logError(6, fmt.Sprintf("Unable to create ComponentDetectionQuery %s: %v", ComponentDetectionQueryName, err))
				FailedCDQCreationsPerThread[threadIndex] += 1
				increaseBar(cdqsBar, cdqsBarMutex)
				continue
			}
			if cdq.Name != ComponentDetectionQueryName {
				logError(7, fmt.Sprintf("Actual cdq name (%s) does not match expected (%s): %v", cdq.Name, ComponentDetectionQueryName, err))
				FailedCDQCreationsPerThread[threadIndex] += 1
				increaseBar(cdqsBar, cdqsBarMutex)
				continue
			}
			if len(cdq.Status.ComponentDetected) > 1 {
				logError(8, fmt.Sprintf("cdq (%s) detected more than 1 component", cdq.Name))
				FailedCDQCreationsPerThread[threadIndex] += 1
				increaseBar(cdqsBar, cdqsBarMutex)
				continue
			}

			CDQCreationTimeSumPerThread[threadIndex] += cdqCreationTime
			if cdqCreationTime > CDQCreationTimeMaxPerThread[threadIndex] {
				CDQCreationTimeMaxPerThread[threadIndex] = cdqCreationTime
			}
			SuccessfulCDQCreationsPerThread[threadIndex] += 1

			increaseBar(cdqsBar, cdqsBarMutex)

			for _, compStub := range cdq.Status.ComponentDetected {
				startTimeForComponent := time.Now()
				component, err := framework.AsKubeDeveloper.HasController.CreateComponent(compStub.ComponentStub, usernamespace, "", "", ApplicationName, pipelineSkipInitialChecks, map[string]string{})
				componentCreationTime := time.Since(startTimeForComponent)

				if err != nil {
					logError(9, fmt.Sprintf("Unable to create the Component %s: %v", compStub.ComponentStub.ComponentName, err))
					FailedComponentCreationsPerThread[threadIndex] += 1
					increaseBar(componentsBar, componentsBarMutex)
					continue
				}
				if component.Name != compStub.ComponentStub.ComponentName {
					logError(10, fmt.Sprintf("Actual component name (%s) does not match expected (%s): %v", component.Name, compStub.ComponentStub.ComponentName, err))
					FailedComponentCreationsPerThread[threadIndex] += 1
					increaseBar(componentsBar, componentsBarMutex)
					continue
				}
				userComponentMap.Store(username, component.Name)

				ComponentCreationTimeSumPerThread[threadIndex] += componentCreationTime
				if componentCreationTime > ComponentCreationTimeMaxPerThread[threadIndex] {
					ComponentCreationTimeMaxPerThread[threadIndex] = componentCreationTime
				}
				SuccessfulComponentCreationsPerThread[threadIndex] += 1

				increaseBar(componentsBar, componentsBarMutex)
			}

			chPipelines <- username
		}
		close(chPipelines)
	}()

	if waitPipelines {
		go func() {
			defer wg.Done()
			for username := range chPipelines {
				framework := frameworkForUser(username)
				if framework == nil {
					logError(11, fmt.Sprintf("Framework not found for username %s", username))
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
				usernamespace := framework.UserNamespace
				componentName := componentForUser(username)
				if componentName == "" {
					logError(12, fmt.Sprintf("Component not found for username %s", username))
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
				applicationName := fmt.Sprintf("%s-app", username)
				pipelineCreatedRetryInterval := time.Second * 5
				pipelineCreatedTimeout := time.Minute * 15
				var pipelineRun *v1beta1.PipelineRun
				err := k8swait.PollUntilContextTimeout(context.Background(), pipelineCreatedRetryInterval, pipelineCreatedTimeout, false, func(ctx context.Context) (done bool, err error) {
					pipelineRun, err = framework.AsKubeDeveloper.HasController.GetComponentPipelineRun(componentName, applicationName, usernamespace, "")
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					return true, nil
				})
				if err != nil {
					logError(13, fmt.Sprintf("PipelineRun for applicationName/componentName %s/%s has not been created within %v: %v", applicationName, componentName, pipelineCreatedTimeout, err))
					FailedPipelineRunsPerThread[threadIndex] += 1
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
				userComponentPipelineRunMap.Store(username, pipelineRun.Name)

				pipelineRunRetryInterval := time.Second * 5
				pipelineRunTimeout := time.Minute * 60
				err = k8swait.PollUntilContextTimeout(context.Background(), pipelineRunRetryInterval, pipelineRunTimeout, false, func(ctx context.Context) (done bool, err error) {
					pipelineRun, err = framework.AsKubeDeveloper.HasController.GetComponentPipelineRun(componentName, applicationName, usernamespace, "")
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					if pipelineRun.IsDone() {
						succeededCondition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
						if succeededCondition.IsFalse() {
							dur := pipelineRun.Status.CompletionTime.Sub(pipelineRun.CreationTimestamp.Time)
							PipelineRunFailedTimeSumPerThread[threadIndex] += dur
							logError(14, fmt.Sprintf("Pipeline run for applicationName/componentName %s/%s failed due to %v: %v", applicationName, componentName, succeededCondition.Reason, succeededCondition.Message))
							FailedPipelineRunsPerThread[threadIndex] += 1
						} else {
							dur := pipelineRun.Status.CompletionTime.Sub(pipelineRun.CreationTimestamp.Time)
							PipelineRunSucceededTimeSumPerThread[threadIndex] += dur
							if dur > PipelineRunSucceededTimeMaxPerThread[threadIndex] {
								PipelineRunSucceededTimeMaxPerThread[threadIndex] = dur
							}
							SuccessfulPipelineRunsPerThread[threadIndex] += 1
							chIntegrationTestsPipelines <- username
						}
						increaseBar(pipelinesBar, pipelinesBarMutex)
					}
					return pipelineRun.IsDone(), nil
				})
				if err != nil {
					logError(15, fmt.Sprintf("Pipeline run for applicationName/componentName %s/%s failed to succeed within %v: %v", applicationName, componentName, pipelineRunTimeout, err))
					FailedPipelineRunsPerThread[threadIndex] += 1
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
			}
			close(chIntegrationTestsPipelines)
		}()
	}

	if waitIntegrationTestsPipelines {
		go func() {
			defer wg.Done()
			for username := range chIntegrationTestsPipelines {
				// since username added to chIntegrationTestsPipelinesents only after valid framework, usernamespace, componentName, testScenarioName,
				//  componentPipelineRunName, and applicationName have been created
				//  we don't need to verify validity for neither
				framework := frameworkForUser(username)
				usernamespace := framework.UserNamespace
				componentName := componentForUser(username)
				testScenarioName := testScenarioForUser(username)
				componentPipelineRunName := userComponentPipelineRunForUser(username)
				applicationName := fmt.Sprintf("%s-app", username)
				SnapshotCreatedRetryInterval := time.Second * 5
				SnapshotCreatedTimeout := time.Minute * 30
				IntegrationTestsPipelineCreatedRetryInterval := time.Second * 5
				IntegrationTestsPipelineCreatedTimeout := time.Minute * 15

				var snapshot *appstudioApi.Snapshot
				err := k8swait.PollUntilContextTimeout(context.Background(), SnapshotCreatedRetryInterval, SnapshotCreatedTimeout, false, func(ctx context.Context) (done bool, err error) {
					snapshot, err = framework.AsKubeDeveloper.IntegrationController.GetSnapshot("", componentPipelineRunName, "", usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					return true, nil
				})
				if err != nil {
					logError(16, fmt.Sprintf("Snapshot for applicationName/componentName %s/%s has not been created within %v: %v", applicationName, componentName, SnapshotCreatedTimeout, err))
					FailedIntegrationTestsPipelineRunsPerThread[threadIndex] += 1
					increaseBar(integrationTestsPipelinesBar, integrationTestsPipelinesBarMutex)
					continue
				}

				var IntegrationTestsPipelineRun *v1beta1.PipelineRun
				err = k8swait.PollUntilContextTimeout(context.Background(), IntegrationTestsPipelineCreatedRetryInterval, IntegrationTestsPipelineCreatedTimeout, false, func(ctx context.Context) (done bool, err error) {
					IntegrationTestsPipelineRun, err = framework.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(testScenarioName, snapshot.Name, usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					return true, nil
				})
				if err != nil {
					logError(17, fmt.Sprintf("IntegrationTestPipelineRun for applicationName/testScenarioName/snapshotName %s/%s/%s has not been created within %v: %v", applicationName, testScenarioName, snapshot.Name, IntegrationTestsPipelineCreatedTimeout, err))
					FailedIntegrationTestsPipelineRunsPerThread[threadIndex] += 1
					increaseBar(integrationTestsPipelinesBar, integrationTestsPipelinesBarMutex)
					continue
				}
				IntegrationTestsPipelineRunRetryInterval := time.Second * 5
				IntegrationTestsPipelineRunTimeout := time.Minute * 60
				err = k8swait.PollUntilContextTimeout(context.Background(), IntegrationTestsPipelineRunRetryInterval, IntegrationTestsPipelineRunTimeout, false, func(ctx context.Context) (done bool, err error) {
					IntegrationTestsPipelineRun, err = framework.AsKubeDeveloper.IntegrationController.GetIntegrationPipelineRun(testScenarioName, snapshot.Name, usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					if IntegrationTestsPipelineRun.IsDone() {
						succeededCondition := IntegrationTestsPipelineRun.Status.GetCondition(apis.ConditionSucceeded)
						if succeededCondition.IsFalse() {
							dur := IntegrationTestsPipelineRun.Status.CompletionTime.Sub(IntegrationTestsPipelineRun.CreationTimestamp.Time)
							IntegrationTestsPipelineRunFailedTimeSumPerThread[threadIndex] += dur
							logError(18, fmt.Sprintf("IntegrationTestPipelineRun for applicationName/testScenarioName/snapshotName %s/%s/%s failed due to %v: %v", applicationName, testScenarioName, snapshot.Name, succeededCondition.Reason, succeededCondition.Message))
							FailedIntegrationTestsPipelineRunsPerThread[threadIndex] += 1
						} else {
							dur := IntegrationTestsPipelineRun.Status.CompletionTime.Sub(IntegrationTestsPipelineRun.CreationTimestamp.Time)
							IntegrationTestsPipelineRunSucceededTimeSumPerThread[threadIndex] += dur
							if dur > IntegrationTestsPipelineRunSucceededTimeMaxPerThread[threadIndex] {
								IntegrationTestsPipelineRunSucceededTimeMaxPerThread[threadIndex] = dur
							}
							SuccessfulIntegrationTestsPipelineRunsPerThread[threadIndex] += 1
							chDeployments <- username
						}
						increaseBar(integrationTestsPipelinesBar, integrationTestsPipelinesBarMutex)
					}
					return IntegrationTestsPipelineRun.IsDone(), nil
				})
				if err != nil {
					logError(19, fmt.Sprintf("IntegrationTestPipelineRun for applicationName/testScenarioName/snapshotName %s/%s/%s failed to succeed within %v: %v", applicationName, testScenarioName, snapshot.Name, IntegrationTestsPipelineRunTimeout, err))
					FailedIntegrationTestsPipelineRunsPerThread[threadIndex] += 1
					increaseBar(integrationTestsPipelinesBar, integrationTestsPipelinesBarMutex)
					continue
				}
			}
			close(chDeployments)
		}()
	}

	if waitDeployments {
		go func() {
			defer wg.Done()
			for username := range chDeployments {
				// since username added to chDeployments only after valid framework, usernamespace, componentName, and applicationName have been created
				//  we don't need to verify validity for neither
				framework := frameworkForUser(username)
				usernamespace := framework.UserNamespace
				componentName := componentForUser(username)
				applicationName := fmt.Sprintf("%s-app", username)
				deploymentCreatedRetryInterval := time.Second * 5
				deploymentCreatedTimeout := time.Minute * 15
				var deployment *appsv1.Deployment

				// Deploy the component using gitops and check for the health
				err := k8swait.PollUntilContextTimeout(context.Background(), deploymentCreatedRetryInterval, deploymentCreatedTimeout, false, func(ctx context.Context) (done bool, err error) {
					deployment, err = framework.AsKubeDeveloper.CommonController.GetDeployment(componentName, usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					return true, nil
				})
				if err != nil {
					logError(20, fmt.Sprintf("Deployment for applicationName/componentName %s/%s has not been created within %v: %v", applicationName, componentName, deploymentCreatedTimeout, err))
					FailedDeploymentsPerThread[threadIndex] += 1
					increaseBar(deploymentsBar, deploymentsBarMutex)
					continue
				}

				deploymentRetryIntervalInitialValue := time.Second * 5
				deploymentRetryInterval := deploymentRetryIntervalInitialValue
				deploymentTimeout := time.Minute * 30
				deploymentFailCount := 0
				deploymentFailCountThreshold := 5

				// To avoid race conditions we want to check for deployment failure that occurs more than once to signify failure (deploymentFailCount > 3)
				err = k8swait.PollUntilContextTimeout(context.Background(), deploymentRetryInterval, deploymentTimeout, false, func(ctx context.Context) (done bool, err error) {
					deployment, err = framework.AsKubeDeveloper.CommonController.GetDeployment(componentName, usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}

					creationTimestamp := deployment.ObjectMeta.CreationTimestamp
					deploymentIsDone, lastUpdateTimeOfDone := checkDeploymentIsDone(deployment)

					var (
						deploymentFailed       bool
						errorMessage           string
						lastUpdateTimeOfFailed metav1.Time
					)
					if !deploymentIsDone {
						deploymentFailed, errorMessage, lastUpdateTimeOfFailed = checkDeploymentFailed(deployment)
						if deploymentFailed {
							deploymentFailCount++
							deploymentRetryInterval += 10 // increase retry interval by 5 seconds to give more time for the deploymnt to get ready or to make sure the failure persists
						} else {
							deploymentFailCount = 0
							deploymentRetryInterval = deploymentRetryIntervalInitialValue // reset deploymentRetryInterval to its initial value
						}
					} else {
						deploymentFailCount = 0
						deploymentRetryInterval = deploymentRetryIntervalInitialValue // reset deploymentRetryInterval to its initial value
					}
					if deploymentIsDone {
						dur := lastUpdateTimeOfDone.Time.Sub(creationTimestamp.Time)
						DeploymentSucceededTimeSumPerThread[threadIndex] += dur
						if dur > DeploymentSucceededTimeMaxPerThread[threadIndex] {
							DeploymentSucceededTimeMaxPerThread[threadIndex] = dur
						}
						SuccessfulDeploymentsPerThread[threadIndex] += 1
						increaseBar(deploymentsBar, deploymentsBarMutex)
					} else if deploymentFailed && deploymentFailCount > deploymentFailCountThreshold {
						dur := lastUpdateTimeOfFailed.Time.Sub(creationTimestamp.Time)
						DeploymentFailedTimeSumPerThread[threadIndex] += dur
						logError(21, fmt.Sprintf("Deployment for applicationName/componentName %s/%s failed due to %s", applicationName, componentName, errorMessage))
						FailedDeploymentsPerThread[threadIndex] += 1
						increaseBar(deploymentsBar, deploymentsBarMutex)
					}
					return deploymentIsDone || (deploymentFailed && deploymentFailCount > deploymentFailCountThreshold), nil
				})
				if err != nil {
					logError(22, fmt.Sprintf("Deployment for applicationName/componentName %s/%s failed to succeed within %v: %v", applicationName, componentName, deploymentTimeout, err))
					FailedDeploymentsPerThread[threadIndex] += 1
					increaseBar(deploymentsBar, deploymentsBarMutex)
					continue
				}
			}
		}()
	}
	wg.Wait()
}

func checkDeploymentFailed(deployment *appsv1.Deployment) (bool, string, metav1.Time) {
	var lastUpdateTime metav1.Time = metav1.Now() // initialize with the current time

	// Check if the Deployment is in a stable state
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, "", lastUpdateTime
	}

	/* Iterate over all conditions
		   https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#failed-deployment
	       All the below tests catches all the described failure reasons
	*/
	for _, condition := range deployment.Status.Conditions {
		switch condition.Type {
		case appsv1.DeploymentAvailable:
			if condition.Status == corev1.ConditionFalse {
				return true, fmt.Sprintf("Deployment is not available: %s", condition.Message), condition.LastUpdateTime
			}
		case appsv1.DeploymentProgressing:
			if condition.Status == corev1.ConditionFalse {
				return true, fmt.Sprintf("Deployment failed to progress: %s", condition.Message), condition.LastUpdateTime
			}
		case appsv1.DeploymentReplicaFailure:
			if condition.Status == corev1.ConditionTrue {
				return true, fmt.Sprintf("Deployment failed during rollout: %s", condition.Message), condition.LastUpdateTime
			}
		}
	}

	return false, "", lastUpdateTime
}

func checkDeploymentIsDone(deployment *appsv1.Deployment) (bool, metav1.Time) {
	var deploymentCompleted bool = false
	var deploymentReachedDesiredState bool = false
	var lastUpdateTime metav1.Time = metav1.Now() // initialize with the current time

	// Check if the Deployment is in a stable state
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, lastUpdateTime
	}

	/* Check the DeploymentProgressing condition
	   https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#complete-deployment

	   When the rollout becomes “complete”, the Deployment controller sets a condition with the following attributes to the Deployment's .status.conditions:
	   type: Progressing
	   status: "True"
	   reason: NewReplicaSetAvailable
	*/
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionTrue && condition.Reason == "NewReplicaSetAvailable" {
			// --> Deployment is complete
			deploymentCompleted = true
			lastUpdateTime = condition.LastUpdateTime
			break
		}
	}

	// Check the replica counts
	// The desired number of replicas are running, ready, and have the updated version of the app
	if *deployment.Spec.Replicas == deployment.Status.ReadyReplicas && *deployment.Spec.Replicas == deployment.Status.UpdatedReplicas {
		// --> Deployment has achieved desired state
		deploymentReachedDesiredState = true
	}

	return deploymentCompleted && deploymentReachedDesiredState, lastUpdateTime
}
