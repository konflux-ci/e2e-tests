#!/usr/bin/env python

import argparse
import collections
import copy
import datetime
import itertools
import json
import logging
import os
import os.path
import re
import sys
import yaml
import time

import matplotlib.pyplot
import matplotlib.colors

import tabulate


def str2date(date_str):
    if isinstance(date_str, datetime.datetime):
        return date_str
    else:
        return datetime.datetime.fromisoformat(date_str.replace("Z", "+00:00"))


class DateTimeEncoder(json.JSONEncoder):
    def default(self, o):
        if isinstance(o, datetime.datetime):
            return o.isoformat()
        return super().default(o)


class DateTimeDecoder(json.JSONDecoder):
    def __init__(self, *args, **kwargs):
        super().__init__(object_hook=self.object_hook, *args, **kwargs)

    def object_hook(self, o):
        ret = {}
        for key, value in o.items():
            if isinstance(value, str):
                try:
                    ret[key] = datetime.datetime.fromisoformat(value)
                except ValueError:
                    ret[key] = value
            else:
                ret[key] = value
        return ret


class Something:
    def __init__(self, data_dir):
        self.data_pipelineruns = {}
        self.data_taskruns = []
        self.data_pods = []
        self.data_taskruns = []
        self.data_dir = data_dir
        self.pr_lanes = []

        self.fig_path = os.path.join(self.data_dir, "output.svg")

        self.pr_count = 0
        self.tr_count = 0
        self.pod_count = 0
        self.pr_skips = 0  # how many PipelineRuns we skipped
        self.tr_skips = 0  # how many TaskRuns we skipped
        self.pod_skips = 0  # how many Pods we skipped
        self.pr_duration = datetime.timedelta(0)  # total time of all PipelineRuns
        self.tr_duration = datetime.timedelta(0)  # total time of all TaskRuns
        self.pr_idle_duration = datetime.timedelta(
            0
        )  # total time in PipelineRuns when no TaskRun was running
        self.pr_conditions = collections.defaultdict(lambda: 0)
        self.tr_conditions = collections.defaultdict(lambda: 0)
        self.tr_statuses = collections.defaultdict(lambda: 0)

        self._populate(self.data_dir)
        self._merge_taskruns()
        self._merge_pods()

    def _merge_taskruns(self):
        for tr in self.data_taskruns:
            if tr["pipelinerun"] not in self.data_pipelineruns:
                logging.info(
                    f"TaskRun {tr['name']} pipelinerun {tr['pipelinerun']} unknown, skipping."
                )
                self.tr_skips += 1
                continue

            if "taskRuns" not in self.data_pipelineruns[tr["pipelinerun"]]:
                self.data_pipelineruns[tr["pipelinerun"]]["taskRuns"] = {}

            if tr["task"] in self.data_pipelineruns[tr["pipelinerun"]]["taskRuns"]:
                logging.info(
                    f"TaskRun {tr['name']} task {tr['task']} already in PipelineRun, strange, skipping."
                )
                self.tr_skips += 1
                continue

            tr_task = tr["task"]
            tr_pipelinerun = tr["pipelinerun"]
            del tr["name"]
            del tr["task"]
            del tr["pipelinerun"]

            self.data_pipelineruns[tr_pipelinerun]["taskRuns"][tr_task] = tr

        self.data_taskruns = []

    def _merge_pods(self):
        for pod in self.data_pods:
            if pod["pipelinerun"] not in self.data_pipelineruns:
                logging.info(
                    f"Pod {pod['name']} pipelinerun {pod['pipelinerun']} unknown, skipping."
                )
                self.pod_skips += 1
                continue

            if pod["task"] not in self.data_pipelineruns[pod["pipelinerun"]]["taskRuns"]:
                logging.info(f"Pod {pod['name']} task {pod['task']} unknown, skipping.")
                self.pod_skips += 1
                continue

            if (
                pod["name"]
                != self.data_pipelineruns[pod["pipelinerun"]]["taskRuns"][pod["task"]]["podName"]
            ):
                logging.info(
                    f"Pod {pod['name']} task labels does not match TaskRun podName, skipping."
                )
                self.pod_skips += 1
                continue

            self.data_pipelineruns[pod["pipelinerun"]]["taskRuns"][pod["task"]]["node_name"] = pod[
                "node_name"
            ]

        self.data_pods = []

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
                print(f"Loaded {datafile} in {(end - start):.2f} seconds")

                if "kind" in data and data["kind"] == "List":
                    if "items" not in data:
                        logging.info(f"Skipping {datafile} as it does not contain items")
                        continue

                    for i in data["items"]:
                        self._populate_add_one(i)
                else:
                    self._populate_add_one(data)

    def _populate_add_one(self, something):
        if "kind" not in something:
            logging.info("Skipping item because it does not have kind")
            return

        if something["kind"] == "PipelineRun":
            self._populate_pipelinerun(something)
        elif something["kind"] == "TaskRun":
            self._populate_taskrun(something)
        elif something["kind"] == "Pod":
            self._populate_pod(something)
        else:
            logging.info(f"Skipping item because it has unexpeted kind {something['kind']}")
            return

    def _populate_pipelinerun(self, pr):
        """Load PipelineRun."""
        try:
            pr_name = pr["metadata"]["name"]
        except KeyError as e:
            logging.info(
                f"PipelineRun '{str(pr)[:200]}...' missing name, skipping: {e}"
            )
            self.pr_skips += 1
            return

        try:
            pr_conditions = pr["status"]["conditions"]
        except KeyError as e:
            logging.info(f"PipelineRun {pr_name} missing conditions, skipping: {e}")
            self.pr_conditions["Missing conditions"] += 1
            self.pr_skips += 1
            return

        pr_condition_ok = False
        for c in pr_conditions:
            if c["type"] == "Succeeded":
                message = re.sub(
                    r'PipelineRun "[a-z0-9-]+"', 'PipelineRun "REPLACED"', c["message"]
                )
                self.pr_conditions[message] += 1

                if c["status"] == "True":
                    pr_condition_ok = True
                break
        else:
            self.pr_conditions["Missing type"] += 1
        ###if not pr_condition_ok:
        ###    logging.info(
        ###        f"PipelineRun {pr_name} is not in right condition, skipping: {pr_conditions}"
        ###    )
        ###    self.pr_skips += 1
        ###    return

        try:
            pr_creationTimestamp = str2date(pr["metadata"]["creationTimestamp"])
            pr_completionTime = str2date(pr["status"]["completionTime"])
            pr_startTime = str2date(pr["status"]["startTime"])
        except KeyError as e:
            logging.info(f"PipelineRun {pr_name} missing some fields, skipping: {e}")
            self.pr_skips += 1
            return

        self.data_pipelineruns[pr_name] = {
            "creationTimestamp": pr_creationTimestamp,
            "completionTime": pr_completionTime,
            "startTime": pr_startTime,
            "condition": pr_condition_ok,
        }

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
            tr_pipelinerun = tr["metadata"]["labels"]["tekton.dev/pipelineRun"]
        except KeyError as e:
            logging.info(
                f"TaskRun {tr_name} missing task or pipelinerun, skipping: {e}"
            )
            self.tr_skips += 1
            return

        try:
            tr_conditions = tr["status"]["conditions"]
        except KeyError as e:
            logging.info(f"TaskRun {tr_name} missing conditions, skipping: {e}")
            self.tr_conditions["Missing conditions"] += 1
            self.tr_skips += 1
            return

        tr_condition_ok = False
        for c in tr_conditions:
            if c["type"] == "Succeeded":
                message = re.sub(
                    r'TaskRun "[a-z0-9-]+"', 'TaskRun "REPLACED"', c["message"]
                )
                self.tr_conditions[message] += 1
                if c["status"] == "True":
                    tr_condition_ok = True
                break
        else:
            self.tr_conditions["Missing type"] += 1
        ###if not tr_condition_ok:
        ###    logging.info(f"TaskRun {tr_name} in wrong condition, skipping: {c}")
        ###    self.tr_skips += 1
        ###    return

        try:
            self.tr_statuses[tr["spec"]["statusMessage"]] += 1
        except KeyError as e:
            self.tr_statuses["Missing spec.statusMessage"] += 1

        try:
            tr_creationTimestamp = str2date(tr["metadata"]["creationTimestamp"])
            tr_completionTime = str2date(tr["status"]["completionTime"])
            tr_startTime = str2date(tr["status"]["startTime"])
            tr_podName = tr["status"]["podName"]
            tr_namespace = tr["metadata"]["namespace"]
        except KeyError as e:
            logging.info(f"TaskRun {tr_name} missing some fields, skipping: {e}")
            self.tr_skips += 1
            return

        self.data_taskruns.append(
            {
                "name": tr_name,
                "task": tr_task,
                "pipelinerun": tr_pipelinerun,
                "creationTimestamp": tr_creationTimestamp,
                "completionTime": tr_completionTime,
                "startTime": tr_startTime,
                "podName": tr_podName,
                "namespace": tr_namespace,
                "condition": tr_condition_ok,
            }
        )

    def _populate_pod(self, pod):
        """Load Pod."""
        try:
            pod_name = pod["metadata"]["name"]
        except KeyError as e:
            logging.info(f"Pod missing name, skipping: {e}, {str(pod)[:200]}")
            self.pod_skips += 1
            return

        try:
            pod_pipelinerun = pod["metadata"]["labels"]["tekton.dev/pipelineRun"]
            pod_task = pod["metadata"]["labels"]["tekton.dev/pipelineTask"]
        except KeyError as e:
            logging.info(f"Pod {pod_name} missing pipelinerun or task, skipping: {e}")
            self.pod_skips += 1
            return

        try:
            pod_node_name = pod["spec"]["nodeName"]
        except KeyError as e:
            logging.info(f"Pod {pod_name} missing node name filed, skipping: {e}")
            self.pod_skips += 1
            return

        self.data_pods.append(
            {
                "name": pod_name,
                "pipelinerun": pod_pipelinerun,
                "task": pod_task,
                "node_name": pod_node_name,
            }
        )

    def _dump_json(self, data, path):
        with open(path, "w") as fp:
            json.dump(data, fp, cls=DateTimeEncoder, sort_keys=True, indent=4)

    def _load_json(self, path):
        with open(path, "r") as fp:
            return json.load(fp, cls=DateTimeDecoder)

    def _compute_lanes(self):
        """
        Based on loaded PipelineRun and TaskRun data, compute lanes for a graph.

        Visualizing overlapping intervals:
        https://www.nxn.se/valent/visualizing-overlapping-intervals
        """

        def fits_into_lane(entity, lane):
            start = "creationTimestamp"
            end = "completionTime"
            logging.debug(
                f"Checking if entity ({entity[start]} - {entity[end]}) fits into lane with {len(lane)} members"
            )
            for member in lane:
                if (
                    member[start] <= entity[start] <= member[end]
                    or member[start] <= entity[end] <= member[end]
                    or entity[start] <= member[start] <= entity[end]
                ):
                    logging.debug(
                        f"Entity ({entity[start]} - {entity[end]}) does not fit because of lane member ({member[start]} - {member[end]})"
                    )
                    return False
            logging.debug(f"Entity ({entity[start]} - {entity[end]}) fits")
            return True

        for pr_name, pr_times in self.data_pipelineruns.items():
            pr = copy.deepcopy(pr_times)
            pr["name"] = pr_name

            # How do we organize it's TRs?
            tr_lanes = []

            for tr_name, tr_times in pr["taskRuns"].items():
                tr = copy.deepcopy(tr_times)
                tr["name"] = tr_name
                for lane in tr_lanes:
                    # Is there a lane without colnflict?
                    if fits_into_lane(tr, lane):
                        lane.append(tr)
                        break
                else:
                    # Adding new lane
                    tr_lanes.append([tr])

            pr["tr_lanes"] = tr_lanes
            del pr["taskRuns"]

            # Where it fits in a list of PRs?
            for lane in self.pr_lanes:
                # Is there a lane without colnflict?
                if fits_into_lane(pr, lane):
                    lane.append(pr)
                    break
            else:
                # Adding new lane
                self.pr_lanes.append([pr])

        self.pr_lanes.sort(key=lambda k: min([i["creationTimestamp"] for i in k]))

    def _compute_times(self):
        """
        Based on computed lanes, compute some statistical measures for the run.
        """

        def add_time_interval(existing, new):
            """
            Merge the new interval with first overlaping existing interval or add new one to list.
            """
            start = "creationTimestamp"
            end = "completionTime"
            processed = False

            for t in existing:
                # If both ends are inside of existing interval, we ignore it
                if t[start] <= new[start] <= t[end] and t[start] <= new[end] <= t[end]:
                    processed = True
                    continue

                # If start is inside existing interval, but end is outside of it,
                # we extend existing interval
                if t[start] <= new[start] <= t[end]:
                    if new[end] > t[end]:
                        t[end] = new[end]
                        processed = True
                        continue

                # If end is inside existing interval, but start is outside of it,
                # we extend existing interval
                if t[start] <= new[end] <= t[end]:
                    if new[start] < t[start]:
                        t[start] = new[start]
                        processed = True
                        continue

            if not processed:
                existing.append(new)

        start = "creationTimestamp"
        end = "completionTime"

        self.pr_count = len(self.data_pipelineruns)
        self.tr_count = sum([len(i["taskRuns"]) for i in self.data_pipelineruns.values()])
        self.pod_count = sum(
            [
                len([ii for ii in i["taskRuns"].values() if "node_name" in ii])
                for i in self.data_pipelineruns.values()
            ]
        )

        for pr_name, pr_times in self.data_pipelineruns.items():
            pr_duration = pr_times[end] - pr_times[start]
            self.pr_duration += pr_duration

            # Combine TaskRuns so they do not overlap
            trs = []
            for tr_name, tr_times in pr_times["taskRuns"].items():
                self.tr_duration += tr_times[end] - tr_times[start]
                add_time_interval(trs, tr_times)

            # Combine new intervals so they do not overlap
            trs_no_overlap = []
            for interval in trs:
                add_time_interval(trs_no_overlap, interval)

            tr_simple_duration = datetime.timedelta(0)
            for interval in trs_no_overlap:
                tr_simple_duration += interval[end] - interval[start]

            self.pr_idle_duration += pr_duration - tr_simple_duration

        print()
        print(
            f"During processing, we skipped {self.pr_skips} PipelineRuns, {self.tr_skips} TaskRuns and {self.pod_skips} Pods."
        )
        print(
            f"There was {self.pr_count} PipelineRuns and {self.tr_count} TaskRuns and {self.pod_count} Pods."
        )
        print(
            f"In total PipelineRuns took {self.pr_duration} and TaskRuns took {self.tr_duration}, PipelineRuns were idle for {self.pr_idle_duration}"
        )
        pr_duration_avg = (
            (self.pr_duration / self.pr_count).total_seconds()
            if self.pr_count != 0
            else None
        )
        tr_duration_avg = (
            (self.tr_duration / self.tr_count).total_seconds()
            if self.tr_count != 0
            else None
        )
        pr_idle_duration_avg = (
            (self.pr_idle_duration / self.pr_count).total_seconds()
            if self.pr_count != 0
            else None
        )
        print(
            f"In average PipelineRuns took {pr_duration_avg} and TaskRuns took {tr_duration_avg}, PipelineRuns were idle for {pr_idle_duration_avg} seconds"
        )

    def _compute_nodes(self):
        """
        Based on loaded data, compute how many TaskRuns run on what nodes.
        """
        nodes = {}
        for pr_name, pr_data in self.data_pipelineruns.items():
            for tr_name, tr_data in pr_data["taskRuns"].items():
                try:
                    node_name = tr_data["node_name"]
                except KeyError:
                    logging.info(
                        f"TaskRun {tr_name} missing node_name field, skipping."
                    )
                    continue
                if node_name not in nodes:
                    nodes[node_name] = 1
                else:
                    nodes[node_name] += 1

        print("\nNumber of TaskRuns per node:")
        for node, count in sorted(nodes.items(), key=lambda item: item[1]):
            print(f"    {node}: {count}")

    def _show_pr_tr_nodes(self):
        """
        Show which PipelineRuns and TaskRuns were running on which node
        """
        table = []
        stats = {}
        for pr_name, pr_data in self.data_pipelineruns.items():
            pr_tr_nodes = {}
            for tr_name, tr_data in pr_data["taskRuns"].items():
                try:
                    node_name = tr_data["node_name"]
                except KeyError:
                    logging.info(
                        f"TaskRun {tr_name} missing node_name field, skipping."
                    )
                    continue
                table.append([pr_name, tr_name, node_name])
                pr_tr_nodes[tr_name] = node_name

            # Compile stats of whoch tasks ran on same node (in one PR) most often
            for tr1 in pr_tr_nodes:
                for tr2 in pr_tr_nodes:
                    if pr_tr_nodes[tr1] == pr_tr_nodes[tr2]:
                        if tr1 not in stats:
                            stats[tr1] = {}
                        if tr2 not in stats[tr1]:
                            stats[tr1][tr2] = 0
                        stats[tr1][tr2] += 1

        # Transform the stats to the form tabulate can handle
        table_keys = sorted(list(stats.keys()))
        table_data = []
        for tr1 in table_keys:
            table_row = []
            for tr2 in table_keys:
                try:
                    table_row.append(stats[tr1][tr2])
                except KeyError as e:
                    logging.warning(f"Failed to transform stats: {e}")
            table_data.append([tr1] + table_row)

        # print("\nWhich PipelineRuns and TaskRuns ran on which node:")
        # print(tabulate.tabulate(
        #     table,
        #     headers=["PipelineRun", "TaskRun", "Node"],
        # ))

        print(
            "\nWhich TaskRuns inside of one PipelineRun were sharing node most often:"
        )
        print(
            tabulate.tabulate(
                table_data,
                headers=["TaskRun"] + table_keys,
            )
        )

    def _show_pr_tr_conditions(self):
        print("\nPipelineRuns conditions frequency")
        print(
            tabulate.tabulate(
                self.pr_conditions.items(),
                headers=["Condition message", "Count"],
            )
        )
        print("\nTaskRuns conditions frequency")
        print(
            tabulate.tabulate(
                self.tr_conditions.items(),
                headers=["Condition message", "Count"],
            )
        )
        print("\nTaskRuns status messages frequency")
        print(
            tabulate.tabulate(
                self.tr_statuses.items(),
                headers=["Status message", "Count"],
            )
        )

    def _plot_graph(self):
        """
        Based on computed lanes, plot a graph.

        Horizontal bar plot with gaps:
        https://matplotlib.org/stable/gallery/lines_bars_and_markers/broken_barh.html#sphx-glr-gallery-lines-bars-and-markers-broken-barh-py
        """

        def entity_to_coords(entity):
            start = "creationTimestamp"
            end = "completionTime"
            return (
                entity[start].timestamp(),
                (entity[end] - entity[start]).total_seconds(),
            )

        def get_min(entity, current_min):
            start = "creationTimestamp"
            return min(entity[start].timestamp(), current_min)

        def get_max(entity, current_max):
            end = "completionTime"
            return max(entity[end].timestamp(), current_max)

        size = max(5, self.pr_count / 2)
        size = min(size, 100)
        fig, ax = matplotlib.pyplot.subplots(figsize=(size, size))

        fig_x_min = sys.maxsize
        fig_x_max = 0

        tr_height = 10
        fig_pr_y_pos = 0
        colors = sorted(
            matplotlib.colors.TABLEAU_COLORS,
            key=lambda c: tuple(
                matplotlib.colors.rgb_to_hsv(matplotlib.colors.to_rgb(c))
            ),
        )
        colors = [
            "tab:gray",
            "tab:brown",
            "tab:orange",
            "tab:olive",
            "tab:green",
            "tab:cyan",
            "tab:blue",
            "tab:purple",
            "tab:pink",
            "tab:red",
        ]

        # Get graph x range
        for pr_lane in self.pr_lanes:
            for pr in pr_lane:
                fig_x_min = get_min(pr, fig_x_min)
                fig_x_max = get_max(pr, fig_x_max)
        fig_x_repair = -1 * fig_x_min   # Repair all x coords by this value

        for pr_lane in self.pr_lanes:
            for pr in pr_lane:
                pr_coords = entity_to_coords(pr)
                if pr["condition"]:
                    ax.broken_barh(
                        [[pr_coords[0] - 1 + fig_x_repair, pr_coords[1] + 2]],
                        (fig_pr_y_pos + 1, tr_height * len(pr["tr_lanes"]) - 2),
                        facecolors="#eeeeee",
                        edgecolor="black",
                    )
                else:
                    ax.broken_barh(
                        [[pr_coords[0] - 1 + fig_x_repair, pr_coords[1] + 2]],
                        (fig_pr_y_pos + 1, tr_height * len(pr["tr_lanes"]) - 2),
                        facecolors="#eeeeee",
                        edgecolor="black",
                        hatch="\\",
                    )
                txt = ax.text(
                    x=pr_coords[0] + 4 + fig_x_repair,
                    y=fig_pr_y_pos + tr_height * len(pr["tr_lanes"]) - tr_height * 0.5,
                    s=pr["name"],
                    fontsize=8,
                    horizontalalignment="left",
                    verticalalignment="center",
                    color="#222222",
                    rotation=-10,
                    rotation_mode="anchor",
                )
                ax.add_artist(txt)
                names_sorted = []
                for tr_lane in pr["tr_lanes"]:
                    for tr in tr_lane:
                        names_sorted.append(tr["name"])
                names_sorted.sort()
                names_to_colors = dict(zip(names_sorted, itertools.cycle(colors)))
                fig_tr_y_pos = fig_pr_y_pos
                for tr_lane in pr["tr_lanes"]:
                    for tr in tr_lane:
                        tr_coords = entity_to_coords(tr)
                        if tr["condition"]:
                            ax.broken_barh(
                                [[tr_coords[0] + fig_x_repair, tr_coords[1]]],
                                (fig_tr_y_pos + 2, tr_height - 4),
                                facecolors=names_to_colors[tr["name"]],
                            )
                        else:
                            ax.broken_barh(
                                [[tr_coords[0] + fig_x_repair, tr_coords[1]]],
                                (fig_tr_y_pos + 2, tr_height - 4),
                                facecolors=names_to_colors[tr["name"]],
                                hatch="///",
                            )
                        txt = ax.text(
                            x=tr_coords[0] + tr_coords[1] / 2 + fig_x_repair,
                            y=fig_tr_y_pos + tr_height / 2,
                            s=tr["name"],
                            fontsize=6,
                            horizontalalignment="left",
                            verticalalignment="center",
                            color="darkgray",
                            rotation=30,
                            rotation_mode="anchor",
                        )
                        ax.add_artist(txt)
                    fig_tr_y_pos += tr_height
            fig_pr_y_pos += tr_height * max([len(pr["tr_lanes"]) for pr in pr_lane])
        ax.set_ylim(0, fig_pr_y_pos)
        ax.set_xlim(0, fig_x_max + fig_x_repair + 60)
        ax.set_xlabel("timestamps [s]")
        ax.grid(True)

        # matplotlib.pyplot.show()
        matplotlib.pyplot.savefig(self.fig_path, bbox_inches="tight")

    def doit(self):
        self._compute_lanes()
        self._compute_times()
        self._plot_graph()
        self._show_pr_tr_conditions()
        #self._show_pr_tr_nodes()
        self._compute_nodes()


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
