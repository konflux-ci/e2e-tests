# Service Provider Integration tests suite

This suite contains a set of tests that covers SPI scenarios.

Steps to run 'spi-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run `make build`
3) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="spi-suite"`

#### Environments

Valid Quay username and token are required to be able to run the suite: SPI will use them to create valid configurations to test private container image deployment. 
Values can be provided by setting the following environment variables.

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `QUAY_OAUTH_USER` | yes | A quay.io username used to push/build containers  | ''  |
| `QUAY_OAUTH_TOKEN` | yes | A quay.io token used to push/build containers. Note: the token and username must be a robot account with access to your repository | '' |
| `SPI_GITHUB_CLIENT_ID` | yes | Github Oauth application Client ID  | ''  |
| `SPI_GITHUB_CLIENT_SECRET` | yes | Github Oauth application Client secret | ''  |
| `CYPRESS_GH_USER` | yes | Github Oauth application Client ID  | ''  |
| `CYPRESS_GH_PASSWORD` | yes | Github Oauth application Client ID  | ''  |
| `CYPRESS_GH_2FA_CODE` | yes | Github Oauth application Client ID  | ''  |
| `OAUTH_REDIRECT_PROXY_URL` | yes | Github Oauth application Client ID  | ''  |

### OAUTH flow tests

The oauth authentication flow involves some steps that can only be executed in the browser; e2e-tests do this by using [cypress](https://github.com/cypress-io/cypress) framework to simulate the user steps. The specs that Cypress will run are located in [this repo](https://github.com/redhat-appstudio-qe/cypress-browser-oauth-flow). 
After creating SPI needed resources, e2e completes the steps a user would perform in a browser by creating a short-lived pod with the Cypress image, injecting the cypress specs by cloning the above repo and running them.
To complete the authetication, a test user is needed: the tests expect username, password and the 2FA setup key to be provided. 

SPI service will need to be configured to use those providers. E2E-tests do this automatically in the setup phase, by expecting the clientid and secret for each supported provider to be supplied via environment variables. 

If we are running tests in CI, we need to handle the dynamic url the cluster is assigned with. To do that, we use a redirect proxy that allows us to have a static oauth url in the providers configuration and, at the same time, will redirect the callback call to the spi component in our cluster. OAUTH_REDIRECT_PROXY_URL env should containt the url of such proxy.

If not running in CI, SPI expects that the callback url in the provider configuration is set to the default one: homepage URL + /oauth/callback

Tested provider are: Github.

