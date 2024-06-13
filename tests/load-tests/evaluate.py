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
COLUMN_METRIC = 1
COLUMN_DURATION = 2
COLUMN_PARAMS = 3
COLUMN_ERROR = 4

# Metrics we care about that together form KPI metric duration
METRICS = [
    "HandleUser",
    "createApplication",
    "validateApplication",
    "createIntegrationTestScenario",
    "validateIntegrationTestScenario",
    "createComponent",
    "validatePipelineRunCreation",
    "validatePipelineRunCondition",
    "validatePipelineRunSignature",
    "validateSnapshotCreation",
    "validateTestPipelineRunCreation",
    "validateTestPipelineRunCondition",
]


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
    elif len(data) == 1:
        return {
            "samples": 1,
            "min": data[0],
            "mean": data[0],
            "max": data[0],
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
    input_file = sys.argv[1]
    output_file = sys.argv[2]

    stats_raw = {}

    with open(input_file, "r") as fp:
        csvreader = csv.reader(fp)
        for row in csvreader:
            if row == []:
                continue

            when = str2date(row[COLUMN_WHEN])
            metric = row[COLUMN_METRIC]
            duration = float(row[COLUMN_DURATION])
            error = row[COLUMN_ERROR] != "<nil>"

            for m in METRICS:
                if m not in stats_raw:
                    stats_raw[m] = {
                        "pass": {"duration": [], "when": []},
                        "fail": {"duration": [], "when": []},
                    }

                if metric.endswith("." + m):
                    stats_raw[m]["fail" if error else "pass"]["duration"].append(duration)
                    stats_raw[m]["fail" if error else "pass"]["when"].append(when)

    # print(f"Raw stats: {stats_raw}")

    stats = {}
    kpi_sum = 0.0
    kpi_errors = 0

    for m in METRICS:
        stats[m] = {"pass": {}, "fail": {}}
        stats[m]["pass"]["duration"] = count_stats(stats_raw[m]["pass"]["duration"])
        stats[m]["fail"]["duration"] = count_stats(stats_raw[m]["fail"]["duration"])
        stats[m]["pass"]["when"] = count_stats_when(stats_raw[m]["pass"]["when"])
        stats[m]["fail"]["when"] = count_stats_when(stats_raw[m]["fail"]["when"])

        if "mean" in stats[m]["pass"]["duration"]:
            kpi_sum += stats[m]["pass"]["duration"]["mean"]

        s = stats[m]["pass"]["duration"]["samples"] + stats[m]["fail"]["duration"]["samples"]
        if s == 0:
            stats[m]["error_rate"] = None
        else:
            stats[m]["error_rate"] = stats[m]["fail"]["duration"]["samples"] / s
            kpi_errors += stats[m]["fail"]["duration"]["samples"]

    stats["KPI"] = {}
    stats["KPI"]["mean"] = kpi_sum
    stats["KPI"]["errors"] = kpi_errors

    print(f"KPI mean: {stats['KPI']['mean']}")
    print(f"KPI errors: {stats['KPI']['errors']}")

    with open(output_file, "w") as fp:
        json.dump(stats, fp, indent=4)
    print(f"Stats dumped to {output_file}")


if __name__ == "__main__":
    sys.exit(main())
