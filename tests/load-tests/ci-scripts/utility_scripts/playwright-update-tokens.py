#!/usr/bin/env python3

# Docs:
#     This script uses credentials (username and password) from users.json
#     to login to console.redhat.com and generate new offline token. It
#     saves updated content to users-new.json.
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
#     ci-scripts/utility_scripts/playwright-update-tokens.py
#
#     To sort final file I have used:
#     cat users-new.json | jq '. | sort_by(.username[11:] | tonumber)'
#
#     Note I was not able to run with more threads/processes as I was
#     getting "Access Denied" errors. I guess it was some rate limiting.

import playwright.sync_api
import time
import json
import multiprocessing
import queue
import os.path
import sys
import traceback

sys.path.append(os.path.dirname(os.path.realpath(__file__)))
import playwright_lib

PLAYWRIGHT_HEADLESS = True
PLAYWRIGHT_VIDEO_DIR = "videos/"


def workload(user):
    try:
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

            # Go to OpenShift Token page
            page.goto("https://console.redhat.com/openshift/token")
            page.wait_for_url("https://console.redhat.com/openshift/token**")

            # Confirm I want to load a token
            page.locator('a:has-text("use API tokens to authenticate")').click()

            # Wait for token
            button_token = page.locator('//button[text()="Load token"]')
            if button_token.is_visible():
                button_token.click()
            attempt = 1
            attempt_max = 100
            while True:
                input_token = page.locator(
                    '//input[@aria-label="Copyable token" and not(contains(@value, "ocm login "))]'
                )
                input_token_value = input_token.get_attribute("value")
                # Token value is populated assynchronously, so call it ready once
                # it is longer than string "" or "null"
                if len(input_token_value) > 10:
                    break
                if attempt > attempt_max:
                    input_token_value = "Failed"
                    break
                attempt += 1
                time.sleep(1)
            print(f"Token for user {username}: {input_token_value}")

            page.close()
            browser.close()

            user["token"] = input_token_value
            return user

    except Exception as e:
        print(f"[ERROR] Failed while processing {user['username']}")
        traceback.print_exc()
        raise


def process_it(output_queue, user):
    try:
        output_queue.put({"result": workload(user)})
    except Exception as e:
        output_queue.put({"exception": e})


def main():

    with open("users.json", "r") as fd:
        users = json.load(fd)

    users_new = []
    users_allowlist = []  # keep empty to allow all

    for user in users:
        if users_allowlist != [] and user["username"] not in users_allowlist:
            print(f"Skipping user {user['username']} as it is not in allow list")
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
                users_new.append(output["result"])
                print(f"Completed processing {user['username']}")
            elif "exception" in output:
                print(f"Failed processing {user['username']}: {output['exception']}")
            else:
                print("Some crazy error")

    with open("users-new.json", "w") as fd:
        print(f"Dumping {len(users_new)} users")
        users = json.dump(users_new, fd)


if __name__ == "__main__":
    main()
