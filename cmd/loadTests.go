package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/codeready-toolchain/toolchain-e2e/setup/auth"
	"github.com/codeready-toolchain/toolchain-e2e/setup/configuration"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics"
	"github.com/codeready-toolchain/toolchain-e2e/setup/metrics/queries"
	"github.com/codeready-toolchain/toolchain-e2e/setup/terminal"
	"github.com/codeready-toolchain/toolchain-e2e/setup/users"
	"github.com/codeready-toolchain/toolchain-e2e/setup/wait"
	"github.com/google/uuid"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uitable/util/strutil"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/spf13/cobra"
	k8swait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

var (
	usernamePrefix       = "testuser"
	numberOfUsers        int
	userBatches          int
	waitPipelines        bool
	verbose              bool
	QuarkusDevfileSource string = "https://github.com/devfile-samples/devfile-sample-code-with-quarkus"
	token                string
	logConsole           bool
	failFast             bool
	disableMetrics       bool
)

var (
	AverageUserCreationTime            time.Duration
	AverageResourceCreationTimePerUser time.Duration
	AveragePipelineRunTimePerUser      time.Duration
	FailedUserCreations                int64
	FailedResourceCreations            int64
	FailedPipelineRuns                 int64
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "load-test",
	Short: "Used to Generate Users and Run Load Tests on AppStudio.",
	Long:  `Used to Generate Users and Run Load Tests on AppStudio.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	SilenceErrors: true,
	SilenceUsage:  false,
	Args:          cobra.NoArgs,
	Run:           setup,
}

// ExecuteLoadTest adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func ExecuteLoadTest() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringVar(&usernamePrefix, "username", usernamePrefix, "the prefix used for usersignup names")
	// TODO use a custom kubeconfig and introduce debug logging and trace
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "if 'debug' traces should be displayed in the console")
	rootCmd.Flags().IntVarP(&numberOfUsers, "users", "u", 5, "the number of user accounts to provision")
	rootCmd.Flags().IntVarP(&userBatches, "batch", "b", 5, "create user accounts in batches of N, increasing batch size may cause performance problems")
	rootCmd.Flags().BoolVarP(&waitPipelines, "waitpipelines", "w", false, "if you want to wait for pipelines to finish")
	rootCmd.Flags().BoolVarP(&logConsole, "log-to-console", "l", false, "if you want to log to console in addition to the log file")
	rootCmd.Flags().BoolVar(&failFast, "fail-fast", false, "if you want the test to fail fast at first failure")
	rootCmd.Flags().BoolVar(&disableMetrics, "disable-metrics", false, "if you want to disable metrics gathering")
}

func logError(errCode int, message string) {
	msg := fmt.Sprintf("Error #%d: %s", errCode, message)
	if failFast {
		klog.Fatalln(msg)
	} else {
		klog.Infoln(msg)
	}
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

	if numberOfUsers%userBatches != 0 {
		klog.Fatalf("Please Provide Correct Batches!")
		os.Exit(1)
	}

	klog.Infof("Number of users: %d", numberOfUsers)
	klog.Infof("Batch Size: %d", userBatches)

	klog.Infof("🕖 initializing...\n")
	framework, err := framework.NewFramework("load-tests")
	if err != nil {
		klog.Fatalf("error creating client-go %v", err)
	}

	if len(token) == 0 {
		token, err = auth.GetTokenFromOC()
		if err != nil {
			tokenRequestURI, err := auth.GetTokenRequestURI(framework.AsKubeAdmin.CommonController.KubeRest()) // authorization.FindTokenRequestURI(framework.CommonController.KubeRest())
			if err != nil {
				klog.Fatalf("a token is required to capture metrics, use oc login to log into the cluster: %v", err)
			}
			klog.Fatalf("a token is required to capture metrics, use oc login to log into the cluster. alternatively request a token and use the token flag: %v", tokenRequestURI)
		}
	}

	var stopMetrics chan struct{}
	var metricsInstance *metrics.Gatherer
	if !disableMetrics {
		metricsInstance = metrics.NewEmpty(term, framework.AsKubeAdmin.CommonController.KubeRest(), 10*time.Minute)

		prometheusClient := metrics.GetPrometheusClient(term, framework.AsKubeAdmin.CommonController.KubeRest(), token)

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
	klog.Infof("🍿 provisioning users...\n")

	chUsers := make(chan int, numberOfUsers)
	chPipelines := make(chan int, numberOfUsers)

	uip := uiprogress.New()
	var wg sync.WaitGroup
	uip.Start()
	AppStudioUsersBar := uip.AddBar(numberOfUsers).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Users (%d/%d) [%d failed]", b.Current(), numberOfUsers, FailedUserCreations), userBatches, ' ')
	})

	if waitPipelines {
		wg.Add(3)
	} else {
		wg.Add(2)
	}

	go func() {
	UserLoop:
		for AppStudioUsersBar.Incr() {
			userIndex := AppStudioUsersBar.Current()
			startTime := time.Now()
			username := fmt.Sprintf("%s-%04d", usernamePrefix, userIndex)
			if err := users.Create(framework.AsKubeAdmin.CommonController.KubeRest(), username, constants.HostOperatorNamespace, constants.MemberOperatorNamespace); err != nil {
				logError(1, fmt.Sprintf("Unable to provision user '%s': %v", username, err))
				atomic.StoreInt64(&FailedUserCreations, atomic.AddInt64(&FailedUserCreations, 1))
				continue
			}
			if userIndex%userBatches == 0 {
				for i := userIndex - userBatches + 1; i <= userIndex; i++ {
					user := fmt.Sprintf("%s-%04d", usernamePrefix, i)
					usernamespace := fmt.Sprintf("%s-tenant", user)
					if err := wait.ForNamespace(framework.AsKubeAdmin.CommonController.KubeRest(), usernamespace); err != nil {
						logError(2, fmt.Sprintf("Unable to find namespace '%s' within %v: %v", usernamespace, configuration.DefaultTimeout, err))
						atomic.StoreInt64(&FailedUserCreations, atomic.AddInt64(&FailedUserCreations, 1))
						continue UserLoop
					}
					chUsers <- i
				}
			}
			UserCreationTime := time.Since(startTime)
			AverageUserCreationTime += UserCreationTime
		}
		close(chUsers)
		wg.Done()
	}()
	ResourcesBar := uip.AddBar(numberOfUsers).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio User Resources (%d/%d) [%d failed]", b.Current(), numberOfUsers, FailedResourceCreations), userBatches, ' ')
	})
	go func() {
		for userIndex := range chUsers {
			startTime := time.Now()
			username := fmt.Sprintf("%s-%04d", usernamePrefix, userIndex)
			usernamespace := fmt.Sprintf("%s-tenant", username)
			_, errors := framework.AsKubeAdmin.CommonController.CreateRegistryAuthSecret(
				constants.RegistryAuthSecretName,
				usernamespace,
				utils.GetDockerConfigJson(),
			)
			if errors != nil {
				logError(3, fmt.Sprintf("Unable to create the secret %s: %v", constants.RegistryAuthSecretName, errors))
				atomic.StoreInt64(&FailedResourceCreations, atomic.AddInt64(&FailedResourceCreations, 1))
				continue
			}
			// time.Sleep(time.Second * 2)
			ApplicationName := fmt.Sprintf("%s-app", username)
			app, err := framework.AsKubeAdmin.HasController.CreateHasApplication(ApplicationName, usernamespace)
			if err != nil {
				logError(4, fmt.Sprintf("Unable to create the Application %s: %v", ApplicationName, err))
				atomic.StoreInt64(&FailedResourceCreations, atomic.AddInt64(&FailedResourceCreations, 1))
				continue
			}
			gitopsRepoTimeout := 60 * time.Second
			if err := utils.WaitUntil(framework.AsKubeAdmin.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), gitopsRepoTimeout); err != nil {
				logError(5, fmt.Sprintf("Unable to create application gitops repo within %v: %v", gitopsRepoTimeout, err))
				atomic.StoreInt64(&FailedResourceCreations, atomic.AddInt64(&FailedResourceCreations, 1))
				continue
			}
			ComponentName := fmt.Sprintf("%s-component", username)
			ComponentContainerImage := fmt.Sprintf("quay.io/%s/test-images:%s-%s", utils.GetQuayIOOrganization(), username, strings.Replace(uuid.New().String(), "-", "", -1))
			component, err := framework.AsKubeAdmin.HasController.CreateComponent(
				ApplicationName,
				ComponentName,
				usernamespace,
				QuarkusDevfileSource,
				"",
				"",
				ComponentContainerImage,
				"",
				true,
			)
			if err != nil {
				logError(6, fmt.Sprintf("Unable to create the Component %s: %v", ComponentName, err))
				atomic.StoreInt64(&FailedResourceCreations, atomic.AddInt64(&FailedResourceCreations, 1))
				continue
			}
			if component.Name != ComponentName {
				logError(7, fmt.Sprintf("Component Name Does not Match: %v", err))
				atomic.StoreInt64(&FailedResourceCreations, atomic.AddInt64(&FailedResourceCreations, 1))
				continue
			}
			if userIndex%userBatches == 0 {
				for i := userIndex - userBatches + 1; i <= userIndex; i++ {
					time.Sleep(time.Second * 1)
					// Todo Add validation after each batch
				}
			}
			ResourceCreationTime := time.Since(startTime)
			AverageResourceCreationTimePerUser += ResourceCreationTime
			chPipelines <- userIndex
			ResourcesBar.Incr()
		}
		close(chPipelines)
		wg.Done()
	}()
	if waitPipelines {
		PipelinesBar := uip.AddBar(numberOfUsers).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
			return strutil.PadLeft(fmt.Sprintf("Waiting for pipelines to finish (%d/%d) [%d failed]", b.Current(), numberOfUsers, FailedPipelineRuns), userBatches, ' ')
		})
		go func() {
			for userIndex := range chPipelines {
				username := fmt.Sprintf("%s-%04d", usernamePrefix, userIndex)
				usernamespace := fmt.Sprintf("%s-tenant", username)
				ComponentName := fmt.Sprintf("%s-component", username)
				ApplicationName := fmt.Sprintf("%s-app", username)
				DefaultRetryInterval := time.Millisecond * 200
				DefaultTimeout := time.Minute * 17
				error := k8swait.Poll(DefaultRetryInterval, DefaultTimeout, func() (done bool, err error) {
					pipelineRun, err := framework.AsKubeAdmin.HasController.GetComponentPipelineRun(ComponentName, ApplicationName, usernamespace, "")
					if err != nil {
						return false, nil
					}
					if pipelineRun.IsDone() {
						AveragePipelineRunTimePerUser += time.Since(pipelineRun.GetCreationTimestamp().Time)
						PipelinesBar.Incr()
					}
					return pipelineRun.IsDone(), nil
				})
				if error != nil {
					logError(8, fmt.Sprintf("Pipeline run for %s/%s failed: %v", ApplicationName, ComponentName, error))
					atomic.StoreInt64(&FailedPipelineRuns, atomic.AddInt64(&FailedPipelineRuns, 1))
					continue
				}
			}
			wg.Done()
		}()
	}

	// Todo add cleanup functions that will delete user signups

	wg.Wait()
	uip.Stop()
	klog.Infof("🏁 Load Test Completed!")
	klog.Infof("📈 Results 📉")
	klog.Infof("Average Time taken to spin up users: %.2f s", AverageUserCreationTime.Seconds()/float64(numberOfUsers))
	klog.Infof("Average Time taken to Create Resources: %.2f s", AverageResourceCreationTimePerUser.Seconds()/float64(numberOfUsers))
	klog.Infof("Average Time taken to Run Pipelines: %.2f s", AveragePipelineRunTimePerUser.Seconds()/float64(numberOfUsers))
	klog.Infof("Number of times user creation failed: %d (%.2f %%)", FailedUserCreations, float64(FailedUserCreations)/float64(numberOfUsers))
	klog.Infof("Number of times resource creation failed: %d (%.2f %%)", FailedResourceCreations, float64(FailedResourceCreations)/float64(numberOfUsers))
	klog.Infof("Number of times pipeline run failed: %d (%.2f %%)", FailedPipelineRuns, float64(FailedPipelineRuns)/float64(numberOfUsers))
	klog.StopFlushDaemon()
	klog.Flush()
	if !disableMetrics {
		defer close(stopMetrics)
		metricsInstance.PrintResults()
	}
}
