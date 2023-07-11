package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/codeready-toolchain/toolchain-e2e/setup/auth"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics/queries"
	"github.com/codeready-toolchain/toolchain-e2e/setup/terminal"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uitable/util/strutil"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
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
	componentRepoUrl          string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
	usernamePrefix            string = "testuser"
	numberOfUsers             int
	waitPipelines             bool
	waitDeployments           bool
	verbose                   bool
	token                     string
	logConsole                bool
	failFast                  bool
	disableMetrics            bool
	threadCount               int
	pipelineSkipInitialChecks bool
)

var (
	UserCreationTimeMaxPerThread         []time.Duration
	ResourceCreationTimeMaxPerThread     []time.Duration
	PipelineRunSucceededTimeMaxPerThread []time.Duration
	DeploymentSucceededTimeMaxPerThread  []time.Duration
	UserCreationTimeSumPerThread         []time.Duration
	ResourceCreationTimeSumPerThread     []time.Duration
	PipelineRunSucceededTimeSumPerThread []time.Duration
	PipelineRunFailedTimeSumPerThread    []time.Duration
	DeploymentSucceededTimeSumPerThread  []time.Duration
	DeploymentFailedTimeSumPerThread     []time.Duration
	SuccessfulUserCreationsPerThread     []int64
	SuccessfulResourceCreationsPerThread []int64
	SuccessfulPipelineRunsPerThread      []int64
	SuccessfulDeploymentsPerThread       []int64
	FailedUserCreationsPerThread         []int64
	FailedResourceCreationsPerThread     []int64
	FailedPipelineRunsPerThread          []int64
	FailedDeploymentsPerThread           []int64
	frameworkMap                         *sync.Map
	userComponentMap                     *sync.Map
	errorCountMap                        map[int]ErrorCount
	errorMutex                           = &sync.Mutex{}
	usersBarMutex                        = &sync.Mutex{}
	resourcesBarMutex                    = &sync.Mutex{}
	pipelinesBarMutex                    = &sync.Mutex{}
	deploymentsBarMutex                  = &sync.Mutex{}
	threadsWG                            *sync.WaitGroup
	logData                              LogData
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
	Timestamp                         string            `json:"timestamp"`
	EndTimestamp                      string            `json:"endTimestamp"`
	MachineName                       string            `json:"machineName"`
	BinaryDetails                     string            `json:"binaryDetails"`
	ComponentRepoUrl                  string            `json:"componentRepoUrl"`
	NumberOfThreads                   int               `json:"threads"`
	NumberOfUsersPerThread            int               `json:"usersPerThread"`
	NumberOfUsers                     int               `json:"totalUsers"`
	PipelineSkipInitialChecks         bool              `json:"pipelineSkipInitialChecks"`
	LoadTestCompletionStatus          string            `json:"status"`
	AverageTimeToSpinUpUsers          float64           `json:"createUserTimeAvg"`
	MaxTimeToSpinUpUsers              float64           `json:"createUserTimeMax"`
	AverageTimeToCreateResources      float64           `json:"createResourcesTimeAvg"`
	MaxTimeToCreateResources          float64           `json:"createResourcesTimeMax"`
	AverageTimeToRunPipelineSucceeded float64           `json:"runPipelineSucceededTimeAvg"`
	MaxTimeToRunPipelineSucceeded     float64           `json:"runPipelineSucceededTimeMax"`
	AverageTimeToRunPipelineFailed    float64           `json:"runPipelineFailedTimeAvg"`
	AverageTimeToDeploymentSucceeded  float64           `json:"DeploymentSucceededTimeAvg"`
	MaxTimeToDeploymentSucceeded      float64           `json:"DeploymentSucceededTimeMax"`
	AverageTimeToDeploymentFailed     float64           `json:"DeploymentFailedTimeAvg"`
	UserCreationFailureCount          int64             `json:"createUserFailures"`
	UserCreationFailureRate           float64           `json:"createUserFailureRate"`
	ResourceCreationFailureCount      int64             `json:"createResourcesFailures"`
	ResourceCreationFailureRate       float64           `json:"createResourcesFailureRate"`
	PipelineRunFailureCount           int64             `json:"runPipelineFailures"`
	PipelineRunFailureRate            float64           `json:"runPipelineFailureRate"`
	DeploymentFailureCount            int64             `json:"DeploymentFailures"`
	DeploymentFailureRate             float64           `json:"DeploymentFailureRate"`
	ErrorCounts                       []ErrorCount      `json:"errorCounts"`
	Errors                            []ErrorOccurrence `json:"errors"`
	ErrorsTotal                       int               `json:"errorsTotal"`
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

func ExecuteLoadTest() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&componentRepoUrl, "component-repo", componentRepoUrl, "the component repo URL to be used")
	rootCmd.Flags().StringVar(&usernamePrefix, "username", usernamePrefix, "the prefix used for usersignup names")
	// TODO use a custom kubeconfig and introduce debug logging and trace
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "if 'debug' traces should be displayed in the console")
	rootCmd.Flags().IntVarP(&numberOfUsers, "users", "u", 5, "the number of user accounts to provision per thread")
	rootCmd.Flags().BoolVarP(&waitPipelines, "waitpipelines", "w", false, "if you want to wait for pipelines to finish")
	rootCmd.Flags().BoolVarP(&waitDeployments, "waitdeployments", "d", false, "if you want to wait for deployments to finish")
	rootCmd.Flags().BoolVarP(&logConsole, "log-to-console", "l", false, "if you want to log to console in addition to the log file")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "if you want the test to fail fast at first failure")
	rootCmd.Flags().BoolVar(&disableMetrics, "disable-metrics", false, "if you want to disable metrics gathering")
	rootCmd.Flags().IntVarP(&threadCount, "threads", "t", 1, "number of concurrent threads to execute")
	rootCmd.Flags().BoolVar(&pipelineSkipInitialChecks, "pipeline-skip-initial-checks", true, "if pipeline runs' initial checks are to be skipped")
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
	term := terminal.New(cmd.InOrStdin, cmd.OutOrStdout, verbose)

	// waitDeployments is valid only if waitPipelines
	if waitDeployments && !waitPipelines {
		klog.Fatalf("Error: The --waitdeployments flag requires the --waitpipelines flag.")
	}

	logFile, err := os.Create("load-tests.log")
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
	globalframework, err := framework.NewFramework("load-tests")
	if err != nil {
		klog.Fatalf("error creating client-go %v", err)
	}

	if len(token) == 0 {
		token, err = auth.GetTokenFromOC()
		if err != nil {
			tokenRequestURI, err := auth.GetTokenRequestURI(globalframework.AsKubeAdmin.CommonController.KubeRest())
			if err != nil {
				klog.Fatalf("a token is required to capture metrics, use oc login to log into the cluster: %v", err)
			}
			klog.Fatalf("a token is required to capture metrics, use oc login to log into the cluster. alternatively request a token and use the token flag: %v", tokenRequestURI)
		}
	}

	var stopMetrics chan struct{}
	var metricsInstance *metrics.Gatherer
	if !disableMetrics {
		metricsInstance = metrics.NewEmpty(term, globalframework.AsKubeAdmin.CommonController.KubeRest(), 10*time.Minute)
		prometheusClient := metrics.GetPrometheusClient(term, globalframework.AsKubeAdmin.CommonController.KubeRest(), token)

		metricsInstance.AddQueries(
			queries.QueryClusterCPUUtilisation(prometheusClient),
			queries.QueryClusterMemoryUtilisation(prometheusClient),
			queries.QueryNodeMemoryUtilisation(prometheusClient),
			queries.QueryEtcdMemoryUsage(prometheusClient),
			queries.QueryWorkloadCPUUsage(prometheusClient, constants.OLMOperatorNamespace, constants.OLMOperatorWorkload),
			queries.QueryWorkloadMemoryUsage(prometheusClient, constants.OLMOperatorNamespace, constants.OLMOperatorWorkload),
			queries.QueryOpenshiftKubeAPIMemoryUtilisation(prometheusClient),
			queries.QueryWorkloadCPUUsage(prometheusClient, constants.OSAPIServerNamespace, constants.OSAPIServerWorkload),
			queries.QueryWorkloadMemoryUsage(prometheusClient, constants.OSAPIServerNamespace, constants.OSAPIServerWorkload),
			queries.QueryWorkloadCPUUsage(prometheusClient, constants.HostOperatorNamespace, constants.HostOperatorWorkload),
			queries.QueryWorkloadMemoryUsage(prometheusClient, constants.HostOperatorNamespace, constants.HostOperatorWorkload),
			queries.QueryWorkloadCPUUsage(prometheusClient, constants.MemberOperatorNamespace, constants.MemberOperatorWorkload),
			queries.QueryWorkloadMemoryUsage(prometheusClient, constants.MemberOperatorNamespace, constants.MemberOperatorWorkload),
			queries.QueryWorkloadCPUUsage(prometheusClient, "application-service", "application-service-application-service-controller-manager"),
			queries.QueryWorkloadMemoryUsage(prometheusClient, "application-service", "application-service-application-service-controller-manager"),
			queries.QueryWorkloadCPUUsage(prometheusClient, "build-service", "build-service-controller-manager"),
			queries.QueryWorkloadMemoryUsage(prometheusClient, "build-service", "build-service-controller-manager"),
		)
		stopMetrics = metricsInstance.StartGathering()

		klog.Infof("Sleeping till all metrics queries gets init")
		time.Sleep(time.Second * 10)
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

	AppStudioUsersBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Users (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedUserCreationsPerThread)), barLength, ' ')
	})

	ResourcesBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio User Resources (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedResourceCreationsPerThread)), barLength, ' ')
	})

	PipelinesBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Waiting for pipelines to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedPipelineRunsPerThread)), barLength, ' ')
	})

	DeploymentsBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Waiting for deployments to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedDeploymentsPerThread)), barLength, ' ')
	})

	UserCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	ResourceCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	PipelineRunSucceededTimeMaxPerThread = make([]time.Duration, threadCount)
	DeploymentSucceededTimeMaxPerThread = make([]time.Duration, threadCount)

	UserCreationTimeSumPerThread = make([]time.Duration, threadCount)
	ResourceCreationTimeSumPerThread = make([]time.Duration, threadCount)
	PipelineRunSucceededTimeSumPerThread = make([]time.Duration, threadCount)
	PipelineRunFailedTimeSumPerThread = make([]time.Duration, threadCount)
	DeploymentSucceededTimeSumPerThread = make([]time.Duration, threadCount)
	DeploymentFailedTimeSumPerThread = make([]time.Duration, threadCount)

	SuccessfulUserCreationsPerThread = make([]int64, threadCount)
	SuccessfulResourceCreationsPerThread = make([]int64, threadCount)
	SuccessfulPipelineRunsPerThread = make([]int64, threadCount)
	SuccessfulDeploymentsPerThread = make([]int64, threadCount)

	FailedUserCreationsPerThread = make([]int64, threadCount)
	FailedResourceCreationsPerThread = make([]int64, threadCount)
	FailedPipelineRunsPerThread = make([]int64, threadCount)
	FailedDeploymentsPerThread = make([]int64, threadCount)

	frameworkMap = &sync.Map{}
	userComponentMap = &sync.Map{}
	errorCountMap = make(map[int]ErrorCount)

	rand.Seed(time.Now().UnixNano())

	threadsWG = &sync.WaitGroup{}
	threadsWG.Add(threadCount)
	for thread := 0; thread < threadCount; thread++ {
		go userJourneyThread(frameworkMap, threadsWG, thread, AppStudioUsersBar, ResourcesBar, PipelinesBar, DeploymentsBar)
	}

	// Todo add cleanup functions that will delete user signups

	threadsWG.Wait()
	uip.Stop()

	logData.EndTimestamp = time.Now().Format("2006-01-02T15:04:05Z07:00")
	logData.LoadTestCompletionStatus = "Completed"

	userCreationFailureCount := sumFromArray(FailedUserCreationsPerThread)
	logData.UserCreationFailureCount = userCreationFailureCount

	averageTimeToSpinUpUsers := float64(0)
	userCreationSuccessCount := sumFromArray(SuccessfulUserCreationsPerThread)
	if userCreationSuccessCount > 0 {
		averageTimeToSpinUpUsers = sumDurationFromArray(UserCreationTimeSumPerThread).Seconds() / float64(userCreationSuccessCount)
	}
	logData.AverageTimeToSpinUpUsers = averageTimeToSpinUpUsers
	logData.MaxTimeToSpinUpUsers = maxDurationFromArray(UserCreationTimeMaxPerThread).Seconds()

	resourceCreationFailureCount := sumFromArray(FailedResourceCreationsPerThread)
	logData.ResourceCreationFailureCount = resourceCreationFailureCount

	averageTimeToCreateResources := float64(0)
	resourceCreationSuccessCount := sumFromArray(SuccessfulResourceCreationsPerThread)
	if resourceCreationSuccessCount > 0 {
		averageTimeToCreateResources = sumDurationFromArray(ResourceCreationTimeSumPerThread).Seconds() / float64(resourceCreationSuccessCount)
	}
	logData.AverageTimeToCreateResources = averageTimeToCreateResources
	logData.MaxTimeToCreateResources = maxDurationFromArray(ResourceCreationTimeMaxPerThread).Seconds()

	pipelineRunFailureCount := sumFromArray(FailedPipelineRunsPerThread)
	logData.PipelineRunFailureCount = pipelineRunFailureCount

	deploymentFailureCount := sumFromArray(FailedDeploymentsPerThread)
	logData.DeploymentFailureCount = deploymentFailureCount

	averageTimeToRunPipelineSucceeded := float64(0)
	pipelineRunSuccessCount := sumFromArray(SuccessfulPipelineRunsPerThread)
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

	averageTimeToDeploymentSucceeded := float64(0)
	deploymentSuccessCount := sumFromArray(SuccessfulDeploymentsPerThread)
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

	userCreationFailureRate := float64(userCreationFailureCount) / float64(overallCount)
	logData.UserCreationFailureRate = userCreationFailureRate

	resourceCreationFailureRate := float64(resourceCreationFailureCount) / float64(overallCount)
	logData.ResourceCreationFailureRate = resourceCreationFailureRate

	pipelineRunFailureRate := float64(pipelineRunFailureCount) / float64(overallCount)
	logData.PipelineRunFailureRate = pipelineRunFailureRate

	deploymentFailureRate := float64(deploymentFailureCount) / float64(overallCount)
	logData.DeploymentFailureRate = deploymentFailureRate

	klog.Infof("🏁 Load Test Completed!")
	klog.Infof("📈 Results 📉")
	klog.Infof("Average Time to spin up users: %.2f s", averageTimeToSpinUpUsers)
	klog.Infof("Maximal Time to spin up users: %.2f s", logData.MaxTimeToSpinUpUsers)
	klog.Infof("Average Time to create Resources: %.2f s", averageTimeToCreateResources)
	klog.Infof("Maximal Time to create Resources: %.2f s", logData.MaxTimeToCreateResources)
	klog.Infof("Average Time to run Pipelines successfully: %.2f s", averageTimeToRunPipelineSucceeded)
	klog.Infof("Maximal Time to run Pipelines successfully: %.2f s", logData.MaxTimeToRunPipelineSucceeded)
	klog.Infof("Average Time to fail Pipelines: %.2f s", averageTimeToRunPipelineFailed)
	klog.Infof("Average Time to deploy component successfully: %.2f s", averageTimeToDeploymentSucceeded)
	klog.Infof("Maximal Time to deploy component successfully: %.2f s", logData.MaxTimeToDeploymentSucceeded)
	klog.Infof("Average Time to fail Deployments: %.2f s", averageTimeToDeploymentFailed)
	klog.Infof("Number of times user creation failed: %d (%.2f %%)", userCreationFailureCount, userCreationFailureRate*100)
	klog.Infof("Number of times resource creation failed: %d (%.2f %%)", resourceCreationFailureCount, resourceCreationFailureRate*100)
	klog.Infof("Number of times pipeline run failed: %d (%.2f %%)", pipelineRunFailureCount, pipelineRunFailureRate*100)
	klog.Infof("Number of times deployment failed: %d (%.2f %%)", deploymentFailureCount, deploymentFailureRate*100)
	klog.Infoln("Error summary:")
	for _, errorCount := range errorCountMap {
		klog.Infof("Number of error #%d occured: %d", errorCount.ErrorCode, errorCount.Count)
		logData.ErrorCounts = append(logData.ErrorCounts, errorCount)
	}
	logData.ErrorsTotal = len(logData.Errors)
	klog.Infof("Total number of errors occured: %d", logData.ErrorsTotal)

	err = createLogDataJSON("load-tests.json", logData)
	if err != nil {
		klog.Errorf("error while marshalling JSON: %v\n", err)
	}

	klog.StopFlushDaemon()
	klog.Flush()
	if !disableMetrics {
		defer close(stopMetrics)
		metricsInstance.PrintResults()
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
	mutex.Lock()
	defer mutex.Unlock()
	bar.Incr()
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

func tryNewFramework(username string, timeout time.Duration) (*framework.Framework, error) {
	ch := make(chan *framework.Framework)
	var fw *framework.Framework
	var err error
	go func() {
		fw, err = framework.NewFrameworkWithTimeout(username, time.Minute*60)
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

func userJourneyThread(frameworkMap *sync.Map, threadWaitGroup *sync.WaitGroup, threadIndex int, usersBar *uiprogress.Bar, resourcesBar *uiprogress.Bar, pipelinesBar *uiprogress.Bar, deploymentsBar *uiprogress.Bar) {
	chUsers := make(chan string, numberOfUsers)
	chPipelines := make(chan string, numberOfUsers)
	chDeployments := make(chan string, numberOfUsers)

	defer threadWaitGroup.Done()

	var wg *sync.WaitGroup = &sync.WaitGroup{}

	if waitPipelines {
		if waitDeployments {
			wg.Add(4)
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
			username := fmt.Sprintf("%s-%04d", usernamePrefix, threadIndex*numberOfUsers+userIndex)
			framework, err := tryNewFramework(username, 60*time.Minute)
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
			startTime := time.Now()
			framework := frameworkForUser(username)
			if framework == nil {
				logError(2, fmt.Sprintf("Framework not found for username %s", username))
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			usernamespace := framework.UserNamespace
			ApplicationName := fmt.Sprintf("%s-app", username)
			app, err := framework.AsKubeDeveloper.HasController.CreateApplicationWithTimeout(ApplicationName, usernamespace, 60*time.Minute)
			if err != nil {
				logError(4, fmt.Sprintf("Unable to create the Application %s: %v", ApplicationName, err))
				FailedResourceCreationsPerThread[threadIndex] += 1
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
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
				logError(5, fmt.Sprintf("Unable to create application %s gitops repo within %v: %v", ApplicationName, gitopsRepoTimeout, err))
				FailedResourceCreationsPerThread[threadIndex] += 1
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}

			ComponentDetectionQueryName := fmt.Sprintf("%s-cdq", username)
			cdq, err := framework.AsKubeDeveloper.HasController.CreateComponentDetectionQueryWithTimeout(ComponentDetectionQueryName, usernamespace, componentRepoUrl, "", "", "", false, 60*time.Minute)
			if err != nil {
				logError(6, fmt.Sprintf("Unable to create ComponentDetectionQuery %s: %v", ComponentDetectionQueryName, err))
				FailedResourceCreationsPerThread[threadIndex] += 1
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			if cdq.Name != ComponentDetectionQueryName {
				logError(7, fmt.Sprintf("Actual cdq name (%s) does not match expected (%s): %v", cdq.Name, ComponentDetectionQueryName, err))
				FailedResourceCreationsPerThread[threadIndex] += 1
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			if len(cdq.Status.ComponentDetected) > 1 {
				logError(8, fmt.Sprintf("cdq (%s) detected more than 1 component", cdq.Name))
				FailedResourceCreationsPerThread[threadIndex] += 1
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}

			for _, compStub := range cdq.Status.ComponentDetected {
				component, err := framework.AsKubeDeveloper.HasController.CreateComponent(compStub.ComponentStub, usernamespace, "", "", ApplicationName, pipelineSkipInitialChecks, map[string]string{})

				if err != nil {
					logError(9, fmt.Sprintf("Unable to create the Component %s: %v", compStub.ComponentStub.ComponentName, err))
					FailedResourceCreationsPerThread[threadIndex] += 1
					increaseBar(resourcesBar, resourcesBarMutex)
					continue
				}
				if component.Name != compStub.ComponentStub.ComponentName {
					logError(10, fmt.Sprintf("Actual component name (%s) does not match expected (%s): %v", component.Name, compStub.ComponentStub.ComponentName, err))
					FailedResourceCreationsPerThread[threadIndex] += 1
					increaseBar(resourcesBar, resourcesBarMutex)
					continue
				}
				userComponentMap.Store(username, component.Name)
			}

			resourceCreationTime := time.Since(startTime)
			ResourceCreationTimeSumPerThread[threadIndex] += resourceCreationTime
			if resourceCreationTime > ResourceCreationTimeMaxPerThread[threadIndex] {
				ResourceCreationTimeMaxPerThread[threadIndex] = resourceCreationTime
			}
			SuccessfulResourceCreationsPerThread[threadIndex] += 1

			chPipelines <- username
			increaseBar(resourcesBar, resourcesBarMutex)
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
				err := k8swait.Poll(pipelineCreatedRetryInterval, pipelineCreatedTimeout, func() (done bool, err error) {
					pipelineRun, err = framework.AsKubeDeveloper.HasController.GetComponentPipelineRun(componentName, applicationName, usernamespace, "")
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					return true, nil
				})
				if err != nil {
					logError(13, fmt.Sprintf("PipelineRun for %s/%s has not been created within %v: %v", applicationName, componentName, pipelineCreatedTimeout, err))
					FailedPipelineRunsPerThread[threadIndex] += 1
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
				pipelineRunRetryInterval := time.Second * 5
				pipelineRunTimeout := time.Minute * 60
				err = k8swait.Poll(pipelineRunRetryInterval, pipelineRunTimeout, func() (done bool, err error) {
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
							logError(14, fmt.Sprintf("Pipeline run for %s/%s failed due to %v: %v", applicationName, componentName, succeededCondition.Reason, succeededCondition.Message))
							FailedPipelineRunsPerThread[threadIndex] += 1
						} else {
							dur := pipelineRun.Status.CompletionTime.Sub(pipelineRun.CreationTimestamp.Time)
							PipelineRunSucceededTimeSumPerThread[threadIndex] += dur
							if dur > PipelineRunSucceededTimeMaxPerThread[threadIndex] {
								PipelineRunSucceededTimeMaxPerThread[threadIndex] = dur
							}
							SuccessfulPipelineRunsPerThread[threadIndex] += 1
							chDeployments <- username
						}
						increaseBar(pipelinesBar, pipelinesBarMutex)
					}
					return pipelineRun.IsDone(), nil
				})
				if err != nil {
					logError(15, fmt.Sprintf("Pipeline run for %s/%s failed to succeed within %v: %v", applicationName, componentName, pipelineRunTimeout, err))
					FailedPipelineRunsPerThread[threadIndex] += 1
					increaseBar(pipelinesBar, pipelinesBarMutex)
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
				err := k8swait.Poll(deploymentCreatedRetryInterval, deploymentCreatedTimeout, func() (done bool, err error) {
					deployment, err = framework.AsKubeDeveloper.CommonController.GetDeployment(componentName, usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					return true, nil
				})
				if err != nil {
					logError(16, fmt.Sprintf("Deployment for %s/%s has not been created within %v: %v", applicationName, componentName, deploymentCreatedTimeout, err))
					FailedDeploymentsPerThread[threadIndex] += 1
					increaseBar(deploymentsBar, deploymentsBarMutex)
					continue
				}

				deploymentRetryInterval := time.Second * 5
				deploymentTimeout := time.Minute * 25
				deploymentPauseToMitigateRaceConditions := time.Second * 10

				err = k8swait.Poll(deploymentRetryInterval, deploymentTimeout, func() (done bool, err error) {
					deployment, err = framework.AsKubeDeveloper.CommonController.GetDeployment(componentName, usernamespace)
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}

					// Pause briefly to allow any ongoing updates to the Deployment to complete
					time.Sleep(deploymentPauseToMitigateRaceConditions)

					creationTimestamp := deployment.ObjectMeta.CreationTimestamp
					deploymentIsDone, completionTime := checkDeploymentIsDone(deployment)

					var (
						deploymentFailed   bool
						errorMessage       string
						lastTransitionTime metav1.Time
					)
					if !deploymentIsDone {
						deploymentFailed, errorMessage, lastTransitionTime = checkDeploymentFailed(deployment)
					}
					if deploymentIsDone {
						dur := completionTime.Time.Sub(creationTimestamp.Time)
						DeploymentSucceededTimeSumPerThread[threadIndex] += dur
						if dur > DeploymentSucceededTimeMaxPerThread[threadIndex] {
							DeploymentSucceededTimeMaxPerThread[threadIndex] = dur
						}
						SuccessfulDeploymentsPerThread[threadIndex] += 1
						increaseBar(deploymentsBar, deploymentsBarMutex)
					} else if deploymentFailed {
						dur := lastTransitionTime.Time.Sub(creationTimestamp.Time)
						DeploymentFailedTimeSumPerThread[threadIndex] += dur
						logError(17, fmt.Sprintf("Deployment for %s/%s failed due to %s", applicationName, componentName, errorMessage))
						FailedDeploymentsPerThread[threadIndex] += 1
						increaseBar(deploymentsBar, deploymentsBarMutex)
					}
					return deploymentIsDone || deploymentFailed, nil
				})
				if err != nil {
					logError(18, fmt.Sprintf("Deployment for %s/%s failed to succeed within %v: %v", applicationName, componentName, deploymentTimeout, err))
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
	var lastTransitionTime metav1.Time = metav1.Now() // initialize with the current time

	// Check if the Deployment is in a stable state
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, "", lastTransitionTime
	}

	// Check if the Deployment is unable to progress
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse {
			lastTransitionTime = condition.LastTransitionTime
			return true, fmt.Sprintf("Deployment failed to progress: %s", condition.Message), lastTransitionTime
		}
	}

	// Check if the Deployment couldn't create the required number of pods
	if deployment.Spec.Replicas != nil && deployment.Status.AvailableReplicas < *deployment.Spec.Replicas {
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionFalse {
				lastTransitionTime = condition.LastTransitionTime
				break
			}
		}
		return true, fmt.Sprintf("Deployment failed to create required pods: wanted %d, have %d", *deployment.Spec.Replicas, deployment.Status.AvailableReplicas), lastTransitionTime
	}

	// Check for errors during rollout
	if deployment.Status.UpdatedReplicas < deployment.Status.Replicas {
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentReplicaFailure {
				lastTransitionTime = condition.LastTransitionTime
				break
			}
		}
		return true, fmt.Sprintf("Deployment failed during rollout: updated %d, total %d", deployment.Status.UpdatedReplicas, deployment.Status.Replicas), lastTransitionTime
	}

	return false, "", lastTransitionTime
}

func checkDeploymentIsDone(deployment *appsv1.Deployment) (bool, metav1.Time) {
	var deploymentAvailable bool = false
	var deploymentReachedDesiredState bool = false
	var CompletionTime metav1.Time = metav1.Now() // initialize with the current time

	// Check if the Deployment is in a stable state
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, CompletionTime
	}

	// Check the DeploymentAvailable condition
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			// Available condition is true
			// --> Deployment is available
			deploymentAvailable = true
			CompletionTime = condition.LastTransitionTime
			break
		}
	}

	// Check the replica counts
	if *deployment.Spec.Replicas == deployment.Status.ReadyReplicas && *deployment.Spec.Replicas == deployment.Status.UpdatedReplicas {
		// The desired number of replicas are running, ready, and have the updated version of the app
		// --> Deployment has achieved desired state
		deploymentReachedDesiredState = true
	}

	return deploymentAvailable && deploymentReachedDesiredState, CompletionTime
}
