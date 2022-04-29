/*
Copyright © 2022 Meer Sawood <msawood@redhat.com>

*/
package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/codeready-toolchain/toolchain-e2e/setup/users"
	"github.com/codeready-toolchain/toolchain-e2e/setup/wait"
	"github.com/google/uuid"
	"github.com/gosuri/uiprogress"
	"github.com/gosuri/uitable/util/strutil"
	"github.com/redhat-appstudio/e2e-tests/pkg/apis/prometheus"
	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/authorization"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/getters"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/metrics"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var (
	usernamePrefix       = "appstudio-user"
	// kubeconfig           string
	numberOfUsers        int
	userBatches          int
	token 				 string
	QuarkusDevfileSource string = "https://github.com/redhat-appstudio-qe/devfile-sample-code-with-quarkus"
	
	
)

var (
	AverageUserCreationTime time.Duration
	AverageResourceCreationTimePerUser  time.Duration
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "load-test",
	Short: "Used to Generate Users and Run Load Tests on AppStudio.",
	Long: `Used to Generate Users and Run Load Tests on AppStudio.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	SilenceErrors: true,
	SilenceUsage:  false,
	Args:          cobra.NoArgs,
	Run:           setup,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.e2e-tests.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringVar(&usernamePrefix, "username", usernamePrefix, "the prefix used for usersignup names")
	// TODO use a custom kubeconfig and introduce debug logging and trace
	// rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	// rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "if 'debug' traces should be displayed in the console")
	rootCmd.Flags().IntVarP(&numberOfUsers, "users", "u", 5, "the number of user accounts to provision")
	rootCmd.Flags().IntVarP(&userBatches, "batch", "b", 5, "create user accounts in batches of N, increasing batch size may cause performance problems")
}


func setup(cmd *cobra.Command, args []string) {

	cmd.SilenceUsage = true

	if numberOfUsers%userBatches != 0{
		klog.Fatalf("Please Provide Correct Batches!")
		os.Exit(1)
	}
	
	klog.Infof("Number of users: %d", numberOfUsers)
	klog.Infof("Batch Size: %d", userBatches)

	klog.Infof("🕖 initializing...\n")
	framework, err := framework.NewFramework()
	if err != nil {
		klog.Errorf("error creating client-go %v", err)
	}

	if len(token) == 0 {
		token, err = authorization.CollectTokenFromOC()
		if err != nil {
			tokenRequestURI, err := authorization.FindTokenRequestURI(framework.CommonController.KubeRest())
			if err != nil {
				klog.Fatalf("a token is required to capture metrics, use oc login to log into the cluster: %v", err)
			}
			klog.Fatalf("a token is required to capture metrics, use oc login to log into the cluster. alternatively request a token and use the token flag: %v", tokenRequestURI)
		}
	}

	metricsInstance := metrics.CreateNewInstance(framework.CommonController.KubeRest(), token, 5*time.Minute)

	prometheusClient := prometheus.GetPrometheusClient(framework.CommonController.KubeRest(), token)

	metricsInstance.AddGetters(
		getters.GetWorkloadCPUUsage(prometheusClient, "application-service", "application-service-controller-manager"),
		getters.GetWorkloadMemoryUsage(prometheusClient, "application-service", "application-service-controller-manager"),
		//getters.GetWorkloadCPUUsage(prometheusClient, "build-service", "build-service-controller-manager"),
		//getters.GetWorkloadMemoryUsage(prometheusClient, "application-service", "build-service-controller-manager"),
	)
	

	klog.Infof("🍿 provisioning users...\n")

	uip := uiprogress.New()
	uip.Start()
	var wg sync.WaitGroup
	ch := make(chan bool)
	stopMetrics := metricsInstance.StartCollecting()

	klog.Infof("Sleeping till all metrics queries gets init")
	time.Sleep(time.Second * 10)

	AppStudioUsersBar := uip.AddBar(numberOfUsers).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio Users (%d/%d)", b.Current(), numberOfUsers), userBatches, ' ')
	})

	wg.Add(2)
	go func(){
		for AppStudioUsersBar.Incr(){
			startTime := time.Now()
			username := fmt.Sprintf("%s-%04d", usernamePrefix, AppStudioUsersBar.Current())
			if err := users.Create(framework.CommonController.KubeRest(), username,constants.HostOperatorNamespace, constants.MemberOperatorNamespace); err != nil {
				klog.Fatalf("failed to provision user '%s'", username)
				klog.Errorf(err.Error())
			}
			if AppStudioUsersBar.Current()%userBatches == 0 {
				for i := AppStudioUsersBar.Current() - userBatches + 1; i < AppStudioUsersBar.Current(); i++ {
					if err := wait.ForNamespace(framework.CommonController.KubeRest(), username); err != nil {
						klog.Fatalf("failed to find namespace '%s'", username)
						klog.Errorf(err.Error())
					}
				}
			}
			UserCreationTime := time.Since(startTime)
			AverageUserCreationTime += UserCreationTime
		}
		wg.Done()
		ch <- true
	}()
	ResourcesBar := uip.AddBar(numberOfUsers).AppendCompleted().PrependFunc(func(b *uiprogress.Bar) string {
		return strutil.PadLeft(fmt.Sprintf("Creating AppStudio User Resources (%d/%d)", b.Current(), numberOfUsers), userBatches, ' ')
	})
	go func(){
		<-ch
		for ResourcesBar.Incr(){
			startTime := time.Now()
			username := fmt.Sprintf("%s-%04d", usernamePrefix, ResourcesBar.Current())
			_, errors := framework.CommonController.CreateRegistryAuthSecret(
				"redhat-appstudio-registry-pull-secret",
				username,
				constants.GetDockerConfigJson(),
			)
			if errors != nil {
				klog.Fatalf("Problem Creating the secret: %v", errors)
			}
			// time.Sleep(time.Second * 2)
			ApplicationName := fmt.Sprintf("%s-app", username)
			_, err := framework.HasController.CreateHasApplication(ApplicationName, username)
			if err != nil{
				klog.Fatalf("Problem Creating the Application: %v", err)
			}
			ComponentName := fmt.Sprintf("%s-component", username)
			ComponentContainerImage := fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/devfile-sample-code-with-quarkus:%s", username, strings.Replace(uuid.New().String(), "-", "", -1))
			component, err :=  framework.HasController.CreateComponent(
				ApplicationName,
				ComponentName,
				username,
				QuarkusDevfileSource,
				ComponentContainerImage,
			)
			if component.Name != ComponentName {
				klog.Fatalf("Component Name Does not Match: %v", err)
			}
			if err != nil{
				klog.Fatalf("Problem Creating the Component: %v", err)
			}
			if ResourcesBar.Current()%userBatches == 0 {
				for i := ResourcesBar.Current() - userBatches + 1; i < ResourcesBar.Current(); i++ {
					time.Sleep(time.Second * 2)
					// Todo Add validation after each batch
				}
			}
			ResourceCreationTime := time.Since(startTime)
			AverageResourceCreationTimePerUser += ResourceCreationTime
		}
		wg.Done()
	}()
	

	// Todo add cleanup functions that will delete user signups 
	
	wg.Wait()
	uip.Stop()
	defer close(stopMetrics)
	klog.Infof("🏁 Load Test Completed!")
	klog.Infof("📈 Results 📉")
	klog.Infof("Average Time taken to spin up users: %.2f s", AverageUserCreationTime.Seconds()/float64(numberOfUsers))
	klog.Infof("Average Time taken to Create Resources: %.2f s", AverageResourceCreationTimePerUser.Seconds()/float64(numberOfUsers))
	metricsInstance.PrintResults()
	

}