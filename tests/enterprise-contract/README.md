# Enterprise Contract

The Enterprise Contract is a set of tools for verifing the provenence of container images built in Red Hat Trusted Application Pipeline and validating them against a clearly defined policy.

The Enterprise Contract policy is defined using the rego policy language and is described here in Release Policy [Release Policy](https://redhat-appstudio.github.io/docs.stonesoup.io/ec-policies/release_policy.html) and Pipeline Policy[Pipeline Policy](https://redhat-appstudio.github.io/docs.stonesoup.io/ec-policies/pipeline_policy.html)

The enterprise-contract suite contains a set of tests that covers Enterprise Contract policies.

Steps to run 'enterprise-contract-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="enterprise-contract-suite"`