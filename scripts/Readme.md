# Start AppStudio in e2e mode and run the e2e tests

1. Access To Openshift Cluster

2. Login to the cluster as `admin`

   ```bash
    oc login -u <user> -p <password> --server=<oc_api_url>
   ```

3. Install Red Hat App Studio in e2e mode

   ```bash
    $ROOT_DIR/scripts/install-appstudio-e2e-mode.sh install
   ```
4. Compile the e2e tests

   ```bash
    make build
   ```
5. Run the e2e tests

   ```bash
    `$ROOT_DIR/bin/e2e-appstudio`
   ```

Where are:

- `install` - Flag to indicate the installation. If the flag will not be present you can `source` the script and use the bash functions.

The following environments are used to launch the Red Hat AppStudio installation in e2e mode and the tests execution:

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `GITHUB_TOKEN` | yes | A github token used to create AppStudio applications in github  | ''  |
| `QUAY_TOKEN` | yes | A quay token to push components images to quay.io. Note the quay token must be in base 64 format `ewogI3dJhdXRocyI6I...` | '' |
| `GITHUB_E2E_ORGANIZATION` | no | GitHub Organization where to create/push Red Hat AppStudio Applications  | `redhat-appstudio-qe`  |
| `QUAY_E2E_ORGANIZATION` | no | Quay organization where to push components containers | `redhat-appstudio-qe` |
| `E2E_APPLICATIONS_NAMESPACE` | no | Name of the namespace used for running HAS E2E tests | `appstudio-e2e-test` |

* NOTE: Make sure that your Github Token have the following rights in the github organization where you will run the e2e tests.
    - `repo`
    - `delete_repo`
