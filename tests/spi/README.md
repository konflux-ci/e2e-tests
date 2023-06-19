# Service Provider Integration tests suite

This suite contains a set of tests that covers SPI scenarios.

Steps to run 'spi-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run `make build`
3) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="spi-suite"`

#### Environments

Values can be provided by setting the following environment variables.

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `QUAY_OAUTH_USER` | yes | A quay.io username used to push/build containers  | ''  |
| `QUAY_OAUTH_TOKEN` | yes | A quay.io token used to push/build containers. Note: the token and username must be a robot account with access to your repository | '' |
| `SPI_GITHUB_CLIENT_ID` | yes | Github Oauth application Client ID  | ''  |
| `SPI_GITHUB_CLIENT_SECRET` | yes | Github Oauth application Client secret | ''  |
| `CYPRESS_GH_USER` | yes | Github user used by Cypress to simulate user's in-browser login  | ''  |
| `CYPRESS_GH_PASSWORD` | yes | Github password used by Cypress to simulate user's in-browser login  | ''  |
| `CYPRESS_GH_2FA_CODE` | yes | Github user 2FA code used by Cypress to simulate user's in-browser login | ''  |
| `OAUTH_REDIRECT_PROXY_URL` | yes | Redirect Proxy public url | ''  |

### Provate Repositories

Valid Quay username and token are required to be able to run the suite: SPI will use them to create valid configurations to test private container image deployment. 

### OAuth flow tests

The oauth authentication flow involves some steps that can only be executed in the browser; e2e-tests do this by using [cypress](https://github.com/cypress-io/cypress) framework to simulate the user steps. The specs that Cypress will run are located in [this repository](https://github.com/redhat-appstudio-qe/cypress-browser-oauth-flow). After creating SPI needed resources, e2e completes the steps a user would perform in a browser by creating a short-lived pod with the Cypress image, injecting the cypress specs by cloning the above repo and running them.
To complete the authetication, a test user is needed: the tests expect username, password and the 2FA setup key to be provided. 

SPI service needs to be configured to use the authetication providers. E2E-tests do this automatically in the setup phase, by expecting the clientid and secret of an oauth application for each supported provider to be supplied via environment variables. 

If tests are running in CI, we need to handle the dynamic url the cluster is assigned with every time e2e-tests run. To do that, we use a simple redirect proxy that allows us to: 
- set a static oauth callback url in the providers configuration; 
- redirect the callback calls from the authentication providers to the spi component in our cluster. 
The OAUTH_REDIRECT_PROXY_URL env should containt the url of such proxy and the callback url of you oauth apps should be set with the same url. 
The proxy is stateless, requires no prior registration and the redirect translation will happen automatically using the state data provided by the SPI component itself.
The recirect proxy used is in [this repository](https://github.com/redhat-appstudio-qe/spi-oauth-redirect-proxy).

If not running in CI, SPI expects that the callback url in the provider configuration is set to the default one: homepage URL + /oauth/callback

Tested provider are: Github.

