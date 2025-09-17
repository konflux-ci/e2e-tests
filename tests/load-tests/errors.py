#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import collections
import csv
import json
import os
import re
import sys
import yaml


# Column indexes in input data
COLUMN_WHEN = 0
COLUMN_CODE = 1
COLUMN_MESSAGE = 2

# Errors patterns we recognize (when newlines were removed)
ERRORS = {
    ("Application creation failed because it already exists", r"Application failed creation: Unable to create the Application .*: applications.appstudio.redhat.com .* already exists"),
    ("Application creation failed because of TLS handshake timeout", r"Application failed creation: Unable to create the Application .*: failed to get API group resources: unable to retrieve the complete list of server APIs: appstudio.redhat.com/v1alpha1: Get .*: net/http: TLS handshake timeout"),
    ("Application creation failed because resourcequota object has been modified", r"Application failed creation: Unable to create the Application [^ ]+: Operation cannot be fulfilled on resourcequotas [^ ]+: the object has been modified; please apply your changes to the latest version and try again"),
    ("Application creation timed out waiting for quota evaluation", r"Application failed creation: Unable to create the Application .*: Internal error occurred: resource quota evaluation timed out"),
    ("Build Pipeline Run was cancelled", r"Build Pipeline Run failed run: PipelineRun for component .* in namespace .* failed: .* Reason:Cancelled.*Message:PipelineRun .* was cancelled"),
    ("Component creation failed because resourcequota object has been modified", r"Component failed creation: Unable to create the Component [^ ]+: Operation cannot be fulfilled on resourcequotas [^ ]+: the object has been modified; please apply your changes to the latest version and try again"),
    ("Component creation timed out waiting for image-controller annotations", r"Component failed creation: Unable to create the Component .* timed out when waiting for image-controller annotations to be updated on component"),   # obsolete
    ("Component creation timed out waiting for image repository to be ready", r"Component failed creation: Unable to create the Component .* timed out waiting for image repository to be ready for component .* in namespace .*: context deadline exceeded"),
    ("Couldnt get pipeline via bundles resolver from quay.io due to 429", r"Message:Error retrieving pipeline for pipelinerun .*bundleresolver.* cannot retrieve the oci image: GET https://quay.io/v2/.*unexpected status code 429 Too Many Requests"),
    ("Couldnt get pipeline via git resolver from gitlab.cee due to 429", r"Message:.*resolver failed to get Pipeline.*error requesting remote resource.*Git.*https://gitlab.cee.redhat.com/.* status code: 429"),
    ("Couldnt get pipeline via http resolver from gitlab.cee", r"Message:.*resolver failed to get Pipeline.*error requesting remote resource.*Http.*https://gitlab.cee.redhat.com/.* is not found"),
    ("Couldnt get task via buldles resolver from quay.io due to 404", r"Message:.*Couldn't retrieve Task .*resolver type bundles.*https://quay.io/.* status code 404 Not Found"),
    ("Couldnt get task via buldles resolver from quay.io due to 429", r"Message:.*Couldn't retrieve Task .*resolver type bundles.*https://quay.io/.* status code 429 Too Many Requests"),
    ("Couldnt get task via buldles resolver from quay.io due to manifest unknown", r"Build Pipeline Run failed run: PipelineRun for component [^ ]+ in namespace [^ ]+ failed: .* Reason:CouldntGetTask Message:Pipeline [^ ]+ can't be Run; it contains Tasks that don't exist: Couldn't retrieve Task .resolver type bundles.*name = .* cannot retrieve the oci image: GET https://quay.io/[^ ]+: MANIFEST_UNKNOWN: manifest unknown"),
    ("Couldnt get task via bundles resolver because control characters in yaml", r"Build Pipeline Run failed run: PipelineRun for component [^ ]+ in namespace [^ ]+ failed: .* Reason:CouldntGetTask Message:Pipeline [^ ]+ can't be Run; it contains Tasks that don't exist: Couldn't retrieve Task .resolver type bundles.*name = .* invalid runtime object: yaml: control characters are not allowed"),
    ("Couldnt get task via bundles resolver from quay.io due to digest mismatch", r"Build Pipeline Run failed run: PipelineRun for component [^ ]+ in namespace [^ ]+ failed: .* Reason:CouldntGetTask Message:Pipeline [^ ]+ can't be Run; it contains Tasks that don't exist: Couldn't retrieve Task .resolver type bundles.*name = .*: error requesting remote resource: error getting \"bundleresolver\" .*: cannot retrieve the oci image: manifest digest: [^ ]+ does not match requested digest: [^ ]+ for .quay.io/"),
    ("Couldnt get task via bundles resolver from quay.io due to unexpected end of JSON input", r"Build Pipeline Run failed run: PipelineRun for component .* in namespace .* failed: .* Reason:CouldntGetTask Message:Pipeline .* can't be Run; it contains Tasks that don't exist: Couldn't retrieve Task .resolver type bundles.*name = .*: error requesting remote resource: error getting \"bundleresolver\" .*: cannot retrieve the oci image: unexpected end of JSON input"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Message:.*Couldn't retrieve Task .*resolver type git.*https://gitlab.cee.redhat.com/.* status code: 429"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Reason:CouldntGetTask Message:.*Couldn't retrieve Task .resolver type git.*https://gitlab.cee.redhat.com/.* error requesting remote resource: error getting .Git. .*: error resolving repository: git clone error: Cloning into .* error: RPC failed; HTTP 429 curl 22 The requested URL returned error: 429 fatal: expected 'packfile': exit status 128"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Reason:CouldntGetTask Message:.*Couldn't retrieve Task .resolver type git.*https://gitlab.cee.redhat.com/.* error requesting remote resource: error getting .Git. .*: error resolving repository: git clone error: Cloning into .* remote: Retry later fatal: unable to access 'https://gitlab.cee.redhat.com/.*': The requested URL returned error: 429: exit status 128"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Reason:CouldntGetTask Message:.*Couldn't retrieve Task .resolver type git.*https://gitlab.cee.redhat.com/.* error requesting remote resource: error getting .Git. .*: git fetch error: error: RPC failed; HTTP 429 curl 22 The requested URL returned error: 429"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Reason:CouldntGetTask Message:.*Couldn't retrieve Task .resolver type git.*https://gitlab.cee.redhat.com/.* error requesting remote resource: error getting .Git. .*: git clone error: Cloning into .* error: RPC failed; HTTP 429 curl 22 The requested URL returned error: 429"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Reason:CouldntGetTask Message:.*Couldn't retrieve Task .resolver type git.*https://gitlab.cee.redhat.com/.* error requesting remote resource: error getting .Git. .*: git fetch error: error: RPC failed; HTTP 429 curl 22 The requested URL returned error: 429 fatal: expected 'acknowledgments': exit status 128"),
    ("Couldnt get task via git resolver from gitlab.cee due to 429", r"Reason:CouldntGetTask Message:.*Couldn't retrieve Task .resolver type git.*https://gitlab.cee.redhat.com/.* error requesting remote resource: error getting .Git. .*: git fetch error: remote: Retry later fatal: unable to access .*: The requested URL returned error: 429: exit status 128"),
    ("Couldnt get task via http resolver from gitlab.cee", r"Message:.*Couldn't retrieve Task .*resolver type http.*error getting.*requested URL .*https://gitlab.cee.redhat.com/.* is not found"),
    ("Error deleting on-pull-request default PipelineRun", r"Repo-templating workflow component cleanup failed: Error deleting on-pull-request default PipelineRun in namespace .*: Unable to list PipelineRuns for component .* in namespace .*: context deadline exceeded"),
    ("Error updating .tekton file in gitlab.cee.redhat.com", r"Repo-templating workflow component cleanup failed: Error templating PaC files: Failed to update file .tekton/[^ ]+ in repo .*: Failed to update/create file: PUT https://gitlab.cee.redhat.com/api/v4/projects/[^ ]+/repository/files/.tekton/.*: 400 .message: A file with this name doesn't exist."),
    ("Failed application creation when calling mapplication.kb.io webhook", r"Application failed creation: Unable to create the Application .*: Internal error occurred: failed calling webhook .*mapplication.kb.io.*: failed to call webhook: Post .*https://application-service-webhook-service.application-service.svc:443/mutate-appstudio-redhat-com-v1alpha1-application.* no endpoints available for service .*application-service-webhook-service"),
    ("Failed component creation because it already exists", r"Component failed creation: Unable to create the Component [^ ]+: components.appstudio.redhat.com \"[^ ]+\" already exists"),
    ("Failed component creation because resource quota evaluation timed out", r"Component failed creation: Unable to create the Component .*: Internal error occurred: resource quota evaluation timed out"),
    ("Failed component creation when calling mcomponent.kb.io webhook", r"Component failed creation: Unable to create the Component .*: Internal error occurred: failed calling webhook .*mcomponent.kb.io.*: failed to call webhook: Post .*https://application-service-webhook-service.application-service.svc:443/mutate-appstudio-redhat-com-v1alpha1-component.* no endpoints available for service .*application-service-webhook-service.*"),
    ("Failed creating integration test scenario because admission webhook dintegrationtestscenario.kb.io could not find application", r"Integration test scenario failed creation: Unable to create the Integration Test Scenario [^ ]+: admission webhook \"dintegrationtestscenario.kb.io\" denied the request: could not find application '[^ ]+' in namespace '[^ ]+'"),
    ("Failed creating integration test scenario because cannot set blockOwnerDeletion if an ownerReference refers to a resource you can't set finalizers on", r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .* integrationtestscenarios.appstudio.redhat.com .* is forbidden: cannot set blockOwnerDeletion if an ownerReference refers to a resource you can't set finalizers on"),
    ("Failed creating integration test scenario because it already exists", r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .* integrationtestscenarios.appstudio.redhat.com .* already exists"),
    ("Failed creating integration test scenario because of timeout", r"Integration test scenario failed creation: Unable to create the Integration Test Scenario [^ ]+ in namespace jhutar-tenant: context deadline exceeded"),
    ("Failed getting PaC pull number because PaC public route does not exist", r"Component failed validation: Unable to get PaC pull number for component .* in namespace .*: PaC component .* in namespace .* failed on PR annotation: Incorrect state: .*\"error-message\":\"52: Pipelines as Code public route does not exist\""),
    ("Failed Integration test scenario when calling dintegrationtestscenario.kb.io webhook", r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .*: Internal error occurred: failed calling webhook .*dintegrationtestscenario.kb.io.*: failed to call webhook: Post .*https://integration-service-webhook-service.integration-service.svc:443/mutate-appstudio-redhat-com-v1beta2-integrationtestscenario.*: no endpoints available for service .*integration-service-webhook-service"),
    ("Failed to add imagePullSecrets to build SA", r"Failed to configure pipeline imagePullSecrets: Unable to add secret .* to service account build-pipeline-.*: context deadline exceeded"),
    ("Failed to git fetch from gitlab.cee due to connectivity issues", r"Error running git .fetch.*: exit status 128.*remote: Retry later.*fatal: unable to access 'https://gitlab.cee.redhat.com/[^ ]+': The requested URL returned error: 429.*Error fetching git repository: failed to fetch [^ ]+: exit status 128"),
    ("Failed to link pipeline image pull secret to build service account because SA was not found", r"Failed to configure pipeline imagePullSecrets: Unable to add secret .* to service account .*: serviceaccounts .* not found"),
    ("Failed to merge MR on CEE GitLab due to 405", r"Repo-templating workflow component cleanup failed: Merging [0-9]+ failed: [Pp][Uu][Tt] .*https://gitlab.cee.redhat.com/api/.*/merge_requests/[0-9]+/merge.*message: 405 Method Not Allowed"),
    ("Failed to merge MR on CEE GitLab due to DNS error", r"Repo-templating workflow component cleanup failed: Merging [0-9]+ failed: [Pp][Uu][Tt] .*https://gitlab.cee.redhat.com/api/.*/merge_requests/[0-9]+/merge.*Temporary failure in name resolution"),
    ("Failed validating release condition", r"Release .* in namespace .* failed: .*Message:Release validation failed.*"),
    ("GitLab token used by test expired", r"Repo forking failed: Error deleting project .*: DELETE https://gitlab.cee.redhat.com/.*: 401 .*error: invalid_token.*error_description: Token is expired. You can either do re-authorization or token refresh"),
    ("Pipeline failed", r"Build Pipeline Run failed run:.*Message:Tasks Completed: [0-9]+ \(Failed: [1-9]+,"),
    ("Post-test data collection failed", r"Failed to collect application JSONs"),
    ("Post-test data collection failed", r"Failed to collect pipeline run JSONs"),
    ("Post-test data collection failed", r"Failed to collect release related JSONs"),
    ("Release failed in progress without error given", r"Release failed: Release .* in namespace .* failed: .Type:Released Status:False .* Reason:Progressing Message:.$"),
    ("Release failure: PipelineRun not created", r"couldn't find PipelineRun in managed namespace '%s' for a release '%s' in '%s' namespace"),
    ("Release Pipeline failed", r"Release pipeline run failed:.*Message:Tasks Completed: [0-9]+ \(Failed: [1-9]+,"),
    ("Repo forking failed as GitLab CEE says 401 Unauthorized", r"Repo forking failed: Error deleting project .*: DELETE https://gitlab.cee.redhat.com/.*: 401 .*message: 401 Unauthorized.*"),
    ("Repo forking failed as GitLab CEE says 405 Method Not Allowed", r"Repo forking failed: Error deleting project [^ ]+: DELETE https://gitlab.cee.redhat.com/[^ ]+: 405 .message: Non GET methods are not allowed for moved projects."),
    ("Repo forking failed as GitLab CEE says 500 Internal Server Error", r"Repo forking failed: Error deleting project .*: GET https://gitlab.cee.redhat.com/.*: 500 failed to parse unknown error format.*500: We're sorry, something went wrong on our end"),
    ("Repo forking failed as the target is still being deleted", r"Repo forking failed: Error forking project .* POST https://gitlab.cee.redhat.com.* 409 .*Project namespace name has already been taken, The project is still being deleted"),
    ("Repo forking failed as we got TLS handshake timeout talking to GitLab CEE", r"Repo forking failed: Error deleting project .*: Delete \"https://gitlab.cee.redhat.com/api/v4/projects/.*\": net/http: TLS handshake timeout"),
    ("Repo forking failed as we got TLS handshake timeout talking to GitLab CEE", r"Repo forking failed: Error getting project [^ ]+: Get \"https://gitlab.cee.redhat.com/api/v4/projects/.*\": net/http: TLS handshake timeout"),
    ("Repo forking failed because gitlab.com returned 503", r"Repo forking failed: Error checking repository .*: GET https://api.github.com/repos/.*: 503 No server is currently available to service your request. Sorry about that. Please try resubmitting your request and contact us if the problem persists.*"),
    ("Repo forking failed because import failed", r"Repo forking failed: Error waiting for project [^ ]+ .ID: [0-9]+. fork to complete: Forking of project [^ ]+ .ID: [0-9]+. failed with import status: failed"),
    ("Repo forking failed when deleting target repo on github.com because 504", r"Repo forking failed: Error deleting repository .*: DELETE https://api.github.com/repos/.*: 504 We couldn't respond to your request in time. Sorry about that. Please try resubmitting your request and contact us if the problem persists."),
    ("Repo forking failed when deleting target repo on gitlab.com (not CEE!) due unathorized", r"Repo forking failed: Error deleting project .* DELETE https://gitlab.com/.* 401 .* Unauthorized"),
    ("Repo templating failed when updating file on github.com because 500", r"Repo-templating workflow component cleanup failed: Error templating PaC files: Failed to update file .tekton/[^ ]+.yaml in repo [^ ]+ revision main: error when updating a file on github: PUT https://api.github.com/repos/[^ ]+: 500"),
    ("Repo templating failed when updating file on github.com because 502", r"Repo-templating workflow component cleanup failed: Error templating PaC files: Failed to update file .tekton/[^ ]+.yaml in repo [^ ]+ revision main: error when updating a file on github: PUT https://api.github.com/repos/[^ ]+: 502 Server Error"),
    ("Repo templating failed when updating file on github.com because 504", r"Repo-templating workflow component cleanup failed: Error templating PaC files: Failed to update file .tekton/[^ ]+.yaml in repo [^ ]+ revision main: error when updating a file on github: PUT https://api.github.com/repos/[^ ]+: 504 We couldn't respond to your request in time. Sorry about that. Please try resubmitting your request and contact us if the problem persists."),
    ("Test Pipeline failed", r"Test Pipeline Run failed run:.*Message:Tasks Completed: [0-9]+ \(Failed: [1-9]+,"),
    ("Timeout creating application calling mapplication.kb.io webhook", r"Application failed creation: Unable to create the Application [^ ]+: Internal error occurred: failed calling webhook .mapplication.kb.io.: failed to call webhook: Post .https://application-service-webhook-service.application-service.svc:443/mutate-appstudio-redhat-com-v1alpha1-application[^ ]+.: context deadline exceeded"),
    ("Timeout forking the repo before the actual test", r"Repo forking failed: context deadline exceeded"),
    ("Timeout forking the repo before the actual test", r"Repo forking failed: Error forking project .*: context deadline exceeded"),
    ("Timeout forking the repo before the actual test", r"Repo forking failed: Error waiting for project [^ ]+ .ID: [0-9]+. fork to complete: context deadline exceeded"),
    ("Timeout getting build service account", r"Component build SA not present: Component build SA .* not present: context deadline exceeded"),
    ("Timeout getting PaC pull number when validating component", r"Component failed validation: Unable to get PaC pull number for component .* in namespace .*: context deadline exceeded"),
    ("Timeout getting pipeline", r"Message:.*resolver failed to get Pipeline.*resolution took longer than global timeout of .*"),
    ("Timeout getting task via git resolver from gitlab.cee", r"Message:.*Couldn't retrieve Task .*resolver type git.*https://gitlab.cee.redhat.com/.* resolution took longer than global timeout of .*"),
    # Last time I seen this we discussed it here:
    #
    #   https://redhat-internal.slack.com/archives/C04PZ7H0VA8/p1751530663606749
    #
    # And it manifested itself by check on initial PR failing with:
    #
    #   the namespace of the provided object does not match the namespace sent on the request
    #
    # And folks noticed this in the PaC controller logs:
    #
    #   There was an error starting the PipelineRun test-rhtap-1-app-ryliu-comp-0-on-pull-request-, creating pipelinerun
    #   test-rhtap-1-app-ryliu-comp-0-on-pull-request- in namespace test-rhtap-1-tenant has failed. Tekton Controller has
    #   reported this error: ```Internal error occurred: failed calling webhook "vpipelineruns.konflux-ci.dev": failed
    #   to call webhook: Post "https://etcd-shield.etcd-shield.svc:443/validate-tekton-dev-v1-pipelinerun?timeout=10s":
    #   context deadline exceeded```
    ("Timeout listing pipeline runs", r"Repo-templating workflow component cleanup failed: Error deleting on-pull-request default PipelineRun in namespace .*: Unable to list PipelineRuns for component .* in namespace .*: context deadline exceeded"),
    ("Timeout listing pipeline runs", r"Repo-templating workflow component cleanup failed: Error deleting on-push merged PipelineRun in namespace .*: Unable to list PipelineRuns for component .* in namespace .*: context deadline exceeded"),
    ("Timeout onboarding component", r"Component failed onboarding: context deadline exceeded"),
    ("Timeout waiting for build pipeline to be created", r"Build Pipeline Run failed creation: context deadline exceeded"),
    ("Timeout waiting for integration test scenario to validate", r"Integration test scenario failed validation: context deadline exceeded"),
    ("Timeout waiting for release pipeline to be created", r"Release pipeline run failed creation: context deadline exceeded"),
    ("Timeout waiting for snapshot to be created", r"Snapshot failed creation: context deadline exceeded"),
    ("Timeout waiting for test pipeline to create", r"Test Pipeline Run failed creation: context deadline exceeded"),
    ("Timeout waiting for test pipeline to finish", r"Test Pipeline Run failed run: context deadline exceeded"),
    ("Unable to connect to server", r"Error: Unable to connect to server"),
}

FAILED_PLR_ERRORS = {
    ("SKIP", r"Skipping step because a previous step failed"),   # This is a special "wildcard" error, let's keep it on top and do not change "SKIP" reason as it is used in the code
    ("Bad Gateway when pulling container image from quay.io", r"Error: initializing source docker://quay.io/[^ ]+: reading manifest [^ ]+ in quay.io/[^ ]+: received unexpected HTTP status: 502 Bad Gateway "),
    ("buildah build failed to pull container from registry.access.redhat.com because digest mismatch", r"buildah build.*FROM registry.access.redhat.com/[^ ]+ Trying to pull registry.access.redhat.com/[^ ]+ Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+: copying system image from manifest list: parsing image configuration: Download config.json digest [^ ]+ does not match expected [^ ]+"),
    ("buildah build failed to pull container from registry.access.redhat.com because of 403", r"Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/.*: copying system image from manifest list: determining manifest MIME type for docker://registry.access.redhat.com/.*: reading manifest .* in registry.access.redhat.com/.*: StatusCode: 403"),
    ("buildah build failed to pull container from registry.access.redhat.com because of 500 Internal Server Error", r"buildah build.*FROM registry.access.redhat.com/[^ ]+ Trying to pull registry.access.redhat.com/[^ ]+ Getting image source signatures Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+ copying system image from manifest list: reading signatures: reading signature from https://access.redhat.com/[^ ]+ received unexpected HTTP status: 500 Internal Server Error"),
    ("Can not find chroot_scan.tar.gz file", r"tar: .*/chroot_scan.tar.gz: Cannot open: No such file or directory"),
    ("Can not find Dockerfile", r"Cannot find Dockerfile Dockerfile"),
    ("DNF failed to download repodata from Download Devel because could not resolve host", r"Errors during downloading metadata for repository '[^ ]+':   - Curl error .6.: Couldn't resolve host name for http://download.devel.redhat.com/brewroot/repos/[^ ]+ .Could not resolve host: download\.devel\.redhat\.com."),
    ("DNF failed to download repodata from Download Devel because timeout", r"dnf.exceptions.RepoError: Failed to download metadata for repo 'build': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried .* CRITICAL Error: Failed to download metadata for repo 'build': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried [^ ]+/mock/.*Failed to connect to download-[0-9]+.beak-[0-9]+.prod.iad2.dc.redhat.com"),
    ("DNF failed to download repodata from Download Devel because timeout", r"dnf.exceptions.RepoError: Failed to download metadata for repo 'build': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried .* CRITICAL Error: Failed to download metadata for repo 'build': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried .*/mock/.*Failed to connect to download.devel.redhat.com"),
    ("DNF failed to download repodata from Koji", r"ERROR Command returned error: Failed to download metadata (baseurl: \"https://kojipkgs.fedoraproject.org/repos/[^ ]*\") for repository \"build\": Usable URL not found"),
    ("Enterprise contract results failed validation", r"^false $"),
    ("Error allocating host as provision TR already exists", r"Error allocating host: taskruns.tekton.dev \".*provision\" already exists"),
    ("Error allocating host because of insufficient free addresses in subnet", r"Error allocating host: failed to launch EC2 instance for .* operation error EC2: RunInstances, https response error StatusCode: 400, RequestID: .*, api error InsufficientFreeAddressesInSubnet: There are not enough free addresses in subnet .* to satisfy the requested number of instances."),
    ("Error allocating host because of provisioning error", r"Error allocating host: failed to provision host"),
    ("Failed because CPU is not x86-64-v4", r"ERROR: CPU is not x86-64-v4, aborting build."),
    ("Failed because of quay.io returned 502", r"level=fatal msg=.Error parsing image name .*docker://quay.io/.* Requesting bearer token: invalid status code from registry 502 .Bad Gateway."),
    ("Failed because registry.access.redhat.com returned 503 when reading manifest", r"source-build:ERROR:command execution failure, status: 1, stderr: time=.* level=fatal msg=.Error parsing image name .* reading manifest .* in registry.access.redhat.com/.* received unexpected HTTP status: 503 Service Unavailable"),
    ("Failed downloading rpms for hermetic builds due to 504 errors", r"mock-hermetic-repo.*urllib3.exceptions.MaxRetryError: HTTPSConnectionPool.*: Max retries exceeded with url: .*.rpm .Caused by ResponseError..too many 504 error responses..."),
    ("Failed downloading rpms for hermetic builds", r"mock-hermetic-repo.*ERROR:__main__:RPM deps downloading failed"),
    ("Failed to connect to MPC VM", r"ssh: connect to host [0-9]+.[0-9]+.[0-9]+.[0-9]+ port 22: Connection timed out"),
    ("Failed to prefetch dependencies due to download timeout", r"ERROR Unsuccessful download: .* ERROR FetchError: exception_name: TimeoutError.*If the issue seems to be on the cachi2 side, please contact the maintainers."),
    ("Failed to provision MPC VM due to resource quota evaluation timed out", r"cat /ssh/error Error allocating host: Internal error occurred: resource quota evaluation timed out"),   # KONFLUX-9798
    ("Failed to pull container from access.redhat.com because of DNS error", r"Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/.*: copying system image from manifest list: reading signatures: Get \"https://access.redhat.com/.*\": dial tcp: lookup access.redhat.com: Temporary failure in name resolution"),
    ("Failed to pull container from quay.io because of DNS error", r"Error: copying system image from manifest list: reading blob .*: Get \"https://cdn[0-9]+.quay.io/.*\": dial tcp: lookup cdn[0-9]+.quay.io: Temporary failure in name resolution"),
    ("Failed to pull container from quay.io due to 404", r"Error response from registry: recognizable error message not found: PUT .https://quay.io/[^ ]+.: response status code 404: Not Found Command exited with non-zero status 1"),
    ("Failed to pull container from registry.access.redhat.com because of 500 Internal Server Error", r"Trying to pull registry.access.redhat.com/[^ ]+ Getting image source signatures Error: copying system image from manifest list: reading signatures: reading signature from https://access.redhat.com/[^ ]+: status 500 .Internal Server Error."),
    ("Failed to pull container from registry.access.redhat.com because of DNS error", r"Error: initializing source docker://registry.access.redhat.com/.* pinging container registry registry.access.redhat.com: Get \"https://registry.access.redhat.com/v2/\": dial tcp: lookup registry.access.redhat.com: Temporary failure in name resolution"),
    ("Failed to pull container from registry.access.redhat.com because of remote tls error", r"Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+ copying system image from manifest list: reading blob [^ ]+: Get .https://cdn[0-9]+.quay.io/[^ ]+ remote error: tls: internal error"),
    ("Failed to pull container from registry.access.redhat.com because of remote tls error", r"Trying to pull registry.access.redhat.com/[^ ]+ Error: copying system image from manifest list: parsing image configuration: Get .https://cdn[0-9]+.quay.io/[^ ]+ remote error: tls: internal error"),
    ("Failed to pull container from registry.access.redhat.com because of unauthorized", r"Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+: initializing source docker://registry.access.redhat.com/[^ ]+: unable to retrieve auth token: invalid username/password: unauthorized: Please login to the Red Hat Registry using your Customer Portal credentials."),
    ("Failed to pull container from registry.access.redhat.com because of unauthorized", r"unable to retrieve auth token: invalid username/password: unauthorized: Please login to the Red Hat Registry using your Customer Portal credentials. .* subprocess.CalledProcessError: Command ...podman....pull.*registry.access.redhat.com/.* returned non-zero exit status 125"),
    ("Failed to pull container from registry.access.redhat.com because of unauthorized", r"unable to retrieve auth token: invalid username/password: unauthorized: Please login to the Red Hat Registry using your Customer Portal credentials. .* subprocess.CalledProcessError: Command ...skopeo....inspect.*docker://registry.access.redhat.com/.* returned non-zero exit status 1"),
    ("Failed to pull container from registry.fedoraproject.org", r"Error: internal error: unable to copy from source docker://registry.fedoraproject.org/[^ ]+: initializing source docker://registry.fedoraproject.org/[^ ]+: pinging container registry registry.fedoraproject.org: Get \"https://registry.fedoraproject.org/v2/\": dial tcp [^ ]+: connect: connection refused"),
    ("Failed to push SBOM to quay.io", r"Uploading SBOM file for [^ ]+ to [^ ]+ with mediaType [^ ]+.  Error: Get .https://quay.io/v2/.: dial tcp .[0-9a-f:]+.:443: connect: network is unreachable [^ ]+: error during command execution: Get .https://quay.io/v2/.: dial tcp .[0-9a-f:]+.:443: connect: network is unreachable"),
    ("Failed to push SBOM to quay.io", r"Uploading SBOM file for [^ ]+ to [^ ]+ with mediaType [^ ]+.  Error: PUT https://quay.io/v2/[^ ]+: unexpected status code 200 OK [^ ]+: error during command execution: PUT https://quay.io/v2/[^ ]+: unexpected status code 200 OK"),
    ("Failed to push to quai.io due to 404", r"Error response from registry: recognizable error message not found: PUT \"https://quay.io/[^ ]+\": response status code 404"),
    ("Failed to ssh to remote MPC VM", r"[^ ]+@[0-9.]+: Permission denied .publickey,gssapi-keyex,gssapi-with-mic..\s*$"),   # KONFLUX-9742
    ("Gateway Time-out when pulling container image from quay.io", r"Error: initializing source docker://quay.io/[^ ]+: reading manifest [^ ]+ in quay.io/[^ ]+: received unexpected HTTP status: 504 Gateway Time-out"),
    ("Gateway Time-out when pulling container image", r"Error: copying system image from manifest list: parsing image configuration: fetching blob: received unexpected HTTP status: 504 Gateway Time-out"),
    ("Getting repo tags from quay.io failed because of 502 Bad Gateway", r"Error determining repository tags: pinging container registry quay.io: received unexpected HTTP status: 502 Bad Gateway"),
    ("Introspection failed because of incomplete .docker/config.json", r".* level=fatal msg=\"Error parsing image name .*: getting username and password: reading JSON file .*/tekton/home/.docker/config.json.*: unmarshaling JSON at .*: unexpected end of JSON input\""),
    ("Invalid reference when processing SBOM", r"SBOM .* error during command execution: could not parse reference: quay.io/[^ ]+"),
    ("No podman installed on a MPC VM", r"remote_cmd podman unshare setfacl .* \+ ssh -o StrictHostKeyChecking=no [^ ]+ podman unshare setfacl .* bash: line 1: podman: command not found"),   # KONFLUX-9944
    ("Release failed because unauthorized when pulling policy", r"Error: pulling policy: GET .https://quay.io/v2/konflux-ci/konflux-vanguard/data-acceptable-bundles/blobs/sha256:[0-9a-z]+.: response status code 401: Unauthorized"),
    ("Release failed because unauthorized when pushing artifact", r"Prepared artifact from /var/workdir/release .* Token not found for quay.io/konflux-ci/release-service-trusted-artifacts Uploading [0-9a-z]+ sourceDataArtifact Error response from registry: unauthorized: access to the requested resource is not authorized: map.. Command exited with non-zero status 1"),
    ("RPM build failed: bool cannot be defined via typedef", r"error: .bool. cannot be defined via .typedef..*error: Bad exit status from /var/tmp/rpm-tmp..* ..build."),
    ("Script gather-rpms.py failed because of too many values to unpack", r"Handling archdir [^ ]+ Traceback.*File \"/usr/bin/gather-rpms.py\".*nvr, btime, size, sigmd5, _ = .*ValueError: too many values to unpack"),
    ("Script mock-hermetic-repo failed because pull from registry.access.redhat.com failed", r"Error: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+: determining manifest MIME type for docker://registry.access.redhat.com/[^ ]+: Manifest does not match provided manifest digest [^ ]+.*/usr/bin/mock-hermetic-repo.*subprocess.CalledProcessError.*Command ...podman....pull.* returned non-zero exit status 125"),
    ("Script mock-hermetic-repo failed because pull from registry.access.redhat.com failed", r"mock-hermetic-repo.*Error: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+: determining manifest MIME type for docker://registry.access.redhat.com/[^ ]+: Manifest does not match provided manifest digest.*subprocess.CalledProcessError.*Command ...podman....pull.* returned non-zero exit status 125"),
    ("Script mock-hermetic-repo failed because pull from registry.access.redhat.com failed", r"/usr/bin/mock-hermetic-repo.*Error: internal error: unable to copy from source docker://registry.access.redhat.com/[^ ]+: initializing source docker://registry.access.redhat.com/[^ ]+: unable to retrieve auth token: invalid username/password: unauthorized.*subprocess.CalledProcessError.*Command '.'podman', 'pull', '--arch', '[^ ]+', 'registry.access.redhat.com/[^ ]+'.' returned non-zero exit status 125"),
    ("Script rpm_verifier failed to access image layer from quay.io because 502 Bad Gateway", r"rpm_verifier --image-url quay.io/.* Image: quay.io/.* error: unable to access the source layer sha256:[0-9a-z]+: received unexpected HTTP status: 502 Bad Gateway"),
    ("Script rpm_verifier failed to pull image from quay.io because 502 Bad Gateway", r"rpm_verifier.*error: unable to read image quay.io/[^ ]+: Get .https://quay.io/[^ ]+.: received unexpected HTTP status: 502 Bad Gateway"),
}

FAILED_TR_ERRORS = {
    ("Missing expected fields in TaskRun", r"Missing expected fields in TaskRun"),   # This is special error, meaning everithing failed basically
    ("SKIP", r"\"message\": \"All Steps have completed executing\""),   # Another special error to avoid printing 'Unknown error:' message
    ("SKIP", r"\"message\": \".* exited with code 1.*\""),   # Another special error to avoid printing 'Unknown error:' message
    ("SKIP", r"\"message\": \".* exited with code 255.*\""),   # Another special error to avoid printing 'Unknown error:' message
    ("Back-off pulling task run image from quay.io", r"the step .* in TaskRun .* failed to pull the image .*. The pod errored with the message: \\\"Back-off pulling image \\\"quay.io/.*"),
    ("Back-off pulling task run image from registry.access.redhat.com", r"the step .* in TaskRun .* failed to pull the image .*. The pod errored with the message: \\\"Back-off pulling image \\\"registry.access.redhat.com/.*"),
    ("Back-off pulling task run image from registry.redhat.io", r"the step .* in TaskRun .* failed to pull the image .*. The pod errored with the message: \\\"Back-off pulling image \\\"registry.redhat.io/.*"),
    ("Build failed for unspecified reasons", r"build failed for unspecified reasons."),
    ("Failed to create task run pod because ISE on webhook proxy.operator.tekton.dev", r"failed to create task run pod .*: Internal error occurred: failed calling webhook \\\"proxy.operator.tekton.dev\\\": failed to call webhook: Post \\\"https://tekton-operator-proxy-webhook.openshift-pipelines.svc:443/defaulting.timeout=10s\\\": context deadline exceeded. Maybe missing or invalid Task .*"),
    ("Not enough nodes to schedule pod", r".message.: .pod status ..PodScheduled..:..False..; message: ..[0-9/]+ nodes are available: .*: [0-9]+ Preemption is not helpful for scheduling."),
    ("Pod creation failed because resource quota evaluation timed out", r".message.: .failed to create task run pod [^ ]+: Internal error occurred: resource quota evaluation timed out. Maybe missing or invalid Task [^ ]+., .reason.: .PodCreationFailed."),
    ("Pod creation failed with reason error", r"\"message\": \".* exited with code 2: Error\""),
    ("Pod stuck in incorrect status", r".message.: .pod status ..PodReadyToStartContainers..:..False..; message: ....., .reason.: .Pending., .status.: .Unknown."),
}


def message_to_reason(reasons_and_errors: set, msg: str) -> str:
    """
    Classifies an error message using regular expressions and returns the error name.

    Args:
      msg: The input error message string.

    Returns:
      The name of the error if a pattern matches, otherwise string "UNKNOWN".
    """
    msg = msg.replace("\n", " ")  # Remove newlines
    for error_name, pattern in reasons_and_errors:
        if re.search(pattern, msg):
            return error_name
    print(f"Unknown error: {msg}")
    return "UNKNOWN"


def add_reason(error_messages, error_by_code, error_by_reason, message, reason="", code=0):
    if reason == "":
        reason = message
    print("Added", message, reason, code)
    error_messages.append(message)
    error_by_code[code] += 1
    error_by_reason[reason] += 1


def load(datafile):
    if datafile.endswith(".yaml") or datafile.endswith(".yml"):
        try:
            with open(datafile, "r") as fd:
                data = yaml.safe_load(fd)
        except yaml.scanner.ScannerError:
            raise Exception(f"File {datafile} is malfrmed YAML, skipping it")
    elif datafile.endswith(".json"):
        try:
            with open(datafile, "r") as fp:
                data = json.load(fp)
        except json.decoder.JSONDecodeError:
            raise Exception(f"File {datafile} is malfrmed JSON, skipping it")
    else:
        raise Exception("Unknown data file format")

    return data


def find_all_failed_plrs(data_dir):
    for currentpath, folders, files in os.walk(data_dir):
        for datafile in files:
            if not datafile.startswith("collected-pipelinerun-"):
                continue

            datafile = os.path.join(currentpath, datafile)
            data = load(datafile)

            # Skip PLRs that did not failed
            try:
                succeeded = True
                for c in data["status"]["conditions"]:
                    if c["type"] == "Succeeded":
                        if c["status"] == "False":   # possibly switch this to `!= "True"` but that might be too big change for normal runs
                            succeeded = False
                            break
                if succeeded:
                    continue
            except KeyError:
                continue

            yield data


def find_first_failed_build_plr(data_dir, plr_type):
    """ This function is intended for jobs where we only run one concurrent
    builds, so no more than one can failed: our load test probes.

    This is executed when test hits "Pipeline failed" error and this is
    first step to identify task that failed so we can identify error in
    the pod log.

    It goes through given data directory (probably "collected-data/") and
    loads all files named "collected-pipelinerun-*" and checks that PLR is
    a "build" PLR and it is failed one.
    """

    for data in find_all_failed_plrs(data_dir):
        data = load(datafile)

        if plr_type == "build":
            plr_type_label = "build"
        elif plr_type == "release":
            plr_type_label = "managed"
        else:
            raise Exception("Unknown PLR type")

        # Skip PLRs that do not have expected type
        try:
            if data["metadata"]["labels"]["pipelines.appstudio.openshift.io/type"] != plr_type_label:
                continue
        except KeyError:
            continue

        return data


def find_trs(plr):
    try:
        for tr in plr["status"]["childReferences"]:
            yield tr["name"]
    except KeyError:
        return


def check_failed_taskrun(data_dir, ns, tr_name):
    datafile = os.path.join(data_dir, ns, "1", "collected-taskrun-" + tr_name + ".json")
    data = load(datafile)

    try:
        pod_name = data["status"]["podName"]
        for condition in data["status"]["conditions"]:
            if condition["type"] == "Succeeded":
                break
    except KeyError:
        return False, "Missing expected fields in TaskRun"
    else:
        if pod_name == "":
            return False, json.dumps(condition, sort_keys=True)
        else:
            return True, json.dumps(condition, sort_keys=True)


def find_failed_containers(data_dir, ns, tr_name):
    datafile = os.path.join(data_dir, ns, "1", "collected-taskrun-" + tr_name + ".json")
    data = load(datafile)

    try:
        pod_name = data["status"]["podName"]
        for sr in data["status"]["steps"]:
            if sr["terminated"]["exitCode"] != 0:
                yield (pod_name, sr["container"])
    except KeyError:
        return


def load_container_log(data_dir, ns, pod_name, cont_name):
    datafile = os.path.join(data_dir, ns, "1", "pod-" + pod_name + "-" + cont_name + ".log")
    print(f"Checking errors in {datafile}")
    with open(datafile, "r") as fd:
        return fd.read()


def investigate_failed_plr(dump_dir, plr_type="build"):
    reasons = []

    try:
        plr = find_first_failed_build_plr(dump_dir, plr_type)
        if plr is None:
            return ["SORRY PLR not found"]

        plr_ns = plr["metadata"]["namespace"]

        for tr_name in find_trs(plr):
            tr_ok, tr_message = check_failed_taskrun(dump_dir, plr_ns, tr_name)

            if tr_ok:
                try:
                    for pod_name, cont_name in find_failed_containers(dump_dir, plr_ns, tr_name):
                        log_lines = load_container_log(dump_dir, plr_ns, pod_name, cont_name)
                        reason = message_to_reason(FAILED_PLR_ERRORS, log_lines)

                        if reason == "SKIP":
                            continue

                        reasons.append(reason)
                except FileNotFoundError as e:
                    print(f"Failed to locate required files: {e}")

            reason = message_to_reason(FAILED_TR_ERRORS, tr_message)
            if reason != "SKIP":
                reasons.append(reason)
    except Exception as e:
        return ["SORRY " + str(e)]

    reasons = list(set(reasons))   # get unique reasons only
    reasons.sort()   # sort reasons
    return reasons


def main():
    input_file = sys.argv[1]
    timings_file = sys.argv[2]
    output_file = sys.argv[3]
    dump_dir = sys.argv[4]

    error_messages = []  # list of error messages
    error_by_code = collections.defaultdict(
        lambda: 0
    )  # key: numeric error code, value: number of such errors
    error_by_reason = collections.defaultdict(
        lambda: 0
    )  # key: textual error reason, value: number of such errors

    try:
        with open(input_file, "r") as fp:
            csvreader = csv.reader(fp)
            for row in csvreader:
                if row == []:
                    continue

                code = row[COLUMN_CODE]
                message = row[COLUMN_MESSAGE]

                reason = message_to_reason(ERRORS, message)

                if reason == "Pipeline failed":
                    reasons2 = investigate_failed_plr(dump_dir, "build")
                    reason = reason + ": " + ", ".join(reasons2)

                if reason == "Release Pipeline failed":
                    reasons2 = investigate_failed_plr(dump_dir, "release")
                    reason = reason + ": " + ", ".join(reasons2)

                add_reason(error_messages, error_by_code, error_by_reason, message, reason, code)
    except FileNotFoundError:
        print("No errors file found, good :-D")

    timings = {}
    try:
        with open(timings_file, "r") as fp:
            timings = json.load(fp)
    except FileNotFoundError:
        print("No timings file found, strange :-/")
        error_messages.append("No timings file found")
        add_reason(error_messages, error_by_code, error_by_reason, "No timings file found")

    try:
        if timings["KPI"]["mean"] == -1:
            if len(error_messages) == 0:
                add_reason(error_messages, error_by_code, error_by_reason, "No test run finished")
    except KeyError:
        print("No KPI metrics in timings data, strange :-(")
        add_reason(error_messages, error_by_code, error_by_reason, "No KPI metrics in timings data")

    data = {
        "error_by_code": error_by_code,
        "error_by_reason": error_by_reason,
        "error_reasons_simple": "; ".join([f"{v}x {k}" for k, v in error_by_reason.items() if k != "Post-test data collection failed"]),
        "error_messages": error_messages,
    }

    print(f"Errors detected: {len(error_messages)}")
    print("Errors by reason:")
    for k, v in error_by_reason.items():
        print(f"   {v}x {k}")

    with open(output_file, "w") as fp:
        json.dump(data, fp, indent=4)
    print(f"Data dumped to {output_file}")


def investigate_all_failed_plr(dump_dir):
    reasons = []

    for plr in find_all_failed_plrs(dump_dir):
        plr_ns = plr["metadata"]["namespace"]

        for tr_name in find_trs(plr):
            tr_ok, tr_message = check_failed_taskrun(dump_dir, plr_ns, tr_name)

            if tr_ok:
                try:
                    for pod_name, cont_name in find_failed_containers(dump_dir, plr_ns, tr_name):
                        log_lines = load_container_log(dump_dir, plr_ns, pod_name, cont_name)
                        reason = message_to_reason(FAILED_PLR_ERRORS, log_lines)

                        if reason == "SKIP":
                            continue

                        reasons.append(reason)
                except FileNotFoundError as e:
                    print(f"Failed to locate required files: {e}")

            reason = message_to_reason(FAILED_TR_ERRORS, tr_message)
            if reason != "SKIP":
                reasons.append(reason)

    return sorted(reasons)


def main_custom():
    dump_dir = sys.argv[1]
    output_file = os.path.join(dump_dir, "errors-output.json")

    error_messages = []  # list of error messages
    error_by_code = collections.defaultdict(
        lambda: 0
    )  # key: numeric error code, value: number of such errors
    error_by_reason = collections.defaultdict(
        lambda: 0
    )  # key: textual error reason, value: number of such errors

    reasons = investigate_all_failed_plr(dump_dir)
    for r in reasons:
        add_reason(error_messages, error_by_code, error_by_reason, r)

    data = {
        "error_by_code": error_by_code,
        "error_by_reason": error_by_reason,
        "error_reasons_simple": "; ".join([f"{v}x {k}" for k, v in error_by_reason.items() if k != "Post-test data collection failed"]),
        "error_messages": error_messages,
    }

    print(f"Errors detected: {len(error_messages)}")
    print("Errors by reason:")
    for k, v in error_by_reason.items():
        print(f"   {v}x {k}")

    with open(output_file, "w") as fp:
        json.dump(data, fp, indent=4)
    print(f"Data dumped to {output_file}")


if __name__ == "__main__":
    if len(sys.argv) == 2:
        # When examining just custom collected-data directory
        sys.exit(main_custom())
    else:
        sys.exit(main())
