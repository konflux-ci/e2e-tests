package contract

import ecp "github.com/enterprise-contract/enterprise-contract-controller/api/v1alpha1"

// PolicySpecWithSourceConfig returns a new EnterpriseContractPolicySpec which is a deep copy of
// the provided spec with each source config updated.
func PolicySpecWithSourceConfig(spec ecp.EnterpriseContractPolicySpec, sourceConfig ecp.SourceConfig) ecp.EnterpriseContractPolicySpec {
	var sources []ecp.Source
	for _, s := range spec.Sources {
		source := s.DeepCopy()
		source.Config = sourceConfig.DeepCopy()
		sources = append(sources, *source)
	}

	newSpec := *spec.DeepCopy()
	newSpec.Sources = sources
	return newSpec
}
