package e2e

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redhat-appstudio/jvm-build-service/pkg/reconciler/jbsconfig"
	v13 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	jvmclientset "github.com/redhat-appstudio/jvm-build-service/pkg/client/clientset/versioned"
	"github.com/redhat-appstudio/jvm-build-service/pkg/reconciler/artifactbuild"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/printers"
	kubeset "k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

func generateName(base string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))
}

func dumpBadEvents(ta *testArgs) {
	eventClient := kubeClient.EventsV1().Events(ta.ns)
	eventList, err := eventClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		ta.Logf(fmt.Sprintf("error listing events: %s", err.Error()))
		return
	}
	ta.Logf(fmt.Sprintf("dumpBadEvents have %d items in total list", len(eventList.Items)))
	for _, event := range eventList.Items {
		if event.Type == corev1.EventTypeNormal {
			continue
		}
		ta.Logf(fmt.Sprintf("non-normal event reason %s about obj %s:%s message %s", event.Reason, event.Regarding.Kind, event.Regarding.Name, event.Note))
	}
}

func dumpNodes(ta *testArgs) {
	nodeClient := kubeClient.CoreV1().Nodes()
	nodeList, err := nodeClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		ta.Logf(fmt.Sprintf("error listin nodes: %s", err.Error()))
		return
	}
	ta.Logf(fmt.Sprintf("dumpNodes found %d nodes in list", len(nodeList.Items)))
	for _, node := range nodeList.Items {
		_, master := node.Labels["node-role.kubernetes.io/master"]
		if master {
			ta.Logf(fmt.Sprintf("Node %s is master node", node.Name))
		}
		if node.Status.Allocatable.Cpu() == nil {
			ta.Logf(fmt.Sprintf("Node %s does not have allocatable cpu", node.Name))
			continue
		}
		if node.Status.Allocatable.Memory() == nil {
			ta.Logf(fmt.Sprintf("Node %s does not have allocatable mem", node.Name))
			continue
		}
		if node.Status.Allocatable.Storage() == nil {
			ta.Logf(fmt.Sprintf("Node %s does not have allocatable storage", node.Name))
			continue
		}
		if node.Status.Capacity.Cpu() == nil {
			ta.Logf(fmt.Sprintf("Node %s does not have capacity cpu", node.Name))
			continue
		}
		if node.Status.Capacity.Memory() == nil {
			ta.Logf(fmt.Sprintf("Node %s does not have capacity mem", node.Name))
			continue
		}
		if node.Status.Capacity.Storage() == nil {
			ta.Logf(fmt.Sprintf("Node %s does not have capacity storage", node.Name))
			continue
		}
		alloccpu := node.Status.Allocatable.Cpu()
		allocmem := node.Status.Allocatable.Memory()
		allocstorage := node.Status.Allocatable.Storage()
		capaccpu := node.Status.Capacity.Cpu()
		capacmem := node.Status.Capacity.Memory()
		capstorage := node.Status.Capacity.Storage()
		ta.Logf(fmt.Sprintf("Node %s allocatable CPU %s allocatable mem %s allocatable storage %s capacity CPU %s capacitymem %s capacity storage %s",
			node.Name,
			alloccpu.String(),
			allocmem.String(),
			allocstorage.String(),
			capaccpu.String(),
			capacmem.String(),
			capstorage.String()))
	}
}

func debugAndFailTest(ta *testArgs, failMsg string) {
	GenerateStatusReport(ta.ns, jvmClient, kubeClient, tektonClient)
	dumpPodDetails(ta)
	dumpBadEvents(ta)
	ta.t.Fatalf(failMsg)

}

func commonSetup(t *testing.T, gitCloneUrl string, namespace string) *testArgs {

	ta := &testArgs{
		t:        t,
		timeout:  time.Minute * 15,
		interval: time.Second * 10,
	}
	setupClients(ta.t)

	if len(ta.ns) == 0 {
		ta.ns = generateName(namespace)
		namespace := &corev1.Namespace{}
		namespace.Name = ta.ns
		_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})

		if err != nil {
			debugAndFailTest(ta, fmt.Sprintf("%#v", err))
		}

		if err != nil {
			debugAndFailTest(ta, fmt.Sprintf("%#v", err))
		}
	}

	eventClient := kubeClient.EventsV1().Events(ta.ns)
	go watchEvents(eventClient, ta)
	dumpNodes(ta)

	var err error

	// have seen delays in CRD presence along with missing pipeline SA
	err = wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, 1*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		_, err = apiextensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), "tasks.tekton.dev", metav1.GetOptions{})
		if err != nil {
			ta.Logf(fmt.Sprintf("get of task CRD: %s", err.Error()))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		debugAndFailTest(ta, "task CRD not present in timely fashion")
	}

	ta.gitClone = &v1beta1.Task{}
	obj := streamRemoteYamlToTektonObj(gitCloneUrl, ta.gitClone, ta)
	var ok bool
	ta.gitClone, ok = obj.(*v1beta1.Task)
	if !ok {
		debugAndFailTest(ta, fmt.Sprintf("%s did not produce a task: %#v", gitCloneTaskUrl, obj))
	}
	ta.gitClone, err = tektonClient.TektonV1beta1().Tasks(ta.ns).Create(context.TODO(), ta.gitClone, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}

	path, err := os.Getwd()
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}

	mavenYamlPath := filepath.Join(path, "..", "..", "deploy", "base", "maven-v0.2.yaml")
	ta.maven = &v1beta1.Task{}
	obj = streamFileYamlToTektonObj(mavenYamlPath, ta.maven, ta)
	ta.maven, ok = obj.(*v1beta1.Task)
	if !ok {
		debugAndFailTest(ta, fmt.Sprintf("file %s did not produce a task: %#v", mavenYamlPath, obj))
	}
	// override images if need be
	quayUsername, _ := os.LookupEnv("QUAY_USERNAME")
	analyserImage := os.Getenv("JVM_BUILD_SERVICE_REQPROCESSOR_IMAGE")
	if len(analyserImage) > 0 {
		ta.Logf(fmt.Sprintf("PR analyzer image: %s", analyserImage))
		for i, step := range ta.maven.Spec.Steps {
			if step.Name != "analyse-dependencies" {
				continue
			}
			ta.Logf(fmt.Sprintf("Updating analyse-dependencies step with image %s", analyserImage))
			ta.maven.Spec.Steps[i].Image = analyserImage
		}
	} else if len(quayUsername) > 0 {
		image := "quay.io/" + quayUsername + "/hacbs-jvm-build-request-processor:dev"
		for i, step := range ta.maven.Spec.Steps {
			if step.Name != "analyse-dependencies" {
				continue
			}
			ta.Logf(fmt.Sprintf("Updating analyse-dependencies step with image %s", image))
			ta.maven.Spec.Steps[i].Image = image
			if strings.Contains(image, "minikube") {
				ta.maven.Spec.Steps[i].ImagePullPolicy = corev1.PullNever
				ta.Logf("Setting pull policy to never for minikube tests")
			}
		}
	}
	ta.maven, err = tektonClient.TektonV1beta1().Tasks(ta.ns).Create(context.TODO(), ta.maven, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	pipelineYamlPath := filepath.Join(path, "..", "..", "hack", "examples", "pipeline.yaml")
	ta.pipeline = &v1beta1.Pipeline{}
	obj = streamFileYamlToTektonObj(pipelineYamlPath, ta.pipeline, ta)
	ta.pipeline, ok = obj.(*v1beta1.Pipeline)
	if !ok {
		debugAndFailTest(ta, fmt.Sprintf("file %s did not produce a pipeline: %#v", pipelineYamlPath, obj))
	}
	ta.pipeline, err = tektonClient.TektonV1beta1().Pipelines(ta.ns).Create(context.TODO(), ta.pipeline, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	return ta
}
func setup(t *testing.T, namespace string) *testArgs {
	return setupConfig(t, namespace, false)
}
func setupHermetic(t *testing.T, namespace string) *testArgs {
	return setupConfig(t, namespace, true)
}
func setupConfig(t *testing.T, namespace string, hermetic bool) *testArgs {

	ta := commonSetup(t, gitCloneTaskUrl, namespace)
	err := wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, 1*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		_, err = kubeClient.CoreV1().ServiceAccounts(ta.ns).Get(context.TODO(), "pipeline", metav1.GetOptions{})
		if err != nil {
			ta.Logf(fmt.Sprintf("get of pipeline SA err: %s", err.Error()))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		debugAndFailTest(ta, "pipeline SA not created in timely fashion")
	}

	owner := os.Getenv("QUAY_E2E_ORGANIZATION")
	if owner == "" {
		owner = "redhat-appstudio-qe"
	}

	decoded, err := base64.StdEncoding.DecodeString(os.Getenv("QUAY_TOKEN"))
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	secret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "jvm-build-image-secrets", Namespace: ta.ns},
		Data: map[string][]byte{".dockerconfigjson": decoded}}
	_, err = kubeClient.CoreV1().Secrets(ta.ns).Create(context.TODO(), &secret, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}

	jbsConfig := v1alpha1.JBSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ta.ns,
			Name:      v1alpha1.JBSConfigName,
		},
		Spec: v1alpha1.JBSConfigSpec{
			EnableRebuilds: true,
			MavenBaseLocations: map[string]string{
				"maven-repository-300-jboss":     "https://repository.jboss.org/nexus/content/groups/public/",
				"maven-repository-301-confluent": "https://packages.confluent.io/maven",
				"maven-repository-302-redhat":    "https://maven.repository.redhat.com/ga",
				"maven-repository-303-jitpack":   "https://jitpack.io",
				"maven-repository-304-gradle":    "https://repo.gradle.org/artifactory/libs-releases"},

			CacheSettings: v1alpha1.CacheSettings{ //up the cache size, this is a lot of builds all at once, we could limit the number of pods instead but this gets the test done faster
				RequestMemory: "1024Mi",
				LimitMemory:   "1024Mi",
				WorkerThreads: "100",
				RequestCPU:    "10m",
			},
			Registry: v1alpha1.ImageRegistry{
				Host:       "quay.io",
				Owner:      owner,
				Repository: "test-images",
				PrependTag: strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
			RelocationPatterns: []v1alpha1.RelocationPatternElement{
				{
					RelocationPattern: v1alpha1.RelocationPattern{
						BuildPolicy: "default",
						Patterns: []v1alpha1.PatternElement{
							{
								Pattern: v1alpha1.Pattern{
									From: "(io.github.stuartwdouglas.hacbs-test.simple):(simple-jdk17):(99-does-not-exist)",
									To:   "io.github.stuartwdouglas.hacbs-test.simple:simple-jdk17:0.1.2",
								},
							},
							{
								Pattern: v1alpha1.Pattern{
									From: "org.graalvm.sdk:graal-sdk:21.3.2",
									To:   "org.graalvm.sdk:graal-sdk:21.3.2.0-1-redhat-00001",
								},
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.JBSConfigStatus{},
	}
	if hermetic {
		jbsConfig.Spec.HermeticBuilds = v1alpha1.HermeticBuildTypeRequired
	}
	_, err = jvmClient.JvmbuildserviceV1alpha1().JBSConfigs(ta.ns).Create(context.TODO(), &jbsConfig, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	err = waitForCache(ta)
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	return ta
}

func waitForCache(ta *testArgs) error {
	err := wait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		cache, err := kubeClient.AppsV1().Deployments(ta.ns).Get(context.TODO(), v1alpha1.CacheDeploymentName, metav1.GetOptions{})
		if err != nil {
			ta.Logf(fmt.Sprintf("get of cache: %s", err.Error()))
			return false, nil
		}
		if cache.Status.AvailableReplicas > 0 {
			ta.Logf("Cache is available")
			return true, nil
		}
		for _, cond := range cache.Status.Conditions {
			if cond.Type == v13.DeploymentProgressing && cond.Status == "False" {
				return false, errors.New("cache deployment failed")
			}

		}
		ta.Logf("Cache is progressing")
		return false, nil
	})
	if err != nil {
		debugAndFailTest(ta, "cache not present in timely fashion")
	}
	return err
}

func bothABsAndDBsGenerated(ta *testArgs) (bool, error) {
	abList, err := jvmClient.JvmbuildserviceV1alpha1().ArtifactBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		ta.Logf(fmt.Sprintf("error listing artifactbuilds: %s", err.Error()))
		return false, nil
	}
	gotABs := false
	if len(abList.Items) > 0 {
		gotABs = true
	}
	dbList, err := jvmClient.JvmbuildserviceV1alpha1().DependencyBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		ta.Logf(fmt.Sprintf("error listing dependencybuilds: %s", err.Error()))
		return false, nil
	}
	gotDBs := false
	if len(dbList.Items) > 0 {
		gotDBs = true
	}
	if gotABs && gotDBs {
		return true, nil
	}
	return false, nil
}

//func projectCleanup(ta *testArgs) {
//	projectClient.ProjectV1().Projects().Delete(context.Background(), ta.ns, metav1.DeleteOptions{})
//}

func decodeBytesToTektonObjbytes(bytes []byte, obj runtime.Object, ta *testArgs) runtime.Object {
	decodingScheme := runtime.NewScheme()
	utilruntime.Must(v1beta1.AddToScheme(decodingScheme))
	decoderCodecFactory := serializer.NewCodecFactory(decodingScheme)
	decoder := decoderCodecFactory.UniversalDecoder(v1beta1.SchemeGroupVersion)
	err := runtime.DecodeInto(decoder, bytes, obj)
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	return obj
}

func encodeToYaml(obj runtime.Object) string {

	y := printers.YAMLPrinter{}
	b := bytes.Buffer{}
	_ = y.PrintObj(obj, &b)
	return b.String()
}

func streamRemoteYamlToTektonObj(url string, obj runtime.Object, ta *testArgs) runtime.Object {
	resp, err := http.Get(url) //#nosec G107
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	return decodeBytesToTektonObjbytes(bytes, obj, ta)
}

func streamFileYamlToTektonObj(path string, obj runtime.Object, ta *testArgs) runtime.Object {
	bytes, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	return decodeBytesToTektonObjbytes(bytes, obj, ta)
}

func prPods(ta *testArgs, name string) []corev1.Pod {
	podClient := kubeClient.CoreV1().Pods(ta.ns)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", name),
	}
	podList, err := podClient.List(context.TODO(), listOptions)
	if err != nil {
		ta.Logf(fmt.Sprintf("error listing pr pods %s", err.Error()))
		return []corev1.Pod{}
	}
	return podList.Items
}

//go:embed report.html
var reportTemplate string

// dumping the logs slows down generation
// when working on the report you might want to turn it off
// this should always be true in the committed code though
const DUMP_LOGS = true

func GenerateStatusReport(namespace string, jvmClient *jvmclientset.Clientset, kubeClient *kubeset.Clientset, pipelineClient *pipelineclientset.Clientset) {

	directory := os.Getenv("ARTIFACT_DIR")
	if directory == "" {
		directory = "/tmp/jvm-build-service-report"
	} else {
		directory = directory + "/jvm-build-service-report/" + namespace
	}
	err := os.MkdirAll(directory, 0755) //#nosec G306 G301
	if err != nil {
		panic(err)
	}
	podClient := kubeClient.CoreV1().Pods(namespace)
	podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	pipelineList, err := pipelineClient.TektonV1beta1().PipelineRuns(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	artifact := ArtifactReportData{}
	dependency := DependencyReportData{}
	dependencyBuildClient := jvmClient.JvmbuildserviceV1alpha1().DependencyBuilds(namespace)
	artifactBuilds, err := jvmClient.JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, ab := range artifactBuilds.Items {
		localDir := ab.Status.State + "/" + ab.Name
		tmp := ab
		createdBy := ""
		if ab.Annotations != nil {
			for k, v := range ab.Annotations {
				if strings.HasPrefix(k, artifactbuild.DependencyBuildContaminatedByAnnotation) {
					createdBy = " (created by build " + v + ")"
				}
			}
		}
		message := ""
		if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
			message = " " + ab.Status.Message
		}
		instance := &ReportInstanceData{Name: ab.Name + createdBy + message, State: ab.Status.State, Yaml: encodeToYaml(&tmp)}
		artifact.Instances = append(artifact.Instances, instance)
		artifact.Total++
		print(ab.Status.State + "\n")
		switch ab.Status.State {
		case v1alpha1.ArtifactBuildStateComplete:
			artifact.Complete++
		case v1alpha1.ArtifactBuildStateFailed:
			artifact.Failed++
		case v1alpha1.ArtifactBuildStateMissing:
			artifact.Missing++
		default:
			artifact.Other++
		}

		_ = os.MkdirAll(directory+"/"+localDir, 0755) //#nosec G306 G301
		for _, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, ab.Name) {
				logFile := dumpPod(pod, directory, localDir, podClient, true)
				instance.Logs = append(instance.Logs, logFile...)
			}
		}
		for _, pipelineRun := range pipelineList.Items {
			if strings.HasPrefix(pipelineRun.Name, ab.Name) {
				t := pipelineRun
				yaml := encodeToYaml(&t)
				target := directory + "/" + localDir + "-" + "pipeline-" + t.Name
				err := os.WriteFile(target, []byte(yaml), 0644) //#nosec G306)
				if err != nil {
					print(fmt.Sprintf("Failed to write pipleine file %s: %s", target, err))
				}
				instance.Logs = append(instance.Logs, localDir+"-"+"pipeline-"+t.Name)
			}
		}
	}
	sort.Sort(SortableArtifact(artifact.Instances))

	dependencyBuilds, err := dependencyBuildClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for _, db := range dependencyBuilds.Items {
		dependency.Total++
		localDir := db.Status.State + "/" + db.Name
		tmp := db
		tool := "maven"
		if db.Status.CurrentBuildAttempt() != nil {
			tool = db.Status.CurrentBuildAttempt().Recipe.Tool
		}
		if db.Status.FailedVerification {
			tool += " (FAILED VERIFICATION)"
		}
		url := strings.TrimSuffix(db.Spec.ScmInfo.SCMURL, ".git")
		if strings.Contains(url, "github.com") {
			if len(db.Spec.ScmInfo.Tag) == 40 && !strings.Contains(db.Spec.ScmInfo.Tag, ".") && !strings.Contains(db.Spec.ScmInfo.Tag, "-") {
				url = fmt.Sprintf("%s/commit/%s", url, db.Spec.ScmInfo.Tag)
			} else {
				url = fmt.Sprintf("%s/releases/tag/%s", url, db.Spec.ScmInfo.Tag)
			}
		}
		instance := &ReportInstanceData{
			State:  db.Status.State,
			Yaml:   encodeToYaml(&tmp),
			Name:   fmt.Sprintf("%s @{%s} (%s) %s", db.Spec.ScmInfo.SCMURL, db.Spec.ScmInfo.Tag, db.Name, tool),
			GitUrl: url,
		}

		dependency.Instances = append(dependency.Instances, instance)
		print(db.Status.State + "\n")
		switch db.Status.State {
		case v1alpha1.DependencyBuildStateComplete:
			dependency.Complete++
		case v1alpha1.DependencyBuildStateFailed:
			dependency.Failed++
		case v1alpha1.DependencyBuildStateContaminated:
			dependency.Contaminated++
		case v1alpha1.DependencyBuildStateBuilding:
			dependency.Building++
		default:
			dependency.Other++
		}
		_ = os.MkdirAll(directory+"/"+localDir, 0755) //#nosec G306 G301
		for index, docker := range db.Status.BuildAttempts {

			localPart := localDir + "-docker-" + strconv.Itoa(index) + ".txt"
			fileName := directory + "/" + localPart
			err = os.WriteFile(fileName, []byte(docker.Build.DiagnosticDockerFile), 0644) //#nosec G306
			if err != nil {
				print(fmt.Sprintf("Failed to write docker filer %s: %s", fileName, err))
			} else {
				instance.Logs = append(instance.Logs, localPart)
			}
		}
		for _, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, db.Name) {
				logFile := dumpPod(pod, directory, localDir, podClient, true)
				instance.Logs = append(instance.Logs, logFile...)
			}
		}
		for _, pipelineRun := range pipelineList.Items {
			if strings.HasPrefix(pipelineRun.Name, db.Name) {
				t := pipelineRun
				yaml := encodeToYaml(&t)
				localPart := localDir + "-" + "pipeline-" + t.Name
				target := directory + "/" + localPart
				err := os.WriteFile(target, []byte(yaml), 0644) //#nosec G306)
				if err != nil {
					print(fmt.Sprintf("Failed to write pipleine file %s: %s", target, err))
				} else {
					instance.Logs = append(instance.Logs, localPart)
				}
				if db.Status.FailedVerification {
					verification := ""
					for _, res := range pipelineRun.Status.PipelineResults {
						if res.Name == artifactbuild.PipelineResultVerificationResult {
							verification = res.Value.StringVal
						}
					}
					if verification != "" {
						localPart := localDir + "-" + "pipeline-" + t.Name + "-FAILED-VERIFICATION"
						target := directory + "/" + localPart

						parsed := map[string][]string{}
						err := json.Unmarshal([]byte(verification), &parsed)
						if err != nil {
							print(fmt.Sprintf("Failed to parse json for pipleine file %s: %s", target, err))
						}
						output := ""
						for k, v := range parsed {
							if len(v) > 0 {
								output += "\n\nFAILED: " + k + "\n"
								for _, i := range v {
									output += "\t" + i + "\n"
								}
							}
						}

						err = os.WriteFile(target, []byte(output), 0644) //#nosec G306)
						if err != nil {
							print(fmt.Sprintf("Failed to write pipleine file %s: %s", target, err))
						} else {
							instance.Logs = append(instance.Logs, localPart)
						}
					}
				}
			}
		}
	}
	sort.Sort(SortableArtifact(dependency.Instances))

	report := directory + "/index.html"

	data := ReportData{
		Name:       namespace,
		Artifact:   artifact,
		Dependency: dependency,
	}

	_ = os.MkdirAll(directory+"/logs", 0755) //#nosec G306 G301
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "jvm-build-workspace-artifact-cache") {
			logFile := dumpPod(pod, directory, "logs", podClient, true)
			data.CacheLogs = append(data.CacheLogs, logFile...)
		}
	}
	operatorPodClient := kubeClient.CoreV1().Pods("jvm-build-service")
	operatorList, err := operatorPodClient.List(context.TODO(), metav1.ListOptions{})
	if err == nil {
		for _, pod := range operatorList.Items {
			logFile := dumpPod(pod, directory, "logs", operatorPodClient, true)
			data.OperatorLogs = append(data.OperatorLogs, logFile...)
		}
	}

	t, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)
	err = t.Execute(buf, data)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(report, buf.Bytes(), 0644) //#nosec G306
	if err != nil {
		panic(err)
	}
	print("Created report file://" + report + "\n")
}

func innerDumpPod(req *rest.Request, baseDirectory, localDirectory, podName, containerName string, skipSkipped bool) error {
	var readCloser io.ReadCloser
	var err error
	readCloser, err = req.Stream(context.TODO())
	if err != nil {
		print(fmt.Sprintf("error getting pod logs for container %s: %s", containerName, err.Error()))
		return err
	}
	defer func(readCloser io.ReadCloser) {
		err := readCloser.Close()
		if err != nil {
			print(fmt.Sprintf("Failed to close ReadCloser reading pod logs for container %s: %s", containerName, err.Error()))
		}
	}(readCloser)
	var b []byte
	b, err = io.ReadAll(readCloser)
	if skipSkipped && len(b) < 1000 {
		if strings.Contains(string(b), "Skipping step because a previous step failed") {
			return errors.New("the step failed")
		}
	}
	if err != nil {
		print(fmt.Sprintf("error reading pod stream %s", err.Error()))
		return err
	}
	directory := baseDirectory + "/" + localDirectory
	err = os.MkdirAll(directory, 0755) //#nosec G306 G301
	if err != nil {
		print(fmt.Sprintf("Failed to create artifact dir %s: %s", directory, err))
		return err
	}
	localPart := localDirectory + podName + "-" + containerName
	fileName := baseDirectory + "/" + localPart
	err = os.WriteFile(fileName, b, 0644) //#nosec G306
	if err != nil {
		print(fmt.Sprintf("Failed artifact dir %s: %s", directory, err))
		return err
	}
	return nil
}

func dumpPod(pod corev1.Pod, baseDirectory string, localDirectory string, kubeClient v12.PodInterface, skipSkipped bool) []string {
	if !DUMP_LOGS {
		return []string{}
	}
	containers := []corev1.Container{}
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	ret := []string{}
	for _, container := range containers {
		req := kubeClient.GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name})
		err := innerDumpPod(req, baseDirectory, localDirectory, pod.Name, container.Name, skipSkipped)
		if err != nil {
			continue
		}
		ret = append(ret, localDirectory+pod.Name+"-"+container.Name)
	}
	return ret
}

type ArtifactReportData struct {
	Complete  int
	Failed    int
	Missing   int
	Other     int
	Total     int
	Instances []*ReportInstanceData
}

type DependencyReportData struct {
	Complete     int
	Failed       int
	Contaminated int
	Building     int
	Other        int
	Total        int
	Instances    []*ReportInstanceData
}
type ReportData struct {
	Name         string
	Artifact     ArtifactReportData
	Dependency   DependencyReportData
	CacheLogs    []string
	OperatorLogs []string
}

type ReportInstanceData struct {
	Name   string
	Logs   []string
	State  string
	Yaml   string
	GitUrl string
}

type SortableArtifact []*ReportInstanceData

func (a SortableArtifact) Len() int           { return len(a) }
func (a SortableArtifact) Less(i, j int) bool { return strings.Compare(a[i].Name, a[j].Name) < 0 }
func (a SortableArtifact) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func setupMinikube(t *testing.T, namespace string) *testArgs {

	ta := commonSetup(t, minikubeGitCloneTaskUrl, namespace)
	//go through and limit all deployments
	//we have very little memory, we need some limits to make sure minikube can actually run
	//limit every deployment to 100mb
	list, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	for _, ns := range list.Items {
		deploymentList, err := kubeClient.AppsV1().Deployments(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			debugAndFailTest(ta, err.Error())
		}
		for depIdx := range deploymentList.Items {
			dep := deploymentList.Items[depIdx]
			fmt.Printf("Adjusting memory limit for pod %s.%s\n", dep.Namespace, dep.Name)
			for i := range dep.Spec.Template.Spec.Containers {
				if dep.Spec.Template.Spec.Containers[i].Resources.Limits == nil {
					dep.Spec.Template.Spec.Containers[i].Resources.Limits = corev1.ResourceList{}
				}
				if dep.Spec.Template.Spec.Containers[i].Resources.Requests == nil {
					dep.Spec.Template.Spec.Containers[i].Resources.Requests = corev1.ResourceList{}
				}
				dep.Spec.Template.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = resource.MustParse("110Mi")
				dep.Spec.Template.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("100Mi")
			}
			_, err := kubeClient.AppsV1().Deployments(ns.Name).Update(context.TODO(), &dep, metav1.UpdateOptions{})
			if err != nil {
				panic(err)
			}
		}
	}

	//create the ServiceAccount
	sa := corev1.ServiceAccount{}
	sa.Name = "pipeline"
	sa.Namespace = ta.ns
	_, err = kubeClient.CoreV1().ServiceAccounts(ta.ns).Create(context.Background(), &sa, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, "pipeline SA not created in timely fashion")
	}
	//now create the binding
	crb := v1.ClusterRoleBinding{}
	crb.Name = "pipeline-" + ta.ns
	crb.Namespace = ta.ns
	crb.RoleRef.Name = "pipeline"
	crb.RoleRef.Kind = "ClusterRole"
	crb.Subjects = []v1.Subject{{Name: "pipeline", Kind: "ServiceAccount", Namespace: ta.ns}}
	_, err = kubeClient.RbacV1().ClusterRoleBindings().Create(context.Background(), &crb, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, "pipeline SA not created in timely fashion")
	}

	devIp := os.Getenv("DEV_IP")
	owner := "testuser"
	jbsConfig := v1alpha1.JBSConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ta.ns,
			Name:        v1alpha1.JBSConfigName,
			Annotations: map[string]string{jbsconfig.TestRegistry: "true"},
		},
		Spec: v1alpha1.JBSConfigSpec{
			EnableRebuilds:    true,
			AdditionalRecipes: []string{"https://github.com/jvm-build-service-test-data/recipe-repo"},
			BuildSettings: v1alpha1.BuildSettings{
				BuildRequestMemory: "256Mi",
				TaskRequestMemory:  "256Mi",
				TaskLimitMemory:    "256mi",
			},
			MavenBaseLocations: map[string]string{
				"maven-repository-300-jboss":     "https://repository.jboss.org/nexus/content/groups/public/",
				"maven-repository-301-confluent": "https://packages.confluent.io/maven",
				"maven-repository-302-redhat":    "https://maven.repository.redhat.com/ga",
				"maven-repository-303-jitpack":   "https://jitpack.io"},

			CacheSettings: v1alpha1.CacheSettings{ //up the cache size, this is a lot of builds all at once, we could limit the number of pods instead but this gets the test done faster
				RequestMemory: "1024Mi",
				LimitMemory:   "1024Mi",
				RequestCPU:    "200m",
				LimitCPU:      "500m",
				WorkerThreads: "100",
				DisableTLS:    true,
				Storage:       "756Mi",
			},
			Registry: v1alpha1.ImageRegistry{
				Host:       devIp,
				Owner:      owner,
				Repository: "test-images",
				Port:       "5000",
				Insecure:   true,
				PrependTag: strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
		},
		Status: v1alpha1.JBSConfigStatus{},
	}
	_, err = jvmClient.JvmbuildserviceV1alpha1().JBSConfigs(ta.ns).Create(context.TODO(), &jbsConfig, metav1.CreateOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}

	time.Sleep(time.Second * 10)

	dumpPodDetails(ta)

	err = waitForCache(ta)
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	return ta
}

func dumpPodDetails(ta *testArgs) {
	list, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	for _, ns := range list.Items {
		podList, err := kubeClient.CoreV1().Pods(ns.Name).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			debugAndFailTest(ta, err.Error())
		}
		for _, pod := range podList.Items {
			fmt.Printf("Pod %s\n", pod.Name)
			for _, cs := range pod.Spec.Containers {
				fmt.Printf("Container %s has CPU limit %s and request %s and Memory limit %s and request %s\n", cs.Name, cs.Resources.Limits.Cpu().String(), cs.Resources.Requests.Cpu().String(), cs.Resources.Limits.Memory().String(), cs.Resources.Requests.Memory().String())
			}
		}
	}
}
