package build



type SbomCyclonedx struct {
	BomFormat   string
	SpecVersion string
	Version     int
	Components  []struct {
		Name    string `json:"name"`
		Purl    string `json:"purl"`
		Type    string `json:"type"`
		Version string `json:"version"`
	} `json:"components"`
}
