# Remote Secret tests suite

This suite contains a set of tests that covers Remote Secret scenarios.

Steps to run 'remote-secret-suite':

1) Follow the instructions from the [Readme](../../docs/Installation.md) scripts to install AppStudio in e2e mode
2) Run `make build`
3) Run the e2e suite: `./bin/e2e-appstudio --ginkgo.focus="remote-secret-suite"`

# Scenarios

### Remote Secret interaction with Environments

This scenario ensures that:

- Remote Secret is created in target namespaces of all Environments of an Application and Component (no "appstudio.redhat.com/environment" label is specified)
- Remote Secret is created only in target namespace of the specified Environments of an Application and Component ("appstudio.redhat.com/environment" label is specified)
- Remote Secret is created in all target namespaces of specified multiple Environements ("appstudio.redhat.com/environment" annotation is specified)
- Remote Secret is deleted when Environment is deleted
- Remote Secret target namespace is added to Status.Targets 
- Remote Secret target namespace is deleted from Status.Targets when Environment is deleted

To check the above cases, three different Environments are created, using the same test cluster as BYOC target for simplicity.
Three Remote Secrets with different labels and annotations (based on the required case) are then created, all the checks for each case is performed (secret existence or removal in target namespace, Status.Target updated).

### Test all the options of the authz of remote secret target deployment
