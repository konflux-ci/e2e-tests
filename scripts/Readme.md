# Start AppStudio in e2e mode

1. Access To Openshift Cluster

2. Login to the cluster as `admin`

   ```
   oc login -u <user> -p <password> --server=<oc_api_url>
   ```

3. Run the test from your machine

   ```
   $ROOT_DIR/scripts/install-appstudio-e2e-mode.sh install
   ```

Where are:

- `install` - Flag to indicate the installation. If the flag will not be present you can `source` the script and use the bash functions.
