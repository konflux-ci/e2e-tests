# Installation of AppStudio E2E Mode in OpenShift CI

The following guide will walk through the deployment of AppStudio E2E mode in Openshift CI Pull Requests jobs

## Install e2e binary in openshift-ci and use pairing PRs feature

The e2e tests are executed against almost all AppStudio repositories.

Sometimes when we have changes in AppStudio we are introducing some breaking changes and the e2e will fail. To prevent this the e2e framework installation in openshift-ci support a new feature of pairing the PR opened against an AppStudio repository with the e2e forks based in branch names. Before the e2e framework will be executed in openshift-ci, the logic automatically tries to pair a PR opened in some repo with a branch of the same name that
potentially could exists in the developer's fork of the e2e repository

For example, if a developer with GH account `cooljohn` opens a PR (for application-service repo) from a branch `new-feature`, then the logic checks if there is a branch `new-feature` also in the `cooljohn/e2e-tests` fork and if exists will start to install the e2e framework from those branch

Once the service PR passes CI with its pairing and is merged, the next step is to get the references in infra-deployments updated.
Many services have automation to create PRs in infra-deployments to update their component to the latest version.
However, this recently merged latest version will likely need the still unmerged e2e-tests update.
The solution is to manually create a PR to infra-deployments that bumps the component version with the *same* branch name.
This will allow the infra-deployments CI to run with your branch of e2e-tests, allowing the CI to pass.
Finally, once the infra-deployments CI passes and PR merges, you should be able to rerun your e2e-tests PR and have it pass and merge.

To continue on the previous example, once the application-service PR merges, `cooljohn` should open a pull request to infra-deployments using the branch `new-feature`
to bump the application-service version to the one recently created by the application-service PR merging. This lets the infra-deployments CI run with the required
e2e-tests changes from `cooljohn`'s fork.

Pairing PRs is handled automatically by running this command from a root directory of this repository:

```bash
   make ci/test/e2e
```
