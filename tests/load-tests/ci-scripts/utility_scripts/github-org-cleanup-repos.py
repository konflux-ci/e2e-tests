#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import argparse
import re
import github  # install "PyGithub"
import os
import datetime


def iso_date(s: str) -> datetime.datetime:
    try:
        dt = datetime.datetime.fromisoformat(s)
    except ValueError:
        try:
            dt = datetime.datetime.strptime(s, "%Y-%m-%d")
        except ValueError:
            raise argparse.ArgumentTypeError(f"Not a valid ISO date: '{s}'.")

    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=datetime.timezone.utc)
    return dt


def list_and_delete_repos(
    token: str,
    organization: str,
    regexp: str,
    updated_before: datetime.datetime,
    delete_repos: bool,
) -> None:
    """Lists repositories in a given organization and optionally deletes them.

    Args:
        token: GitHub personal access token.
        organization: GitHub organization name.
        regexp: Regexp repo names have to match to list them.
        updated_before: Filter repos updated before this date.
        delete_repos: Whether to delete repositories.
    """

    g = github.Github(token)
    org = g.get_organization(organization)

    for repo in org.get_repos():
        # Check if repo name matches
        if regexp is not None and not re.fullmatch(regexp, repo.name):
            continue

        # Check if repo was updated before given date
        if updated_before is not None and repo.updated_at >= updated_before:
            continue

        # List or delete the repo
        if delete_repos:
            try:
                repo.delete()
            except Exception as e:
                print(f"{repo.name} - Error deleting repository: {e}")
            finally:
                print(f"{repo.name} - deleted")
        else:
            print(f"{repo.name} updated at {repo.updated_at}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="List and optionally delete GitHub repositories",
    )
    parser.add_argument(
        "--token",
        default=os.environ.get("GITHUB_TOKEN"),
        help="GitHub personal access token",
    )
    parser.add_argument(
        "--organization",
        required=True,
        help="GitHub organization name",
    )
    parser.add_argument(
        "--regexp",
        default=None,
        help="Only list or delete repositories with matching name",
    )
    parser.add_argument(
        "--updated-before",
        type=iso_date,
        default=None,
        help="Only list or delete repositories updated before this date (ISO 8601 format, defaults to UTC if no timezone is provided)",
    )
    parser.add_argument(
        "--delete",
        action="store_true",
        help="Delete repositories (dangerous)",
    )
    args = parser.parse_args()

    list_and_delete_repos(
        args.token, args.organization, args.regexp, args.updated_before, args.delete
    )
