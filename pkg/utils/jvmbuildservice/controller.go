package jvmbuildservice

import (
	"context"
	"strconv"
	"time"

	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/apis/kubernetes"
	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SuiteController struct {
	*kubeCl.K8sClient
}

func NewSuiteControler(kube *kubeCl.K8sClient) (*SuiteController, error) {
	return &SuiteController{
		kube,
	}, nil
}

func (s *SuiteController) ListArtifactBuilds(namespace string) (*v1alpha1.ArtifactBuildList, error) {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) DeleteArtifactBuild(name, namespace string) error {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().ArtifactBuilds(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (s *SuiteController) ListDependencyBuilds(namespace string) (*v1alpha1.DependencyBuildList, error) {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().DependencyBuilds(namespace).List(context.TODO(), metav1.ListOptions{})
}

func (s *SuiteController) DeleteDependencyBuild(name, namespace string) error {
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().DependencyBuilds(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (s *SuiteController) CreateJBSConfig(name, namespace, imageRegistryOwner string) (*v1alpha1.JBSConfig, error) {
	config := &v1alpha1.JBSConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.JBSConfigSpec{
			EnableRebuilds:              true,
			RequireArtifactVerification: true,
			MavenBaseLocations: map[string]string{
				"maven-repository-300-jboss":                      "https://repository.jboss.org/nexus/content/groups/public/",
				"maven-repository-301-gradleplugins":              "https://plugins.gradle.org/m2",
				"maven-repository-302-confluent":                  "https://packages.confluent.io/maven",
				"maven-repository-303-gradle":                     "https://repo.gradle.org/artifactory/libs-releases",
				"maven-repository-304-eclipselink":                "https://download.eclipse.org/rt/eclipselink/maven.repo",
				"maven-repository-305-redhat":                     "https://maven.repository.redhat.com/ga",
				"maven-repository-306-jitpack":                    "https://jitpack.io",
				"maven-repository-307-jsweet":                     "https://repository.jsweet.org/artifactory/libs-release-local",
				"maven-repository-308-jenkins":                    "https://repo.jenkins-ci.org/public/",
				"maven-repository-309-spring-plugins":             "https://repo.springsource.org/plugins-release",
				"maven-repository-310-dokkadev":                   "https://maven.pkg.jetbrains.space/kotlin/p/dokka/dev",
				"maven-repository-311-ajoberstar":                 "https://ajoberstar.org/bintray-backup",
				"maven-repository-312-googleandroid":              "https://dl.google.com/dl/android/maven2/",
				"maven-repository-313-kotlinnative14linux":        "https://download.jetbrains.com/kotlin/native/builds/releases/1.4/linux",
				"maven-repository-314-jcs":                        "https://packages.jetbrains.team/maven/p/jcs/maven",
				"maven-repository-315-kotlin-bootstrap":           "https://maven.pkg.jetbrains.space/kotlin/p/kotlin/bootstrap/",
				"maven-repository-315-kotlin-kotlin-dependencies": "https://maven.pkg.jetbrains.space/kotlin/p/kotlin/kotlin-dependencies"},
			ImageRegistry: v1alpha1.ImageRegistry{
				Host:       "quay.io",
				Owner:      imageRegistryOwner,
				Repository: "test-images",
				PrependTag: strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
			CacheSettings: v1alpha1.CacheSettings{},
			BuildSettings: v1alpha1.BuildSettings{},
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
	}
	return s.JvmbuildserviceClient().JvmbuildserviceV1alpha1().JBSConfigs(namespace).Create(context.TODO(), config, metav1.CreateOptions{})
}
