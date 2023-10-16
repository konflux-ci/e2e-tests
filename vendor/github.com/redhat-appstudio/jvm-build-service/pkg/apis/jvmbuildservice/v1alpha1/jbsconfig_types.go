package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HermeticBuildType string

const (
	JBSConfigName                           = "jvm-build-config"
	ImageSecretName                         = "jvm-build-image-secrets" //#nosec
	GitSecretName                           = "jvm-build-git-secrets"   //#nosec
	TlsSecretName                           = "jvm-build-tls-secrets"   //#nosec
	TlsConfigMapName                        = "jvm-build-tls-ca"        //#nosec
	ImageSecretTokenKey                     = ".dockerconfigjson"       //#nosec
	GitSecretTokenKey                       = ".git-credentials"        //#nosec
	CacheDeploymentName                     = "jvm-build-workspace-artifact-cache"
	ConfigArtifactCacheRequestMemoryDefault = "512Mi"
	ConfigArtifactCacheRequestCPUDefault    = "1"
	ConfigArtifactCacheLimitMemoryDefault   = "512Mi"
	ConfigArtifactCacheLimitCPUDefault      = "4"
	ConfigArtifactCacheIOThreadsDefault     = "4"
	ConfigArtifactCacheWorkerThreadsDefault = "50"
	ConfigArtifactCacheStorageDefault       = "10Gi"

	HermeticBuildTypeNone     HermeticBuildType = "None"
	HermeticBuildTypeRequired HermeticBuildType = "Required"
)

type JBSConfigSpec struct {
	EnableRebuilds bool `json:"enableRebuilds,omitempty"`

	// If this is true then the build will fail if artifact verification fails
	// otherwise deploy will happen as normal, but a field will be set on the DependencyBuild
	RequireArtifactVerification bool              `json:"requireArtifactVerification,omitempty"`
	HermeticBuilds              HermeticBuildType `json:"hermeticBuilds,omitempty"`

	AdditionalRecipes []string `json:"additionalRecipes,omitempty"`

	MavenBaseLocations map[string]string `json:"mavenBaseLocations,omitempty"`

	SharedRegistries []ImageRegistry `json:"sharedRegistries,omitempty"`
	Registry         ImageRegistry   `json:"registry,omitempty"`
	// Deprecated: Replaced by explicit declaration of Registry above.
	ImageRegistry      `json:",inline,omitempty"`
	CacheSettings      CacheSettings              `json:"cacheSettings,omitempty"`
	BuildSettings      BuildSettings              `json:"buildSettings,omitempty"`
	RelocationPatterns []RelocationPatternElement `json:"relocationPatterns,omitempty"`
}

type JBSConfigStatus struct {
	Message          string         `json:"message,omitempty"`
	ImageRegistry    *ImageRegistry `json:"imageRegistry,omitempty"`
	RebuildsPossible bool           `json:"rebuildsPossible,omitempty"`
}

type CacheSettings struct {
	RequestMemory string `json:"requestMemory,omitempty"`
	RequestCPU    string `json:"requestCPU,omitempty"`
	LimitMemory   string `json:"limitMemory,omitempty"`
	LimitCPU      string `json:"limitCPU,omitempty"`
	IOThreads     string `json:"ioThreads,omitempty"`
	WorkerThreads string `json:"workerThreads,omitempty"`
	Storage       string `json:"storage,omitempty"`
	DisableTLS    bool   `json:"disableTLS,omitempty"`
}

type BuildSettings struct {
	// The requested memory for the build and deploy steps of a pipeline
	BuildRequestMemory string `json:"buildRequestMemory,omitempty"`
	// The requested CPU for the build and deploy steps of a pipeline
	BuildRequestCPU string `json:"buildRequestCPU,omitempty"`
	// The requested memory for all other steps of a pipeline
	TaskRequestMemory string `json:"taskRequestMemory,omitempty"`
	// The requested CPU for all other steps of a pipeline
	TaskRequestCPU string `json:"taskRequestCPU,omitempty"`
	// The memory limit for all other steps of a pipeline
	TaskLimitMemory string `json:"taskLimitMemory,omitempty"`
	// The CPU limit for all other steps of a pipeline
	TaskLimitCPU string `json:"taskLimitCPU,omitempty"`
}
type ImageRegistry struct {
	Host       string `json:"host,omitempty"` // Defaults to quay.io in ImageRegistry()
	Port       string `json:"port,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Repository string `json:"repository,omitempty"` // Defaults to artifact-deployments in ImageRegistry()
	Insecure   bool   `json:"insecure,omitempty"`
	PrependTag string `json:"prependTag,omitempty"`
}

type RelocationPatternElement struct {
	RelocationPattern RelocationPattern `json:"relocationPattern"`
}

type RelocationPattern struct {
	BuildPolicy string           `json:"buildPolicy,omitempty" default:"default"`
	Patterns    []PatternElement `json:"patterns,omitempty"`
}

type PatternElement struct {
	Pattern Pattern `json:"pattern"`
}

type Pattern struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=jbsconfigs,scope=Namespaced
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`
// JBSConfig TODO provide godoc description
type JBSConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JBSConfigSpec   `json:"spec"`
	Status JBSConfigStatus `json:"status,omitempty"`
}

func (in *JBSConfig) ImageRegistry() ImageRegistry {
	ret := in.Spec.Registry
	if ret.Host == "" {
		ret.Host = "quay.io"
	}
	if ret.Repository == "" {
		ret.Repository = "artifact-deployments"
	}
	if in.Status.ImageRegistry == nil {
		return ret
	}
	if in.Status.ImageRegistry.Host != "" {
		ret.Host = in.Status.ImageRegistry.Host
	}
	if in.Status.ImageRegistry.Owner != "" {
		ret.Owner = in.Status.ImageRegistry.Owner
	}
	if in.Status.ImageRegistry.Repository != "" {
		ret.Repository = in.Status.ImageRegistry.Repository
	}
	if in.Status.ImageRegistry.Port != "" {
		ret.Port = in.Status.ImageRegistry.Port
	}
	if in.Status.ImageRegistry.PrependTag != "" {
		ret.PrependTag = in.Status.ImageRegistry.PrependTag
	}
	if in.Status.ImageRegistry.Insecure {
		ret.Insecure = true
	}
	return ret
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JBSConfigList contains a list of SystemConfig
type JBSConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JBSConfig `json:"items"`
}
