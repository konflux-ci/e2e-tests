# Release Pipelines Tests

This suite contains e2e-tests for testing release pipelines from repository [release-service-catalog](https://github.com/redhat-appstudio/release-service-catalog/tree/main), those tests run against RHTAP RH-Stage.

### All tests must have the label `release-pipelines` to avoid running them against the dev environment by OpenShift CI
## prerequisites: 
   - Export the following environment variables:
		```
    	- TOOLCHAIN_API_URL_ENV: Offline token used for getting Keycloak token in order to authenticate against stage/prod cluster
		- KEYLOAK_URL_ENV:       Keycloak URL used for authentication against stage/prod cluster
		- OFFLINE_TOKEN_ENV :    Toolchain API URL used for authentication against stage/prod cluster
		```
   -  The tests will run on two dedicated namespaces, so a user not part of them need to request access to the following namespaces:
		```
		- dev-release-team-tenant
		- managed-release-team-tenant
		```

## How to Run tests:

### 1. To run all e2e-tests in suite ensure your changes of the suite tests are saved.
- navigate to `e2e-tests` directoy 
 ```bash
 make build 
 ./bin/e2e-appstudio --ginkgo.junit-report=report.xml  --ginkgo.focus="pipelines"
 ```

### 2. To run specific test we should define the test with specific label.
[Ginkgo](https://onsi.github.io/ginkgo/#why-ginkgo) uses labels filtering, adding a label for a test will enable to run specific tests with this label. 

For example: 
If a test has the label `label-test` to a test in suite `pipelines` then the test should run using the following arguments:

- navigate to `e2e-tests` directoy 
 ```bash
 make build 
 ./bin/e2e-appstudio --ginkgo.junit-report=report.xml  --ginkgo.focus="pipelines" --ginkgo.label-filter="label-test"
 ```



