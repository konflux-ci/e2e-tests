#!/usr/bin/env python

import argparse
import collections
import csv
import datetime
import json
import logging
import os
import os.path
import sys
import yaml
import time
import operator
import statistics
import re

import tabulate


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

class DateTimeDecoder(json.JSONDecoder):
    def __init__(self, *args, **kwargs):
        super().__init__(object_hook=self.object_hook, *args, **kwargs)

    def object_hook(self, o):
        ret = {}
        for key, value in o.items():
            if isinstance(value, str):
                try:
                    ret[key] = str2date(value)
                except ValueError:
                    ret[key] = value
            else:
                ret[key] = value
        return ret

class Something:
    def __init__(self, data_dir, dump_json):
        self.data_pipelineruns = []
        self.data_taskruns = []

        self.data_dir = data_dir
        self.dump_json = dump_json

        self.pr_skips = 0  # how many PipelineRuns we skipped
        self.tr_skips = 0  # how many TaskRuns we skipped
        self.step_skips = 0  # how many steps we skipped

        self._populate(self.data_dir)

    def _load_json(self, path):
        with open(path, "r") as fp:
            return json.load(fp, cls=DateTimeDecoder)

    def _populate(self, data_dir):
        for currentpath, folders, files in os.walk(data_dir):
            for datafile in files:
                datafile = os.path.join(currentpath, datafile)

                start = time.time()
                if datafile.endswith(".yaml") or datafile.endswith(".yml"):
                    with open(datafile, "r") as fd:
                        data = yaml.safe_load(fd)
                elif datafile.endswith(".json"):
                    try:
                        data = self._load_json(datafile)
                    except json.decoder.JSONDecodeError:
                        logging.warning(f"File {datafile} is malfrmed, skipping it")
                        continue
                else:
                    continue
                end = time.time()
                logging.debug(f"Loaded {datafile} in {(end - start):.2f} seconds")

                if "kind" not in data:
                    logging.info(f"Skipping {datafile} as it does not contain kind")
                    continue

                if data["kind"] == "List":
                    if "items" not in data:
                        logging.info(f"Skipping {datafile} as it does not contain items")
                        continue

                    for i in data["items"]:
                        self._populate_add_one(i)
                else:
                    self._populate_add_one(data)

    def _populate_add_one(self, data):
        if "kind" not in data:
            logging.info("Skipping item because it does not have kind")
            return

        if data["kind"] == "PipelineRun":
            self._populate_pipelinerun(data)
        elif data["kind"] == "TaskRun":
            self._populate_taskrun(data)
        else:
            logging.debug(f"Skipping item because it has unexpected kind {data['kind']}")
            return

    def _populate_pipelinerun(self, pr):
        """Load PipelineRun."""
        try:
            pr_name = pr["metadata"]["name"]
            pr_type = pr["metadata"]["labels"]["pipelines.appstudio.openshift.io/type"]
            pr_creation_time = str2date(pr["metadata"]["creationTimestamp"])
            pr_start_time = str2date(pr["status"]["startTime"])
            pr_completion_time = str2date(pr["status"]["completionTime"])
            _pr_succeeded = [cond for cond in pr["status"]["conditions"] if cond["type"] == "Succeeded"]
            assert len(_pr_succeeded) == 1, f"PipelineRun should have exactly one 'Succeeded' condition: {_pr_succeeded}"
            pr_result = _pr_succeeded[0]["status"] == "True"
            pr_tasks = [t["name"] for t in pr["status"]["childReferences"] if t["kind"] == "TaskRun"]
        except KeyError as e:
            logging.info(f"PipelineRun incomplete, skipping: {e}, {str(pr)[:200]}")
            self.pr_skips += 1
            return

        self.data_pipelineruns.append({
            "name": pr_name,
            "type": pr_type,
            "result": pr_result,
            "creation": pr_creation_time,
            "start": pr_start_time,
            "completion": pr_completion_time,
            "tasks": pr_tasks,
        })

    def _populate_taskrun(self, tr):
        """Load TaskRun (and it's steps)."""
        try:
            tr_name = tr["metadata"]["name"]
            tr_task = tr["metadata"]["labels"]["tekton.dev/task"]
            tr_pipelinerun = tr["metadata"]["labels"]["tekton.dev/pipelineRun"]
            tr_creation_time = str2date(tr["metadata"]["creationTimestamp"])
            tr_start_time = str2date(tr["status"]["startTime"])
            tr_completion_time = str2date(tr["status"]["completionTime"])
            _tr_succeeded = [cond for cond in tr["status"]["conditions"] if cond["type"] == "Succeeded"]
            assert len(_tr_succeeded) == 1, f"TaskRun should have exactly one 'Succeeded' condition: {_tr_succeeded}"
            tr_result = _tr_succeeded[0]["status"] == "True"

            tr_platform = None
            if "params" in tr["spec"]:
                for p in tr["spec"]["params"]:
                    if p["name"] == "PLATFORM":
                        tr_platform = p["value"]

            tr_steps = {}
            for s in tr["status"]["steps"]:
                try:
                    if s["terminationReason"] == "Completed" and s["terminated"]["exitCode"] == 0 and s["terminated"]["reason"] == "Completed":
                        s_result = True
                    else:
                        s_result = False
                    if s["terminated"]["startedAt"] is None or s["terminated"]["finishedAt"] is None:
                        raise KeyError("Field terminated.startedAt and/or terminated.finishedAt is None")
                    tr_steps[s["container"]] = {
                        "started": str2date(s["terminated"]["startedAt"]),
                        "finished": str2date(s["terminated"]["finishedAt"]),
                        "result": s_result,
                    }
                except KeyError as e:
                    logging.info(f"Step incomplete, skipping: {e}, {str(s)[:200]}")
                    self.step_skips += 1

        except KeyError as e:
            logging.info(f"TaskRun incomplete, skipping: {e}, {str(tr)[:200]}")
            self.tr_skips += 1
            return

        self.data_taskruns.append({
            "name": tr_name,
            "task": tr_task,
            "pipelinerun": tr_pipelinerun,
            "result": tr_result,
            "creation": tr_creation_time,
            "start": tr_start_time,
            "completion": tr_completion_time,
            "platform": tr_platform,
            "steps": tr_steps,
        })

    def _dump_as_csv(self, name, table, table_header):
        name_full = os.path.join(self.data_dir, name)
        with open(name_full, "w") as fd:
            writer = csv.writer(fd)
            writer.writerow(table_header)
            for row in table:
                writer.writerow(row)

    def _format_interval(self, interval):
        """
        Return interval (list of two datestamps) formated as a string with ISO formated dates.
        """
        return f"({interval[0].isoformat()} - {interval[1].isoformat()})"

    def _merge_time_interval(self, new, existing):
        """
        Merge the new interval with first overlaping existing interval or add new one to list.

        Parameters:
            new ... time interval (lis with start and end time) we want to merge into current list of intervals
            existing ... iterable of intervals with current non-overlaping time intervals

        Returns:
            New list of existing intervals

        Example:
            Say you have list of these intervals:
              [   <----->  <-->          <---->         ]
            And you want to add this one:
                      <--->
            So you should end up with this list:
              [   <----------->          <---->         ]
        """
        start = 0
        end = 1

        for t in existing:
            # If both ends are inside of existing interval, we ignore it
            if t[start] <= new[start] <= t[end] and t[start] <= new[end] <= t[end]:
                logging.info(f"Interval {self._format_interval(new)} is inside of member {self._format_interval(t)}, no action needed")
                return existing   # no more processing needed

            # If start is inside existing interval, but end is outside of it,
            # we extend existing interval
            if t[start] <= new[start] <= t[end]:
                if new[end] > t[end]:
                    logging.info(f"Interval {self._format_interval(new)} extends member {self._format_interval(t)}, so adding to right of it and need to recompute")
                    existing.remove(t)
                    t[end] = new[end]
                    return self._merge_time_interval(t, existing)

            # If end is inside existing interval, but start is outside of it,
            # we extend existing interval
            if t[start] <= new[end] <= t[end]:
                if new[start] < t[start]:
                    logging.info(f"Interval {self._format_interval(new)} extends member {self._format_interval(t)}, so adding to left of it and need to recompute")
                    existing.remove(t)
                    t[start] = new[start]
                    return self._merge_time_interval(t, existing)

        # If new interval did not collided with none of existing ones,
        # just add it to the list
        logging.info(f"Interval {self._format_interval(new)} does not collide with any member, adding it")
        return existing + [new]


    def doit(self):
        # Normalize data into the structure we will use and do some cross checks
        data = {}

        # Populate PRs to the data structure
        for pr in self.data_pipelineruns:
            data[pr["name"]] = {
                "type": pr["type"],
                "result": pr["result"],
                "creation": pr["creation"],
                "start": pr["start"],
                "completion": pr["completion"],
                "tasks": pr["tasks"],
                "taskruns": {},
            }

        # Populate TRs to the data structure
        for tr in self.data_taskruns:
            # Check if TR's PR was correctly loaded
            if tr["pipelinerun"] not in data:
                logging.warning(f"TaskRuns {tr['name']} pipelinerun {tr['pipelinerun']} was not loaded, skipping it")
                self.pr_skips += 1
                continue

            # Check if TR's task is expected by TR's PR
            if tr["name"] not in data[tr["pipelinerun"]]["tasks"]:
                logging.error(f"TaskRuns {tr['name']} ({tr['task']}) missing in pipelinerun {tr['pipelinerun']}, this is strange")
                sys.exit(1)   # If this happened, it is very strange

            data[tr["pipelinerun"]]["taskruns"][tr["name"]] = {
                "task": tr["task"],
                "result": tr["result"],
                "creation": tr["creation"],
                "start": tr["start"],
                "completion": tr["completion"],
                "platform": tr["platform"],
                "steps": tr["steps"],
            }

        # Check if we have all TRs for all the PRs
        for pr_name, pr_data in data.items():
            expected_trs = set(pr_data["tasks"])
            current_trs = set(list(pr_data["taskruns"].keys()))
            if expected_trs != current_trs:
                logging.warning(f"Not all pipelinerun {pr_name} task runs were loaded: {expected_trs - current_trs}")
                self.tr_skips += len(expected_trs) - len(current_trs)

        # Collect data
        result = {
            "pipelineruns": {
            },
            "taskruns": {
            },
            "platformtaskruns": {
            },
            "steps": {
            },
        }
        for pr_name, pr_data in data.items():
            pr_id = pr_data["type"]
            logging.debug(f"Working on PipelineRun {pr_id}")

            if pr_id not in result["pipelineruns"]:
                result["pipelineruns"][pr_id] = {
                    "passed": {
                        "duration": [],
                        "running": [],
                        "scheduled": [],
                        "idle": [],
                    },
                    "failed": {
                        "duration": [],
                        "running": [],
                        "scheduled": [],
                        "idle": [],
                    },
                }

            # Composing list of TRs intervals to get idle time later
            pr_tr_intervals = []
            for tr_name, tr_data in pr_data["taskruns"].items():
                pr_tr_intervals = self._merge_time_interval([tr_data["creation"], tr_data["completion"]], pr_tr_intervals)

            pr_result = "passed" if pr_data["result"] else "failed"
            pr_duration = pr_data["completion"] - pr_data["creation"]
            pr_running = pr_data["completion"] - pr_data["start"]
            pr_scheduled = pr_data["start"] - pr_data["creation"]
            pr_idle = pr_duration - sum([i[1] - i[0] for i in pr_tr_intervals], datetime.timedelta())

            result["pipelineruns"][pr_id][pr_result]["duration"].append(pr_duration)
            result["pipelineruns"][pr_id][pr_result]["running"].append(pr_running)
            result["pipelineruns"][pr_id][pr_result]["scheduled"].append(pr_scheduled)
            result["pipelineruns"][pr_id][pr_result]["idle"].append(pr_idle)

            for tr_name, tr_data in pr_data["taskruns"].items():
                tr_id = f"{pr_id}/{tr_data['task']}"
                ptr_id = f"{pr_id}/{tr_data['task']}-{tr_data['platform']}"
                logging.debug(f"Working on TaskRun {tr_id}")

                if tr_id not in result["taskruns"]:
                    result["taskruns"][tr_id] = {
                        "passed": {
                            "duration": [],
                            "running": [],
                            "scheduled": [],
                            "idle": [],
                        },
                        "failed": {
                            "duration": [],
                            "running": [],
                            "scheduled": [],
                            "idle": [],
                        },
                    }

                # Composing list of TRs intervals to get idle time later
                tr_s_intervals = []
                for s_name, s_data in tr_data["steps"].items():
                    tr_s_intervals = self._merge_time_interval([s_data["started"], s_data["finished"]], tr_s_intervals)

                tr_result = "passed" if tr_data["result"] else "failed"
                tr_duration = tr_data["completion"] - tr_data["creation"]
                tr_running = tr_data["completion"] - tr_data["start"]
                tr_scheduled = tr_data["start"] - tr_data["creation"]
                tr_idle = tr_duration - sum([i[1] - i[0] for i in tr_s_intervals], datetime.timedelta())

                result["taskruns"][tr_id][tr_result]["duration"].append(tr_duration)
                result["taskruns"][tr_id][tr_result]["running"].append(tr_running)
                result["taskruns"][tr_id][tr_result]["scheduled"].append(tr_scheduled)
                result["taskruns"][tr_id][tr_result]["idle"].append(tr_idle)

                if tr_data['platform'] is not None:
                    if ptr_id not in result["platformtaskruns"]:
                        result["platformtaskruns"][ptr_id] = {
                            "passed": {
                                "duration": [],
                                "running": [],
                                "scheduled": [],
                                "idle": [],
                            },
                            "failed": {
                                "duration": [],
                                "running": [],
                                "scheduled": [],
                                "idle": [],
                            },
                        }
                    result["platformtaskruns"][ptr_id][tr_result]["duration"].append(tr_duration)
                    result["platformtaskruns"][ptr_id][tr_result]["running"].append(tr_running)
                    result["platformtaskruns"][ptr_id][tr_result]["scheduled"].append(tr_scheduled)
                    result["platformtaskruns"][ptr_id][tr_result]["idle"].append(tr_idle)

                for s_name, s_data in tr_data["steps"].items():
                    s_id = f"{tr_id}/{s_name}"
                    logging.debug(f"Working on Step {s_id}")

                    if s_id not in result["steps"]:
                        result["steps"][s_id] = {
                            "passed": {
                                "duration": [],
                            },
                            "failed": {
                                "duration": [],
                            },
                        }

                    s_result = "passed" if s_data["result"] else "failed"
                    s_duration = s_data["finished"] - s_data["started"]

                    result["steps"][s_id][s_result]["duration"].append(s_duration)

        # Compute statistical data
        for e in ("pipelineruns", "taskruns", "platformtaskruns", "steps"):
            for my_id, my_data1 in result[e].items():
                for my_result, my_data2 in my_data1.items():
                    for my_stat, my_data3 in my_data2.items():
                        if len(my_data3) == 0:
                            my_data2[my_stat] = {
                                "samples": 0,
                            }
                            continue

                        if isinstance(my_data3[0], datetime.timedelta):
                            my_data3 = [i.seconds for i in my_data3]

                        my_data2[my_stat] = {
                            "mean": statistics.mean(my_data3),
                            "stdev": statistics.stdev(my_data3) if len(my_data3) >= 2 else 0,
                            "min": min(my_data3),
                            "max": max(my_data3),
                            "samples": len(my_data3),
                        }

        dumpable_result = {
            "errors": {
                "pipelineruns": self.pr_skips,
                "taskruns": self.tr_skips,
                "steps": self.step_skips,
                "total": self.pr_skips + self.tr_skips + self.step_skips,
            },
            "stats": result,
        }
        with open(self.dump_json, "w") as fd:
            json.dump(dumpable_result, fd, sort_keys=True, indent=4)

        print("Result data:", self.dump_json)
        print("PipelineRuns skipped:", self.pr_skips)
        print("TaskRuns skipped:", self.tr_skips)
        print("Steps skipped:", self.step_skips)

def doit(args):
    something = Something(
        data_dir=args.data_dir,
        dump_json=args.dump_json,
    )
    return something.doit()


def main():
    parser = argparse.ArgumentParser(
        description="Show PipelineRuns and TaskRuns",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "--data-dir",
        required=True,
        help="Directory from where to load YAML/JSON data",
    )
    parser.add_argument(
        "--dump-json",
        default="get-taskrun-durations.json",
        help="File where to dump computed stats",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Show verbose output",
    )
    parser.add_argument(
        "-d",
        "--debug",
        action="store_true",
        help="Show debug output",
    )
    args = parser.parse_args()

    fmt = "%(asctime)s %(name)s %(levelname)s %(message)s"
    if args.verbose:
        logging.basicConfig(format=fmt, level=logging.INFO)
    elif args.debug:
        logging.basicConfig(format=fmt, level=logging.DEBUG)
    else:
        logging.basicConfig(format=fmt)

    logging.debug(f"Args: {args}")

    return doit(args)


if __name__ == "__main__":
    sys.exit(main())
