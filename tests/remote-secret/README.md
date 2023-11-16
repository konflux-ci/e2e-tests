# Remote Secret tests suite

This suite contains a set of tests that covers Remote Secret scenarios. For detailed information regarding the remote-secret functionalities, please read the [remote-secret documentation](https://github.com/redhat-appstudio/remote-secret/blob/main/docs/USER.md).

Steps to run 'remotesecret-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run `make build`
3) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="remotesecret-suite"`

# Scenarios

## Remote Secret interaction with Environments ([SVPI-632](https://issues.redhat.com/browse/SVPI-632), [SVPI-633](https://issues.redhat.com/browse/SVPI-633), [SVPI-653](https://issues.redhat.com/browse/SVPI-653), [SVPI-654](https://issues.redhat.com/browse/SVPI-654),[environments.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/remote-secret/environments.go))

This scenario ensures that:

- Remote Secret is created in target namespaces of all Environments of an Application and Component (no "appstudio.redhat.com/environment" label is specified)
- Remote Secret is created only in target namespace of the specified Environments of an Application and Component ("appstudio.redhat.com/environment" label is specified)
- Remote Secret is created in all target namespaces of specified multiple Environments ("appstudio.redhat.com/environment" annotation is specified)
- Remote Secret is deleted when Environment is deleted
- Remote Secret target namespace is added to Status.Targets 
- Remote Secret target namespace is deleted from Status.Targets when Environment is deleted

To check the above cases, three different Environments are created, using the same test cluster as BYOC target for simplicity.
Three Remote Secrets with different labels and annotations (based on the required case) are then created and all the checks for each case are performed (secret existence or removal in target namespace, Status.Target updated).

## Kubeconfig auth ([SVPI-558](https://issues.redhat.com/browse/SVPI-558), [kubeconfig-auth.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/remote-secret/kubeconfig-auth.go))
[Kubeconfig auth](https://github.com/redhat-appstudio/remote-secret/blob/main/docs/USER.md#another-cluster) is an authorization option that allows deploying the secrets defined by the remote secret to a completely different cluster using a referenced kubeconfig configuration.

To avoid the complexity of using two clusters on the test, the test points to the running cluster kubeconfig, since the goal is to test the kubeconfig auth.


## Service account auth ([SVPI-558](https://issues.redhat.com/browse/SVPI-558), [service-account-auth.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/remote-secret/service-account-auth.go))
[Service account auth](https://github.com/redhat-appstudio/remote-secret/blob/main/docs/USER.md#associating-the-secret-with-a-service-account-in-the-targets) is an authorization option for deploying secrets to different target namespace than where the remote secret lives.

## Target current namespace where the remote secret lives ([SVPI-558](https://issues.redhat.com/browse/SVPI-558), [target-current-namespace.go](https://github.com/redhat-appstudio/e2e-tests/blob/main/tests/remote-secret/target-current-namespace.go))
[Current namespace auth](https://github.com/redhat-appstudio/remote-secret/blob/main/docs/USER.md#same-namespace) is the simplest way of authorization since targeting current namespace where the remote secret lives is always allowed.