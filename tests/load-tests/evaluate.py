#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import csv
import datetime
import json
import re
import statistics
import sys


# Column indexes in input data
COLUMN_WHEN = 0
COLUMN_PER_USER_T = 1
COLUMN_PER_APP_T = 2
COLUMN_PER_COMP_T = 3
COLUMN_REPEATS_COUNTER = 4
COLUMN_METRIC = 5
COLUMN_DURATION = 6
COLUMN_PARAMS = 7
COLUMN_ERROR = 8

# Metrics we care about that together form KPI metric duration
METRICS = [
    "HandleUser",
    "createApplication",
    "validateApplication",
    "createIntegrationTestScenario",
    "createComponent",
    "getPaCPullNumber",
    "validateComponent",
    "validatePipelineRunCreation",
    "validatePipelineRunCondition",
    "validatePipelineRunSignature",
    "validateSnapshotCreation",
    "validateTestPipelineRunCreation",
    "validateTestPipelineRunCondition",
    "createReleasePlan",
    "createReleasePlanAdmission",
    "validateReleasePlan",
    "validateReleasePlanAdmission",
    "validateReleaseCreation",
    "validateReleasePipelineRunCreation",
    "validateReleasePipelineRunCondition",
    "validateReleaseCondition",
]

# These metrics will be ignored if running on non-CI cluster
METRICS_CI = [
    "HandleUser",
]

# These metrics will be ignored if ITS was skipped
METRICS_ITS = [
    "createIntegrationTestScenario",
    "validateTestPipelineRunCreation",
    "validateTestPipelineRunCondition",
]

# These metrics will be ignored if Release was skipped
METRICS_RELEASE = [
    "createReleasePlan",
    "createReleasePlanAdmission",
    "validateReleasePlan",
    "validateReleasePlanAdmission",
    "validateReleaseCreation",
    "validateReleasePipelineRunCreation",
    "validateReleasePipelineRunCondition",
    "validateReleaseCondition",
]


class SinglePass:
    """Structure to record data about one specific pass through loadtest workload, identified by an identier (touple with loadtest's per user, per application and per component thread index and repeats counter."""

    def __init__(self):
        self._metrics = {}

    def add(self, metric, duration):
        """Adds given metric to data about this pass."""
        assert metric not in self._metrics
        self._metrics[metric] = duration

    def complete(self, expected_metrics):
        """Checks if we have all expected metrics."""
        current = set(self._metrics.keys())
        return current == expected_metrics

    def total(self):
        """Return total duration."""
        return sum(self._metrics.values())

    @staticmethod
    def i_matches(identifier1, identifier2):
        """Check if first provided identifier matches second one. When we have -1 instead of some value(s) in the first identifier, it acts as a wildcard."""
        if identifier1[3] == -1 or identifier1[3] == identifier2[3]:
            if identifier1[2] == -1 or identifier1[2] == identifier2[2]:
                if identifier1[1] == -1 or identifier1[1] == identifier2[1]:
                    if identifier1[0] == -1 or identifier1[0] == identifier2[0]:
                        return True
        return False

    @staticmethod
    def i_complete(identifier):
        """Check this is complete identifier (does not contain wildcards)."""
        return -1 not in identifier


def str2date(date_str):
    if isinstance(date_str, datetime.datetime):
        return date_str
    else:
        try:
            return datetime.datetime.fromisoformat(date_str)
        except ValueError:   # Python before 3.11
            # Convert "...Z" to "...+00:00"
            date_str = date_str.replace("Z", "+00:00")
            # Remove microseconds part
            date_str = re.sub(r"(.*)(\.\d+)(\+.*)", r"\1\3", date_str)
            # Convert simplified date
            return datetime.datetime.fromisoformat(date_str)

def count_stats(data):
    if len(data) == 0:
        return {
            "samples": 0,
        }
    else:
        return {
            "samples": len(data),
            "min": min(data),
            "mean": statistics.mean(data),
            "max": max(data),
        }

def count_stats_when(data):
    if len(data) == 0:
        return {}
    elif len(data) == 1:
        return {
            "min": data[0].isoformat(),
            "max": data[0].isoformat(),
            "mean": data[0].isoformat(),
            "span": 0,
        }
    else:
        return {
            "min": min(data).isoformat(),
            "max": max(data).isoformat(),
            "mean": datetime.datetime.fromtimestamp(statistics.mean([i.timestamp() for i in data]), tz=datetime.timezone.utc).isoformat(),
            "span": (max(data) - min(data)).total_seconds(),
        }


def main():
    options_file = sys.argv[1]
    input_file = sys.argv[2]
    output_file = sys.argv[3]

    # Load test options
    with open(options_file, "r") as fp:
        options = json.load(fp)

    # Determine what metrics we need to skip based on options
    to_skip = []
    if options["Stage"]:
        print("NOTE: Ignoring CI cluster related metrics because running against non-CI cluster")
        to_skip += METRICS_CI
    if options["TestScenarioGitURL"] == "":
        print("NOTE: Ignoring ITS related metrics because they were disabled at test run")
        to_skip += METRICS_ITS
    if options["ReleasePolicy"] == "":
        print("NOTE: Ignoring Release related metrics because they were disabled at test run")
        to_skip += METRICS_RELEASE

    # When processing, only consider these metrics
    expected_metrics = set(METRICS) - set(to_skip)

    stats_raw = {}
    stats_passes = {}

    rows_incomplete = []

    # Prepopulate stats_raw data structure
    for m in expected_metrics:
        stats_raw[m] = {
            "pass": {"duration": [], "when": []},
            "fail": {"duration": [], "when": []},
        }

    with open(input_file, "r") as fp:
        csvreader = csv.reader(fp)
        for row in csvreader:
            if row == []:
                continue

            when = str2date(row[COLUMN_WHEN])
            per_user_t = int(row[COLUMN_PER_USER_T])
            per_app_t = int(row[COLUMN_PER_APP_T])
            per_comp_t = int(row[COLUMN_PER_COMP_T])
            repeats_counter = int(row[COLUMN_REPEATS_COUNTER])
            metric = row[COLUMN_METRIC].split(".")[-1]
            duration = float(row[COLUMN_DURATION])
            error = row[COLUMN_ERROR] != "<nil>"

            if metric not in expected_metrics:
                continue

            # First add this record to stats_raw that allows us to track stats per metric
            stats_raw[metric]["fail" if error else "pass"]["duration"].append(duration)
            stats_raw[metric]["fail" if error else "pass"]["when"].append(when)

            # Second add this record to stats_passes that allows us to track full completed passes
            if not error:
                identifier = (per_user_t, per_app_t, per_comp_t, repeats_counter)

                if SinglePass.i_complete(identifier):
                    if identifier not in stats_passes:
                        stats_passes[identifier] = SinglePass()
                    stats_passes[identifier].add(metric, duration)
                else:
                    # Safe this metric for later once we have all passes
                    rows_incomplete.append((identifier, metric, duration))

    # Now when we have data about all passes, add metrics that had incomplete identifiers (with wildcards)
    for incomplete in rows_incomplete:
        identifier, metric, duration = incomplete
        found = [v for k, v in stats_passes.items() if SinglePass.i_matches(identifier, k)]
        for i in found:
            i.add(metric, duration)

    #print("Raw stats:")
    #print(json.dumps(stats_raw, indent=4, default=lambda o: '<' + str(o) + '>'))
    #print(json.dumps({str(k): v for k, v in stats_passes.items()}, indent=4, default=lambda o: '<' + str(o._metrics) + '>'))

    stats = {}
    kpi_mean_data = []
    kpi_successes = 0
    kpi_errors = 0

    for m in expected_metrics:
        stats[m] = {"pass": {"duration": {"samples": 0}, "when": {}}, "fail": {"duration": {"samples": 0}, "when": {}}}
        if m in stats_raw:
            stats[m]["pass"]["duration"] = count_stats(stats_raw[m]["pass"]["duration"])
            stats[m]["fail"]["duration"] = count_stats(stats_raw[m]["fail"]["duration"])
            stats[m]["pass"]["when"] = count_stats_when(stats_raw[m]["pass"]["when"])
            stats[m]["fail"]["when"] = count_stats_when(stats_raw[m]["fail"]["when"])

    for k, v in stats_passes.items():
        if v.complete(expected_metrics):
            kpi_successes += 1
            kpi_mean_data.append(v.total())
        else:
            kpi_errors += 1

    stats["KPI"] = {}
    stats["KPI"]["mean"] = sum(kpi_mean_data) / kpi_successes if kpi_successes > 0 else -1
    stats["KPI"]["successes"] = kpi_successes
    stats["KPI"]["errors"] = kpi_errors

    #print("Final stats:")
    #print(json.dumps(stats, indent=4))

    print(f"KPI mean: {stats['KPI']['mean']}")
    print(f"KPI successes: {stats['KPI']['successes']}")
    print(f"KPI errors: {stats['KPI']['errors']}")

    with open(output_file, "w") as fp:
        json.dump(stats, fp, indent=4)
    print(f"Stats dumped to {output_file}")


if __name__ == "__main__":
    sys.exit(main())
