# Build tests

Contains E2E tests related to [Enterprise components](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/enterprise-contract).

Steps to run tests within `enterprise-contract` directory:

1. Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2. Run the enterprise-contract suite: `./bin/e2e-appstudio --ginkgo.focus="enterprise-contract"`

## Enterprise Contract service suite specs

1. Verify rules are applied to pipeline run attestations associated with container images built by HACBS
2. Verify rules are applied to Tekton pipeline definitions.
