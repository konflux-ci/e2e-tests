# Service Provider Integration tests suite

This suite contains a set of tests that covers SPI scenarios. For detailed information regarding the SPI functionalities, please read the [SPI documentation](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/USER.md).


Steps to run 'spi-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run `make build`
3) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="spi-suite"`

#### Environment Variables

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


# Scenarios

## Ensure that a user can't access and use secrets from another workspace ([SVPI-495](https://issues.redhat.com/browse/SVPI-495), [access-control.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/access-control.go))

Access control tests are important to detect/prevent misconfigurations in authentication and authorization mechanisms. Being SPI responsible for obtaining authentication tokens, we need to be sure that a user cannot access and use secrets from workspaces that should not have access. 

Assuming that:
 * User A is the owner of workspace A and has access to workspace C as the maintainer
 * User B is the owner of workspace B
 * User C is the owner of workspace C

This test verifies that:
* user A can access the SPIAccessToken A in workspace A
* user A can not access the SPIAccessToken B in workspace B
* user A can access the SPIAccessToken C in workspace C
* user B can not access the SPIAccessToken A in workspace A
* user B can access the SPIAccessToken B in workspace B
* user B can not access the SPIAccessToken C in workspace C
* user C can not access the SPIAccessToken A in workspace A
* user C can not access the SPIAccessToken B in workspace B
* user C can access the SPIAccessToken C in workspace C
<br>

* user A can read the GitHub repo in workspace A
* user A can not read the GitHub repo in workspace B
* user A can read the GitHub repo in workspace C
* user B can not read the GitHub repo in workspace A
* user B can read the GitHub repo in workspace B
* user B can not read the GitHub repo in workspace C
* user C can not read the GitHub repo in workspace A
* user C can not read the GitHub repo in workspace B
* user C can read the GitHub repo in workspace C
<br>

* user A can read the secret in workspace A
* user A can not read the secret in workspace B
* user A can not read the secret in workspace C (although workspace C is shared with user A, the role given is maintainer, which does not have any permissions for secrets object)
* user B can not read the secret in workspace A
* user B can read the secret in workspace B
* user B can not read the secret in workspace C
* user C can not read the secret in workspace A
* user C can not read the secret in workspace B
* user C can read the secret in workspace C
<br>

* user's A pod deployed in workspace A should be able to construct an API request that reads code in the Github repo for workspace A
* user's A pod deployed in workspace A should not be able to construct an API request that reads code in the Github repo for workspace B
* user's A pod deployed in workspace A should be able to construct an API request that reads code in the Github repo for workspace C
* user's B pod deployed in workspace B should not be able to construct an API request that reads code in the Github repo for workspace A
* user's B pod deployed in workspace B should be able to construct an API request that reads code in the Github repo for workspace B
* user's B pod deployed in workspace B should not be able to construct an API request that reads code in the Github repo for workspace C
* user's C pod deployed in workspace C should not be able to construct an API request that reads code in the Github repo for workspace A
* user's C pod deployed in workspace C should not be able to construct an API request that reads code in the Github repo for workspace B
* user's C pod deployed in workspace C should be able to construct an API request that reads code in the Github repo for workspace C


## Get file content from a private Github repository with Remote Secret ([SVPI-402](https://issues.redhat.com/browse/SVPI-402), [SVPI-621](https://issues.redhat.com/browse/SVPI-621), [get-file-content-rs.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/get-file-content-rs.go))

One of the SPI's uses cases is the [file content request](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/USER.md#retrieving-file-content-from-scm-repository). In order to request the file content of private GitHub repositories, a GitHub token is needed. So, a Remote Secret should be deployed with a GitHub token, before the file content request. To test this use case, this test presents the following flow:

* creates RemoteSecret
* creates upload secret (with the GitHub token)
* checks if remote secret was deployed
* checks targets in RemoteSecret status
* checks if secret was created in target namespace
* creates SPIFileContentRequest
* SPIFileContentRequest should be in Delivered phase and content should be provided

## Get file content from a private Github repository with SPIAccessToken [*marked for deletion*] ([SVPI-402](https://issues.redhat.com/browse/SVPI-402), [get-file-content.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/get-file-content.go))
Another way of getting file content from a private Github repository is through SPIAccessToken (SPIAccessTokenBinding). The SPIAccessTokenBinding should be in the Injected phase, before the file content request. To test this use case, this test presents the following flow:

- creates SPITokenBinding
- uploads token
- creates SPIFileContentRequest
- SPIFileContentRequest should be in Delivered phase and content should be provided


## Check SA creation and linking to the secret requested by SPIAccessTokenBinding [*marked for deletion*] ([SVPI-406](https://issues.redhat.com/browse/SVPI-406), [link-secret-sa.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/link-secret-sa.go))
One of the SPIAccessTokenBinding functionalities is [providing secrets to a service account](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/USER.md#providing-secrets-to-a-service-account). There are three ways of linking secrets to a SA: link a secret to an existing service account, link a secret to an existing service account as image pull secret, and link a secret to a managed service account. So, this test covers them all:

 - Test Scenario 1: link a secret to an existing service account
 - Test Scenario 2: link a secret to an existing service account as image pull secret
 - Test Scenario 3: link a secret to a managed service account


Flow of each test:
 - creates SPITokenBinding with SA associated
 - uploads token
 - checks if SA was linked to the secret


## Github OAuth flow to upload token ([SVPI-395](https://issues.redhat.com/browse/SVPI-395), [oauth.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/oauth.go))

The oauth authentication flow involves some steps that can only be executed in the browser; e2e-tests do this by using [cypress](https://github.com/cypress-io/cypress) framework to simulate the user steps. The specs that Cypress will run are located in [this repository](https://github.com/redhat-appstudio-qe/cypress-browser-oauth-flow). After creating SPI needed resources, e2e completes the steps a user would perform in a browser by creating a short-lived pod with the Cypress image, injecting the cypress specs by cloning the above repo and running them.
To complete the authentication, a test user is needed: the tests expect username, password and the 2FA setup key to be provided. 

SPI service needs to be configured to use the authentication providers. E2E-tests do this automatically in the setup phase, by expecting the clientid and secret of an oauth application for each supported provider to be supplied via environment variables. 

If tests are running in CI, we need to handle the dynamic url the cluster is assigned with every time e2e-tests run. To do that, we use a simple redirect proxy that allows us to: 
- set a static oauth callback url in the provider’s configuration; 
- redirect the callback calls from the authentication providers to the spi component in our cluster. 
The OAUTH_REDIRECT_PROXY_URL env should contain the url of such proxy and the callback url of you oauth apps should be set with the same url. 
The proxy is stateless, requires no prior registration and the redirect translation will happen automatically using the state data provided by the SPI component itself.
The redirect proxy used is in [this repository](https://github.com/redhat-appstudio-qe/spi-oauth-redirect-proxy).

If not running in CI, SPI expects that the callback url in the provider configuration is set to the default one: homepage URL + /oauth/callback


## Check ImagePullSecret usage for the private Quay image and check the secret that can be used with scopeo Tekton task to authorize a copy of one private Quay image to the second Quay image repository [*marked for deletion*] ([SVPI-407](https://issues.redhat.com/browse/SVPI-407), [SVPI-408](https://issues.redhat.com/browse/SVPI-408), [quay-imagepullsecret-usage.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/quay-imagepullsecret-usage.go))

To avoid code repetition, SVPI-408 was integrated with SVPI-407.

Flow of the test:
 - creates SPITokenBinding
 - uploads token
 - creates a Pod from a Private Quay image
 - checks the secret that can be used with scopeo Tekton task to authorize a copy of one private Quay image to the second Quay image repository


## Upload token with k8s secret [*marked for deletion*] ([SVPI-399](https://issues.redhat.com/browse/SVPI-399), [token-upload-k8s.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/token-upload-k8s.go))

One of the ways to upload secrets to SPI is [using Kubernetes Secret](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/USER.md#uploading-access-token-to-spi-using-kubernetes-secret).

Test Scenario 1: Upload token with k8s secret (associate it to existing SPIAccessToken)
Test Scenario 2: Upload token with k8s secret (create new SPIAccessToken automatically)

Flow of Test Scenario 1:
 - creates SPITokenBinding
 - creates secret with access token and associate it to an existing SPIAccessToken
 - SPITokenBinding should be in Injected phase
 - upload secret should be automatically be removed
 - SPIAccessToken exists and is in Read phase

Flow of Test Scenario 2:
 - creates secret with access token and associate it to an existing SPIAccessToken
 - upload secret should be automatically be removed
 - SPIAccessToken exists and is in Read phase


## Token upload rest endpoint [*marked for deletion*] ([SVPI-398](https://issues.redhat.com/browse/SVPI-398), [token-upload-rest-endpoint.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/spi/token-upload-rest-endpoint.go))
Another way of uploading secrets to SPI is using [REST endpoint](https://github.com/redhat-appstudio/service-provider-integration-operator/blob/main/docs/USER.md#storing-username-and-password-credentials-for-any-provider-by-its-url).

 - Test Scenario 1: Token upload rest endpoint [public repository]
 - Test Scenario 2: Token upload rest endpoint [private repository]

Flow of each test:
 - creates SPITokenBinding
 - checks access to GitHub repository before token upload
 - uploads token
 - checks access to GitHub repository after token upload
