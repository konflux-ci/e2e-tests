#!/usr/bin/env python
# -*- coding: UTF-8 -*-

import time


def goto_login_and_accept_cookies(page):
    """Open a login page and accept cookies dialog"""
    page.goto("https://console.redhat.com")
    page.wait_for_url("https://sso.redhat.com/**")

    # Accept cookies
    cookies_iframe = page.frame_locator('iframe[name="trustarc_cm"]')
    cookies_button = cookies_iframe.get_by_role(
        "button", name="Agree and proceed with standard settings"
    )
    if cookies_button.is_visible():
        cookies_button.click()
    else:
        print("Cookies button not found or already clicked.")


def form_login(page, username, password):
    """Wait for login form and use it"""
    page.wait_for_selector('//h1[text()="Log in to your Red Hat account"]')
    input_user = page.locator('//input[@id="username-verification"]')
    time.sleep(1)
    input_user.fill(username)
    button_next = page.locator('//button[@id="login-show-step2"]')
    button_next.click()
    button_next.wait_for(state="hidden")
    input_pass = page.locator('//input[@id="password"]')
    input_pass.wait_for(state="visible")
    input_pass.fill(password)
    page.locator('//button[@id="rh-password-verification-submit-button"]').click()
    page.wait_for_url("https://console.redhat.com/**")
    page.wait_for_selector('//h2[text()="Welcome to your Hybrid Cloud Console."]')

