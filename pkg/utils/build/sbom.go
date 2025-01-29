package build

import (
	"encoding/json"
	"fmt"
)

type Sbom interface {
	GetPackages() []SbomPackage
}

type SbomPackage interface {
	GetName() string
	GetVersion() string
	GetPurl() string
}

type SbomCyclonedx struct {
	BomFormat   string
	SpecVersion string
	Version     int
	Components  []CyclonedxComponent `json:"components"`
}

type CyclonedxComponent struct {
	Name    string `json:"name"`
	Purl    string `json:"purl"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

func (s *SbomCyclonedx) GetPackages() []SbomPackage {
	packages := []SbomPackage{}
	for i := range s.Components {
		packages = append(packages, &s.Components[i])
	}
	return packages
}

func (c *CyclonedxComponent) GetName() string {
	return c.Name
}

func (c *CyclonedxComponent) GetVersion() string {
	return c.Version
}

func (c *CyclonedxComponent) GetPurl() string {
	return c.Purl
}

func UnmarshalSbom(data []byte) (Sbom, error) {
	cdx := SbomCyclonedx{}
	if err := json.Unmarshal(data, &cdx); err != nil {
		return nil, fmt.Errorf("unmarshalling SBOM: %w", err)
	}
	if cdx.BomFormat != "" {
		return &cdx, nil
	}
	return nil, fmt.Errorf("unmarshalling SBOM: doesn't look like CycloneDX")
}
