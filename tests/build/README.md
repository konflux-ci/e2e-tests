# Build tests

Contains E2E tests related to [Build components](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build).

Steps to run tests within `build` directory:

1. Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2. Run the tekton chains suite: `./bin/e2e-appstudio --ginkgo.focus="chains-suite"`
3. Run the build-service suite: `./bin/e2e-appstudio --ginkgo.focus="build-service-suite"`
   1. To test the build of multiple components (from multiple Github repositories), export the environment variable `COMPONENT_REPO_URLS` with value that points
      to multiple Github repo URLs, separated by a comma, e.g.: `export COMPONENT_REPO_URLS=https://github.com/redhat-appstudio-qe/devfile-sample-hello-world,https://github.com/devfile-samples/devfile-sample-python-basic`

## Build service suite specs

1. Verify that the creation of AppStudio component with container image source doesn't trigger PipelineRun
2. Verify that the creation of AppStudio component with Git URL source triggers PipelineRun
   1. Verify that the PipelineRun succeeds

## Tekton chains suite specs
TBD
