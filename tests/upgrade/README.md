# Upgrade tests suite

Steps to run upgrade tests:

1) Setup all required variables(GITHUB_TOKEN, MY_GITHUB_ORG, QUAY_E2E_ORGANIZATION, QUAY_TOKEN, DEFAULT_QUAY_ORG, DEFAULT_QUAY_ORG_TOKEN, DOCKER_IO_AUTH, UPGRADE_BRANCH, UPGRADE_FORK_ORGANIZATION)
2) Connect to cluster
3) Run `make build`
3) `mage local:testUpgrade` - it will bootstral a cluster, create workload, upgrade cluster and verify workload

#### Environments

Values can be provided by setting the following environment variables.

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `UPGRADE_BRANCH` | yes | Branch with changes  | ''  |
| `UPGRADE_FORK_ORGANIZATION` | no | Fork with branch to upgrade | 'redhat-appstudio' |

