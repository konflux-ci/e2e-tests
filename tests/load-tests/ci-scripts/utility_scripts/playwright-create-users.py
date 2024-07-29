#!/usr/bin/env python3

# Docs:
#     This script creates new Red Hat users and stores username and password
#     to users-new.json.
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
#     ci-scripts/utility_scripts/playwright-create-users.py
#
#     To sort final file I have used:
#     cat users-new.json | jq '. | sort_by(.username[11:] | tonumber)'
#
#     Note I was not able to run with higher concurrency as I was
#     getting "Access Denied" errors. I guess it was some rate limiting.

import playwright.sync_api
import email.parser
import email.policy
import json
import subprocess
import time
import uuid
import multiprocessing
import queue
import os.path
import sys

sys.path.append(os.path.dirname(os.path.realpath(__file__)))
import playwright_lib

PLAYWRIGHT_HEADLESS = False
PLAYWRIGHT_VIDEO_DIR = "videos/"


def get_verification_link(user_email):
    """
    This uses mine specific mail setup and will not probably work for
    anybody else. If you are in that case sitoation, yo need to click all
    the verification emails manually or automate it to suit your setup.
    """
    attempt = 1
    attempt_max = 3
    attempt_sleep = 10
    while True:
        if attempt > attempt_max:
            raise Exception(f"Out of tries to get verification email for {user_email}")

        try:
            fetchmail = subprocess.run(
                ["fetchmail"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT
            )
            if fetchmail.returncode not in (
                0,
                1,
            ):  # 0 = emails downloaded, 1 = no new emails
                raise Exception(
                    f"Running 'fetchmail' failed with {fetchmail.returncode}: {fetchmail.stdout}"
                )

            notmuch_search = subprocess.run(
                [
                    "notmuch",
                    "search",
                    "--output=files",
                    "--format=json",
                    "--sort=newest-first",
                    f"to:'{user_email}' AND subject:'Verify email for Red Hat account' AND date:'1hour..now'",
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            if notmuch_search.returncode != 0:
                raise Exception(
                    f"Running 'notmuch search ...' failed with {notmuch_search.returncode}: {notmuch_search.stderr}"
                )

            message_files_json = notmuch_search.stdout
            message_files = json.loads(message_files_json)
            if len(message_files) != 1:
                raise Exception(
                    f"Command 'notmuch search ...' returned unexpected number of files: {message_files}"
                )

            if not os.path.isfile(message_files[0]):
                raise Exception("File {message_files[0]} is missing")
        except Exception as e:
            print(f"Getting mail failed: {e}")
            attempt += 1
            time.sleep(attempt_sleep)
        else:
            break

    with open(message_files[0], "r") as fp:
        msg = email.parser.Parser(policy=email.policy.default).parse(fp)
    simplest = msg.get_body(preferencelist=("plain"))
    for line in simplest.get_content().splitlines():
        line = line.strip()
        if line.startswith(
            "https://sso.redhat.com/auth/realms/redhat-external/login-actions/action-token?key="
        ):
            return line


def click_and_wait_hard(
    page, clickable, verifier, timeout_ms_start=1000, timeout_ms_max=30000
):
    """
    Try to click at given thingy (probably link) and wait for another
    thingy to appear, try it multiple times untill it works
    """
    timeout = timeout_ms_start
    while True:
        clickable_locator = page.locator(clickable)
        clickable_locator.click()
        try:
            page.wait_for_selector(verifier, timeout=timeout)
        except playwright.sync_api.TimeoutError as e:
            if timeout >= timeout_ms_max:
                raise  # giving up
            timeout *= 2
        else:
            break  # success


def generate_password():
    """
    Keep generating password until it contains both digit and letter.
    Given special char ("-") is granted, this will meet password requirements.
    """
    while True:
        password = str(uuid.uuid4())
        contains_digit = any(char.isdigit() for char in password)
        contains_alpha = any(char.isalpha() for char in password)
        if contains_digit and contains_alpha:
            return password


def workload(user):
    username = user["username"]
    email = f"jhutar+{username}@redhat.com"
    password = generate_password()

    with playwright.sync_api.sync_playwright() as p:
        browser = p.chromium.launch(
            headless=PLAYWRIGHT_HEADLESS,
        )
        context = browser.new_context(
            record_video_dir=PLAYWRIGHT_VIDEO_DIR,
        )
        page = context.new_page()

        playwright_lib.goto_login_and_accept_cookies(page)

        # Go to registration form
        click_and_wait_hard(
            page,
            '//a[@id="rh-login-registration-link"]',
            '//h1[text()="Register for a Red Hat account"]',
        )

        # Fill the form
        user_input = page.locator('//input[@id="username"]')
        user_input.fill(username)
        pass_input = page.locator('//input[@id="password"]')
        pass_input.fill(password)
        first_input = page.locator('//input[@id="firstName"]')
        first_input.fill("Performance")
        last_input = page.locator('//input[@id="lastName"]')
        last_input.fill("Testing")
        email_input = page.locator('//input[@id="email"]')
        email_input.fill(email)
        terms_checkbox = page.locator(
            '//input[contains(@id, "user.attributes.tcacc-SSO/ssoSignIn/")]'
        )
        terms_checkbox.click()
        submit_button = page.locator('//input[@id="regform-submit"]')
        submit_button.click()
        page.wait_for_selector('//h1[text()="Email address verification"]')

        # Verify account
        verification_link = get_verification_link(email)
        page.goto(verification_link)
        page.wait_for_selector(
            f'//h1[text()="Confirm validity of e-mail address {email}."]'
        )
        finish_button = page.locator('//a[text()="Finish"]')
        finish_button.click()
        page.wait_for_selector('//h1[text()="Your email address has been verified"]')

        page.close()
        browser.close()

        return user


def process_it(output_queue, user):
    try:
        output_queue.put({"result": workload(user)})
    except Exception as e:
        output_queue.put({"exception": e})


def user_generator(i):
    return {
        "username": f"test_rhtap_{i}",
        "password": "",
        "token": "",
        "ssourl": "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token",
        "apiurl": "https://api-toolchain-host-operator.apps.stone-stg-host.qc0p.p1.openshiftapps.com",
        "verified": True,
    }


def main():
    users_new = []
    for user in [user_generator(i) for i in range(101, 201)]:
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

    for user in users_new:
        user["username"] = user["username"].replace("_", "-")
    print(users_new)
    with open("users-new.json", "w") as fd:
        print(f"Dumping {len(users_new)} users")
        json.dump(users_new, fd)


if __name__ == "__main__":
    main()
