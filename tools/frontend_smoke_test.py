"""Deterministic Chromium smoke test for the embedded desktop frontend."""

from __future__ import annotations

import argparse
import json
import shutil
import threading
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import Error as PlaywrightError
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[1]
WEB_ROOT = ROOT / "web"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def profile() -> dict[str, object]:
    return {
        "schemaVersion": 1,
        "migrated": True,
        "settings": {"server": "kiss", "theme": "dark", "view": "list"},
        "itemFilters": {},
        "monsterFilters": {},
        "favorites": [],
        "history": [],
        "recentlyViewed": [],
    }


META = {
    "title": "Iris Online Database",
    "servers": [
        {
            "key": "kiss",
            "name": "Iris Kiss Kiss",
            "directDropsUpdatedAt": "2026-08-12",
            "dropListsUpdatedAt": "2026-08-12",
            "worldDropsUpdatedAt": "2026-08-12",
        },
        {"key": "original", "name": "The Original"},
    ],
    "effectSpecs": {},
}

MONSTER = {
    "monster": {
        "id": 42,
        "name": "Тестовый хранитель",
        "category": "Монстр",
        "typeName": "Босс",
        "level": 70,
        "aggressive": True,
        "hp": 1000,
        "note": "Детерминированная запись для frontend smoke test.",
    },
    "slots": [
        {
            "choices": [
                {
                    "items": [
                        {
                            "itemId": 1001,
                            "item": "Серебряный глаз",
                            "baseAttemptChance": 70,
                        },
                        {
                            "itemId": 1002,
                            "item": "Агатовое сердце",
                            "baseAttemptChance": 33.3334,
                        },
                    ]
                }
            ]
        }
    ],
    "worldRuleCount": 0,
}

CHEST_ITEM = {
    "item": {
        "id": 2001,
        "name": "Тестовая шкатулка",
        "category": "Шкатулки",
        "typeLine": "Шкатулка",
        "sellType": 0,
    },
    "bonuses": [],
    "drops": [],
    "chest": {
        "drawCount": 1,
        "items": [
            {
                "itemId": 2002,
                "item": "Тестовая руна",
                "itemKnown": True,
                "chanceKnown": True,
                "chance": 25,
                "variants": [],
            }
        ],
    },
}

RUNE_ITEM = {
    "item": {
        "id": 2002,
        "name": "Тестовая руна",
        "category": "Руны",
        "typeLine": "Руна",
        "sellType": 0,
    },
    "bonuses": [],
    "drops": [],
}


class FixtureState:
    def __init__(self) -> None:
        self.profile = profile()
        self.community_failures = False


class FixtureServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, state: FixtureState) -> None:
        super().__init__(("127.0.0.1", 0), FixtureHandler)
        self.state = state


class FixtureHandler(BaseHTTPRequestHandler):
    server: FixtureServer

    def log_message(self, _format: str, *_args: object) -> None:
        return

    def send_json(self, value: object, status: HTTPStatus = HTTPStatus.OK) -> None:
        body = json.dumps(value, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler contract
        parsed = urlsplit(self.path)
        if parsed.path == "/api/user-data":
            self.send_json(self.server.state.profile)
            return
        if parsed.path == "/api/meta":
            self.send_json(META)
            return
        if parsed.path == "/api/update-check":
            self.send_json(
                {
                    "currentVersion": "2.0.0",
                    "latestVersion": "2.0.0",
                    "updateAvailable": False,
                    "checked": True,
                }
            )
            return
        if parsed.path == "/api/community-status":
            if self.server.state.community_failures:
                self.send_json(
                    {
                        "available": True,
                        "stale": True,
                        "communityUrl": "https://vk.ru/wall-59626511",
                        "latestPostId": 62337,
                        "latestPostUrl": "https://vk.ru/wall-59626511_62337",
                        "latestPostText": "Новый экспериментальный режим Vulkan 🧪.",
                        "sourceUpdatedAt": "2026-08-12T16:10:24Z",
                    }
                )
                return
            self.send_json(
                {
                    "available": True,
                    "communityUrl": "https://vk.ru/wall-59626511",
                    "latestPostId": 62337,
                    "latestPostUrl": "https://vk.ru/wall-59626511_62337",
                    "latestPostText": (
                        "Новый экспериментальный режим Vulkan 🧪. "
                        "Текст новости безопасно экранируется: <img src=x onerror=alert(1)>."
                    ),
                    "publishedAt": "2026-08-12T16:10:24Z",
                    "sourceUpdatedAt": "2026-08-12T16:10:24Z",
                }
            )
            return
        if parsed.path == "/api/monsters/42":
            self.send_json(MONSTER)
            return
        if parsed.path == "/api/items/2001":
            self.send_json(CHEST_ITEM)
            return
        if parsed.path == "/api/items/2002":
            self.send_json(RUNE_ITEM)
            return
        self.serve_asset(parsed.path)

    def do_PUT(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler contract
        if urlsplit(self.path).path != "/api/user-data":
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        length = min(int(self.headers.get("Content-Length", "0")), 1 << 20)
        try:
            value = json.loads(self.rfile.read(length))
        except (UnicodeDecodeError, json.JSONDecodeError):
            self.send_error(HTTPStatus.BAD_REQUEST)
            return
        self.server.state.profile = value
        self.send_json(value)

    def serve_asset(self, request_path: str) -> None:
        relative = "index.html" if request_path in {"", "/"} else request_path[1:]
        candidate = (WEB_ROOT / relative).resolve()
        if WEB_ROOT.resolve() not in candidate.parents or not candidate.is_file():
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        body = candidate.read_bytes()
        content_types = {
            ".html": "text/html; charset=utf-8",
            ".css": "text/css; charset=utf-8",
            ".js": "text/javascript; charset=utf-8",
            ".svg": "image/svg+xml",
        }
        self.send_response(HTTPStatus.OK)
        self.send_header(
            "Content-Type",
            content_types.get(candidate.suffix.lower(), "application/octet-stream"),
        )
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)


def launch_browser(playwright):
    try:
        return playwright.chromium.launch(headless=True)
    except PlaywrightError:
        executable = next(
            (
                value
                for value in (
                    shutil.which("chromium"),
                    shutil.which("chromium-browser"),
                    shutil.which("google-chrome"),
                )
                if value
            ),
            None,
        )
        if not executable:
            raise
        return playwright.chromium.launch(headless=True, executable_path=executable)


def playwright_failure_category(error: PlaywrightError) -> str:
    message = str(error).lower()
    if "executable doesn't exist" in message:
        return "BROWSER_MISSING"
    if "host system is missing dependencies" in message:
        return "BROWSER_DEPENDENCIES"
    return "BROWSER_RUNTIME"


def exercise_frontend(base_url: str, state: FixtureState) -> None:
    with sync_playwright() as playwright:
        browser = launch_browser(playwright)
        try:
            for scale in (1, 1.25, 1.5, 2):
                context = browser.new_context(
                    viewport={"width": 720, "height": 820},
                    device_scale_factor=scale,
                )
                context.add_init_script(
                    """
                    window.__externalURLs = [];
                    window.go = {main: {DesktopBridge: {OpenExternalURL: async value => {
                      window.__externalURLs.push(String(value));
                    }}}};
                    """
                )
                page = context.new_page()
                errors: list[str] = []
                page.on(
                    "pageerror",
                    lambda error, page_errors=errors: page_errors.append(str(error)),
                )
                page.goto(base_url, wait_until="networkidle")
                page.wait_for_selector(".vk-news-card")
                require(
                    page.locator(".version-status-number").inner_text()
                    == "Версия 2.0.0",
                    "version label regression",
                )
                require(
                    "Запись № 62337" in page.locator(".vk-news-card-meta").inner_text(),
                    "VK post metadata regression",
                )
                require(
                    page.locator(".vk-news-card img[onerror]").count() == 0,
                    "remote news text was not escaped",
                )
                require(
                    not page.evaluate(
                        "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                    ),
                    "desktop frontend has horizontal overflow",
                )
                require(not errors, "desktop frontend raised a JavaScript error")
                context.close()

            context = browser.new_context(viewport={"width": 1280, "height": 900})
            context.add_init_script(
                """
                window.__externalURLs = [];
                window.go = {main: {DesktopBridge: {OpenExternalURL: async value => {
                  window.__externalURLs.push(String(value));
                }}}};
                """
            )
            page = context.new_page()
            errors = []
            page.on("pageerror", lambda error: errors.append(str(error)))
            page.goto(base_url, wait_until="networkidle")
            page.locator('.home-resources a[href="https://irisonline.ru/"]').click()
            require(
                page.url.startswith(base_url), "external link navigated inside WebView"
            )
            require(
                page.evaluate("window.__externalURLs") == ["https://irisonline.ru/"],
                "external URL bridge was not called",
            )
            page.locator(
                '.home-resources a[href="https://github.com/fsibatov/iris-online-database"]'
            ).click(button="middle")
            require(
                page.evaluate("window.__externalURLs")
                == [
                    "https://irisonline.ru/",
                    "https://github.com/fsibatov/iris-online-database",
                ],
                "middle-click bypassed the external URL bridge",
            )

            page.evaluate("location.hash = 'item/2001'")
            page.wait_for_selector('.detail-page[data-route="item/2001"]')
            page.locator('.chest-content-row[href="#item/2002"]').click()
            page.wait_for_selector('.detail-page[data-route="item/2002"]')
            back = page.locator("[data-route-back]")
            require(
                back.inner_text().strip() == "Назад", "detail back action is missing"
            )
            back.click()
            page.wait_for_selector('.detail-page[data-route="item/2001"]')
            require(
                page.get_by_role("heading", name="Тестовая шкатулка").count() == 1,
                "back from a contained item did not restore its chest",
            )

            page.evaluate("location.hash = 'monster/42'")
            page.wait_for_selector('.detail-page[data-route="monster/42"]')
            rows = page.locator(".drop-preview-list a")
            require(rows.count() == 2, "compact loot preview row count regression")
            require(
                rows.nth(0).inner_text().strip().endswith("— 70%"),
                "integer loot chance formatting regression",
            )
            require(
                rows.nth(1).inner_text().strip().endswith("— 33,3334%"),
                "decimal loot chance formatting regression",
            )
            require(
                "за одну основную попытку"
                not in page.locator(".monster-drop-preview").inner_text().lower(),
                "verbose chance phrase remains in compact loot preview",
            )
            require(
                page.locator(
                    '.drop-preview-list a[href="#item/1001"]',
                    has_text="Серебряный глаз",
                ).count()
                == 1,
                "compact loot item link regression",
            )
            require(
                page.get_by_role("button", name="Показать всю добычу").count() == 1,
                "full loot action is missing",
            )

            page.evaluate("location.hash = 'home'")
            page.wait_for_selector(".vk-news-card")
            state.community_failures = True
            page.get_by_role("button", name="Проверить новую запись").click()
            page.wait_for_timeout(250)
            require(
                page.locator(".vk-news-card").count() == 1,
                "last-known-good VK card disappeared after network failure",
            )
            require(
                page.locator(".vk-news-text", has_text="Vulkan").count() == 1,
                "last-known-good VK text disappeared after network failure",
            )
            require(
                page.locator(".vk-news-stale", has_text="Сохранённая копия").count()
                == 1,
                "stale VK preview is not disclosed",
            )

            page.keyboard.press("/")
            require(
                page.locator("#globalSearch").evaluate(
                    "node => node === document.activeElement"
                ),
                "keyboard search shortcut did not move focus",
            )
            require(not errors, "desktop frontend raised a JavaScript error")
            context.close()
        finally:
            browser.close()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.parse_args()
    state = FixtureState()
    server = FixtureServer(state)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        host, port = server.server_address
        try:
            exercise_frontend(f"http://{host}:{port}/", state)
        except PlaywrightError as error:
            category = playwright_failure_category(error)
            if category in {"BROWSER_MISSING", "BROWSER_DEPENDENCIES"}:
                print(f"Embedded frontend smoke test: NOT EXECUTABLE [{category}]")
                return 2
            print(f"Embedded frontend smoke test: FAIL [{category}]")
            return 1
        except RuntimeError:
            print("Embedded frontend smoke test: FAIL [REGRESSION]")
            return 1
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    print("Embedded frontend smoke test: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
