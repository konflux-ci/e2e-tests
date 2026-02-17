#!/usr/bin/env python3
# -*- coding: UTF-8 -*-

import collections
import csv
import json
import logging
import os
import re
import sys
from pathlib import Path
from typing import Any, Generator, Pattern

import yaml


# Constants for config file paths relative to this script
CONFIG_DIR = Path("ci-scripts/config")
ERRORS_CONFIG = CONFIG_DIR / "errors-loadtest_output.yaml"
CONTAINER_LOGS_CONFIG = CONFIG_DIR / "errors-container_logs.yaml"
TR_CONDITIONS_CONFIG = CONFIG_DIR / "errors-tr_conditions.yaml"


class ErrorMatcher:
    """
    Handles loading error patterns from configuration files and matching
    them against provided message strings.
    """

    def __init__(self, config_path: Path):
        """Initializes the matcher with patterns from the given config file."""
        self.patterns: list[tuple[str, Pattern, str]] = []
        self._load_config(config_path)

    def _load_config(self, relative_path: Path) -> None:
        """Loads and compiles regex patterns from a YAML configuration file."""
        base_path = Path(__file__).resolve().parent
        full_path = base_path / relative_path

        try:
            with open(full_path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or []
                for entry in data:
                    reason = entry.get("reason", "UNKNOWN")
                    regexp = re.compile(entry["regexp"])
                    caused_by = entry.get("caused_by", "UNKNOWN")
                    self.patterns.append((reason, regexp, caused_by))
        except FileNotFoundError:
            logging.warning(f"Config file not found: {full_path}")
        except yaml.YAMLError as e:
            logging.error(f"Error parsing YAML {full_path}: {e}")

    def match(self, message: str) -> tuple[str, str]:
        """
        Matches a message against loaded patterns. Returns a tuple of
        (reason, caused_by).
        """
        # Optimize: remove newlines and limit size for efficient matching
        clean_msg = message.replace("\n", " ")[-250000:]

        for reason, pattern, caused_by in self.patterns:
            if pattern.search(clean_msg):
                print(f"Matched error pattern: {pattern.pattern}")
                return reason, caused_by

        print(f"Unknown error: {clean_msg}")
        return "UNKNOWN", "UNKNOWN"


class Analyzer:
    """
    Investigates PipelineRuns, TaskRuns, and container logs to identify
    failure causes in a test dump directory.
    """

    def __init__(self, dump_dir: Path):
        """Initializes the analyzer with a target data dump directory."""
        self.dump_dir = dump_dir
        self.plr_matcher = ErrorMatcher(CONTAINER_LOGS_CONFIG)
        self.tr_matcher = ErrorMatcher(TR_CONDITIONS_CONFIG)

    def load_json(self, path: Path) -> dict[str, Any]:
        """Loads a JSON file and returns its content as a dictionary."""
        try:
            with open(path, "r", encoding="utf-8") as f:
                return json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            return {}

    def find_all_failed_plrs(self) -> Generator[dict[str, Any], None, None]:
        """Yields all PipelineRuns in the dump directory that have failed."""
        for root, _, files in os.walk(self.dump_dir):
            for filename in files:
                if not filename.startswith("collected-pipelinerun-"):
                    continue

                filepath = Path(root) / filename
                data = self.load_json(filepath)

                has_failure = False
                conditions = data.get("status", {}).get("conditions", [])
                for cond in conditions:
                    if (
                        cond.get("type") == "Succeeded"
                        and cond.get("status") == "False"
                    ):
                        has_failure = True
                        break

                if has_failure:
                    yield data

    def find_failed_plr_by_type(self, plr_type: str) -> dict[str, Any] | None:
        """Finds the first failed PipelineRun of a specific type."""
        target_label = "managed" if plr_type == "release" else "build"

        for plr in self.find_all_failed_plrs():
            labels = plr.get("metadata", {}).get("labels", {})
            if (
                labels.get("pipelines.appstudio.openshift.io/type")
                == target_label
            ):
                return plr
        return None

    def get_task_runs(self, plr: dict[str, Any]) -> list[str]:
        """Extracts TaskRun names from a PipelineRun status."""
        return [
            tr.get("name")
            for tr in plr.get("status", {}).get("childReferences", [])
            if tr.get("name")
        ]

    def check_task_run(self, ns: str, tr_name: str) -> tuple[bool, str, Path]:
        """Checks a specific TaskRun file for failure conditions."""
        tr_path = (
            self.dump_dir / ns / "0" / f"collected-taskrun-{tr_name}.json"
        )

        if not tr_path.exists():
            print(f"WARNING: Missing file: {tr_path}")
            return False, "Missing expected TaskRun file", tr_path

        data = self.load_json(tr_path)
        status = data.get("status", {})
        pod_name = status.get("podName", "")

        conditions = status.get("conditions", [])
        failure_msg = ""
        for cond in conditions:
            if cond.get("type") == "Succeeded":
                failure_msg = json.dumps(cond, sort_keys=True)
                break

        if not pod_name:
            return (
                False,
                failure_msg or "Missing expected fields in TaskRun",
                tr_path,
            )

        return (
            True,
            failure_msg or "Missing expected fields in TaskRun",
            tr_path,
        )

    def get_failed_containers(
        self, ns: str, tr_name: str
    ) -> Generator[tuple[str, str], None, None]:
        """Identifies which containers in a TaskRun have failed."""
        tr_path = (
            self.dump_dir / ns / "0" / f"collected-taskrun-{tr_name}.json"
        )
        data = self.load_json(tr_path)

        pod_name = data.get("status", {}).get("podName")
        if not pod_name:
            return

        steps = data.get("status", {}).get("steps", [])
        for step in steps:
            terminated = step.get("terminated", {})
            if (
                terminated.get("exitCode", 0) != 0
                and terminated.get("reason") != "TaskRunCancelled"
            ):
                yield pod_name, step.get("container")

    def read_container_log(self, ns: str, pod_name: str, container: str) -> str:
        """Reads the log file content for a specific container."""
        log_path = self.dump_dir / ns / "0" / f"pod-{pod_name}-{container}.log"
        print(f"Checking errors in {log_path}")
        try:
            return log_path.read_text(encoding="utf-8", errors="replace")
        except FileNotFoundError:
            return ""

    def investigate_plr(self, plr_type: str = "build") -> list[tuple[str, str]]:
        """
        Performs a deep investigation into a failed PipelineRun to find
        the root causes.
        """
        reasons = []
        try:
            plr = self.find_failed_plr_by_type(plr_type)
            if not plr:
                return [("SORRY PLR not found", "UNKNOWN")]

            ns = plr.get("metadata", {}).get("namespace")
            if not ns:
                return [("SORRY PLR namespace missing", "UNKNOWN")]

            for tr_name in self.get_task_runs(plr):
                is_valid, tr_msg, tr_file = self.check_task_run(ns, tr_name)

                if is_valid:
                    try:
                        for pod, container in self.get_failed_containers(
                            ns, tr_name
                        ):
                            log_content = self.read_container_log(
                                ns, pod, container
                            )
                            reason, caused_by = self.plr_matcher.match(
                                log_content
                            )
                            if reason != "SKIP":
                                reasons.append((reason, caused_by))
                    except FileNotFoundError as e:
                        print(f"Failed to locate required files: {e}")

                print(f"Checking errors in condition message of {tr_file}")
                reason, caused_by = self.tr_matcher.match(tr_msg)
                if reason != "SKIP":
                    reasons.append((reason, caused_by))

        except Exception as e:
            logging.exception("Investigating PLR failed")
            return [(f"SORRY {e}", "UNKNOWN")]

        return sorted(list(set(reasons)))


class StatsProcessor:
    """
    Collects, aggregates, and summarizes error statistics for final reporting.
    """

    def __init__(self):
        """Initializes collectors for error messages, codes, and reasons."""
        self.error_messages: list[str] = []
        self.error_by_code: dict[int, int] = collections.defaultdict(int)
        self.error_by_reason: dict[str, int] = collections.defaultdict(int)
        self.caused_by_list: list[str] = []

    def add(
        self,
        message: str,
        reason: str,
        caused_by: str | list[str],
        code: int = 0,
    ) -> None:
        """Adds a single error occurrence to the aggregated statistics."""
        final_reason = reason or message
        self.error_messages.append(message)
        self.error_by_code[code] += 1
        self.error_by_reason[final_reason] += 1

        causes = caused_by if isinstance(caused_by, list) else [caused_by]
        for cause in causes:
            if cause and cause != "SKIP":
                self.caused_by_list.append(cause)

    def dump(self, output_path: Path) -> None:
        """Writes the aggregated statistics to a JSON file."""
        data = {
            "error_by_code": self.error_by_code,
            "error_by_reason": self.error_by_reason,
            "error_reasons_simple": "; ".join(
                f"{v}x {k}"
                for k, v in self.error_by_reason.items()
                if k != "Post-test data collection failed"
            ),
            "error_messages": self.error_messages,
            "error_caused_by": self.caused_by_list,
            "error_caused_by_simple": ", ".join(set(self.caused_by_list)),
        }

        print(f"Errors detected: {len(self.error_messages)}")
        print("Errors by reason:")
        for k, v in self.error_by_reason.items():
            print(f"   {v}x {k}")

        with open(output_path, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=4)
        print(f"Data dumped to {output_path}")


def process_csv_mode(
    input_file: Path, timings_file: Path, output_file: Path, dump_dir: Path
):
    """
    Processes errors identified in a CSV input file, potentially triggering
    deeper investigations into the dump directory.
    """
    matcher = ErrorMatcher(ERRORS_CONFIG)
    analyzer = Analyzer(dump_dir)
    stats = StatsProcessor()

    if input_file.exists():
        with open(input_file, "r", encoding="utf-8") as f:
            reader = csv.reader(f)
            for row in reader:
                if not row:
                    continue

                code = int(row[1]) if len(row) > 1 else 0
                message = row[2] if len(row) > 2 else ""

                reason, caused_by = matcher.match(message)
                current_causes = [caused_by]

                if reason == "Pipeline failed":
                    sub_errors = analyzer.investigate_plr("build")
                    reason += ": " + ", ".join(r[0] for r in sub_errors)
                    current_causes = [r[1] for r in sub_errors]

                elif reason == "Release Pipeline failed":
                    sub_errors = analyzer.investigate_plr("release")
                    reason += ": " + ", ".join(r[0] for r in sub_errors)
                    current_causes = [r[1] for r in sub_errors]

                stats.add(message, reason, current_causes, code)
    else:
        print("No errors file found, good :-D")

    timings = {}
    if timings_file.exists():
        try:
            with open(timings_file, "r", encoding="utf-8") as f:
                timings = json.load(f)
        except json.JSONDecodeError:
            print("No KPI metrics in timings data, strange :-(")
            stats.add(
                "No KPI metrics in timings data",
                "No KPI metrics in timings data",
                [],
            )
    else:
        print("No timings file found, strange :-/")
        stats.add("No timings file found", "No timings file found", [])

    if timings.get("KPI", {}).get("mean") == -1:
        if not stats.error_messages:
            stats.add("No test run finished", "No test run finished", [])

    stats.dump(output_file)


def process_dir_mode(dump_dir: Path):
    """
    Processes all failed PipelineRuns found in a dump directory to build
    an error summary.
    """
    output_file = dump_dir / "errors-output.json"
    analyzer = Analyzer(dump_dir)
    stats = StatsProcessor()

    reasons = []

    for plr in analyzer.find_all_failed_plrs():
        ns = plr.get("metadata", {}).get("namespace")
        if not ns:
            continue

        for tr_name in analyzer.get_task_runs(plr):
            is_valid, tr_msg, tr_file = analyzer.check_task_run(ns, tr_name)

            if is_valid:
                try:
                    for pod, container in analyzer.get_failed_containers(
                        ns, tr_name
                    ):
                        log = analyzer.read_container_log(ns, pod, container)
                        reason, caused_by = analyzer.plr_matcher.match(log)
                        if reason != "SKIP":
                            reasons.append((reason, caused_by))
                except FileNotFoundError as e:
                    print(f"Failed to locate required files: {e}")

            print(f"Checking errors in condition message of {tr_file}")
            reason, caused_by = analyzer.tr_matcher.match(tr_msg)
            if reason != "SKIP":
                reasons.append((reason, caused_by))

    reasons.sort()

    for r, caused_by in reasons:
        stats.add(r, r, caused_by)

    stats.dump(output_file)


def main():
    """Entry point for the script, handling command line arguments."""
    args = sys.argv[1:]

    if len(args) == 4:
        process_csv_mode(
            Path(args[0]), Path(args[1]), Path(args[2]), Path(args[3])
        )
    elif len(args) == 1:
        process_dir_mode(Path(args[0]))
    else:
        sys.exit(1)


if __name__ == "__main__":
    main()