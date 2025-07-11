# Conforma (formerly Enterprise Contract) 

Conforma (formerly Enterprise Contract) is a set of tools for verifying the provenance of container images built in Red Hat Trusted Application Pipeline and validating them against a clearly defined policy.

The Conforma policy is defined using the rego policy language and is described here in [Release Policy](https://conforma.dev/docs/policy/release_policy.html) and [Pipeline Policy](https://conforma.dev/docs/policy/pipeline_policy.html)

The enterprise-contract suite contains a set of tests that covers Conforma policies.

Steps to run 'enterprise-contract-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="enterprise-contract-suite"`