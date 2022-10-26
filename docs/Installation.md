# Installation of AppStudio E2E Mode in OpenShift CI

The following guide will walk through the deployment of AppStudio E2E mode in Openshift CI Pull Requests jobs

## Install e2e binary in openshift-ci and use pairing PRs feature

The e2e tests are executed against almost all AppStudio repositories.

Sometimes when we have changes in AppStudio we are introducing some breaking changes and the e2e will fail. To prevent this the e2e framework installation in openshift-ci support a new feature of pairing the PR opened against an AppStudio repository with the e2e forks based in branch names. Before the e2e framework will be executed in openshift-ci, the logic automatically tries to pair a PR opened in some repo with a branch of the same name that
potentially could exists in the developer's fork of the e2e repository

For example, if a developer with GH account `cooljohn` opens a PR (for application-service repo) from a branch `new-feature`, then the logic checks if there is a branch `new-feature` also in the `cooljohn/e2e-tests` fork and if exists will start to install the e2e framework from those branch

Pairing PRs is handled automatically by running this command from a root directory of this repository:

```bash
   make ci/test/e2e
```

## Install Red Hat App Studio in e2e mode in kcp

The Red Hat App Studio installation scripts support kcp. To install App studio in e2e mode for kcp we need to execute the following script:

```bash
   export OFFLINE_TOKEN=<the_token_goes_here>
   /bin/bash scripts/install-appstudio-kcp.sh -kc kcp-stable-root -kk <path-to-kcp-kubeconfig> -ck <path-to-physical-cluster-kubeconfig> -s
```

This example script will install App Studio in e2e mode into a stable KCP version. The `OFFLINE_TOKEN` is used in case you want to authenticate against Red Hat SSO. To get the the token you need to go to https://cloud.redhat.com/openshift/token. The token looks like `eyJhbGciOiJIUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJh...`
