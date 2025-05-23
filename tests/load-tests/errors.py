#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import csv
import json
import re
import sys
import collections


# Column indexes in input data
COLUMN_WHEN = 0
COLUMN_CODE = 1
COLUMN_MESSAGE = 2

# Errors patterns we recognize (when newlines were removed)
ERRORS = {
    "Component creation timed out waiting for image-controller annotations": r"Component failed creation: Unable to create the Component .* timed out when waiting for image-controller annotations to be updated on component",
    "Couldnt get pipeline via bundles resolver from quay.io due to 429": r"Message:Error retrieving pipeline for pipelinerun .*bundleresolver.* cannot retrieve the oci image: GET https://quay.io/v2/.*unexpected status code 429 Too Many Requests",
    "Couldnt get pipeline via git resolver from gitlab.cee due to 429": r"Message:.*resolver failed to get Pipeline.*error requesting remote resource.*Git.*https://gitlab.cee.redhat.com/.* status code: 429",
    "Couldnt get pipeline via http resolver from gitlab.cee": r"Message:.*resolver failed to get Pipeline.*error requesting remote resource.*Http.*https://gitlab.cee.redhat.com/.* is not found",
    "Couldnt get task via buldles resolver from quay.io due to 429": r"Message:.*Couldn't retrieve Task .*resolver type bundles.*https://quay.io/.* status code 429 Too Many Requests",
    "Couldnt get task via git resolver from gitlab.cee due to 429": r"Message:.*Couldn't retrieve Task .*resolver type git.*https://gitlab.cee.redhat.com/.* status code: 429",
    "Couldnt get task via http resolver from gitlab.cee": r"Message:.*Couldn't retrieve Task .*resolver type http.*error getting.*requested URL .*https://gitlab.cee.redhat.com/.* is not found",
    "Failed application creation when calling mapplication.kb.io webhook": r"Application failed creation: Unable to create the Application .*: Internal error occurred: failed calling webhook .*mapplication.kb.io.*: failed to call webhook: Post .*https://application-service-webhook-service.application-service.svc:443/mutate-appstudio-redhat-com-v1alpha1-application.* no endpoints available for service .*application-service-webhook-service",
    "Failed Integration test scenario when calling dintegrationtestscenario.kb.io webhook": r"Integration test scenario failed creation: Unable to create the Integration Test Scenario .*: Internal error occurred: failed calling webhook .*dintegrationtestscenario.kb.io.*: failed to call webhook: Post .*https://integration-service-webhook-service.integration-service.svc:443/mutate-appstudio-redhat-com-v1beta2-integrationtestscenario.*: no endpoints available for service .*integration-service-webhook-service",
    "Failed to link pipeline image pull secret to build service account": r"Failed to configure pipeline imagePullSecrets: Unable to add secret .* to service account .*: serviceaccounts .* not found",
    "Failed to merge MR on CEE GitLab due to 405": r"Repo-templating workflow component cleanup failed: Merging [0-9]+ failed: [Pp][Uu][Tt] .*https://gitlab.cee.redhat.com/api/.*/merge_requests/[0-9]+/merge.*message: 405 Method Not Allowed",
    "Failed to merge MR on CEE GitLab due to DNS error": r"Repo-templating workflow component cleanup failed: Merging [0-9]+ failed: [Pp][Uu][Tt] .*https://gitlab.cee.redhat.com/api/.*/merge_requests/[0-9]+/merge.*Temporary failure in name resolution",
    "Pipeline failed": r"Message:Tasks Completed: [0-9]+ \(Failed: [1-9]+,",
    "Post-test data collection failed": r"Failed to collect pipeline run JSONs",
    "Timeout getting pipeline": r"Message:.*resolver failed to get Pipeline.*resolution took longer than global timeout of .*",
    "Timeout getting task via git resolver from gitlab.cee": r"Message:.*Couldn't retrieve Task .*resolver type git.*https://gitlab.cee.redhat.com/.* resolution took longer than global timeout of .*",
}


def message_to_reason(msg: str) -> str | None:
    """
    Classifies an error message using regular expressions and returns the error name.

    Args:
      msg: The input error message string.

    Returns:
      The name of the error if a pattern matches, otherwise string "UNKNOWN".
    """
    msg = msg.replace("\n", " ")  # Remove newlines
    for error_name, pattern in ERRORS.items():
        if re.search(pattern, msg):
            return error_name
    print(f"Unknown error: {msg}")
    return "UNKNOWN"


def main():
    input_file = sys.argv[1]
    output_file = sys.argv[2]

    error_messages = []  # list of error messages
    error_reasons = []  # list of textual error reasons
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

                reason = message_to_reason(message)

                error_messages.append(message)
                error_reasons.append(reason)
                error_by_code[code] += 1
                error_by_reason[reason] += 1
    except FileNotFoundError:
        print("No errors file found, good :-D")

    data = {
        "error_by_code": error_by_code,
        "error_by_reason": error_by_reason,
        "error_reasons_simple": "; ".join(error_reasons),
        "error_messages": error_messages,
    }

    print(f"Errors detected: {len(error_messages)}")
    print("Errors by reason:")
    for k, v in error_by_reason.items():
        print(f"   {v} x {k}")

    with open(output_file, "w") as fp:
        json.dump(data, fp, indent=4)
    print(f"Data dumped to {output_file}")


if __name__ == "__main__":
    sys.exit(main())
