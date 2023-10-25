# Red Hat AppStudio E2E Tests and Testing Framework

Testing framework and E2E tests are written in [Go](https://go.dev/) using [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) frameworks to cover Red Hat AppStudio.
It is recommended to install AppStudio in E2E mode, but the E2E suite can be also usable in [development and preview modes](https://github.com/redhat-appstudio/infra-deployments#preview-mode-for-your-clusters).

# Features

* Instrumented tests with Ginkgo 2.0 framework. You can find more information in [Ginkgo documentation](https://onsi.github.io/ginkgo/).
* Uses client-go to connect to OpenShift Cluster.
* Ability to run the E2E tests everywhere: locally([CRC/OpenShift local](https://developers.redhat.com/products/openshift-local/overview)), OpenShift Cluster, OSD...
* Writes tests results in JUnit XML/JSON file to a custom directory by using `--ginkgo.junit(or json)-report` flag.
* Ability to run the test suites separately.

# Running the tests
When you want to run the E2E tests for AppStudio you need to have installed tools(in Requirements chapter), installed the AppStudio in E2E mode and compiled `e2e-appstudio` binary.

## Requirements
Requirements for installing AppStudio in E2E mode and running the E2E tests:

* An OpenShift 4.11 or higher Environment (If you are using CRC/OpenShift Local please also review [optional-codeready-containers-post-bootstrap-configuration](https://github.com/redhat-appstudio/infra-deployments#optional-codeready-containers-post-bootstrap-configuration))
* A machine from which to run the install (usually your laptop) with required tools:
  * A properly setup Go workspace using **Go 1.19 is required**
  * The OpenShift Command Line Tool (oc) **Use the version coresponding to the Openshift version**
  * yq
  * jq
  * git
  * helm
* Tokens
  * Github Token with the following permissions
    * `repo`
    * `delete_repo`
  * Valid quay token where to push AppStudio components images generated by the e2e framework

## Install AppStudio in E2E mode

Before executing the e2e suites you need to have deployed AppStudio in E2E Mode to your cluster.

1. Before deploying AppStudio in E2E mode you need to login to your OpenShift cluster with OpenShift Command Line Tool as `admin` (by default  `kubeadmin`):

   ```bash
    oc login -u <user> -p <password> --server=<oc_api_url>
   ```

2. Export required (and recommended) environment variables from [default.env](default.env). Copy the file (`cp default.env user.env`), edit the required variables and source it (`source user.env`).

3. Install dependencies:

``` bash
# Install dependencies
$ go mod tidy
# or go mod tidy -compat=1.19
# Copy the dependencies to vendor folder
$ go mod vendor
```

1. Install Red Hat AppStudio in e2e mode. By default the installation script will use the `redhat-appstudio-qe` GitHub organization for pushing changes to `infra-deployments` repository.

**It is recommended to use your fork of [infra-deployments repo](https://github.com/redhat-appstudio/infra-deployments) in your GitHub org instead** - you can change the GitHub organization with environment variable `export MY_GITHUB_ORG=<name-of-your-github-org>`.

Some tests could require you have a Github App created in order to test Component builds via Pipelines as Code. 
Such tests are [rhtap-demo](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/rhtap-demo/rhtap-demo.go) and [build](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/build/build.go).

In this case, before you bootstrap a cluster, make sure you [created a Github App for your GitHub account](https://github.com/settings/apps). Fill in following details:
* **GitHub App name**: unique app name
* **Homepage URL**: some dummy URL (like https://example.com)
* **Webhook**: mark as active and put some dummy URL to the **Webhook URL** field
* **Permissions** and **Subscribe to events**: refer to [the guide in PaC documentation](https://pipelinesascode.com/docs/install/github_apps/#setup-manually)
* **Where can this GitHub App be installed?**: Any account

Hit create, make a note of the **App ID** and navigate to the **Private keys** section where you generate a private key that gets downloaded automatically. Then export following environment variables (or if you're using [.env file](default.env), update values of following variables):

```
export E2E_PAC_GITHUB_APP_ID='<YOUR_APP_ID>'

export E2E_PAC_GITHUB_APP_PRIVATE_KEY=$(base64 < /PATH/TO/YOUR/DOWNLOADED/PRIVATE_KEY.pem)
```

Navigate back to [your GitHub App](https://github.com/settings/apps), select **Install App** and select your GitHub org (the one that you're using in `MY_GITHUB_ORG` env var). Feel free to install it to all repositories of that organization or the **forked repositories** currently used by [rhtap-demo](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/rhtap-demo/rhtap-demo.go) and [build](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/build/build.go) tests ([`hacbs-test-project`](https://github.com/redhat-appstudio-qe/hacbs-test-project) and [`devfile-sample-hello-world`](https://github.com/redhat-appstudio-qe/devfile-sample-hello-world)).

For bootstrapping a cluster, run the following command:

   ```bash
      make local/cluster/prepare
   ```

More information about how to deploy Red Hat AppStudio
are in the [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) repository.

## Building and running the e2e tests
You can use the following make target to build and run the tests:
   ```bash
      make local/test/e2e
   ```

Or build and run the tests without scripts:
1. Install dependencies and build the tests:

   ``` bash
   # Install dependencies
   $ go mod tidy
   # Copy the dependencies to vendor folder
   $ go mod vendor
   # Create `e2e-appstudio` binary in bin folder. Please add the binary to the path or just execute `./bin/e2e-appstudio`
   $ make build
   ```

2. Run the e2e tests:
The `e2e-appstudio` command is the root command that executes all test functionality. To obtain all available flags for the binary please use `--help` flags. All ginkgo flags and go tests are available in `e2e-appstudio` binary.

Some tests could require you to have specific container image repo's created (if you're using your own container image org/user account (`QUAY_E2E_ORGANIZATION`) or your own GitHub organization (`MY_GITHUB_ORG`)
In that case, before you run the test, make sure you have created
* `test-images` repo in quay.io, i.e. `quay.io/<QUAY_E2E_ORGANIZATION>/test-images` and make it **public**
  * also make sure that the docker config, that is encoded in the value of `QUAY_TOKEN` environment variable, contains a correct credentials required to push to `test-images` repo. And make sure the robot account or user account has the **write** permissions set for `test-images` repo which is required by the tests to push the generated artifacts.
* forked following GitHub repositories to your org (specified in `MY_GITHUB_ORG` env var)
  * https://github.com/redhat-appstudio-qe/devfile-sample-hello-world (for running build-service tests)
  * https://github.com/redhat-appstudio-qe/hacbs-test-project (for rhtap-demo test)
  * https://github.com/redhat-appstudio-qe/strategy-configs (for rhtap-demo test)

   ```bash
    `./bin/e2e-appstudio`
   ```

The instructions for every test suite can be found in the [tests folder](tests), e.g. [has Readme.md](tests/has/README.md).
You can also specify which tests you want to run using [labels](docs/LabelsNaming.md) or [Ginkgo Focus](docs/DeveloperFocus.md).

# Red Hat AppStudio Load Tests

Load tests for AppStudio are also in this repository. More information about load tests are in [LoadTests.md](docs/LoadTests.md).

# Running Red Hat AppStudio Tests in OpenShift CI

Overview for OpenShift CI and AppStudio E2E tests is in [OpenshiftCI.md](docs/OpenShiftCI.md). How to install E2E binary is in [Installation.md](docs/Installation.md).

# Develop new tests

 The current structure of how tests are stored in this repo are as follows:

 * The equivalent of Ginkgo Suites, `*_suite_test.go`, reside in the `cmd/` directory
 * The equivalent of Ginkgo Tests,  `*_test.go`, reside in the `tests/` directory

We've provided some tooling to generate test suite packages and test spec files to get you up and running a little faster:

```bash
      make local/template/generate-test-spec
      make local/template/generate-test-suite
```

For more information refer to [Generate Tests](docs/DeveloperGenerateTest.md).

## Tips
* Make sure you've implemented any required controller functionality that is required for your tests within the following files
   * `pkg/utils/<new controller directory>` - net new controller logic for a new service or component
   * `pkg/framework/framework.go` - import the new controller and update the `Framework` struct to be able to initialize the new controller
* Every test package should be imported to `cmd/e2e_test.go`, e.g. [has](https://github.com/redhat-appstudio/e2e-tests/blob/main/cmd/e2e_test.go#L15).
* Every new test should have correct [labels](docs/LabelsNaming.md).
* Every test should have meaningful description with JIRA/GitHub issue key.
* (Recommended) Use JIRA integration for linking issues and commits (just add JIRA issue key in the commit message).
* When running via mage you can filter the suites run by specifying the
  `E2E_TEST_SUITE_LABEL` environment variable. For example:
  `E2E_TEST_SUITE_LABEL=ec ./mage runE2ETests`
* `klog` level can be controlled via `KLOG_VERBOSITY` environment variable. For
  example: `KLOG_VERBOSITY=9 ./mage runE2ETests` would output `curl` commands
  issued via Kubernetes client from sigs.k8s.io/controller-runtime

```golang
// cmd/e2e_test.go
package common

import (
	// ensure these packages are scanned by ginkgo for e2e tests
	_ "github.com/redhat-appstudio/e2e-tests/tests/common"
	_ "github.com/redhat-appstudio/e2e-tests/tests/has"
)
```

# Troubleshooting e2e-tests issues in openshift-ci
The whole process of investigating issues is defined in [InvestigatingCIFailures](docs/InvestigatingCIFailures.md).

# Reporting issues
Please follow the process in [Reporting and escalating CI Issue](docs/InvestigatingCIFailures.md#reporting-and-escalating-ci-issue) for reporting issues.

# Debugging tests
## In vscode
There is launch configuration in `.vscode/launch.json` called `Launch demo suites`.
Running this configuration, you'll be asked for github token and then e2e-demos suite will run with default configuration.
If you want to run/debug different suite, change `-ginkgo.focus` parameter in `.vscode/launch.json`.

# Cleanup of redhat-appstudio-qe org

Our automated tests running in CI create lot of repositories in our redhat-appstudio-qe github org.

There is a mage target that can cleanup those repositories - `mage local:cleanupGithubOrg`.

For more info & usage, please run `mage -h local:cleanupGithubOrg`.

# PR review process in e2e-tests
* Since RHTAP is leveraging OpenShift CI, [OWNERS](https://github.com/redhat-appstudio/e2e-tests/blob/main/OWNERS) file (in a directory or its parent directory) takes effect in e2e-tests PR review process. Reviewers are selected by [Blunderbuss plugin](https://github.com/kubernetes/test-infra/tree/master/prow/plugins/blunderbuss).
  * [code review process](https://github.com/kubernetes/community/blob/master/contributors/guide/owners.md#the-code-review-process)
  * root [OWNERS](https://github.com/redhat-appstudio/e2e-tests/blob/main/OWNERS) is maintained by the whole RHTAP team, while sub directory OWNERS files are maintained by sub workstream.

* To get a PR merged, the following is needed:
  1. approved label added by selected [approver](https://github.com/kubernetes/community/blob/master/community-membership.md#approver) in relevant OWNERS file through [Blunderbuss plugin](https://github.com/kubernetes/test-infra/tree/master/prow/plugins/blunderbuss).
  2. Tide check passed.
