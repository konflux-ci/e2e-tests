package e2e

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
	"testing"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	"github.com/redhat-appstudio/jvm-build-service/pkg/reconciler/artifactbuild"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"knative.dev/pkg/apis"
)

func runBasicTests(t *testing.T, doSetup func(t *testing.T, namespace string) *testArgs, namespace string) {
	runPipelineTests(t, doSetup, "run-e2e-shaded-app.yaml", namespace)
}

func runPipelineTests(t *testing.T, doSetup func(t *testing.T, namespace string) *testArgs, pipeline string, namespace string) {
	ta := doSetup(t, namespace)
	//TODO, for now at least, keeping our test project to allow for analyzing the various CRD instances both for failure
	// and successful runs (in case a run succeeds, but we find something amiss if we look at passing runs; our in repo
	// tests do now run in conjunction with say the full suite of e2e's in the e2e-tests runs, so no contention there.
	//defer projectCleanup(ta)

	path, err := os.Getwd()
	if err != nil {
		debugAndFailTest(ta, err.Error())
	}
	ta.Logf(fmt.Sprintf("current working dir: %s", path))

	runYamlPath := filepath.Join(path, "..", "..", "hack", "examples", pipeline)
	ta.run = &v1beta1.PipelineRun{}
	var ok bool
	obj := streamFileYamlToTektonObj(runYamlPath, ta.run, ta)
	ta.run, ok = obj.(*v1beta1.PipelineRun)
	if !ok {
		debugAndFailTest(ta, fmt.Sprintf("file %s did not produce a pipelinerun: %#v", runYamlPath, obj))
	}

	set := os.Getenv("TESTSET")
	//if the GAVS env var is set then we just create pre-defined GAVS
	//otherwise we do a full build of a sample project
	if len(set) > 0 {
		bytes, err := os.ReadFile(filepath.Clean(filepath.Join(path, "minikube.yaml")))
		if err != nil {
			debugAndFailTest(ta, err.Error())
			return
		}
		testData := map[string][]string{}
		err = yaml.Unmarshal(bytes, &testData)
		if err != nil {
			debugAndFailTest(ta, err.Error())
			return
		}

		parts := testData[set]
		if len(parts) == 0 {
			debugAndFailTest(ta, "No test data for "+set)
			return
		}
		for _, s := range parts {
			ta.Logf(fmt.Sprintf("Creating ArtifactBuild for GAV: %s", s))
			ab := v1alpha1.ArtifactBuild{}
			ab.Name = artifactbuild.CreateABRName(s)
			ab.Namespace = ta.ns
			ab.Spec.GAV = s
			_, err := jvmClient.JvmbuildserviceV1alpha1().ArtifactBuilds(ta.ns).Create(context.TODO(), &ab, metav1.CreateOptions{})
			if err != nil {
				return
			}
		}
	} else {

		ta.run, err = tektonClient.TektonV1beta1().PipelineRuns(ta.ns).Create(context.TODO(), ta.run, metav1.CreateOptions{})
		if err != nil {
			debugAndFailTest(ta, err.Error())
		}
		ta.t.Run("pipelinerun completes successfully", func(t *testing.T) {
			err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, ta.timeout, true, func(ctx context.Context) (done bool, err error) {
				pr, err := tektonClient.TektonV1beta1().PipelineRuns(ta.ns).Get(context.TODO(), ta.run.Name, metav1.GetOptions{})
				if err != nil {
					ta.Logf(fmt.Sprintf("get pr %s produced err: %s", ta.run.Name, err.Error()))
					return false, nil
				}
				if !pr.IsDone() {
					if err != nil {
						ta.Logf(fmt.Sprintf("problem marshalling in progress pipelinerun to bytes: %s", err.Error()))
						return false, nil
					}
					ta.Logf(fmt.Sprintf("in flight pipeline run: %s", pr.Name))
					return false, nil
				}
				if !pr.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
					prBytes, err := json.MarshalIndent(pr, "", "  ")
					if err != nil {
						ta.Logf(fmt.Sprintf("problem marshalling failed pipelinerun to bytes: %s", err.Error()))
						return false, nil
					}
					debugAndFailTest(ta, fmt.Sprintf("unsuccessful pipeline run: %s", string(prBytes)))
				}
				return true, nil
			})
			if err != nil {
				debugAndFailTest(ta, fmt.Sprintf("failure occured when waiting for the pipeline run to complete: %v", err))
			}
		})
	}

	ta.t.Run("artifactbuilds and dependencybuilds generated", func(t *testing.T) {
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, ta.timeout, true, func(ctx context.Context) (done bool, err error) {
			return bothABsAndDBsGenerated(ta)
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for generation of artifactbuilds and dependencybuilds")
		}
	})

	ta.t.Run("all artfactbuilds and dependencybuilds complete", func(t *testing.T) {
		defer GenerateStatusReport(ta.ns, jvmClient, kubeClient, tektonClient)
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, time.Hour, true, func(ctx context.Context) (done bool, err error) {
			abList, err := jvmClient.JvmbuildserviceV1alpha1().ArtifactBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error list artifactbuilds: %s", err.Error()))
				return false, err
			}
			//we want to make sure there is more than one ab, and that they are all complete
			abComplete := len(abList.Items) > 0
			ta.Logf(fmt.Sprintf("number of artifactbuilds: %d", len(abList.Items)))
			for _, ab := range abList.Items {
				if ab.Status.State != v1alpha1.ArtifactBuildStateComplete {
					ta.Logf(fmt.Sprintf("artifactbuild %s not complete", ab.Spec.GAV))
					abComplete = false
					break
				}
			}
			dbList, err := jvmClient.JvmbuildserviceV1alpha1().DependencyBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error list dependencybuilds: %s", err.Error()))
				return false, err
			}
			dbComplete := len(dbList.Items) > 0
			ta.Logf(fmt.Sprintf("number of dependencybuilds: %d", len(dbList.Items)))
			dbCompleteCount := 0
			for _, db := range dbList.Items {
				if db.Status.State == v1alpha1.DependencyBuildStateFailed {
					ta.Logf(fmt.Sprintf("depedencybuild %s FAILED", db.Spec.ScmInfo.SCMURL))
					return false, fmt.Errorf("depedencybuild %s for repo %s FAILED", db.Name, db.Spec.ScmInfo.SCMURL)
				} else if db.Status.State != v1alpha1.DependencyBuildStateComplete {
					if dbComplete {
						//only print the first one
						ta.Logf(fmt.Sprintf("depedencybuild %s not complete", db.Spec.ScmInfo.SCMURL))
					}
					dbComplete = false
				} else if db.Status.State == v1alpha1.DependencyBuildStateComplete {
					dbCompleteCount++
				}
			}
			if abComplete && dbComplete {
				return true, nil
			}
			ta.Logf(fmt.Sprintf("completed %d/%d DependencyBuilds", dbCompleteCount, len(dbList.Items)))
			return false, nil
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for some artifactbuilds and dependencybuilds to complete")
		}
	})

	if len(set) > 0 {
		//no futher checks required here
		//we are just checking that the GAVs in question actually build
		return
	}

	ta.t.Run("contaminated build is resolved", func(t *testing.T) {
		//our sample repo has shaded-jdk11 which is contaminated by simple-jdk8
		var contaminated string
		var simpleJDK8 string
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, 3*ta.timeout, true, func(ctx context.Context) (done bool, err error) {

			dbContaminated := false
			shadedComplete := false
			contaminantBuild := false
			dbList, err := jvmClient.JvmbuildserviceV1alpha1().DependencyBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error list dependencybuilds: %s", err.Error()))
				return false, err
			}
			ta.Logf(fmt.Sprintf("number of dependencybuilds: %d", len(dbList.Items)))
			for _, db := range dbList.Items {
				if db.Status.State == v1alpha1.DependencyBuildStateContaminated {
					dbContaminated = true
					contaminated = db.Name
					break
				} else if strings.Contains(db.Spec.ScmInfo.SCMURL, "shaded-jdk11") && db.Status.State == v1alpha1.DependencyBuildStateComplete {
					//its also possible that the build has already resolved itself
					contaminated = db.Name
					shadedComplete = true
				} else if strings.Contains(db.Spec.ScmInfo.SCMURL, "simple-jdk8") {
					contaminantBuild = true
				}
			}
			if dbContaminated || (shadedComplete && contaminantBuild) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for contaminated build to appear")
		}
		ta.Logf(fmt.Sprintf("contaminated dependencybuild: %s", contaminated))
		//make sure simple-jdk8 was requested as a result
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, 2*ta.timeout, true, func(ctx context.Context) (done bool, err error) {
			abList, err := jvmClient.JvmbuildserviceV1alpha1().ArtifactBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error list artifactbuilds: %s", err.Error()))
				return false, err
			}
			found := false
			ta.Logf(fmt.Sprintf("number of artifactbuilds: %d", len(abList.Items)))
			for _, ab := range abList.Items {
				if strings.Contains(ab.Spec.GAV, "simple-jdk8") {
					simpleJDK8 = ab.Name
					found = true
					break
				}
			}
			return found, nil
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for simple-jdk8 to appear as an artifactbuild")
		}
		//now make sure simple-jdk8 eventually completes
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, 2*ta.timeout, true, func(ctx context.Context) (done bool, err error) {
			ab, err := jvmClient.JvmbuildserviceV1alpha1().ArtifactBuilds(ta.ns).Get(context.TODO(), simpleJDK8, metav1.GetOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error getting simple-jdk8 ArtifactBuild: %s", err.Error()))
				return false, err
			}
			ta.Logf(fmt.Sprintf("simple-jdk8 State: %s", ab.Status.State))
			return ab.Status.State == v1alpha1.ArtifactBuildStateComplete, nil
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for simple-jdk8 to complete")
		}
		//now make sure shaded-jdk11 eventually completes
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, 2*ta.timeout, true, func(ctx context.Context) (done bool, err error) {
			db, err := jvmClient.JvmbuildserviceV1alpha1().DependencyBuilds(ta.ns).Get(context.TODO(), contaminated, metav1.GetOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error getting shaded-jdk11 DependencyBuild: %s", err.Error()))
				return false, err
			}
			ta.Logf(fmt.Sprintf("shaded-jdk11 State: %s", db.Status.State))
			if db.Status.State == v1alpha1.DependencyBuildStateFailed {
				msg := fmt.Sprintf("contaminated db %s failed, exitting wait", contaminated)
				ta.Logf(msg)
				return false, fmt.Errorf(msg)
			}
			return db.Status.State == v1alpha1.DependencyBuildStateComplete, err
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for shaded-jdk11 to complete")
		}
	})

	ta.t.Run("make sure second build access cached dependencies", func(t *testing.T) {
		//first delete all existing PipelineRuns to free up resources
		//mostly for minikube
		runs, lerr := tektonClient.TektonV1beta1().PipelineRuns(ta.ns).List(context.TODO(), metav1.ListOptions{})
		if lerr != nil {
			debugAndFailTest(ta, fmt.Sprintf("error listing runs %s", lerr.Error()))
		}
		for _, r := range runs.Items {
			err := tektonClient.TektonV1beta1().PipelineRuns(ta.ns).Delete(context.TODO(), r.Name, metav1.DeleteOptions{})
			if err != nil {
				debugAndFailTest(ta, fmt.Sprintf("error deleting runs %s", err.Error()))
			}
		}

		ta.run = &v1beta1.PipelineRun{}
		obj = streamFileYamlToTektonObj(runYamlPath, ta.run, ta)
		ta.run, ok = obj.(*v1beta1.PipelineRun)
		if !ok {
			debugAndFailTest(ta, fmt.Sprintf("file %s did not produce a pipelinerun: %#v", runYamlPath, obj))
		}
		ta.run, err = tektonClient.TektonV1beta1().PipelineRuns(ta.ns).Create(context.TODO(), ta.run, metav1.CreateOptions{})
		if err != nil {
			debugAndFailTest(ta, err.Error())
		}

		ctx := context.TODO()
		watch, werr := tektonClient.TektonV1beta1().PipelineRuns(ta.ns).Watch(ctx, metav1.ListOptions{})
		if werr != nil {
			debugAndFailTest(ta, fmt.Sprintf("error creating watch %s", werr.Error()))
		}
		defer watch.Stop()

		exitForLoop := false
		podClient := kubeClient.CoreV1().Pods(ta.ns)

		for {
			select {
			// technically this is not needed, since we just created the context above; but if go testing changes
			// such that it carries a context, we'll want to use that here
			case <-ctx.Done():
				ta.Logf("context done")
				exitForLoop = true
				break
			case <-time.After(15 * time.Minute):
				msg := "timed out waiting for second build to complete"
				ta.Logf(msg)
				// call stop here in case the defer is bypass by a call to t.Fatal
				watch.Stop()
				debugAndFailTest(ta, msg)
			case event := <-watch.ResultChan():
				if event.Object == nil {
					continue
				}
				pr, ok := event.Object.(*v1beta1.PipelineRun)
				if !ok {
					continue
				}
				if pr.Name != ta.run.Name {
					if pr.IsDone() {
						ta.Logf(fmt.Sprintf("got event for pipelinerun %s in a terminal state", pr.Name))
						continue
					}
					debugAndFailTest(ta, fmt.Sprintf("another non-completed pipeline run %s was generated when it should not", pr.Name))
				}
				ta.Logf(fmt.Sprintf("done processing event for pr %s", pr.Name))
				if pr.IsDone() {
					pods := prPods(ta, pr.Name)
					if len(pods) == 0 {
						debugAndFailTest(ta, fmt.Sprintf("pod for pipelinerun %s unexpectedly missing", pr.Name))
					}
					containers := []corev1.Container{}
					containers = append(containers, pods[0].Spec.InitContainers...)
					containers = append(containers, pods[0].Spec.Containers...)
					for _, container := range containers {
						if !strings.Contains(container.Name, "analyse-dependencies") {
							continue
						}
						req := podClient.GetLogs(pods[0].Name, &corev1.PodLogOptions{Container: container.Name})
						readCloser, err := req.Stream(context.TODO())
						if err != nil {
							ta.Logf(fmt.Sprintf("error getting pod logs for container %s: %s", container.Name, err.Error()))
							continue
						}
						b, err := io.ReadAll(readCloser)
						if err != nil {
							ta.Logf(fmt.Sprintf("error reading pod stream %s", err.Error()))
							continue
						}
						cLog := string(b)
						if strings.Contains(cLog, "\"publisher\" : \"central\"") {
							debugAndFailTest(ta, fmt.Sprintf("pipelinerun %s has container %s with dep analysis still pointing to central %s", pr.Name, container.Name, cLog))
						}
						if !strings.Contains(cLog, "\"publisher\" : \"rebuilt\"") {
							debugAndFailTest(ta, fmt.Sprintf("pipelinerun %s has container %s with dep analysis that does not access rebuilt %s", pr.Name, container.Name, cLog))
						}
						if !strings.Contains(cLog, "\"java:scm-uri\" : \"https://github.com/stuartwdouglas/hacbs-test-simple-jdk8.git\"") {
							debugAndFailTest(ta, fmt.Sprintf("pipelinerun %s has container %s with dep analysis did not include java:scm-uri %s", pr.Name, container.Name, cLog))
						}
						if !strings.Contains(cLog, "\"java:scm-commit\" : \"") {
							debugAndFailTest(ta, fmt.Sprintf("pipelinerun %s has container %s with dep analysis did not include java:scm-commit %s", pr.Name, container.Name, cLog))
						}
						break
					}
					ta.Logf(fmt.Sprintf("pr %s is done and has correct analyse-dependencies output, exiting", pr.Name))
					exitForLoop = true
					break
				}
			}
			if exitForLoop {
				break
			}

		}
	})
	ta.t.Run("Correct JDK identified for JDK11 build", func(t *testing.T) {
		//test that we don't attempt to use JDK8 on a JDK11+ project
		err = wait.PollUntilContextTimeout(context.TODO(), ta.interval, 2*ta.timeout, true, func(ctx context.Context) (done bool, err error) {

			dbList, err := jvmClient.JvmbuildserviceV1alpha1().DependencyBuilds(ta.ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				ta.Logf(fmt.Sprintf("error list dependencybuilds: %s", err.Error()))
				return false, err
			}
			ta.Logf(fmt.Sprintf("number of dependencybuilds: %d", len(dbList.Items)))
			for _, db := range dbList.Items {
				if !strings.Contains(db.Spec.ScmInfo.SCMURL, "shaded-jdk11") ||
					db.Status.State == "" ||
					db.Status.State == v1alpha1.DependencyBuildStateNew ||
					db.Status.State == v1alpha1.DependencyBuildStateAnalyzeBuild {
					continue
				}
				jdk7 := false
				jdk8 := false
				jdk11 := false
				jdk17 := false
				for _, i := range db.Status.PotentialBuildRecipes {
					jdk7 = jdk7 || i.JavaVersion == "7"
					jdk8 = jdk8 || i.JavaVersion == "8"
					jdk11 = jdk11 || i.JavaVersion == "11"
					jdk17 = jdk17 || i.JavaVersion == "17"
				}
				for _, i := range db.Status.BuildAttempts {
					jdk7 = jdk7 || i.Recipe.JavaVersion == "7"
					jdk8 = jdk8 || i.Recipe.JavaVersion == "8"
					jdk11 = jdk11 || i.Recipe.JavaVersion == "11"
					jdk17 = jdk17 || i.Recipe.JavaVersion == "17"
				}

				if jdk7 {
					return false, fmt.Errorf("build should not have been attempted with jdk7")
				}
				if jdk8 {
					return false, fmt.Errorf("build should not have been attempted with jdk8")
				}
				if !jdk11 {
					return false, fmt.Errorf("build should have been attempted with jdk11")
				}
				if !jdk17 {
					return false, fmt.Errorf("build should have been attempted with jdk17")
				}
				return true, nil

			}
			return false, nil
		})
		if err != nil {
			debugAndFailTest(ta, "timed out waiting for contaminated build to appear")
		}
	})

	ta.t.Run("Logs and source included in image", func(t *testing.T) {

		rebuiltList, err := jvmClient.JvmbuildserviceV1alpha1().RebuiltArtifacts(ta.ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		for _, artifact := range rebuiltList.Items {
			sourceFound := false
			logsFound := false
			sbomFound := false
			imageName := artifact.Spec.Image

			if !strings.Contains(imageName, "quay.io") {
				//skip this on the minikube tests
				//we can't access the internal image registry
				continue
			}

			println(imageName)
			ref, err := name.ParseReference(imageName)
			if err != nil {
				panic(err)
			}

			rmt, err := remote.Get(ref)
			if err != nil {
				panic(err)
			}

			img, err := rmt.Image()
			if err != nil {
				panic(err)
			}
			export := path + "/foo.tar"
			if err := crane.Save(img, "foo", export); err != nil {
				panic(err)
			}
			f, err := os.Open(export) //#nosec G304
			if err != nil {
				panic(err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()
			tr := tar.NewReader(f)
			for {
				cur, err := tr.Next()
				if err == io.EOF {
					break
				} else if err != nil {
					panic(err)
				}
				if cur.Typeflag != tar.TypeReg {
					continue
				}
				println(cur.Name)
				if strings.HasSuffix(cur.Name, ".tar.gz") {
					zip, err := gzip.NewReader(tr)
					if err != nil {
						panic(err)
					}
					innerTar := tar.NewReader(zip)
					for {
						cur, err := innerTar.Next()
						if err == io.EOF {
							break
						} else if err != nil {
							panic(err)
						}
						if cur.Typeflag != tar.TypeReg {
							continue
						}
						println(cur.Name)
						if cur.Name == "logs/maven.log" || cur.Name == "logs/gradle.log" {
							logsFound = true
						} else if cur.Name == "source/.git/HEAD" {
							sourceFound = true
						} else if cur.Name == "logs/build-sbom.json" {
							sbomFound = true
							bom := &cdx.BOM{}
							decoder := cdx.NewBOMDecoder(innerTar, cdx.BOMFileFormatJSON)
							if err = decoder.Decode(bom); err != nil {
								panic(err)
							}
							if bom.Dependencies != nil && len(*bom.Dependencies) < 30 {
								_ = cdx.NewBOMEncoder(os.Stdout, cdx.BOMFileFormatXML).
									SetPretty(true).
									Encode(bom)
								panic("Not enough dependencies in build SBom")
							}
						}
					}
				}
			}
			if !sourceFound {
				panic("Source was not found in image " + imageName)
			}
			if !logsFound {
				panic("Logs were not found in image " + imageName)
			}
			if !sbomFound {
				panic("Build SBOM were not found in image " + imageName)
			}
		}
	})
}

func watchEvents(eventClient v1.EventInterface, ta *testArgs) {
	ctx := context.TODO()
	watch, err := eventClient.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for {
		res := <-watch.ResultChan()
		if res.Object == nil {
			continue
		}
		event, ok := res.Object.(*v12.Event)
		if !ok {
			continue
		}
		if event.Type == corev1.EventTypeNormal {
			continue
		}
		ta.Logf(fmt.Sprintf("non-normal event reason %s about obj %s:%s message %s", event.Reason, event.Regarding.Kind, event.Regarding.Name, event.Note))
	}
}
