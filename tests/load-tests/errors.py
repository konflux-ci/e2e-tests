#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import collections
import csv
import json
import logging
import os
import re
import sys
import yaml


# Column indexes in input data
COLUMN_WHEN = 0
COLUMN_CODE = 1
COLUMN_MESSAGE = 2


# Errors patterns we recognize (when newlines were removed)
# Generic guideline on constructing error reasons: <who - which tool failed> <what - what action failed> <why - why it failed>
with open("ci-scripts/config/errors-loadtest_output.yaml", "r") as fd:
    ERRORS = [(e["reason"], re.compile(e["regexp"])) for e in yaml.load(fd, Loader=yaml.SafeLoader)]
with open("ci-scripts/config/errors-container_logs.yaml", "r") as fd:
    FAILED_PLR_ERRORS = [(e["reason"], re.compile(e["regexp"])) for e in yaml.load(fd, Loader=yaml.SafeLoader)]
with open("ci-scripts/config/errors-tr_conditions.yaml", "r") as fd:
    FAILED_TR_ERRORS = [(e["reason"], re.compile(e["regexp"])) for e in yaml.load(fd, Loader=yaml.SafeLoader)]


def message_to_reason(reasons_and_errors: set, msg: str) -> str:
    """
    Classifies an error message using regular expressions and returns the error name.

    Args:
      msg: The input error message string.

    Returns:
      The name of the error if a pattern matches, otherwise string "UNKNOWN".
    """
    msg = msg.replace("\n", " ")  # Remove newlines
    msg = msg[-250000:]   # Just look at last 250k bytes
    for error_name, pattern in reasons_and_errors:
        if re.search(pattern, msg):
            return error_name
    print(f"Unknown error: {msg}")
    return "UNKNOWN"


def add_reason(error_messages, error_by_code, error_by_reason, message, reason="", code=0):
    if reason == "":
        reason = message
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

    for plr in find_all_failed_plrs(data_dir):
        if plr_type == "build":
            plr_type_label = "build"
        elif plr_type == "release":
            plr_type_label = "managed"
        else:
            raise Exception("Unknown PLR type")

        # Skip PLRs that do not have expected type
        try:
            if plr["metadata"]["labels"]["pipelines.appstudio.openshift.io/type"] != plr_type_label:
                continue
        except KeyError:
            continue

        return plr


def find_trs(plr):
    try:
        for tr in plr["status"]["childReferences"]:
            yield tr["name"]
    except KeyError:
        return


def check_failed_taskrun(data_dir, ns, tr_name):
    datafile = os.path.join(data_dir, ns, "0", "collected-taskrun-" + tr_name + ".json")
    try:
        data = load(datafile)
    except FileNotFoundError as e:
        print(f"ERROR: Missing file: {str(e)}")
        return False, "Missing expected TaskRun file"

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
    datafile = os.path.join(data_dir, ns, "0", "collected-taskrun-" + tr_name + ".json")
    data = load(datafile)

    try:
        pod_name = data["status"]["podName"]
        for sr in data["status"]["steps"]:
            if sr["terminated"]["exitCode"] == 0:
                continue
            if sr["terminated"]["reason"] == "TaskRunCancelled":
                continue
            yield (pod_name, sr["container"])
    except KeyError:
        return


def load_container_log(data_dir, ns, pod_name, cont_name):
    datafile = os.path.join(data_dir, ns, "0", "pod-" + pod_name + "-" + cont_name + ".log")
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
        logging.exception("Investigating PLR failed")
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
