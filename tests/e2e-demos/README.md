# E2E DEMOS

The e2e-demos suite contains a set of tests that covers AppStudio demos.

Steps to run 'e2e-demos-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="e2e-demos-suite"`

## Test Generator

The test specs in e2e-demo-suite are generated dynamically using ginkgo specs.

If you want to test your own Component (repository), all you need to do is to update the `TestConfig` variable in [config.go](./config/config.go)


To run the e2e-demos with a custom yaml use: `./bin/e2e-appstudio --ginkgo.focus="e2e-demos-suite"

## Run tests with private component

Red Hat AppStudio E2E framework now supports creating components from private quay.io images and GitHub repositories.

#### Environments

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `QUAY_OAUTH_USER` | yes | A quay.io username used to push/build containers  | ''  |
| `QUAY_OAUTH_TOKEN` | yes | A quay.io token used to push/build containers. Note: the token and username must be a robot account with access to your repository | '' |

#### Setup configuration for private repositories

1. Define in your configuration for the application and the component
Example of a config for GitHub private repository:

```go
var TestConfig = []TestSpec{
    {
        Name:            "nodejs private component test",
        ApplicationName: "nodejs-private-app",
        Components: []ComponentSpec{
            {
                Name:              "nodejs-private-comp",
                Private:           true,
                Language:          "JavaScript",
                GitSourceUrl:      "https://github.com/redhat-appstudio-qe-bot/nodejs-health-check.git",
                HealthEndpoint:    "/live",
            },
        },
    },
}
```

2. Run the e2e tests
