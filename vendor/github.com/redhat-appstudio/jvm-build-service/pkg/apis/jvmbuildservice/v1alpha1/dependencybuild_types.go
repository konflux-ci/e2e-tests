package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DependencyBuildState string

const (
	DependencyBuildStateNew          = "DependencyBuildStateNew"
	DependencyBuildStateAnalyzeBuild = "DependencyBuildStateAnalyzeBuild"
	DependencyBuildStateSubmitBuild  = "DependencyBuildStateSubmitBuild"
	DependencyBuildStateBuilding     = "DependencyBuildStateBuilding"
	DependencyBuildStateComplete     = "DependencyBuildStateComplete"
	DependencyBuildStateFailed       = "DependencyBuildStateFailed"
	DependencyBuildStateContaminated = "DependencyBuildStateContaminated"
)

type DependencyBuildSpec struct {
	ScmInfo SCMInfo `json:"scm,omitempty"`
	Version string  `json:"version,omitempty"`
}

type DependencyBuildStatus struct {
	// Conditions for capturing generic status
	// NOTE: inspecting the fabric8 Status class, it looked analogous to k8s Condition,
	// and then I took the liberty of making it an array, given best practices in the k8s/ocp ecosystems
	Conditions   []metav1.Condition `json:"conditions,omitempty"`
	State        string             `json:"state,omitempty"`
	Message      string             `json:"message,omitempty"`
	Contaminants []Contaminant      `json:"contaminates,omitempty"`
	//BuildRecipe the current build recipe. If build is done then this recipe was used
	//to get to the current state
	// Deprecated
	DeprecatedCurrentBuildRecipe *BuildRecipe `json:"currentBuildRecipe,omitempty"`
	// PotentialBuildRecipes additional recipes to try if the current recipe fails
	PotentialBuildRecipes []*BuildRecipe `json:"potentialBuildRecipes,omitempty"`
	//FailedBuildRecipes recipes that resulted in a failure
	//if the current state is failed this may include the current BuildRecipe
	//Deprecated
	DeprecatedFailedBuildRecipes []*BuildRecipe `json:"failedBuildRecipes,omitempty"`
	//Deprecated
	DeprecatedLastCompletedBuildPipelineRun string   `json:"lastCompletedBuildPipelineRun,omitempty"`
	CommitTime                              int64    `json:"commitTime,omitempty"`
	DeployedArtifacts                       []string `json:"deployedArtifacts,omitempty"`
	FailedVerification                      bool     `json:"failedVerification,omitempty"`
	Hermetic                                bool     `json:"hermetic,omitempty"`
	// Deprecated
	DeprecatedDiagnosticDockerFiles []string         `json:"diagnosticDockerFiles,omitempty"`
	PipelineRetries                 int              `json:"pipelineRetries,omitempty"`
	BuildAttempts                   []*BuildAttempt  `json:"buildAttempts,omitempty"`
	DiscoveryPipelineResults        *PipelineResults `json:"discoveryPipelineResults,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=dependencybuilds,scope=Namespaced
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.scm.scmURL`
// +kubebuilder:printcolumn:name="Tag",type=string,JSONPath=`.spec.scm.tag`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Message",type=string,JSONPath=`.status.message`

// DependencyBuild TODO provide godoc description
type DependencyBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DependencyBuildSpec   `json:"spec"`
	Status DependencyBuildStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DependencyBuildList contains a list of DependencyBuild
type DependencyBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DependencyBuild `json:"items"`
}

type BuildAttempt struct {
	Recipe *BuildRecipe      `json:"buildRecipe,omitempty"`
	Build  *BuildPipelineRun `json:"build,omitempty"`
}

type BuildPipelineRun struct {
	PipelineName         string                   `json:"pipelineName"`
	Complete             bool                     `json:"complete"`
	Succeeded            bool                     `json:"succeeded,omitempty"`
	DiagnosticDockerFile string                   `json:"diagnosticDockerFile,omitempty"`
	Results              *BuildPipelineRunResults `json:"results,omitempty"`
}

type BuildPipelineRunResults struct {
	//the image resulting from the run
	Image       string `json:"image,omitempty"`
	ImageDigest string `json:"imageDigest"`
	//If the resulting image was verified
	Verified bool `json:"verified,omitempty"`
	// The produced GAVs
	Gavs []string `json:"gavs,omitempty"`
	// The hermetic build image produced by the build
	HermeticBuildImage string `json:"hermeticBuildImage,omitempty"`

	// The Tekton results
	PipelineResults *PipelineResults `json:"pipelineResults,omitempty"`
}

func (r *DependencyBuildStatus) GetBuildPipelineRun(pipeline string) *BuildAttempt {
	for i := range r.BuildAttempts {
		ba := r.BuildAttempts[i]
		if ba.Build != nil {
			if ba.Build.PipelineName == pipeline {
				return ba
			}
		}
	}
	return nil
}
func (r *DependencyBuildStatus) CurrentBuildAttempt() *BuildAttempt {
	if len(r.BuildAttempts) == 0 {
		return nil
	}
	return r.BuildAttempts[len(r.BuildAttempts)-1]
}

type BuildRecipe struct {
	Pipeline            string               `json:"pipeline,omitempty"`
	Tool                string               `json:"tool,omitempty"`
	Image               string               `json:"image,omitempty"`
	CommandLine         []string             `json:"commandLine,omitempty"`
	EnforceVersion      string               `json:"enforceVersion,omitempty"`
	ToolVersion         string               `json:"toolVersion,omitempty"`
	JavaVersion         string               `json:"javaVersion,omitempty"`
	PreBuildScript      string               `json:"preBuildScript,omitempty"`
	PostBuildScript     string               `json:"postBuildScript,omitempty"`
	AdditionalDownloads []AdditionalDownload `json:"additionalDownloads,omitempty"`
	DisableSubmodules   bool                 `json:"disableSubmodules,omitempty"`
	AdditionalMemory    int                  `json:"additionalMemory,omitempty"`
	Repositories        []string             `json:"repositories,omitempty"`
	AllowedDifferences  []string             `json:"allowedDifferences,omitempty"`
}
type Contaminant struct {
	GAV                   string   `json:"gav,omitempty"`
	ContaminatedArtifacts []string `json:"contaminatedArtifacts,omitempty"`
}
type AdditionalDownload struct {
	Uri         string `json:"uri,omitempty"`
	Sha256      string `json:"sha256,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	BinaryPath  string `json:"binaryPath,omitempty"`
	PackageName string `json:"packageName,omitempty"`
	FileType    string `json:"type"`
}

// A representation of the Tekton Results records for a pipeline
type PipelineResults struct {
	Result string `json:"result,omitempty"`
	Record string `json:"record,omitempty"`
	Logs   string `json:"logs,omitempty"`
}
