# Service Provider Integration tests suite

This suite contains a set of tests that covers SPI scenarios.

Steps to run 'spi-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="spi-suite"`

#### Environments

Valid Quay username and token are required to be able to run the suite: SPI will use them to create valid configurations to test private container image deployment. 
Values can be provided by setting the follwing enviroment variables.

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `QUAY_OAUTH_USER` | yes | A quay.io username used to push/build containers  | ''  |
| `QUAY_OAUTH_TOKEN` | yes | A quay.io token used to push/build containers. Note: the token and username must be a robot account with access to your repository | '' |

