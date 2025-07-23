#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import csv
import json
import re
import sys
import collections
import os
import time


# Column indexes in input data
COLUMN_WHEN = 0
COLUMN_CODE = 1
COLUMN_MESSAGE = 2

# Errors patterns we recognize (when newlines were removed)
ERRORS = {
    "Application creation failed because of TLS handshake timeout": r"Application failed creation: Unable to create the Application .*: failed to get API group resources: unable to retrieve the complete list of server APIs: appstudio.redhat.com/v1alpha1: Get .*: net/http: TLS handshake timeout",
    "Application creation timed out waiting for quota evaluation": r"Application failed creation: Unable to create the Application .*: Internal error occurred: resource quota evaluation timed out",
    "Build Pipeline Run was cancelled" : r"Build Pipeline Run failed run: PipelineRun for component .* in namespace .* failed: .* Reason:Cancelled .* Message:PipelineRun .* was cancelled",
    "Component creation timed out waiting for image-controller annotations": r"Component failed creation: Unable to create the Component .* timed out when waiting for image-controller annotations to be updated on component",
    "Couldnt get pipeline via bundles resolver from quay.io due to 429": r"Message:Error retrieving pipeline for pipelinerun .*bundleresolver.* cannot retrieve the oci image: GET https://quay.io/v2/.*unexpected status code 429 Too Many Requests",
    "Couldnt get pipeline via git resolver from gitlab.cee due to 429": r"Message:.*resolver failed to get Pipeline.*error requesting remote resource.*Git.*https://gitlab.cee.redhat.com/.* status code: 429",
    "Couldnt get pipeline via http resolver from gitlab.cee": r"Message:.*resolver failed to get Pipeline.*error requesting remote resource.*Http.*https://gitlab.cee.redhat.com/.* is not found",
    "Couldnt get task via buldles resolver from quay.io due to 429": r"Message:.*Couldn't retrieve Task .*resolver type bundles.*https://quay.io/.* status code 429 Too Many Requests",
    "Couldnt get task via git resolver from gitlab.cee due to 429": r"Message:.*Couldn't retrieve Task .*resolver type git.*https://gitlab.cee.redhat.com/.* status code: 429",
    "Couldnt get task via http resolver from gitlab.cee": r"Message:.*Couldn't retrieve Task .*resolver type http.*error getting.*requested URL .*https://gitlab.cee.redhat.com/.* is not found",
    "Error deleting on-pull-request default PipelineRun": r"Repo-templating workflow component cleanup failed: Error deleting on-pull-request default PipelineRun in namespace .*: Unable to list PipelineRuns for component .* in namespace .*: context deadline exceeded",
    "Failed application creation when calling mapplication.kb.io webhook": r"Application failed creation: Unable to create the Application .*: Internal error occurred: failed calling webhook .*mapplication.kb.io.*: failed to call webhook: Post .*https://application-service-webhook-service.application-service.svc:443/mutate-appstudio-redhat-com-v1alpha1-application.* no endpoints available for service .*application-service-webhook-service",
    "Failed component creation because resource quota evaluation timed out": r"Component failed creation: Unable to create the Component .*: Internal error occurred: resource quota evaluation timed out",
    "Failed component creation when calling mcomponent.kb.io webhook": r"Component failed creation: Unable to create the Component .*: Internal error occurred: failed calling webhook .*mcomponent.kb.io.*: failed to call webhook: Post .*https://application-service-webhook-service.application-service.svc:443/mutate-appstudio-redhat-com-v1alpha1-component.* no endpoints available for service .*application-service-webhook-service.*",
    "Failed creating integration test scenario because cannot set blockOwnerDeletion if an ownerReference refers to a resource you can't set finalizers on": r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .* integrationtestscenarios.appstudio.redhat.com .* is forbidden: cannot set blockOwnerDeletion if an ownerReference refers to a resource you can't set finalizers on",
    "Failed creating integration test scenario because it already exists": r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .* integrationtestscenarios.appstudio.redhat.com .* already exists",
    "Failed Integration test scenario when calling dintegrationtestscenario.kb.io webhook": r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .*: Internal error occurred: failed calling webhook .*dintegrationtestscenario.kb.io.*: failed to call webhook: Post .*https://integration-service-webhook-service.integration-service.svc:443/mutate-appstudio-redhat-com-v1beta2-integrationtestscenario.*: no endpoints available for service .*integration-service-webhook-service",
    "Failed to link pipeline image pull secret to build service account because SA was not found": r"Failed to configure pipeline imagePullSecrets: Unable to add secret .* to service account .*: serviceaccounts .* not found",
    "Failed to merge MR on CEE GitLab due to 405": r"Repo-templating workflow component cleanup failed: Merging [0-9]+ failed: [Pp][Uu][Tt] .*https://gitlab.cee.redhat.com/api/.*/merge_requests/[0-9]+/merge.*message: 405 Method Not Allowed",
    "Failed to merge MR on CEE GitLab due to DNS error": r"Repo-templating workflow component cleanup failed: Merging [0-9]+ failed: [Pp][Uu][Tt] .*https://gitlab.cee.redhat.com/api/.*/merge_requests/[0-9]+/merge.*Temporary failure in name resolution",
    "Failed validating release condition": r"Release .* in namespace .* failed: .*Message:Release validation failed.*",
    "GitLab token used by test expired": r"Repo forking failed: Error deleting project .*: DELETE https://gitlab.cee.redhat.com/.*: 401 .*error: invalid_token.*error_description: Token is expired. You can either do re-authorization or token refresh",
    "Pipeline failed": r"Message:Tasks Completed: [0-9]+ \(Failed: [1-9]+,",
    "Post-test data collection failed": r"Failed to collect pipeline run JSONs",
    "Release failed in progress without error given": r"Release failed: Release .* in namespace .* failed: .Type:Released Status:False .* Reason:Progressing Message:.$",
    "Release failure: PipelineRun not created": r"couldn't find PipelineRun in managed namespace '%s' for a release '%s' in '%s' namespace",
    "Repo forking failed as GitLab CEE says 401 Unauthorized": r"Repo forking failed: Error deleting project .*: DELETE https://gitlab.cee.redhat.com/.*: 401 .*message: 401 Unauthorized.*",
    "Repo forking failed as the target is still being deleted": r"Repo forking failed: Error forking project .* POST https://gitlab.cee.redhat.com.* 409 .*Project namespace name has already been taken, The project is still being deleted",
    "Repo forking failed because gitlab.com returned 503": r"Repo forking failed: Error checking repository .*: GET https://api.github.com/repos/.*: 503 No server is currently available to service your request. Sorry about that. Please try resubmitting your request and contact us if the problem persists.*",
    "Repo forking failed when deleting target repo on gitlab.com (not CEE!) due unathorized": r"Repo forking failed: Error deleting project .* DELETE https://gitlab.com/.* 401 .* Unauthorized",
    "Timeout forking the repo before the actual test": r"Repo forking failed: Error forking project .*: context deadline exceeded",
    "Timeout getting build service account": r"Component build SA failed creation: Component build SA .* not created: context deadline exceeded",
    "Timeout getting pipeline": r"Message:.*resolver failed to get Pipeline.*resolution took longer than global timeout of .*",
    "Timeout getting task via git resolver from gitlab.cee": r"Message:.*Couldn't retrieve Task .*resolver type git.*https://gitlab.cee.redhat.com/.* resolution took longer than global timeout of .*",
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
    "Timeout listing pipeline runs": r"Repo-templating workflow component cleanup failed: Error deleting on-pull-request default PipelineRun in namespace .*: Unable to list PipelineRuns for component .* in namespace .*: context deadline exceeded",
    "Timeout listing pipeline runs": r"Repo-templating workflow component cleanup failed: Error deleting on-push merged PipelineRun in namespace .*: Unable to list PipelineRuns for component .* in namespace .*: context deadline exceeded",
    "Timeout waiting for build pipeline to be created": r"Build Pipeline Run failed creation: context deadline exceeded",
    "Timeout waiting for integration test scenario to validate": r"Integration test scenario failed validation: context deadline exceeded",
    "Timeout waiting for snapshot to be created": r"Snapshot failed creation: context deadline exceeded",
    "Timeout waiting for test pipeline to create": r"Test Pipeline Run failed creation: context deadline exceeded",
    "Timeout waiting for test pipeline to finish": r"Test Pipeline Run failed run: context deadline exceeded",
}

FAILED_PLR_ERRORS = {
    "SKIP": r"Skipping step because a previous step failed",   # This is a special "wildcard" error, let's keep it on top and do not change "SKIP" reason as it is used in the code
    "Bad Gateway when pulling container image": r"Error: initializing source .* reading manifest .* in .* received unexpected HTTP status: 502 Bad Gateway ",
    "buildah build failed creating build container: registry.access.redhat.com returned 403": r"Error: creating build container: internal error: unable to copy from source docker://registry.access.redhat.com/.*: copying system image from manifest list: determining manifest MIME type for docker://registry.access.redhat.com/.*: reading manifest .* in registry.access.redhat.com/.*: StatusCode: 403",
    "Can not find chroot_scan.tar.gz file": r"tar: .*/chroot_scan.tar.gz: Cannot open: No such file or directory",
    "Can not find Dockerfile": r"Cannot find Dockerfile Dockerfile",
    "DNF failed to download repodata from Koji": r"ERROR Command returned error: Failed to download metadata (baseurl: \"https://kojipkgs.fedoraproject.org/repos/[^ ]*\") for repository \"build\": Usable URL not found",
    "Error allocating host as provision TR already exists": r"Error allocating host: taskruns.tekton.dev \".*provision\" already exists",
    "Error allocating host because of insufficient free addresses in subnet": r"Error allocating host: failed to launch EC2 instance for .* operation error EC2: RunInstances, https response error StatusCode: 400, RequestID: .*, api error InsufficientFreeAddressesInSubnet: There are not enough free addresses in subnet .* to satisfy the requested number of instances.",
    "Error allocating host because of provisioning error": r"Error allocating host: failed to provision host",
    "Failed because of quay.io returned 502": r"level=fatal msg=.Error parsing image name .*docker://quay.io/.* Requesting bearer token: invalid status code from registry 502 .Bad Gateway.",
    "Failed because registry.access.redhat.com returned 503 when reading manifest": r"source-build:ERROR:command execution failure, status: 1, stderr: time=.* level=fatal msg=.Error parsing image name .* reading manifest .* in registry.access.redhat.com/.* received unexpected HTTP status: 503 Service Unavailable",
    "Gateway Time-out when pulling container image": r"Error: copying system image from manifest list: parsing image configuration: fetching blob: received unexpected HTTP status: 504 Gateway Time-out",
    "Introspection failed because of incomplete .docker/config.json": r".* level=fatal msg=\"Error parsing image name .*: getting username and password: reading JSON file .*/tekton/home/.docker/config.json.*: unmarshaling JSON at .*: unexpected end of JSON input\"",
    "RPM build failed: bool cannot be defined via typedef": r"error: .bool. cannot be defined via .typedef..*error: Bad exit status from /var/tmp/rpm-tmp..* ..build.",
}


def message_to_reason(reasons_and_errors: dict, msg: str) -> str | None:
    """
    Classifies an error message using regular expressions and returns the error name.

    Args:
      msg: The input error message string.

    Returns:
      The name of the error if a pattern matches, otherwise string "UNKNOWN".
    """
    msg = msg.replace("\n", " ")  # Remove newlines
    for error_name, pattern in reasons_and_errors.items():
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
        except json.decoder.JSONDecodeError:
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


def find_first_failed_build_plr(data_dir):
    """ This function is intended for jobs where we only run one concurrent
    builds, so no more than one can failed: our load test probes.

    This is executed when test hits "Pipeline failed" error and this is
    first step to identify task that failed so we can identify error in
    the pod log.

    It goes through given data directory (probably "collected-data/") and
    loads all files named "collected-pipelinerun-*" and checks that PLR is
    a "build" PLR and it is failed one.
    """

    for currentpath, folders, files in os.walk(data_dir):
        for datafile in files:
            if not datafile.startswith("collected-pipelinerun-"):
                continue

            datafile = os.path.join(currentpath, datafile)
            data = load(datafile)

            # Skip PLRs that are not "build" PLRs
            try:
                if data["metadata"]["labels"]["pipelines.appstudio.openshift.io/type"] != "build":
                    continue
            except KeyError:
                continue

            # Skip PLRs that did not failed
            try:
                succeeded = True
                for c in data["status"]["conditions"]:
                    if c["type"] == "Succeeded":
                        if c["status"] == "False":
                            succeeded = False
                            break
                if succeeded:
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

def investigate_failed_plr(dump_dir):
    try:
        reasons = []

        plr = find_first_failed_build_plr(dump_dir)
        if plr == None:
            return ["SORRY PLR not found"]

        plr_ns = plr["metadata"]["namespace"]

        for tr_name in find_trs(plr):
            for pod_name, cont_name in find_failed_containers(dump_dir, plr_ns, tr_name):
                log_lines = load_container_log(dump_dir, plr_ns, pod_name, cont_name)
                reason = message_to_reason(FAILED_PLR_ERRORS, log_lines)

                if reason == "SKIP":
                    continue

                reasons.append(reason)

        reasons = list(set(reasons))   # get unique reasons only
        reasons.sort()   # sort reasons
        return reasons
    except FileNotFoundError as e:
        print(f"Failed to locate required files: {e}")
        return ["SORRY, missing data"]
    except Exception as e:
        return ["SORRY " + str(e)]

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
                    reasons2 = investigate_failed_plr(dump_dir)
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
        "error_reasons_simple": "; ".join([f"{v}x {k}" for k, v in error_by_reason.items()]),
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
    sys.exit(main())
