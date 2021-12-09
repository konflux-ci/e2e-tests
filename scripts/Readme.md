# Run E2E tests as a pod in

1. Deploy Red Hat App Studio from [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) repository

2. Access To Openshift Cluster

3. Login to the cluster as `admin`

   ```
   oc login -u <user> -p <password> --server=<oc_api_url>
   ```

4. Run the test from your machine

   ```
   ./run-tests-in-k8s.sh <namespace> <report-dir>
   ```

Where are:

- `namespace` - Namespace where you want to deploy the e2e tests. Optional. by default the tests will run in `appstudio-e2e` dir.
- `report-dir` - Directory where you want to download the results of tests from pods. Optional. By default will be saved in `$ROOT_REPO/tmp`
