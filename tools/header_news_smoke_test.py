"""Focused UI smoke test for the header version status and VK news block."""

from __future__ import annotations

import argparse
import shutil
import urllib.error
import urllib.request
from urllib.parse import urlsplit

from playwright.sync_api import Error as PlaywrightError
from playwright.sync_api import sync_playwright
from smoke_common import RunningApp, open_loopback_url, require_binary


def install_local_storage(page) -> None:
    page.evaluate("""() => {
      const values = new Map();
      Object.defineProperty(window, 'localStorage', {
        configurable: true,
        value: {
          getItem: key => values.has(String(key)) ? values.get(String(key)) : null,
          setItem: (key, value) => values.set(String(key), String(value)),
          removeItem: key => values.delete(String(key)),
          clear: () => values.clear(),
        },
      });
    }""")


def proxy_request(app, available: bool):
    def handler(route):
        request = route.request
        parsed = urlsplit(request.url)
        if parsed.path == "/api/update-check":
            route.fulfill(
                status=200,
                headers={"Content-Type": "application/json"},
                body='{"currentVersion":"1.1.0","latestVersion":"1.1.0","updateAvailable":false,"checked":true}',
            )
            return
        if parsed.path == "/api/community-status":
            if available:
                body = (
                    '{"available":true,"communityUrl":"https://vk.ru/wall-59626511",'
                    '"latestPostId":62336,"latestPostUrl":"https://vk.ru/wall-59626511_62336",'
                    '"latestPostText":"Тестовая последняя запись сообщества. Проверяем длинный текст и переносы."}'
                )
            else:
                body = (
                    '{"available":false,"communityUrl":"https://vk.ru/wall-59626511"}'
                )
            route.fulfill(
                status=200, headers={"Content-Type": "application/json"}, body=body
            )
            return

        target = (
            app.base_url + parsed.path + (("?" + parsed.query) if parsed.query else "")
        )
        headers = {"Accept": request.headers.get("accept", "*/*")}
        if request.headers.get("content-type"):
            headers["Content-Type"] = request.headers["content-type"]
        upstream = urllib.request.Request(
            target,
            data=request.post_data_buffer,
            headers=headers,
            method=request.method,
        )
        try:
            with open_loopback_url(upstream, timeout=10) as response:
                response_headers = {
                    key: value
                    for key, value in response.headers.items()
                    if key.lower()
                    not in {"content-encoding", "transfer-encoding", "connection"}
                }
                route.fulfill(
                    status=response.status,
                    headers=response_headers,
                    body=response.read(),
                )
        except urllib.error.HTTPError as error:
            route.fulfill(
                status=error.code,
                headers={
                    "Content-Type": error.headers.get("Content-Type", "text/plain")
                },
                body=error.read(),
            )

    return handler


def load_app(page, app, available: bool) -> None:
    page.route("http://iris.test/**", proxy_request(app, available))
    install_local_storage(page)
    with open_loopback_url(app.base_url + "/", timeout=10) as response:
        html = response.read().decode("utf-8")
    html = html.replace("<head>", '<head><base href="http://iris.test/">', 1)
    page.set_content(html, wait_until="networkidle")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    binary = require_binary(args.binary)

    with RunningApp(binary, ["-no-browser"]) as app:
        app.wait_ready()
        with sync_playwright() as playwright:
            try:
                browser = playwright.chromium.launch(headless=True)
            except PlaywrightError:
                executable = (
                    shutil.which("chromium")
                    or shutil.which("chromium-browser")
                    or shutil.which("google-chrome")
                )
                if not executable:
                    raise
                browser = playwright.chromium.launch(
                    headless=True, executable_path=executable
                )

            for width in (320, 375, 640, 1280):
                page = browser.new_page(viewport={"width": width, "height": 1000})
                errors: list[str] = []
                page.on(
                    "pageerror", lambda error, errors=errors: errors.append(str(error))
                )
                load_app(page, app, True)
                page.wait_for_selector(".vk-news-card")
                assert page.locator(".section-tabs-row > #sectionTabs").count() == 1
                assert page.locator(".section-tabs-row > #versionStatus").count() == 1
                assert "home-vk-news" in (
                    page.locator(".home-page > *").last.get_attribute("class") or ""
                )
                assert (
                    page.locator(
                        '.vk-news-card a[href="https://vk.ru/wall-59626511_62336"]'
                    ).count()
                    == 1
                )
                assert (
                    page.locator(
                        ".vk-news-text",
                        has_text="Тестовая последняя запись сообщества.",
                    ).count()
                    == 1
                )
                assert not page.evaluate(
                    "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                )
                assert not errors, errors
                page.close()

            page = browser.new_page(viewport={"width": 375, "height": 900})
            load_app(page, app, False)
            page.wait_for_selector(".vk-news-fallback")
            assert (
                page.locator('.vk-news-fallback img[src="/vk-fallback.svg"]').count()
                == 1
            )
            assert (
                page.locator(".vk-news-fallback", has_text="Новости недоступны").count()
                == 1
            )
            page.close()
            browser.close()

    print("Header/news UI smoke test: PASS")


if __name__ == "__main__":
    main()
