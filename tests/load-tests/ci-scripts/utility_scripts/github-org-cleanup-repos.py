#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import argparse
import re
import github  # install "PyGithub"
import os


def list_and_delete_repos(
    token: str,
    organization: str,
    regexp: str,
    delete_repos: bool,
) -> None:
    """Lists repositories in a given organization and optionally deletes them.

    Args:
        token: GitHub personal access token.
        organization: GitHub organization name.
        regexp: Regexp repo names have to match to list them.
        delete_repos: Whether to delete repositories.
    """

    g = github.Github(token)
    org = g.get_organization(organization)

    for repo in org.get_repos():
        # Check if repo name matches
        if regexp is not None and not re.fullmatch(regexp, repo.name):
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
            print(repo.name)


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
        "--delete",
        action="store_true",
        help="Delete repositories (dangerous)",
    )
    args = parser.parse_args()

    list_and_delete_repos(args.token, args.organization, args.regexp, args.delete)
