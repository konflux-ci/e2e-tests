package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeready-toolchain/toolchain-e2e/setup/auth"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics/queries"
	"github.com/codeready-toolchain/toolchain-e2e/setup/terminal"
	"github.com/google/uuid"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uitable/util/strutil"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/rand"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"knative.dev/pkg/apis"
)

var (
	componentRepoUrl string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
	usernamePrefix   string = "testuser"
	numberOfUsers    int
	waitPipelines    bool
	verbose          bool
	token            string
	logConsole       bool
	failFast         bool
	disableMetrics   bool
	threadCount      int
)

var (
	AverageUserCreationTime            []time.Duration
	AverageResourceCreationTimePerUser []time.Duration
	AveragePipelineRunTimePerUser      []time.Duration
	FailedUserCreations                []int64
	FailedResourceCreations            []int64
	FailedPipelineRuns                 []int64
	frameworkMap                       *sync.Map
	userComponentMap                   *sync.Map
	errorOccurredMap                   map[int]ErrorOccurrence
	errorMutex                         = &sync.Mutex{}
	usersBarMutex                      = &sync.Mutex{}
	resourcesBarMutex                  = &sync.Mutex{}
	pipelinesBarMutex                  = &sync.Mutex{}
	threadsWG                          *sync.WaitGroup
	logData                            LogData
)

type ErrorOccurrence struct {
	ErrorCode     int    `json:"errorCode"`
	LatestMessage string `json:"latestMessage"`
	Count         int    `json:"count"`
}

type LogData struct {
	Timestamp                    string            `json:"timestamp"`
	EndTimestamp                 string            `json:"endTimestamp"`
	MachineName                  string            `json:"machineName"`
	BinaryDetails                string            `json:"binaryDetails"`
	ComponentRepoUrl             string            `json:"componentRepoUrl"`
	NumberOfThreads              int               `json:"threads"`
	NumberOfUsersPerThread       int               `json:"usersPerThread"`
	NumberOfUsers                int               `json:"totalUsers"`
	LoadTestCompletionStatus     string            `json:"status"`
	AverageTimeToSpinUpUsers     float64           `json:"createUserTimeAvg"`
	AverageTimeToCreateResources float64           `json:"createResourcesTimeAvg"`
	AverageTimeToRunPipelines    float64           `json:"runPipelineTimeAvg"`
	UserCreationFailureCount     int64             `json:"createUserFailures"`
	UserCreationFailureRate      float64           `json:"createUserFailureRate"`
	ResourceCreationFailureCount int64             `json:"createResourcesFailures"`
	ResourceCreationFailureRate  float64           `json:"createResourcesFailureRate"`
	PipelineRunFailureCount      int64             `json:"runPipelineFailures"`
	PipelineRunFailureRate       float64           `json:"runPipelineFailureRate"`
	ErrorsOccurred               []ErrorOccurrence `json:"errors"`
	ErrorsTotal                  int               `json:"errorsTotal"`
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
	rootCmd.Flags().BoolVarP(&logConsole, "log-to-console", "l", false, "if you want to log to console in addition to the log file")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "if you want the test to fail fast at first failure")
	rootCmd.Flags().BoolVar(&disableMetrics, "disable-metrics", false, "if you want to disable metrics gathering")
	rootCmd.Flags().IntVarP(&threadCount, "threads", "t", 1, "number of concurrent threads to execute")
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
	errorOccurrence, ok := errorOccurredMap[errCode]
	if ok {
		errorOccurrence.Count += 1
		errorOccurrence.LatestMessage = message
		errorOccurredMap[errCode] = errorOccurrence
	} else {
		errorOccurrence := ErrorOccurrence{
			ErrorCode:     errCode,
			LatestMessage: message,
			Count:         1,
		}
		errorOccurredMap[errCode] = errorOccurrence
	}
	logData.ErrorsTotal += 1
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
		Timestamp:              time.Now().Format("2006-01-02T15:04:05Z07:00"),
		MachineName:            machineName,
		BinaryDetails:          binaryDetails,
		ComponentRepoUrl:       componentRepoUrl,
		NumberOfThreads:        threadCount,
		NumberOfUsersPerThread: numberOfUsers,
		NumberOfUsers:          overallCount,
	}

	klog.Infof("🍿 provisioning users...\n")

	uip := uiprogress.New()
	uip.Start()

	barLength := 60

	AppStudioUsersBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Users (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedUserCreations)), barLength, ' ')
	})

	ResourcesBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio User Resources (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedResourceCreations)), barLength, ' ')
	})

	PipelinesBar := uip.AddBar(overallCount).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Waiting for pipelines to finish (%d/%d) [%d failed]", b.Current(), overallCount, sumFromArray(FailedPipelineRuns)), barLength, ' ')
	})

	AverageUserCreationTime = make([]time.Duration, threadCount)
	AverageResourceCreationTimePerUser = make([]time.Duration, threadCount)
	AveragePipelineRunTimePerUser = make([]time.Duration, threadCount)
	FailedUserCreations = make([]int64, threadCount)
	FailedResourceCreations = make([]int64, threadCount)
	FailedPipelineRuns = make([]int64, threadCount)
	frameworkMap = &sync.Map{}
	userComponentMap = &sync.Map{}
	errorOccurredMap = make(map[int]ErrorOccurrence)

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

	averageTimeToSpinUpUsers := averageDurationFromArray(AverageUserCreationTime)
	logData.AverageTimeToSpinUpUsers = averageTimeToSpinUpUsers

	averageTimeToCreateResources := averageDurationFromArray(AverageResourceCreationTimePerUser)
	logData.AverageTimeToCreateResources = averageTimeToCreateResources

	averageTimeToRunPipelines := averageDurationFromArray(AveragePipelineRunTimePerUser)
	logData.AverageTimeToRunPipelines = averageTimeToRunPipelines

	userCreationFailureCount := sumFromArray(FailedUserCreations)
	logData.UserCreationFailureCount = userCreationFailureCount

	userCreationFailureRate := float64(sumFromArray(FailedUserCreations)) / float64(overallCount)
	logData.UserCreationFailureRate = userCreationFailureRate

	resourceCreationFailureCount := sumFromArray(FailedResourceCreations)
	logData.ResourceCreationFailureCount = resourceCreationFailureCount

	resourceCreationFailureRate := float64(sumFromArray(FailedResourceCreations)) / float64(overallCount)
	logData.ResourceCreationFailureRate = resourceCreationFailureRate

	pipelineRunFailureCount := sumFromArray(FailedPipelineRuns)
	logData.PipelineRunFailureCount = pipelineRunFailureCount

	pipelineRunFailureRate := float64(sumFromArray(FailedPipelineRuns)) / float64(overallCount)
	logData.PipelineRunFailureRate = pipelineRunFailureRate

	klog.Infof("🏁 Load Test Completed!")
	klog.Infof("📈 Results 📉")
	klog.Infof("Average Time taken to spin up users: %.2f s", averageTimeToSpinUpUsers)
	klog.Infof("Average Time taken to Create Resources: %.2f s", averageTimeToCreateResources)
	klog.Infof("Average Time taken to Run Pipelines: %.2f s", averageTimeToRunPipelines)
	klog.Infof("Number of times user creation failed: %d (%.2f %%)", userCreationFailureCount, userCreationFailureRate*100)
	klog.Infof("Number of times resource creation failed: %d (%.2f %%)", resourceCreationFailureCount, resourceCreationFailureRate*100)
	klog.Infof("Number of times pipeline run failed: %d (%.2f %%)", pipelineRunFailureCount, pipelineRunFailureRate*100)
	errorOccurredList := []ErrorOccurrence{}
	for _, errorOccurrence := range errorOccurredMap {
		errorOccurredList = append(errorOccurredList, errorOccurrence)
		klog.Infof("Number of error #%d occured: %d", errorOccurrence.ErrorCode, errorOccurrence.Count)
	}
	logData.ErrorsOccurred = errorOccurredList
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

func averageDurationFromArray(durations []time.Duration) float64 {
	sum := 0.0
	for _, i := range durations {
		sum += i.Seconds()
	}
	return sum / float64(len(durations))
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
			framework, err := tryNewFramework(username, 60*time.Minute)
			if err != nil {
				logError(1, fmt.Sprintf("Unable to provision user '%s': %v", username, err))
				atomic.StoreInt64(&FailedUserCreations[threadIndex], atomic.AddInt64(&FailedUserCreations[threadIndex], 1))
				increaseBar(usersBar, usersBarMutex)
				continue
			} else {
				frameworkMap.Store(username, framework)
			}

			chUsers <- username

			UserCreationTime := time.Since(startTime)
			AverageUserCreationTime[threadIndex] += UserCreationTime
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
			_, errors := framework.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(
				constants.RegistryAuthSecretName,
				usernamespace,
				utils.GetDockerConfigJson(),
			)
			if errors != nil {
				logError(3, fmt.Sprintf("Unable to create the secret %s in namespace %s: %v", constants.RegistryAuthSecretName, usernamespace, errors))
				atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			ApplicationName := fmt.Sprintf("%s-app", username)
			app, err := framework.AsKubeDeveloper.HasController.CreateHasApplicationWithTimeout(ApplicationName, usernamespace, 60*time.Minute)
			if err != nil {
				logError(4, fmt.Sprintf("Unable to create the Application %s: %v", ApplicationName, err))
				atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
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
				atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}

			ComponentDetectionQueryName := fmt.Sprintf("%s-cdq", username)
			cdq, err := framework.AsKubeDeveloper.HasController.CreateComponentDetectionQueryWithTimeout(ComponentDetectionQueryName, usernamespace, componentRepoUrl, "", "", "", false, 60*time.Minute)
			if err != nil {
				logError(6, fmt.Sprintf("Unable to create ComponentDetectionQuery %s: %v", ComponentDetectionQueryName, err))
				atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			if cdq.Name != ComponentDetectionQueryName {
				logError(7, fmt.Sprintf("Actual cdq name (%s) does not match expected (%s): %v", cdq.Name, ComponentDetectionQueryName, err))
				atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}
			if len(cdq.Status.ComponentDetected) > 1 {
				logError(7, fmt.Sprintf("cdq (%s) detected more than 1 component", cdq.Name))
				atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
				increaseBar(resourcesBar, resourcesBarMutex)
				continue
			}

			for _, compStub := range cdq.Status.ComponentDetected {
				ComponentContainerImage := fmt.Sprintf("quay.io/%s/test-images:%s-%s", utils.GetQuayIOOrganization(), username, strings.Replace(uuid.New().String(), "-", "", -1))
				component, err := framework.AsKubeDeveloper.HasController.CreateComponentFromStub(compStub, usernamespace, ComponentContainerImage, "", ApplicationName)

				if err != nil {
					logError(6, fmt.Sprintf("Unable to create the Component %s: %v", compStub.ComponentStub.ComponentName, err))
					atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
					increaseBar(resourcesBar, resourcesBarMutex)
					continue
				}
				if component.Name != compStub.ComponentStub.ComponentName {
					logError(7, fmt.Sprintf("Actual component name (%s) does not match expected (%s): %v", component.Name, compStub.ComponentStub.ComponentName, err))
					atomic.StoreInt64(&FailedResourceCreations[threadIndex], atomic.AddInt64(&FailedResourceCreations[threadIndex], 1))
					increaseBar(resourcesBar, resourcesBarMutex)
					continue
				}
				userComponentMap.Store(username, component.Name)
			}

			ResourceCreationTime := time.Since(startTime)
			AverageResourceCreationTimePerUser[threadIndex] += ResourceCreationTime

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
					logError(8, fmt.Sprintf("Framework not found for username %s", username))
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
				usernamespace := framework.UserNamespace
				componentName := componentForUser(username)
				if componentName == "" {
					logError(9, fmt.Sprintf("Component not found for username %s", username))
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
				applicationName := fmt.Sprintf("%s-app", username)
				pipelineRetryInterval := time.Second * 5
				pipelineTimeout := time.Minute * 60
				error := k8swait.Poll(pipelineRetryInterval, pipelineTimeout, func() (done bool, err error) {
					pipelineRun, err := framework.AsKubeDeveloper.HasController.GetComponentPipelineRun(componentName, applicationName, usernamespace, "")
					if err != nil {
						time.Sleep(time.Millisecond * time.Duration(rand.IntnRange(10, 200)))
						return false, nil
					}
					if pipelineRun.IsDone() {
						AveragePipelineRunTimePerUser[threadIndex] += pipelineRun.Status.CompletionTime.Sub(pipelineRun.CreationTimestamp.Time)
						succeededCondition := pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
						if succeededCondition.IsFalse() {
							logError(10, fmt.Sprintf("Pipeline run for %s/%s failed due to %v: %v", applicationName, componentName, succeededCondition.Reason, succeededCondition.Message))
							atomic.StoreInt64(&FailedPipelineRuns[threadIndex], atomic.AddInt64(&FailedPipelineRuns[threadIndex], 1))
						}
						increaseBar(pipelinesBar, pipelinesBarMutex)
					}
					return pipelineRun.IsDone(), nil
				})
				if error != nil {
					logError(11, fmt.Sprintf("Pipeline run for %s/%s failed to succeed within %v: %v", applicationName, componentName, pipelineTimeout, error))
					atomic.StoreInt64(&FailedPipelineRuns[threadIndex], atomic.AddInt64(&FailedPipelineRuns[threadIndex], 1))
					increaseBar(pipelinesBar, pipelinesBarMutex)
					continue
				}
			}
		}()
	}
	wg.Wait()
}
