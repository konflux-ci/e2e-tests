# Build tests

Contains E2E tests related to [Build components](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build).

Steps to run tests within `build` directory:

1. Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2. Run the tekton chains suite: `./bin/e2e-appstudio --ginkgo.focus="chains-suite"`
3. Run the build-service suite: `./bin/e2e-appstudio --ginkgo.focus="build-service-suite"`

## Build service suite specs

1. Verify that the creation of AppStudio component with container image source doesn't trigger PipelineRun
2. Verify that the creation of AppStudio component with Git URL source triggers PipelineRun
   1. Verify that the PipelineRun succeeds

## Tekton chains suite specs
TBD

