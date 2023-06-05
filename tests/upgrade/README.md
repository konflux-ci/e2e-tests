# Service Provider Integration tests suite

This suite contains a set of tests that covers SPI scenarios.

Steps to run 'spi-suite':

1) Setup all required variables(GITHUB_TOKEN, MY_GITHUB_ORG, QUAY_E2E_ORGANIZATION, QUAY_TOKEN, DEFAULT_QUAY_ORG, DEFAULT_QUAY_ORG_TOKEN, DOCKER_IO_AUTH, UPGRADE_BRANCH, UPGRADE_FORK)
2) Connect to cluster
3) Run `make build`
3) `mage local:testUpgrade` - it will bootstral a cluster, create workload, upgrade cluster and verify workload

#### Environments

Values can be provided by setting the following environment variables.

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `UPGRADE_BRANCH` | yes | Branch with changes  | ''  |
| `UPGRADE_FORK` | no | Fork with branch to upgrade | 'infra-deployments' |

