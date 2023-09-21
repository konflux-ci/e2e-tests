# Red Hat AppStudio E2E Tests and Testing Framework

Testing framework and E2E tests are written in [Go](https://go.dev/) using [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) frameworks to cover Red Hat AppStudio.
It is recommended to install AppStudio in E2E mode, but the E2E suite can be also usable in [development and preview modes](https://github.com/redhat-appstudio/infra-deployments#preview-mode-for-your-clusters).

# Features

* Instrumented tests with Ginkgo 2.0 framework. You can find more information in [Ginkgo documentation](https://onsi.github.io/ginkgo/).
* Uses client-go to connect to OpenShift Cluster.
* Ability to run the E2E tests everywhere: locally([CRC/OpenShift local](https://developers.redhat.com/products/openshift-local/overview)), OpenShift Cluster, OSD...
* Writes tests results in JUnit XML/JSON file to a custom directory by using `--ginkgo.junit(or json)-report` flag.
* Ability to run the test suites separately.

## To start running E2E tests


## To start developing tests

To develop new tests in the RHTAP developer consider first to read our tips for a better experience:
* Basic tips to write readable tests. [Documentation](docs/Guidelines.md)
* How to auto generate tests. [Documentation](docs/DeveloperGenerateTest.md)

## To start debugging CI
