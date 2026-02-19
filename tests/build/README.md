# Build tests

Contains E2E tests related to [Build Service](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/build-service) and [image-controller](https://github.com/redhat-appstudio/infra-deployments/tree/main/components/image-controller) components .

Steps to run tests within `build` directory:

1. Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2. Run the build-service suite: `./bin/e2e-appstudio --ginkgo.focus="build-service-suite"`
   1. To test the build of multiple components (from multiple Github repositories), export the environment variable `COMPONENT_REPO_URLS` with value that points
      to multiple Github repo URLs, separated by a comma, e.g.: `export COMPONENT_REPO_URLS=https://github.com/redhat-appstudio-qe/devfile-sample-hello-world,https://github.com/devfile-samples/devfile-sample-python-basic`

## Running build tests locally on a kind cluster

### Installing konflux locally

__Note:__ For now, it is only supported on x86_64 Linux platforms

1. First make sure [the dependencies](https://github.com/konflux-ci/konflux-ci?tab=readme-ov-file#installing-software-dependencies) are installed on your local machine

2. Bootstrap the [konflux cluster](https://github.com/konflux-ci/konflux-ci?tab=readme-ov-file#bootstrapping-the-cluster) following upstream documentation

### Applying build-service local changes

Build the image with your local changes and push to your own `quay.io` repo, like `quay.io/susdas/build-controller:bugfix`

Replace the custom image and tag [here](https://github.com/konflux-ci/konflux-ci/blob/28e9b85f8943aaed03e9ba74899086f502d8543f/konflux-ci/build-service/core/kustomization.yaml#L14-L15) before installing konflux.

If you have already installed konflux locally, then to apply `build-service` changes, run below command
```
kubectl apply -k konflux-ci/build-service
```

__Note:__ If you have any [config](https://github.com/konflux-ci/build-service/tree/main/config) related changes, then push the changes to your fork and replace the url [here](https://github.com/konflux-ci/konflux-ci/blob/28e9b85f8943aaed03e9ba74899086f502d8543f/konflux-ci/build-service/core/kustomization.yaml#L4)

### Prepare the environment needed for running the tests

1. Get the smee channel id created for you while deploying konflux using command `grep value dependencies/smee/smee-channel-id.yaml`
```
export SMEE_CHANNEL=<smee_channel>
```

2. Follow the instructions on step 2 [here](https://github.com/konflux-ci/konflux-ci?tab=readme-ov-file#enable-pipelines-triggering-via-webhooks) for setting up a new github app (one time activity)
```
export APP_ID=<app_id>
export APP_PRIVATE_KEY=<private_key>
export APP_WEBHOOK_SECRET=<webhook_secret>
```

3. Install the github app configured in the previous step 2 on all the sample github repositories to be used during tests

4. Setup quay repository to be used in the tests, same to the values we set [here](https://github.com/konflux-ci/e2e-tests/blob/c03c22276fdadaf3e9bf63d56829c4f7c22ad385/default.env#L11-L17), needed for configuring image-controller
```
export QUAY_ORG=<quay_org>
export QUAY_TOKEN=<quay_token>
```

5. Next, run the command `./test/e2e/prepare-e2e.sh` to create the kubernetes secrets needed for running e2e-tests

6. Set the environment variables that are used in the tests
```
# this is for running tests in upstream konflux cluster
export TEST_ENVIRONMENT=upstream

# Same values to be used from the step 4 above
export DEFAULT_QUAY_ORG=<quay_org>
export DEFAULT_QUAY_ORG_TOKEN=<quay_org_token>

# Same as MY_GITHUB_ORG used in https://github.com/konflux-ci/e2e-tests/blob/main/default.env
export MY_GITHUB_ORG=<github_org>

# Same as GITHUB_TOKEN used in https://github.com/konflux-ci/e2e-tests/blob/main/default.env
export GITHUB_TOKEN=<github_token>

# Same as QUAY_TOKEN used in https://github.com/konflux-ci/e2e-tests/blob/main/default.env, note that this value is different from the quay token used in previous step 4, need to set again after running `prepare-e2e.sh` script
export QUAY_TOKEN=<quay_token>
```

7. For running GitLab related tests, replace `PAC_WEBHOOK_URL` value to public smee server url [here](https://github.com/konflux-ci/konflux-ci/blob/b67f2e3412b00686a0ba66e8c2a697fb1b977e10/konflux-ci/build-service/core/build-service-env-patch.yaml#L11), as shown below
```diff
           env:
             - name: PAC_WEBHOOK_URL
-              value: http://pipelines-as-code-controller.pipelines-as-code.svc.cluster.local:8180
+              value: https://smee.io/eZKOv78ryCsNbUFLFYWd7C5t9K6gZNuzhrxsLDe5

```
After changing the value, run `kubectl apply -k konflux-ci/build-service` for applying changes to the cluster.

8. Set the below two environment variables needed for the gitlab tests
```
# Same as GITLAB_QE_ORG used in https://github.com/konflux-ci/e2e-tests/blob/main/default.env
export GITLAB_QE_ORG=<gitlab_org>

# Same as GITLAB_BOT_TOKEN used in https://github.com/konflux-ci/e2e-tests/blob/main/default.env
export GITLAB_BOT_TOKEN=<gitlab_bot_token>
```

9. For running Codeberg/Forgejo tests, set the following environment variables:
```
export CODEBERG_QE_ORG=konflux-qe
export CODEBERG_BOT_TOKEN=<codeberg_bot_token>
```

The initial test repository is available at: https://codeberg.org/konflux-qe/devfile-sample-hello-world

### Running the tests

From the e2e-tests local clone directory, use `ginkgo` command to run the `build-service` labelled tests
```
ginkgo -v --label-filter="build-service" ./cmd
```

For running a single test by name, we may use ginkgo’s focus argument to run it, for example:
```
ginkgo -v --focus="should not trigger a PipelineRun" ./cmd
```