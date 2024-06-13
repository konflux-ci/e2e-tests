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
    def __init__(self, data_dir):
        self.data_taskruns = []
        self.data_dir = data_dir

        self.tr_skips = 0  # how many TaskRuns we skipped

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
                    data = self._load_json(datafile)
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

        print(f"We loaded {len(self.data_taskruns)} and skipped {self.tr_skips} TaskRuns")

    def _populate_add_one(self, something):
        if "kind" not in something:
            logging.info("Skipping item because it does not have kind")
            return

        if something["kind"] == "TaskRun":
            self._populate_taskrun(something)
        else:
            logging.debug(f"Skipping item because it has unexpected kind {something['kind']}")
            return

    def _populate_taskrun(self, tr):
        """Load TaskRun."""
        try:
            tr_name = tr["metadata"]["name"]
        except KeyError as e:
            logging.info(f"TaskRun missing name, skipping: {e}, {str(tr)[:200]}")
            self.tr_skips += 1
            return

        try:
            tr_task = tr["metadata"]["labels"]["tekton.dev/pipelineTask"]
        except KeyError as e:
            logging.info(
                f"TaskRun {tr_name} missing task, skipping: {e}"
            )
            self.tr_skips += 1
            return

        try:
            tr_conditions = tr["status"]["conditions"]
        except KeyError as e:
            logging.info(f"TaskRun {tr_name} missing conditions, skipping: {e}")
            self.tr_skips += 1
            return

        tr_condition_ok = False
        for c in tr_conditions:
            if c["type"] == "Succeeded":
                if c["status"] == "True":
                    tr_condition_ok = True
                break
        ###if not tr_condition_ok:
        ###    logging.info(f"TaskRun {tr_name} in wrong condition, skipping: {c}")
        ###    self.tr_skips += 1
        ###    return

        try:
            tr_creationTimestamp = str2date(tr["metadata"]["creationTimestamp"])
            tr_completionTime = str2date(tr["status"]["completionTime"])
            tr_startTime = str2date(tr["status"]["startTime"])
            tr_namespace = tr["metadata"]["namespace"]
        except KeyError as e:
            logging.info(f"TaskRun {tr_name} missing some fields, skipping: {e}")
            self.tr_skips += 1
            return

        self.data_taskruns.append(
            {
                "namespace": tr_namespace,
                "name": tr_name,
                "task": tr_task,
                "condition": tr_condition_ok,
                "pending_duration": (tr_startTime - tr_creationTimestamp).total_seconds(),
                "running_duration": (tr_completionTime - tr_startTime).total_seconds(),
                "duration": (tr_completionTime - tr_creationTimestamp).total_seconds(),
            }
        )

    def _show_multi_arch_tasks(self):
        # All data
        table_header = [
            "namespace",
            "name",
            "task",
            "duration",
            "condition",
        ]
        table = []
        for tr in self.data_taskruns:
            table.append([
                tr["namespace"],
                tr["name"],
                tr["task"],
                tr["duration"],
                tr["condition"],
            ])
        table.sort(key=operator.itemgetter(3))
        print("\nTaskRuns breakdown:\n")
        print(tabulate.tabulate(table, headers=table_header))
        self._dump_as_csv("taskruns-breakdown-all.csv", table, table_header)

        # Per task average
        data = {}
        for tr in self.data_taskruns:
            if not tr["condition"]:
                continue   # skip failed tasks
            if tr["task"] not in data:
                data[tr["task"]] = {
                    "count": 0,
                    "times": [],
                }
            data[tr["task"]]["count"] += 1
            data[tr["task"]]["times"].append(tr["duration"])
        table_header = [
            "task",
            "duration_avg_sec",
            "duration_stdev",
            "duration_samples",
        ]
        table = []
        for t, v in data.items():
            table.append([
                t,
                sum(v["times"]) / v["count"] if v["count"] > 0 else None,
                statistics.stdev(v["times"]) if len(v["times"]) >= 2 else None,
                v["count"],
            ])
        table.sort(key=operator.itemgetter(1))
        print("\nTaskRuns breakdown averages by task (only successfull):\n")
        print(tabulate.tabulate(table, headers=table_header, floatfmt=".0f"))
        self._dump_as_csv("taskruns-breakdown-averages.csv", table, table_header)

    def _dump_as_csv(self, name, table, table_header):
        name_full = os.path.join(self.data_dir, name)
        with open(name_full, "w") as fd:
            writer = csv.writer(fd)
            writer.writerow(table_header)
            for row in table:
                writer.writerow(row)

    def doit(self):
        self._show_multi_arch_tasks()

def doit(args):
    something = Something(
        data_dir=args.data_dir,
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
        help="Directory from where to load YAML data and where to put output SVG",
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
