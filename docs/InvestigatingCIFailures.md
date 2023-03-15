# Stonesoup CI Failure Investigation Process

## Investigating CI Infra issues
1. Review job results
(You should be able to identify from the prow job log, if the failure is related to OpenShift CI or tests)
    1. Cluster is provisioned and tests have been run?
        - No
            1. Review log
                - Could be related to OpenShift CI itself: no cluster from cluster pool, job failed to provision cluster, quay.io outage..
                - OpenShift CI failures are usually on channels: **#forum-testplatform**, **#4-dev-triag**, **#announce-testplatform**
            2. Restart the run
                - (in case of OpenShift CI outage - it will not help, but in case of a high workload on OpenShift CI it could help - typically in case no clusters in cluster pool)
        - Yes, you can see the test results
2. Review test results:
    1. Some of the tests failed
        - Review tests logs
            1. Review, if these failures can be related to your PR
            2. Review issues marked with label **ci-fail**
                - You can get these issues with [Jira filter](https://issues.redhat.com/issues/?filter=12405699)
                - The failure could be a known and already reported issue
            3. You can look at the stacktrace and source code to determine, which component/part of the test failed
                - Investigate OpenShift CI Cluster logs
3. Investigate OpenShift CI Cluster logs
    - Every Prow job executed by the CI system generates an artifacts directory containing information about that execution and its results.
    1. Open a link to prow job from your PR -> **Open Artifacts link**
    2. Review logs from folders:
        - **redhat-appstudio_e2e-tests/redhat-appstudio-e2e/**               
            - Store xunit files related to appstudio e2e-tests.
        - **/artifacts/appstudio-e2e-tests/redhat-appstudio-gather/artifacts**
            - Contains information about pipelineruns, pipelines, operators, configuration, Stonesoup Kube APIs informations, components, application, environment..
        - **/artifacts/appstudio-e2e-tests/redhat-appstudio-hypershift-gather/artifacts/**
           - Contains information about PVC, roles, bindings, configmaps…
           - Contains also folder pods. This folder contains logs from all pods and running services.
               - For example there is log from application-service named like: application-service_application-service-application-service-controller-manager
           - This artifacts are present only with hypershift installer.
        - **redhat-appstudio_e2e-tests/gather-extra/**
           - Stores all cluster pods logs, events, configmaps etc. 
           - This artifacts are present only when we dont use hypershift.
        - More details on all artifacts can be found in [OpenShift CI documentation](https://docs.ci.openshift.org/docs/how-tos/artifacts/ )

## Reporting and escalating CI Issue
1. Create JIRA issue
Please report the issue in the STONEBUGS JIRA project with label **ci-fail** and **quality** in case you don't know the correct component/service. In case you know which component is responsible for the failure, use components project and also use the label **ci-fail**.
The QE team will get a notification, when a new issue is created with this label.
Please include:
    - **Link to the prow job**
    - **Failure message**
    - **Relevant logs**
    - (+ could be helpful to also include Slack thread conversation link in the ticket)
2. Post this issue in **#forum-stonesoup-qe**(ping **@ic-appstudio-qe**) channel and relevant component channel.
    - You can also raise this issue on **#wg-developer-stonesoup** channel and your lead can raise this issue on SoS call(and PM call and architects call, if this is necessary).

