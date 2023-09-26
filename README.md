# RHTAP E2E Tests and Testing Framework

Testing framework and E2E tests are written in [Go](https://go.dev/) using [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) frameworks to cover Red Hat AppStudio.
It is recommended to install AppStudio in E2E mode, but the E2E suite can be also usable in [development and preview modes](https://github.com/redhat-appstudio/infra-deployments#preview-mode-for-your-clusters).

# Features

* Instrumented tests with Ginkgo 2.0 framework. You can find more information in [Ginkgo documentation](https://onsi.github.io/ginkgo/).
* Ability to run the E2E tests everywhere: locally([CRC/OpenShift local](https://developers.redhat.com/products/openshift-local/overview)), OpenShift Cluster, OSD...
* Writes tests results in JUnit XML/JSON file to a custom directory by using `--ginkgo.junit(or json)-report` flag.
* Ability to run the test suites separately.

## Start running E2E tests

All the instructions about installing RHTAP locally/CI and running tests locally/CI can be found in this [Documentation](docs/Installation.md), which contains also information about how to pair PR when breaking changes are introduced.

## Start developing tests

To develop new tests in RHTAP consider first to read some tips for a better experience:
* Basic tips to write readable tests. [Documentation](docs/Guidelines.md).
* How to auto generate tests. [Documentation](docs/DeveloperGenerateTest.md).

## Start debugging CI

To onboard a new component in Openshift CI follow this [Documentation](docs/OpenShiftCI.md).
To debug CI jobs follow this [Documentation](docs/InvestigatingCIFailures.md).

***HAPPY TESTING!***

