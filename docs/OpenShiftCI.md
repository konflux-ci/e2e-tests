# Intro

This document overviews the workflow for onboarding new public Red Hat App Studio repositories to the Openshift CI/Prow. Prow is the k8s-native upstream CI system, source code hosted in the kubernetes/test-infra repository. Prow interacts with GitHub to provide the automation UX that developers use on their pull requests, as well as orchestrating test workloads for those pull requests.

## Steps to create an openshift-ci jobs

Openshift-ci is already enabled for all repositories in [Red Hat Appstudio organization](https://github.com/redhat-appstudio). For adding new jobs you need to create job configuration in [openshift/release](https://github.com/openshift/release) repo as described in following steps.

### Bootstrapping Configuration for a new Repository

From the root of the [openshift/release](https://github.com/openshift/release) repository, run the following target and use the interactive tool to bootstrap a configuration for your repository:

``` bash
    make new-repo
```

This should fully configure your repository, so the changes that it produces are ready to be submitted in a pull request. The resulting YAML file called $org-$repo-$branch.yaml will be found in the ci-operator/config/$org/$repo directory.

### Define tests container build root

The tests in openshift-ci runs in a container. Some tests needs to install some additional tools in the tests container like kubectl, kustomize etc. In that case we need to create a Dockerfile to install those tools. Example of openshift-ci container can be found in [redhat-appstudio/infra-deployments]( https://github.com/redhat-appstudio/infra-deployments/blob/main/.ci/openshift-ci/Dockerfile).

After the creation of the Dockerfile in our repo, in the release/ci-operator/config/$org/$repo we need to add to our config the new dockerfile path to make aware openshift-ci from where to build the container. Example of infra-deployments:

```yaml
build_root:
  project_image:
    dockerfile_path: .ci/openshift-ci/Dockerfile
```

### Build repository containers in openshift-ci

To test the latest changes in the PR in openshift-ci we need to build a container image for the tests. For that purpose we can use the `images` feature. Example:

```yaml
images:
- dockerfile_path: Dockerfile # the path of the repository dockerfile
  to: redhat-appstudio-has-image # tag of the image
```

To use the image in our tests, in the test steps in config job we need to define the image dependency as environment:

```yaml
      dependencies:
      - env: HAS_CONTROLLER_IMAGE
        name: redhat-appstudio-has-image
```

You can take a look at the [application-service](https://github.com/openshift/release/blob/master/ci-operator/config/redhat-appstudio/application-service/redhat-appstudio-application-service-main.yaml) configuration.

### Cluster Pools

App Studio jobs are running using cluster pool for all jobs. Please find more information about how cluster pools work in openshift-ci in the following [docs](https://docs.ci.openshift.org/docs/architecture/ci-operator/#testing-with-a-cluster-from-a-cluster-pool).

### Define your tests

Next step after configuring cluster and dependencies is to define the tests execution. Please find more information in [multi-stage](https://docs.ci.openshift.org/docs/architecture/step-registry/) tests section.

### Sanitize config and create the jobs

Last steps in openshift-ci configuration is to sanitize your configuration and generating the jobs. For that we need to execute the following:

```bash
    make ci-operator-config # sanitize configuration
    make jobs # create jobs from the configuration you created in ci-operator/config/$org/$repo after executing make new-repo
```

Now feel free to create your Pull request in openshift/release repo!

NOTE: For more openshift-ci docs please click [here](https://docs.ci.openshift.org/docs/)
