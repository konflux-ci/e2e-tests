#!/usr/bin/env python3

# Docs:
#     This script uses credentials (username and password) from users.json
#     to login to console.dev.redhat.com and join waitlist for Stage Konflux.
#
# Setup:
#     python -m venv venv
#     source venv/bin/activate
#     pip install playwright
#     playwright install   # download Chromium browser
#
# Running:
#     Consider PLAYWRIGHT_HEADLESS = True so it runs in background
#
#     ci-scripts/utility_scripts/playwright-join-waitlist.py
#
#     Note I was not able to run with bigger concurrency as I was
#     getting "Access Denied" errors. I guess it was some rate limiting.

import playwright.sync_api
import json
import multiprocessing
import queue
import os.path
import sys

sys.path.append(os.path.dirname(os.path.realpath(__file__)))
import playwright_lib

PLAYWRIGHT_HEADLESS = False
PLAYWRIGHT_VIDEO_DIR = "videos/"


def workload(user):
    username = user["username"].replace("-", "_")
    password = user["password"]

    with playwright.sync_api.sync_playwright() as p:
        browser = p.chromium.launch(
            headless=PLAYWRIGHT_HEADLESS,
        )
        context = browser.new_context(
            record_video_dir=PLAYWRIGHT_VIDEO_DIR,
        )
        page = context.new_page()

        playwright_lib.goto_login_and_accept_cookies(page)

        playwright_lib.form_login(page, username, password)

        # Go to Konflux
        page.goto("https://console.dev.redhat.com/preview/application-pipeline")

        # Accept terms and conditions if this appears
        try:
            page.wait_for_selector(
                '//h1[text()="We need a little more information"]', timeout=15
            )
        except playwright.sync_api.TimeoutError as e:
            pass
        else:
            terms_checkbox = page.locator(
                '//input[contains(@id, "user.attributes.tcacc-SSO/developersPortalSubscriptionCreation/")]'
            )
            terms_checkbox.click()
            submit_button = page.locator('//button[@id="regform-submit"]')
            submit_button.click()

        # Clisk join waitlist button
        page.wait_for_selector('//h1[contains(text(), "Get started with")]')
        join_button = page.locator('//button[text()="Join the waitlist"]')
        join_button.click()
        page.wait_for_selector(
            '//h4[contains(text(), "We have received your request")]'
        )


def process_it(output_queue, user):
    try:
        output_queue.put({"result": workload(user)})
    except Exception as e:
        output_queue.put({"exception": e})


def main():

    with open("users.json", "r") as fd:
        users = json.load(fd)

    users_allowlist = []  # keep empty to allow all

    for user in users:
        if users_allowlist is [] or user["username"] in users_allowlist:
            continue
        result_queue = multiprocessing.Queue()
        process = multiprocessing.Process(target=process_it, args=(result_queue, user))
        process.start()
        try:
            output = result_queue.get(timeout=100)
        except queue.Empty:
            process.terminate()
            print(f"Timeout processing {user['username']}")
        else:
            if "result" in output:
                print(f"Completed processing {user['username']}")
            elif "exception" in output:
                print(f"Failed processing {user['username']}: {output['exception']}")
            else:
                print("Some crazy error")


if __name__ == "__main__":
    main()
