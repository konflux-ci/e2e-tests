package jvmbuildservice

import (
	"context"
	"strconv"
	"time"

	"github.com/redhat-appstudio/jvm-build-service/pkg/apis/jvmbuildservice/v1alpha1"
	"github.com/redhat-appstudio/jvm-build-service/pkg/reconciler/jbsconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateJBSConfig creates a JBSConfig object.
func (j *JvmbuildserviceController) CreateJBSConfig(name, namespace string) (*v1alpha1.JBSConfig, error) {
	config := &v1alpha1.JBSConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{jbsconfig.DeleteImageRepositoryAnnotationName: "true"}},
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
				PrependTag: strconv.FormatInt(time.Now().UnixMilli(), 10),
			},
			CacheSettings: v1alpha1.CacheSettings{
				RequestMemory: "256Mi",
				RequestCPU:    "100m",
				Storage:       "1Gi",
			},
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
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().JBSConfigs(namespace).Create(context.TODO(), config, metav1.CreateOptions{})
}

// DeleteJBSConfig removes a JBSConfig from a given namespace.
func (j *JvmbuildserviceController) DeleteJBSConfig(name string, namespace string) error {
	return j.JvmbuildserviceClient().JvmbuildserviceV1alpha1().JBSConfigs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
