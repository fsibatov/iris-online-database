from __future__ import annotations

import argparse
import json
import re
import shutil
import threading
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import Error as PlaywrightError
from playwright.sync_api import TimeoutError as PlaywrightTimeoutError
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parents[1]
CURRENT_VERSION = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
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
        "quality": "Не указано",
        "qualityId": 0,
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
        "quality": "Событийное",
        "qualityId": 9,
        "sellType": 0,
    },
    "bonuses": [],
    "drops": [],
}

ENHANCED_ITEM = {
    "item": {
        "id": 2003,
        "name": "Тестовый усиленный посох",
        "category": "Оружие",
        "typeLine": "Посох",
        "quality": "Редкое",
        "qualityId": 4,
        "physicalMin": 10,
        "physicalMax": 20,
        "enhancedIndex": 1,
        "sellType": 0,
    },
    "bonuses": [],
    "drops": [],
    "enhancement": {
        "profileId": 1,
        "maxLevel": 10,
        "levels": [
            {
                "level": level,
                "label": "Без усиления" if level == 0 else f"+{level}",
                "stats": [
                    {
                        "type": 1201,
                        "name": "Физическая атака",
                        "baseMin": 10,
                        "baseMax": 20,
                        "bonus": level,
                        "percent": level,
                        "isRange": True,
                    }
                ],
            }
            for level in range(11)
        ],
    },
}

TITLE = {
    "title": {"index": 991, "name": "Антагонист I", "level": 1},
    "effect": "+1 к тестовой характеристике",
    "drops": [],
    "itemIds": [1550112],
}

TRANSFORMATION_EMPTY = {
    "card": {
        "itemId": 3001,
        "name": "Карта пустой формы",
        "formName": "Тестовая форма",
        "quality": "Не указано",
        "qualityId": 0,
        "monsterId": 4001,
        "runSpeed": 450,
        "effectiveRunSpeed": 450,
        "speedDelta": 0,
        "speedDeltaPercent": 0,
        "durationMs": 0,
        "formCharacteristics": [],
        "skills": [],
    },
    "basePlayerRunSpeed": 450,
    "drops": [],
}

TRANSFORMATION_CATALOG = {
    "transformations": [
        {
            "id": 1021022,
            "itemId": 1021022,
            "name": "Карта превращения нииля",
            "formName": "Превращение в Нииля",
            "quality": "Необычное",
            "qualityId": 3,
            "effectiveRunSpeed": 460,
            "speedDelta": 10,
            "speedDeltaPercent": 2.2,
            "skillStatuses": [
                {"kind": "effect", "name": "Волна исцеления"},
                {"kind": "effect", "name": "Снятие отрицательных эффектов"},
            ],
        }
    ],
    "total": 1,
    "page": 1,
    "pages": 1,
    "filters": {"characteristics": [], "qualities": ["Необычное"]},
}


class FixtureState:
    def __init__(self) -> None:
        self.profile = profile()
        self.community_failures = False
        self.stage = "initialization"
        self.page_errors: list[str] = []
        self.page_state: dict[str, object] | None = None


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

    def do_GET(self) -> None:  # noqa: N802
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
                    "currentVersion": CURRENT_VERSION,
                    "latestVersion": CURRENT_VERSION,
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
        if parsed.path == "/api/items/2003":
            self.send_json(ENHANCED_ITEM)
            return
        if parsed.path == "/api/titles/991":
            self.send_json(TITLE)
            return
        if parsed.path == "/api/transformations/3001":
            self.send_json(TRANSFORMATION_EMPTY)
            return
        if parsed.path == "/api/transformations":
            self.send_json(TRANSFORMATION_CATALOG)
            return
        self.serve_asset(parsed.path)

    def do_PUT(self) -> None:  # noqa: N802
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


def launch_browser(playwright, *, hide_scrollbars: bool = True):
    ignored_arguments = [] if hide_scrollbars else ["--hide-scrollbars"]
    try:
        return playwright.chromium.launch(
            headless=True, ignore_default_args=ignored_arguments
        )
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
        return playwright.chromium.launch(
            headless=True,
            executable_path=executable,
            ignore_default_args=ignored_arguments,
        )


def playwright_failure_category(error: PlaywrightError) -> str:
    message = str(error).lower()
    if "executable doesn't exist" in message:
        return "BROWSER_MISSING"
    if "host system is missing dependencies" in message:
        return "BROWSER_DEPENDENCIES"
    if isinstance(error, PlaywrightTimeoutError):
        return "BROWSER_TIMEOUT"
    if "target page, context or browser has been closed" in message:
        return "BROWSER_CLOSED"
    return "BROWSER_RUNTIME"


def failure_details(error: object) -> str:
    message = re.sub(r"\x1b\[[0-?]*[ -/]*[@-~]", "", str(error)).strip()
    return re.sub(
        r"(?i)(?:[a-z]:[\\/]Users[\\/]|/(?:home|Users)/)[^\\/\r\n\"']+",
        "[user]",
        message,
    )


def report_failure(state: FixtureState, error: object, category: str) -> int:
    unavailable = category in {"BROWSER_MISSING", "BROWSER_DEPENDENCIES"}
    status = "NOT EXECUTABLE" if unavailable else "FAIL"
    print(f"Embedded frontend smoke test: {status} [{category}]")
    print(f"Stage: {state.stage}")
    print(failure_details(error))
    if state.page_state is not None:
        print(f"Page: {json.dumps(state.page_state, ensure_ascii=True)}")
    for page_error in state.page_errors:
        print(f"JavaScript: {failure_details(page_error)}")
    return 2 if unavailable else 1


def smoke_page(context, state: FixtureState):
    page = context.new_page()
    errors: list[str] = []
    state.page_errors = errors
    state.page_state = None
    page.on("pageerror", lambda error: errors.append(str(error)))
    return page, errors


def capture_failure_state(page, state: FixtureState) -> None:
    if page is None:
        return
    try:
        state.page_state = page.evaluate(
            """() => ({
                route: location.hash,
                focus: document.activeElement?.id || document.activeElement?.tagName,
                dialogOpen: document.getElementById('infoDialog')?.open,
                filtersOpen: document.getElementById('filterDrawer')?.hidden === false,
                scrollLocked: document.documentElement.classList.contains('overlay-open'),
                viewport: [innerWidth, innerHeight],
                contentWidth: document.body?.getBoundingClientRect().width
            })"""
        )
    except PlaywrightError:
        state.page_state = None


def refresh_news(page) -> None:
    button = page.locator('[data-action="refresh-vk-news"]')
    button.click()
    page.locator('[data-action="refresh-vk-news"]:not(:disabled)').wait_for(
        state="visible"
    )


def require_header_text(page) -> None:
    require(
        page.locator(
            "#battlegroundStatus[title], #battlegroundStatus [title], "
            "#versionStatus[title], #versionStatus [title]"
        ).count()
        == 0,
        "header statuses show redundant hover tooltips",
    )
    require(
        page.locator("#battlegroundCountdown").evaluate(
            """node => {
                const timer = getComputedStyle(node);
                const version = getComputedStyle(document.querySelector('.version-status-number'));
                return ['fontFamily', 'fontSize', 'fontWeight'].every(key => timer[key] === version[key]);
            }"""
        ),
        "countdown typography differs from the version number",
    )
    for selector in ("#battlegroundStatus", "#versionStatus"):
        geometry = page.locator(selector).evaluate(
            """node => {
                const bounds = node.getBoundingClientRect();
                const boxes = Array.from(node.children, child => child.getBoundingClientRect());
                const dot = node.querySelector('.version-status-dot')?.getBoundingClientRect();
                const center = (bounds.top + bounds.bottom) / 2;
                return {
                    diameter: dot ? Math.min(dot.width, dot.height) : null,
                    gap: Math.min(...boxes.slice(1).map((box, index) => box.left - boxes[index].right)),
                    aligned: boxes.every(box => Math.abs((box.top + box.bottom) / 2 - center) <= 1),
                    contained: boxes.every(box =>
                        box.left >= bounds.left && box.right <= bounds.right &&
                        box.top >= bounds.top && box.bottom <= bounds.bottom)
                };
            }"""
        )
        require(
            (geometry["diameter"] is None or geometry["diameter"] >= 10)
            and geometry["gap"] >= 4
            and geometry["aligned"]
            and geometry["contained"],
            f"header status elements do not fit on one line: {selector}, {geometry}",
        )
    samples = {
        "#battlegroundName": ["Противостояние", "Захват флага", "Горнило"],
        "#battlegroundCountdown": ["00:00", "29:59"],
        ".version-status-number": [f"Версия {CURRENT_VERSION}"],
        "#versionStatusText": [
            "Не проверена",
            "Проверка…",
            "Проверить",
            "Обновление",
            "Актуальная",
        ],
    }
    for selector, texts in samples.items():
        node = page.locator(selector)
        require(node.is_visible(), f"header text is hidden: {selector}")
        metrics = node.evaluate(
            """(node, samples) => {
                const bounds = node.getBoundingClientRect();
                const style = getComputedStyle(node);
                const range = document.createRange();
                range.selectNodeContents(node);
                const text = range.getBoundingClientRect();
                const canvas = document.createElement('canvas');
                const context = canvas.getContext('2d');
                context.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
                const widths = samples.map(value => context.measureText(value).width);
                return {
                    text: node.textContent,
                    width: bounds.width,
                    requiredWidth: Math.max(text.width, ...widths),
                    top: text.top - bounds.top,
                    bottom: bounds.bottom - text.bottom,
                    lineHeight: style.lineHeight
                };
            }""",
            texts,
        )
        require(
            metrics["requiredWidth"] <= metrics["width"] + 1
            and metrics["top"] >= -1
            and metrics["bottom"] >= -1,
            f"header text does not fit: {selector}, {metrics}",
        )


def exercise_layout(page, state: FixtureState, mode: str) -> None:
    selectors = (".topbar", "#searchWidget", "#battlegroundStatus", "#versionStatus")
    for theme in ("dark", "light"):
        state.stage = f"layout/{mode}/{theme}/theme"
        if page.locator("html").get_attribute("data-theme") != theme:
            page.locator("#moreButton").click()
            page.locator('[data-menu-action="theme"]').click()
        require(
            page.locator("html").get_attribute("data-theme") == theme,
            f"requested theme was not applied: {theme}",
        )
        for width, height in (
            (320, 820),
            (390, 844),
            (720, 520),
            (1024, 768),
            (1280, 820),
            (1440, 1000),
        ):
            page.set_viewport_size({"width": width, "height": height})
            baseline = {}
            for route in ("home", "transformations", "item/2001"):
                state.stage = f"layout/{mode}/{theme}/{width}px/route:{route}"
                page.evaluate("route => { location.hash = route; }", route)
                page.wait_for_selector(f'.page[data-route="{route}"]:not([aria-busy])')
                page.evaluate("window.scrollTo({top: 0, behavior: 'instant'})")
                require(
                    not page.evaluate(
                        "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                    ),
                    f"horizontal overflow: {theme}, {width}px, {route}",
                )
                for selector in selectors:
                    box = page.locator(selector).bounding_box()
                    require(box is not None, f"missing layout element: {selector}")
                    if selector in baseline:
                        require(
                            all(
                                abs(box[key] - baseline[selector][key]) <= 1
                                for key in ("x", "y", "width", "height")
                            ),
                            f"header shifts between routes: {selector}, {theme}, {width}px",
                        )
                    else:
                        baseline[selector] = box
                require(
                    re.fullmatch(
                        r"\d{2}:\d{2}",
                        page.locator("#battlegroundCountdown").inner_text().strip(),
                    )
                    is not None,
                    "countdown contains extra labels",
                )
                require(
                    page.locator("#battlegroundName").inner_text()
                    in ("Противостояние", "Захват флага", "Горнило"),
                    "battleground name is missing or incorrect",
                )
                require(
                    re.search(
                        r"БГ|UTC",
                        page.locator("#battlegroundStatus").inner_text(),
                    )
                    is None,
                    "battleground contains a redundant label or server time",
                )
                require_header_text(page)
                require(
                    page.locator("#battlegroundCountdown").evaluate(
                        r"""node => {
                            const [r, g, b] = getComputedStyle(node).color.match(/\d+/g).map(Number);
                            return g > r && g > b;
                        }"""
                    ),
                    f"countdown is not green in the {theme} theme",
                )
                require(
                    page.locator("#serverSelect").evaluate(
                        "node => node.getBoundingClientRect().width >= 120"
                    ),
                    f"server selector collapsed at {width}px",
                )
            overlay_baseline = {
                selector: page.locator(selector).bounding_box()
                for selector in (*selectors, "#mainContent", "#mobileNav")
                if page.locator(selector).is_visible()
            }
            scroll_before = page.evaluate("window.scrollY")
            dialog_top = None
            for action in ("about", "data"):
                state.stage = f"layout/{mode}/{theme}/{width}px/dialog:{action}/open"
                page.locator("#moreButton").click()
                page.locator(f'[data-menu-action="{action}"]').click()
                page.wait_for_selector("#infoDialog[open]")
                box = page.locator("#infoDialog").bounding_box()
                require(box is not None, "information dialog is missing")
                if dialog_top is not None:
                    require(
                        abs(box["y"] - dialog_top) <= 1,
                        "information dialogs change their opening position",
                    )
                dialog_top = box["y"]
                for selector, before in overlay_baseline.items():
                    box = page.locator(selector).bounding_box()
                    require(
                        box is not None
                        and before is not None
                        and abs(box["x"] - before["x"]) <= 1
                        and abs(box["width"] - before["width"]) <= 1,
                        f"opening a dialog shifts the page: {selector}, {width}px, "
                        f"before={before}, after={box}",
                    )
                state.stage = f"layout/{mode}/{theme}/{width}px/dialog:{action}/close"
                page.keyboard.press("Escape")
                page.wait_for_selector("#infoDialog[open]", state="hidden")
                page.locator("html:not(.overlay-open) #moreButton:focus").wait_for(
                    state="visible"
                )
                require(
                    abs(page.evaluate("window.scrollY") - scroll_before) <= 1,
                    "closing a dialog changed the page scroll position",
                )
                for selector, before in overlay_baseline.items():
                    box = page.locator(selector).bounding_box()
                    require(
                        box is not None
                        and before is not None
                        and abs(box["x"] - before["x"]) <= 1
                        and abs(box["width"] - before["width"]) <= 1,
                        f"closing a dialog shifts the page: {selector}, {width}px, "
                        f"before={before}, after={box}",
                    )
            state.stage = f"layout/{mode}/{theme}/{width}px/filters/open"
            page.evaluate("location.hash = 'transformations'")
            page.wait_for_selector('.catalog-page[data-route="transformations"]')
            before = page.locator(".topbar").bounding_box()
            page.locator(".filter-button").click()
            page.wait_for_selector("#filterDrawer", state="visible")
            box = page.locator(".topbar").bounding_box()
            require(
                before is not None
                and box is not None
                and abs(box["x"] - before["x"]) <= 1
                and abs(box["width"] - before["width"]) <= 1,
                f"opening filters shifts the header: {width}px, before={before}, after={box}",
            )
            state.stage = f"layout/{mode}/{theme}/{width}px/filters/close"
            page.locator("#closeFiltersButton").click()
            page.wait_for_selector("#filterDrawer", state="hidden")
            require(
                page.evaluate(
                    "!document.documentElement.classList.contains('overlay-open')"
                    " && !document.querySelector('.app-shell').hasAttribute('inert')"
                ),
                "closing filters left the background locked",
            )
    state.stage = f"layout/{mode}/restore-home"
    page.locator("#moreButton").click()
    page.locator('[data-menu-action="theme"]').click()
    page.set_viewport_size({"width": 1280, "height": 900})
    page.evaluate("location.hash = 'home'")
    page.wait_for_selector('.home-page[data-route="home"]')


def exercise_frontend(base_url: str, state: FixtureState) -> None:
    with sync_playwright() as playwright:
        state.stage = "browser/hidden-scrollbars/launch"
        browser = launch_browser(playwright)
        page = None
        try:
            for scale in (1, 1.25, 1.5, 2):
                state.stage = f"startup/scale:{scale}"
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
                page, errors = smoke_page(context, state)
                page.goto(base_url, wait_until="networkidle")
                page.wait_for_selector(".vk-news-card")
                require(
                    page.locator(".version-status-number").inner_text()
                    == f"Версия {CURRENT_VERSION}",
                    "version label regression",
                )
                require_header_text(page)
                require(
                    "Запись № 62337" in page.locator(".vk-news-card-meta").inner_text(),
                    "VK post metadata regression",
                )
                require(
                    page.locator(".vk-news-card img[onerror]").count() == 0,
                    "remote news text was not escaped",
                )
                state.stage = f"news/scale:{scale}/refresh"
                refresh_news(page)
                toast_text = page.locator("#toast").inner_text().strip()
                require(
                    toast_text == "Новых записей ВКонтакте нет.",
                    f"unchanged VK refresh has misleading status text: {toast_text!r}",
                )
                require(
                    not page.evaluate(
                        "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                    ),
                    "desktop frontend has horizontal overflow",
                )
                require(not errors, "desktop frontend raised a JavaScript error")
                context.close()

            state.stage = "desktop/startup"
            context = browser.new_context(viewport={"width": 1280, "height": 900})
            context.add_init_script(
                """
                window.__externalURLs = [];
                window.go = {main: {DesktopBridge: {OpenExternalURL: async value => {
                  window.__externalURLs.push(String(value));
                }}}};
                """
            )
            page, errors = smoke_page(context, state)
            page.goto(base_url, wait_until="networkidle")
            exercise_layout(page, state, "hidden-scrollbars")
            state.stage = "desktop/external-links"
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

            state.stage = "desktop/chest"
            page.evaluate("location.hash = 'item/2001'")
            page.wait_for_selector('.detail-page[data-route="item/2001"]')
            require(
                page.locator(".rarity-label.quality-shop").inner_text().strip()
                == "Покупной",
                "purchased tooltip category color/label regression",
            )
            state.stage = "desktop/chest/contained-item"
            page.locator('.chest-content-row[href="#item/2002"]').click()
            page.wait_for_selector('.detail-page[data-route="item/2002"]')
            require(
                page.locator(".rarity-label.quality-event").inner_text().strip()
                == "Ивентовый",
                "event tooltip category color/label regression",
            )
            back = page.locator("[data-route-back]")
            require(
                back.inner_text().strip() == "Назад", "detail back action is missing"
            )
            require(
                page.locator(".breadcrumbs").inner_text().strip() == "Назад",
                "detail breadcrumb repeats the current entity name",
            )
            back_style = back.evaluate(
                "el => ({size: parseFloat(getComputedStyle(el).fontSize), "
                "weight: parseInt(getComputedStyle(el).fontWeight, 10)})"
            )
            require(
                back_style["size"] >= 16 and back_style["weight"] >= 700,
                "detail back action is not visually prominent",
            )
            state.stage = "desktop/chest/back"
            back.click()
            page.wait_for_selector('.detail-page[data-route="item/2001"]')
            require(
                page.get_by_role("heading", name="Тестовая шкатулка").count() == 1,
                "back from a contained item did not restore its chest",
            )

            state.stage = "desktop/title"
            page.evaluate("location.hash = 'title/991'")
            page.wait_for_selector('.detail-page[data-route="title/991"]')
            title_heading = page.get_by_role("heading", name="Антагонист I")
            title_box = title_heading.bounding_box()
            require(
                title_box is not None and title_box["width"] >= 300,
                "title detail heading collapsed on desktop",
            )
            require(
                title_box["height"] <= 80,
                "title detail heading wrapped vertically on desktop",
            )
            require(
                page.locator(".detail-summary--title").count() == 1,
                "title detail does not use the dedicated two-column layout",
            )
            title_badge = page.locator(".title-index-badge--large")
            require(
                title_badge.inner_text().strip() == "991",
                "title detail index badge is missing or malformed",
            )
            badge_metrics = page.evaluate(
                """() => {
                    const heading = document.querySelector('.title-heading-line > h1');
                    const badge = document.querySelector('.title-index-badge--large');
                    const style = getComputedStyle(heading);
                    const canvas = document.createElement('canvas');
                    const context = canvas.getContext('2d');
                    context.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
                    const glyph = context.measureText('А');
                    const glyphHeight = glyph.actualBoundingBoxAscent + glyph.actualBoundingBoxDescent;
                    return { badgeHeight: badge.getBoundingClientRect().height, glyphHeight };
                }"""
            )
            require(
                abs(badge_metrics["badgeHeight"] - badge_metrics["glyphHeight"]) <= 4,
                "title index badge height does not match the title capital height",
            )
            technical = page.get_by_text("Технические сведения", exact=True)
            require(technical.count() == 1, "title technical details are missing")
            technical.click()
            require(
                page.get_by_text("Связанный предмет — ID", exact=True).count() == 1,
                "title technical item reference is missing",
            )
            state.stage = "desktop/title/360px"
            page.set_viewport_size({"width": 360, "height": 780})
            page.wait_for_timeout(50)
            mobile_badge_metrics = page.evaluate(
                """() => {
                    const heading = document.querySelector('.title-heading-line > h1');
                    const badge = document.querySelector('.title-index-badge--large');
                    const style = getComputedStyle(heading);
                    const canvas = document.createElement('canvas');
                    const context = canvas.getContext('2d');
                    context.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
                    const glyph = context.measureText('А');
                    const glyphHeight = glyph.actualBoundingBoxAscent + glyph.actualBoundingBoxDescent;
                    return { badgeHeight: badge.getBoundingClientRect().height, glyphHeight };
                }"""
            )
            require(
                abs(
                    mobile_badge_metrics["badgeHeight"]
                    - mobile_badge_metrics["glyphHeight"]
                )
                <= 4,
                "title index badge height diverges on a narrow viewport",
            )
            title_box = title_heading.bounding_box()
            require(
                title_box is not None and title_box["width"] >= 160,
                "title detail heading collapsed on a narrow viewport",
            )
            require(
                title_box["height"] <= 96,
                "title detail heading became a vertical word stack",
            )
            require(
                not page.evaluate(
                    "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                ),
                "title detail introduced horizontal overflow",
            )
            page.set_viewport_size({"width": 1280, "height": 900})

            state.stage = "desktop/enhancement"
            page.evaluate("location.hash = 'item/2003'")
            page.wait_for_selector('.detail-page[data-route="item/2003"]')
            enhancement = page.locator("[data-enhancement-level]")
            require(enhancement.count() == 1, "enhancement level selector is missing")
            require(
                enhancement.locator('option[value="10"]').inner_text().strip() == "+10",
                "+10 enhancement option is malformed",
            )
            enhancement_width = enhancement.evaluate(
                "node => node.getBoundingClientRect().width"
            )
            require(
                enhancement_width >= 80,
                f"enhancement level selector is too narrow: {enhancement_width}",
            )
            enhancement.select_option("10")
            require(
                enhancement.input_value() == "10", "+10 enhancement is not selectable"
            )
            page.set_viewport_size({"width": 320, "height": 780})
            page.wait_for_timeout(50)
            require(
                not page.evaluate(
                    "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                ),
                "enhancement selector introduced horizontal overflow at 320px",
            )
            page.set_viewport_size({"width": 1280, "height": 900})

            state.stage = "desktop/transformations/catalogue"
            page.evaluate("location.hash = 'transformations'")
            page.wait_for_selector('.catalog-page[data-catalog-kind="transformations"]')
            niil = page.locator(
                ".transformation-result-row", has_text="Карта превращения нииля"
            )
            require(niil.count() == 1, "Niil transformation preview is missing")
            require(
                niil.locator(".transformation-status--effect").inner_text().strip()
                == "Эффект: Волна исцеления, Снятие отрицательных эффектов",
                "Niil utility effects are missing from the transformation preview",
            )

            state.stage = "desktop/transformations/detail"
            page.evaluate("location.hash = 'transformation/3001'")
            page.wait_for_selector('.detail-page[data-route="transformation/3001"]')
            require(
                page.get_by_role("heading", name="Карта пустой формы").count() == 1,
                "transformation detail fixture did not render",
            )
            require(
                page.locator(".transformation-buffs").count() == 0,
                "empty beneficial-effects section is still rendered",
            )
            require(
                page.locator(".transformation-skills").count() == 0,
                "empty transformation-skills section is still rendered",
            )
            require(
                page.get_by_text("Полезные эффекты", exact=True).count() == 0,
                "empty beneficial-effects heading remains visible",
            )
            require(
                page.locator(".transformation-detail .empty-copy").count() == 0,
                "transformation detail still renders an empty-data placeholder",
            )
            for narrow_width in (320, 380):
                state.stage = f"desktop/transformations/{narrow_width}px"
                page.set_viewport_size({"width": narrow_width, "height": 780})
                page.wait_for_timeout(50)
                require(
                    not page.evaluate(
                        "document.documentElement.scrollWidth > document.documentElement.clientWidth"
                    ),
                    f"transformation detail has horizontal overflow at {narrow_width}px",
                )
            page.set_viewport_size({"width": 1280, "height": 900})

            state.stage = "desktop/monster"
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

            state.stage = "news/stale-response"
            page.evaluate("location.hash = 'home'")
            page.wait_for_selector(".vk-news-card")
            state.community_failures = True
            refresh_news(page)
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
            toast_text = page.locator("#toast").inner_text().strip()
            require(
                toast_text
                == "Не удалось проверить обновление: показана сохранённая запись ВКонтакте.",
                f"stale VK refresh has misleading status text: {toast_text!r}",
            )

            state.stage = "desktop/search-shortcut"
            page.keyboard.press("/")
            require(
                page.locator("#globalSearch").evaluate(
                    "node => node === document.activeElement"
                ),
                "keyboard search shortcut did not move focus",
            )
            require(not errors, "desktop frontend raised a JavaScript error")
            context.close()
        except (PlaywrightError, RuntimeError):
            capture_failure_state(page, state)
            raise
        finally:
            browser.close()

        state.stage = "browser/visible-scrollbars/launch"
        state.page_errors = []
        browser = launch_browser(playwright, hide_scrollbars=False)
        page = None
        try:
            context = browser.new_context(
                viewport={"width": 1280, "height": 900}, device_scale_factor=1.25
            )
            page, errors = smoke_page(context, state)
            page.goto(base_url, wait_until="networkidle")
            exercise_layout(page, state, "visible-scrollbars")
            require(not errors, "scrollbar layout raised a JavaScript error")
            context.close()
        except (PlaywrightError, RuntimeError):
            capture_failure_state(page, state)
            raise
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
            return report_failure(state, error, category)
        except RuntimeError as error:
            return report_failure(state, error, "REGRESSION")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    print("Embedded frontend smoke test: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
