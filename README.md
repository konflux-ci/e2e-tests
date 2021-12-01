# E2E Red Hat App Studio Framework

Testing framework solution written in golang using ginkgo framework to cover Red Hat AppStudio.

# Specifications

* Instrumented tests with ginkgo v2 framework. Find more info: <https://docs.google.com/document/d/1h28ZknXRsTLPNNiOjdHIO-F2toCzq4xoZDXbfYaBdoQ/edit#heading=h.ptojc6n4azyr>
* Use client-go to connect to Openshift Cluster.
* Ability to run the E2E tests everywhere; locally, Openshift Cluster, OSD...
* Writes out a junit XML/JSON file with tests results to a custom directory by using `--ginkgo.junit(or json)-report` flag.
* Ability to run test suites separetly.

# Setup

Before executing the e2e suites you need to have deployed App Studio component/s to your cluster.

Log into your openshift cluster, using `oc login -u <user> -p <password> <oc_api_url>.`

A properly setup Go workspace using **Go 1.17 is required**.

Install dependencies:

```
# Install dependencies
$ go mod tidy
# Copy the dependencies to vendor folder
$ go mod vendor
# Create `e2e-appstudio` binary in bin folder. Please add the binary to the path or just execute `./bin/e2e-appstudio`
$ make build
```

## The `e2e-appstudio` command

The `e2e-appstudio` command is the root command that executes all test functionality. To obtain all available flags for the binary please use `--help` flags. All ginkgo
flags and go tests are available in `e2e-appstudio` binary.

# Develop new tests

* Create test folder under framework folder: `pkg/framework/[<application-name>]...`, e.g.
  * `pkg/framework/application-service` - all tests used owned by App Studio application team
* Every test package should be imported to `cmd/e2e_test.go`, e.g.

```golang
// cmd/e2e_test.go
package common

import (
	// ensure these packages are scanned by ginkgo for e2e tests
	_ "github.com/redhat-appstudio/e2e-tests/pkg/framework/application-service"
	_ "github.com/redhat-appstudio/e2e-tests/pkg/framework/common"
)
```
