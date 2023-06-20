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

	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uitable/util/strutil"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	loadtestUtils "github.com/redhat-appstudio/e2e-tests/pkg/utils/loadtests"
	"github.com/spf13/cobra"
	metrics "github.com/redhat-appstudio-qe/perf-monitoring/metrics"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
	verbose                   bool
	stage					  bool
	logConsole                bool
	failFast                  bool
	threadCount               int
	pipelineSkipInitialChecks bool
)

var (
	UserCreationTimeMaxPerThread         []time.Duration
	ResourceCreationTimeMaxPerThread     []time.Duration
	PipelineRunSucceededTimeMaxPerThread []time.Duration
	UserCreationTimeSumPerThread         []time.Duration
	ResourceCreationTimeSumPerThread     []time.Duration
	PipelineRunSucceededTimeSumPerThread []time.Duration
	PipelineRunFailedTimeSumPerThread    []time.Duration
	SuccessfulUserCreationsPerThread     []int64
	SuccessfulResourceCreationsPerThread []int64
	SuccessfulPipelineRunsPerThread      []int64
	FailedUserCreationsPerThread         []int64
	FailedResourceCreationsPerThread     []int64
	FailedPipelineRunsPerThread          []int64
	frameworkMap                         *sync.Map
	userComponentMap                     *sync.Map
	errorCountMap                        map[int]ErrorCount
	errorMutex                           = &sync.Mutex{}
	usersBarMutex                        = &sync.Mutex{}
	resourcesBarMutex                    = &sync.Mutex{}
	pipelinesBarMutex                    = &sync.Mutex{}
	threadsWG                            *sync.WaitGroup
	logData                              LogData
	stageUsers 							[]loadtestUtils.User
	selectedUsers 						[]loadtestUtils.User
	CI 									bool
	JobName 							string
	metricsController					*metrics.MetricsPush
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
	UserCreationFailureCount          int64             `json:"createUserFailures"`
	UserCreationFailureRate           float64           `json:"createUserFailureRate"`
	ResourceCreationFailureCount      int64             `json:"createResourcesFailures"`
	ResourceCreationFailureRate       float64           `json:"createResourcesFailureRate"`
	PipelineRunFailureCount           int64             `json:"runPipelineFailures"`
	PipelineRunFailureRate            float64           `json:"runPipelineFailureRate"`
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
	rootCmd.Flags().BoolVarP(&stage, "stage", "s", false, "is you want to run the test on stage")
	rootCmd.Flags().IntVarP(&numberOfUsers, "users", "u", 5, "the number of user accounts to provision per thread")
	rootCmd.Flags().BoolVarP(&waitPipelines, "waitpipelines", "w", false, "if you want to wait for pipelines to finish")
	rootCmd.Flags().BoolVarP(&logConsole, "log-to-console", "l", false, "if you want to log to console in addition to the log file")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "if you want the test to fail fast at first failure")
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
	
	//Job Name to Store Metrics Captured During Test
	JobName = loadtestUtils.GetJobName()
	metricsController = metrics.NewMetricController(constants.LoadTestsIngesterURL, JobName)
	metricsController.InitPusher()	
	
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
	metricsController.PushMetricsTotal(float64(overallCount))

	klog.Infof("Number of threads: %d", threadCount)
	klog.Infof("Number of users per thread: %d", numberOfUsers)
	klog.Infof("Number of users overall: %d", overallCount)
	klog.Infof("Pipeline run initial checks skipped: %t", pipelineSkipInitialChecks)

	klog.Infof("🕖 initializing...\n")

	if(stage){
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

	AppStudioUsersBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Users (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedUserCreationsPerThread)), barLength, ' ')
	})

	ResourcesBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio User Resources (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedResourceCreationsPerThread)), barLength, ' ')
	})

	PipelinesBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Waiting for pipelines to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedPipelineRunsPerThread)), barLength, ' ')
	})

	UserCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	ResourceCreationTimeMaxPerThread = make([]time.Duration, threadCount)
	PipelineRunSucceededTimeMaxPerThread = make([]time.Duration, threadCount)
	UserCreationTimeSumPerThread = make([]time.Duration, threadCount)
	ResourceCreationTimeSumPerThread = make([]time.Duration, threadCount)
	PipelineRunSucceededTimeSumPerThread = make([]time.Duration, threadCount)
	PipelineRunFailedTimeSumPerThread = make([]time.Duration, threadCount)
	SuccessfulUserCreationsPerThread = make([]int64, threadCount)
	SuccessfulResourceCreationsPerThread = make([]int64, threadCount)
	SuccessfulPipelineRunsPerThread = make([]int64, threadCount)
	FailedUserCreationsPerThread = make([]int64, threadCount)
	FailedResourceCreationsPerThread = make([]int64, threadCount)
	FailedPipelineRunsPerThread = make([]int64, threadCount)
	frameworkMap = &sync.Map{}
	userComponentMap = &sync.Map{}
	errorCountMap = make(map[int]ErrorCount)

	rand.Seed(time.Now().UnixNano())

	threadsWG = &sync.WaitGroup{}
	threadsWG.Add(threadCount)
	for thread := 0; thread < threadCount; thread++ {
		go userJourneyThread(frameworkMap, threadsWG, thread, AppStudioUsersBar, ResourcesBar, PipelinesBar)
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

	userCreationFailureRate := float64(userCreationFailureCount) / float64(overallCount)
	logData.UserCreationFailureRate = userCreationFailureRate

	resourceCreationFailureRate := float64(resourceCreationFailureCount) / float64(overallCount)
	logData.ResourceCreationFailureRate = resourceCreationFailureRate

	pipelineRunFailureRate := float64(pipelineRunFailureCount) / float64(overallCount)
	logData.PipelineRunFailureRate = pipelineRunFailureRate

	klog.Infof("🏁 Load Test Completed!")
	if stage{
		StageCleanup(selectedUsers)
	}
	klog.Infof("📈 Results 📉")
	klog.Infof("Average Time to spin up users: %.2f s", averageTimeToSpinUpUsers)
	klog.Infof("Maximal Time to spin up users: %.2f s", logData.MaxTimeToSpinUpUsers)
	klog.Infof("Average Time to create Resources: %.2f s", averageTimeToCreateResources)
	klog.Infof("Maximal Time to create Resources: %.2f s", logData.MaxTimeToCreateResources)
	klog.Infof("Average Time to run Pipelines successfully: %.2f s", averageTimeToRunPipelineSucceeded)
	klog.Infof("Maximal Time to run Pipelines successfully: %.2f s", logData.MaxTimeToRunPipelineSucceeded)
	klog.Infof("Average Time to fail Pipelines: %.2f s", averageTimeToRunPipelineFailed)
	klog.Infof("Number of times user creation failed: %d (%.2f %%)", userCreationFailureCount, userCreationFailureRate*100)
	klog.Infof("Number of times resource creation failed: %d (%.2f %%)", resourceCreationFailureCount, resourceCreationFailureRate*100)
	klog.Infof("Number of times pipeline run failed: %d (%.2f %%)", pipelineRunFailureCount, pipelineRunFailureRate*100)
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
}

func StageCleanup(users []loadtestUtils.User){

	for _, user := range users {
		framework := frameworkForUser(user.Username)
		err := framework.AsKubeDeveloper.HasController.DeleteAllApplicationsInASpecificNamespace(framework.UserNamespace, 60*time.Minute)
		if err!= nil{
			klog.Errorf("while deleting resources for user: %s, got error: %v\n",user.Username, err)
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

func tryNewFramework(username string, user loadtestUtils.User,  timeout time.Duration) (*framework.Framework, error) {
	ch := make(chan *framework.Framework)
	var fw *framework.Framework
	var err error
	go func() {
		if stage {
			fw, err = framework.NewFrameworkStageWithTimeout(user.Username,user.APIURL,user.SSOURL,user.Token, time.Minute*60)
		}else {
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

func userJourneyThread(frameworkMap *sync.Map, threadWaitGroup *sync.WaitGroup, threadIndex int, usersBar *uiprogress.Bar, resourcesBar *uiprogress.Bar, pipelinesBar *uiprogress.Bar) {
	chUsers := make(chan string, numberOfUsers)
	chPipelines := make(chan string, numberOfUsers)

	defer threadWaitGroup.Done()

	var wg *sync.WaitGroup = &sync.WaitGroup{}

	if waitPipelines {
		wg.Add(3)
	} else {
		wg.Add(2)
	}

	    go func() {
		defer wg.Done()
		for userIndex := 1; userIndex <= numberOfUsers; userIndex++ {
			startTime := time.Now()
			username := fmt.Sprintf("%s-%04d", usernamePrefix, threadIndex*numberOfUsers+userIndex)
			var framework *framework.Framework
			var user loadtestUtils.User
			var err error
			if stage {
				user = selectedUsers[threadIndex*numberOfUsers+userIndex-1]
				username = user.Username
				framework, err = tryNewFramework(username, user, 60*time.Minute)
			}else {
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
			metricsController.PushMetricsUsers(float64(FailedUserCreationsPerThread[threadIndex]), float64(SuccessfulUserCreationsPerThread[threadIndex]), float64(userCreationTime))
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
			klog.Infof("ns: %s, app: %s, framwork: %v",usernamespace, ApplicationName, framework)
			app, err := framework.AsKubeDeveloper.HasController.CreateHasApplicationWithTimeout(ApplicationName, usernamespace, 60*time.Minute)
			if err != nil {
				logError(4, fmt.Sprintf("Unable to create the Application %s: %v", ApplicationName, err))
				FailedResourceCreationsPerThread[threadIndex] += 1
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			//when application got created
			for _, cond := range app.Status.Conditions{
				if cond.Type == "Created"{
					//use this this is the actual time when application is ready
					applicationCameIntoExistence := cond.LastTransitionTime
					metricsController.PushApplicationMetrics(float64(app.CreationTimestamp.Time.Unix()),float64(applicationCameIntoExistence.Time.Unix()))
				}
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
			for _, cond := range cdq.Status.Conditions{
				if cond.Type == "Completed"{
					CameIntoExistence := cond.LastTransitionTime
					metricsController.PushCDQMetrics(float64(cdq.CreationTimestamp.Time.Unix()),float64(CameIntoExistence.Time.Unix()))
				}
			}

			for _, compStub := range cdq.Status.ComponentDetected {
				component, err := framework.AsKubeDeveloper.HasController.CreateComponentFromStubSkipInitialChecks(compStub, usernamespace, "", "", ApplicationName, pipelineSkipInitialChecks)

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
				for _, cond := range component.Status.Conditions{
					if cond.Type == "Created"{
						CameIntoExistence := cond.LastTransitionTime
						metricsController.PushComponentMetrics(float64(component.CreationTimestamp.Time.Unix()),float64(CameIntoExistence.Time.Unix()))
					}
				}	
				userComponentMap.Store(username, component.Name)
			}

			resourceCreationTime := time.Since(startTime)
			ResourceCreationTimeSumPerThread[threadIndex] += resourceCreationTime
			if resourceCreationTime > ResourceCreationTimeMaxPerThread[threadIndex] {
				ResourceCreationTimeMaxPerThread[threadIndex] = resourceCreationTime
			}
			SuccessfulResourceCreationsPerThread[threadIndex] += 1
			metricsController.PushMetricsResources(float64(FailedResourceCreationsPerThread[threadIndex]), float64(ResourceCreationTimeSumPerThread[threadIndex]))

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
						}
						metricsController.PushMetricsPipelines(float64(FailedPipelineRunsPerThread[threadIndex]), float64(PipelineRunSucceededTimeMaxPerThread[threadIndex]))
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
		}()
	}
	wg.Wait()
}
