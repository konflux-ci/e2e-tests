# BYOC test

### Prerequisites for running this test against your own cluster
1. Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode.
2. Make sure the cluster you are about to run this test against is public (i.e. hosted on a public cloud provider).
3. Access to an external openshift cluster to use it as RHTAP environment.
4. Have installed vcluster binary. How to install [here](https://www.vcluster.com/docs/getting-started/setup).
5. Have installed helm binary. How to install [here](https://helm.sh/docs/helm/helm_install/).

#### Environments

| Variable | Required | Explanation | Default Value |
|---|---|---|---|
| `BYOC_KUBECONFIG` | yes | A valid path to a openshift kubeconfig file. Note: Your kubeconfig should contain token instead of certificates:https://issues.redhat.com/browse/GITOPSRVCE-554  | ''  |

### Description

This tests try to simulate user workflow from creating an Openshift/Kubernetes envionment in RHTAP to deploy the applications to the given environment.
The tests are creating a virtual cluster in ur RHTAP cluster to test pure kubernetes environments.

#### What is virtual Cluster?
Virtual clusters are Kubernetes clusters that run on top of other Kubernetes clusters. Compared to fully separate "real" clusters, virtual clusters do not have their own node pools or networking.
Instead, they are scheduling workloads inside the underlying cluster while having their own control plane.

More information about vcluster architecture can be found at vcluster [website](https://www.vcluster.com/docs/architecture/basics)

### How to run
To run the byoc tests run the following command in your  with a custom yaml use:
   ```bash
    export E2E_TEST_SUITE_LABEL=byoc &&  make local/test/e2e
   ```
