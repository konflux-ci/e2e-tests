/* SBOM of type structures matches Pyxis structure

When SBOM files are uploaded to Pyxis it's required to be modified according to
Pyxis's requirment where the CamelBack method does not match,
in scrip https://github.com/redhat-appstudio/release-service-utils/blob/main/pyxis/upload_sbom.py
was made this match, underscore was between two words instead of CamelBack.

*/

package release

// Links defines structure of all links can be in SBOM of Pyxis style
type Links struct {
	Artifacts       ArtifactLinks        `json:"artifacts"`
	Requests        RequestLinks         `json:"requests"`
	RpmManifest     RpmManifestLinks     `json:"rpm_manifest"`
	TestResults     TestResultsLinks     `json:"test_results"`
	Vulnerabilities VulnerabilitiesLinks `json:"vulnerabilities"`
}

type ArtifactLinks struct {
	Href string `json:"href"`
}

type RequestLinks struct {
	Href string `json:"href"`
}

type RpmManifestLinks struct {
	Href string `json:"href"`
}

type TestResultsLinks struct {
	Href string `json:"href"`
}

type VulnerabilitiesLinks struct {
	Href string `json:"href"`
}

// ContentManifest id of content manifest
type ContentManifest struct {
	ID string `json:"_id"`
}

// ContentManifestComponent contains information of components in SBOM
type ContentManifestComponent struct {
	ID      string `json:"_id"`
	Name    string `json:"name"`
	Purl    string `json:"purl"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

type FreshnessGrade struct {
	CreationDate string `json:"creation_date"`
	Grade        string `json:"grade"`
	StartDate    string `json:"start_date"`
}

// ParsedData general details of env
type ParsedData struct {
	Architecture  string   `json:"architecture"`
	DockerVersion string   `json:"docker_version"`
	EnvVariables  []string `json:"env_variables"`
}

// Image contains all information of all structured SBOM Pyxis style
type Image struct {
	ID                        string                     `json:"_id"`
	Links                     Links                      `json:"_links"`
	Architecture              string                     `json:"architecture"`
	Certified                 bool                       `json:"certified"`
	ContentManifest           ContentManifest            `json:"content_manifest"`
	ContentManifestComponents []ContentManifestComponent `json:"content_manifest_components"`
	CreatedBy                 string                     `json:"created_by"`
	CreatedOnBehalfOf         interface{}                `json:"created_on_behalf_of"`
	CreationDate              string                     `json:"creation_date"`
	DockerImageDigest         string                     `json:"docker_image_digest"`
	FreshnessGrades           []FreshnessGrade           `json:"freshness_grades"`
	ImageID                   string                     `json:"image_id"`
	LastUpdateDate            string                     `json:"last_update_date"`
	LastUpdatedBy             string                     `json:"last_updated_by"`
	ObjectType                string                     `json:"object_type"`
	ParsedData                ParsedData                 `json:"parsed_data"`
}
